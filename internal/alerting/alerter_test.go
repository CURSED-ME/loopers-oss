package alerting

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestAlerterDelivery(t *testing.T) {
	// Spin up mock webhook receiver
	var mu sync.Mutex
	var received []map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var event map[string]interface{}
		if err := json.Unmarshal(body, &event); err != nil {
			t.Errorf("failed to unmarshal request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		received = append(received, event)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := AlertingConfig{
		WebhookURL: server.URL,
		Thresholds: []ThresholdConfig{
			{Percent: 50, Message: "50% message"},
			{Percent: 80, Message: "80% message"},
		},
	}

	alerter := NewAlerter(cfg, nil)
	defer alerter.Close()

	// 1. Test Block Event Alert
	alerter.TriggerBlockAlert("test-hash", "test-key", "openai", "gpt-4o", "daily", 10.05, 10.00, 0.05)

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(received) != 1 {
		t.Fatalf("expected 1 received event, got %d", len(received))
	}
	event := received[0]
	mu.Unlock()

	if event["event"] != "budget_exceeded" || event["key_hash"] != "test-hash" || event["window"] != "daily" {
		t.Errorf("unexpected event content: %v", event)
	}
}

func TestAlerterThresholdCheck(t *testing.T) {
	// Skip Redis test if not available
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Skipping Redis-based threshold checks test: local Redis not available")
		return
	}
	defer rdb.Close()

	// Clean up potential keys
	rdb.Del(ctx, "loopers:alert:test-hash:daily:2026-05-23:50")

	var mu sync.Mutex
	var received []map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		var event map[string]interface{}
		json.Unmarshal(body, &event)
		received = append(received, event)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := AlertingConfig{
		WebhookURL: server.URL,
		Thresholds: []ThresholdConfig{
			{Percent: 50, Message: "50% message"},
		},
	}

	alerter := NewAlerter(cfg, rdb)
	defer alerter.Close()

	spends := map[string]float64{"daily": 6.00}
	limits := map[string]float64{"daily": 10.00}

	// 1. Should fire threshold alert (spend is 6.00 which is 60% of 10.00)
	alerter.TriggerThresholdAlerts(ctx, "test-hash", "test-key", "openai", spends, limits)
	time.Sleep(100 * time.Millisecond)

	// 2. Trigger again -> should NOT fire due to Redis deduplication
	alerter.TriggerThresholdAlerts(ctx, "test-hash", "test-key", "openai", spends, limits)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Errorf("expected exactly 1 received event, got %d", len(received))
	} else {
		event := received[0]
		if event["event"] != "budget_threshold" || event["threshold_percent"] != float64(50) {
			t.Errorf("unexpected event: %v", event)
		}
	}
}
