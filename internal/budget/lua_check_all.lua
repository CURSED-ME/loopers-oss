-- lua_check_all.lua
-- Atomic budget check + reservation for multiple windows.
-- KEYS[1..N]: spend keys
-- ARGV[1]: estimated_cost as string (e.g., "0.0125")
-- ARGV[2..N+1]: window names
-- ARGV[N+2..2N+1]: limits
-- ARGV[2N+2..3N+1]: window TTLs in seconds

local est_cost_str = ARGV[1]
local est_cost_num = tonumber(est_cost_str)
local num_windows = #KEYS

-- First pass: check limits for all windows
for i = 1, num_windows do
    local spend_key = KEYS[i]
    local window_name = ARGV[i+1]
    local limit_str = ARGV[i + 1 + num_windows]
    
    if limit_str and limit_str ~= "" and limit_str ~= "0" then
        local limit = tonumber(limit_str)
        local current_str = redis.call('GET', spend_key)
        local current = 0
        if current_str then
            current = tonumber(current_str)
        end
        
        local projected = current + est_cost_num
        if projected > limit then
            -- BLOCK: return as strings to preserve integer precision without scientific notation
            return {0, string.format("%.0f", current), string.format("%.0f", limit), window_name}
        end
    end
end

-- Second pass: RESERVE for all windows
for i = 1, num_windows do
    local spend_key = KEYS[i]
    local window_ttl = tonumber(ARGV[i + 1 + (num_windows * 2)])
    redis.call('INCRBY', spend_key, est_cost_str)
    redis.call('EXPIRE', spend_key, window_ttl)
end

return {1, "0", "0", "ok"}
