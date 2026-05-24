package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/loopers-oss/loopers/internal/logging"
	"github.com/loopers-oss/loopers/internal/provider"
)

type GeminiProvider struct{}

// NewGeminiProvider creates a new instance of GeminiProvider.
func NewGeminiProvider() *GeminiProvider {
	return &GeminiProvider{}
}

// Ensure GeminiProvider implements provider.Provider.
var _ provider.Provider = (*GeminiProvider)(nil)

func (g *GeminiProvider) Name() string {
	return "gemini"
}

func (g *GeminiProvider) BaseURL() string {
	return "https://generativelanguage.googleapis.com"
}

func (g *GeminiProvider) InjectAuth(req *http.Request, providerKey string) {
	q := req.URL.Query()
	q.Set("key", providerKey)
	req.URL.RawQuery = q.Encode()
}

func (g *GeminiProvider) RewritePath(originalPath string) string {
	return strings.TrimPrefix(originalPath, "/gemini")
}

func (g *GeminiProvider) ParseRequest(req *http.Request, body []byte) (model string, isStream bool, maxTokens int, mutatedBody []byte, err error) {
	path := req.URL.Path

	// Check if this is a streaming request based on path
	if strings.Contains(path, ":streamGenerateContent") {
		isStream = true
	}

	// Extract the model name from URL path, e.g. "/gemini/v1beta/models/gemini-3.5-flash:generateContent"
	idx := strings.Index(path, "models/")
	if idx != -1 {
		start := idx + len("models/")
		end := strings.Index(path[start:], ":")
		if end != -1 {
			model = path[start : start+end]
		} else {
			model = path[start:]
		}
	}

	// Extract max output tokens from generationConfig if specified
	var payload struct {
		GenerationConfig struct {
			MaxOutputTokens int `json:"maxOutputTokens"`
		} `json:"generationConfig"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		maxTokens = payload.GenerationConfig.MaxOutputTokens
	}

	return model, isStream, maxTokens, body, nil
}

func (g *GeminiProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	// 1. Try Gemini's countTokens API
	tokens, err := callGeminiCountTokensAPI(ctx, model, body, providerKey)
	if err == nil {
		return tokens, nil
	}

	logging.Logger.Warn().Err(err).Msg("Gemini countTokens API failed, falling back to local estimation")

	// 2. Fallback to local tiktoken estimation
	return countGeminiTokensFallback(body)
}

func (g *GeminiProvider) ParseStreamChunk(chunk []byte) (inputTokens int, outputTokens int, isDone bool, err error) {
	return parseGeminiStreamChunk(chunk)
}

func (g *GeminiProvider) ParseNonStreamResponse(body []byte) (inputTokens int, outputTokens int, err error) {
	var payload struct {
		UsageMetadata struct {
			PromptTokens     int `json:"promptTokenCount"`
			CandidatesTokens int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, 0, err
	}
	return payload.UsageMetadata.PromptTokens, payload.UsageMetadata.CandidatesTokens, nil
}

func (g *GeminiProvider) FormatBudgetExceededSSE() []byte {
	return formatGeminiBudgetExceededSSE()
}
