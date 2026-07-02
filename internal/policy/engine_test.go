package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEngine_Evaluate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loopers-policy-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	policyFile := filepath.Join(tempDir, "test.rego")
	policyContent := `package loopers.policy

default allow = false
default deny = false

allow {
	input.agent.owner == "admin"
}

deny {
	input.request.model == "gpt-4-expensive"
	input.agent.owner != "admin"
}
`
	if err := os.WriteFile(policyFile, []byte(policyContent), 0644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	cfg := Config{
		Enabled:       true,
		PolicyDir:     tempDir,
		DefaultAction: "deny",
	}

	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	tests := []struct {
		name          string
		input         EvalInput
		expectAllowed bool
		expectReason  string
	}{
		{
			name: "allow admin",
			input: EvalInput{
				Agent:   AgentContext{Owner: "admin"},
				Request: RequestContext{Model: "gpt-3.5"},
			},
			expectAllowed: true,
		},
		{
			name: "deny non-admin default",
			input: EvalInput{
				Agent:   AgentContext{Owner: "intern"},
				Request: RequestContext{Model: "gpt-3.5"},
			},
			expectAllowed: false,
		},
		{
			name: "deny specific model",
			input: EvalInput{
				Agent:   AgentContext{Owner: "intern"},
				Request: RequestContext{Model: "gpt-4-expensive"},
			},
			expectAllowed: false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := engine.Evaluate(ctx, tt.input)
			if err != nil {
				t.Fatalf("evaluation failed: %v", err)
			}
			if decision.Allowed != tt.expectAllowed {
				t.Errorf("expected allowed=%v, got %v (reason: %s)", tt.expectAllowed, decision.Allowed, decision.Reason)
			}
		})
	}
}

func TestEngine_EmptyDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loopers-policy-empty")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := Config{
		Enabled:       true,
		PolicyDir:     tempDir,
		DefaultAction: "deny",
	}

	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	decision, err := engine.Evaluate(context.Background(), EvalInput{})
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	if decision.Allowed {
		t.Errorf("expected default deny with empty policies")
	}
}
