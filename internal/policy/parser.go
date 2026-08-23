package policy

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Condition struct {
	Field string `yaml:"field"`
	Op    string `yaml:"op"` // "contains" | "matches_regex" | "equals" | "not_equals"
	Value string `yaml:"value"`
}

type SessionFlow struct {
	Requires []string `yaml:"requires"`
	Steps    int      `yaml:"within_last_steps"`
}

type Rule struct {
	Name          string          `yaml:"name"`
	Description   string          `yaml:"description,omitempty"`
	Match         MatchFilter     `yaml:"match"`
	Conditions    []Condition     `yaml:"conditions,omitempty"`
	SessionFlow   *SessionFlow    `yaml:"session_flow,omitempty"`
	Action        string          `yaml:"action"` // "allow" | "deny" | "escalate" | "quarantine" | "transform"
	Reason        string          `yaml:"reason,omitempty"`
	Severity      string          `yaml:"severity,omitempty"`       // "info" | "warn" | "critical"
	Transforms    []TransformRule `yaml:"transforms,omitempty"`     // Used when action == "transform"
	EscalateTo    string          `yaml:"escalate_to,omitempty"`    // Used when action == "escalate"
	QuarantineFor string          `yaml:"quarantine_for,omitempty"` // Used when action == "quarantine"
}

type MatchFilter struct {
	Type string `yaml:"type"` // "llm_call" | "mcp_tool_call"
	Tool string `yaml:"tool,omitempty"`
}

type FSMTransition struct {
	From    string `yaml:"from"`
	To      string `yaml:"to"`
	Trigger string `yaml:"trigger"`
}

type FSMConfig struct {
	InitialState string          `yaml:"initial_state"`
	Transitions  []FSMTransition `yaml:"transitions"`
}

