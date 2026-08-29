# Adding a New AI Provider to Loopers

This guide provides a step-by-step walkthrough of how to add support for a new AI provider to Loopers OSS.

Loopers uses a pluggable interface design to ensure new providers can be integrated cleanly without touching the core proxy or budget engine logic.

---

## Step 1: Understand the `Provider` Interface

Every AI provider adapter in Loopers must implement the `Provider` interface located in [provider.go](file:///c:/Users/varad/loopers-oss/internal/provider/provider.go):

```go
type Provider interface {
	Name() string
	BaseURL() string
	InjectAuth(req *http.Request, providerKey string)
	RewritePath(originalPath string) string
	ParseRequest(req *http.Request, body []byte) (model string, isStream bool, maxTokens int, mutatedBody []byte, err error)
	CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error)
	ParseStreamChunk(chunk []byte) (inputTokens int, outputTokens int, isDone bool, err error)
	ParseNonStreamResponse(body []byte) (inputTokens int, outputTokens int, err error)
	FormatBudgetExceededSSE() []byte
}
```

---

## Step 2: Create the Provider Package

1. Create a new subdirectory under `internal/provider/` named after your provider (e.g., `internal/provider/cohere/`).
2. Implement your provider struct in `provider.go` within that package:

```go
package cohere

type CohereProvider struct{}

func NewCohereProvider() *CohereProvider {
	return &CohereProvider{}
}

func (p *CohereProvider) Name() string {
	return "cohere"
}

func (p *CohereProvider) BaseURL() string {
	return "https://api.cohere.com"
}
```

---

## Step 3: Implement Provider-Specific Logic

### Auth Injection & Path Rewriting

Define how credentials are sent upstream and how proxy paths map to provider API endpoints:

```go
func (p *CohereProvider) InjectAuth(req *http.Request, providerKey string) {
	req.Header.Set("Authorization", "Bearer "+providerKey)
}

func (p *CohereProvider) RewritePath(originalPath string) string {
	// E.g., "/cohere/v1/chat" -> "/v1/chat"
	return strings.TrimPrefix(originalPath, "/cohere")
}
```

### Request Parsing

Extract metadata (model name, streaming setting, max output tokens) from the incoming JSON request payload. You may also modify the body here if needed:

```go
func (p *CohereProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
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
```

### Input Token Counting

Estimate input token usage before sending the request. If the provider supports a local/remote token count endpoint, try invoking it; otherwise, fall back to a local tokenizer like `tiktoken` with a conservative buffer:

```go
func (p *CohereProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	// Try calling provider API or use tiktoken estimation:
	return len(body) / 4, nil // Example simple estimation
}
```

### Intercepting Stream Chunks

Parse SSE (Server-Sent Event) data stream frames on-the-fly to keep running counts of input/output tokens during a streaming session:

```go
func (p *CohereProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	// Parse the provider-specific event chunk
	// Return (inputTokens, outputTokens, isDone, err)
}
```

### Intercepting Non-Stream Responses

Extract final token counts directly from the completed response body:

```go
func (p *CohereProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	var resp struct {
		Meta struct {
			Billing struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"billing"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, 0, err
	}
	return resp.Meta.Billing.InputTokens, resp.Meta.Billing.OutputTokens, nil
}
```

### SSE Overrun Responses

Return the exact byte payload representing a closed stream showing a budget overrun error in the provider's standard format (e.g. OpenAI error structure):

```go
func (p *CohereProvider) FormatBudgetExceededSSE() []byte {
	return []byte("data: {\"error\": {\"message\": \"Loopers budget exceeded - stream terminated.\", \"type\": \"budget_exceeded\"}}\n\n")
}
```

---

## Step 4: Register the Provider

Register your provider instance in `NewServer` in [server.go](file:///c:/Users/varad/loopers-oss/internal/server/server.go):

```go
import "github.com/try-loopers/loopers-oss/internal/provider/cohere"

// ...
func NewServer(redisClient *budget.Client, pricingStore *pricing.Store) *Server {
	// ...
	reg := provider.NewRegistry()
	reg.Register(openai.NewOpenAIProvider())
	reg.Register(cohere.NewCohereProvider()) // Register Cohere here
	// ...
}
```

---

## Step 5: Add Provider to Whitelists

1. In [commands.go](file:///c:/Users/varad/loopers-oss/cmd/loopers/commands.go), add the provider name to the validation whitelist inside `keysCreateCmd`:
```diff
- if keyProvider != "openai" && keyProvider != "anthropic" && ... {
+ if keyProvider != "openai" && keyProvider != "cohere" && ... {
```

2. Add your provider's pricing parameters into [pricing.yaml](file:///c:/Users/varad/loopers-oss/pricing.yaml):

```yaml
providers:
  cohere:
    default_max_output_tokens: 4096
    models:
      "command-r-plus":
        input_per_1m: 3.00
        output_per_1m: 15.00
      "_fallback":
        input_per_1m: 3.00
        output_per_1m: 15.00
```

---

## Step 6: Test Your Provider

1. Write unit tests in your provider directory (e.g., `provider_test.go`) validating that response parsing and token calculations match upstream formats.
2. Run validation tools:
   ```bash
   go run cmd/pricing-validator/main.go pricing.yaml
   ```
3. Run the Go test suite:
   ```bash
   go test -v ./internal/provider/cohere/...
   ```
