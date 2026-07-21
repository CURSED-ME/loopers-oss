-- lua_lease_acquire.lua
local lease_id = ARGV[1]
local req_chunk_str = ARGV[2]
local req_chunk = tonumber(req_chunk_str)
local num_windows = tonumber(ARGV[3])

local spend_keys = {}
local limits = {}
local ttls = {}

local idx = 4
for i = 1, num_windows do
    spend_keys[i] = ARGV[idx]; idx = idx + 1
    limits[i] = tonumber(ARGV[idx]); idx = idx + 1
    ttls[i] = tonumber(ARGV[idx]); idx = idx + 1
end

local min_available = req_chunk

-- Pass 1: Find max available chunk
for i = 1, num_windows do
    if limits[i] and limits[i] > 0 then
        local spent = tonumber(redis.call('GET', spend_keys[i]) or "0")
        local reserved = tonumber(redis.call('GET', spend_keys[i] .. ':reserved') or "0")
        local avail = limits[i] - spent - reserved
        if avail < min_available then
            min_available = avail
        end
    end
end

if min_available <= 0 then
    return {0, "0"}
end

local min_avail_str = string.format("%.0f", min_available)

-- Pass 2: Reserve the chunk
for i = 1, num_windows do
    local rkey = spend_keys[i] .. ':reserved'
    redis.call('INCRBY', rkey, min_avail_str)
    redis.call('EXPIRE', rkey, ttls[i])
    -- ensure spend key has TTL
    redis.call('EXPIRE', spend_keys[i], ttls[i])
    -- Save this key to the lease's list of keys
    redis.call('SADD', 'loopers:lease:' .. lease_id .. ':keys', spend_keys[i])
    redis.call('EXPIRE', 'loopers:lease:' .. lease_id .. ':keys', 86400)
end

redis.call('SET', 'loopers:lease:' .. lease_id .. ':amount', min_avail_str, 'EX', 86400)
redis.call('SET', 'loopers:lease:' .. lease_id .. ':active', '1', 'EX', 15)
redis.call('SADD', 'loopers:active_leases', lease_id)

return {1, min_avail_str}
