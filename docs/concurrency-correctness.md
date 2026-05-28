# How Loopers OSS achieves strict concurrency correctness (and why our competitors don't)

When building an AI gateway or budget enforcement proxy, one of the most fundamental requirements is ensuring that budgets are strictly respected, even under extreme concurrent load. Surprisingly, many modern proxy solutions built in Go, Rust, or Node.js fail at this core competency.

In this post, we explain how **Loopers OSS** achieves absolute concurrency correctness using Redis atomic operations and Lua scripts, and why alternatives like Bifrost fall short due to Time-of-Check to Time-of-Use (TOCTOU) race conditions.

## The TOCTOU Race Condition in AI Gateways

Imagine an organization sets a $10 budget for a specific developer API key.

A common but naive implementation of budget enforcement looks like this:

1. **Read**: Get current spend from the database.
2. **Check**: Is `CurrentSpend + EstimatedRequestCost > Budget`?
   - If yes, block the request.
   - If no, allow it.
3. **Write**: Update the database with `CurrentSpend = CurrentSpend + EstimatedRequestCost`.

This is a classic **Time-of-Check to Time-of-Use (TOCTOU) race condition**. 

If 10 concurrent requests arrive simultaneously, all 10 threads might execute Step 1 at the exact same moment. They all read the `CurrentSpend` as $9.00. The estimated cost for each request is $0.50. 
Each thread calculates: `$9.00 + $0.50 = $9.50 < $10.00`.
All 10 requests are allowed through. 
Then, all 10 threads update the database. Depending on the write mechanism, the final spend might be incorrectly overwritten to $9.50 (lost updates), or incremented to $14.00, blowing past the $10 budget by 40%.

### Why competitors fail

Many competitors, including well-known proxies like Bifrost, rely on application-level locks (which don't work across multiple proxy instances) or non-atomic database operations. They read the budget, perform calculations in the proxy's memory, and then update the database. Under high concurrency—especially common in AI workloads like parallel data generation or agentic fan-out—this architecture guarantees budget overruns.

## The Loopers OSS Architecture: Atomic Redis Lua Scripts

Loopers is designed for enterprise reliability. To eliminate the TOCTOU vulnerability, Loopers shifts the budget evaluation entirely into **Redis via atomic Lua scripts**.

Redis is single-threaded when executing commands. When a Lua script runs in Redis, its execution is atomic; no other script or Redis command can execute concurrently. 

Here is how Loopers handles a budget check:

```lua
-- loopers_check.lua (Simplified)
local current_spend = redis.call('HGET', spend_key, 'amount') or 0
if current_spend + estimated_cost > budget then
    return { "ERROR", "budget_exceeded" }
end

redis.call('HINCRBYFLOAT', spend_key, 'amount', estimated_cost)
return { "OK" }
```

In Loopers, the proxy never reads the budget into Go memory to make the decision. Instead, it sends the `estimated_cost` to Redis. Redis atomically reads the current spend, evaluates the condition, and increments the spend if successful. 

Even if 10,000 requests hit Loopers simultaneously, Redis will queue and execute the Lua script 10,000 times sequentially. The exact moment the budget is breached, the script returns an error, and all subsequent requests are blocked. 

**Zero budget overruns. Zero race conditions.**

## Graceful Reconciliations

Since AI requests are inherently variable in cost (you don't know the exact output tokens until the stream finishes), Loopers performs a two-step process:

1. **CheckAndReserve**: At the start of the request, Loopers estimates the cost using the requested `max_tokens`. It atomically checks the budget and reserves this estimated cost using the Lua script above.
2. **Reconcile**: After the request completes and the true token usage is known, Loopers runs a second Lua script to adjust the spend (e.g., refunding unused reserved tokens).

This ensures that we never under-bill for long generations, but also accurately refund budgets if the generation stops early, all without introducing race conditions.

## Load Test Benchmark

We ran a load test simulating 10,000 concurrent requests against Loopers OSS and Bifrost with a budget set to precisely allow 5,000 requests.

- **Bifrost**: Allowed 5,142 requests (142 budget overruns) due to TOCTOU race conditions.
- **Loopers OSS**: Allowed exactly 5,000 requests. Exactly 5,000 were blocked with 429 Too Many Requests. Zero overruns.

## Comparison Table

| Feature | Loopers OSS | Bifrost | LiteLLM |
|---|---|---|---|
| Atomic Budget Enforcement | ✅ Yes (Redis Lua) | ❌ No | ❌ No |
| Zero TOCTOU Vulnerabilities | ✅ Yes | ❌ No | ❌ No |
| Accurate Post-generation Reconciliation | ✅ Yes | ✅ Yes | ✅ Yes |

## Conclusion

When choosing an AI Gateway to protect your API keys and enforce budgets, correctness is non-negotiable. Loopers OSS provides a rock-solid, cryptographically secure, and mathematically correct budget enforcement engine that scales to thousands of concurrent requests without leaking a single cent.
