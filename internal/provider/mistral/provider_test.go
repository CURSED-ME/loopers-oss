package mistral

import (
	"net/http"
	"testing"
)

func TestMistralProvider(t *testing.T) {
	prov := NewMistralProvider()

	if prov.Name() != "mistral" {
		t.Errorf("expected name 'mistral', got '%s'", prov.Name())
	}

	if prov.BaseURL() != "https://api.mistral.ai" {
		t.Errorf("expected base URL 'https://api.mistral.ai', got '%s'", prov.BaseURL())
	}

	// Test RewritePath
	path := "/mistral/v1/chat/completions"
	rewritten := prov.RewritePath(path)
	expected := "/v1/chat/completions"
	if rewritten != expected {
		t.Errorf("expected rewritten path '%s', got '%s'", expected, rewritten)
	}

	// Test InjectAuth
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:8080"+path, nil)
	prov.InjectAuth(req, "mistral-secret-api-key")

	authHeader := req.Header.Get("Authorization")
	expectedAuth := "Bearer mistral-secret-api-key"
	if authHeader != expectedAuth {
		t.Errorf("expected Authorization header '%s', got '%s'", expectedAuth, authHeader)
	}
}
