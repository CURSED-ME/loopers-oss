package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/policy"
)

// Config configures the verification engine.
type Config struct {
	PolicyFile    string   `json:"policy_file"`
	PolicyDir     string   `json:"policy_dir"`
	Presets       []string `json:"presets"`
	DefaultAction string   `json:"default_action"` // "allow" or "deny"
}

// SessionTraceFile represents the structured export of a session's execution traces.
type SessionTraceFile struct {
	Version    string                `json:"version,omitempty"`
	SessionID  string                `json:"session_id,omitempty"`
	Agent      *policy.AgentContext  `json:"agent,omitempty"`
	TaintFlags map[string]bool       `json:"taint_flags,omitempty"`
	Traces     []policy.SessionTrace `json:"traces"`
}

// Violation records a single policy violation at a specific step in the trace.
type Violation struct {
	StepIndex int                  `json:"step_index"`
	Timestamp int64                `json:"timestamp"`
	Action    policy.ActionContext `json:"action"`
	Reason    string               `json:"reason"`
}

// VerificationReport contains the complete compliance audit results for a trace.
type VerificationReport struct {
	SessionID       string        `json:"session_id,omitempty"`
	TotalSteps      int           `json:"total_steps"`
	ActionsAudited  int           `json:"actions_audited"`
	ViolationsCount int           `json:"violations_count"`
	Status          string        `json:"status"` // "PASSED" | "FAILED"
	Violations      []Violation   `json:"violations"`
	DurationMs      int64         `json:"duration_ms"`
}

// Verifier wraps the policy engine for offline trace validation.
type Verifier struct {
	engine *policy.Engine
	cfg    Config
}

// NewVerifier initializes a new Verifier with the specified policy configuration.
func NewVerifier(cfg Config) (*Verifier, error) {
	if cfg.DefaultAction == "" {
		cfg.DefaultAction = "allow"
	}

	policyDir := cfg.PolicyDir
	if policyDir == "" && (cfg.PolicyFile != "" || len(cfg.Presets) > 0) {
		// If specific policy file or presets are requested without an explicit policy directory,
		// use an isolated directory to avoid accidentally picking up unrelated ambient .rego files.
		tmpDir, err := os.MkdirTemp("", "loopers-verifier-policies-*")
		if err == nil {
			policyDir = tmpDir
			defer os.RemoveAll(tmpDir)
		}
	}

	pCfg := policy.Config{
		Enabled:       true,
		PolicyFile:    cfg.PolicyFile,
		PolicyDir:     policyDir,
		Presets:       cfg.Presets,
		DefaultAction: cfg.DefaultAction,
	}

	engine, err := policy.NewEngine(pCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize policy engine: %w", err)
	}

	return &Verifier{
		engine: engine,
		cfg:    cfg,
	}, nil
}

// ParseTraceData parses trace JSON data supporting both []SessionTrace and SessionTraceFile.
func ParseTraceData(data []byte) (*SessionTraceFile, error) {
	// Strip UTF-8 Byte Order Mark (BOM) if present (common on Windows PowerShell)
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	// 1. Try parsing as SessionTraceFile
	var traceFile SessionTraceFile
	if err := json.Unmarshal(data, &traceFile); err == nil && len(traceFile.Traces) > 0 {
		return &traceFile, nil
	}

	// 2. Try parsing as direct []policy.SessionTrace
	var traces []policy.SessionTrace
	if err := json.Unmarshal(data, &traces); err == nil && len(traces) > 0 {
		return &SessionTraceFile{
			Traces: traces,
		}, nil
	}

	return nil, fmt.Errorf("invalid trace data: payload must be a JSON array of SessionTrace objects or a SessionTraceFile object")
}

// VerifyTraceFile reads a JSON file and audits all traces against loaded policies.
func (v *Verifier) VerifyTraceFile(ctx context.Context, filePath string) (*VerificationReport, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read trace file %s: %w", filePath, err)
	}

	traceFile, err := ParseTraceData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse trace file %s: %w", filePath, err)
	}

	return v.VerifyTrace(ctx, traceFile)
}

