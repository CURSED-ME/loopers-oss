package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/budget"
	"github.com/CURSED-ME/loopers-oss/internal/keyring"
	"github.com/CURSED-ME/loopers-oss/internal/pricing"
	"github.com/alicebob/miniredis/v2"
	"github.com/spf13/viper"
)

func setupStressServer(t *testing.T, upstreamHandler http.HandlerFunc) (*miniredis.Miniredis, *budget.Client, *Server, string) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	redisClient, err := budget.NewClient(mr.Addr(), "", 0)
	if err != nil {
		mr.Close()
		t.Fatalf("failed to create redis client: %v", err)
	}

	// Create temp pricing.yaml
	tmpFile, _ := os.CreateTemp("", "pricing*.yaml")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte(`
tool_costs:
  defaults:
    unknown_tool: 0.001
  tools:
    "snowflake_query":
      cost_per_call: 0.01
`))
	tmpFile.Close()

	pricingStore, err := pricing.LoadStore(tmpFile.Name())
	if err != nil {
		redisClient.Close()
		mr.Close()
		t.Fatalf("failed to load pricing store: %v", err)
	}

	// Setup mock upstream MCP server
	upstream := httptest.NewServer(upstreamHandler)
	t.Cleanup(upstream.Close)

	// Configure Viper for MCP
	viper.Set("mcp", map[string]interface{}{
		"enabled": true,
		"servers": []map[string]interface{}{
			{
				"name": "stress_server",
				"url":  upstream.URL,
			},
		},
		"circuit_breaker": map[string]interface{}{
			"enabled":        true,
			"threshold":      2,
			"window_seconds": 60,
		},
	})

	s := NewServer(redisClient, pricingStore)

	// Register key
	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)
	rdb := redisClient.GetUnderlyingClient()
	ctx := context.Background()

	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":       "stress-key",
		"provider":   "openai",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"active":     "true",
	})

	// Set daily budget limit of $10.00
	configKey := "loopers:budget:" + keyHash + ":config"
	rdb.HSet(ctx, configKey, "daily", "10.00")

	t.Cleanup(func() {
		rdb.Del(ctx, "loopers:key:"+keyHash)
		rdb.Del(ctx, configKey)
		redisClient.Close()
		mr.Close()
		viper.Reset()
	})

	return mr, redisClient, s, rawKey
}

// 1. TestMCP_Stress_HighConcurrency
// Hitting the MCP proxy under high concurrency with multiple goroutines.
// Half spam the same tool call with same arguments under a single session (trips circuit breaker).
// Half make clean calls (verify budget accuracy and no cross-talk).
func TestMCP_Stress_HighConcurrency(t *testing.T) {
	// Count upstream hits
	var upstreamHits int64
	upstreamHandler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&upstreamHits, 1)

		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "Success from stress upstream"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	_, _, s, rawKey := setupStressServer(t, upstreamHandler)
	router := s.GetRouter()

	const numWorkers = 50
	const callsPerWorker = 10
	var wg sync.WaitGroup

	var totalSuccess int64
	var totalTripped int64
	var totalOtherErrors int64

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Half of the workers will spam identical requests to trip the circuit breaker.
			// Half of the workers will send random/clean arguments.
			isSpammer := workerID%2 == 0

			for j := 0; j < callsPerWorker; j++ {
				var body []byte
				var sessionID string

				if isSpammer {
					sessionID = "spammer-session"
					body = []byte(`{
						"jsonrpc": "2.0",
						"id": 1,
						"method": "tools/call",
						"params": {
							"name": "snowflake_query",
							"arguments": {
								"query": "SELECT 1"
							}
						}
					}`)
				} else {
					sessionID = fmt.Sprintf("clean-session-%d", workerID)
					body = []byte(fmt.Sprintf(`{
						"jsonrpc": "2.0",
						"id": 1,
						"method": "tools/call",
						"params": {
							"name": "snowflake_query",
							"arguments": {
								"query": "SELECT RANDOM %d-%d"
							}
						}
					}`, workerID, j))
				}

				req, _ := http.NewRequest("POST", "/mcp/stress_server/tools/call", bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+rawKey)
				req.Header.Set("X-Loopers-Session-ID", sessionID)
				req.Header.Set("X-Loopers-Provider-Key", "dummy")

				w := newCloseNotifierRecorder()
				router.ServeHTTP(w, req)

				switch w.Code {
				case http.StatusOK:
					atomic.AddInt64(&totalSuccess, 1)
				case http.StatusTooManyRequests:
					// Verify it's either circuit breaker or budget (it should be circuit breaker)
					var errResp map[string]interface{}
					_ = json.Unmarshal(w.Body.Bytes(), &errResp)
					if errResp["type"] == "mcp_circuit_breaker" {
						atomic.AddInt64(&totalTripped, 1)
					} else {
						t.Errorf("Unexpected 429 type: %v", errResp)
					}
				default:
					atomic.AddInt64(&totalOtherErrors, 1)
					t.Errorf("Unexpected status code: %d, body: %s", w.Code, w.Body.String())
				}
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Stress Concurrency Finished: Success=%d, CB Tripped=%d, Other Errors=%d, Upstream Hits=%d",
		totalSuccess, totalTripped, totalOtherErrors, upstreamHits)

	// Spammers: 25 workers * 10 calls = 250 calls.
	// Only the first 4 calls for the spammer session should succeed, rest (246) should trip.
	// Clean workers: 25 workers * 10 calls = 250 calls. All should succeed.
	// So expected success = ~254. Expected tripped = ~246.
	// Due to concurrency, there might be tiny differences in the exact moment the 5th call executes,
	// but totalSuccess must be roughly 254.
	if totalSuccess < 250 || totalSuccess > 260 {
		t.Errorf("Expected totalSuccess to be around 254, got %d", totalSuccess)
	}

	if totalTripped < 240 || totalTripped > 250 {
		t.Errorf("Expected totalTripped to be around 246, got %d", totalTripped)
	}

	if totalOtherErrors > 0 {
		t.Errorf("Expected 0 other errors, got %d", totalOtherErrors)
	}

	if upstreamHits != totalSuccess {
		t.Errorf("Expected upstream hits (%d) to match total success (%d)", upstreamHits, totalSuccess)
	}
}

