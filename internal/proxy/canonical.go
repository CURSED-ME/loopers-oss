package proxy

import (
	"encoding/json"
	"strings"

	"github.com/CURSED-ME/loopers-oss/internal/policy"
)

// MapLLMRequestToContext parses provider request bodies to extract structured ActionContext.
func MapLLMRequestToContext(provider string, body []byte) (policy.ActionContext, error) {
	actionCtx := policy.ActionContext{
		Type:      "llm_call",
		Provider:  provider,
		Messages:  make([]policy.CanonicalMessage, 0),
		Tools:     make([]policy.CanonicalToolDefinition, 0),
		ToolCalls: make([]policy.CanonicalToolCall, 0),
	}

	if len(body) == 0 {
		return actionCtx, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		actionCtx.PromptText = string(body)
		return actionCtx, nil
	}

	if model, ok := data["model"].(string); ok && model != "" {
		actionCtx.Model = model
	}

	var prompts []string
	providerLower := strings.ToLower(provider)

	switch providerLower {
	case "anthropic":
		// Handle Anthropic system prompt
		if sys, ok := data["system"]; ok {
			if sysStr, ok := sys.(string); ok && sysStr != "" {
				prompts = append(prompts, sysStr)
				actionCtx.Messages = append(actionCtx.Messages, policy.CanonicalMessage{
					Role:    "system",
					Content: sysStr,
				})
			} else if sysArr, ok := sys.([]interface{}); ok {
				var sysParts []string
				for _, part := range sysArr {
					if partMap, ok := part.(map[string]interface{}); ok {
						if text, ok := partMap["text"].(string); ok && text != "" {
							sysParts = append(sysParts, text)
						}
					}
				}
				if len(sysParts) > 0 {
					combined := strings.Join(sysParts, "\n")
					prompts = append(prompts, combined)
					actionCtx.Messages = append(actionCtx.Messages, policy.CanonicalMessage{
						Role:    "system",
						Content: combined,
					})
				}
			}
		}

		// Handle Anthropic messages
		if msgs, ok := data["messages"].([]interface{}); ok {
			for _, msg := range msgs {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					role, _ := msgMap["role"].(string)
					if role == "" {
						role = "user"
					}
					var msgContent string

					if content := msgMap["content"]; content != nil {
						if contentStr, ok := content.(string); ok {
							msgContent = contentStr
							prompts = append(prompts, contentStr)
						} else if contentArr, ok := content.([]interface{}); ok {
							var textParts []string
							for _, block := range contentArr {
								if blockMap, ok := block.(map[string]interface{}); ok {
									blockType, _ := blockMap["type"].(string)
									if blockType == "tool_use" {
										id, _ := blockMap["id"].(string)
										name, _ := blockMap["name"].(string)
										args, _ := blockMap["input"].(map[string]interface{})
										actionCtx.ToolCalls = append(actionCtx.ToolCalls, policy.CanonicalToolCall{
											ID:        id,
											Name:      name,
											Arguments: args,
										})
									} else if text, ok := blockMap["text"].(string); ok && text != "" {
										textParts = append(textParts, text)
									}
								}
							}
							if len(textParts) > 0 {
								msgContent = strings.Join(textParts, "\n")
								prompts = append(prompts, msgContent)
							}
						}
					}

					actionCtx.Messages = append(actionCtx.Messages, policy.CanonicalMessage{
						Role:    role,
						Content: msgContent,
					})
				}
			}
		}

		// Handle Anthropic tools
		if tools, ok := data["tools"].([]interface{}); ok {
			for _, tool := range tools {
				if toolMap, ok := tool.(map[string]interface{}); ok {
					name, _ := toolMap["name"].(string)
					desc, _ := toolMap["description"].(string)
					params, _ := toolMap["input_schema"].(map[string]interface{})
					actionCtx.Tools = append(actionCtx.Tools, policy.CanonicalToolDefinition{
						Name:        name,
						Description: desc,
						Parameters:  params,
					})
				}
			}
		}

	case "gemini":
		// Handle Gemini contents
		if contents, ok := data["contents"].([]interface{}); ok {
			for _, content := range contents {
				if contentMap, ok := content.(map[string]interface{}); ok {
					role, _ := contentMap["role"].(string)
					if role == "model" {
						role = "assistant"
					} else if role == "" {
						role = "user"
					}

					var textParts []string
					if parts, ok := contentMap["parts"].([]interface{}); ok {
						for _, part := range parts {
							if partMap, ok := part.(map[string]interface{}); ok {
								if text, ok := partMap["text"].(string); ok && text != "" {
									textParts = append(textParts, text)
								}
								// Check function_call
								fc := partMap["function_call"]
								if fc == nil {
									fc = partMap["functionCall"]
								}
								if fcMap, ok := fc.(map[string]interface{}); ok {
									name, _ := fcMap["name"].(string)
									args, _ := fcMap["args"].(map[string]interface{})
									actionCtx.ToolCalls = append(actionCtx.ToolCalls, policy.CanonicalToolCall{
										Name:      name,
										Arguments: args,
									})
								}
							}
						}
					}
					msgContent := strings.Join(textParts, "\n")
					if msgContent != "" {
						prompts = append(prompts, msgContent)
					}
					actionCtx.Messages = append(actionCtx.Messages, policy.CanonicalMessage{
						Role:    role,
						Content: msgContent,
					})
				}
			}
		}

		// Handle Gemini tools
		if tools, ok := data["tools"].([]interface{}); ok {
			for _, tool := range tools {
				if toolMap, ok := tool.(map[string]interface{}); ok {
					decls := toolMap["function_declarations"]
					if decls == nil {
						decls = toolMap["functionDeclarations"]
					}
					if declArr, ok := decls.([]interface{}); ok {
						for _, decl := range declArr {
							if declMap, ok := decl.(map[string]interface{}); ok {
								name, _ := declMap["name"].(string)
								desc, _ := declMap["description"].(string)
								params, _ := declMap["parameters"].(map[string]interface{})
								actionCtx.Tools = append(actionCtx.Tools, policy.CanonicalToolDefinition{
									Name:        name,
									Description: desc,
									Parameters:  params,
								})
							}
						}
					}
				}
			}
		}

	default:
		// Default to OpenAI / OpenAI-compatible format
		if msgs, ok := data["messages"].([]interface{}); ok {
			for _, msg := range msgs {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					role, _ := msgMap["role"].(string)
					if role == "" {
						role = "user"
					}
					var msgContent string

					if content := msgMap["content"]; content != nil {
						if contentStr, ok := content.(string); ok {
							msgContent = contentStr
							prompts = append(prompts, contentStr)
						} else if contentArr, ok := content.([]interface{}); ok {
							var textParts []string
							for _, part := range contentArr {
								if partMap, ok := part.(map[string]interface{}); ok {
									if text, ok := partMap["text"].(string); ok && text != "" {
										textParts = append(textParts, text)
									}
								}
							}
							if len(textParts) > 0 {
								msgContent = strings.Join(textParts, "\n")
								prompts = append(prompts, msgContent)
							}
						}
					}

					// Handle OpenAI tool_calls in message
					if tcs, ok := msgMap["tool_calls"].([]interface{}); ok {
						for _, tc := range tcs {
							if tcMap, ok := tc.(map[string]interface{}); ok {
								id, _ := tcMap["id"].(string)
								if fn, ok := tcMap["function"].(map[string]interface{}); ok {
									name, _ := fn["name"].(string)
									var args map[string]interface{}
									if argsStr, ok := fn["arguments"].(string); ok && argsStr != "" {
										_ = json.Unmarshal([]byte(argsStr), &args)
									} else if argsMap, ok := fn["arguments"].(map[string]interface{}); ok {
										args = argsMap
									}
									actionCtx.ToolCalls = append(actionCtx.ToolCalls, policy.CanonicalToolCall{
										ID:        id,
										Name:      name,
										Arguments: args,
									})
								}
							}
						}
					}

					actionCtx.Messages = append(actionCtx.Messages, policy.CanonicalMessage{
						Role:    role,
						Content: msgContent,
					})
				}
			}
		}

		// Handle OpenAI tools
		if tools, ok := data["tools"].([]interface{}); ok {
			for _, tool := range tools {
				if toolMap, ok := tool.(map[string]interface{}); ok {
					if fn, ok := toolMap["function"].(map[string]interface{}); ok {
						name, _ := fn["name"].(string)
						desc, _ := fn["description"].(string)
						params, _ := fn["parameters"].(map[string]interface{})
						actionCtx.Tools = append(actionCtx.Tools, policy.CanonicalToolDefinition{
							Name:        name,
							Description: desc,
							Parameters:  params,
						})
					}
				}
			}
		}

		// Handle legacy OpenAI functions
		if funcs, ok := data["functions"].([]interface{}); ok {
			for _, fn := range funcs {
				if fnMap, ok := fn.(map[string]interface{}); ok {
					name, _ := fnMap["name"].(string)
					desc, _ := fnMap["description"].(string)
					params, _ := fnMap["parameters"].(map[string]interface{})
					actionCtx.Tools = append(actionCtx.Tools, policy.CanonicalToolDefinition{
						Name:        name,
						Description: desc,
						Parameters:  params,
					})
				}
			}
		}
	}

	actionCtx.PromptText = strings.Join(prompts, "\n")
	return actionCtx, nil
}

