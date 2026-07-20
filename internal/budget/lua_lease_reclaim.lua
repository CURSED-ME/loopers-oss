-- lua_lease_reclaim.lua
-- Scans active_leases. For each, if the :active key is missing, reclaims it.
-- This can be called periodically by any proxy.
local leases = redis.call('SMEMBERS', 'loopers:active_leases')
local reclaimed_count = 0

for _, lease_id in ipairs(leases) do
    local is_active = redis.call('EXISTS', 'loopers:lease:' .. lease_id .. ':active')
    if is_active == 0 then
        -- Reclaim it
        local amt_key = 'loopers:lease:' .. lease_id .. ':amount'
        local remaining = tonumber(redis.call('GET', amt_key) or "0")
        
        if remaining > 0 then
            local keys = redis.call('SMEMBERS', 'loopers:lease:' .. lease_id .. ':keys')
            for _, spend_key in ipairs(keys) do
                redis.call('INCRBY', spend_key .. ':reserved', -remaining)
            end
        end
        
        -- Cleanup
        redis.call('DEL', amt_key)
        redis.call('DEL', 'loopers:lease:' .. lease_id .. ':keys')
        redis.call('SREM', 'loopers:active_leases', lease_id)
        reclaimed_count = reclaimed_count + 1
    end
end

return {reclaimed_count}
