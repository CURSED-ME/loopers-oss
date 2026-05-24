package gemini

import (
	"bytes"
	"encoding/json"
)

// parseGeminiStreamChunk extracts token usage from a Gemini stream chunk.
func parseGeminiStreamChunk(chunk []byte) (int, int, bool, error) {
	// Clean up SSE prefix if present
	cleanChunk := bytes.TrimSpace(chunk)
	if bytes.HasPrefix(cleanChunk, []byte("data: ")) {
		cleanChunk = bytes.TrimSpace(bytes.TrimPrefix(cleanChunk, []byte("data: ")))
	}

	// Clean up JSON array stream brackets/commas if present
	cleanChunk = bytes.TrimPrefix(cleanChunk, []byte("["))
	cleanChunk = bytes.TrimPrefix(cleanChunk, []byte(","))
	cleanChunk = bytes.TrimSuffix(cleanChunk, []byte("]"))
	cleanChunk = bytes.TrimSpace(cleanChunk)

	if len(cleanChunk) == 0 {
		return 0, 0, false, nil
	}

	var payload struct {
		UsageMetadata struct {
			PromptTokens     int `json:"promptTokenCount"`
			CandidatesTokens int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(cleanChunk, &payload); err == nil {
		if payload.UsageMetadata.PromptTokens > 0 || payload.UsageMetadata.CandidatesTokens > 0 {
			// Apply 10% safety buffer on output tokens to handle known Flash Lite undercounting issues
			outTokens := int(float64(payload.UsageMetadata.CandidatesTokens) * 1.10)
			return payload.UsageMetadata.PromptTokens, outTokens, false, nil
		}
	}

	return 0, 0, false, nil
}

// formatGeminiBudgetExceededSSE returns the Gemini SSE error frame for budget exceeded.
func formatGeminiBudgetExceededSSE() []byte {
	return []byte(`data: {"error":{"code":429,"message":"Budget exceeded mid-stream","status":"RESOURCE_EXHAUSTED"}}` + "\n\n")
}
