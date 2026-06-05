package fireworks

import (
	"context"
	"net/http"
	"strings"

	"github.com/loopers-oss/loopers/internal/provider"
	"github.com/loopers-oss/loopers/internal/provider/openai"
)

type FireworksProvider struct {
	openAI *openai.OpenAIProvider
}

func NewFireworksProvider() *FireworksProvider {
	return &FireworksProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

var _ provider.Provider = (*FireworksProvider)(nil)

func (f *FireworksProvider) Name() string    { return "fireworks" }
func (f *FireworksProvider) BaseURL() string { return "https://api.fireworks.ai/inference/v1" }
func (f *FireworksProvider) InjectAuth(req *http.Request, providerKey string) {
	f.openAI.InjectAuth(req, providerKey)
}
func (f *FireworksProvider) RewritePath(originalPath string) string {
	return strings.TrimPrefix(originalPath, "/fireworks")
}
func (f *FireworksProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	return f.openAI.ParseRequest(req, body)
}
func (f *FireworksProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return f.openAI.RewriteModel(req, body, fallbackModel)
}
func (f *FireworksProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return f.openAI.CountInputTokens(ctx, model, body, providerKey)
}
func (f *FireworksProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	return f.openAI.ParseStreamChunk(chunk)
}
func (f *FireworksProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	return f.openAI.ParseNonStreamResponse(body)
}
func (f *FireworksProvider) FormatBudgetExceededSSE() []byte {
	return f.openAI.FormatBudgetExceededSSE()
}
