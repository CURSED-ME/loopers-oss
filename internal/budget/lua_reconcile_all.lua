-- lua_reconcile_all.lua
-- Atomic reconciliation for multiple windows.
-- KEYS[1..N]: spend keys
-- ARGV[1]: reserved_cost as string (e.g., "0.0125")
-- ARGV[2]: actual_cost as string (e.g., "0.0080")
-- ARGV[3..N+2]: window TTLs in seconds

local num_windows = #KEYS
local reserved_cost = tonumber(ARGV[1])
local actual_cost   = tonumber(ARGV[2])

local delta = actual_cost - reserved_cost

if delta ~= 0 then
    local delta_str = string.format("%.0f", delta)
    for i = 1, num_windows do
        local spend_key = KEYS[i]
        local window_ttl = tonumber(ARGV[i + 2])
        redis.call('INCRBY', spend_key, delta_str)
        redis.call('EXPIRE', spend_key, window_ttl)
    end
end

return {1, "ok"}
