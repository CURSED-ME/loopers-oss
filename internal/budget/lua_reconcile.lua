-- lua_reconcile.lua
-- Post-stream reconciliation.
-- KEYS[1]: spend key (e.g., loopers:spend:{hash}:daily:2026-05-22)
-- ARGV[1]: reserved_cost as string (e.g., "0.0125")
-- ARGV[2]: actual_cost as string (e.g., "0.0080")
-- ARGV[3]: window_ttl in seconds

local spend_key     = KEYS[1]
local reserved_cost = tonumber(ARGV[1])
local actual_cost   = tonumber(ARGV[2])
local window_ttl    = tonumber(ARGV[3])

local delta = actual_cost - reserved_cost

if delta ~= 0 then
    redis.call('INCRBY', spend_key, string.format("%.0f", delta))
    redis.call('EXPIRE', spend_key, window_ttl)
end

-- Return the new spend
local current_str = redis.call('GET', spend_key)
local current = 0
if current_str then
    current = tonumber(current_str)
end

return {1, string.format("%.0f", current)}
