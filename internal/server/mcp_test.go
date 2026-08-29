package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/try-loopers/loopers-oss/internal/budget"
	"github.com/try-loopers/loopers-oss/internal/keyring"
	"github.com/try-loopers/loopers-oss/internal/pricing"
	"github.com/alicebob/miniredis/v2"
	"github.com/spf13/viper"
)

func TestMCP_Integration(t *testing.T) {
	viper.Set("testing.allow_private_urls", true)
	t.Cleanup(func() { viper.Set("testing.allow_private_urls", false) })

	// 1. Setup Mock Upstream MCP Server
	mcpServerCallCount := 0
	mcpUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mcpServerCallCount++

		// Read body
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Respond with a mock successful JSON-RPC response
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Success from mock filesystem server",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mcpUpstream.Close()

	// 2. Setup Loopers Server with miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient, err := budget.NewClient(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("Failed to create redis client: %v", err)
	}
	defer redisClient.Close()

	// Create temp pricing.yaml
	tmpFile, _ := os.CreateTemp("", "pricing*.yaml")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte(`
tool_costs:
  defaults:
    unknown_tool: 0.01
  tools:
    "snowflake_query":
      cost_per_call: 0.05
`))
	tmpFile.Close()

	pricingStore, _ := pricing.LoadStore(tmpFile.Name())

	// Configure Viper for MCP
	viper.Set("mcp", map[string]interface{}{
		"enabled": true,
		"servers": []map[string]interface{}{
			{
				"name": "filesystem",
				"url":  mcpUpstream.URL,
			},
		},
		"circuit_breaker": map[string]interface{}{
			"enabled":        true,
			"threshold":      2,
			"window_seconds": 60,
		},
	})
	defer viper.Reset()

	s := NewServer(redisClient, pricingStore)
	defer s.Shutdown()
	r := s.GetRouter()

	// 3. Register Key and Budget
	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)
	rdb := redisClient.GetUnderlyingClient()
	ctx := context.Background()

	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":       "mcp-test-key",
		"provider":   "openai",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"active":     "true",
	})
	defer rdb.Del(ctx, "loopers:key:"+keyHash)

	// Set a mock budget config (daily limit of $0.08)
	configKey := "loopers:budget:{" + keyHash + "}:config"
	rdb.HSet(ctx, configKey, "daily", "0.08")
	defer rdb.Del(ctx, configKey)

	// 4. Test Case 1: Valid Forwarding & Budget deduction
	reqBody := []byte(`{
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

	req, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.Header.Set("X-Loopers-Session-ID", "123e4567-e89b-12d3-a456-426614174000")
	req.Header.Set("X-Loopers-Provider-Key", "dummy")

	w := newCloseNotifierRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var jsonRPCResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &jsonRPCResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify cost header was added (0.05 is cost for snowflake_query)
	costHeader := w.Header().Get("X-Loopers-Tool-Cost")
	if costHeader != "0.050000" {
		t.Errorf("Expected cost header 0.050000, got %s", costHeader)
	}

	// 5. Test Case 2: Circuit Breaker
	// Call same tool with same args again (since threshold is 2, this 2nd call should trip the circuit breaker)
	req2, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBody))
	req2.Header.Set("Authorization", "Bearer "+rawKey)
	req2.Header.Set("X-Loopers-Session-ID", "123e4567-e89b-12d3-a456-426614174001")
	req2.Header.Set("X-Loopers-Provider-Key", "dummy")

	w2 := newCloseNotifierRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429 for circuit breaker, got %d. Body: %s", w2.Code, w2.Body.String())
	}

	// 6. Test Case 3: Budget Exceeded
	// The budget is $0.08. First call deducted $0.05. Remaining is $0.03.
	// A new session calls snowflake_query ($0.05). Since remaining budget ($0.03) < cost ($0.05), it should be blocked.
	req3Body := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "tools/call",
		"params": {
			"name": "snowflake_query",
			"arguments": {
				"query": "SELECT 2"
			}
		}
	}`)
	req3, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(req3Body))
	req3.Header.Set("Authorization", "Bearer "+rawKey)
	req3.Header.Set("X-Loopers-Session-ID", "123e4567-e89b-12d3-a456-426614174002")
	req3.Header.Set("X-Loopers-Provider-Key", "dummy")

	w3 := newCloseNotifierRecorder()
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429 (budget exceeded), got %d. Body: %s", w3.Code, w3.Body.String())
		// 7. Test Case 4: Non-tools/call method (e.g. tools/list)
		// Should pass through without deducting budget or checking circuit breaker.
		req4Body := []byte(`{
		"jsonrpc": "2.0",
		"id": 3,
		"method": "tools/list"
	}`)
		req4, _ := http.NewRequest("POST", "/mcp/filesystem/tools/list", bytes.NewReader(req4Body))
		req4.Header.Set("Authorization", "Bearer "+rawKey)

		w4 := newCloseNotifierRecorder()
		r.ServeHTTP(w4, req4)

		if w4.Code != http.StatusOK {
			t.Errorf("Expected status 200 for tools/list, got %d", w4.Code)
		}
		if w4.Header().Get("X-Loopers-Tool-Cost") != "" {
			t.Errorf("Expected no cost header for tools/list, got %s", w4.Header().Get("X-Loopers-Tool-Cost"))
		}

		// 8. Test Case 5: Malformed JSON-RPC (Upstream returns 400)
		// Should pass through to upstream. Upstream mock returns 400 because body is invalid JSON.
		// Since it's not a valid tools/call, it passes through transparently and no cost is charged.
		req5Body := []byte(`{ invalid_json }`)
		req5, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(req5Body))
		req5.Header.Set("Authorization", "Bearer "+rawKey)

		w5 := newCloseNotifierRecorder()
		r.ServeHTTP(w5, req5)

		if w5.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for malformed json, got %d", w5.Code)
		}

		// 9. Test Case 6: Upstream Failure Refunds Cost
		// Send a valid tools/call but to a mock endpoint that returns 500.
		// Actually, we can just send a valid tools/call but malformed json.
		// Wait, if it's a valid tools/call parsed by Loopers, but upstream fails...
		// We can't do that easily unless we change the mock server.
		// Let's create a new mock server for 500 just for this test!
		errorUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer errorUpstream.Close()

		// Add it to viper config
		viper.Set("mcp.servers", []map[string]interface{}{
			{
				"name": "filesystem",
				"url":  mcpUpstream.URL,
			},
			{
				"name": "error_server",
				"url":  errorUpstream.URL,
			},
		})

		// We need to re-init server to pick up new config
		s2 := NewServer(redisClient, pricingStore)
		defer s2.Shutdown()
		r2 := s2.GetRouter()

		// Remaining budget is $0.03.
		// Let's call an unknown tool (cost 0.01) on the error server.
		req6Body := []byte(`{
		"jsonrpc": "2.0",
		"id": 4,
		"method": "tools/call",
		"params": {
			"name": "unknown_tool",
			"arguments": {}
		}
	}`)
		req6, _ := http.NewRequest("POST", "/mcp/error_server/tools/call", bytes.NewReader(req6Body))
		req6.Header.Set("Authorization", "Bearer "+rawKey)
		req6.Header.Set("X-Loopers-Session-ID", "123e4567-e89b-12d3-a456-426614174003")

		w6 := newCloseNotifierRecorder()
		r2.ServeHTTP(w6, req6)

		if w6.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 from upstream, got %d", w6.Code)
		}
		if w6.Header().Get("X-Loopers-Tool-Cost") != "" {
			t.Errorf("Expected no cost header because upstream failed, got %s", w6.Header().Get("X-Loopers-Tool-Cost"))
		}

		// Because it failed, the $0.01 should have been refunded.
		// Verify budget is still $0.03 by making another call that costs $0.01 to a working server
		req7, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(req6Body))
		req7.Header.Set("Authorization", "Bearer "+rawKey)
		req7.Header.Set("X-Loopers-Session-ID", "123e4567-e89b-12d3-a456-426614174004") // new session to avoid circuit breaker

		w7 := newCloseNotifierRecorder()
		r2.ServeHTTP(w7, req7)

		if w7.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. This means the budget wasn't refunded properly!", w7.Code)
		}
		if w7.Header().Get("X-Loopers-Tool-Cost") != "0.010000" {
			t.Errorf("Expected cost header 0.010000, got %s", w7.Header().Get("X-Loopers-Tool-Cost"))
		}
	}
}
