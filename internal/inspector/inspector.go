package inspector

import (
	"encoding/json"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// InjectionPatterns covers direct and indirect prompt injection signatures.
// Shared across response inspection and tool list sanitization.
var InjectionPatterns = []string{
	"ignore previous instructions",
	"forget your instructions",
	"you must now",
	"new instructions",
	"override",
	"bypass",
	"print instructions",
	"system prompt",
	"[system:",
	"<instructions>",
	"</instructions>",
	"ignore system directive",
	"you are now",
	"disregard prior",
}

// ZeroWidthChars contains invisible Unicode characters used for obfuscation.
var ZeroWidthChars = []string{
	"\u200B", // zero-width space
	"\u200C", // zero-width non-joiner
	"\u200D", // zero-width joiner
	"\uFEFF", // zero-width no-break space
}

// SecretRegexes matches common credential, token, and private key leakage patterns.
var SecretRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`), // AWS Access Key
	regexp.MustCompile(`sk-[a-zA-Z0-9]{32,}`),                                                   // OpenAI / generic API keys
	regexp.MustCompile(`sk-or-v1-[a-zA-Z0-9]{40,}`),                                             // OpenRouter keys
	regexp.MustCompile(`eyJ[a-zA-Z0-9-_]+\.eyJ[a-zA-Z0-9-_]+\.[a-zA-Z0-9-_]+`),                  // Standard JWT format
	regexp.MustCompile(`-----BEGIN [A-Z ]+PRIVATE KEY-----`),                                    // PEM Private keys
	regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),                                                   // GitHub Personal Access Token
	regexp.MustCompile(`xoxb-[0-9]+-[0-9A-Za-z-]+`),                                             // Slack bot tokens
}

// TraversalPatterns detects file path traversal attempts in content.
var TraversalPatterns = []string{
	"../../",
	"/etc/passwd",
	"/proc/",
	"/sys/",
	"C:\\Windows\\",
	"%2e%2e",
}

// InspectionResult represents the verdict and mutated payload from response inspection.
type InspectionResult struct {
	Action  string // "allow" | "transform" | "quarantine"
	Reason  string
	NewBody []byte
}

// SanitizeString normalizes Unicode and strips zero-width characters.
func SanitizeString(input string) string {
	normalized := norm.NFKC.String(input)
	for _, z := range ZeroWidthChars {
		normalized = strings.ReplaceAll(normalized, z, "")
	}
	return strings.ToLower(normalized)
}

// CheckString inspects an individual string for secrets, injections, and traversal patterns.
// Precedence: quarantine (secrets) > transform (injection/traversal) > allow.
func CheckString(s string, customPatterns []string) (string, string, string) {
	var (
		action   = "allow"
		reason   string
		mutated  = s
		hasZeroW = false
	)

	// 1. Check for Zero-Width Chars
	for _, z := range ZeroWidthChars {
		if strings.Contains(mutated, z) {
			hasZeroW = true
			mutated = strings.ReplaceAll(mutated, z, "")
		}
	}
	if hasZeroW {
		action = "transform"
		reason = "Zero-width characters detected and removed"
	}

	// 2. Check for Prompt Injection / Traversal
	clean := SanitizeString(mutated)
	hasInjection := false

	for _, pat := range InjectionPatterns {
		if strings.Contains(clean, strings.ToLower(pat)) {
			hasInjection = true
			break
		}
	}

	if !hasInjection {
		for _, pat := range customPatterns {
			if strings.Contains(clean, strings.ToLower(pat)) {
				hasInjection = true
				break
			}
		}
	}

	if !hasInjection {
		for _, pat := range TraversalPatterns {
			if strings.Contains(clean, strings.ToLower(pat)) {
				hasInjection = true
				break
			}
		}
	}

	if hasInjection {
		action = "transform"
		reason = "Prompt injection or path traversal pattern detected"
		mutated = "[Content removed: security policy]"
	}

	// 3. Check for Secret Leakage (Highest Precedence)
	for _, re := range SecretRegexes {
		if re.MatchString(s) {
			action = "quarantine"
			reason = "Secret or credential exfiltration detected in tool response"
			mutated = re.ReplaceAllString(mutated, "***")
			break
		}
	}

	return action, reason, mutated
}

// InspectToolResponse synchronously inspects a JSON-RPC tool response body.
// It recursively inspects any string fields in the response payload.
func InspectToolResponse(body []byte, customPatterns []string) InspectionResult {
	if len(body) == 0 {
		return InspectionResult{Action: "allow", NewBody: body}
	}

	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// Non-JSON content: fail-open for transparent pass-through
		return InspectionResult{Action: "allow", NewBody: body}
	}

	var (
		highestAction = "allow"
		primaryReason string
		mutated       bool
	)

	updateVerdict := func(action, reason string) {
		if action == "quarantine" {
			highestAction = "quarantine"
			primaryReason = reason
		} else if action == "transform" && highestAction != "quarantine" {
			highestAction = "transform"
			primaryReason = reason
		}
	}

	var walkNode func(node interface{}) interface{}
	walkNode = func(node interface{}) interface{} {
		switch v := node.(type) {
		case string:
			action, reason, newStr := CheckString(v, customPatterns)
			if action != "allow" {
				updateVerdict(action, reason)
			}
			if newStr != v {
				mutated = true
				return newStr
			}
			return v

		case map[string]interface{}:
			newMap := make(map[string]interface{}, len(v))
			for k, val := range v {
				newMap[k] = walkNode(val)
			}
			return newMap

		case []interface{}:
			newSlice := make([]interface{}, len(v))
			for i, val := range v {
				newSlice[i] = walkNode(val)
			}
			return newSlice

		default:
			return node
		}
	}

	transformedData := walkNode(data)

	if highestAction == "allow" {
		return InspectionResult{
			Action:  "allow",
			NewBody: body,
		}
	}

	newBody := body
	if mutated {
		if encoded, err := json.Marshal(transformedData); err == nil {
			newBody = encoded
		}
	}

	return InspectionResult{
		Action:  highestAction,
		Reason:  primaryReason,
		NewBody: newBody,
	}
}
