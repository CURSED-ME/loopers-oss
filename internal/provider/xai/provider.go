package xai

import (
	"context"
	"net/http"
	"strings"

	"github.com/try-loopers/loopers-oss/internal/provider"
	"github.com/try-loopers/loopers-oss/internal/provider/openai"
)

type XAIProvider struct {
	openAI *openai.OpenAIProvider
}

func NewXAIProvider() *XAIProvider {
	return &XAIProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

var _ provider.Provider = (*XAIProvider)(nil)

func (x *XAIProvider) Name() string    { return "xai" }
func (x *XAIProvider) BaseURL() string { return "https://api.x.ai/v1" }
func (x *XAIProvider) InjectAuth(req *http.Request, providerKey string) {
	x.openAI.InjectAuth(req, providerKey)
}
func (x *XAIProvider) RewritePath(originalPath string) string {
	return strings.TrimPrefix(originalPath, "/xai")
}
func (x *XAIProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	return x.openAI.ParseRequest(req, body)
}
func (x *XAIProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return x.openAI.RewriteModel(req, body, fallbackModel)
}
func (x *XAIProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return x.openAI.CountInputTokens(ctx, model, body, providerKey)
}
func (x *XAIProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	return x.openAI.ParseStreamChunk(chunk)
}
func (x *XAIProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	return x.openAI.ParseNonStreamResponse(body)
}
func (x *XAIProvider) FormatBudgetExceededSSE() []byte {
	return x.openAI.FormatBudgetExceededSSE()
}
