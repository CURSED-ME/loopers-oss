# Post-Mortem: Loopers OOM Crash (Exit Code 137)

## Executive Summary
During the Episode 1 (`ep01-litellm`) budget leak benchmarks, the `loopers` container consistently crashed with **Exit Code 137 (OOM Killed)** within a fraction of a second when subjected to 1,000 concurrent requests via `k6`. 

Initial theories pointed to an aggressive memory allocation issue (e.g., buffering 10MB per request payload due to `MaxBytesReader`), but a deeper dive into the source code revealed a **textbook connection and goroutine leak** in Go's Reverse Proxy implementation.

---

## 🔍 Root Cause Analysis

The crash originates in `c:\Users\varad\loopers-oss\internal\server\server.go`, specifically within the `modifyResponse` callback used by the `httputil.ReverseProxy`. 

When Loopers intercepts a standard, non-streaming response to calculate token usage and costs, it reads the entire upstream response body into memory and replaces it with an `io.NopCloser`:

```go
// Location: loopers-oss/internal/server/server.go

} else {
    respBodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }
    
    // ❌ FATAL FLAW: Missing resp.Body.Close() here
    
    resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
```

### The Cascading Failure
According to Go's `httputil.ReverseProxy` official documentation: 
> *"If [ModifyResponse] modifies the Body, it is responsible for closing the old Body."*

Because Loopers completely drops the pointer to the original `resp.Body` without calling `.Close()`, the underlying `net/http.Transport` never realizes the HTTP request has finished. This causes a massive cascading failure:

1. **Connection Exhaustion:** The proxy never returns the TCP connections to the idle pool. It is forced to open a brand new TCP connection for every single incoming request.
2. **Goroutine Leaks:** Go's `net/http` spins up two background goroutines (`readLoop` and `writeLoop`) for every active TCP connection. Because the connections are left dangling in an open state, these goroutines are orphaned and live forever.
3. **Memory Exhaustion:** Each orphaned connection holds onto internal 4KB read/write buffers, internal context trees, and stack memory.

When hit with 1,000 concurrent virtual users, Loopers instantly orphans thousands of TCP connections. The Go runtime responds by spawning tens of thousands of trapped `readLoop`/`writeLoop` goroutines, instantly exhausting the strict 2GB memory limit set by the Docker container and triggering a SIGKILL (Exit Code 137) from the Linux OOM Killer.

> [!NOTE] 
> The streaming path in Loopers (`proxy/stream.go`) actually handles this correctly by using a `defer original.Close()`. This memory leak is isolated entirely to the standard, non-streaming execution path.

---

## 🛠️ Steps to Recreate the Failure

To reliably reproduce this memory leak locally using the current benchmark harness, follow these exact steps:

### 1. Start the Environment
Navigate to the `ep01-litellm` benchmark directory and spin up the Docker Compose stack with a 2GB memory limit:
```bash
cd c:\Users\varad\llm-gateway-benchmark\ep01-litellm
docker compose up -d
```

### 2. Seed the Loopers Budget
Provision the Loopers proxy with a valid budget in Redis:
```bash
# Set Loopers API Key
docker exec -it ep01-litellm-redis-1 redis-cli HSET "loopers:key:b168b4e75b33ee699d775588c229f7206c3e6890bbc8514310108aec4af35082" name "loopers-test" provider "openai" active "true"

# Set Loopers $0.01 Budget Limit
docker exec -it ep01-litellm-redis-1 redis-cli HSET "loopers:budget:b168b4e75b33ee699d775588c229f7206c3e6890bbc8514310108aec4af35082:config" minute "0.01" hourly "0.01" daily "0.01" weekly "0.01" monthly "0.01"
```

### 3. Run the Load Test
Execute the `k6` budget leak benchmark using a Dockerized k6 runner targeted directly at the Loopers proxy. This script fires 1,000 concurrent virtual users (non-streaming requests):
```bash
docker run --rm -i --network ep01-litellm_default -v c:\Users\varad\llm-gateway-benchmark:/benchmark grafana/k6 run --env PROXY=loopers /benchmark/shared/harness/budget_leak_test.js
```

### 4. Observe the Crash
While the load test is running, in a separate terminal, monitor the Docker container status:
```bash
docker compose ps
```
Within a few seconds, the `ep01-litellm-loopers-1` container state will change to **`Exited (137)`**, and the `k6` benchmark will immediately start failing with `EOF` or `connection refused` errors as the gateway goes offline.
