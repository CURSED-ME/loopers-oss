package openrouter

import (
	"net/http"
	"testing"
)

func TestOpenRouterProvider(t *testing.T) {
	prov := NewOpenRouterProvider()

	if prov.Name() != "openrouter" {
		t.Errorf("expected name 'openrouter', got '%s'", prov.Name())
	}

	if prov.BaseURL() != "https://openrouter.ai/api" {
		t.Errorf("expected base URL 'https://openrouter.ai/api', got '%s'", prov.BaseURL())
	}

	// Test RewritePath standard
	path := "/openrouter/v1/chat/completions"
	rewritten := prov.RewritePath(path)
	expected := "/api/v1/chat/completions"
	if rewritten != expected {
		t.Errorf("expected rewritten path '%s', got '%s'", expected, rewritten)
	}

	// Test RewritePath when user includes /api in request
	pathWithAPI := "/openrouter/api/v1/chat/completions"
	rewrittenAPI := prov.RewritePath(pathWithAPI)
	if rewrittenAPI != expected {
		t.Errorf("expected rewritten path '%s', got '%s'", expected, rewrittenAPI)
	}

	// Test InjectAuth
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:8080"+path, nil)
	prov.InjectAuth(req, "sk-or-v1-secret-key")

	authHeader := req.Header.Get("Authorization")
	expectedAuth := "Bearer sk-or-v1-secret-key"
	if authHeader != expectedAuth {
		t.Errorf("expected Authorization header '%s', got '%s'", expectedAuth, authHeader)
	}
}
