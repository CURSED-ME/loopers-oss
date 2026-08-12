package proxy

import (
	"encoding/json"
	"strings"
)

// MapLLMCall parses provider request bodies to extract prompt text.
func MapLLMCall(provider string, body []byte) (string, error) {
	if len(body) == 0 {
		return "", nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// Fallback to raw body string if it's not JSON
		return string(body), nil
	}

	var prompts []string

	provider = strings.ToLower(provider)
	switch provider {
	case "anthropic":
		// Handle Anthropic system prompt if set
		if sys, ok := data["system"]; ok {
			if sysStr, ok := sys.(string); ok {
				prompts = append(prompts, sysStr)
			} else if sysArr, ok := sys.([]interface{}); ok {
				for _, part := range sysArr {
					if partMap, ok := part.(map[string]interface{}); ok {
						if text, ok := partMap["text"].(string); ok {
							prompts = append(prompts, text)
						}
					}
				}
			}
		}
		// Handle Anthropic messages
		if msgs, ok := data["messages"].([]interface{}); ok {
			for _, msg := range msgs {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					if content := msgMap["content"]; content != nil {
						if contentStr, ok := content.(string); ok {
							prompts = append(prompts, contentStr)
						} else if contentArr, ok := content.([]interface{}); ok {
							for _, block := range contentArr {
								if blockMap, ok := block.(map[string]interface{}); ok {
									if text, ok := blockMap["text"].(string); ok {
										prompts = append(prompts, text)
									}
								}
							}
						}
					}
				}
			}
		}
	case "gemini":
		// Handle Gemini contents
		if contents, ok := data["contents"].([]interface{}); ok {
			for _, content := range contents {
				if contentMap, ok := content.(map[string]interface{}); ok {
					if parts, ok := contentMap["parts"].([]interface{}); ok {
						for _, part := range parts {
							if partMap, ok := part.(map[string]interface{}); ok {
								if text, ok := partMap["text"].(string); ok {
									prompts = append(prompts, text)
								}
							}
						}
					}
				}
			}
		}
	default:
		// Default to OpenAI/standard chat format
		if msgs, ok := data["messages"].([]interface{}); ok {
			for _, msg := range msgs {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					if content := msgMap["content"]; content != nil {
						if contentStr, ok := content.(string); ok {
							prompts = append(prompts, contentStr)
						} else if contentArr, ok := content.([]interface{}); ok {
							for _, part := range contentArr {
								if partMap, ok := part.(map[string]interface{}); ok {
									if text, ok := partMap["text"].(string); ok {
										prompts = append(prompts, text)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return strings.Join(prompts, "\n"), nil
}
