package session

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/redis/go-redis/v9"
)

var validSessionID = regexp.MustCompile(`^[a-zA-Z0-9._:@/-]{1,256}$`)

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
func (m *Manager) EnforceAbsoluteTTL(ctx context.Context, sessionID string, maxTTLSeconds int) (bool, error) {
	if maxTTLSeconds <= 0 {
		return true, nil
	}

	createdKey := fmt.Sprintf("loopers:session:%s:created", sessionID)
	now := time.Now().Unix()

	// Try to set the creation time only if it doesn't exist
	// We set a very long TTL (e.g., 7 days) so we remember it expired,
	// but it doesn't live in Redis forever.
	set, err := m.rdb.SetNX(ctx, createdKey, now, 7*24*time.Hour).Result()
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
func (m *Manager) CheckBlastRadius(ctx context.Context, sessionID, serverName string, maxServers int) (bool, error) {
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

	serversKey := fmt.Sprintf("loopers:session:%s:servers", sessionID)

	res, err := m.rdb.Eval(ctx, luaBlastRadius, []string{serversKey}, serverName, maxServers).Result()
	if err != nil {
		return false, fmt.Errorf("redis error in blast radius script: %w", err)
	}

	allowed := res.(int64) == 1
	return allowed, nil
}
