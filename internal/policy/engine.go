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

	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

type Config struct {
	Enabled       bool   `mapstructure:"enabled"`
	PolicyDir     string `mapstructure:"policy_dir"`
	DefaultAction string `mapstructure:"default_action"` // "deny" or "allow"
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

type SessionContext struct {
	ID    string  `json:"id,omitempty"`
	Spend float64 `json:"spend,omitempty"`
	Steps int     `json:"steps,omitempty"`
}

type EvalInput struct {
	Agent   AgentContext   `json:"agent"`
	Request RequestContext `json:"request"`
	Session SessionContext `json:"session,omitempty"`
}

type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type Engine struct {
	cfg        Config
	mu         sync.RWMutex
	modules    map[string]*ast.Module
	compiler   *ast.Compiler
	allowQuery rego.PreparedEvalQuery
	denyQuery  rego.PreparedEvalQuery
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

	modules := make(map[string]*ast.Module)

	absDir, err := filepath.Abs(e.cfg.PolicyDir)
	if err != nil {
		return fmt.Errorf("invalid policy dir: %w", err)
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

	e.modules = modules
	e.compiler = compiler
	e.allowQuery = allowQuery
	e.denyQuery = denyQuery

	logging.Logger.Info().Int("modules", len(modules)).Str("policy_dir", e.cfg.PolicyDir).Msg("Policies reloaded successfully")
	return nil
}

func (e *Engine) Evaluate(ctx context.Context, input EvalInput) (Decision, error) {
	e.mu.RLock()
	allowQuery := e.allowQuery
	denyQuery := e.denyQuery
	hasCompiler := e.compiler != nil
	e.mu.RUnlock()

	decision := Decision{
		Allowed: e.cfg.DefaultAction == "allow",
		Reason:  fmt.Sprintf("default %s", e.cfg.DefaultAction),
	}

	if !hasCompiler {
		return decision, nil
	}

	rs, err := allowQuery.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return Decision{}, fmt.Errorf("failed to evaluate allow rule: %w", err)
	}

	if len(rs) > 0 && len(rs[0].Expressions) > 0 {
		if allowed, ok := rs[0].Expressions[0].Value.(bool); ok && allowed {
			decision.Allowed = true
			decision.Reason = "explicitly allowed by policy"
		}
	}

	// Check deny rules if it's currently allowed (deny overrides allow)
	if decision.Allowed {
		rsDeny, err := denyQuery.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			return Decision{}, fmt.Errorf("failed to evaluate deny rule: %w", err)
		}

		if len(rsDeny) > 0 && len(rsDeny[0].Expressions) > 0 {
			val := rsDeny[0].Expressions[0].Value

			// Handle sets (multiple deny reasons) or string/bool
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
				decision.Allowed = false
				decision.Reason = strings.Join(denyReasons, ", ")
			}
		}
	} else {
		decision.Reason = "no allow rule matched (default deny)"
	}

	return decision, nil
}
