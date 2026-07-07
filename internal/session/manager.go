package session

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/redis/go-redis/v9"
)

var validSessionID = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,256}$`)

// IsValidID checks if a session ID conforms to the allowed format.
func IsValidID(sessionID string) bool {
	return validSessionID.MatchString(sessionID)
}

type Manager struct {
	rdb *redis.Client
}

func NewManager(rdb *redis.Client) *Manager {
	return &Manager{rdb: rdb}
}

// EnforceAbsoluteTTL ensures a session does not exceed its maximum lifetime.
// It returns true if the session is valid (or newly created), false if it has expired.
func (m *Manager) EnforceAbsoluteTTL(ctx context.Context, keyHash string, sessionID string, maxTTLSeconds int) (bool, error) {
	if maxTTLSeconds <= 0 {
		return true, nil
	}

	createdKey := fmt.Sprintf("loopers:session:%s:%s:created", keyHash, sessionID)
	now := time.Now().Unix()

	// We set the TTL to max(7 days, maxTTLSeconds + 1 day) to ensure
	// the key doesn't prematurely expire while the session might still be active.
	// If it expired, it would reset the start time and bypass the limit.
	ttl := 7 * 24 * time.Hour
	requiredTTL := time.Duration(maxTTLSeconds)*time.Second + (24 * time.Hour)
	if requiredTTL > ttl {
		ttl = requiredTTL
	}

	set, err := m.rdb.SetNX(ctx, createdKey, now, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis error checking session TTL: %w", err)
	}

	if set {
		// New session, valid
		return true, nil
	}

	// Session already exists, check its creation time
	createdAt, err := m.rdb.Get(ctx, createdKey).Int64()
	if err != nil {
		return false, fmt.Errorf("redis error getting session creation time: %w", err)
	}

	if now-createdAt > int64(maxTTLSeconds) {
		// Absolute TTL exceeded
		return false, nil
	}

	return true, nil
}

// CheckBlastRadius tracks the number of distinct MCP servers a session has contacted.
// It returns false if adding the current server would exceed maxServers.
func (m *Manager) CheckBlastRadius(ctx context.Context, keyHash string, sessionID, serverName string, maxServers int) (bool, error) {
	if maxServers <= 0 {
		return true, nil
	}

	const luaBlastRadius = `
local key = KEYS[1]
local server = ARGV[1]
local max = tonumber(ARGV[2])

-- Check count BEFORE adding
local current = redis.call('SCARD', key)
if current >= max and redis.call('SISMEMBER', key, server) == 0 then
    return 0
end
redis.call('SADD', key, server)
redis.call('EXPIRE', key, 604800) -- 7 days
return 1
`

	serversKey := fmt.Sprintf("loopers:session:%s:%s:servers", keyHash, sessionID)

	res, err := m.rdb.Eval(ctx, luaBlastRadius, []string{serversKey}, serverName, maxServers).Result()
	if err != nil {
		return false, fmt.Errorf("redis error in blast radius script: %w", err)
	}

	allowed := res.(int64) == 1
	return allowed, nil
}

// TrackAndLimitSessions tracks the number of unique active sessions for a keyHash.
// It returns false if the keyHash has exceeded the maximum number of concurrent sessions.
func (m *Manager) TrackAndLimitSessions(ctx context.Context, keyHash, sessionID string, maxSessions int) (bool, error) {
	if maxSessions <= 0 {
		return true, nil
	}

	// We use a ZSET to track sessions with their last seen timestamp.
	// We first remove expired sessions, then check the count.
	const luaSessionTracker = `
local key = KEYS[1]
local session_id = ARGV[1]
local now = tonumber(ARGV[2])
local max = tonumber(ARGV[3])
local ttl_cutoff = tonumber(ARGV[4])

-- Clean up expired sessions (older than 24h by default, matching standard TTLs)
redis.call('ZREMRANGEBYSCORE', key, '-inf', ttl_cutoff)

-- Is this session already in the set?
local score = redis.call('ZSCORE', key, session_id)
if score then
    -- Already active, just update its timestamp
    redis.call('ZADD', key, now, session_id)
    redis.call('EXPIRE', key, 86400)
    return 1
end

-- It's a new session. Check if we're at the limit.
local count = redis.call('ZCARD', key)
if count >= max then
    return 0
end

-- Add the new session
redis.call('ZADD', key, now, session_id)
redis.call('EXPIRE', key, 86400)
return 1
`
	sessionsKey := fmt.Sprintf("loopers:key:%s:active_sessions", keyHash)
	now := time.Now().Unix()
	ttlCutoff := now - 86400 // Assumes active sessions expire if unseen for 24h

	res, err := m.rdb.Eval(ctx, luaSessionTracker, []string{sessionsKey}, sessionID, now, maxSessions, ttlCutoff).Result()
	if err != nil {
		return false, fmt.Errorf("redis error in session tracker script: %w", err)
	}

	allowed := res.(int64) == 1
	return allowed, nil
}
