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
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type mockProvider struct {
	baseURL string
}

func (m *mockProvider) Name() string                                     { return "mock" }
func (m *mockProvider) BaseURL() string                                  { return m.baseURL }
func (m *mockProvider) InjectAuth(req *http.Request, providerKey string) {}
func (m *mockProvider) RewritePath(originalPath string) string           { return originalPath }
func (m *mockProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	return "mock-model", false, 100, body, nil
}
func (m *mockProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return body, nil
}
func (m *mockProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return 50, nil
}
func (m *mockProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	return 0, 0, false, nil
}
func (m *mockProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	return 50, 100, nil
}
func (m *mockProvider) FormatBudgetExceededSSE() []byte {
	return nil
}

func TestShadowMode(t *testing.T) {
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

	// Create a temp pricing yaml with non-zero prices so estimatedCost > 0 and budget check trips
	yamlContent := []byte(`
providers:
  mock:
    default_max_output_tokens: 100
    models:
      mock-model:
        input_per_1m: 10.0
        output_per_1m: 30.0
`)
	tmpFile, _ := os.CreateTemp("", "pricing*.yaml")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write(yamlContent)
	tmpFile.Close()

	pricingStore, err := pricing.LoadStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load pricing store: %v", err)
	}

	// Start a dummy upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer upstream.Close()

	s := NewServer(redisClient, pricingStore)

	// Register mock provider
	s.registry.Register(&mockProvider{baseURL: upstream.URL})
	// Setup route for mock provider manually since setupRoutes already ran
	s.router.POST("/mock/*path", func(c *gin.Context) {
		s.handleProxy(c, "mock")
	})

	ctx := context.Background()
	rdb := redisClient.GetUnderlyingClient()

	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)
	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":       "shadow-test-key",
		"provider":   "mock",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"active":     "true",
	})
	defer rdb.Del(ctx, "loopers:key:"+keyHash)

	// Set a tiny budget that will definitely fail
	configKey := "loopers:budget:" + keyHash + ":config"
	rdb.HSet(ctx, configKey, "minute", "0.000001")
	defer rdb.Del(ctx, configKey)
	defer rdb.Del(ctx, "loopers:spend:"+keyHash+":minute:"+time.Now().UTC().Format("2006-01-02T15:04"))

	// Enable Shadow Mode
	s.shadowMode = true

	// Initial metric count
	initialShadowBlocks := testutil.ToFloat64(shadowBlockedTotal.WithLabelValues("mock", "minute"))

	// Make request
	reqBody := []byte(`{"messages": []}`)
	req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.Header.Set("X-Loopers-Provider-Key", "dummy")

	w := httptest.NewRecorder()
	s.GetRouter().ServeHTTP(w, req)

	// In shadow mode, it should proceed to upstream and return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK due to shadow mode, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify metric incremented
	finalShadowBlocks := testutil.ToFloat64(shadowBlockedTotal.WithLabelValues("mock", "minute"))
	if finalShadowBlocks != initialShadowBlocks+1 {
		t.Errorf("Expected shadowBlockedTotal metric to increment, got %v (was %v)", finalShadowBlocks, initialShadowBlocks)
	}

	// Disable Shadow Mode and verify it gets blocked
	s.shadowMode = false
	req, _ = http.NewRequest("POST", "/mock/v1/chat", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.Header.Set("X-Loopers-Provider-Key", "dummy")

	w = httptest.NewRecorder()
	s.GetRouter().ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests when shadow mode is false, got %d", w.Code)
	}
}
