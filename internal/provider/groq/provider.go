package groq

import (
	"context"
	"net/http"
	"strings"

	"github.com/loopers-oss/loopers/internal/provider"
	"github.com/loopers-oss/loopers/internal/provider/openai"
)

type GroqProvider struct {
	openAI *openai.OpenAIProvider
}

func NewGroqProvider() *GroqProvider {
	return &GroqProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

var _ provider.Provider = (*GroqProvider)(nil)

func (g *GroqProvider) Name() string { return "groq" }
func (g *GroqProvider) BaseURL() string { return "https://api.groq.com/openai" }
func (g *GroqProvider) InjectAuth(req *http.Request, providerKey string) { g.openAI.InjectAuth(req, providerKey) }
func (g *GroqProvider) RewritePath(originalPath string) string { return strings.TrimPrefix(originalPath, "/groq") }
func (g *GroqProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) { return g.openAI.ParseRequest(req, body) }
func (g *GroqProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) { return g.openAI.RewriteModel(req, body, fallbackModel) }
func (g *GroqProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) { return g.openAI.CountInputTokens(ctx, model, body, providerKey) }
func (g *GroqProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) { return g.openAI.ParseStreamChunk(chunk) }
func (g *GroqProvider) ParseNonStreamResponse(body []byte) (int, int, error) { return g.openAI.ParseNonStreamResponse(body) }
func (g *GroqProvider) FormatBudgetExceededSSE() []byte { return g.openAI.FormatBudgetExceededSSE() }