// VerifyTrace sequentially simulates and verifies the trace against policies.
func (v *Verifier) VerifyTrace(ctx context.Context, traceFile *SessionTraceFile) (*VerificationReport, error) {
	startTime := time.Now()

	report := &VerificationReport{
		SessionID:  traceFile.SessionID,
		TotalSteps: len(traceFile.Traces),
		Violations: make([]Violation, 0),
		Status:     "PASSED",
	}

	// Initialize simulated session state mirroring internal/session/manager.go
	toolsCalled := make([]string, 0, len(traceFile.Traces))
	recentTraces := make([]policy.SessionTrace, 0, 15)
	taintFlags := make(map[string]bool)
	for k, val := range traceFile.TaintFlags {
		taintFlags[k] = val
	}

	agentCtx := policy.AgentContext{
		Name:      "offline-verifier",
		AgentName: "trace-auditor",
	}
	if traceFile.Agent != nil {
		agentCtx = *traceFile.Agent
	}

	for i, trace := range traceFile.Traces {
		actionType := trace.Type
		if actionType == "" {
			if trace.ToolName != "" {
				actionType = "mcp_tool_call"
			} else {
				actionType = "llm_call"
			}
		}

		// Only audit active calls (actions), skip responses
		if actionType == "llm_call" || actionType == "mcp_tool_call" {
			report.ActionsAudited++

			// Snapshot current session context up to this step (newest first)
			cappedTools := make([]string, 0, 50)
			if len(toolsCalled) > 50 {
				cappedTools = append(cappedTools, toolsCalled[:50]...)
			} else {
				cappedTools = append(cappedTools, toolsCalled...)
			}

			cappedTraces := make([]policy.SessionTrace, 0, 15)
			if len(recentTraces) > 15 {
				cappedTraces = append(cappedTraces, recentTraces[:15]...)
			} else {
				cappedTraces = append(cappedTraces, recentTraces...)
			}

			copiedTaint := make(map[string]bool, len(taintFlags))
			for k, val := range taintFlags {
				copiedTaint[k] = val
			}

			sessionCtx := policy.SessionContext{
				ID:          traceFile.SessionID,
				Steps:       i,
				TaintFlags:  copiedTaint,
				ToolsCalled: cappedTools,
				Traces:      cappedTraces,
			}

			actionCtx := policy.ActionContext{
				Type:          actionType,
				Provider:      trace.Provider,
				Model:         trace.Model,
				PromptText:    trace.Content,
				ToolName:      trace.ToolName,
				ToolArguments: trace.Arguments,
			}

			reqCtx := policy.RequestContext{
				Provider:  trace.Provider,
				Model:     trace.Model,
				Method:    actionType,
				ToolName:  trace.ToolName,
				MCPServer: trace.Provider,
			}

			evalInput := policy.EvalInput{
				Agent:   agentCtx,
				Request: reqCtx,
				Session: sessionCtx,
				Action:  actionCtx,
			}

			decision, err := v.engine.Evaluate(ctx, evalInput)
			if err != nil {
				return nil, fmt.Errorf("evaluation error at step %d: %w", i, err)
			}

			if !decision.Allowed {
				report.Violations = append(report.Violations, Violation{
					StepIndex: i,
					Timestamp: trace.Timestamp,
					Action:    actionCtx,
					Reason:    decision.Reason,
				})
			}
		}

		// Update cumulative session state for subsequent steps
		if trace.ToolName != "" {
			// Prepend newest tool call (matching Redis LPUSH)
			toolsCalled = append([]string{trace.ToolName}, toolsCalled...)
		}
		// Prepend newest trace (matching Redis LPUSH)
		recentTraces = append([]policy.SessionTrace{trace}, recentTraces...)
	}

	report.ViolationsCount = len(report.Violations)
	if report.ViolationsCount > 0 {
		report.Status = "FAILED"
	}
	report.DurationMs = time.Since(startTime).Milliseconds()

	return report, nil
}