// MapLLMCall parses provider request bodies to extract prompt text.
func MapLLMCall(provider string, body []byte) (string, error) {
	ctx, err := MapLLMRequestToContext(provider, body)
	if err != nil {
		return "", err
	}
	return ctx.PromptText, nil
}

// MapLLMResponse parses provider response bodies to extract completion text.
func MapLLMResponse(provider string, body []byte) (string, error) {
	if len(body) == 0 {
		return "", nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	var completions []string
	provider = strings.ToLower(provider)
	switch provider {
	case "anthropic":
		if content, ok := data["content"].([]interface{}); ok {
			for _, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok {
						completions = append(completions, text)
					}
				}
			}
		}
	case "gemini":
		if candidates, ok := data["candidates"].([]interface{}); ok {
			for _, cand := range candidates {
				if candMap, ok := cand.(map[string]interface{}); ok {
					if content, ok := candMap["content"].(map[string]interface{}); ok {
						if parts, ok := content["parts"].([]interface{}); ok {
							for _, part := range parts {
								if partMap, ok := part.(map[string]interface{}); ok {
									if text, ok := partMap["text"].(string); ok {
										completions = append(completions, text)
									}
								}
							}
						}
					}
				}
			}
		}
	default:
		// Default to OpenAI/standard chat response
		if choices, ok := data["choices"].([]interface{}); ok {
			for _, choice := range choices {
				if choiceMap, ok := choice.(map[string]interface{}); ok {
					if msg, ok := choiceMap["message"].(map[string]interface{}); ok {
						if content, ok := msg["content"].(string); ok {
							completions = append(completions, content)
						}
					}
				}
			}
		}
	}

	return strings.Join(completions, "\n"), nil
}
