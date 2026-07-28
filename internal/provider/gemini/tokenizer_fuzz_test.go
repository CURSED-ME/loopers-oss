package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// FuzzCountGeminiTokensFallback tests that token counting (both via API endpoint and local fallback)
// doesn't panic or block when provided with arbitrary byte slices (e.g. malformed JSON).
func FuzzCountGeminiTokensFallback(f *testing.F) {
	// Warm up fallback encoding once before fuzz workers start to avoid any initialization contention
	_, _ = getCL100kEncoding()

	// Add some seed corpus examples
	f.Add([]byte(`{"contents":[{"role":"user","parts":[{"text":"hello world"}]}]}`))
	f.Add([]byte(`{"contents":[]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(``))

	// Start a fast local server that mimics the Gemini countTokens endpoint.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"totalTokens":1}`))
	}))
	defer ts.Close()

	p := NewGeminiProviderWithOptions(ts.URL, ts.Client())

	f.Fuzz(func(t *testing.T, data []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Exercise both provider token count (mocked via httptest) and fallback
		_, _ = p.CountInputTokens(ctx, "gemini-3.5-flash", data, "test-key")
		_, _ = countGeminiTokensFallback(data)
	})
}
