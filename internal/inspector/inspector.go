package inspector

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/try-loopers/loopers-oss/internal/syntactic"
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

// PIIRegexes matches common PII patterns in completion text.
var (
	EmailRegex      = regexp.MustCompile(`(?i)\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`)
	VisaRegex       = regexp.MustCompile(`\b4[0-9]{12}(?:[0-9]{3})?\b`)
	MasterCardRegex = regexp.MustCompile(`\b(?:5[1-5][0-9]{14}|2(?:2[2-9][1-9]|[3-6][0-9]{2}|7(?:[01][0-9]|20))[0-9]{12})\b`)
	AmexRegex       = regexp.MustCompile(`\b3[47][0-9]{13}\b`)
	DiscoverRegex   = regexp.MustCompile(`\b6(?:011|5[0-9]{2})[0-9]{12}\b`)
	SSNRegex        = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	PhoneRegex      = regexp.MustCompile(`(?:(?:\+1|\b1)[-.\s]?)?(?:\([0-9]{3}\)|\b[0-9]{3})[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b`)
)

// InternalNetworkPatterns matches private infrastructure indicators.
var InternalNetworkPatterns = []string{
	"10.", "172.16.", "172.17.", "172.18.", "172.19.", "172.20.",
	"172.21.", "172.22.", "172.23.", "172.24.", "172.25.", "172.26.",
	"172.27.", "172.28.", "172.29.", "172.30.", "172.31.",
	"192.168.", "127.0.0.1", "localhost",
	".internal", ".local", ".corp",
}

// IsValidLuhn validates credit card numbers using the standard Luhn checksum algorithm.
func IsValidLuhn(number string) bool {
	var digits []int
	for _, ch := range number {
		if ch >= '0' && ch <= '9' {
			digits = append(digits, int(ch-'0'))
		}
	}
	if len(digits) < 11 || len(digits) > 19 {
		return false
	}

	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		val := digits[i]
		if alt {
			val *= 2
			if val > 9 {
				val -= 9
			}
		}
		sum += val
		alt = !alt
	}
	return sum%10 == 0
}

// DLPConfig controls the outbound LLM completion DLP gate.
// Loaded by server.go via viper.UnmarshalKey("server.dlp", &cfg).
type DLPConfig struct {
	Enabled            bool     `mapstructure:"enabled"`
	Action             string   `mapstructure:"action"` // "mask" | "quarantine"
	ScanSecrets        bool     `mapstructure:"scan_secrets"`
	ScanPII            bool     `mapstructure:"scan_pii"`
	ScanNetwork        bool     `mapstructure:"scan_network"`
	AllowedHosts       []string `mapstructure:"allowed_hosts"` // e.g. ["localhost", "example.com"]
	QuarantineDuration string   `mapstructure:"quarantine_duration"`
}

// DLPResult represents the verdict and detected matches from outbound DLP scanning.
type DLPResult struct {
	Action  string   `json:"action"` // "allow" | "mask" | "quarantine"
	Reason  string   `json:"reason,omitempty"`
	Matches []string `json:"matches,omitempty"`
}

// InspectionResult represents the verdict and mutated payload from response inspection.
type InspectionResult struct {
	Action  string // "allow" | "transform" | "quarantine"
	Reason  string
	NewBody []byte
}

// SanitizeString normalizes Unicode, homoglyphs, encodings, and strips invisible characters.
func SanitizeString(input string) string {
	return syntactic.Normalize(input)
}

