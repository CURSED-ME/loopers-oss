package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/budget"
	"github.com/CURSED-ME/loopers-oss/internal/keyring"
	"github.com/CURSED-ME/loopers-oss/internal/pricing"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/spf13/viper"
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
	defer s.Shutdown()
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
	defer s.Shutdown()
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
	configKey := "loopers:budget:{" + keyHash + "}:config"
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

func TestSessionIDValidation(t *testing.T) {
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

	// Load pricing
	pricingStore, _ := pricing.LoadStore("../../pricing.yaml")
	s := NewServer(redisClient, pricingStore)
	defer s.Shutdown()

	// Register mock provider
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer upstream.Close()
	s.RegisterProviderRoute(&mockProvider{baseURL: upstream.URL})

	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)
	ctx := context.Background()
	rdb := redisClient.GetUnderlyingClient()
	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":     "test-key",
		"provider": "mock",
		"active":   "true",
	})
	defer rdb.Del(ctx, "loopers:key:"+keyHash)

	tests := []struct {
		name      string
		sessionID string
		wantCode  int
	}{
		{"valid standard", "123e4567-e89b-12d3-a456-426614174000", http.StatusOK},
		{"valid dotted", "550e8400-e29b-41d4-a716-446655440000", http.StatusOK},
		{"invalid email-like", "user@domain.com", http.StatusBadRequest},
		{"invalid url-like", "org/team/session", http.StatusBadRequest},
		{"invalid colon", "org:team:session", http.StatusBadRequest},
		{"invalid space", "sess 123", http.StatusBadRequest},
		{"invalid special char", "sess!123", http.StatusBadRequest},
		{"invalid quotes", "sess\"123\"", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewBuffer([]byte(`{"model":"mock-model"}`)))
			req.Header.Set("Authorization", "Bearer "+rawKey)
			req.Header.Set("X-Loopers-Provider-Key", "dummy")
			req.Header.Set("X-Loopers-Session-ID", tt.sessionID)
			w := newCloseNotifierRecorder()
			s.GetRouter().ServeHTTP(w, req)
			if w.Code != tt.wantCode {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.wantCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestSybilAttackPrevention(t *testing.T) {
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

	// Load pricing
	pricingStore, _ := pricing.LoadStore("../../pricing.yaml")
	s := NewServer(redisClient, pricingStore)
	defer s.Shutdown()

	// Register mock provider
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer upstream.Close()
	s.RegisterProviderRoute(&mockProvider{baseURL: upstream.URL})

	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)
	ctx := context.Background()
	rdb := redisClient.GetUnderlyingClient()
	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":     "test-key",
		"provider": "mock",
		"active":   "true",
	})
	defer rdb.Del(ctx, "loopers:key:"+keyHash)

	viper.Set("session.max_per_key", 5)
	defer viper.Reset()

	for i := 1; i <= 6; i++ {
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewBuffer([]byte(`{"model":"mock-model"}`)))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", fmt.Sprintf("550e8400-e29b-41d4-a716-44665544000%d", i))

		w := newCloseNotifierRecorder()
		s.GetRouter().ServeHTTP(w, req)

		if i <= 5 {
			if w.Code != http.StatusOK {
				t.Errorf("Expected request %d to succeed with 200, got %d. Body: %s", i, w.Code, w.Body.String())
			}
		} else {
			if w.Code != http.StatusTooManyRequests {
				t.Errorf("Expected Sybil attack request %d to be blocked with 429, got %d. Body: %s", i, w.Code, w.Body.String())
			}
		}
	}
}
