package riskprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key pattern: loopers:risk_profile:{keyHash}
// The {keyHash} hash tag ensures Redis Cluster hash slot co-location.
const redisKeyPrefix = "loopers:risk_profile:"

func profileKey(keyHash string) string {
	return fmt.Sprintf("%s{%s}", redisKeyPrefix, keyHash)
}

// AgentRiskProfile represents the persistent cross-session behavioral risk state of an agent.
type AgentRiskProfile struct {
	KeyHash              string    `json:"key_hash"`
	TotalSpend           float64   `json:"total_spend"`            // Lifetime spend across all sessions
	TotalPolicyBlocks    int64     `json:"total_policy_blocks"`    // Lifetime policy block count
	TotalEscalations     int64     `json:"total_escalations"`      // Lifetime escalation count
	PersistentTaintFlags []string  `json:"persistent_taint_flags"` // Never cleared across sessions
	RiskScore            int       `json:"risk_score"`             // Clamped between 0 and 100
	LastHighRiskAction   time.Time `json:"last_high_risk_action"`  // Timestamp of last high-risk action
	QuarantineUntil      time.Time `json:"quarantine_until"`       // Expiration time if under auto-quarantine
	LastDecayTime        time.Time `json:"last_decay_time"`        // Timestamp for 24h decay tracking
	SessionCount         int64     `json:"session_count"`          // Lifetime unique sessions initiated
}

// UnmarshalJSON handles both JSON arrays [] and empty JSON objects {} for persistent_taint_flags safely.
func (rp *AgentRiskProfile) UnmarshalJSON(data []byte) error {
	type Alias AgentRiskProfile
	aux := &struct {
		PersistentTaintFlags json.RawMessage `json:"persistent_taint_flags"`
		*Alias
	}{
		Alias: (*Alias)(rp),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.PersistentTaintFlags) > 0 && string(aux.PersistentTaintFlags) != "null" {
		if string(aux.PersistentTaintFlags) == "{}" {
			rp.PersistentTaintFlags = []string{}
		} else {
			var flags []string
			if err := json.Unmarshal(aux.PersistentTaintFlags, &flags); err != nil {
				return err
			}
			rp.PersistentTaintFlags = flags
		}
	} else {
		rp.PersistentTaintFlags = []string{}
	}
	return nil
}

// GetProfile retrieves the risk profile for a keyHash from Redis, applying lazy decay if applicable.
// If no profile exists, a fresh zero-valued profile is returned without error.
func GetProfile(ctx context.Context, rdb *redis.Client, keyHash string) (*AgentRiskProfile, error) {
	if rdb == nil {
		return &AgentRiskProfile{
			KeyHash:              keyHash,
			LastDecayTime:        time.Now(),
			PersistentTaintFlags: []string{},
		}, nil
	}

	key := profileKey(keyHash)
	val, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return &AgentRiskProfile{
			KeyHash:              keyHash,
			LastDecayTime:        time.Now(),
			PersistentTaintFlags: []string{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis error getting risk profile for key %s: %w", keyHash, err)
	}

	var rp AgentRiskProfile
	if err := json.Unmarshal([]byte(val), &rp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal risk profile JSON: %w", err)
	}

	if rp.PersistentTaintFlags == nil {
		rp.PersistentTaintFlags = []string{}
	}

	if rp.LastDecayTime.IsZero() {
		rp.LastDecayTime = time.Now()
	}

	// Evaluate lazy decay: -5 points per 24h of inactivity
	if rp.RiskScore > 0 && time.Since(rp.LastDecayTime) >= 24*time.Hour {
		elapsed := time.Since(rp.LastDecayTime)
		days := int(elapsed / (24 * time.Hour))
		if days > 0 {
			rp.RiskScore -= (5 * days)
			if rp.RiskScore < 0 {
				rp.RiskScore = 0
			}
			rp.LastDecayTime = rp.LastDecayTime.Add(time.Duration(days) * 24 * time.Hour)
			_ = SaveProfile(ctx, rdb, &rp, 0)
		}
	}

	return &rp, nil
}

// SaveProfile persists the given AgentRiskProfile into Redis.
func SaveProfile(ctx context.Context, rdb *redis.Client, rp *AgentRiskProfile, ttl time.Duration) error {
	if rdb == nil || rp == nil {
		return nil
	}

	if rp.PersistentTaintFlags == nil {
		rp.PersistentTaintFlags = []string{}
	}

	data, err := json.Marshal(rp)
	if err != nil {
		return fmt.Errorf("failed to marshal risk profile: %w", err)
	}

	key := profileKey(rp.KeyHash)
	if err := rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis error saving risk profile: %w", err)
	}

	return nil
}

