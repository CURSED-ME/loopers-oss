---
id: adding-a-provider
title: Adding a New Provider
sidebar_label: Adding a Provider
description: Step by step guide to implementing a new LLM provider adapter in Loopers.
---

# Adding a New Provider

This guide walks you through implementing a new AI provider adapter for Loopers. A provider adapter acts as a translator. It helps Loopers understand how to talk to a specific AI service (like OpenAI, Anthropic, or Gemini).

Each provider has different API formats, auth headers, and response styles. The provider adapter standardizes these differences.

## 1. Implement the Provider Interface

All providers implement the `Provider` interface found in `internal/provider/provider.go`:

```go
package provider

import (
	"context"
	"net/http"
)

type Provider interface {
	// Name returns the provider name (e.g. "openai", "gemini")
	Name() string

	// BaseURL returns the real AI provider API base URL
	BaseURL() string

	// InjectAuth sets the provider specific auth headers on the request.
	// providerKey is the raw API key from X-Loopers-Provider-Key.
	InjectAuth(req *http.Request, providerKey string)

	// RewritePath transforms the Loopers route path to the upstream path.
	// For example: "/anthropic/v1/messages" -> "/v1/messages"
	RewritePath(originalPath string) string

	// ParseRequest extracts model name, streaming flag, and max_tokens from the request/body.
	ParseRequest(req *http.Request, body []byte) (model string, isStream bool, maxTokens int, mutatedBody []byte, err error)

	// RewriteModel rewrites the request to use a fallback model.
	// It modifies req in place if needed and returns the modified JSON body.
	RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error)

	// CountInputTokens returns the estimated input token count for the request.
	// ctx and providerKey are passed for providers that require API based counting (like Gemini or Anthropic).
	CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error)

	// ParseStreamChunk processes a single SSE EventStream chunk and returns token usage if available.
	// Returns (inputTokens, outputTokens, isDone, err).
	ParseStreamChunk(chunk []byte) (inputTokens int, outputTokens int, isDone bool, err error)

	// ParseNonStreamResponse extracts token usage from a non streaming response body.
	ParseNonStreamResponse(body []byte) (inputTokens int, outputTokens int, err error)

	// FormatBudgetExceededSSE returns the provider specific SSE error frame for mid stream budget cut.
	FormatBudgetExceededSSE() []byte
}
```

## 2. Create the Provider Directory and Files

Create a new package inside `internal/provider/myprovider/myprovider.go`:

```go
package myprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type MyProvider struct {
	baseURL string
}

func NewMyProvider() *MyProvider {
	return &MyProvider{
		baseURL: "https://api.myprovider.com",
	}
}

func (p *MyProvider) Name() string    { return "myprovider" }
func (p *MyProvider) BaseURL() string { return p.baseURL }

func (p *MyProvider) InjectAuth(req *http.Request, providerKey string) {
	req.Header.Set("Authorization", "Bearer " + providerKey)
}

func (p *MyProvider) RewritePath(originalPath string) string {
	// Strip the prefix /myprovider
	return originalPath[len("/myprovider"):]
}

func (p *MyProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	var payload struct {
		Model     string `json:"model"`
		Stream    bool   `json:"stream"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false, 0, nil, err
	}
	return payload.Model, payload.Stream, payload.MaxTokens, body, nil
}

func (p *MyProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload["model"] = fallbackModel
	return json.Marshal(payload)
}

func (p *MyProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	// Simple estimation or use tokenizer
	return len(body) / 4, nil
}

func (p *MyProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	// Parse individual streaming chunk
	return 0, 1, false, nil
}

func (p *MyProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	// Extract usage stats
	return 10, 20, nil
}

func (p *MyProvider) FormatBudgetExceededSSE() []byte {
	return []byte("data: [BUDGET EXCEEDED]\n\n")
}
```

## 3. Register the Provider

Open `internal/server/server.go`. Inside `NewServer()`, import your provider package and register it to the registry:

```go
import "github.com/loopers-oss/loopers/internal/provider/myprovider"

// ... inside NewServer()
reg.Register(myprovider.NewMyProvider())
```

## 4. Add Pricing Rules

Add your model costs to the `pricing.yaml` file:

```yaml
providers:
  myprovider:
    models:
      my-model-v1:
        input_cost_per_1k_tokens: 0.0015
        output_cost_per_1k_tokens: 0.002
```

## 5. Add Tests

Create unit tests inside `internal/provider/myprovider/myprovider_test.go` to verify:
* Request parsing works correctly.
* Model names and values are extracted properly.
* Token count estimations are accurate.
* Upstream request headers are set correctly.

Run your tests using:
```bash
go test -v ./internal/provider/myprovider/...
```