// 2. TestMCP_Stress_MemoryPressure
// Alternating between sending very large payloads (5MB) and small payloads (1KB)
// to verify memory pooling behavior under concurrency and ensure no data contamination.
func TestMCP_Stress_MemoryPressure(t *testing.T) {
	upstreamHandler := func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "Success"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	_, _, s, rawKey := setupStressServer(t, upstreamHandler)
	router := s.GetRouter()

	// Generate a 3MB string for large payloads
	largeData := make([]byte, 3*1024*1024)
	for i := range largeData {
		largeData[i] = 'a'
	}

	const numRequests = 100
	var wg sync.WaitGroup

	// Record memory stats before stress
	var msStart runtime.MemStats
	runtime.ReadMemStats(&msStart)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			var body []byte
			var query string
			isLarge := idx%2 == 0

			if isLarge {
				query = string(largeData)
			} else {
				query = fmt.Sprintf("small-query-%d", idx)
			}

			reqMap := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      idx,
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name": "snowflake_query",
					"arguments": map[string]interface{}{
						"query": query,
					},
				},
			}
			body, _ = json.Marshal(reqMap)

			req, _ := http.NewRequest("POST", "/mcp/stress_server/tools/call", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+rawKey)
			req.Header.Set("X-Loopers-Session-ID", fmt.Sprintf("sess-mem-%d", idx))

			w := newCloseNotifierRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Request %d failed with code %d: %s", idx, w.Code, w.Body.String())
				return
			}

			// Parse response to ensure we got correct ID back (ensures no request/response cross-talk)
			var resp map[string]interface{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			idFloat, ok := resp["id"].(float64)
			if !ok || int(idFloat) != idx {
				t.Errorf("Mismatch/Corruption: Expected ID %d, got %v", idx, resp["id"])
			}
		}(i)
	}

	wg.Wait()

	// Force garbage collection
	runtime.GC()
	var msEnd runtime.MemStats
	runtime.ReadMemStats(&msEnd)

	t.Logf("Memory Stress Stats: HeapAlloc Before=%d KB, HeapAlloc After=%d KB",
		msStart.HeapAlloc/1024, msEnd.HeapAlloc/1024)
}

