package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

type Config struct {
	Enabled       bool     `mapstructure:"enabled"`
	PolicyDir     string   `mapstructure:"policy_dir"`
	PolicyFile    string   `mapstructure:"policy_file"`
	Presets       []string `mapstructure:"presets"`
	DefaultAction string   `mapstructure:"default_action"` // "deny" or "allow"
}

type AgentContext struct {
	KeyHash   string            `json:"key_hash"`
	Name      string            `json:"name"`
	AgentName string            `json:"agent_name"`
	Owner     string            `json:"owner"`
	Provider  string            `json:"provider"`
	Tags      map[string]string `json:"tags"`
}

type RequestContext struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Method    string `json:"method"` // "llm_call" or "mcp_tool_call"
	ToolName  string `json:"tool_name,omitempty"`
	MCPServer string `json:"mcp_server,omitempty"`
	Path      string `json:"path"`
}

type SessionTrace struct {
	Timestamp int64                  `json:"timestamp"`
	Type      string                 `json:"type"`                // "llm_call" | "llm_response" | "mcp_tool_call" | "mcp_tool_response"
	Provider  string                 `json:"provider"`            // target provider or tool server
	Model     string                 `json:"model,omitempty"`     // LLM model name (if applicable)
	Content   string                 `json:"content,omitempty"`   // truncated prompt text or tool response string
	ToolName  string                 `json:"tool_name,omitempty"` // tool name if tool call
	Arguments map[string]interface{} `json:"arguments,omitempty"` // tool arguments if tool call
}

type SessionDriftContext struct {
	AnchorSimilarity    float64 `json:"anchor_similarity"`
	PriorTurnSimilarity float64 `json:"prior_turn_similarity"`
	DriftScore          float64 `json:"drift_score"`
	DriftDetected       bool    `json:"drift_detected"`
	TurnCount           int     `json:"turn_count"`
}

type SessionContext struct {
	ID          string              `json:"id,omitempty"`
	State       string              `json:"state,omitempty"`
	Spend       float64             `json:"spend,omitempty"`
	Steps       int                 `json:"steps,omitempty"`
	TaintFlags  map[string]bool     `json:"taint_flags"`  // Persistent taint flags for the session (e.g. "secret_accessed")
	ToolsCalled []string            `json:"tools_called"` // Recent tool call history (newest first, capped at 50)
	Traces      []SessionTrace      `json:"traces"`       // Recent request/response traces (newest first, capped at 15)
	Drift       SessionDriftContext `json:"drift,omitempty"`
}

type CanonicalMessage struct {
	Role    string `json:"role"`    // "system" | "user" | "assistant" | "tool"
	Content string `json:"content"` // Text content of the message
}

type CanonicalToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type CanonicalToolCall struct {
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ObfuscationContext struct {
	ObfuscationDetected bool     `json:"obfuscation_detected"`
	HasHomoglyphs       bool     `json:"has_homoglyphs"`
	HasInvisibleChars   bool     `json:"has_invisible_chars"`
	HasBase64Payloads   bool     `json:"has_base64_payloads"`
	HasEncodingAttacks  bool     `json:"has_encoding_attacks"`
	HasDelimPadding     bool     `json:"has_delim_padding"`
	DecodedLayers       []string `json:"decoded_layers,omitempty"`
}

type ActionContext struct {
	Type             string                    `json:"type"`                        // "llm_call" | "mcp_tool_call"
	Provider         string                    `json:"provider"`                    // "openai" | "anthropic" | "gemini" | etc.
	Model            string                    `json:"model"`                       // e.g. "gpt-4o"
	PromptText       string                    `json:"prompt_text"`                 // Concatenated raw prompts
	NormalizedPrompt string                    `json:"normalized_prompt,omitempty"` // Canonical de-obfuscated prompt
	Obfuscation      ObfuscationContext        `json:"obfuscation,omitempty"`       // Obfuscation detection telemetry
	ToolName         string                    `json:"tool_name,omitempty"`         // if tool call
	ToolArguments    map[string]interface{}    `json:"tool_arguments,omitempty"`    // if tool call
	Messages         []CanonicalMessage        `json:"messages,omitempty"`          // Canonical messages
	Tools            []CanonicalToolDefinition `json:"tools,omitempty"`             // Canonical tools defined
	ToolCalls        []CanonicalToolCall       `json:"tool_calls,omitempty"`        // Canonical tool calls requested
}

type AgentRiskContext struct {
	RiskScore            int      `json:"risk_score"`
	TotalPolicyBlocks    int64    `json:"total_policy_blocks"`
	TotalEscalations     int64    `json:"total_escalations"`
	TotalSpend           float64  `json:"total_spend"`
	PersistentTaintFlags []string `json:"persistent_taint_flags"`
	SessionCount         int64    `json:"session_count"`
	QuarantineActive     bool     `json:"quarantine_active"`
}

type EvalInput struct {
	Agent     AgentContext     `json:"agent"`
	Request   RequestContext   `json:"request"`
	Session   SessionContext   `json:"session,omitempty"`
	Action    ActionContext    `json:"action,omitempty"`
	AgentRisk AgentRiskContext `json:"agent_risk,omitempty"`
}

type TransformRule struct {
	Field     string `json:"field" yaml:"field"`
	Operation string `json:"operation" yaml:"operation"` // "mask" | "redact"
}

type Decision struct {
	Action        string          `json:"action"` // "allow" | "deny" | "escalate" | "quarantine" | "transform"
	Reason        string          `json:"reason,omitempty"`
	Severity      string          `json:"severity,omitempty"`       // "info" | "warn" | "critical"
	Transforms    []TransformRule `json:"transforms,omitempty"`     // Only relevant when Action == "transform"
	EscalateTo    string          `json:"escalate_to,omitempty"`    // "human" | "saas_control_plane"
	QuarantineFor string          `json:"quarantine_for,omitempty"` // Go duration string e.g. "1h", "30m"
	Evidence      []string        `json:"evidence,omitempty"`       // Rule names that triggered this verdict
}

// IsAllowed returns true if the action is allow.
func (d Decision) IsAllowed() bool {
	return d.Action == "allow"
}

type Engine struct {
	cfg            Config
	mu             sync.RWMutex
	modules        map[string]*ast.Module
	compiler       *ast.Compiler
	allowQuery     rego.PreparedEvalQuery
	denyQuery      rego.PreparedEvalQuery
	decisionsQuery rego.PreparedEvalQuery
	fsm            *FSMConfig
}

func NewEngine(cfg Config) (*Engine, error) {
	if cfg.PolicyDir == "" {
		cfg.PolicyDir = "./policies"
	}
	if cfg.DefaultAction == "" {
		cfg.DefaultAction = "deny"
	}

	e := &Engine{
		cfg: cfg,
	}

	if err := e.Reload(); err != nil {
		return nil, err
	}

	return e, nil
}

func (e *Engine) Reload() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.fsm = nil

	modules := make(map[string]*ast.Module)

	absDir, err := filepath.Abs(e.cfg.PolicyDir)
	if err != nil {
		return fmt.Errorf("invalid policy dir: %w", err)
	}
	if symDir, err := filepath.EvalSymlinks(absDir); err == nil {
		absDir = symDir
	}
	e.cfg.PolicyDir = absDir

	// Check if directory exists
	info, err := os.Stat(e.cfg.PolicyDir)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Logger.Warn().Str("policy_dir", e.cfg.PolicyDir).Msg("Policy directory does not exist, creating it")
			if err := os.MkdirAll(e.cfg.PolicyDir, 0700); err != nil {
				return fmt.Errorf("failed to create policy directory: %w", err)
			}
		} else {
			return fmt.Errorf("failed to stat policy directory: %w", err)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("policy path %s is not a directory", e.cfg.PolicyDir)
	}

	err = filepath.Walk(e.cfg.PolicyDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".rego") {
			return nil
		}

		// VULN-033: Prevent directory traversal via symlinks
		absPath, absErr := filepath.Abs(path)
		if absErr != nil || !strings.HasPrefix(absPath, e.cfg.PolicyDir) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read policy file %s: %w", path, err)
		}

		hash := sha256.Sum256(content)
		checksum := hex.EncodeToString(hash[:])
		logging.Logger.Debug().Str("file", path).Str("sha256", checksum).Msg("Loaded policy file")

		module, err := ast.ParseModule(path, string(content))
		if err != nil {
			return fmt.Errorf("failed to parse policy file %s: %w", path, err)
		}

		modules[path] = module
		return nil
	})

	if err != nil {
		return err
	}

	// Load and transpile PolicyFile if configured
	if e.cfg.PolicyFile != "" {
		if _, err := os.Stat(e.cfg.PolicyFile); err == nil {
			data, err := os.ReadFile(e.cfg.PolicyFile)
			if err != nil {
				return fmt.Errorf("failed to read policy file %s: %w", e.cfg.PolicyFile, err)
			}
			card, err := ParseYAML(data)
			if err != nil {
				return fmt.Errorf("failed to parse YAML policy: %w", err)
			}
			if card.FSM != nil {
				e.fsm = card.FSM
			}
			regoCode, err := TranspileToRego(card)
			if err != nil {
				return fmt.Errorf("failed to transpile YAML policy to Rego: %w", err)
			}
			module, err := ast.ParseModule(e.cfg.PolicyFile, regoCode)
			if err != nil {
				return fmt.Errorf("failed to parse transpiled Rego module: %w", err)
			}
			modules[e.cfg.PolicyFile] = module
			logging.Logger.Info().Str("file", e.cfg.PolicyFile).Msg("Loaded YAML Policy Card successfully")
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat policy file %s: %w", e.cfg.PolicyFile, err)
		}
	}

	// If no custom policies/presets are configured or found, default to 'safety' preset
	if len(modules) == 0 && len(e.cfg.Presets) == 0 {
		logging.Logger.Info().Msg("Policy engine enabled without custom files or presets; defaulting to 'safety' preset.")
		e.cfg.Presets = []string{"safety"}
	}

	// Load and transpile presets if configured
	for _, presetName := range e.cfg.Presets {
		data, err := GetPreset(presetName)
		if err != nil {
			return fmt.Errorf("failed to load preset %s: %w", presetName, err)
		}
		card, err := ParseYAML(data)
		if err != nil {
			return fmt.Errorf("failed to parse preset %s YAML: %w", presetName, err)
		}
		if card.FSM != nil && e.fsm == nil {
			e.fsm = card.FSM
		}
		regoCode, err := TranspileToRego(card)
		if err != nil {
			return fmt.Errorf("failed to transpile preset %s YAML to Rego: %w", presetName, err)
		}
		module, err := ast.ParseModule(fmt.Sprintf("preset:%s", presetName), regoCode)
		if err != nil {
			return fmt.Errorf("failed to parse transpiled Rego module for preset %s: %w", presetName, err)
		}
		modules[fmt.Sprintf("preset:%s", presetName)] = module
		logging.Logger.Info().Str("preset", presetName).Msg("Loaded preset Policy Card successfully")
	}

	// Compile the modules together
	compiler := ast.NewCompiler()
	compiler.Compile(modules)
	if compiler.Failed() {
		return fmt.Errorf("failed to compile policies: %w", compiler.Errors)
	}

	ctx := context.Background()
	allowQuery, err := rego.New(
		rego.Query("data.loopers.policy.allow"),
		rego.Compiler(compiler),
	).PrepareForEval(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare allow query: %w", err)
	}

	denyQuery, err := rego.New(
		rego.Query("data.loopers.policy.deny"),
		rego.Compiler(compiler),
	).PrepareForEval(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare deny query: %w", err)
	}

	decisionsQuery, err := rego.New(
		rego.Query("data.loopers.policy.decisions"),
		rego.Compiler(compiler),
	).PrepareForEval(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare decisions query: %w", err)
	}

	e.modules = modules
	e.compiler = compiler
	e.allowQuery = allowQuery
	e.denyQuery = denyQuery
	e.decisionsQuery = decisionsQuery

	logging.Logger.Info().Int("modules", len(modules)).Str("policy_dir", e.cfg.PolicyDir).Msg("Policies reloaded successfully")
	return nil
}

