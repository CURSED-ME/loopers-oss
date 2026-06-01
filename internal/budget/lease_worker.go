package budget

import (
	"context"
	"fmt"
	"time"

	"github.com/loopers-oss/loopers/internal/logging"
)

// StartLeaseWorkers starts the background goroutines for heartbeat and reclaim.
func (lm *LeaseManager) StartLeaseWorkers(ctx context.Context) {
	go lm.heartbeatLoop(ctx)
	go lm.reclaimLoop(ctx)
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
		
		leaseID := lease.LeaseID
		if leaseID == "" {
			return true // No active lease ID yet
		}

		// Swap out the spent nano atomically to get what we've spent since the last heartbeat
		spentNano := lease.SpentNano.Swap(0)
		spentUSD := FromNano(spentNano)

		args := []interface{}{
			leaseID,
			fmt.Sprintf("%f", spentUSD),
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
				lease.LeaseID = ""
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
