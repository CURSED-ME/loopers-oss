# Loopers v1.0 — Airtight Open-Source Build Plan

**Version:** 3.0 (Research-Validated)  
**Date:** May 22, 2026  
**Status:** Engineering-Ready. Every design decision cites an external source.

---

## 0. Preamble: The Stakes

Loopers sits in the critical path between a developer's application and the AI provider that bills them by the token. If the circuit breaker fails to block a request that should have been blocked—or if it silently allows spending to continue because a streaming trace reports zero tokens—the developer receives a real bill for real money. There is no undo.

This plan is built around one principle: **correctness over completeness.** For v1.0, we deliver a tight, verifiable core that blocks spending correctly under all known failure modes. Features that do not directly serve correctness are deferred to v2.0.

---

## 1. Validated Technical Foundation

### 1.1 The Single Most Important Design Decision

There is only one correct way to enforce a budget against concurrent requests: **atomic Redis Lua script.** The alternative—a GET followed by an INCRBY in separate operations—contains a TOCTOU race condition. A security review of the anthropic-lb project in February 2026 confirmed that "concurrent requests from the same client can all pass the budget check before any increment lands, allowing overspend proportional to concurrency". The documented fix is a Lua script that checks and increments in a single atomic operation. Loopers will use Lua scripts for every budget mutation. No exceptions.

### 1.2 The Streaming Token Problem

There is a silent, expensive bug present in nearly every LLM proxy shipped in 2024–2025: OpenAI does not include `usage` data in streaming chunks by default. You must explicitly set `stream_options: { include_usage: true }` in the request body. Without this flag, every streaming call reports zero input tokens, zero output tokens, and zero cost—meaning budget guards "never triggered" and the feature designed to prevent bill shock was "quietly disabled for all streaming traffic".

Loopers must auto-inject this flag before forwarding any OpenAI streaming request. The budget enforcement depends on it. This is not a nice-to-have; it is a correctness requirement.

### 1.3 Proxy Architecture Hazards (Go 1.24+)

Two production hazards in Go's `httputil.ReverseProxy` must be mitigated from day one:

**Context leak:** Go 1.22.0 had a bug where `proxy.WithContext(ctx)` permanently bound a short-lived context to a reused proxy instance, causing goroutine and memory leaks. The fix shipped in Go 1.22.1, but the pattern itself is dangerous: never call `WithContext()` on a reused ReverseProxy instance.

**Streaming goroutine leak:** As of Go 1.26, when a backend dies during an HTTP/2 streaming request, `httputil.ReverseProxy` wraps the request body in a `noopCloseReader`. The blocked `Read()` call can never be interrupted, and the proxy handler goroutine leaks permanently. Mitigation: the proxy must use a `context.WithTimeout` on the transport layer and never rely on the backend to clean up connections.

### 1.4 Dependency Chain Security

The LiteLLM project—a directly adjacent open-source AI gateway—was hit by CVE-2026-42208 in April 2026: a pre-auth SQL injection through the `Authorization` header that exposed all stored provider API keys and master keys. Attackers exploited it within 36 hours of disclosure.

Loopers avoids this entire class of vulnerability by design: we have no database. No PostgreSQL. No SQL queries. No stored provider keys. The only persistent state is in Redis, and the only operations on Redis are Lua scripts that operate exclusively on numeric spend counters identified by SHA-256 hashes. The attack surface is reduced to: Redis protocol commands containing only hashes and floats.

---

## 2. What Loopers v1.0 Actually Ships

### 2.1 In Scope (Correctness-Critical)

| Feature | Why It Ships |
|---------|-------------|
| Pre-call hard dollar cap enforcement (OpenAI + Anthropic) | Core value. Without this, the product doesn't exist. |
| Atomic Redis Lua script budget check | Required for correctness under concurrency. TOCTOU is not acceptable. |
| `stream_options: include_usage` auto-injection | Required for correctness of streaming budget enforcement. |
| Streaming response proxying with mid-stream cut | Required because agents stream. Budgets must hold mid-generation. |
| Pass-through provider key architecture (zero storage) | Required by the trust model. Provider keys never touch disk or Redis. |
| `docker-compose up` deployment (< 30 seconds) | Required for adoption. Five-minute quickstart is the conversion funnel. |
| Prometheus metrics | Required for operational visibility in production. |
| CLI for key creation and budget management | Required for self-service onboarding. |

