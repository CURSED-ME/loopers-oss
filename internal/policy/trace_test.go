package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluate_TracesExposedToOPA(t *testing.T) {
	policyDir := t.TempDir()

	tracePolicy := `
package loopers.policy

default allow = false

allow {
    true
}

deny["write blocked because database query returned credentials earlier"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "write_file"
    
    # Iterate through traces to see if any mcp_tool_response from database returned credentials
    trace := input.session.traces[_]
    trace.type == "mcp_tool_response"
    trace.provider == "database"
    contains(trace.content, "credentials")
}
`
	if err := os.WriteFile(filepath.Join(policyDir, "trace.rego"), []byte(tracePolicy), 0600); err != nil {
		t.Fatalf("failed to write trace policy: %v", err)
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

	// Case 1: Traces exist but no database output containing "credentials" -> write_file should be ALLOWED
	decision, err := engine.Evaluate(ctx, EvalInput{
		Agent: AgentContext{
			KeyHash: "test-hash",
			Name:    "test-key",
		},
		Request: RequestContext{
			Method:   "mcp_tool_call",
			ToolName: "write_file",
			Provider: "filesystem",
		},
		Session: SessionContext{
			ID: "sess-001",
			Traces: []SessionTrace{
				{
					Timestamp: 1234567,
					Type:      "mcp_tool_call",
					Provider:  "database",
					ToolName:  "query",
				},
				{
					Timestamp: 1234568,
					Type:      "mcp_tool_response",
					Provider:  "database",
					ToolName:  "query",
					Content:   "returned normal row data",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("evaluation error (case 1): %v", err)
	}
	if !decision.Allowed {
		t.Errorf("case 1: expected write_file to be allowed, got denied: %s", decision.Reason)
	}

	// Case 2: Database response trace contains "credentials" -> write_file should be DENIED
	decision, err = engine.Evaluate(ctx, EvalInput{
		Agent: AgentContext{
			KeyHash: "test-hash",
			Name:    "test-key",
		},
		Request: RequestContext{
			Method:   "mcp_tool_call",
			ToolName: "write_file",
			Provider: "filesystem",
		},
		Session: SessionContext{
			ID: "sess-001",
			Traces: []SessionTrace{
				{
					Timestamp: 1234567,
					Type:      "mcp_tool_call",
					Provider:  "database",
					ToolName:  "query",
				},
				{
					Timestamp: 1234568,
					Type:      "mcp_tool_response",
					Provider:  "database",
					ToolName:  "query",
					Content:   "returned column credentials", // trigger content containing "credentials"
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("evaluation error (case 2): %v", err)
	}
	if decision.Allowed {
		t.Errorf("case 2: expected write_file to be DENIED when database trace returns credentials, got allowed")
	}
	if decision.Reason == "" {
		t.Errorf("case 2: expected non-empty denial reason")
	}
}
