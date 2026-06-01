package budget

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// AcquireLeaseRedis asks Redis for a new chunk of budget.
// It executes the lua_lease_acquire.lua script to evaluate 5 budget windows and deducts maxChunkUSD from all of them.
// If less than maxChunkUSD is available across any window, it deducts the minimum available amount.
// Returns the granted amount in USD, the generated lease ID, and an error.
func (c *Client) AcquireLeaseRedis(ctx context.Context, keyHash string, maxChunkUSD float64) (float64, string, error) {
	configKey := fmt.Sprintf("loopers:budget:%s:config", keyHash)
	limits, err := c.getBudgetConfig(ctx, configKey)
	if err != nil {
		return 0, "", err
	}

	windows := getWindowConfigs(keyHash, time.Now().UTC())

	if len(windows) == 0 {
		// Uncapped budget, we can just grant the max chunk implicitly.
		// However, to keep it simple, we just grant an arbitrarily large lease.
		// Actually, if it's uncapped, we should just return maxChunkUSD and a dummy lease ID.
		return maxChunkUSD, uuid.NewString(), nil
	}

	// 2. Prepare arguments for the Lua script
	leaseID := uuid.NewString()
	args := []interface{}{
		leaseID,
		fmt.Sprintf("%f", maxChunkUSD),
		len(windows),
	}

	keys := []string{} // Not using KEYS because we dynamically hit multiple, cluster mode handles tags differently if needed

	var numWindows int
	for _, w := range windows {
		limitStr, ok := limits[w.Name]
		if !ok || limitStr == "" {
			continue // No limit for this window type
		}
		
		limitUSD, err := strconv.ParseFloat(limitStr, 64)
		if err != nil || limitUSD <= 0 {
			continue
		}
		
		args = append(args, w.Key)
		args = append(args, fmt.Sprintf("%f", limitUSD))
		args = append(args, strconv.Itoa(w.TTL))
		numWindows++
	}
	
	args[2] = numWindows // Update actual window count

	if numWindows == 0 {
		return maxChunkUSD, uuid.NewString(), nil
	}

	// 3. Execute script
	result, err := leaseAcquireScript.Run(ctx, c.rdb, keys, args...).Result()
	if err != nil {
		return 0, "", err
	}

	resArr, ok := result.([]interface{})
	if !ok || len(resArr) != 2 {
		return 0, "", fmt.Errorf("unexpected lua result format")
	}

	status, ok := resArr[0].(int64)
	if !ok {
		return 0, "", fmt.Errorf("unexpected lua status format")
	}

	if status == 0 {
		return 0, "", ErrBudgetExceeded
	}

	grantedStr, ok := resArr[1].(string)
	if !ok {
		return 0, "", fmt.Errorf("unexpected lua granted amount format")
	}

	grantedUSD, err := strconv.ParseFloat(grantedStr, 64)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse granted amount: %w", err)
	}

	return grantedUSD, leaseID, nil
}