// Lua script for atomic risk score updates
const updateRiskScoreLua = `
local key = KEYS[1]
local delta = tonumber(ARGV[1])
local isHighRisk = ARGV[2] == "1"
local reason = ARGV[3]
local nowRFC3339 = ARGV[4]
local keyHash = ARGV[5]

local raw = redis.call('GET', key)
local doc = {}
if raw then
    doc = cjson.decode(raw)
else
    doc = {
        key_hash = keyHash,
        total_spend = 0.0,
        total_policy_blocks = 0,
        total_escalations = 0,
        persistent_taint_flags = {},
        risk_score = 0,
        last_high_risk_action = "0001-01-01T00:00:00Z",
        quarantine_until = "0001-01-01T00:00:00Z",
        last_decay_time = nowRFC3339,
        session_count = 0
    }
end

if not doc.risk_score then doc.risk_score = 0 end
if not doc.total_policy_blocks then doc.total_policy_blocks = 0 end
if not doc.total_escalations then doc.total_escalations = 0 end
if not doc.persistent_taint_flags then doc.persistent_taint_flags = {} end

-- Update risk score clamped to [0, 100]
doc.risk_score = doc.risk_score + delta
if doc.risk_score > 100 then doc.risk_score = 100 end
if doc.risk_score < 0 then doc.risk_score = 0 end

-- Update specific counters based on reason
if reason == "policy_block" then
    doc.total_policy_blocks = doc.total_policy_blocks + 1
elseif reason == "escalate" then
    doc.total_escalations = doc.total_escalations + 1
end

if isHighRisk then
    doc.last_high_risk_action = nowRFC3339
    doc.last_decay_time = nowRFC3339
end

local encoded = cjson.encode(doc)
redis.call('SET', key, encoded)
return encoded
`

// UpdateRiskScore atomically adjusts an agent's risk score by delta, updating counters and timestamps.
func UpdateRiskScore(ctx context.Context, rdb *redis.Client, keyHash string, delta int, isHighRisk bool, reason string) (*AgentRiskProfile, error) {
	if rdb == nil {
		return nil, nil
	}

	key := profileKey(keyHash)
	nowStr := time.Now().UTC().Format(time.RFC3339Nano)
	highRiskArg := "0"
	if isHighRisk {
		highRiskArg = "1"
	}

	res, err := rdb.Eval(ctx, updateRiskScoreLua, []string{key}, delta, highRiskArg, reason, nowStr, keyHash).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to update risk score in redis: %w", err)
	}

	encoded, ok := res.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected return type from updateRiskScore Lua script")
	}

	var updated AgentRiskProfile
	if err := json.Unmarshal([]byte(encoded), &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated risk profile: %w", err)
	}

	return &updated, nil
}

// Lua script for atomic taint flag addition and +5 risk score update
const addPersistentTaintFlagLua = `
local key = KEYS[1]
local flag = ARGV[1]
local nowRFC3339 = ARGV[2]
local keyHash = ARGV[3]

local raw = redis.call('GET', key)
local doc = {}
if raw then
    doc = cjson.decode(raw)
else
    doc = {
        key_hash = keyHash,
        total_spend = 0.0,
        total_policy_blocks = 0,
        total_escalations = 0,
        persistent_taint_flags = {},
        risk_score = 0,
        last_high_risk_action = "0001-01-01T00:00:00Z",
        quarantine_until = "0001-01-01T00:00:00Z",
        last_decay_time = nowRFC3339,
        session_count = 0
    }
end

if not doc.risk_score then doc.risk_score = 0 end
if not doc.persistent_taint_flags then doc.persistent_taint_flags = {} end

-- Deduplicate flag
local exists = false
for _, f in ipairs(doc.persistent_taint_flags) do
    if f == flag then
        exists = true
        break
    end
end

if not exists then
    table.insert(doc.persistent_taint_flags, flag)
    doc.risk_score = doc.risk_score + 5
    if doc.risk_score > 100 then doc.risk_score = 100 end
end

local encoded = cjson.encode(doc)
redis.call('SET', key, encoded)
return encoded
`

