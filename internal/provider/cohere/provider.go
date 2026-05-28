package cohere

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/loopers-oss/loopers/internal/provider"
	"github.com/loopers-oss/loopers/internal/provider/openai"
)

type CohereProvider struct {
	openAI *openai.OpenAIProvider
}

func NewCohereProvider() *CohereProvider {
	return &CohereProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

var _ provider.Provider = (*CohereProvider)(nil)

func (c *CohereProvider) Name() string { return "cohere" }
func (c *CohereProvider) BaseURL() string { return "https://api.cohere.com/v1" }

func (c *CohereProvider) InjectAuth(req *http.Request, providerKey string) {
	req.Header.Set("Authorization", "Bearer "+providerKey)
}

func (c *CohereProvider) RewritePath(originalPath string) string {
	return strings.TrimPrefix(originalPath, "/cohere")
}

func (c *CohereProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	return c.openAI.ParseRequest(req, body)
}

func (c *CohereProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return c.openAI.RewriteModel(req, body, fallbackModel)
}

func (c *CohereProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return c.openAI.CountInputTokens(ctx, model, body, providerKey)
}

func (c *CohereProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	return c.openAI.ParseStreamChunk(chunk)
}

func (c *CohereProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	// First try OpenAI format (in case they use the compatible endpoint)
	in, out, _ := c.openAI.ParseNonStreamResponse(body)
	if in > 0 || out > 0 {
		return in, out, nil
	}

	// Try Cohere native format
	var respObj struct {
		Meta struct {
			BilledUnits struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"billed_units"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &respObj); err == nil {
		return respObj.Meta.BilledUnits.InputTokens, respObj.Meta.BilledUnits.OutputTokens, nil
	}

	return 0, 0, nil
}

func (c *CohereProvider) FormatBudgetExceededSSE() []byte {
	return c.openAI.FormatBudgetExceededSSE()
}