func actionRank(action string) int {
	switch strings.ToLower(action) {
	case "deny":
		return 5
	case "quarantine":
		return 4
	case "escalate":
		return 3
	case "transform":
		return 2
	case "allow":
		return 1
	default:
		return 0
	}
}

func severityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "critical":
		return 3
	case "warn":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func parseDecisionFromMap(m map[string]interface{}) Decision {
	var d Decision
	if act, ok := m["action"].(string); ok {
		d.Action = strings.ToLower(act)
	}
	if r, ok := m["reason"].(string); ok {
		d.Reason = r
	}
	if s, ok := m["severity"].(string); ok {
		d.Severity = s
	}
	if esc, ok := m["escalate_to"].(string); ok {
		d.EscalateTo = esc
	}
	if q, ok := m["quarantine_for"].(string); ok {
		d.QuarantineFor = q
	}
	if ev, ok := m["evidence"].([]interface{}); ok {
		for _, e := range ev {
			if es, ok := e.(string); ok {
				d.Evidence = append(d.Evidence, es)
			}
		}
	} else if evStr, ok := m["evidence"].(string); ok {
		d.Evidence = append(d.Evidence, evStr)
	}
	if tr, ok := m["transforms"].([]interface{}); ok {
		for _, t := range tr {
			if tm, ok := t.(map[string]interface{}); ok {
				var rule TransformRule
				if f, ok := tm["field"].(string); ok {
					rule.Field = f
				}
				if op, ok := tm["operation"].(string); ok {
					rule.Operation = op
				}
				if rule.Field != "" {
					d.Transforms = append(d.Transforms, rule)
				}
			}
		}
	}
	return d
}

