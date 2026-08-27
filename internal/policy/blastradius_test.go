package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPolicy_BlastRadiusEvaluation(t *testing.T) {
	yamlPolicy := `
version: loopers.com/v1alpha1
metadata:
  name: blast-radius-governance
rules:
  - name: escalate-high-blast-radius
    match:
      type: mcp_tool_call
    conditions:
      - field: blast_radius
        op: greater_than
        value: "60"
    action: escalate
    escalate_to: human
    severity: warn
    reason: "Tool blast radius exceeds threshold (>60)"

  - name: deny-critical-blast-radius
    match:
      type: mcp_tool_call
    conditions:
      - field: blast_radius_tier
        op: equals
        value: "critical"
    action: deny
    severity: critical
    reason: "Critical blast radius operations are prohibited"
`
	tmpDir, err := os.MkdirTemp("", "loopers-blastradius-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	policyPath := filepath.Join(tmpDir, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte(yamlPolicy), 0600); err != nil {
		t.Fatalf("Failed to write policy file: %v", err)
	}

	engine, err := NewEngine(Config{
		Enabled:       true,
		PolicyFile:    policyPath,
		DefaultAction: "allow",
	})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	ctx := context.Background()

	// 1. Low risk tool (blast radius = 10, tier = "low") -> should allow
	dec, err := engine.Evaluate(ctx, EvalInput{
		Action: ActionContext{
			Type:            "mcp_tool_call",
			ToolName:        "read_file",
			BlastRadius:     10,
			BlastRadiusTier: "low",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if dec.Action != "allow" {
		t.Errorf("Expected allow for low blast radius, got %s (reason: %s)", dec.Action, dec.Reason)
	}

	// 2. High blast radius tool (blast radius = 70, tier = "high") -> should escalate
	dec, err = engine.Evaluate(ctx, EvalInput{
		Action: ActionContext{
			Type:            "mcp_tool_call",
			ToolName:        "execute_bash",
			BlastRadius:     70,
			BlastRadiusTier: "high",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if dec.Action != "escalate" {
		t.Errorf("Expected escalate for high blast radius, got %s (reason: %s)", dec.Action, dec.Reason)
	}
	if dec.EscalateTo != "human" {
		t.Errorf("Expected EscalateTo 'human', got %s", dec.EscalateTo)
	}

	// 3. Critical blast radius tool (blast radius = 95, tier = "critical") -> should deny
	dec, err = engine.Evaluate(ctx, EvalInput{
		Action: ActionContext{
			Type:            "mcp_tool_call",
			ToolName:        "delete_database",
			BlastRadius:     95,
			BlastRadiusTier: "critical",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if dec.Action != "deny" {
		t.Errorf("Expected deny for critical blast radius, got %s (reason: %s)", dec.Action, dec.Reason)
	}
}

func TestPolicy_MCPSandboxPresetBlastRadius(t *testing.T) {
	engine, err := NewEngine(Config{
		Enabled:       true,
		Presets:       []string{"mcp_sandbox"},
		DefaultAction: "allow",
	})
	if err != nil {
		t.Fatalf("Failed to initialize engine with mcp_sandbox preset: %v", err)
	}

	ctx := context.Background()

	// 1. Safe tool call (blast radius = 0) -> allow
	dec, err := engine.Evaluate(ctx, EvalInput{
		Action: ActionContext{
			Type:            "mcp_tool_call",
			ToolName:        "list_directory",
			BlastRadius:     0,
			BlastRadiusTier: "low",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if dec.Action != "allow" {
		t.Errorf("Expected allow, got %s", dec.Action)
	}

	// 2. High blast radius tool (blast radius = 65) -> escalate
	dec, err = engine.Evaluate(ctx, EvalInput{
		Action: ActionContext{
			Type:            "mcp_tool_call",
			ToolName:        "run_script",
			BlastRadius:     65,
			BlastRadiusTier: "high",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if dec.Action != "escalate" {
		t.Errorf("Expected escalate for blast radius 65, got %s (reason: %s)", dec.Action, dec.Reason)
	}

	// 3. Critical blast radius tool (blast radius = 90) -> deny
	dec, err = engine.Evaluate(ctx, EvalInput{
		Action: ActionContext{
			Type:            "mcp_tool_call",
			ToolName:        "delete_cluster",
			BlastRadius:     90,
			BlastRadiusTier: "critical",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if dec.Action != "deny" {
		t.Errorf("Expected deny for blast radius 90, got %s (reason: %s)", dec.Action, dec.Reason)
	}
}