type PolicyCard struct {
	Version  string `yaml:"version"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	FSM   *FSMConfig `yaml:"fsm,omitempty"`
	Rules []Rule     `yaml:"rules"`
}

func ParseYAML(data []byte) (*PolicyCard, error) {
	var card PolicyCard
	if err := yaml.Unmarshal(data, &card); err != nil {
		return nil, fmt.Errorf("failed to parse YAML policy: %w", err)
	}
	return &card, nil
}

func TranspileToRego(card *PolicyCard) (string, error) {
	var sb strings.Builder
	sb.WriteString("package loopers.policy\n\n")

	for _, rule := range card.Rules {
		action := strings.ToLower(rule.Action)
		validActions := map[string]bool{
			"allow":      true,
			"deny":       true,
			"escalate":   true,
			"quarantine": true,
			"transform":  true,
		}
		if !validActions[action] {
			return "", fmt.Errorf("invalid action %q in rule %q; must be one of: 'allow', 'deny', 'escalate', 'quarantine', 'transform'", rule.Action, rule.Name)
		}

		reason := rule.Reason
		if reason == "" {
			switch action {
			case "deny":
				reason = fmt.Sprintf("Blocked by rule: %s", rule.Name)
			case "escalate":
				reason = fmt.Sprintf("Escalation required by rule: %s", rule.Name)
			case "quarantine":
				reason = fmt.Sprintf("Quarantine triggered by rule: %s", rule.Name)
			case "transform":
				reason = fmt.Sprintf("Transform applied by rule: %s", rule.Name)
			default:
				reason = fmt.Sprintf("Matched rule: %s", rule.Name)
			}
		}
		// Escape double quotes in strings
		escapedReason := strings.ReplaceAll(reason, "\"", "\\\"")
		escapedName := strings.ReplaceAll(rule.Name, "\"", "\\\"")

		severity := rule.Severity
		if severity == "" {
			if action == "deny" || action == "quarantine" {
				severity = "critical"
			} else if action == "escalate" {
				severity = "warn"
			} else {
				severity = "info"
			}
		}
		escapedSeverity := strings.ReplaceAll(severity, "\"", "\\\"")

		// 1. For backward compatibility with legacy allow/deny queries
		if action == "deny" {
			sb.WriteString(fmt.Sprintf("# Rule (legacy deny): %s\ndeny[%q] {\n", rule.Name, escapedReason))
			writeRuleConditions(&sb, rule)
			sb.WriteString("}\n\n")
		} else if action == "allow" {
			sb.WriteString(fmt.Sprintf("# Rule (legacy allow): %s\nallow {\n", rule.Name))
			writeRuleConditions(&sb, rule)
			sb.WriteString("}\n\n")
		}

		// 2. Structured decision output in data.loopers.policy.decisions
		sb.WriteString(fmt.Sprintf("# Rule (structured decision): %s\ndecisions[decision] {\n", rule.Name))
		writeRuleConditions(&sb, rule)

		switch action {
		case "escalate":
			escalateTo := rule.EscalateTo
			if escalateTo == "" {
				escalateTo = "human"
			}
			escapedEscalateTo := strings.ReplaceAll(escalateTo, "\"", "\\\"")
			sb.WriteString(fmt.Sprintf("    decision := {\n        \"action\": %q,\n        \"reason\": %q,\n        \"severity\": %q,\n        \"escalate_to\": %q,\n        \"evidence\": [%q]\n    }\n", action, escapedReason, escapedSeverity, escapedEscalateTo, escapedName))

		case "quarantine":
			quarantineFor := rule.QuarantineFor
			if quarantineFor == "" {
				quarantineFor = "1h"
			}
			escapedQuarantineFor := strings.ReplaceAll(quarantineFor, "\"", "\\\"")
			sb.WriteString(fmt.Sprintf("    decision := {\n        \"action\": %q,\n        \"reason\": %q,\n        \"severity\": %q,\n        \"quarantine_for\": %q,\n        \"evidence\": [%q]\n    }\n", action, escapedReason, escapedSeverity, escapedQuarantineFor, escapedName))

		case "transform":
			sb.WriteString("    transforms := [\n")
			for _, tr := range rule.Transforms {
				sb.WriteString(fmt.Sprintf("        {\"field\": %q, \"operation\": %q},\n", strings.ReplaceAll(tr.Field, "\"", "\\\""), strings.ReplaceAll(tr.Operation, "\"", "\\\"")))
			}
			sb.WriteString("    ]\n")
			sb.WriteString(fmt.Sprintf("    decision := {\n        \"action\": %q,\n        \"reason\": %q,\n        \"severity\": %q,\n        \"transforms\": transforms,\n        \"evidence\": [%q]\n    }\n", action, escapedReason, escapedSeverity, escapedName))

		default: // "allow" or "deny"
			sb.WriteString(fmt.Sprintf("    decision := {\n        \"action\": %q,\n        \"reason\": %q,\n        \"severity\": %q,\n        \"evidence\": [%q]\n    }\n", action, escapedReason, escapedSeverity, escapedName))
		}

		sb.WriteString("}\n\n")
	}

	// Generate SessionFlow helper functions
	for _, rule := range card.Rules {
		if rule.SessionFlow != nil && len(rule.SessionFlow.Requires) > 0 {
			cleanRuleName := strings.ReplaceAll(rule.Name, "-", "_")
			steps := rule.SessionFlow.Steps
			if steps <= 0 {
				steps = 50 // Default to max history size
			}
			for _, reqTool := range rule.SessionFlow.Requires {
				sb.WriteString(fmt.Sprintf("satisfied_requires_%s(tools) {\n", cleanRuleName))
				sb.WriteString("    some i\n")
				sb.WriteString(fmt.Sprintf("    tools[i] == %q\n", reqTool))
				sb.WriteString(fmt.Sprintf("    i < %d\n", steps))
				sb.WriteString("}\n\n")
			}
		}
	}

	return sb.String(), nil
}

func writeRuleConditions(sb *strings.Builder, rule Rule) {
	// Apply Match Filters
	if rule.Match.Type != "" {
		sb.WriteString(fmt.Sprintf("    input.action.type == %q\n", rule.Match.Type))
	}
	if rule.Match.Tool != "" {
		sb.WriteString(fmt.Sprintf("    input.action.tool_name == %q\n", rule.Match.Tool))
	}

	// Apply Conditions
	for _, cond := range rule.Conditions {
		regoField, err := mapFieldToRego(cond.Field)
		if err != nil {
			continue
		}

		// Handle different operators
		switch strings.ToLower(cond.Op) {
		case "contains":
			sb.WriteString(fmt.Sprintf("    contains(%s, %q)\n", regoField, cond.Value))
		case "matches_regex":
			sb.WriteString(fmt.Sprintf("    re_match(%q, %s)\n", cond.Value, regoField))
		case "equals":
			if cond.Value == "true" || cond.Value == "false" {
				sb.WriteString(fmt.Sprintf("    %s == %s\n", regoField, cond.Value))
			} else {
				sb.WriteString(fmt.Sprintf("    %s == %q\n", regoField, cond.Value))
			}
		case "not_equals":
			if cond.Value == "true" || cond.Value == "false" {
				sb.WriteString(fmt.Sprintf("    %s != %s\n", regoField, cond.Value))
			} else {
				sb.WriteString(fmt.Sprintf("    %s != %q\n", regoField, cond.Value))
			}
		case "greater_than", ">":
			sb.WriteString(fmt.Sprintf("    %s > %s\n", regoField, cond.Value))
		case "greater_than_or_equals", ">=":
			sb.WriteString(fmt.Sprintf("    %s >= %s\n", regoField, cond.Value))
		case "less_than", "<":
			sb.WriteString(fmt.Sprintf("    %s < %s\n", regoField, cond.Value))
		case "less_than_or_equals", "<=":
			sb.WriteString(fmt.Sprintf("    %s <= %s\n", regoField, cond.Value))
		case "contains_element":
			sb.WriteString(fmt.Sprintf("    %s[_] == %q\n", regoField, cond.Value))
		}
	}

	// Apply SessionFlow Sequence Checks
	if rule.SessionFlow != nil && len(rule.SessionFlow.Requires) > 0 {
		cleanRuleName := strings.ReplaceAll(rule.Name, "-", "_")
		sb.WriteString(fmt.Sprintf("    not satisfied_requires_%s(input.session.tools_called)\n", cleanRuleName))
	}
}

func mapFieldToRego(field string) (string, error) {
	if field == "" {
		return "", fmt.Errorf("field cannot be empty")
	}

	if field == "prompt_text" {
		return "input.action.prompt_text", nil
	}
	if field == "model" {
		return "input.action.model", nil
	}
	if field == "provider" {
		return "input.action.provider", nil
	}
	if strings.HasPrefix(field, "arguments.") {
		argName := field[len("arguments."):]
		if argName == "" {
			return "", fmt.Errorf("invalid arguments field pattern")
		}
		return fmt.Sprintf("input.action.tool_arguments.%s", argName), nil
	}
	if field == "session.state" {
		return "input.session.state", nil
	}
	if strings.HasPrefix(field, "session.taint_flags.") {
		flagName := field[len("session.taint_flags."):]
		if flagName == "" {
			return "", fmt.Errorf("invalid session.taint_flags field pattern")
		}
		return fmt.Sprintf("input.session.taint_flags.%s", flagName), nil
	}
	if field == "session.tools_called_count" {
		return "count(input.session.tools_called)", nil
	}
	if strings.HasPrefix(field, "session.drift.") {
		subField := field[len("session.drift."):]
		if subField == "" {
			return "", fmt.Errorf("invalid session.drift field pattern")
		}
		return fmt.Sprintf("input.session.drift.%s", subField), nil
	}
	if strings.HasPrefix(field, "agent_risk.") {
		subField := field[len("agent_risk."):]
		if subField == "" {
			return "", fmt.Errorf("invalid agent_risk field pattern")
		}
		return fmt.Sprintf("input.agent_risk.%s", subField), nil
	}

	return "", fmt.Errorf("unsupported field %q", field)
}
