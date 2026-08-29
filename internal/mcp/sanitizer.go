package mcp

import (
	"encoding/json"
	"strings"

	"github.com/try-loopers/loopers-oss/internal/inspector"
)

// ListToolsResult represents the payload in the JSON-RPC result for tools/list.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// Tool represents a single tool returned by an MCP server.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// sanitizeString delegates to the unified inspector.SanitizeString function.
func sanitizeString(input string) string {
	return inspector.SanitizeString(input)
}

// SanitizeToolList takes a JSON-RPC response body for tools/list and applies
// sanitization rules, returning the new JSON body.
func SanitizeToolList(body []byte, cfg SanitizerConfig, serverName string) ([]byte, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		// Not JSON, can't sanitize, return original
		return body, nil
	}

	resultMap, ok := resp["result"].(map[string]interface{})
	if !ok {
		return body, nil
	}

	toolsRaw, ok := resultMap["tools"].([]interface{})
	if !ok {
		return body, nil
	}

	var allowedTools map[string]bool
	if cfg.ToolAllowlist != nil {
		if list, exists := cfg.ToolAllowlist[serverName]; exists {
			allowedTools = make(map[string]bool)
			for _, t := range list {
				allowedTools[t] = true
			}
		}
	}

	var sanitizedTools []interface{}
	for _, tr := range toolsRaw {
		toolMap, ok := tr.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := toolMap["name"].(string)

		// 1. Tool allowlist
		if allowedTools != nil && !allowedTools[name] {
			continue // skip this tool
		}

		// 2. Strip injection patterns
		if desc, ok := toolMap["description"].(string); ok {
			cleanDesc := sanitizeString(desc)
			hasInjection := false
			for _, pat := range inspector.InjectionPatterns {
				if strings.Contains(cleanDesc, pat) {
					hasInjection = true
					break
				}
			}

			if hasInjection {
				// Strip malicious description entirely
				toolMap["description"] = "[Description removed due to security policy]"
			} else {
				// 3. Truncate description if clean
				if cfg.MaxDescriptionLength > 0 && len(desc) > cfg.MaxDescriptionLength {
					desc = desc[:cfg.MaxDescriptionLength] + "..."
				}
				toolMap["description"] = desc
			}
		}

		sanitizedTools = append(sanitizedTools, toolMap)
	}

	resultMap["tools"] = sanitizedTools
	resp["result"] = resultMap

	return json.Marshal(resp)
}
