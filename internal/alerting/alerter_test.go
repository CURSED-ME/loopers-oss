package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

	owasp, ok := event["owasp"].(map[string]interface{})
	if !ok || owasp["owasp_category"] != "LLM10:2025" || owasp["severity"] != "critical" {
		t.Errorf("missing or incorrect OWASP metadata in budget_exceeded event: %v", event["owasp"])
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
		
		owasp, ok := event["owasp"].(map[string]interface{})
		if !ok || owasp["owasp_category"] != "LLM10:2025" || owasp["severity"] != "high" {
			t.Errorf("missing or incorrect OWASP metadata in threshold event: %v", event["owasp"])
		}
	}
}

func TestOWASPMetadataOnLoopFingerprint(t *testing.T) {
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

	alerter := NewAlerter(AlertingConfig{WebhookURL: server.URL}, nil)
	defer alerter.Close()

	alerter.TriggerLoopAlert("test-hash", "test-key", "openai", "sess-1", "fingerprint", "details", true)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected exactly 1 received event, got %d", len(received))
	}
	
	event := received[0]
	owasp, ok := event["owasp"].(map[string]interface{})
	if !ok || owasp["owasp_category"] != "LLM06:2025" || owasp["severity"] != "critical" {
		t.Errorf("incorrect OWASP metadata for fingerprint loop: %v", event["owasp"])
	}
	// Velocity rule also maps to "critical" — same OWASP LLM06:2025, same severity
}

func TestOWASPMetadataOnStallWarn(t *testing.T) {
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

	alerter := NewAlerter(AlertingConfig{WebhookURL: server.URL}, nil)
	defer alerter.Close()

	alerter.TriggerLoopAlert("test-hash", "test-key", "openai", "sess-2", "stall", "details", false)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected exactly 1 received event, got %d", len(received))
	}
	
	event := received[0]
	owasp, ok := event["owasp"].(map[string]interface{})
	if !ok || owasp["owasp_category"] != "LLM06:2025" || owasp["severity"] != "medium" {
		t.Errorf("incorrect OWASP metadata for stall warn: %v", event["owasp"])
	}
}

func TestStdoutEmissionNoWebhook(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run Alerter with NO webhook
	cfg := AlertingConfig{}
	alerter := NewAlerter(cfg, nil)
	
	alerter.TriggerBlockAlert("test-hash", "test-key", "openai", "gpt-4", "daily", 10.0, 10.0, 0.5)
	
	// Give worker time to process and write to stdout
	time.Sleep(100 * time.Millisecond)
	alerter.Close()
	
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if output == "" {
		t.Errorf("expected stdout output, got empty string")
	}
	
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(output), &event); err != nil {
		t.Errorf("failed to parse stdout JSON: %v. Output was: %s", err, output)
	}
	
	if event["event"] != "budget_exceeded" {
		t.Errorf("expected budget_exceeded event in stdout, got: %v", event)
	}
}
