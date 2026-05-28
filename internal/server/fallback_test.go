package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loopers-oss/loopers/internal/budget"
	"github.com/loopers-oss/loopers/internal/keyring"
	"github.com/loopers-oss/loopers/internal/pricing"
)

func TestFallbackRouting(t *testing.T) {
	redisAddr := os.Getenv("REDIS_TEST_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisClient, err := budget.NewClient(redisAddr, "", 0)
	if err != nil {
		t.Skip("Skipping test: Redis not running")
		return
	}
	defer redisClient.Close()

	// Create a temp pricing yaml
	yamlContent := []byte(`
providers:
  mock:
    default_max_output_tokens: 100
    models:
      expensive-model:
        input_per_1m: 10.0
        output_per_1m: 30.0
        fallback: cheap-model
      cheap-model:
        input_per_1m: 0.1
        output_per_1m: 0.3
`)
	tmpFile, _ := os.CreateTemp("", "pricing*.yaml")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write(yamlContent)
	tmpFile.Close()

	pricingStore, err := pricing.LoadStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load pricing store: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer upstream.Close()

	s := NewServer(redisClient, pricingStore)
	s.registry.Register(&mockProvider{baseURL: upstream.URL})
	s.router.POST("/mock/*path", func(c *gin.Context) {
		s.handleProxy(c, "mock")
	})

	ctx := context.Background()
	rdb := redisClient.GetUnderlyingClient()

	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)
	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":       "fallback-test-key",
		"provider":   "mock",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"active":     "true",
	})
	defer rdb.Del(ctx, "loopers:key:"+keyHash)

	// Set a budget that blocks expensive-model but allows cheap-model
	// expensive-model est cost = 50 * 10/1M + 100 * 30/1M = 0.0005 + 0.003 = 0.0035
	// cheap-model est cost = 50 * 0.1/1M + 100 * 0.3/1M = 0.000005 + 0.00003 = 0.000035
	configKey := "loopers:budget:" + keyHash + ":config"
	rdb.HSet(ctx, configKey, "minute", "0.001") // Limits expensive, allows cheap
	defer rdb.Del(ctx, configKey)
	defer rdb.Del(ctx, "loopers:spend:"+keyHash+":minute:"+time.Now().UTC().Format("2006-01-02T15:04"))

	// Make request for expensive-model
	reqBody := []byte(`{"model": "expensive-model"}`)
	req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.Header.Set("X-Loopers-Provider-Key", "dummy")

	w := httptest.NewRecorder()
	s.GetRouter().ServeHTTP(w, req)

	// The request should have been routed to the fallback, successfully reserved, and returned 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK due to fallback routing, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify the fallback header is set
	if w.Header().Get("X-Loopers-Fallback") != "cheap-model" {
		t.Errorf("Expected X-Loopers-Fallback header to be cheap-model, got %s", w.Header().Get("X-Loopers-Fallback"))
	}
}
