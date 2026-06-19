package generic

import (
	"context"
	"net/http"
	"strings"

	"github.com/CURSED-ME/loopers-oss/internal/provider"
	"github.com/CURSED-ME/loopers-oss/internal/provider/openai"
)

// GenericProvider wraps the OpenAI provider to support any OpenAI-compatible API.
type GenericProvider struct {
	name    string
	baseURL string
	openai  provider.Provider
}

// NewGenericProvider creates a new generic provider.
func NewGenericProvider(name, baseURL string) *GenericProvider {
	return &GenericProvider{
		name:    name,
		baseURL: baseURL,
		openai:  openai.NewOpenAIProvider(),
	}
}

var _ provider.Provider = (*GenericProvider)(nil)

func (p *GenericProvider) Name() string {
	return p.name
}

func (p *GenericProvider) BaseURL() string {
	return p.baseURL
}

func (p *GenericProvider) InjectAuth(req *http.Request, providerKey string) {
	p.openai.InjectAuth(req, providerKey)
}

func (p *GenericProvider) RewritePath(originalPath string) string {
	return strings.TrimPrefix(originalPath, "/"+p.name)
}

func (p *GenericProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	return p.openai.ParseRequest(req, body)
}

func (p *GenericProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return p.openai.RewriteModel(req, body, fallbackModel)
}

func (p *GenericProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return p.openai.CountInputTokens(ctx, model, body, providerKey)
}

func (p *GenericProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	return p.openai.ParseStreamChunk(chunk)
}

func (p *GenericProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	return p.openai.ParseNonStreamResponse(body)
}

func (p *GenericProvider) FormatBudgetExceededSSE() []byte {
	return p.openai.FormatBudgetExceededSSE()
}
