package blastradius

import (
	"fmt"
	"regexp"
	"strings"
)

// RiskTier represents the categorized severity of a tool execution blast radius.
type RiskTier string

const (
	TierLow      RiskTier = "low"
	TierMedium   RiskTier = "medium"
	TierHigh     RiskTier = "high"
	TierCritical RiskTier = "critical"
)

// BlastRadiusResult holds the calculated risk metrics for a tool invocation.
type BlastRadiusResult struct {
	Score   int      `json:"score"`
	Tier    string   `json:"tier"`
	Reasons []string `json:"reasons"`
}

var (
	destructiveVerbs = map[string]bool{
		"delete":           true,
		"drop":             true,
		"rm":               true,
		"remove":           true,
		"purge":            true,
		"wipe":             true,
		"destroy":          true,
		"truncate":         true,
		"kill":             true,
		"terminate":        true,
		"format":           true,
		"shutdown":         true,
		"erase":            true,
		"revoke":           true,
		"clean":            true,
		"prune":            true,
		"delete_database":  true,
		"drop_database":    true,
		"rm_files":         true,
		"destroy_iam_role": true,
	}

	systemExecVerbs = map[string]bool{
		"exec":         true,
		"execute":      true,
		"eval":         true,
		"bash":         true,
		"sh":           true,
		"cmd":          true,
		"terminal":     true,
		"spawn":        true,
		"sudo":         true,
		"run_script":   true,
		"run_command":  true,
		"shell":        true,
		"system":       true,
		"run":          true,
		"command":      true,
		"execute_bash": true,
	}

	writeMutationVerbs = map[string]bool{
		"write":      true,
		"modify":     true,
		"update":     true,
		"alter":      true,
		"patch":      true,
		"create":     true,
		"insert":     true,
		"upload":     true,
		"push":       true,
		"set":        true,
		"put":        true,
		"deploy":     true,
		"publish":    true,
		"transfer":   true,
		"send":       true,
		"post":       true,
		"write_file": true,
	}

	readOnlyVerbs = map[string]bool{
		"get":             true,
		"read":            true,
		"list":            true,
		"fetch":           true,
		"search":          true,
		"describe":        true,
		"inspect":         true,
		"find":            true,
		"view":            true,
		"check":           true,
		"status":          true,
		"show":            true,
		"read_file":       true,
		"list_directory":  true,
		"search_codebase": true,
	}

	iamKeywords = map[string]bool{
		"iam":            true,
		"policy":         true,
		"permission":     true,
		"role":           true,
		"user_admin":     true,
		"security_group": true,
		"credential":     true,
		"auth":           true,
		"rbac":           true,
		"secret":         true,
		"vault":          true,
		"keyring":        true,
		"token":          true,
		"private_key":    true,
		"api_key":        true,
		"password":       true,
		"access_key":     true,
		"bearer":         true,
	}

	dbKeywords = map[string]bool{
		"database": true,
		"db":       true,
		"postgres": true,
		"mysql":    true,
		"redis":    true,
		"mongodb":  true,
		"sqlite":   true,
		"table":    true,
		"sql":      true,
	}

	infraKeywords = map[string]bool{
		"prod":       true,
		"production": true,
		"cluster":    true,
		"k8s":        true,
		"kubernetes": true,
		"root":       true,
		"system32":   true,
		"registry":   true,
	}

	financialKeywords = map[string]bool{
		"payment":  true,
		"transfer": true,
		"invoice":  true,
		"charge":   true,
		"billing":  true,
		"wallet":   true,
		"payout":   true,
		"refund":   true,
		"wire":     true,
		"credit":   true,
	}

	messagingKeywords = map[string]bool{
		"send_email":   true,
		"post_message": true,
		"notify_slack": true,
		"send_sms":     true,
	}

	urlRegex  = regexp.MustCompile(`(?i)(https?://|ftp://|sftp://|ssh://|webhook\.site|pastebin|ngrok)`)
	ipv4Regex = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
)

