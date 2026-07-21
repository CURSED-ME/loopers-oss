-- lua_loop_fingerprint.lua
-- KEYS[1]: fingerprint ring key  (loopers:loop:fp:{session_id})
-- ARGV[1]: new_item (base64 string)
-- ARGV[2]: now_unix (current unix timestamp as string)
-- ARGV[3]: window_seconds (TTL of the sliding window)
-- ARGV[4]: member_id

local ring_key      = KEYS[1]
local new_item      = ARGV[1]
local now           = tonumber(ARGV[2])
local window        = tonumber(ARGV[3])
local member_id     = ARGV[4]
local cutoff        = now - window

-- 1. Trim expired entries (outside the window)
redis.call('ZREMRANGEBYSCORE', ring_key, '-inf', cutoff)

-- 1b. Limit max elements to 1000 to prevent OOM
local count = redis.call('ZCARD', ring_key)
if count >= 1000 then
    redis.call('ZREMRANGEBYRANK', ring_key, 0, count - 1000)
end

-- 2. Fetch all existing active window members
local members = redis.call('ZRANGE', ring_key, 0, -1)

-- 3. Add current item with timestamp score
redis.call('ZADD', ring_key, now, new_item .. ':' .. member_id)

-- 4. Set TTL
redis.call('EXPIRE', ring_key, window + 10)

return members