// InspectDLPContent scans a plain text string for secrets, PII, and internal network indicators.
// Returns the DLPResult and the sanitized/mutated text string.
func InspectDLPContent(text string, cfg DLPConfig) (DLPResult, string) {
	if !cfg.Enabled || len(text) == 0 {
		return DLPResult{Action: "allow"}, text
	}

	mutated := text
	var matches []string
	var reasons []string
	hasQuarantine := false
	hasMask := false

	isAllowedHost := func(hostOrEmail string) bool {
		lower := strings.ToLower(hostOrEmail)
		for _, allowed := range cfg.AllowedHosts {
			if allowed != "" && (strings.Contains(lower, strings.ToLower(allowed)) || strings.HasSuffix(lower, "@"+strings.ToLower(allowed))) {
				return true
			}
		}
		return false
	}

	// 1. Scan Secrets
	if cfg.ScanSecrets {
		for _, re := range SecretRegexes {
			found := re.FindAllString(mutated, -1)
			if len(found) > 0 {
				matches = append(matches, found...)
				reasons = append(reasons, "Secret or API key detected")
				if cfg.Action == "quarantine" {
					hasQuarantine = true
				} else {
					hasMask = true
				}
				mutated = re.ReplaceAllString(mutated, "***")
			}
		}
	}

	// 2. Scan PII
	if cfg.ScanPII {
		// Email
		emailMatches := EmailRegex.FindAllString(mutated, -1)
		emailFound := false
		for _, email := range emailMatches {
			if isAllowedHost(email) {
				continue
			}
			matches = append(matches, email)
			hasMask = true
			emailFound = true
			mutated = strings.ReplaceAll(mutated, email, "***")
		}
		if emailFound {
			reasons = append(reasons, "Email address detected")
		}

		// Credit Cards with Luhn validation
		ccRegexes := []*regexp.Regexp{VisaRegex, MasterCardRegex, AmexRegex, DiscoverRegex}
		ccFound := false
		for _, re := range ccRegexes {
			ccMatches := re.FindAllString(mutated, -1)
			for _, cc := range ccMatches {
				if IsValidLuhn(cc) {
					matches = append(matches, cc)
					hasMask = true
					ccFound = true
					mutated = strings.ReplaceAll(mutated, cc, "***")
				}
			}
		}
		if ccFound {
			reasons = append(reasons, "Credit card number detected")
		}

		// US SSN
		ssnMatches := SSNRegex.FindAllString(mutated, -1)
		if len(ssnMatches) > 0 {
			matches = append(matches, ssnMatches...)
			hasMask = true
			reasons = append(reasons, "Social security number detected")
			mutated = SSNRegex.ReplaceAllString(mutated, "***")
		}

		// Phone numbers
		phoneMatches := PhoneRegex.FindAllString(mutated, -1)
		if len(phoneMatches) > 0 {
			matches = append(matches, phoneMatches...)
			hasMask = true
			reasons = append(reasons, "Phone number detected")
			mutated = PhoneRegex.ReplaceAllString(mutated, "***")
		}
	}

	// 3. Scan Internal Network
	if cfg.ScanNetwork {
		for _, pat := range InternalNetworkPatterns {
			if strings.Contains(mutated, pat) {
				if isAllowedHost(pat) {
					continue
				}
				matches = append(matches, pat)
				hasMask = true
				reasons = append(reasons, "Internal network indicator detected")
				mutated = strings.ReplaceAll(mutated, pat, "***")
			}
		}
	}

	if hasQuarantine {
		return DLPResult{
			Action:  "quarantine",
			Reason:  strings.Join(reasons, ", "),
			Matches: matches,
		}, mutated
	}

	if hasMask {
		return DLPResult{
			Action:  "mask",
			Reason:  strings.Join(reasons, ", "),
			Matches: matches,
		}, mutated
	}

	return DLPResult{Action: "allow"}, text
}

