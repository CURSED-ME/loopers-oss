package openai

import (
	"bytes"
	"encoding/json"

	"github.com/try-loopers/loopers-oss/pkg/api"
)

// parseOpenAIStreamChunk processes a single SSE chunk for OpenAI and extracts token usage if available.
func parseOpenAIStreamChunk(chunk []byte) (int, int, bool, error) {
	lines := bytes.Split(chunk, []byte("\n"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("data: ")) {
			dataPayload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data: ")))
			if bytes.Equal(dataPayload, []byte("[DONE]")) {
				return 0, 0, true, nil
			}

			var chunkObj api.OpenAIStreamChunk
			if err := json.Unmarshal(dataPayload, &chunkObj); err == nil && chunkObj.Usage != nil {
				return chunkObj.Usage.PromptTokens, chunkObj.Usage.CompletionTokens, false, nil
			}
		}
	}
	return 0, 0, false, nil
}

// formatOpenAIBudgetExceededSSE returns the OpenAI SSE error frame for budget exceeded.
func formatOpenAIBudgetExceededSSE() []byte {
	return []byte(`data: {"error":{"message":"Budget exceeded mid-stream","type":"circuit_break_budget_exceeded","code":"budget_exceeded"}}` + "\n\n" + `data: [DONE]` + "\n\n")
}
