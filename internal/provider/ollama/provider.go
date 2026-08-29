package ollama

import (
	"context"
	"net/http"
	"strings"

	"github.com/try-loopers/loopers-oss/internal/provider"
	"github.com/try-loopers/loopers-oss/internal/provider/openai"
)

type OllamaProvider struct {
	openAI *openai.OpenAIProvider
}

func NewOllamaProvider() *OllamaProvider {
	return &OllamaProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

var _ provider.Provider = (*OllamaProvider)(nil)

func (o *OllamaProvider) Name() string    { return "ollama" }
func (o *OllamaProvider) BaseURL() string { return "http://localhost:11434/v1" }
func (o *OllamaProvider) InjectAuth(req *http.Request, providerKey string) {
	o.openAI.InjectAuth(req, providerKey)
}
func (o *OllamaProvider) RewritePath(originalPath string) string {
	return strings.TrimPrefix(originalPath, "/ollama")
}
func (o *OllamaProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	return o.openAI.ParseRequest(req, body)
}
func (o *OllamaProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return o.openAI.RewriteModel(req, body, fallbackModel)
}
func (o *OllamaProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return o.openAI.CountInputTokens(ctx, model, body, providerKey)
}
func (o *OllamaProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	return o.openAI.ParseStreamChunk(chunk)
}
func (o *OllamaProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	return o.openAI.ParseNonStreamResponse(body)
}
func (o *OllamaProvider) FormatBudgetExceededSSE() []byte {
	return o.openAI.FormatBudgetExceededSSE()
}
