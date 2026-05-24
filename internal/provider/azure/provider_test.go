package azure

import (
	"net/http"
	"testing"
)

func TestAzureProvider(t *testing.T) {
	prov := NewAzureProvider()

	if prov.Name() != "azure" {
		t.Errorf("expected name 'azure', got '%s'", prov.Name())
	}

	// Test RewritePath
	path := "/azure/openai/deployments/my-gpt/chat/completions"
	rewritten := prov.RewritePath(path)
	expected := "/openai/deployments/my-gpt/chat/completions"
	if rewritten != expected {
		t.Errorf("expected rewritten path '%s', got '%s'", expected, rewritten)
	}

	// Test InjectAuth with custom endpoint and API version headers
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:8080"+path, nil)
	req.Header.Set("X-Loopers-Azure-Endpoint", "https://myresource.openai.azure.com")
	req.Header.Set("X-Loopers-Azure-API-Version", "2024-10-21")
	req.Header.Set("Authorization", "Bearer lp-key")

	prov.InjectAuth(req, "azure-secret-api-key")

	// Host should be rewritten
	if req.Host != "myresource.openai.azure.com" {
		t.Errorf("expected host 'myresource.openai.azure.com', got '%s'", req.Host)
	}
	if req.URL.Host != "myresource.openai.azure.com" {
		t.Errorf("expected URL host 'myresource.openai.azure.com', got '%s'", req.URL.Host)
	}

	// Auth header should be api-key, and Bearer authorization should be removed
	apiKey := req.Header.Get("api-key")
	if apiKey != "azure-secret-api-key" {
		t.Errorf("expected api-key 'azure-secret-api-key', got '%s'", apiKey)
	}
	if req.Header.Get("Authorization") != "" {
		t.Error("expected Authorization header to be deleted")
	}

	// api-version query param should be injected
	q := req.URL.Query()
	apiVersion := q.Get("api-version")
	if apiVersion != "2024-10-21" {
		t.Errorf("expected api-version query parameter '2024-10-21', got '%s'", apiVersion)
	}
}
