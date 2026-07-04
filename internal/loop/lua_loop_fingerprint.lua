-- lua_loop_fingerprint.lua
-- KEYS[1]: fingerprint ring key  (loopers:loop:fp:{session_id})
-- ARGV[1]: hash (hex string)
-- ARGV[2]: now_unix (current unix timestamp as string)
-- ARGV[3]: window_seconds (TTL of the sliding window)
-- ARGV[4]: threshold (number of identical hashes to trigger a loop)
-- ARGV[5]: member_id
-- ARGV[6]: max_distance

local ring_key      = KEYS[1]
local hash          = ARGV[1]
local now           = tonumber(ARGV[2])
local window        = tonumber(ARGV[3])
local threshold     = tonumber(ARGV[4])
local member_id     = ARGV[5]
local max_distance  = tonumber(ARGV[6])
local cutoff        = now - window

-- 1. Trim expired entries (outside the window)
redis.call('ZREMRANGEBYSCORE', ring_key, '-inf', cutoff)

-- 2. Add the current hash with current timestamp as score
--    Using ZADD so we add unique (timestamp, hash) pairs.
redis.call('ZADD', ring_key, now, hash .. ':' .. member_id)

-- 3. Set TTL on the ring key to avoid orphaned keys
redis.call('EXPIRE', ring_key, window + 10)

-- 4. Count how many times this specific hash appears in the window
local popcount = {
  [0]=0,[1]=1,[2]=1,[3]=2,[4]=1,[5]=2,[6]=2,[7]=3,
  [8]=1,[9]=2,[10]=2,[11]=3,[12]=2,[13]=3,[14]=3,[15]=4
}

local function bxor_4bit(a,b)
  local c = 0
  local p = 1
  for i=1,4 do
    local a_bit = a % 2
    local b_bit = b % 2
    if a_bit ~= b_bit then c = c + p end
    a = math.floor(a/2)
    b = math.floor(b/2)
    p = p * 2
  end
  return c
end

local function hex_hamming(a, b)
  local dist = 0
  for i = 1, #a do
    local xa = tonumber(string.sub(a, i, i), 16)
    local xb = tonumber(string.sub(b, i, i), 16)
    if xa and xb then
      dist = dist + popcount[bxor_4bit(xa, xb)]
    end
  end
  return dist
end

local members = redis.call('ZRANGE', ring_key, 0, -1)
local count = 0
for _, m in ipairs(members) do
    local hash_part = string.sub(m, 1, 16)
    if hex_hamming(hash_part, hash) <= max_distance then
        count = count + 1
    end
end

-- 5. Return count and whether threshold is exceeded
if count >= threshold then
    return {1, count}   -- 1 = loop detected
end
return {0, count}       -- 0 = no loop
