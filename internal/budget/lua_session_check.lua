-- lua_session_check.lua
-- KEYS[1]: spend key       (loopers:session:{session_id}:spend)
-- KEYS[2]: budget key      (loopers:session:{session_id}:budget)
-- KEYS[3]: steps key       (loopers:session:{session_id}:steps)
-- KEYS[4]: max_steps key   (loopers:session:{session_id}:max_steps)
-- ARGV[1]: est_cost        (e.g., "0.0125")
-- ARGV[2]: input_budget    (e.g., "5.00" or "")
-- ARGV[3]: input_max_steps (e.g., "25" or "")
-- ARGV[4]: ttl             (e.g., "3600")

local spend_key = KEYS[1]
local budget_key = KEYS[2]
local steps_key = KEYS[3]
local max_steps_key = KEYS[4]

local est_cost = tonumber(ARGV[1])
local input_budget = ARGV[2]
local input_max_steps = ARGV[3]
local ttl = tonumber(ARGV[4])

-- 1. Resolve budget limit
local budget = nil
if input_budget ~= "" then
    budget = tonumber(input_budget)
    redis.call('SET', budget_key, input_budget)
    redis.call('EXPIRE', budget_key, ttl)
else
    local saved = redis.call('GET', budget_key)
    if saved then
        budget = tonumber(saved)
        redis.call('EXPIRE', budget_key, ttl)
    end
end

-- 2. Resolve max steps limit
local max_steps = nil
if input_max_steps ~= "" then
    max_steps = tonumber(input_max_steps)
    redis.call('SET', max_steps_key, input_max_steps)
    redis.call('EXPIRE', max_steps_key, ttl)
else
    local saved = redis.call('GET', max_steps_key)
    if saved then
        max_steps = tonumber(saved)
        redis.call('EXPIRE', max_steps_key, ttl)
    end
end

-- 3. Check spend budget
local current_spend = 0
local saved_spend = redis.call('GET', spend_key)
if saved_spend then
    current_spend = tonumber(saved_spend)
end

if budget and (current_spend + est_cost) > budget then
    return {0, tostring(current_spend), tostring(budget), "session_budget_exceeded"}
end

-- 4. Check step count (Note: steps are incremented ONLY if we don't block)
local current_steps = 0
local saved_steps = redis.call('GET', steps_key)
if saved_steps then
    current_steps = tonumber(saved_steps)
end

if max_steps and (current_steps + 1) > max_steps then
    return {0, tostring(current_steps), tostring(max_steps), "session_steps_exceeded"}
end

-- 5. Commit reservation (increment spend and step count)
local new_spend = current_spend + est_cost
redis.call('SET', spend_key, tostring(new_spend))
redis.call('EXPIRE', spend_key, ttl)

local new_steps = current_steps + 1
redis.call('SET', steps_key, tostring(new_steps))
redis.call('EXPIRE', steps_key, ttl)

return {1, tostring(new_spend), tostring(new_steps), "ok"}
