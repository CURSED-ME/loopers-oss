package vllm

import (
	"context"
	"net/http"
	"strings"

	"github.com/CURSED-ME/loopers-oss/internal/provider"
	"github.com/CURSED-ME/loopers-oss/internal/provider/openai"
)

type VLLMProvider struct {
	openAI *openai.OpenAIProvider
}

func NewVLLMProvider() *VLLMProvider {
	return &VLLMProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

var _ provider.Provider = (*VLLMProvider)(nil)

func (v *VLLMProvider) Name() string    { return "vllm" }
func (v *VLLMProvider) BaseURL() string { return "http://localhost:8000/v1" }
func (v *VLLMProvider) InjectAuth(req *http.Request, providerKey string) {
	v.openAI.InjectAuth(req, providerKey)
}
func (v *VLLMProvider) RewritePath(originalPath string) string {
	return strings.TrimPrefix(originalPath, "/vllm")
}
func (v *VLLMProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	return v.openAI.ParseRequest(req, body)
}
func (v *VLLMProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return v.openAI.RewriteModel(req, body, fallbackModel)
}
func (v *VLLMProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return v.openAI.CountInputTokens(ctx, model, body, providerKey)
}
func (v *VLLMProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	return v.openAI.ParseStreamChunk(chunk)
}
func (v *VLLMProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	return v.openAI.ParseNonStreamResponse(body)
}
func (v *VLLMProvider) FormatBudgetExceededSSE() []byte {
	return v.openAI.FormatBudgetExceededSSE()
}
