package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/loopers-oss/loopers/internal/budget"
	"github.com/loopers-oss/loopers/internal/keyring"
	"github.com/loopers-oss/loopers/internal/pricing"
)

type closeNotifierRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func newCloseNotifierRecorder() *closeNotifierRecorder {
	return &closeNotifierRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closed:           make(chan bool, 1),
	}
}

func (c *closeNotifierRecorder) CloseNotify() <-chan bool {
	return c.closed
}

func TestHealthEndpoint(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient, err := budget.NewClient(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("Failed to create redis client: %v", err)
	}
	defer redisClient.Close()

	pricingStore, err := pricing.LoadStore("../../pricing.yaml")
	if err != nil {
		t.Fatalf("Failed to load pricing store: %v", err)
	}

	s := NewServer(redisClient, pricingStore)
	r := s.GetRouter()

	req, _ := http.NewRequest("GET", "/health", nil)
	w := newCloseNotifierRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected health status 200, got %d", w.Code)
	}

	var resp map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", resp["status"])
	}
}

func TestBudgetStatusEndpoint(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient, err := budget.NewClient(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("Failed to create redis client: %v", err)
	}
	defer redisClient.Close()

	pricingStore, err := pricing.LoadStore("../../pricing.yaml")
	if err != nil {
		t.Fatalf("Failed to load pricing store: %v", err)
	}
	s := NewServer(redisClient, pricingStore)
	r := s.GetRouter()

	ctx := context.Background()
	rdb := redisClient.GetUnderlyingClient()

	// 1. Unauthenticated request should fail with 401
	req, _ := http.NewRequest("GET", "/budget/status", nil)
	w := newCloseNotifierRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected unauthorized status 401, got %d", w.Code)
	}

	// 2. Request with invalid key format should fail with 401
	req, _ = http.NewRequest("GET", "/budget/status", nil)
	req.Header.Set("Authorization", "Bearer invalid-format")
	w = newCloseNotifierRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected unauthorized status 401, got %d", w.Code)
	}

	// 3. Request with valid key format but not registered should fail with 401
	rawKey, err := keyring.GenerateRawKey()
	if err != nil {
		t.Fatalf("Failed to generate raw key: %v", err)
	}
	req, _ = http.NewRequest("GET", "/budget/status", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	w = newCloseNotifierRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected unauthorized status 401, got %d", w.Code)
	}

	// 4. Register key and query status
	keyHash := keyring.HashKey(rawKey)
	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":       "test-status-key-" + uuid.New().String(),
		"provider":   "openai",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"active":     "true",
	})
	defer rdb.Del(ctx, "loopers:key:"+keyHash)

	// Set a mock daily budget configuration
	configKey := "loopers:budget:" + keyHash + ":config"
	rdb.HSet(ctx, configKey, "daily", "20.00")
	defer rdb.Del(ctx, configKey)

	req, _ = http.NewRequest("GET", "/budget/status", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	w = newCloseNotifierRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var statusMap map[string]budget.WindowStatus
	err = json.Unmarshal(w.Body.Bytes(), &statusMap)
	if err != nil {
		t.Fatalf("Failed to parse status map: %v", err)
	}

	if statusMap["daily"].Limit != 20.0 {
		t.Errorf("Expected daily limit 20.0, got %f", statusMap["daily"].Limit)
	}
}