func (e *Engine) Evaluate(ctx context.Context, input EvalInput) (Decision, error) {
	e.mu.RLock()
	allowQuery := e.allowQuery
	denyQuery := e.denyQuery
	decisionsQuery := e.decisionsQuery
	hasCompiler := e.compiler != nil
	e.mu.RUnlock()

	defaultAction := strings.ToLower(e.cfg.DefaultAction)
	if defaultAction == "" {
		defaultAction = "deny"
	}

	decision := Decision{
		Action: defaultAction,
		Reason: fmt.Sprintf("default %s", defaultAction),
	}

	if !hasCompiler {
		return decision, nil
	}

	var candidates []Decision

	// 1. Run legacy allow query
	rsAllow, err := allowQuery.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return Decision{}, fmt.Errorf("failed to evaluate allow rule: %w", err)
	}

	legacyAllowed := false
	if len(rsAllow) > 0 && len(rsAllow[0].Expressions) > 0 {
		if allowed, ok := rsAllow[0].Expressions[0].Value.(bool); ok && allowed {
			legacyAllowed = true
			candidates = append(candidates, Decision{
				Action: "allow",
				Reason: "explicitly allowed by policy",
			})
		}
	}

	// 2. Run legacy deny query if allow passed or if default action is allow
	if legacyAllowed || defaultAction == "allow" {
		rsDeny, err := denyQuery.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			return Decision{}, fmt.Errorf("failed to evaluate deny rule: %w", err)
		}

		if len(rsDeny) > 0 && len(rsDeny[0].Expressions) > 0 {
			val := rsDeny[0].Expressions[0].Value
			denyReasons := []string{}

			switch v := val.(type) {
			case []interface{}:
				for _, r := range v {
					if str, ok := r.(string); ok {
						denyReasons = append(denyReasons, str)
					}
				}
			case string:
				denyReasons = append(denyReasons, v)
			case bool:
				if v {
					denyReasons = append(denyReasons, "explicitly denied by policy")
				}
			}

			if len(denyReasons) > 0 {
				candidates = append(candidates, Decision{
					Action:   "deny",
					Reason:   strings.Join(denyReasons, ", "),
					Severity: "critical",
				})
			}
		}
	}

	// 3. Run structured decisions query
	rsDecisions, err := decisionsQuery.Eval(ctx, rego.EvalInput(input))
	if err == nil && len(rsDecisions) > 0 && len(rsDecisions[0].Expressions) > 0 {
		val := rsDecisions[0].Expressions[0].Value
		if decSlice, ok := val.([]interface{}); ok {
			for _, decItem := range decSlice {
				if decMap, ok := decItem.(map[string]interface{}); ok {
					candidates = append(candidates, parseDecisionFromMap(decMap))
				}
			}
		}
	}

	// If no rules matched, return default action verdict
	if len(candidates) == 0 {
		if defaultAction == "deny" {
			decision.Reason = "no allow rule matched (default deny)"
		}
		return decision, nil
	}

	// 4. Precedence Resolution across candidates:
	// Priority: deny > quarantine > escalate > transform > allow
	maxRank := 0
	for _, c := range candidates {
		rank := actionRank(c.Action)
		if rank > maxRank {
			maxRank = rank
		}
	}

	var matchingRankDecisions []Decision
	for _, c := range candidates {
		if actionRank(c.Action) == maxRank {
			matchingRankDecisions = append(matchingRankDecisions, c)
		}
	}

	// Resolve winner amongst matching highest rank
	topAction := matchingRankDecisions[0].Action
	resolvedDecision := Decision{
		Action: topAction,
	}

	var allReasons []string
	var allEvidence []string
	maxSevRank := 0
	topSeverity := ""
	longestQuarantineDur := time.Duration(0)
	var longestQuarantineStr string
	topEscalateTo := "human"
	var allTransforms []TransformRule

	for _, d := range matchingRankDecisions {
		if d.Reason != "" {
			allReasons = append(allReasons, d.Reason)
		}
		if len(d.Evidence) > 0 {
			allEvidence = append(allEvidence, d.Evidence...)
		}
		sevRank := severityRank(d.Severity)
		if sevRank > maxSevRank {
			maxSevRank = sevRank
			topSeverity = d.Severity
		}
		if d.EscalateTo != "" {
			topEscalateTo = d.EscalateTo
		}
		if d.QuarantineFor != "" {
			if dur, parseErr := time.ParseDuration(d.QuarantineFor); parseErr == nil {
				if dur > longestQuarantineDur {
					longestQuarantineDur = dur
					longestQuarantineStr = d.QuarantineFor
				}
			} else if longestQuarantineStr == "" {
				longestQuarantineStr = d.QuarantineFor
			}
		}
		if len(d.Transforms) > 0 {
			allTransforms = append(allTransforms, d.Transforms...)
		}
	}

	resolvedDecision.Reason = strings.Join(allReasons, ", ")
	resolvedDecision.Severity = topSeverity
	resolvedDecision.Evidence = allEvidence

	switch topAction {
	case "escalate":
		resolvedDecision.EscalateTo = topEscalateTo
	case "quarantine":
		if longestQuarantineStr == "" {
			longestQuarantineStr = "1h"
		}
		resolvedDecision.QuarantineFor = longestQuarantineStr
	case "transform":
		resolvedDecision.Transforms = allTransforms
	}

	return resolvedDecision, nil
}

// FSM returns the parsed FSM configuration.
func (e *Engine) FSM() *FSMConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.fsm
}
