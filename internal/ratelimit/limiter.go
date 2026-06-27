package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Enabled            bool `mapstructure:"enabled"`
	RequestsPerMinute  int  `mapstructure:"requests_per_minute"`
	RequestsPerSession int  `mapstructure:"requests_per_session"`
}

type Limiter struct {
	cfg        Config
	rdb        *redis.Client
	rateScript *redis.Script
}

var memberCounter atomic.Uint64

func NewLimiter(cfg Config, rdb *redis.Client) *Limiter {
	// Embedded Lua script for atomic sliding window rate limiting
	const luaRateCheck = `
local ring_key      = KEYS[1]
local hash          = ARGV[1]
local now           = tonumber(ARGV[2])
local window        = tonumber(ARGV[3])
local threshold     = tonumber(ARGV[4])
local member_id     = ARGV[5]
local cutoff        = now - window

-- Remove expired elements
redis.call('ZREMRANGEBYSCORE', ring_key, '-inf', cutoff)
-- Add current request
redis.call('ZADD', ring_key, now, hash .. ':' .. member_id)
-- Set TTL slightly larger than window to ensure cleanup
redis.call('EXPIRE', ring_key, window + 10)

-- Count elements in window
local count = redis.call('ZCARD', ring_key)

local allowed = 1
if count > threshold then
    allowed = 0
end

local remaining = threshold - count
if remaining < 0 then
    remaining = 0
end

return {allowed, remaining}
`

	return &Limiter{
		cfg:        cfg,
		rdb:        rdb,
		rateScript: redis.NewScript(luaRateCheck),
	}
}

// Check enforces the rate limit for a given keyHash. It returns whether the request is allowed,
// the remaining capacity, and an error if the redis operation failed.
func (l *Limiter) Check(ctx context.Context, keyHash string) (allowed bool, remaining int, err error) {
	if !l.cfg.Enabled {
		return true, -1, nil
	}

	ringKey := fmt.Sprintf("loopers:ratelimit:key:%s", keyHash)
	now := time.Now().Unix()
	memberID := strconv.FormatUint(memberCounter.Add(1), 10)
	windowSeconds := 60 // requests_per_minute is measured over a 60-second sliding window

	res, err := l.rateScript.Run(ctx, l.rdb, []string{ringKey}, keyHash, strconv.FormatInt(now, 10), strconv.Itoa(windowSeconds), strconv.Itoa(l.cfg.RequestsPerMinute), memberID).Result()
	if err != nil {
		return false, 0, fmt.Errorf("rate limiter redis script error: %w", err)
	}

	resSlice, ok := res.([]interface{})
	if !ok || len(resSlice) != 2 {
		return false, 0, fmt.Errorf("unexpected script response type: %v", res)
	}

	allowed = resSlice[0].(int64) == 1
	remaining = int(resSlice[1].(int64))

	return allowed, remaining, nil
}
