package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type CircuitBreakerResult struct {
	Tripped bool
	Count   int64
}

type CircuitBreaker struct {
	cfg      CircuitBreakerConfig
	rdb      *redis.Client
	cbScript *redis.Script
}

var cbMemberCounter atomic.Uint64

const luaMCPCircuitBreaker = `
local ring_key      = KEYS[1]
local hash          = ARGV[1]
local now           = tonumber(ARGV[2])
local window        = tonumber(ARGV[3])
local threshold     = tonumber(ARGV[4])
local member_id     = ARGV[5]
local cutoff        = now - window

redis.call('ZREMRANGEBYSCORE', ring_key, '-inf', cutoff)
redis.call('ZADD', ring_key, now, hash .. ':' .. member_id)
redis.call('EXPIRE', ring_key, window + 10)

local members = redis.call('ZRANGE', ring_key, 0, -1)
local count = 0
local prefix = hash .. ':'
for _, m in ipairs(members) do
    if string.sub(m, 1, string.len(prefix)) == prefix then
        count = count + 1
    end
end

if count >= threshold then
    return {1, count}
end
return {0, count}
`

func NewCircuitBreaker(cfg CircuitBreakerConfig, rdb *redis.Client) *CircuitBreaker {
	// Set sane defaults
	if cfg.Threshold <= 0 {
		cfg.Threshold = 5
	}
	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = 60
	}

	return &CircuitBreaker{
		cfg:      cfg,
		rdb:      rdb,
		cbScript: redis.NewScript(luaMCPCircuitBreaker),
	}
}

func hashArgs(args []byte) string {
	h := sha256.New()
	h.Write(args)
	return hex.EncodeToString(h.Sum(nil))
}

func (cb *CircuitBreaker) Check(ctx context.Context, sessionID, toolName string, args []byte) (*CircuitBreakerResult, error) {
	if sessionID == "" {
		return &CircuitBreakerResult{Tripped: false}, nil
	}

	ringKey := fmt.Sprintf("loopers:mcp:cb:%s", sessionID)
	now := time.Now().Unix()

	// Create a stable hash for the tool call
	callHash := fmt.Sprintf("%s|%s", toolName, hashArgs(args))
	memberID := strconv.FormatUint(cbMemberCounter.Add(1), 10)

	res, err := cb.cbScript.Run(ctx, cb.rdb, []string{ringKey}, callHash, strconv.FormatInt(now, 10), strconv.Itoa(cb.cfg.WindowSeconds), strconv.Itoa(cb.cfg.Threshold), memberID).Result()
	if err != nil {
		return nil, fmt.Errorf("circuit breaker redis script error: %w", err)
	}

	resSlice, ok := res.([]interface{})
	if !ok || len(resSlice) != 2 {
		return nil, fmt.Errorf("unexpected script response type: %v", res)
	}

	tripped := resSlice[0].(int64) == 1
	count := resSlice[1].(int64)

	return &CircuitBreakerResult{
		Tripped: tripped,
		Count:   count,
	}, nil
}
