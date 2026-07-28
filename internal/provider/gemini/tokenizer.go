package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pkoukk/tiktoken-go"
)

var (
	cl100kEncoding     *tiktoken.Tiktoken
	cl100kEncodingOnce sync.Once
	cl100kEncodingErr  error
)

func getCL100kEncoding() (*tiktoken.Tiktoken, error) {
	cl100kEncodingOnce.Do(func() {
		cl100kEncoding, cl100kEncodingErr = tiktoken.GetEncoding("cl100k_base")
	})
	return cl100kEncoding, cl100kEncodingErr
}

// callGeminiCountTokensAPI calls Gemini's countTokens API to get the exact prompt token count.
func (g *GeminiProvider) callGeminiCountTokensAPI(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	if providerKey == "" {
		return 0, fmt.Errorf("no provider key for countTokens API")
	}
	apiCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	baseURL := g.BaseURL()
	client := g.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	// countTokens endpoint matches the generateContent endpoint path format
	reqURL := fmt.Sprintf("%s/v1beta/models/%s:countTokens?key=%s", strings.TrimRight(baseURL, "/"), model, providerKey)

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

	resp, err := client.Do(req)
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

	tke, err := getCL100kEncoding()
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
