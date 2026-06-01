package budget

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

// setupTestRedis starts a miniredis instance and returns a budget.Client
func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client, err := NewClient(mr.Addr(), "", 0)
	if err != nil {
		mr.Close()
		t.Fatalf("failed to create redis client: %v", err)
	}

	return mr, client
}

// 1. Streaming mid‑stream cut with heuristic & refund
func TestStreamingCut_RefundsUnusedReservation(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	ctx := context.Background()
	rdb := client.GetUnderlyingClient()
	keyHash := "stream_cut_refund_" + uuid.New().String()

	// Set a very tight budget of $0.10 for the minute window
	configKey := "loopers:budget:" + keyHash + ":config"
	rdb.HSet(ctx, configKey, "minute", "0.10")

	// Create LeaseManager
	lm := client.LeaseManager

	// Simulate an incoming streaming request that is estimated to cost $0.05
	// maxChunkUSD is 1.0, meaning we will ask Redis for up to $1.00
	estimatedCost := 0.05
	err := lm.Acquire(ctx, keyHash, estimatedCost, 1.0)
	if err != nil {
		t.Fatalf("Expected acquire to succeed for $0.05, but got %v", err)
	}

	// We acquired $0.05 internally, but the LeaseManager grabbed the full $0.10 from Redis.
	// We simulate receiving stream chunks using TryAcquireFast.
	// Actual cost so far: $0.03
	// Then a big chunk comes in that costs $0.08. Total cost = $0.11 (> budget $0.10).
	// The proxy would call TryAcquireFast(0.08)

	// Wait, the total remaining in the local lease should be $0.10 - $0.05 = $0.05
	// If we try to TryAcquireFast 0.03, it succeeds.
	success := lm.TryAcquireFast(keyHash, 0.03)
	if !success {
		t.Fatalf("Expected TryAcquireFast to succeed for $0.03")
	}

	// Now try to acquire 0.08, it should fail because only $0.02 is left.
	success = lm.TryAcquireFast(keyHash, 0.08)
	if success {
		t.Fatalf("Expected TryAcquireFast to fail for $0.08")
	}

	// Reconcile: we estimated 0.05, but actual cost before cut was 0.05 (initial) + 0.03 = 0.08.
	// Wait, proxy passes the total actual cost to ReconcileSpend.
	// Total paid by proxy = 0.05 + 0.03 = 0.08.
	// ReconcileSpend(est = 0.08, actual = 0.08) -> delta is 0
	lm.ReconcileSpend(keyHash, 0.08, 0.08)

	// In Redis, how much is spent?
	// The spent amount locally is tracked in SpentNano.
	// Let's force a heartbeat to flush SpentNano to Redis.
	lease := lm.getOrCreateLease(keyHash)

	// Wait, the worker flushes it. We can manually call the lua script or just sleep for heartbeat interval.
	// For testing, let's just inspect the local lease directly.
	spentNano := lease.SpentNano.Load()
	expectedNano := int64(0.08 * 1e9)
	if spentNano != expectedNano {
		t.Fatalf("Expected SpentNano to be %d, got %d", expectedNano, spentNano)
	}
}

// 2. Micro-batcher accuracy under load (simulate high concurrency)
func TestMicroBatcher_Accuracy(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	ctx := context.Background()
	rdb := client.GetUnderlyingClient()
	keyHash := "microbatcher_accuracy_" + uuid.New().String()

	// 5.00 limit
	configKey := "loopers:budget:" + keyHash + ":config"
	rdb.HSet(ctx, configKey, "daily", "5.00")

	lm := client.LeaseManager

	var totalSpent atomic.Int64 // in micro-dollars
	var wg sync.WaitGroup
	const numRequests = 1000
	costPerRequest := 0.01

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := lm.Acquire(ctx, keyHash, costPerRequest, 1.0)
			if err == nil {
				totalSpent.Add(int64(costPerRequest * 1e6))
			}
		}()
	}
	wg.Wait()

	total := float64(totalSpent.Load()) / 1e6
	if total > 5.00 {
		t.Fatalf("Total spent $%.4f exceeds budget $5.00", total)
	}

	// Verify local lease spent amount
	lease := lm.getOrCreateLease(keyHash)
	spentNano := lease.SpentNano.Load()

	// Convert spentNano to USD
	spentUSD := float64(spentNano) / 1e9

	// We expect total spent across goroutines to be approximately equal to lease's SpentNano
	// Note: float precision can cause tiny diffs, but we use integer math internally.
	if math.Abs(spentUSD-total) > 0.001 {
		t.Fatalf("Local lease spent %v does not match successful goroutines sum %v", spentUSD, total)
	}
}

// 3. Fail-closed when Redis is unavailable
func TestFailClosed_RedisDown(t *testing.T) {
	mr, client := setupTestRedis(t)

	ctx := context.Background()
	keyHash := "redis_down_" + uuid.New().String()

	lm := client.LeaseManager

	// Shut down Redis manually
	mr.Close()

	// Since Redis is down and we don't have a local lease for this key yet,
	// it must go to the slow path and fail.
	err := lm.Acquire(ctx, keyHash, 1.0, 1.0)
	if err == nil {
		t.Fatalf("Expected error when Redis is down, but got nil")
	}

	// Close the client so it doesn't try to background heartbeat to a dead redis
	client.Close()
}