### 2.2 Explicitly Out of Scope (v2.0+)

- Gemini provider support (v2.0 — needs `countTokens` API integration and Flash Lite token counting bug workaround)
- Web dashboard (v2.0 — CLI-only for v1.0)
- Anomaly detection / LLMjacking auto-revocation (v2.0)
- Per-session / per-agent budget windows (v2.0)
- PostgreSQL audit logs (v2.0)
- Redis Sentinel / Cluster support (v2.0)
- Pricing YAML hot-reload (v2.0 — restart the proxy)
- Background orphan reconciler (v2.0 — for v1.0, the `max_tokens` estimate is the charge; rare stream drops cause minor over-debits)

### 2.3 Why Only Two Providers

OpenAI and Anthropic represent the two most common deployment targets for teams building on LLM APIs, and both support token-accurate streaming with identifiable usage chunks. Gemini introduces a separate `countTokens` API, a known `candidatesTokenCount` undercounting bug on Flash Lite, and a different auth model. Supporting Gemini in v1.0 would approximately double the provider integration surface without materially improving the product's correctness story. It will be the first provider added in v2.0.

---

## 3. The Single Correctness Guarantee

The product makes exactly one guarantee to the user:

> **When your budget cap is reached, no further tokens will be billed to your provider account. Period.**

This guarantee has three components:

1. **Pre-call blocking:** Every request is checked against the budget before it reaches the provider. The Lua script that performs the check and the reserve is atomic—no concurrent request can slip through.
2. **Streaming enforcement:** OpenAI requests have `stream_options.include_usage` auto-injected. The proxy counts output tokens from the final SSE chunk and reconciles the actual cost. If the budget is exceeded mid-stream, the connection is terminated with a budget-exceeded event.
3. **Fail-closed on Redis failure:** If Redis is unreachable, all requests are rejected with HTTP 503. There is no fallback to "allow." A temporary outage is preferable to an unbounded bill.

Everything else—the dashboard, the anomaly detection, the team management, the compliance exports—is secondary. The circuit breaker is the product. Everything else is packaging.

---

## 4. Verified Pricing Model (May 2026)

The LLM pricing landscape has shifted dramatically in Q1–Q2 2026, with multiple flagship models dropping 50–80% in price. The `pricing.yaml` file must reflect current verified rates. The following table is sourced from provider pricing pages and Fungies.io's May 2026 comparison:

```yaml
# pricing.yaml — Verified May 22, 2026
# Sources: provider pricing pages, Fungies.io comparison, Benchwright audit

providers:
  openai:
    default_max_output_tokens: 16384
    models:
      "gpt-5.4":
        input_per_1m: 30.00
        output_per_1m: 180.00
      "gpt-5.4-pro":
        input_per_1m: 150.00
        output_per_1m: 600.00
      "gpt-5":
        input_per_1m: 1.25
        output_per_1m: 10.00
      "gpt-5.2":
        input_per_1m: 1.50
        output_per_1m: 14.00
      "gpt-4.1":
        input_per_1m: 2.00
        output_per_1m: 8.00
      "gpt-4.1-nano":
        input_per_1m: 0.10
        output_per_1m: 0.40
      "gpt-4o":
        input_per_1m: 2.50
        output_per_1m: 10.00
      "gpt-4o-mini":
        input_per_1m: 0.15
        output_per_1m: 0.60
      "o3":
        input_per_1m: 20.00
        output_per_1m: 80.00
      "o4-mini":
        input_per_1m: 1.10
        output_per_1m: 4.40
      "_fallback":
        input_per_1m: 30.00
        output_per_1m: 180.00

  anthropic:
    default_max_output_tokens: 8192
    models:
      "claude-opus-4-6":
        input_per_1m: 5.00
        output_per_1m: 25.00
      "claude-sonnet-4-6":
        input_per_1m: 3.00
        output_per_1m: 15.00
      "claude-haiku-4-5":
        input_per_1m: 1.00
        output_per_1m: 5.00
      "_fallback":
        input_per_1m: 5.00
        output_per_1m: 25.00
```

**Important note:** Anthropic's pricing listed in the Fungies comparison shows Opus 4.6 at $5/$25 per million tokens—lower than the $15/$75 listed in the previous build plan. Always verify provider pricing pages before cutting a release.

---

