package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestEvaluate_TaintFlagsExposedToOPA verifies that TaintFlags in SessionContext
// are correctly serialised into OPA's input.session.taint_flags and that
// Rego deny rules that reference them work as expected.
func TestEvaluate_TaintFlagsExposedToOPA(t *testing.T) {
	// Create a temporary policy directory with a taint-aware rule
	policyDir := t.TempDir()

	taintPolicy := `
package loopers.policy

default allow = false

allow {
    true
}

deny["outbound HTTP blocked after secret access"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "outbound_http"
    input.session.taint_flags["secret_accessed"]
}
`
	if err := os.WriteFile(filepath.Join(policyDir, "taint.rego"), []byte(taintPolicy), 0600); err != nil {
		t.Fatalf("failed to write taint policy: %v", err)
	}

	engine, err := NewEngine(Config{
		Enabled:       true,
		PolicyDir:     policyDir,
		DefaultAction: "allow",
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	// Case 1: No taint flags -> outbound_http should be ALLOWED
	decision, err := engine.Evaluate(ctx, EvalInput{
		Agent: AgentContext{
			KeyHash: "test-hash",
			Name:    "test-key",
		},
		Request: RequestContext{
			Method:   "mcp_tool_call",
			ToolName: "outbound_http",
			Provider: "test-server",
			Path:     "/mcp/test-server/tools/call",
		},
		Session: SessionContext{
			ID:         "sess-001",
			TaintFlags: map[string]bool{}, // empty — no taint
		},
	})
	if err != nil {
		t.Fatalf("evaluation error (case 1): %v", err)
	}
	if !decision.IsAllowed() {
		t.Errorf("case 1: expected outbound_http to be allowed with no taint, got denied: %s", decision.Reason)
	}

	// Case 2: secret_accessed taint set -> outbound_http should be DENIED
	decision, err = engine.Evaluate(ctx, EvalInput{
		Agent: AgentContext{
			KeyHash: "test-hash",
			Name:    "test-key",
		},
		Request: RequestContext{
			Method:   "mcp_tool_call",
			ToolName: "outbound_http",
			Provider: "test-server",
			Path:     "/mcp/test-server/tools/call",
		},
		Session: SessionContext{
			ID: "sess-001",
			TaintFlags: map[string]bool{
				"secret_accessed": true, // taint is set!
			},
		},
	})
	if err != nil {
		t.Fatalf("evaluation error (case 2): %v", err)
	}
	if decision.IsAllowed() {
		t.Errorf("case 2: expected outbound_http to be DENIED when secret_accessed taint is set, got allowed")
	}
	if decision.Reason == "" {
		t.Errorf("case 2: expected non-empty denial reason")
	}

	// Case 3: taint set but different tool -> should still be allowed
	decision, err = engine.Evaluate(ctx, EvalInput{
		Agent: AgentContext{
			KeyHash: "test-hash",
			Name:    "test-key",
		},
		Request: RequestContext{
			Method:   "mcp_tool_call",
			ToolName: "read_file", // different tool
			Provider: "test-server",
			Path:     "/mcp/test-server/tools/call",
		},
		Session: SessionContext{
			ID: "sess-001",
			TaintFlags: map[string]bool{
				"secret_accessed": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("evaluation error (case 3): %v", err)
	}
	if !decision.IsAllowed() {
		t.Errorf("case 3: expected read_file to be allowed even with taint, got denied: %s", decision.Reason)
	}
}

// TestEvaluate_ToolsCalledExposedToOPA verifies that ToolsCalled in SessionContext
// is correctly exposed to OPA as input.session.tools_called.
func TestEvaluate_ToolsCalledExposedToOPA(t *testing.T) {
	policyDir := t.TempDir()

	historyPolicy := `
package loopers.policy

default allow = false

allow {
    true
}

deny["session has invoked too many distinct tool calls"] {
    count(input.session.tools_called) > 3
    input.request.method == "mcp_tool_call"
}
`
	if err := os.WriteFile(filepath.Join(policyDir, "history.rego"), []byte(historyPolicy), 0600); err != nil {
		t.Fatalf("failed to write history policy: %v", err)
	}

	engine, err := NewEngine(Config{
		Enabled:       true,
		PolicyDir:     policyDir,
		DefaultAction: "allow",
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	baseInput := EvalInput{
		Agent: AgentContext{KeyHash: "test-hash", Name: "test-key"},
		Request: RequestContext{
			Method:   "mcp_tool_call",
			ToolName: "some_tool",
			Provider: "test-server",
			Path:     "/mcp/test-server/tools/call",
		},
	}

	// Case 1: 3 tool calls -> allowed
	baseInput.Session = SessionContext{
		ID:          "sess-002",
		ToolsCalled: []string{"tool1", "tool2", "tool3"},
	}
	d, err := engine.Evaluate(ctx, baseInput)
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}
	if !d.IsAllowed() {
		t.Errorf("case 1: expected allowed with 3 tools_called, got denied: %s", d.Reason)
	}

	// Case 2: 4 tool calls -> denied
	baseInput.Session = SessionContext{
		ID:          "sess-002",
		ToolsCalled: []string{"tool1", "tool2", "tool3", "tool4"},
	}
	d, err = engine.Evaluate(ctx, baseInput)
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}
	if d.IsAllowed() {
		t.Errorf("case 2: expected denied with 4 tools_called, got allowed")
	}
}
