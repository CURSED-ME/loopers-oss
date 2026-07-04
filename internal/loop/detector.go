package loop

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Result struct {
	Detected    bool
	Rule        string // "fingerprint" | "velocity" | "stall"
	Detail      string // human-readable detail for logging
	ShouldBlock bool   // false if action=warn
}

type Detector struct {
	cfg      Config
	rdb      *redis.Client
	fpScript *redis.Script
}

// luaLoopFingerprint is the script that implements the sliding window hash count.
// We embed it directly here for simplicity, though it could be read from a file.
const luaLoopFingerprint = `
local ring_key      = KEYS[1]
local hash          = ARGV[1]
local now           = tonumber(ARGV[2])
local window        = tonumber(ARGV[3])
local threshold     = tonumber(ARGV[4])
local member_id     = ARGV[5]
local max_distance  = tonumber(ARGV[6])
local cutoff        = now - window

redis.call('ZREMRANGEBYSCORE', ring_key, '-inf', cutoff)
redis.call('ZADD', ring_key, now, hash .. ':' .. member_id)
redis.call('EXPIRE', ring_key, window + 10)

local popcount = {
  [0]=0,[1]=1,[2]=1,[3]=2,[4]=1,[5]=2,[6]=2,[7]=3,
  [8]=1,[9]=2,[10]=2,[11]=3,[12]=2,[13]=3,[14]=3,[15]=4
}

local function bxor_4bit(a,b)
  local c = 0
  local p = 1
  for i=1,4 do
    local a_bit = a % 2
    local b_bit = b % 2
    if a_bit ~= b_bit then c = c + p end
    a = math.floor(a/2)
    b = math.floor(b/2)
    p = p * 2
  end
  return c
end

local function hex_hamming(a, b)
  local dist = 0
  for i = 1, #a do
    local xa = tonumber(string.sub(a, i, i), 16)
    local xb = tonumber(string.sub(b, i, i), 16)
    if xa and xb then
      dist = dist + popcount[bxor_4bit(xa, xb)]
    end
  end
  return dist
end

local members = redis.call('ZRANGE', ring_key, 0, -1)
local count = 0
for _, m in ipairs(members) do
    local hash_part = string.sub(m, 1, 16)
    if hex_hamming(hash_part, hash) <= max_distance then
        count = count + 1
    end
end

if count >= threshold then
    return {1, count}
end
return {0, count}
`

func NewDetector(cfg Config, rdb *redis.Client) *Detector {
	return &Detector{
		cfg:      cfg,
		rdb:      rdb,
		fpScript: redis.NewScript(luaLoopFingerprint),
	}
}

// Check runs all enabled sub-detectors against the request.
// It is designed to be called once per request, after session checks pass.
func (d *Detector) Check(ctx context.Context, sessionID, providerPath string, body []byte) (*Result, error) {
	if !d.cfg.Enabled || sessionID == "" {
		return nil, nil
	}

	detCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	// 1. Compute Hash
	hashStr, hashVal, err := NormalizeAndHash(body)
	if err != nil {
		return nil, fmt.Errorf("failed to hash request body: %w", err)
	}

	// 2. Fingerprint Check
	isFPLoop, fpCount, err := d.CheckFingerprint(detCtx, sessionID, hashStr)
	if err != nil {
		return nil, err
	}
	if isFPLoop {
		return &Result{
			Detected:    true,
			Rule:        "fingerprint",
			Detail:      fmt.Sprintf("Similar request detected %d times within %d seconds", fpCount, d.cfg.Fingerprint.WindowSeconds),
			ShouldBlock: true, // Fingerprint matches are highly confident loops
		}, nil
	}

	// 3. Velocity Check
	isVelAnomaly, metric, err := d.CheckVelocity(detCtx, sessionID, providerPath)
	if err != nil {
		return nil, err
	}
	if isVelAnomaly {
		return &Result{
			Detected:    true,
			Rule:        "velocity",
			Detail:      fmt.Sprintf("Velocity threshold exceeded (metric: %.2f)", metric),
			ShouldBlock: true,
		}, nil
	}

	// 4. Stall Check
	isStalled, stallCount, err := d.CheckStall(detCtx, sessionID, hashVal)
	if err != nil {
		return nil, err
	}
	if isStalled {
		shouldBlock := d.cfg.Stall.Action == "block"
		return &Result{
			Detected:    true,
			Rule:        "stall",
			Detail:      fmt.Sprintf("Session stalled: %d sequential low-diversity requests", stallCount),
			ShouldBlock: shouldBlock,
		}, nil
	}

	return nil, nil
}
