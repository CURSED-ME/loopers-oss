package budget

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/logging"
)

const nanoPerUSD = 1e9

func ToNano(usd float64) int64 {
	return int64(math.Round(usd * nanoPerUSD))
}

func FromNano(n int64) float64 {
	return float64(n) / nanoPerUSD
}

type LocalLease struct {
	RemainingNano atomic.Int64
	SpentNano     atomic.Int64
	IsBlocked     atomic.Bool
	LeaseID       string
	KeyHash       string
	mu            sync.Mutex
}

type LeaseManager struct {
	client *Client
	leases sync.Map // map[string]*LocalLease
}

func NewLeaseManager(c *Client) *LeaseManager {
	return &LeaseManager{
		client: c,
	}
}

// getOrCreateLease returns the per-key lease object.
func (lm *LeaseManager) getOrCreateLease(keyHash string) *LocalLease {
	val, ok := lm.leases.Load(keyHash)
	if ok {
		return val.(*LocalLease)
	}

	lease := &LocalLease{
		KeyHash: keyHash,
	}
	actual, _ := lm.leases.LoadOrStore(keyHash, lease)
	return actual.(*LocalLease)
}

// Acquire tries to deduct the cost from the local lease. If insufficient, it acquires a new chunk from Redis.
func (lm *LeaseManager) Acquire(ctx context.Context, keyHash string, estCostUSD float64, maxChunkUSD float64) error {
	costNano := ToNano(estCostUSD)
	lease := lm.getOrCreateLease(keyHash)

	if lease.IsBlocked.Load() {
		return ErrBudgetExceeded
	}

	// Fast path: try to deduct locally
	for {
		current := lease.RemainingNano.Load()
		if current >= costNano {
			if lease.RemainingNano.CompareAndSwap(current, current-costNano) {
				lease.SpentNano.Add(costNano)
				return nil
			}
			continue
		}
		break
	}

	// Slow path: Need more budget. Acquire per-key lock to prevent thundering herd on Redis.
	lease.mu.Lock()
	defer lease.mu.Unlock()

	// Double check after acquiring lock (another goroutine might have replenished it)
	current := lease.RemainingNano.Load()
	if current >= costNano {
		if lease.RemainingNano.CompareAndSwap(current, current-costNano) {
			lease.SpentNano.Add(costNano)
			return nil
		}
		// If CAS fails here, we fall through and just fetch a new lease anyway for simplicity,
		// though normally it shouldn't fail since we hold the lock (but fast-path concurrent requests don't!).
	}

	// Calculate how much we need. At least maxChunk, unless estCost is bigger.
	// But usually maxChunk > estCost.
	requestChunkUSD := maxChunkUSD
	if estCostUSD > maxChunkUSD {
		requestChunkUSD = estCostUSD
	}

	grantedUSD, newLeaseID, err := lm.client.AcquireLeaseRedis(ctx, keyHash, requestChunkUSD)
	if err != nil {
		return err
	}

	grantedNano := ToNano(grantedUSD)

	// If the granted amount isn't even enough for this single request (depletion mode edge case),
	// we must fail it and refund the tiny amount back to the lease.
	if grantedNano < costNano {
		lease.RemainingNano.Add(grantedNano)
		return ErrBudgetExceeded
	}

	// We have enough. Deduct the cost from the granted chunk.
	netAddition := grantedNano - costNano

	// Add the net addition to the local lease
	lease.RemainingNano.Add(netAddition)
	lease.SpentNano.Add(costNano)

	if lease.LeaseID != "" && lease.LeaseID != newLeaseID {
		// Edge case: if we already had a lease ID, we should technically tell the heartbeat manager
		// to flush the old lease ID's spent amount before adopting the new one.
		// For v1, we will just enqueue the old lease ID for a final flush, or let the reclaimer handle the unspent reserved part.
		// A cleaner way is just to maintain ONE lease ID, but Redis gave us a new one.
	}
	lease.LeaseID = newLeaseID

	// Register the new lease with the heartbeat manager (to be implemented)
	// RegisterLeaseHeartbeat(newLeaseID, keyHash)

	return nil
}

// ReconcileSpend updates the local lease with the exact difference between estimated and actual cost.
// If actualCost < estimatedCost, the difference is refunded to the local lease.
// If actualCost > estimatedCost, the difference is synchronized immediately to Redis.
func (lm *LeaseManager) ReconcileSpend(ctx context.Context, keyHash string, estCostUSD float64, actualCostUSD float64) {
	deltaUSD := estCostUSD - actualCostUSD
	if deltaUSD == 0 {
		return
	}

	if deltaUSD > 0 {
		// Refund path (actual < est): handle locally
		deltaNano := ToNano(deltaUSD)
		lease := lm.getOrCreateLease(keyHash)
		lease.RemainingNano.Add(deltaNano)
		lease.SpentNano.Add(-deltaNano)
		return
	}

	// Overage path (actual > est): actual usage exceeded our estimated cost.
	// Since the heartbeat truncates any spend that exceeds the initially reserved amount,
	// we must write the overage directly to the Redis spend keys immediately.
	overageUSD := -deltaUSD

	var err error
	backoffs := []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, 2 * time.Second}
	for attempt := 0; attempt <= len(backoffs); attempt++ {
		err = lm.client.reconcileRedis(ctx, keyHash, 0, overageUSD)
		if err == nil {
			break
		}
		if attempt < len(backoffs) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoffs[attempt]):
			}
		}
	}
	if err != nil {
		logging.Logger.Error().Err(err).Str("key_hash", keyHash).Float64("overage_usd", overageUSD).Msg("budget: failed to sync overage to Redis after retries; guardLoop will correct")
	}
}

// ErrBudgetExceeded is returned when the global budget is too low.
var ErrBudgetExceeded = errors.New("budget exceeded")

// TryAcquireFast attempts to deduct from the local lease instantly without calling Redis.
// Returns false if the local lease is exhausted.
func (lm *LeaseManager) TryAcquireFast(keyHash string, costUSD float64) bool {
	costNano := ToNano(costUSD)
	lease := lm.getOrCreateLease(keyHash)

	if lease.IsBlocked.Load() {
		return false
	}

	for {
		current := lease.RemainingNano.Load()
		if current >= costNano {
			if lease.RemainingNano.CompareAndSwap(current, current-costNano) {
				lease.SpentNano.Add(costNano)
				return true
			}
			continue
		}
		return false
	}
}
