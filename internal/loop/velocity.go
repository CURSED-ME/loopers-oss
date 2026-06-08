package loop

import (
	"context"
	"fmt"
	"time"

	"github.com/loopers-oss/loopers/internal/logging"
)

// CheckVelocity returns (isAnomaly, currentRPS, error)
func (d *Detector) CheckVelocity(ctx context.Context, sessionID, providerPath string) (bool, float64, error) {
	if d.cfg.Velocity.MaxRPS <= 0 && d.cfg.Velocity.MaxEndpointRepeats <= 0 {
		return false, 0, nil
	}

	now := time.Now().Unix()
	var currentRPS float64

	// 1. Check RPS (if enabled)
	if d.cfg.Velocity.MaxRPS > 0 {
		bucket := now / 10
		velKey := fmt.Sprintf("loopers:loop:vel:%s:%d", sessionID, bucket)

		count, err := d.rdb.Incr(ctx, velKey).Result()
		if err != nil {
			return false, 0, fmt.Errorf("velocity check redis error: %w", err)
		}

		if count == 1 {
			// Set TTL to 20s to ensure the 10s bucket expires after its useful life
			if expErr := d.rdb.Expire(ctx, velKey, 20*time.Second).Err(); expErr != nil {
				logging.Logger.Warn().Err(expErr).Str("key", velKey).Msg("velocity_expire_failed")
			}
		}

		currentRPS = float64(count) / 10.0
		if currentRPS > d.cfg.Velocity.MaxRPS {
			return true, currentRPS, nil
		}
	}

	// 2. Check endpoint repetition (if enabled)
	if d.cfg.Velocity.MaxEndpointRepeats > 0 {
		repeatWindow := d.cfg.Velocity.RepeatWindowSeconds
		if repeatWindow <= 0 {
			repeatWindow = 30
		}
		
		repeatBucket := now / int64(repeatWindow)
		repeatKey := fmt.Sprintf("loopers:loop:repeat:%s:%s:%d", sessionID, providerPath, repeatBucket)
		
		rCount, rErr := d.rdb.Incr(ctx, repeatKey).Result()
		if rErr == nil {
			if rCount == 1 {
				if expErr := d.rdb.Expire(ctx, repeatKey, time.Duration(repeatWindow*2)*time.Second).Err(); expErr != nil {
					logging.Logger.Warn().Err(expErr).Str("key", repeatKey).Msg("velocity_repeat_expire_failed")
				}
			}
			if int(rCount) > d.cfg.Velocity.MaxEndpointRepeats {
				return true, float64(rCount), nil
			}
		}
	}

	return false, currentRPS, nil
}