// Calculate inspects tool names and arguments to compute a blast radius risk score (0-100).
func Calculate(toolName string, args map[string]interface{}) BlastRadiusResult {
	score := 0
	reasons := make([]string, 0, 8)

	lowerTool := strings.ToLower(toolName)
	toolTokens := splitTokens(lowerTool)

	hasDestructiveVerb := destructiveVerbs[lowerTool]
	hasSystemExec := systemExecVerbs[lowerTool]
	hasWriteVerb := writeMutationVerbs[lowerTool]
	hasReadOnlyVerb := readOnlyVerbs[lowerTool]

	for _, token := range toolTokens {
		if destructiveVerbs[token] {
			hasDestructiveVerb = true
		}
		if systemExecVerbs[token] {
			hasSystemExec = true
		}
		if writeMutationVerbs[token] {
			hasWriteVerb = true
		}
		if readOnlyVerbs[token] {
			hasReadOnlyVerb = true
		}
	}

	// 1. Tool Verb Classification
	if hasDestructiveVerb {
		score += 35
		reasons = append(reasons, "Tool name contains destructive operational verb (+35)")
	} else if hasSystemExec {
		score += 30
		reasons = append(reasons, "Tool name contains system/shell execution verb (+30)")
	} else if hasWriteVerb {
		score += 15
		reasons = append(reasons, "Tool name contains mutating/write verb (+15)")
	}

	// 2. Scan Arguments and Tool Name
	argStrings := collectStrings(args, 0, 6)
	allText := strings.Join(argStrings, " ")
	fullText := lowerTool + " " + allText
	allTokens := splitTokens(strings.ToLower(fullText))

	// Check IAM / Credential Targets
	if matchAnyToken(allTokens, iamKeywords) {
		score += 25
		reasons = append(reasons, "Target scope involves IAM, secrets, or authentication credentials (+25)")
	}

	// Check Database Targets
	if matchAnyToken(allTokens, dbKeywords) {
		score += 25
		reasons = append(reasons, "Target scope involves database or datastore storage (+25)")
	}

	// Check Production / Infrastructure Targets
	if matchAnyToken(allTokens, infraKeywords) || strings.Contains(fullText, "/etc/passwd") || strings.Contains(fullText, "/etc/shadow") || strings.Contains(fullText, "c:\\windows") {
		score += 25
		reasons = append(reasons, "Target scope involves critical infrastructure, clusters, or production environments (+25)")
	}

	// Check Financial Targets
	if matchAnyToken(allTokens, financialKeywords) {
		score += 25
		reasons = append(reasons, "Target scope involves financial, billing, or monetary transfers (+25)")
	}

	// Check Outbound Messaging
	if matchAnyToken(allTokens, messagingKeywords) {
		score += 20
		reasons = append(reasons, "Target scope involves outbound email or external messaging (+20)")
	}

	// Check Network Egress / URLs
	if urlRegex.MatchString(allText) || ipv4Regex.MatchString(allText) {
		score += 25
		reasons = append(reasons, "Arguments contain external network egress URLs or IP addresses (+25)")
	}

	// Check Bulk Wildcards / Recursion Flags
	if detectBulkWildcards(args, argStrings) {
		score += 20
		reasons = append(reasons, "Arguments specify broad scope, wildcard matching, or recursive execution (+20)")
	}

	// Check Root Path / Path Traversal
	if detectPathTraversalOrRoot(argStrings) {
		score += 20
		reasons = append(reasons, "Arguments target filesystem root or parent directory traversals (+20)")
	}

	// Read-Only Mitigation: only if no destructive or execution verbs were matched
	if hasReadOnlyVerb && !hasDestructiveVerb && !hasSystemExec {
		if score == 0 {
			// Pure read-only safe operation
			score = 0
		} else {
			score -= 10
			if score < 0 {
				score = 0
			}
		}
	}

	// Clamping
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	tier := computeTier(score)

	return BlastRadiusResult{
		Score:   score,
		Tier:    string(tier),
		Reasons: reasons,
	}
}

func computeTier(score int) RiskTier {
	switch {
	case score >= 85:
		return TierCritical
	case score >= 60:
		return TierHigh
	case score >= 30:
		return TierMedium
	default:
		return TierLow
	}
}

func splitTokens(s string) []string {
	f := func(c rune) bool {
		return c == '_' || c == '-' || c == '.' || c == ':' || c == '/' || c == '\\' || c == ' ' || c == '=' || c == '"' || c == '\'' || c == '(' || c == ')' || c == '{' || c == '}' || c == '[' || c == ']' || c == ','
	}
	return strings.FieldsFunc(s, f)
}

func matchAnyToken(tokens []string, targetMap map[string]bool) bool {
	for _, tok := range tokens {
		if targetMap[tok] {
			return true
		}
	}
	return false
}

func collectStrings(args map[string]interface{}, depth int, maxDepth int) []string {
	if depth > maxDepth || len(args) == 0 {
		return nil
	}

	var results []string
	for k, v := range args {
		results = append(results, k)
		switch val := v.(type) {
		case string:
			results = append(results, val)
		case []interface{}:
			for _, item := range val {
				if itemStr, ok := item.(string); ok {
					results = append(results, itemStr)
				} else if itemMap, ok := item.(map[string]interface{}); ok {
					nested := collectStrings(itemMap, depth+1, maxDepth)
					results = append(results, nested...)
				}
			}
		case map[string]interface{}:
			nested := collectStrings(val, depth+1, maxDepth)
			results = append(results, nested...)
		case bool:
			if val {
				results = append(results, k+"=true")
			}
		case fmt.Stringer:
			results = append(results, val.String())
		}
	}
	return results
}

func detectBulkWildcards(args map[string]interface{}, argStrings []string) bool {
	for _, s := range argStrings {
		lower := strings.ToLower(s)
		if lower == "*" || lower == "/*" || lower == ".*" || lower == "%" || lower == "all" {
			return true
		}
		if strings.Contains(lower, "-rf") || strings.Contains(lower, "--all") || strings.Contains(lower, "--force") || strings.Contains(lower, "--cascade") {
			return true
		}
	}

	// Check boolean keys
	for k, v := range args {
		lowerKey := strings.ToLower(k)
		if b, ok := v.(bool); ok && b {
			if lowerKey == "recursive" || lowerKey == "force" || lowerKey == "all" || lowerKey == "cascade" {
				return true
			}
		}
		if num, ok := v.(float64); ok && (lowerKey == "depth" || lowerKey == "max_depth") && num >= 10 {
			return true
		}
		if num, ok := v.(int); ok && (lowerKey == "depth" || lowerKey == "max_depth") && num >= 10 {
			return true
		}
	}

	return false
}

func detectPathTraversalOrRoot(argStrings []string) bool {
	for _, s := range argStrings {
		trimmed := strings.TrimSpace(s)
		if trimmed == "/" || trimmed == "/*" || trimmed == "C:\\" || trimmed == "C:/*" || trimmed == "~" {
			return true
		}
		if strings.Contains(s, "..") || strings.Contains(s, "../..") || strings.Contains(s, "..\\..") {
			return true
		}
		if strings.Contains(strings.ToUpper(s), "DROP DATABASE") || strings.Contains(strings.ToUpper(s), "DROP TABLE") {
			return true
		}
	}
	return false
}
