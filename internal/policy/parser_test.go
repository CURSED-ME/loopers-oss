package policy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseYAML_Valid(t *testing.T) {
	yamlData := []byte(`
version: loopers.com/v1alpha1
metadata:
  name: test-policy
rules:
  - name: block-destructive
    match:
      type: mcp_tool_call
      tool: execute_bash
    conditions:
      - field: arguments.command
        op: contains
        value: "rm -rf"
    action: deny
    reason: "destructive commands forbidden"
`)

	card, err := ParseYAML(yamlData)
	assert.NoError(t, err)
	assert.Equal(t, "loopers.com/v1alpha1", card.Version)
	assert.Equal(t, "test-policy", card.Metadata.Name)
	assert.Len(t, card.Rules, 1)

	rule := card.Rules[0]
	assert.Equal(t, "block-destructive", rule.Name)
	assert.Equal(t, "mcp_tool_call", rule.Match.Type)
	assert.Equal(t, "execute_bash", rule.Match.Tool)
	assert.Len(t, rule.Conditions, 1)
	assert.Equal(t, "arguments.command", rule.Conditions[0].Field)
	assert.Equal(t, "contains", rule.Conditions[0].Op)
	assert.Equal(t, "rm -rf", rule.Conditions[0].Value)
	assert.Equal(t, "deny", rule.Action)
	assert.Equal(t, "destructive commands forbidden", rule.Reason)
}

func TestTranspileToRego(t *testing.T) {
	card := &PolicyCard{
		Version: "loopers.com/v1alpha1",
		Rules: []Rule{
			{
				Name: "block-destructive",
				Match: MatchFilter{
					Type: "mcp_tool_call",
					Tool: "execute_bash",
				},
				Conditions: []Condition{
					{
						Field: "arguments.command",
						Op:    "contains",
						Value: "rm -rf",
					},
					{
						Field: "session.taint_flags.secret_accessed",
						Op:    "equals",
						Value: "true",
					},
				},
				Action: "deny",
				Reason: "Destructive execution denied",
			},
			{
				Name: "allow-safe-mcp",
				Match: MatchFilter{
					Type: "mcp_tool_call",
					Tool: "read_file",
				},
				Action: "allow",
			},
		},
	}

	regoCode, err := TranspileToRego(card)
	assert.NoError(t, err)

	assert.Contains(t, regoCode, "package loopers.policy")
	assert.Contains(t, regoCode, "deny[\"Destructive execution denied\"] {")
	assert.Contains(t, regoCode, "input.action.type == \"mcp_tool_call\"")
	assert.Contains(t, regoCode, "input.action.tool_name == \"execute_bash\"")
	assert.Contains(t, regoCode, "contains(input.action.tool_arguments.command, \"rm -rf\")")
	assert.Contains(t, regoCode, "input.session.taint_flags.secret_accessed == true")
	assert.Contains(t, regoCode, "allow {")
	assert.Contains(t, regoCode, "input.action.tool_name == \"read_file\"")
}

func TestTranspileToRego_MatchesRegex(t *testing.T) {
	card := &PolicyCard{
		Version: "loopers.com/v1alpha1",
		Rules: []Rule{
			{
				Name: "regex-block",
				Match: MatchFilter{
					Type: "llm_call",
				},
				Conditions: []Condition{
					{
						Field: "prompt_text",
						Op:    "matches_regex",
						Value: `\b\d{3}-\d{2}-\d{4}\b`,
					},
				},
				Action: "deny",
				Reason: "SSN leak blocked",
			},
		},
	}

	regoCode, err := TranspileToRego(card)
	assert.NoError(t, err)

	// Rego transpiler must escape backslashes
	expectedRegex := `\\b\\d{3}-\\d{2}-\\d{4}\\b`
	assert.True(t, strings.Contains(regoCode, expectedRegex), "Rego regex string did not escape backslashes correctly. Got: %s", regoCode)
}

func TestTranspileToRego_SessionFlow(t *testing.T) {
	card := &PolicyCard{
		Version: "loopers.com/v1alpha1",
		Rules: []Rule{
			{
				Name: "enforce-command-validation",
				Match: MatchFilter{
					Type: "mcp_tool_call",
					Tool: "execute_bash",
				},
				SessionFlow: &SessionFlow{
					Requires: []string{"dry_run_command"},
					Steps:    2,
				},
				Action: "deny",
				Reason: "Bash execution requested without dry-run",
			},
		},
	}

	regoCode, err := TranspileToRego(card)
	assert.NoError(t, err)

	assert.Contains(t, regoCode, "not satisfied_requires_enforce_command_validation(input.session.tools_called)")
	assert.Contains(t, regoCode, "satisfied_requires_enforce_command_validation(tools) {")
	assert.Contains(t, regoCode, "tools[i] == \"dry_run_command\"")
	assert.Contains(t, regoCode, "i < 2")
}

func TestTranspileToRego_NewActions(t *testing.T) {
	card := &PolicyCard{
		Version: "loopers.com/v1alpha1",
		Rules: []Rule{
			{
				Name: "escalate-high-risk",
				Match: MatchFilter{
					Type: "mcp_tool_call",
					Tool: "drop_db",
				},
				Action:     "escalate",
				EscalateTo: "human",
				Severity:   "critical",
				Reason:     "Destructive DB drop needs approval",
			},
			{
				Name: "quarantine-threat",
				Match: MatchFilter{
					Type: "mcp_tool_call",
					Tool: "exfiltrate",
				},
				Action:        "quarantine",
				QuarantineFor: "30m",
				Severity:      "critical",
				Reason:        "Malicious exfiltration detected",
			},
			{
				Name: "transform-password",
				Match: MatchFilter{
					Type: "mcp_tool_call",
					Tool: "auth_tool",
				},
				Action:   "transform",
				Severity: "info",
				Transforms: []TransformRule{
					{Field: "password", Operation: "mask"},
				},
			},
		},
	}

	regoCode, err := TranspileToRego(card)
	assert.NoError(t, err)

	assert.Contains(t, regoCode, `"action": "escalate"`)
	assert.Contains(t, regoCode, `"escalate_to": "human"`)
	assert.Contains(t, regoCode, `"action": "quarantine"`)
	assert.Contains(t, regoCode, `"quarantine_for": "30m"`)
	assert.Contains(t, regoCode, `"action": "transform"`)
	assert.Contains(t, regoCode, `"field": "password"`)
	assert.Contains(t, regoCode, `"operation": "mask"`)
}

func TestTranspileToRego_DriftConditions(t *testing.T) {
	yamlData := []byte(`
version: loopers.com/v1alpha1
metadata:
  name: drift-policy
rules:
  - name: block-context-drift
    match:
      type: llm_call
    conditions:
      - field: session.drift.drift_detected
        op: equals
        value: "true"
      - field: session.drift.drift_score
        op: greater_than
        value: "0.75"
      - field: session.drift.anchor_similarity
        op: less_than
        value: "0.20"
    action: escalate
    reason: "Severe conversational drift detected"
`)

	card, err := ParseYAML(yamlData)
	assert.NoError(t, err)

	regoCode, err := TranspileToRego(card)
	assert.NoError(t, err)

	assert.Contains(t, regoCode, `input.session.drift.drift_detected == true`)
	assert.Contains(t, regoCode, `input.session.drift.drift_score > 0.75`)
	assert.Contains(t, regoCode, `input.session.drift.anchor_similarity < 0.20`)
	assert.Contains(t, regoCode, `"action": "escalate"`)
	assert.Contains(t, regoCode, `Severe conversational drift detected`)
}
