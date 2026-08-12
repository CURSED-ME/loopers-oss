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
	Name        string       `yaml:"name"`
	Description string       `yaml:"description,omitempty"`
	Match       MatchFilter  `yaml:"match"`
	Conditions  []Condition  `yaml:"conditions,omitempty"`
	SessionFlow *SessionFlow `yaml:"session_flow,omitempty"`
	Action      string       `yaml:"action"` // "allow" | "deny"
	Reason      string       `yaml:"reason,omitempty"`
}

type MatchFilter struct {
	Type string `yaml:"type"` // "llm_call" | "mcp_tool_call"
	Tool string `yaml:"tool,omitempty"`
}

type PolicyCard struct {
	Version  string `yaml:"version"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Rules []Rule `yaml:"rules"`
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
		if action != "allow" && action != "deny" {
			return "", fmt.Errorf("invalid action %q in rule %q; must be 'allow' or 'deny'", rule.Action, rule.Name)
		}

		reason := rule.Reason
		if reason == "" {
			reason = fmt.Sprintf("Blocked by rule: %s", rule.Name)
		}
		// Escape double quotes in reason
		reason = strings.ReplaceAll(reason, "\"", "\\\"")

		if action == "deny" {
			sb.WriteString(fmt.Sprintf("# Rule: %s\ndeny[%q] {\n", rule.Name, reason))
		} else {
			sb.WriteString(fmt.Sprintf("# Rule: %s\nallow {\n", rule.Name))
		}

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
				return "", fmt.Errorf("rule %q: %w", rule.Name, err)
			}

			// Handle different operators
			switch strings.ToLower(cond.Op) {
			case "contains":
				sb.WriteString(fmt.Sprintf("    contains(%s, %q)\n", regoField, cond.Value))
			case "matches_regex":
				sb.WriteString(fmt.Sprintf("    re_match(%q, %s)\n", cond.Value, regoField))
			case "equals":
				sb.WriteString(fmt.Sprintf("    %s == %q\n", regoField, cond.Value))
			case "not_equals":
				sb.WriteString(fmt.Sprintf("    %s != %q\n", regoField, cond.Value))
			default:
				return "", fmt.Errorf("rule %q: unsupported operator %q", rule.Name, cond.Op)
			}
		}

		// Apply SessionFlow Sequence Checks
		if rule.SessionFlow != nil && len(rule.SessionFlow.Requires) > 0 {
			cleanRuleName := strings.ReplaceAll(rule.Name, "-", "_")
			sb.WriteString(fmt.Sprintf("    not satisfied_requires_%s(input.session.tools_called)\n", cleanRuleName))
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

	return "", fmt.Errorf("unsupported field %q", field)
}
