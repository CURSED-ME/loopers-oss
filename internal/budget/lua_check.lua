-- lua_check.lua
-- Atomic budget check + reservation for a single window.
-- KEYS[1]: spend key  (loopers:spend:{hash}:daily:2026-05-22)
-- KEYS[2]: config key (loopers:budget:{hash}:config)
-- ARGV[1]: estimated_cost as string (e.g., "0.0125")
-- ARGV[2]: window name ("daily" or "hourly")
-- ARGV[3]: window TTL in seconds

local spend_key    = KEYS[1]
local config_key   = KEYS[2]
local est_cost_str = ARGV[1]
local est_cost     = tonumber(est_cost_str)
local window_name  = ARGV[2]
local window_ttl   = tonumber(ARGV[3])

-- Read budget limit from config
local limit_str = redis.call('HGET', config_key, window_name)
if not limit_str then
    -- No budget configured for this window -> allow
    return {1, "0", "0", "no_budget"}
end
-- The config limit is in float USD (e.g., "20.00"). Multiply by 10^9 to get Nano-USD.
local limit = tonumber(limit_str) * 1000000000

-- Read current spend
local current_str = redis.call('GET', spend_key)
local current = 0
if current_str then
    current = tonumber(current_str)
end

-- Atomic check
local projected = current + est_cost
if projected > limit then
    -- BLOCK: return as strings to preserve integer precision without scientific notation
    return {0, string.format("%.0f", current), string.format("%.0f", limit), "budget_exceeded"}
end

-- RESERVE: increment and set TTL in one atomic operation
redis.call('INCRBY', spend_key, est_cost_str)
redis.call('EXPIRE', spend_key, window_ttl)

return {1, string.format("%.0f", projected), string.format("%.0f", limit), "ok"}
