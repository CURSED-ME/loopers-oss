package bedrock

import (
	"encoding/json"

	"github.com/pkoukk/tiktoken-go"
)

// countBedrockTokensFallback estimates prompt token count for Bedrock models using cl100k_base with a 15% buffer.
func countBedrockTokensFallback(body []byte) (int, error) {
	// 1. Try parsing messages format (Claude)
	var messagesReq struct {
		Messages []struct {
			Role    string      `json:"role"`
			Content interface{} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &messagesReq); err == nil && len(messagesReq.Messages) > 0 {
		tke, err := tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return 0, err
		}
		tokens := 0
		for _, msg := range messagesReq.Messages {
			tokens += 4 // Average overhead per message
			if text, ok := msg.Content.(string); ok {
				tokens += len(tke.Encode(text, nil, nil))
			} else if parts, ok := msg.Content.([]interface{}); ok {
				for _, part := range parts {
					if partMap, ok := part.(map[string]interface{}); ok {
						if text, ok := partMap["text"].(string); ok {
							tokens += len(tke.Encode(text, nil, nil))
						}
					}
				}
			}
			tokens += len(tke.Encode(msg.Role, nil, nil))
		}
		return int(float64(tokens) * 1.15), nil
	}

	// 2. Try parsing raw prompt format (Llama / Titan / Mistral)
	var promptReq struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(body, &promptReq); err == nil && promptReq.Prompt != "" {
		tke, err := tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return 0, err
		}
		tokens := len(tke.Encode(promptReq.Prompt, nil, nil))
		return int(float64(tokens) * 1.15), nil
	}

	// 3. Generic fallback based on body character count
	charCount := len(body)
	estimatedTokens := int(float64(charCount) / 4.0 * 1.15)
	return estimatedTokens, nil
}