// InspectJSONCompletion parses provider-specific LLM response JSON,
// extracts text fields, runs DLP, and re-marshals the mutated envelope.
// Supported providers: "openai", "anthropic", "gemini", and generic OpenAI-compatible.
func InspectJSONCompletion(body []byte, provider string, cfg DLPConfig) (DLPResult, []byte, error) {
	if !cfg.Enabled || len(body) == 0 {
		return DLPResult{Action: "allow"}, body, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// Non-JSON fallback: inspect plain text directly
		res, mutated := InspectDLPContent(string(body), cfg)
		return res, []byte(mutated), nil
	}

	var (
		overallAction = "allow"
		allMatches    []string
		allReasons    []string
		modified      = false
	)

	applyDLPToText := func(input string) string {
		if input == "" {
			return input
		}
		res, mutated := InspectDLPContent(input, cfg)
		if res.Action == "quarantine" {
			overallAction = "quarantine"
		} else if res.Action == "mask" && overallAction != "quarantine" {
			overallAction = "mask"
		}
		if len(res.Matches) > 0 {
			allMatches = append(allMatches, res.Matches...)
		}
		if res.Reason != "" {
			allReasons = append(allReasons, res.Reason)
		}
		if mutated != input {
			modified = true
		}
		return mutated
	}

	providerLower := strings.ToLower(provider)
	switch providerLower {
	case "anthropic":
		if content, ok := data["content"].([]interface{}); ok {
			for i, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					if textVal, ok := partMap["text"].(string); ok {
						partMap["text"] = applyDLPToText(textVal)
						content[i] = partMap
					}
				}
			}
			data["content"] = content
		}

	case "gemini":
		if candidates, ok := data["candidates"].([]interface{}); ok {
			for i, cand := range candidates {
				if candMap, ok := cand.(map[string]interface{}); ok {
					if content, ok := candMap["content"].(map[string]interface{}); ok {
						if parts, ok := content["parts"].([]interface{}); ok {
							for j, p := range parts {
								if partMap, ok := p.(map[string]interface{}); ok {
									if textVal, ok := partMap["text"].(string); ok {
										partMap["text"] = applyDLPToText(textVal)
										parts[j] = partMap
									}
								}
							}
							content["parts"] = parts
							candMap["content"] = content
							candidates[i] = candMap
						}
					}
				}
			}
			data["candidates"] = candidates
		}

	default:
		// OpenAI / OpenAI-compatible
		if choices, ok := data["choices"].([]interface{}); ok {
			for i, choice := range choices {
				if choiceMap, ok := choice.(map[string]interface{}); ok {
					// Chat completion message
					if msg, ok := choiceMap["message"].(map[string]interface{}); ok {
						if content, ok := msg["content"].(string); ok {
							msg["content"] = applyDLPToText(content)
						}
						// Also check tool calls arguments if present
						if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
							for k, tc := range toolCalls {
								if tcMap, ok := tc.(map[string]interface{}); ok {
									if fn, ok := tcMap["function"].(map[string]interface{}); ok {
										if argsStr, ok := fn["arguments"].(string); ok {
											fn["arguments"] = applyDLPToText(argsStr)
											tcMap["function"] = fn
											toolCalls[k] = tcMap
										}
									}
								}
							}
							msg["tool_calls"] = toolCalls
						}
						choiceMap["message"] = msg
					}
					// Delta format (if stream chunk is passed through here)
					if delta, ok := choiceMap["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok {
							delta["content"] = applyDLPToText(content)
							choiceMap["delta"] = delta
						}
					}
					// Legacy text format
					if textVal, ok := choiceMap["text"].(string); ok {
						choiceMap["text"] = applyDLPToText(textVal)
					}
					choices[i] = choiceMap
				}
			}
			data["choices"] = choices
		}
	}

	if overallAction == "allow" && !modified {
		return DLPResult{Action: "allow"}, body, nil
	}

	var newBody = body
	if modified {
		marshaled, err := json.Marshal(data)
		if err != nil {
			return DLPResult{
				Action:  overallAction,
				Reason:  "failed to serialize redacted JSON: " + err.Error(),
				Matches: allMatches,
			}, nil, fmt.Errorf("failed to marshal redacted completion JSON: %w", err)
		}
		newBody = marshaled
	}

	return DLPResult{
		Action:  overallAction,
		Reason:  strings.Join(allReasons, ", "),
		Matches: allMatches,
	}, newBody, nil
}

// CheckString inspects an individual string for secrets, injections, and traversal patterns.
// Precedence: quarantine (secrets) > transform (injection/traversal/obfuscation) > allow.
func CheckString(s string, customPatterns []string) (string, string, string) {
	var (
		action  = "allow"
		reason  string
		mutated = s
	)

	// Extract all candidate layers (raw, normalized, decoded base64, unescaped)
	layers := syntactic.ExtractAllTextLayers(s)
	obf := syntactic.AnalyzeObfuscation(s)

	// 1. Check for Zero-Width / Invisible Chars or Homoglyphs
	if obf.HasInvisibleChars {
		action = "transform"
		reason = "Invisible or zero-width characters detected and removed"
		mutated, _ = syntactic.StripInvisibleCharacters(s)
	} else if obf.HasHomoglyphs {
		action = "transform"
		reason = "Homoglyph obfuscation detected and normalized"
		mutated, _ = syntactic.CanonicalizeHomoglyphs(s)
	}

	// 2. Check for Prompt Injection / Traversal across all layers
	hasInjection := false
	for _, layer := range layers {
		clean := syntactic.Normalize(layer)

		for _, pat := range InjectionPatterns {
			if strings.Contains(clean, strings.ToLower(pat)) {
				hasInjection = true
				break
			}
		}
		if hasInjection {
			break
		}

		for _, pat := range customPatterns {
			if strings.Contains(clean, strings.ToLower(pat)) {
				hasInjection = true
				break
			}
		}
		if hasInjection {
			break
		}

		for _, pat := range TraversalPatterns {
			if strings.Contains(clean, strings.ToLower(pat)) {
				hasInjection = true
				break
			}
		}
		if hasInjection {
			break
		}
	}

	if hasInjection {
		action = "transform"
		reason = "Prompt injection or path traversal pattern detected"
		mutated = "[Content removed: security policy]"
	}

	// 3. Check for Secret Leakage across all layers (Highest Precedence)
	for _, layer := range layers {
		for _, re := range SecretRegexes {
			if re.MatchString(layer) {
				action = "quarantine"
				reason = "Secret or credential exfiltration detected in tool response"
				mutated = re.ReplaceAllString(s, "***")
				return action, reason, mutated
			}
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
