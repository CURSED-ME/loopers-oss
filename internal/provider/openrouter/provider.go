package openrouter

import (
	"context"
	"net/http"
	"strings"

	"github.com/CURSED-ME/loopers-oss/internal/provider"
	"github.com/CURSED-ME/loopers-oss/internal/provider/openai"
)

type OpenRouterProvider struct {
	openAI *openai.OpenAIProvider
}

// NewOpenRouterProvider creates a new instance of OpenRouterProvider.
func NewOpenRouterProvider() *OpenRouterProvider {
	return &OpenRouterProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

// Ensure OpenRouterProvider implements provider.Provider.
var _ provider.Provider = (*OpenRouterProvider)(nil)

func (o *OpenRouterProvider) Name() string {
	return "openrouter"
}

func (o *OpenRouterProvider) BaseURL() string {
	return "https://openrouter.ai/api"
}

func (o *OpenRouterProvider) InjectAuth(req *http.Request, providerKey string) {
	o.openAI.InjectAuth(req, providerKey)
}

func (o *OpenRouterProvider) RewritePath(originalPath string) string {
	path := strings.TrimPrefix(originalPath, "/openrouter")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if strings.HasPrefix(path, "/api") {
		return path
	}
	return "/api" + path
}

func (o *OpenRouterProvider) ParseRequest(req *http.Request, body []byte) (model string, isStream bool, maxTokens int, mutatedBody []byte, err error) {
	return o.openAI.ParseRequest(req, body)
}

func (o *OpenRouterProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return o.openAI.RewriteModel(req, body, fallbackModel)
}

func (o *OpenRouterProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return o.openAI.CountInputTokens(ctx, model, body, providerKey)
}

func (o *OpenRouterProvider) ParseStreamChunk(chunk []byte) (inputTokens int, outputTokens int, isDone bool, err error) {
	return o.openAI.ParseStreamChunk(chunk)
}

func (o *OpenRouterProvider) ParseNonStreamResponse(body []byte) (inputTokens int, outputTokens int, err error) {
	return o.openAI.ParseNonStreamResponse(body)
}

func (o *OpenRouterProvider) FormatBudgetExceededSSE() []byte {
	return o.openAI.FormatBudgetExceededSSE()
}
