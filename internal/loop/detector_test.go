package loop

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, func() {
		client.Close()
		mr.Close()
	}
}

func TestDetectorDisabled(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	cfg := Config{Enabled: false}
	d := NewDetector(cfg, client)

	res, err := d.Check(context.Background(), "session_1", "/openai/v1/chat/completions", []byte(`{"model": "gpt-4"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil result when disabled")
	}
}

func TestDetectorEmptySession(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	cfg := Config{Enabled: true}
	d := NewDetector(cfg, client)

	res, err := d.Check(context.Background(), "", "/openai/v1/chat/completions", []byte(`{"model": "gpt-4"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil result for empty session")
	}
}

func TestDetectorFingerprintLoop(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	cfg := Config{
		Enabled: true,
		Fingerprint: FingerprintConfig{
			Threshold:     3,
			WindowSeconds: 60,
		},
	}
	d := NewDetector(cfg, client)

	body := []byte(`{"model": "gpt-4", "messages": [{"role": "user", "content": "hi"}]}`)

	// Req 1
	res1, err := d.Check(context.Background(), "session_1", "/openai/v1/chat/completions", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res1 != nil {
		t.Fatalf("expected no loop on req 1")
	}

	// Req 2
	res2, err := d.Check(context.Background(), "session_1", "/openai/v1/chat/completions", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2 != nil {
		t.Fatalf("expected no loop on req 2")
	}

	// Req 3 - should trigger
	res3, err := d.Check(context.Background(), "session_1", "/openai/v1/chat/completions", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res3 == nil {
		t.Fatalf("expected loop on req 3")
	}
	if !res3.Detected || res3.Rule != "fingerprint" || !res3.ShouldBlock {
		t.Fatalf("unexpected result: %+v", res3)
	}

	// Req 4 - different session, should not trigger
	res4, err := d.Check(context.Background(), "session_2", "/openai/v1/chat/completions", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res4 != nil {
		t.Fatalf("expected no loop on new session")
	}
}

func TestDetectorStallWarn(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	cfg := Config{
		Enabled: true,
		Stall: StallConfig{
			MinHammingDistance:    20,
			LowDiversityThreshold: 3,
			Action:                "warn",
		},
	}
	d := NewDetector(cfg, client)

	// Sending 4 identical requests will trigger stall if we disable fingerprinting
	// because Hamming distance is 0 (< 20).
	// Req 1: diversity=NA (count=0)
	// Req 2: diversity=low (count=1)
	// Req 3: diversity=low (count=2)
	// Req 4: diversity=low (count=3 -> triggered)
	body := []byte(`{"model": "gpt-4", "messages": [{"role": "user", "content": "hi"}]}`)

	for i := 0; i < 3; i++ {
		res, _ := d.Check(context.Background(), "session_1", "/openai/v1/chat/completions", body)
		if res != nil {
			t.Fatalf("unexpected loop on req %d", i+1)
		}
	}

	// Req 4 should trigger stall warning
	// Wait a tiny bit just in case
	time.Sleep(10 * time.Millisecond)
	res, err := d.Check(context.Background(), "session_1", "/openai/v1/chat/completions", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected stall detection on req 3")
	}
	if !res.Detected || res.Rule != "stall" || res.ShouldBlock {
		t.Fatalf("unexpected result: %+v (expected warn-only stall)", res)
	}
}

func TestVelocityRPSAnomaly(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	cfg := Config{
		Enabled: true,
		Velocity: VelocityConfig{
			MaxRPS: 0.5, // 5 requests per 10s bucket max
		},
	}
	d := NewDetector(cfg, client)

	// Send 5 requests (should pass)
	for i := 0; i < 5; i++ {
		res, _ := d.Check(context.Background(), "vel_session", "/openai/v1/chat/completions", []byte(`{"m": "a"}`))
		if res != nil {
			t.Fatalf("unexpected velocity anomaly on req %d", i+1)
		}
	}

	// 6th request should trigger anomaly
	res, err := d.Check(context.Background(), "vel_session", "/openai/v1/chat/completions", []byte(`{"m": "a"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.Detected || res.Rule != "velocity" {
		t.Fatalf("expected velocity anomaly on req 6")
	}
}

func TestVelocityEndpointRepeat(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	cfg := Config{
		Enabled: true,
		Velocity: VelocityConfig{
			MaxEndpointRepeats:  3,
			RepeatWindowSeconds: 30,
		},
	}
	d := NewDetector(cfg, client)

	// Send 3 requests to the same endpoint (should pass)
	for i := 0; i < 3; i++ {
		res, _ := d.Check(context.Background(), "vel_ep_session", "/target/path", []byte(`{"m": "a"}`))
		if res != nil {
			t.Fatalf("unexpected endpoint anomaly on req %d", i+1)
		}
	}

	// 4th request to a DIFFERENT endpoint should pass
	res, _ := d.Check(context.Background(), "vel_ep_session", "/other/path", []byte(`{"m": "a"}`))
	if res != nil {
		t.Fatalf("unexpected anomaly on different endpoint")
	}

	// 4th request to the SAME endpoint should trigger
	res, _ = d.Check(context.Background(), "vel_ep_session", "/target/path", []byte(`{"m": "a"}`))
	if res == nil || !res.Detected || res.Rule != "velocity" {
		t.Fatalf("expected endpoint repeat anomaly")
	}
}

func TestFingerprintVolatileFields(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	cfg := Config{
		Enabled: true,
		Fingerprint: FingerprintConfig{
			Threshold:     3,
			WindowSeconds: 60,
		},
	}
	d := NewDetector(cfg, client)

	// These bodies are semantically identical (only volatile fields differ)
	bodies := [][]byte{
		[]byte(`{"model": "gpt-4", "temperature": 0.7, "messages": [{"role": "user", "content": "hi"}]}`),
		[]byte(`{"model": "gpt-4", "temperature": 0.2, "messages": [{"role": "user", "content": "hi"}]}`),
		[]byte(`{"model": "gpt-4", "seed": 42, "messages": [{"role": "user", "content": "hi"}]}`),
	}

	for i, body := range bodies {
		res, _ := d.Check(context.Background(), "vol_session", "/openai/v1/chat/completions", body)
		if i < 2 && res != nil {
			t.Fatalf("unexpected loop on req %d", i+1)
		}
		if i == 2 && res == nil {
			t.Fatalf("expected loop detection on req 3 due to volatile field stripping")
		}
	}
}

func TestStallConcurrentRace(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	cfg := Config{
		Enabled: true,
		Stall: StallConfig{
			MinHammingDistance:    20,
			LowDiversityThreshold: 10,
			Action:                "warn",
		},
	}
	d := NewDetector(cfg, client)

	// Send 5 concurrent identical requests.
	// Due to concurrency, some might overlap, but the transactional logic
	// should prevent the stall counter from spuriously spiking beyond the actual number of requests.
	// Since Threshold is 10, 5 identical requests should NEVER trigger a stall, even with race conditions.
	
	errCh := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			res, err := d.Check(context.Background(), "race_session", "/openai/v1/chat/completions", []byte(`{"m": "a"}`))
			if err != nil {
				errCh <- err
				return
			}
			if res != nil {
				errCh <- fmt.Errorf("unexpected stall detection in concurrent test")
				return
			}
			errCh <- nil
		}()
	}

	for i := 0; i < 5; i++ {
		err := <-errCh
		if err != nil {
			t.Fatalf("concurrent stall failure: %v", err)
		}
	}
}