// 3. TestMCP_Stress_UpstreamLatencyCascade
// Mock an upstream MCP server that is blocked.
// Set maximum inflight requests configuration to 2.
// Verify that excess requests get dropped with 503 Service Unavailable.
func TestMCP_Stress_UpstreamLatencyCascade(t *testing.T) {
	blockChan := make(chan struct{})
	defer func() {
		select {
		case <-blockChan:
		default:
			close(blockChan)
		}
	}()

	upstreamHandler := func(w http.ResponseWriter, r *http.Request) {
		<-blockChan
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "Success"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	// Override max_inflight configuration to 2
	viper.Set("server.max_inflight", 2)
	defer viper.Reset()

	_, _, s, rawKey := setupStressServer(t, upstreamHandler)
	router := s.GetRouter()

	const numRequests = 3
	var wg sync.WaitGroup

	var successes int64
	var limiters int64

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			body := []byte(fmt.Sprintf(`{
				"jsonrpc": "2.0",
				"id": %d,
				"method": "tools/call",
				"params": {
					"name": "snowflake_query",
					"arguments": {
						"query": "SELECT %d"
					}
				}
			}`, idx, idx))

			req, _ := http.NewRequest("POST", "/mcp/stress_server/tools/call", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+rawKey)
			req.Header.Set("X-Loopers-Session-ID", fmt.Sprintf("sess-latency-%d", idx))

			w := newCloseNotifierRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt64(&successes, 1)
			} else if w.Code == http.StatusServiceUnavailable {
				atomic.AddInt64(&limiters, 1)
				// Release the blocked ones once we get a 503
				select {
				case <-blockChan:
				default:
					close(blockChan)
				}
			} else {
				t.Errorf("Unexpected status code: %d, body: %s", w.Code, w.Body.String())
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Latency Cascade Results: Successes=%d, Rate Limited (503)=%d", successes, limiters)

	if limiters != 1 {
		t.Errorf("Expected exactly 1 request to trigger ConcurrencyLimiter (503), got %d", limiters)
	}

	if successes != 2 {
		t.Errorf("Expected exactly 2 requests to succeed, got %d", successes)
	}
}

// 4. TestMCP_Stress_RedisOutageRecovery
// Verify fail-closed behavior when Redis goes offline and ensure the proxy recovers gracefully when Redis comes back.
func TestMCP_Stress_RedisOutageRecovery(t *testing.T) {
	upstreamHandler := func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "Success"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	mr, redisClient, s, rawKey1 := setupStressServer(t, upstreamHandler)
	router := s.GetRouter()

	// Register a second key BEFORE turning Redis offline, so it's registered but uncached.
	rawKey2, _ := keyring.GenerateRawKey()
	keyHash2 := keyring.HashKey(rawKey2)
	rdb := redisClient.GetUnderlyingClient()
	ctx := context.Background()

	rdb.HSet(ctx, "loopers:key:"+keyHash2, map[string]interface{}{
		"name":       "stress-key-2",
		"provider":   "openai",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"active":     "true",
	})
	configKey2 := "loopers:budget:" + keyHash2 + ":config"
	rdb.HSet(ctx, configKey2, "daily", "10.00")

	// 1. Initial Request for rawKey1 (Should succeed, caching key1 and creating local lease)
	body := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "snowflake_query",
			"arguments": {
				"query": "SELECT 1"
			}
		}
	}`)

	req1, _ := http.NewRequest("POST", "/mcp/stress_server/tools/call", bytes.NewReader(body))
	req1.Header.Set("Authorization", "Bearer "+rawKey1)
	req1.Header.Set("X-Loopers-Session-ID", "session-recovery-1")

	w1 := newCloseNotifierRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w1.Code, w1.Body.String())
	}

	// 2. Simulate Redis offline by forcing errors
	mr.SetError("Redis is offline (simulated)")

	// 2a. Request using rawKey2 (Uncached). This should fail because it has to fetch metadata from Redis.
	req2, _ := http.NewRequest("POST", "/mcp/stress_server/tools/call", bytes.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+rawKey2)
	req2.Header.Set("X-Loopers-Session-ID", "session-recovery-2")

	w2 := newCloseNotifierRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized && w2.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected uncached key request to fail-closed (401 or 503) when Redis is down, got %d. Body: %s", w2.Code, w2.Body.String())
	}

	// 2b. Request using rawKey1 (Cached, but we exhaust the remaining local lease).
	// Since local lease has $0.99, we make 100 requests of cost $0.01. The 100th must hit Redis and fail.
	var wCached *closeNotifierRecorder
	for i := 0; i < 100; i++ {
		reqCached, _ := http.NewRequest("POST", "/mcp/stress_server/tools/call", bytes.NewReader(body))
		reqCached.Header.Set("Authorization", "Bearer "+rawKey1)
		reqCached.Header.Set("X-Loopers-Session-ID", fmt.Sprintf("session-recovery-cached-%d", i))

		wCached = newCloseNotifierRecorder()
		router.ServeHTTP(wCached, reqCached)
		if wCached.Code != http.StatusOK {
			break
		}
	}

	if wCached.Code != http.StatusServiceUnavailable && wCached.Code != http.StatusUnauthorized {
		t.Errorf("Expected cached key to fail-closed with 503 or 401 when local lease is exhausted and Redis is down, got %d. Body: %s", wCached.Code, wCached.Body.String())
	}

	// 3. Restore Redis by clearing the simulated error
	mr.SetError("")

	// 4. Verify auto-recovery (Should succeed again)
	var w3 *closeNotifierRecorder
	for attempt := 0; attempt < 5; attempt++ {
		req3, _ := http.NewRequest("POST", "/mcp/stress_server/tools/call", bytes.NewReader(body))
		req3.Header.Set("Authorization", "Bearer "+rawKey1)
		req3.Header.Set("X-Loopers-Session-ID", fmt.Sprintf("session-recovery-3-%d", attempt))

		w3 = newCloseNotifierRecorder()
		router.ServeHTTP(w3, req3)

		if w3.Code == http.StatusOK {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if w3.Code != http.StatusOK {
		t.Errorf("Expected request to succeed after Redis recovery, got %d. Body: %s", w3.Code, w3.Body.String())
	}
}