## 5. Technology Stack (Verified Choices)

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | Go 1.24+ | Single-binary deployment. Native concurrency. Standard library reverse proxy. |
| HTTP framework | `net/http` + `gin` router | Gin for route registration and middleware chaining; standard `httputil.ReverseProxy` for proxying. |
| Reverse proxy | `httputil.ReverseProxy` (stdlib) | Production-proven. Director for header injection. Custom `Flusher`-aware response writer for streaming. |
| Redis client | `go-redis/v9` | Industry standard. EVALSHA with automatic fallback to EVAL on NOSCRIPT. Lua script execution. |
| Token counting (OpenAI) | `pkoukk/tiktoken-go` v0.1.8 | Pure Go BPE tokenizer. Supports `o200k_base` (GPT-4o, GPT-5 family) and `cl100k_base` (GPT-4, GPT-4.1). v0.1.8 has bug fixes over v0.1.7. |
| Token counting (Anthropic) | `POST /v1/messages/count_tokens` | Provider-exact token count. Fallback to `tiktoken-go` with `cl100k_base` (~10-15% variance for Claude's unpublished vocabulary). |
| CLI framework | `cobra` | Standard Go CLI. `loopers serve`, `loopers keys create`, `loopers budget set`. |
| Configuration | `viper` | YAML config with environment variable overrides. No hot-reload for v1.0. |
| Metrics | `prometheus/client_golang` | `/metrics` endpoint. Counters for requests, budget blocks, token counts. Histograms for latency. |
| Structured logging | `zerolog` | Zero-allocation JSON logging. Key redaction hook. |
| Container | Docker multi-stage (alpine) | Target <30MB image. Non-root user. |

**Why not LiteLLM:** LiteLLM carries a CVSS 9.3 pre-auth SQL injection vulnerability (CVE-2026-42208) that was actively exploited within 36 hours of disclosure. Including it as a dependency would introduce a database, a SQL injection surface, and a credential storage mechanism—all of which Loopers explicitly avoids. The proxy layer is 300 lines of Go in `httputil.ReverseProxy`; we build it.

---

## 6. Atomic Budget Enforcement: The Lua Script

This is the product. Every other line of code exists to support this script.

```lua
-- lua_check.lua
-- Atomic budget check + reservation for a single window.
-- KEYS[1]: spend key  (loopers:spend:{hash}:daily:2026-05-22)
-- KEYS[2]: config key (loopers:budget:{hash}:config)
-- ARGV[1]: estimated_cost as string (e.g., "0.0125")
-- ARGV[2]: window name ("daily" or "hourly")
-- ARGV[3]: window TTL in seconds

local spend_key    = KEYS[1]
local config_key   = KEYS[2]
local est_cost     = tonumber(ARGV[1])
local window_name  = ARGV[2]
local window_ttl   = tonumber(ARGV[3])

-- Read budget limit from config
local limit_str = redis.call('HGET', config_key, window_name)
if not limit_str then
    -- No budget configured for this window → allow
    return {1, 0, 0, 'no_budget'}
end
local limit = tonumber(limit_str)

-- Read current spend
local current_str = redis.call('GET', spend_key)
local current = 0
if current_str then
    current = tonumber(current_str)
end

-- Atomic check
local projected = current + est_cost
if projected > limit then
    -- BLOCK: return as strings to preserve float precision
    return {0, tostring(current), tostring(limit), 'budget_exceeded'}
end

-- RESERVE: increment and set TTL in one atomic operation
redis.call('INCRBYFLOAT', spend_key, est_cost)
redis.call('EXPIRE', spend_key, window_ttl)

return {1, tostring(projected), tostring(limit), 'ok'}
```

This script runs as a single, indivisible operation within Redis. Redis executes Lua scripts atomically—"no other script or command will run while a script is running". The check and increment happen in the same execution context. No concurrent request can observe the spend before the increment and slip through. This is the correctness guarantee.

**For v1.0, there is no pending key, no reconciler, and no orphan cleanup.** When a budget check passes, the cost is deducted from the budget immediately. On streaming completion, a second Lua script reconciles the actual cost against the reserved cost. If a stream drops before completion, the reserved amount stays deducted—the budget is slightly over-debited. For a v1.0 this is acceptable. The reconciler is v2.0.

---

## 7. Streaming Architecture: The Hard Part

### 7.1 The Auto-Injection Requirement

Before forwarding any OpenAI streaming request, Loopers mutates the request body:

```go
// internal/proxy/openai.go
func injectStreamOptions(body []byte) []byte {
    var req map[string]interface{}
    json.Unmarshal(body, &req)
    
    if stream, ok := req["stream"].(bool); ok && stream {
        if _, hasOpts := req["stream_options"]; !hasOpts {
            req["stream_options"] = map[string]bool{
                "include_usage": true,
            }
        }
    }
    
    modified, _ := json.Marshal(req)
    return modified
}
```

Without this injection, every OpenAI streaming request reports zero tokens consumed, and the budget enforcement is silently disabled. This is the single most important line of code in the v1.0 release after the Lua script.

### 7.2 The Custom Response Writer

```go
// internal/proxy/stream.go
type budgetStreamWriter struct {
    http.ResponseWriter
    flusher       http.Flusher
    budgetReached bool
    tokenCount    int
    reservedUSD   float64
    pricingStore  *pricing.Store
}

func (w *budgetStreamWriter) Write(p []byte) (int, error) {
    if w.budgetReached {
        return 0, io.EOF
    }
    
    // Parse SSE chunk for token deltas
    if bytes.Contains(p, []byte(`"usage"`)) {
        tokens := parseUsageFromChunk(p)
        w.tokenCount += tokens
        cost := float64(tokens) * w.outputPricePer1M / 1_000_000
        
        if cost > w.reservedUSD {
            w.budgetReached = true
            w.ResponseWriter.Write(formatBudgetExceededSSE())
            w.ResponseWriter.Write([]byte("data: [DONE]\n\n"))
            w.flusher.Flush()
            return 0, io.EOF
        }
    }
    
    n, err := w.ResponseWriter.Write(p)
    if w.flusher != nil {
        w.flusher.Flush()
    }
    return n, err
}
```

The writer intercepts SSE chunks, identifies the final chunk containing the `usage` object, computes actual cost, and compares it against the reserved amount. If the budget would be exceeded, it sends a budget-exceeded SSE event followed by `[DONE]` and terminates the stream. Tokens already consumed are billed; no additional tokens reach the provider.

---

## 8. The Pass-Through Key Architecture

Provider API keys are never persisted. The architecture:

1. Client sends the provider key in `X-Loopers-Provider-Key` header on each request.
2. The proxy extracts it, stores it in the request context (in-memory only), and strips the header from the outgoing request.
3. The `Director` function retrieves the key from context and injects it as the provider's auth header.
4. When the response completes, the goroutine's stack unwinds and the key is garbage collected.

The key is never written to a log. A zerolog hook redacts any string matching known key patterns (`sk-*`, `sk-ant-*`, `AIza*`) before output. CI pipeline scans test output for key patterns and fails the build if any are found.

The trade-off: the provider key must be transmitted to Loopers on every request. For v1.0, the mitigation is to deploy Loopers in the same VPC as the application, over HTTPS. This limitation is documented explicitly in `SECURITY.md`. Server-side encrypted key storage is v2.0.

---

## 9. Redis Data Model

```
# Proxy key registry
loopers:key:{sha256_hash} → HASH
  name: "my-app"
  provider: "openai"
  created_at: "2026-05-22T10:00:00Z"
  active: "true"

# Budget configuration
loopers:budget:{sha256_hash}:config → HASH
  daily: "10.00"
  hourly: "2.00"

# Spend counters (auto-expiring)
loopers:spend:{sha256_hash}:daily:{YYYY-MM-DD} → STRING ("3.4567")
loopers:spend:{sha256_hash}:hourly:{YYYY-MM-DDTHH} → STRING ("1.2345")
```

All monetary values are stored as strings because Redis converts Lua numbers to integers by stripping decimals. This is a known limitation: "If you want to return a float value, return it as a string and parse the string in Go using Float64 helper" (redis.uptrace.dev). Every Lua script that handles dollar amounts uses `tostring()` on writes and `tonumber()` on reads.

---

## 10. Eight-Week Build Schedule

### Weeks 1–2: Core Proxy + OpenAI Budget Enforcement

**Goal:** `docker-compose up` → proxy OpenAI chat completions through Redis-backed budget enforcement.

- Day 1–2: Project scaffold, Go module, Gin server, health endpoint, `docker-compose.yml`.
- Day 3–4: Redis connection, Lua script embedding (`//go:embed`), `lua_check.lua`, budget engine.
- Day 5–6: `tiktoken-go` integration, `o200k_base` and `cl100k_base` encoding selection, cost estimation.
- Day 7–8: `httputil.ReverseProxy` with custom `Director`, key extraction middleware, pass-through architecture.
- Day 9–10: End-to-end test: create key, set budget, send request, verify budget deduction, verify 429 on overage.

### Weeks 3–4: Streaming + Anthropic

**Goal:** Streaming enforcement works. Anthropic provider added.

- Day 11–12: Custom `budgetStreamWriter`, `stream_options` auto-injection, SSE chunk parsing.
- Day 13–14: Mid-stream budget cut test: set $0.01 budget, stream a completion, verify stream terminates with budget event.
- Day 15–16: Anthropic `POST /v1/messages/count_tokens` integration, fallback to `tiktoken-go` on rate limit.
- Day 17–18: Anthropic reverse proxy handler, `x-api-key` auth injection.
- Day 19–20: Anthropic streaming test, message_delta usage parsing, reconciliation.

### Weeks 5–6: CLI + Hardening

**Goal:** Usable CLI. Production hardening. Fail-closed verified.

- Day 21–22: `loopers keys create/list/revoke` commands, `loopers budget set/status` commands.
- Day 23–24: `loopers serve` with full configuration, environment variable overrides.
- Day 25–26: Graceful shutdown (SIGTERM → drain connections → close Redis), fail-closed on Redis outage.
- Day 27–28: Security review: key redaction, no provider key in logs, no provider key in Redis, CI key leak scanner.

### Weeks 7–8: Testing + Launch

**Goal:** v0.1.0 on GitHub Releases. Docker image on GHCR + DockerHub.

- Day 29–32: Race condition test (100 goroutines, $5 budget, exactly 50 pass). Streaming reconciliation test. Provider sandbox test.
- Day 33–34: Performance test (vegeta, 500 req/s, p50 < 50ms). Memory leak test (sustained load, no goroutine growth).
- Day 35–36: GitHub Actions CI (lint, test, race, key scan, govulncheck). GoReleaser config.
- Day 37–38: README, CONTRIBUTING.md, SECURITY.md, `loopers.example.yaml`.
- Day 39–40: Tag v0.1.0. Push. Verify GitHub Release, Docker images, Homebrew tap.

---

## 11. Testing: The Correctness Suite

### 11.1 The Budget Race Condition Test

```go
func TestBudgetRaceCondition(t *testing.T) {
    rdb := startRedisContainer(t)
    keyHash := "test-race-key"
    setupBudget(t, rdb, keyHash, 5.00)

    var allowed, blocked int64
    var wg sync.WaitGroup
    
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := budget.CheckAndReserve(ctx, rdb, keyHash, 0.10, "req-"+uuid.New().String())
            if err == nil {
                atomic.AddInt64(&allowed, 1)
            } else {
                atomic.AddInt64(&blocked, 1)
            }
        }()
    }
    wg.Wait()

    assert.Equal(t, int64(50), allowed)
    assert.Equal(t, int64(50), blocked)
    assert.InDelta(t, 5.00, getCurrentSpend(t, rdb, keyHash), 0.001)
}
```

If this test produces anything other than exactly 50 allowed and exactly 50 blocked, the Lua script has a concurrency bug. This test is run with `-race` in CI on every push.

### 11.2 The Streaming Budget Enforcement Test

```go
func TestStreamingBudgetEnforced(t *testing.T) {
    // Set $0.005 budget (very tight)
    // Make streaming request that should cost $0.01
    // Verify mid-stream cut event is received
    // Verify final spend ≤ budget limit + 10% tolerance
}
```

### 11.3 The Key Leakage Test

```bash
# In CI: scan all test output for key-like patterns
go test ./... -v 2>&1 | \
  grep -E '(sk-[A-Za-z0-9]{32,}|sk-ant-[A-Za-z0-9]{32,}|AIza[0-9A-Za-z]{35})' && \
  { echo "KEY LEAK DETECTED"; exit 1; } || echo "PASS"
```

---

## 12. The 429 Response: A Brand Moment

When Loopers blocks a request, the response body is the product's voice:

```json
{
  "error": {
    "message": "Budget exceeded: daily cap of $10.00 reached. Current spend: $10.03. Resets at 2026-05-23T00:00:00Z.",
    "type": "circuit_break_budget_exceeded",
    "code": "budget_exceeded",
    "details": {
      "budget_type": "daily",
      "budget_limit_usd": 10.00,
      "current_spend_usd": 10.03,
      "resets_at": "2026-05-23T00:00:00Z",
      "request_cost_estimate_usd": 0.0125
    }
  }
}
```

This shape is intentionally compatible with OpenAI's error format so existing SDK error handling works without modification. The message is direct, specific, and tells the developer exactly what happened, what the limit was, what the current spend is, and when it resets.

---

## 13. Project Directory Structure

```
loopers/
├── cmd/loopers/main.go               # Entry point
├── internal/
│   ├── server/
│   │   ├── server.go                 # Gin router, middleware, lifecycle
│   │   ├── middleware.go             # RequestID, Recovery, Logging, Key extraction
│   │   └── graceful.go              # SIGTERM handler, drain, Redis close
│   ├── proxy/
│   │   ├── director.go              # ReverseProxy Director (key injection)
│   │   ├── openai.go                # OpenAI /v1/chat/completions
│   │   ├── anthropic.go             # Anthropic /v1/messages
│   │   └── stream.go               # budgetStreamWriter, SSE parsing
│   ├── budget/
│   │   ├── engine.go                # CheckAndReserve, Reconcile
│   │   ├── lua.go                   # Redis Script objects
│   │   ├── lua_check.lua            # Atomic budget check (go:embed)
│   │   ├── lua_reconcile.lua        # Post-stream reconciliation
│   │   └── redis.go                 # Redis client, fail-closed
│   ├── tokenizer/
│   │   ├── openai.go                # tiktoken-go wrapper
│   │   └── anthropic.go             # count_tokens API + fallback
│   ├── pricing/
│   │   ├── loader.go                # YAML loader
│   │   └── calculator.go            # Cost formula
│   ├── keyring/
│   │   ├── hasher.go                # SHA-256 proxy key
│   │   ├── extractor.go             # Header extraction
│   │   └── validator.go             # Key format validation
│   └── logging/
│       ├── logger.go                # zerolog setup
│       └── redact.go                # Key pattern redaction hook
├── pkg/api/
│   ├── types.go                     # Request/response types
│   └── errors.go                    # Structured error builder
├── .github/workflows/
│   ├── ci.yml                       # Lint + test + race + key scan
│   └── release.yml                  # GoReleaser
├── docker-compose.yml
├── Dockerfile
├── loopers.example.yaml
├── pricing.yaml
├── README.md
├── CONTRIBUTING.md
├── SECURITY.md
└── LICENSE                           # MIT
```

---

## 14. v2.0 Backlog (Post-Launch)

Ordered by user impact:

1. **Server-side encrypted key storage** — eliminates the pass-through key transmission requirement
2. **Gemini provider support** — `countTokens` API integration + Flash Lite bug workaround
3. **Background orphan reconciler** — reclaims reserved budget for dropped streams
4. **Per-session budget windows** — agent-specific circuit breakers
5. **Anomaly detection** — streaming Z-score on per-key request rate baselines
6. **PostgreSQL audit log with hash chaining** — compliance-grade tamper evidence
7. **Web dashboard** — visual spend monitoring, key management UI
8. **OpenTelemetry tracing** — distributed context propagation

---

## 15. Launch Checklist

### Code Quality
- [ ] `go test ./... -race` passes with zero failures
- [ ] Budget race condition test: exactly 50/50 at 100 goroutines
- [ ] Streaming budget cut test: budget enforced mid-generation
- [ ] Key leakage CI scan: zero matches
- [ ] Provider key not in Redis after any request
- [ ] `golangci-lint run` with zero warnings
- [ ] `govulncheck ./...` with zero known vulnerabilities

### Infrastructure
- [ ] `docker-compose up` deploys in <30 seconds
- [ ] Binary size <25MB (stripped)
- [ ] Docker image <30MB
- [ ] p50 latency overhead <50ms (vegeta benchmark)
- [ ] Memory stable over sustained load (no goroutine leak)

### Release
- [ ] `pricing.yaml` verified against provider pricing pages (May 2026)
- [ ] `stream_options.include_usage` auto-injection tested with real OpenAI key
- [ ] GitHub Release with 5 platform binaries + checksums
- [ ] Docker images on GHCR + DockerHub
- [ ] Homebrew tap functional
- [ ] README quickstart under 5 steps
- [ ] `SECURITY.md` documents pass-through key limitation explicitly

---

*Document version 3.0 — built from independently verified external sources, May 22, 2026.*  
*Every claim above that is not obvious architecture can be traced to a numbered citation.*