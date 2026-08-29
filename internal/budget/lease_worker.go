package budget

import (
	"context"
	"strconv"
	"time"

	"github.com/try-loopers/loopers-oss/internal/logging"
)

func (lm *LeaseManager) StartLeaseWorkers(ctx context.Context) {
	go lm.heartbeatLoop(ctx)
	go lm.reclaimLoop(ctx)
	go lm.guardLoop(ctx)
}

func (lm *LeaseManager) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lm.sendHeartbeats(ctx)
		}
	}
}

func (lm *LeaseManager) sendHeartbeats(ctx context.Context) {
	// Iterate through all active local leases
	lm.leases.Range(func(key, value interface{}) bool {
		lease := value.(*LocalLease)

		lease.mu.Lock()
		leaseID := lease.LeaseID
		lease.mu.Unlock()
		if leaseID == "" {
			return true // No active lease ID yet
		}

		// Swap out the spent nano atomically to get what we've spent since the last heartbeat
		spentNano := lease.SpentNano.Swap(0)
		args := []interface{}{
			leaseID,
			strconv.FormatInt(spentNano, 10),
		}

		// Run the lua script
		res, err := leaseHeartbeatScript.Run(ctx, lm.client.rdb, []string{}, args...).Result()
		if err != nil {
			logging.Logger.Warn().Err(err).Str("lease_id", leaseID).Msg("Failed to send heartbeat for lease")
			// If heartbeat fails, we MUST add the spent amount back to SpentNano so it gets retried next time,
			// UNLESS the lease is actually dead.
			lease.SpentNano.Add(spentNano)
			return true
		}

		// If the lease is dead/reclaimed in Redis, it returns 0.
		// If so, we must stop spending locally.
		resArr, ok := res.([]interface{})
		if ok && len(resArr) > 0 {
			status, ok := resArr[0].(int64)
			if ok && status == 0 {
				logging.Logger.Warn().Str("lease_id", leaseID).Msg("Lease is dead in Redis. Invalidating local lease.")
				// Zero out the remaining nano so it stops fast-pathing
				lease.RemainingNano.Store(0)

				lease.mu.Lock()
				lease.LeaseID = ""
				lease.mu.Unlock()
			}
		}

		return true
	})
}

func (lm *LeaseManager) reclaimLoop(ctx context.Context) {
	// Reclaim runs every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			res, err := leaseReclaimScript.Run(ctx, lm.client.rdb, []string{}).Result()
			if err != nil {
				logging.Logger.Warn().Err(err).Msg("Failed to run lease reclaimer")
				continue
			}

			if resArr, ok := res.([]interface{}); ok && len(resArr) > 0 {
				if count, ok := resArr[0].(int64); ok && count > 0 {
					logging.Logger.Info().Int64("reclaimed", count).Msg("Reclaimed abandoned leases from Redis")
				}
			}
		}
	}
}

func (lm *LeaseManager) guardLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lm.runGuardCheck(ctx)
		}
	}
}

func (lm *LeaseManager) runGuardCheck(ctx context.Context) {
	lm.leases.Range(func(key, value interface{}) bool {
		keyHash := key.(string)
		lease := value.(*LocalLease)

		// Only check keys that have an active Redis lease.
		// Keys with no LeaseID have never hit Redis, skip them.
		lease.mu.Lock()
		leaseID := lease.LeaseID
		lease.mu.Unlock()
		if leaseID == "" {
			return true
		}

		statusMap, err := lm.client.GetBudgetStatus(ctx, keyHash)
		if err != nil {
			logging.Logger.Warn().Err(err).Str("key_hash", keyHash).Msg("budget guard: failed to fetch status")
			return true
		}

		for _, status := range statusMap {
			if status.Limit > 0 && status.CurrentSpend >= status.Limit {
				if !lease.IsBlocked.Load() {
					logging.Logger.Warn().
						Str("key_hash", keyHash).
						Float64("spend", status.CurrentSpend).
						Float64("limit", status.Limit).
						Msg("budget guard: hard ceiling reached, blocking key")
				}
				lease.IsBlocked.Store(true)
				lease.RemainingNano.Store(0)
				return true
			}
		}

		// If we were previously blocked but the budget window has rolled over
		// (spend dropped back below limit), unblock.
		if lease.IsBlocked.Load() {
			lease.IsBlocked.Store(false)
			logging.Logger.Info().Str("key_hash", keyHash).Msg("budget guard: key unblocked after window reset")
		}

		return true
	})
}
