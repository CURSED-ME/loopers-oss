-- lua_lease_heartbeat.lua
local lease_id = ARGV[1]
local spend_amount = tonumber(ARGV[2])

-- 1. Refresh active TTL
local exists = redis.call('EXISTS', 'loopers:lease:' .. lease_id .. ':amount')
if exists == 0 then
    return {0} -- Lease is dead/reclaimed
end

redis.call('SET', 'loopers:lease:' .. lease_id .. ':active', '1', 'EX', 15)

-- 2. Convert reserved to spent
if spend_amount > 0 then
    local amt_key = 'loopers:lease:' .. lease_id .. ':amount'
    local current_reserved = tonumber(redis.call('GET', amt_key) or "0")
    if spend_amount > current_reserved then
        spend_amount = current_reserved
    end
    
    if spend_amount > 0 then
        -- Use INCRBY with a negative spend_amount
        redis.call('INCRBY', amt_key, -spend_amount)
        local keys = redis.call('SMEMBERS', 'loopers:lease:' .. lease_id .. ':keys')
        for _, spend_key in ipairs(keys) do
            -- Use INCRBY with a negative spend_amount
            redis.call('INCRBY', spend_key .. ':reserved', -spend_amount)
            redis.call('INCRBY', spend_key, spend_amount)
        end
    end
end
return {1}
