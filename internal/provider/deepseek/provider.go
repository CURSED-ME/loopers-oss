package deepseek

import (
	"context"
	"net/http"
	"strings"

	"github.com/try-loopers/loopers-oss/internal/provider"
	"github.com/try-loopers/loopers-oss/internal/provider/openai"
)

type DeepSeekProvider struct {
	openAI *openai.OpenAIProvider
}

func NewDeepSeekProvider() *DeepSeekProvider {
	return &DeepSeekProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

var _ provider.Provider = (*DeepSeekProvider)(nil)

func (d *DeepSeekProvider) Name() string    { return "deepseek" }
func (d *DeepSeekProvider) BaseURL() string { return "https://api.deepseek.com" }
func (d *DeepSeekProvider) InjectAuth(req *http.Request, providerKey string) {
	d.openAI.InjectAuth(req, providerKey)
}
func (d *DeepSeekProvider) RewritePath(originalPath string) string {
	return strings.TrimPrefix(originalPath, "/deepseek")
}
func (d *DeepSeekProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	return d.openAI.ParseRequest(req, body)
}
func (d *DeepSeekProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return d.openAI.RewriteModel(req, body, fallbackModel)
}
func (d *DeepSeekProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return d.openAI.CountInputTokens(ctx, model, body, providerKey)
}
func (d *DeepSeekProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	return d.openAI.ParseStreamChunk(chunk)
}
func (d *DeepSeekProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	return d.openAI.ParseNonStreamResponse(body)
}
func (d *DeepSeekProvider) FormatBudgetExceededSSE() []byte {
	return d.openAI.FormatBudgetExceededSSE()
}
