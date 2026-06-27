local ring_key      = KEYS[1]
local hash          = ARGV[1]
local now           = tonumber(ARGV[2])
local window        = tonumber(ARGV[3])
local threshold     = tonumber(ARGV[4])
local member_id     = ARGV[5]
local cutoff        = now - window

-- Remove expired elements
redis.call('ZREMRANGEBYSCORE', ring_key, '-inf', cutoff)
-- Add current request
redis.call('ZADD', ring_key, now, hash .. ':' .. member_id)
-- Set TTL slightly larger than window to ensure cleanup
redis.call('EXPIRE', ring_key, window + 10)

-- Count elements in window
local count = redis.call('ZCARD', ring_key)

local allowed = 1
if count > threshold then
    allowed = 0
end

local remaining = threshold - count
if remaining < 0 then
    remaining = 0
end

return {allowed, remaining}
