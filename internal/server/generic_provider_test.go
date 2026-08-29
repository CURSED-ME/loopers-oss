package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/try-loopers/loopers-oss/internal/budget"
	"github.com/try-loopers/loopers-oss/internal/keyring"
	"github.com/try-loopers/loopers-oss/internal/pricing"
	"github.com/alicebob/miniredis/v2"
	"github.com/spf13/viper"
)

func setupTestServerWithGenericProviders(t *testing.T, genericProvidersCfg interface{}) (*Server, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	redisClient, err := budget.NewClient(mr.Addr(), "", 0)
	if err != nil {
		mr.Close()
		t.Fatalf("Failed to create redis client: %v", err)
	}

	tmpFile, _ := os.CreateTemp("", "pricing*.yaml")
	tmpFile.Write([]byte(`providers: {}`))
	tmpFile.Close()
	pricingStore, _ := pricing.LoadStore(tmpFile.Name())

	viper.Set("generic_providers", genericProvidersCfg)
	viper.Set("testing.allow_private_urls", true)

	s := NewServer(redisClient, pricingStore)
	defer s.Shutdown()
	cleanup := func() {
		mr.Close()
		redisClient.Close()
		os.Remove(tmpFile.Name())
		viper.Set("generic_providers", nil)
	}
	return s, cleanup
}

// TestGenericProvider_ValidRegistration ensures a well-formed generic provider is registered and routable.
func TestGenericProvider_ValidRegistration(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"role":"assistant","content":"hi"}}],"usage":{"prompt_tokens":5,"completion_tokens":3}}`))
	}))
	defer upstream.Close()

	mr, _ := miniredis.Run()
	defer mr.Close()
	redisClient, _ := budget.NewClient(mr.Addr(), "", 0)
	defer redisClient.Close()
	tmpFile, _ := os.CreateTemp("", "pricing*.yaml")
	tmpFile.Write([]byte(`
providers:
  my-upstream:
    default_max_output_tokens: 100
    models:
      my-model:
        input_per_1m: 1.0
        output_per_1m: 2.0
`))
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	pricingStore, _ := pricing.LoadStore(tmpFile.Name())

	viper.Set("generic_providers", []map[string]interface{}{
		{"name": "my-upstream", "base_url": upstream.URL},
	})
	viper.Set("testing.allow_private_urls", true)
	defer viper.Set("generic_providers", nil)
	defer viper.Set("testing.allow_private_urls", false)

	s := NewServer(redisClient, pricingStore)
	defer s.Shutdown()

	ctx := context.Background()
	rdb := redisClient.GetUnderlyingClient()
	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)
	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":       "generic-test-key",
		"provider":   "my-upstream",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"active":     "true",
	})
	defer rdb.Del(ctx, "loopers:key:"+keyHash)

	reqBody := []byte(`{"model": "my-model", "messages": [{"role": "user", "content": "hi"}]}`)
	req, _ := http.NewRequest("POST", "/my-upstream/v1/chat/completions", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.Header.Set("X-Loopers-Provider-Key", "dummy-upstream-key")

	w := newCloseNotifierRecorder()
	s.GetRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for generic provider request, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestGenericProvider_BuiltinNameCollision ensures a generic provider cannot shadow a built-in (S3).
func TestGenericProvider_BuiltinNameCollision(t *testing.T) {
	s, cleanup := setupTestServerWithGenericProviders(t, []map[string]interface{}{
		{"name": "openai", "base_url": "https://attacker.example.com"},
	})
	defer cleanup()

	// The registry should still return the real openai provider, not the attacker's one
	p, err := s.GetRegistry().Get("openai")
	if err != nil {
		t.Fatalf("Expected openai to be registered (built-in), got error: %v", err)
	}
	if p.BaseURL() != "https://api.openai.com" {
		t.Errorf("openai was shadowed by generic provider! BaseURL is now %s", p.BaseURL())
	}
}

// TestGenericProvider_DuplicateName ensures duplicate generic provider names are rejected (S1).
func TestGenericProvider_DuplicateName(t *testing.T) {
	// Two entries with the same name — only the first should be registered.
	upstream1 := "https://legit.example.com"
	upstream2 := "https://shadow.example.com"

	s, cleanup := setupTestServerWithGenericProviders(t, []map[string]interface{}{
		{"name": "my-co", "base_url": upstream1},
		{"name": "my-co", "base_url": upstream2},
	})
	defer cleanup()

	p, err := s.GetRegistry().Get("my-co")
	if err != nil {
		t.Fatalf("Expected my-co to be registered, got: %v", err)
	}
	if p.BaseURL() != upstream1 {
		t.Errorf("Expected first registration (%s) to win, got %s", upstream1, p.BaseURL())
	}
}

// TestGenericProvider_InvalidName ensures names with special chars are rejected (G1).
func TestGenericProvider_InvalidName(t *testing.T) {
	invalidNames := []string{
		"my/provider",
		"my provider",
		"my.provider",
		"../evil",
		"my:provider",
	}
	for _, name := range invalidNames {
		s, cleanup := setupTestServerWithGenericProviders(t, []map[string]interface{}{
			{"name": name, "base_url": "https://example.com"},
		})
		_, err := s.GetRegistry().Get(name)
		if err == nil {
			t.Errorf("Expected invalid provider name %q to be rejected, but it was registered", name)
		}
		cleanup()
	}
}
