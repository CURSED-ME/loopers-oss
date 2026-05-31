package openai

import (
	"encoding/json"
	"strings"

	"github.com/loopers-oss/loopers/pkg/api"
	"github.com/pkoukk/tiktoken-go"
)

func init() {
	// Pre-load encodings on startup to prevent massive memory spikes
	// from concurrent initializations when multiple requests hit the server.
	_, _ = tiktoken.GetEncoding("cl100k_base")
	_, _ = tiktoken.GetEncoding("o200k_base")
}

var tokenizationSema = make(chan struct{}, 4)

// countOpenAIRequestTokens parses the OpenAI body and counts prompt tokens.
func countOpenAIRequestTokens(model string, body []byte) (int, error) {
	// Acquire semaphore slot to prevent massive memory spikes from concurrent regexp2 executions in tiktoken
	tokenizationSema <- struct{}{}
	defer func() { <-tokenizationSema }()

	var req api.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return 0, err
	}

	var encodingName string
	modelLower := strings.ToLower(model)

	// Determine matching tiktoken encoding
	if strings.Contains(modelLower, "gpt-4o") ||
		strings.Contains(modelLower, "gpt-5") ||
		strings.Contains(modelLower, "o3") ||
		strings.Contains(modelLower, "o4-mini") {
		encodingName = "o200k_base"
	} else {
		encodingName = "cl100k_base"
	}

	tke, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		tke, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return 0, err
		}
	}

	tokens := 0
	tokensPerMessage := 3
	tokensPerName := 1

	for _, msg := range req.Messages {
		tokens += tokensPerMessage
		if msg.Name != "" {
			tokens += tokensPerName
		}

		switch c := msg.Content.(type) {
		case string:
			tokens += len(tke.Encode(c, nil, nil))
		case []interface{}:
			for _, part := range c {
				if partMap, ok := part.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok {
						tokens += len(tke.Encode(text, nil, nil))
					}
				}
			}
		}

		tokens += len(tke.Encode(msg.Role, nil, nil))
	}

	tokens += 3 // Assistant reply priming overhead
	return tokens, nil
}
