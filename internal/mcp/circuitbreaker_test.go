package mcp

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return mr, rdb
}

func TestCircuitBreaker_Check(t *testing.T) {
	mr, rdb := setupTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	cfg := CircuitBreakerConfig{
		Threshold:     3,
		WindowSeconds: 10,
	}
	cb := NewCircuitBreaker(cfg, rdb)

	ctx := context.Background()
	sessionID := "test-session"
	toolName := "snowflake_query"
	args := []byte(`{"query": "SELECT * FROM db"}`)

	// Call 1
	res, err := cb.Check(ctx, sessionID, toolName, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Tripped {
		t.Errorf("expected circuit breaker not to trip on call 1")
	}

	// Call 2
	res, err = cb.Check(ctx, sessionID, toolName, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Tripped {
		t.Errorf("expected circuit breaker not to trip on call 2")
	}

	// Call 3 - threshold is 3, so it should trip
	res, err = cb.Check(ctx, sessionID, toolName, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Tripped {
		t.Errorf("expected circuit breaker to trip on call 3")
	}

	if res.Count != 3 {
		t.Errorf("expected count to be 3, got %d", res.Count)
	}
}