// AddPersistentTaintFlag appends a persistent taint flag to the profile and increments risk score by +5.
func AddPersistentTaintFlag(ctx context.Context, rdb *redis.Client, keyHash string, flag string) (*AgentRiskProfile, error) {
	if rdb == nil || flag == "" {
		return nil, nil
	}

	key := profileKey(keyHash)
	nowStr := time.Now().UTC().Format(time.RFC3339Nano)

	res, err := rdb.Eval(ctx, addPersistentTaintFlagLua, []string{key}, flag, nowStr, keyHash).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to add persistent taint flag in redis: %w", err)
	}

	encoded, ok := res.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected return type from addPersistentTaintFlag Lua script")
	}

	var updated AgentRiskProfile
	if err := json.Unmarshal([]byte(encoded), &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated risk profile: %w", err)
	}

	return &updated, nil
}

// Lua script for atomic session count increment
const incrementSessionCountLua = `
local key = KEYS[1]
local nowRFC3339 = ARGV[1]
local keyHash = ARGV[2]

local raw = redis.call('GET', key)
local doc = {}
if raw then
    doc = cjson.decode(raw)
else
    doc = {
        key_hash = keyHash,
        total_spend = 0.0,
        total_policy_blocks = 0,
        total_escalations = 0,
        persistent_taint_flags = {},
        risk_score = 0,
        last_high_risk_action = "0001-01-01T00:00:00Z",
        quarantine_until = "0001-01-01T00:00:00Z",
        last_decay_time = nowRFC3339,
        session_count = 0
    }
end

if not doc.session_count then doc.session_count = 0 end
doc.session_count = doc.session_count + 1

local encoded = cjson.encode(doc)
redis.call('SET', key, encoded)
return encoded
`

// IncrementSessionCount atomically increments the session count for the keyHash.
func IncrementSessionCount(ctx context.Context, rdb *redis.Client, keyHash string) error {
	if rdb == nil {
		return nil
	}

	key := profileKey(keyHash)
	nowStr := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := rdb.Eval(ctx, incrementSessionCountLua, []string{key}, nowStr, keyHash).Result()
	if err != nil {
		return fmt.Errorf("failed to increment session count in redis: %w", err)
	}

	return nil
}

// Lua script for atomic lifetime spend update
const addLifetimeSpendLua = `
local key = KEYS[1]
local cost = tonumber(ARGV[1])
local nowRFC3339 = ARGV[2]
local keyHash = ARGV[3]

local raw = redis.call('GET', key)
local doc = {}
if raw then
    doc = cjson.decode(raw)
else
    doc = {
        key_hash = keyHash,
        total_spend = 0.0,
        total_policy_blocks = 0,
        total_escalations = 0,
        persistent_taint_flags = {},
        risk_score = 0,
        last_high_risk_action = "0001-01-01T00:00:00Z",
        quarantine_until = "0001-01-01T00:00:00Z",
        last_decay_time = nowRFC3339,
        session_count = 0
    }
end

if not doc.total_spend then doc.total_spend = 0.0 end
doc.total_spend = doc.total_spend + cost

local encoded = cjson.encode(doc)
redis.call('SET', key, encoded)
return encoded
`

// AddLifetimeSpend atomically increments the total spend recorded for an agent.
func AddLifetimeSpend(ctx context.Context, rdb *redis.Client, keyHash string, cost float64) error {
	if rdb == nil || cost <= 0 {
		return nil
	}

	key := profileKey(keyHash)
	nowStr := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := rdb.Eval(ctx, addLifetimeSpendLua, []string{key}, cost, nowStr, keyHash).Result()
	if err != nil {
		return fmt.Errorf("failed to add lifetime spend in redis: %w", err)
	}

	return nil
}
