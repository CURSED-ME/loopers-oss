package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pkoukk/tiktoken-go"
)

// callGeminiCountTokensAPI calls Gemini's countTokens API to get the exact prompt token count.
func callGeminiCountTokensAPI(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	if providerKey == "" {
		return 0, fmt.Errorf("no provider key for countTokens API")
	}
	apiCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// countTokens endpoint matches the generateContent endpoint path format
	reqURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:countTokens?key=%s", model, providerKey)

	// Create payload containing the contents structure
	var contentPayload struct {
		Contents interface{} `json:"contents"`
	}

	// Try parsing contents from generateContent request body
	var reqBody struct {
		Contents interface{} `json:"contents"`
	}
	if err := json.Unmarshal(body, &reqBody); err == nil && reqBody.Contents != nil {
		contentPayload.Contents = reqBody.Contents
	} else {
		return 0, fmt.Errorf("invalid gemini request body structure")
	}

	payloadBytes, err := json.Marshal(contentPayload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(apiCtx, http.MethodPost, reqURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("non-200 response from Gemini countTokens API: %d", resp.StatusCode)
	}

	var countResp struct {
		TotalTokens int `json:"totalTokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		return 0, err
	}
	return countResp.TotalTokens, nil
}

// countGeminiTokensFallback counts tokens locally using tiktoken cl100k_base with a 15% safety buffer.
func countGeminiTokensFallback(body []byte) (int, error) {
	var req struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return 0, err
	}

	tke, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return 0, err
	}

	tokens := 0
	for _, content := range req.Contents {
		tokens += 4 // Turn/message overhead
		for _, part := range content.Parts {
			tokens += len(tke.Encode(part.Text, nil, nil))
		}
		if content.Role != "" {
			tokens += len(tke.Encode(content.Role, nil, nil))
		}
	}

	// Apply 15% safety buffer for tokenization variance
	bufferedTokens := int(float64(tokens) * 1.15)
	return bufferedTokens, nil
}
