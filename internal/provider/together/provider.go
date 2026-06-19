package together

import (
	"context"
	"net/http"
	"strings"

	"github.com/CURSED-ME/loopers-oss/internal/provider"
	"github.com/CURSED-ME/loopers-oss/internal/provider/openai"
)

type TogetherProvider struct {
	openAI *openai.OpenAIProvider
}

func NewTogetherProvider() *TogetherProvider {
	return &TogetherProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

var _ provider.Provider = (*TogetherProvider)(nil)

func (t *TogetherProvider) Name() string    { return "together" }
func (t *TogetherProvider) BaseURL() string { return "https://api.together.xyz/v1" }
func (t *TogetherProvider) InjectAuth(req *http.Request, providerKey string) {
	t.openAI.InjectAuth(req, providerKey)
}
func (t *TogetherProvider) RewritePath(originalPath string) string {
	return strings.TrimPrefix(originalPath, "/together")
}
func (t *TogetherProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	return t.openAI.ParseRequest(req, body)
}
func (t *TogetherProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return t.openAI.RewriteModel(req, body, fallbackModel)
}
func (t *TogetherProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return t.openAI.CountInputTokens(ctx, model, body, providerKey)
}
func (t *TogetherProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	return t.openAI.ParseStreamChunk(chunk)
}
func (t *TogetherProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	return t.openAI.ParseNonStreamResponse(body)
}
func (t *TogetherProvider) FormatBudgetExceededSSE() []byte {
	return t.openAI.FormatBudgetExceededSSE()
}
