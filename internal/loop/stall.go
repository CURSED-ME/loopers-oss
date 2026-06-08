package loop

import (
	"context"
	"fmt"
	"math/bits"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// CheckStall compares the current hash with the last N hashes to determine
// if the session is stuck making very similar (but not perfectly identical) requests.
// Returns (isStalled, lowDiversityCount, error).
func (d *Detector) CheckStall(ctx context.Context, sessionID string, currentHashVal uint64) (bool, int64, error) {
	if d.cfg.Stall.MinHammingDistance <= 0 || d.cfg.Stall.LowDiversityThreshold <= 0 {
		return false, 0, nil
	}

	listKey := fmt.Sprintf("loopers:loop:stall:%s", sessionID)
	countKey := fmt.Sprintf("loopers:loop:stall_count:%s", sessionID)

	var finalCount int64

	// Watch the listKey for changes (optimistic concurrency)
	err := d.rdb.Watch(ctx, func(tx *redis.Tx) error {
		// Fetch last hash from the list
		lastHashStr, lErr := tx.LIndex(ctx, listKey, -1).Result()

		isLowDiversity := false
		if lErr == nil && lastHashStr != "" {
			lastHashVal, parseErr := strconv.ParseUint(lastHashStr, 10, 64)
			if parseErr == nil {
				// Compute Hamming distance
				distance := bits.OnesCount64(currentHashVal ^ lastHashVal)
				if distance < d.cfg.Stall.MinHammingDistance {
					isLowDiversity = true
				}
			}
		} else if lErr != nil && lErr != redis.Nil {
			return lErr
		}

		// Now issue the write commands inside a transaction
		var incrCmd *redis.IntCmd
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.RPush(ctx, listKey, strconv.FormatUint(currentHashVal, 10))
			pipe.LTrim(ctx, listKey, -10, -1)    // Keep only last 10 hashes for reference
			pipe.Expire(ctx, listKey, time.Hour) // Session TTL

			if isLowDiversity {
				incrCmd = pipe.Incr(ctx, countKey)
				pipe.Expire(ctx, countKey, time.Hour)
			} else {
				// Reset count if progress is made
				pipe.Del(ctx, countKey)
			}
			return nil
		})

		// Retrieve result of the incr command if it executed
		if err == nil && isLowDiversity && incrCmd != nil {
			finalCount = incrCmd.Val()
		}

		return err
	}, listKey)

	if err == redis.TxFailedErr {
		// Concurrent request modified the list while we were computing.
		// Safe to ignore for stall detection (progress is happening concurrently)
		return false, 0, nil
	} else if err != nil {
		return false, 0, fmt.Errorf("stall redis watch error: %w", err)
	}

	isStalled := finalCount >= int64(d.cfg.Stall.LowDiversityThreshold)
	return isStalled, finalCount, nil
}
