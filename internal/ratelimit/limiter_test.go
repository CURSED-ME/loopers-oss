package ratelimit

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestLimiter(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cfg := Config{
		Enabled:           true,
		RequestsPerMinute: 2, // very low limit for testing
	}

	limiter := NewLimiter(cfg, rdb)
	ctx := context.Background()
	keyHash := "test-key-hash"

	// Request 1: should pass
	allowed, remaining, err := limiter.Check(ctx, keyHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected request to be allowed")
	}
	if remaining != 1 {
		t.Errorf("expected 1 remaining, got %d", remaining)
	}

	// Request 2: should pass
	allowed, remaining, err = limiter.Check(ctx, keyHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected request to be allowed")
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}

	// Request 3: should fail
	allowed, remaining, err = limiter.Check(ctx, keyHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Errorf("expected request to be blocked")
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}

}
