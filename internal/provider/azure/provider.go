package azure

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/loopers-oss/loopers/internal/provider"
	"github.com/loopers-oss/loopers/internal/provider/openai"
)

type AzureProvider struct {
	openAI *openai.OpenAIProvider
}

// NewAzureProvider creates a new instance of AzureProvider.
func NewAzureProvider() *AzureProvider {
	return &AzureProvider{
		openAI: openai.NewOpenAIProvider(),
	}
}

// Ensure AzureProvider implements provider.Provider.
var _ provider.Provider = (*AzureProvider)(nil)

func (az *AzureProvider) Name() string {
	return "azure"
}

func (az *AzureProvider) BaseURL() string {
	// Dynamically overwritten in InjectAuth, default to placeholder
	return "https://api.openai.azure.com"
}

func (az *AzureProvider) InjectAuth(req *http.Request, providerKey string) {
	// Extract resource endpoint from headers
	endpoint := req.Header.Get("X-Loopers-Azure-Endpoint")
	req.Header.Del("X-Loopers-Azure-Endpoint")

	if endpoint != "" {
		target, err := url.Parse(endpoint)
		if err == nil {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		}
	}

	// Set api-key header and remove bearer auth if set
	req.Header.Set("api-key", providerKey)
	req.Header.Del("Authorization")

	// Inject api-version if passed as custom header
	apiVersion := req.Header.Get("X-Loopers-Azure-API-Version")
	req.Header.Del("X-Loopers-Azure-API-Version")
	if apiVersion != "" {
		q := req.URL.Query()
		q.Set("api-version", apiVersion)
		req.URL.RawQuery = q.Encode()
	}
}

func (az *AzureProvider) RewritePath(originalPath string) string {
	// Path in Loopers: /azure/openai/deployments/{deployment}/chat/completions
	// -> Path in Upstream: /openai/deployments/{deployment}/chat/completions
	return strings.TrimPrefix(originalPath, "/azure")
}

func (az *AzureProvider) ParseRequest(req *http.Request, body []byte) (model string, isStream bool, maxTokens int, mutatedBody []byte, err error) {
	return az.openAI.ParseRequest(req, body)
}

func (az *AzureProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	// Rewrite JSON body
	newBody, err := az.openAI.RewriteModel(req, body, fallbackModel)
	if err != nil {
		return body, err
	}
	
	// Rewrite URL path /deployments/{deployment}/ -> /deployments/{fallbackModel}/
	path := req.URL.Path
	idx := strings.Index(path, "/deployments/")
	if idx != -1 {
		start := idx + len("/deployments/")
		end := strings.Index(path[start:], "/")
		if end != -1 {
			req.URL.Path = path[:start] + fallbackModel + path[start+end:]
		}
	}
	
	return newBody, nil
}

func (az *AzureProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return az.openAI.CountInputTokens(ctx, model, body, providerKey)
}

func (az *AzureProvider) ParseStreamChunk(chunk []byte) (inputTokens int, outputTokens int, isDone bool, err error) {
	return az.openAI.ParseStreamChunk(chunk)
}

func (az *AzureProvider) ParseNonStreamResponse(body []byte) (inputTokens int, outputTokens int, err error) {
	return az.openAI.ParseNonStreamResponse(body)
}

func (az *AzureProvider) FormatBudgetExceededSSE() []byte {
	return az.openAI.FormatBudgetExceededSSE()
}
