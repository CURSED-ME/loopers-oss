-- lua_loop_fingerprint.lua
-- KEYS[1]: fingerprint ring key  (loopers:loop:fp:{session_id})
-- ARGV[1]: hash (hex string)
-- ARGV[2]: now_unix (current unix timestamp as string)
-- ARGV[3]: window_seconds (TTL of the sliding window)
-- ARGV[4]: threshold (number of identical hashes to trigger a loop)

local ring_key      = KEYS[1]
local hash          = ARGV[1]
local now           = tonumber(ARGV[2])
local window        = tonumber(ARGV[3])
local threshold     = tonumber(ARGV[4])
local cutoff        = now - window

-- 1. Trim expired entries (outside the window)
redis.call('ZREMRANGEBYSCORE', ring_key, '-inf', cutoff)

-- 2. Add the current hash with current timestamp as score
--    Using ZADD so we add unique (timestamp, hash) pairs.
--    We use hash+now as the unique member to allow the same hash at different times.
redis.call('ZADD', ring_key, now, hash .. ':' .. tostring(now))

-- 3. Set TTL on the ring key to avoid orphaned keys
redis.call('EXPIRE', ring_key, window + 10)

-- 4. Count how many times this specific hash appears in the window
local members = redis.call('ZRANGE', ring_key, 0, -1)
local count = 0
for _, m in ipairs(members) do
    if string.sub(m, 1, string.len(hash)) == hash then
        count = count + 1
    end
end

-- 5. Return count and whether threshold is exceeded
if count >= threshold then
    return {1, count}   -- 1 = loop detected
end
return {0, count}       -- 0 = no loop
