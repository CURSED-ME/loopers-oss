package gemini

import (
	"context"
	"net/http"
	"testing"
)

func TestGeminiParseRequest(t *testing.T) {
	prov := NewGeminiProvider()

	// 1. Non-streaming request
	req1, _ := http.NewRequest(http.MethodPost, "http://localhost:8080/gemini/v1beta/models/gemini-3.5-flash:generateContent", nil)
	body1 := []byte(`{"contents":[{"parts":[{"text":"hello"}]}],"generationConfig":{"maxOutputTokens":1024}}`)

	model, isStream, maxTokens, _, err := prov.ParseRequest(req1, body1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != "gemini-3.5-flash" {
		t.Errorf("expected model 'gemini-3.5-flash', got '%s'", model)
	}
	if isStream {
		t.Error("expected non-streaming request")
	}
	if maxTokens != 1024 {
		t.Errorf("expected maxTokens 1024, got %d", maxTokens)
	}

	// 2. Streaming request
	req2, _ := http.NewRequest(http.MethodPost, "http://localhost:8080/gemini/v1beta/models/gemini-3.5-pro:streamGenerateContent?alt=sse", nil)
	body2 := []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`)

	model, isStream, maxTokens, _, err = prov.ParseRequest(req2, body2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != "gemini-3.5-pro" {
		t.Errorf("expected model 'gemini-3.5-pro', got '%s'", model)
	}
	if !isStream {
		t.Error("expected streaming request")
	}
	if maxTokens != 0 {
		t.Errorf("expected maxTokens 0, got %d", maxTokens)
	}
}

func TestGeminiInjectAuth(t *testing.T) {
	prov := NewGeminiProvider()
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:8080/gemini/v1beta/models/gemini-3.5-flash:generateContent", nil)

	prov.InjectAuth(req, "test-api-key")

	q := req.URL.Query()
	key := q.Get("key")
	if key != "test-api-key" {
		t.Errorf("expected query key 'test-api-key', got '%s'", key)
	}
}

func TestGeminiRewritePath(t *testing.T) {
	prov := NewGeminiProvider()
	rewritten := prov.RewritePath("/gemini/v1beta/models/gemini-3.5-flash:generateContent")
	expected := "/v1beta/models/gemini-3.5-flash:generateContent"
	if rewritten != expected {
		t.Errorf("expected rewritten path '%s', got '%s'", expected, rewritten)
	}
}

func TestGeminiParseStreamChunk(t *testing.T) {
	prov := NewGeminiProvider()

	// 1. JSON array streaming chunk (starts with [ or ,)
	chunk1 := []byte(`[{"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":10}}]`)
	inTokens, outTokens, isDone, err := prov.ParseStreamChunk(chunk1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inTokens != 5 {
		t.Errorf("expected inTokens 5, got %d", inTokens)
	}
	// candidate tokens = 10 * 1.10 = 11
	if outTokens != 11 {
		t.Errorf("expected outTokens 11, got %d", outTokens)
	}
	if isDone {
		t.Error("expected isDone to be false")
	}

	// 2. SSE formatted chunk
	chunk2 := []byte(`data: {"candidates":[{"content":{"parts":[{"text":" world"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":15}}`)
	inTokens, outTokens, isDone, err = prov.ParseStreamChunk(chunk2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inTokens != 5 {
		t.Errorf("expected inTokens 5, got %d", inTokens)
	}
	// candidate tokens = 15 * 1.10 = 16 (int truncation of 16.5 is 16)
	if outTokens != 16 {
		t.Errorf("expected outTokens 16, got %d", outTokens)
	}
}

func TestGeminiParseNonStreamResponse(t *testing.T) {
	prov := NewGeminiProvider()
	body := []byte(`{"candidates":[],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":12}}`)

	inTokens, outTokens, err := prov.ParseNonStreamResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inTokens != 8 {
		t.Errorf("expected inTokens 8, got %d", inTokens)
	}
	if outTokens != 12 {
		t.Errorf("expected outTokens 12, got %d", outTokens)
	}
}

func TestGeminiCountInputTokensFallback(t *testing.T) {
	prov := NewGeminiProvider()
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello world"}]}]}`)

	tokens, err := prov.CountInputTokens(context.Background(), "gemini-3.5-flash", body, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "hello world" in cl100k_base has 2 tokens
	// 7 * 1.15 = 8.05 -> int(8.05) = 8
	if tokens != 8 {
		t.Errorf("expected estimated tokens 8, got %d", tokens)
	}
}
