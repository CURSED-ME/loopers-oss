package mistral

import (
	"context"
	"net/http"
	"strings"

	"github.com/loopers-oss/loopers/internal/provider"
	"github.com/loopers-oss/loopers/internal/provider/openai"
)

type MistralProvider struct {
	openAI *openai.OpenAIProvider
}

// NewMistralProvider creates a new instance of MistralProvider.
func NewMistralProvider() *MistralProvider {
	return &MistralProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

// Ensure MistralProvider implements provider.Provider.
var _ provider.Provider = (*MistralProvider)(nil)

func (m *MistralProvider) Name() string {
	return "mistral"
}

func (m *MistralProvider) BaseURL() string {
	return "https://api.mistral.ai"
}

func (m *MistralProvider) InjectAuth(req *http.Request, providerKey string) {
	m.openAI.InjectAuth(req, providerKey)
}

func (m *MistralProvider) RewritePath(originalPath string) string {
	// Path in Loopers: /mistral/v1/chat/completions -> Path in Upstream: /v1/chat/completions
	return strings.TrimPrefix(originalPath, "/mistral")
}

func (m *MistralProvider) ParseRequest(req *http.Request, body []byte) (model string, isStream bool, maxTokens int, mutatedBody []byte, err error) {
	return m.openAI.ParseRequest(req, body)
}

func (m *MistralProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return m.openAI.CountInputTokens(ctx, model, body, providerKey)
}

func (m *MistralProvider) ParseStreamChunk(chunk []byte) (inputTokens int, outputTokens int, isDone bool, err error) {
	return m.openAI.ParseStreamChunk(chunk)
}

func (m *MistralProvider) ParseNonStreamResponse(body []byte) (inputTokens int, outputTokens int, err error) {
	return m.openAI.ParseNonStreamResponse(body)
}

func (m *MistralProvider) FormatBudgetExceededSSE() []byte {
	return m.openAI.FormatBudgetExceededSSE()
}
