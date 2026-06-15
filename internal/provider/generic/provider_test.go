package generic

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestGenericProvider_Basics(t *testing.T) {
	p := NewGenericProvider("openrouter", "https://openrouter.ai/api")

	if p.Name() != "openrouter" {
		t.Errorf("Expected name to be openrouter, got %s", p.Name())
	}

	if p.BaseURL() != "https://openrouter.ai/api" {
		t.Errorf("Expected baseURL to be https://openrouter.ai/api, got %s", p.BaseURL())
	}
}

func TestGenericProvider_InjectAuth(t *testing.T) {
	p := NewGenericProvider("openrouter", "https://openrouter.ai/api")
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	p.InjectAuth(req, "test-key")
	if req.Header.Get("Authorization") != "Bearer test-key" {
		t.Errorf("Expected Authorization: Bearer test-key, got %s", req.Header.Get("Authorization"))
	}
}

func TestGenericProvider_RewritePath(t *testing.T) {
	p := NewGenericProvider("my-provider", "https://example.com")

	cases := []struct {
		input    string
		expected string
	}{
		{"/my-provider/v1/chat/completions", "/v1/chat/completions"},
		{"/my-provider", ""},
		{"/other-provider/v1/chat", "/other-provider/v1/chat"}, // no prefix match — unchanged
	}
	for _, c := range cases {
		got := p.RewritePath(c.input)
		if got != c.expected {
			t.Errorf("RewritePath(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestGenericProvider_ParseRequest(t *testing.T) {
	p := NewGenericProvider("openrouter", "https://openrouter.ai/api")
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	body := []byte(`{"model": "test-model"}`)
	model, isStream, maxTokens, _, err := p.ParseRequest(req, body)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}
	if model != "test-model" {
		t.Errorf("Expected model to be test-model, got %s", model)
	}
	if isStream {
		t.Error("Expected isStream=false")
	}
	_ = maxTokens
}

func TestGenericProvider_CountInputTokens(t *testing.T) {
	p := NewGenericProvider("openrouter", "https://openrouter.ai/api")
	body := []byte(`{"model": "gpt-4", "messages": [{"role": "user", "content": "hello world"}]}`)
	tokens, err := p.CountInputTokens(context.Background(), "gpt-4", body, "test-key")
	if err != nil {
		t.Fatalf("CountInputTokens failed: %v", err)
	}
	if tokens <= 0 {
		t.Errorf("Expected tokens > 0, got %d", tokens)
	}
}

func TestGenericProvider_ParseNonStreamResponse(t *testing.T) {
	p := NewGenericProvider("openrouter", "https://openrouter.ai/api")
	body := []byte(`{"usage": {"prompt_tokens": 10, "completion_tokens": 20}}`)
	in, out, err := p.ParseNonStreamResponse(body)
	if err != nil {
		t.Fatalf("ParseNonStreamResponse failed: %v", err)
	}
	if in != 10 || out != 20 {
		t.Errorf("Expected 10/20, got %d/%d", in, out)
	}
}

func TestGenericProvider_FormatBudgetExceededSSE(t *testing.T) {
	p := NewGenericProvider("openrouter", "https://openrouter.ai/api")
	sse := p.FormatBudgetExceededSSE()
	if len(sse) == 0 {
		t.Error("Expected non-empty SSE error frame")
	}
}
