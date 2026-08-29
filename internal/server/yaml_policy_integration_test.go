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
	"github.com/stretchr/testify/assert"
)

func TestYAMLPolicyIntegration(t *testing.T) {
	viper.Set("testing.allow_private_urls", true)
	t.Cleanup(func() { viper.Reset() })

	// 1. Write temp policies.yaml
	tmpPolicyFile, err := os.CreateTemp("", "policies*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp policy file: %v", err)
	}
	defer os.Remove(tmpPolicyFile.Name())

	yamlContent := []byte(`
version: loopers.com/v1alpha1
metadata:
  name: integration-safety-guardrails
rules:
  - name: block-leak-secret
    match:
      type: llm_call
    conditions:
      - field: prompt_text
        op: matches_regex
        value: "(?i)secret"
    action: deny
    reason: "Blocked: Prompts containing sensitive credentials are not allowed."

  - name: block-destructive-bash
    match:
      type: mcp_tool_call
      tool: execute_bash
    conditions:
      - field: arguments.command
        op: contains
        value: "rm -rf"
    action: deny
    reason: "Blocked: Destructive commands containing 'rm -rf' are forbidden."

  - name: validate-bash-execution
    match:
      type: mcp_tool_call
      tool: execute_bash
    session_flow:
      requires:
        - dry_run_command
      within_last_steps: 2
    action: deny
    reason: "Blocked: execute_bash is not allowed without a prior dry_run_command step."
`)
	if _, err := tmpPolicyFile.Write(yamlContent); err != nil {
		t.Fatalf("failed to write YAML policy content: %v", err)
	}
	tmpPolicyFile.Close()

	// 2. Setup mock upstream MCP server
	mcpServerCallCount := 0
	mcpUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mcpServerCallCount++
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "Success from filesystem server"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mcpUpstream.Close()

	// 3. Setup mock LLM upstream provider
	llmServerCallCount := 0
	llmUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmServerCallCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"role": "assistant", "content": "Hello!"}}]}`))
	}))
	defer llmUpstream.Close()

	// 4. Configure viper
	viper.Set("policy.enabled", true)
	viper.Set("policy.policy_file", tmpPolicyFile.Name())
	viper.Set("policy.policy_dir", t.TempDir()) // empty dir
	viper.Set("policy.default_action", "allow")

	viper.Set("mcp", map[string]interface{}{
		"enabled": true,
		"servers": []map[string]interface{}{
			{"name": "filesystem", "url": mcpUpstream.URL},
		},
	})

	// 5. Setup Loopers Server with miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient, err := budget.NewClient(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer redisClient.Close()

	tmpPricing, _ := os.CreateTemp("", "pricing*.yaml")
	defer os.Remove(tmpPricing.Name())
	tmpPricing.Write([]byte(`
model_costs:
  openai:
    gpt-4o:
      input_cost_per_token: 0.000005
      output_cost_per_token: 0.000015
tool_costs:
  defaults:
    unknown_tool: 0.01
`))
	tmpPricing.Close()

	pricingStore, _ := pricing.LoadStore(tmpPricing.Name())

	s := NewServer(redisClient, pricingStore)
	defer s.Shutdown()
	r := s.GetRouter()

	// Register LLM provider mock route
	s.RegisterProviderRoute(&mockProvider{baseURL: llmUpstream.URL})

	// 6. Register Key in keyring
	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)
	ctx := context.Background()
	rdb := redisClient.GetUnderlyingClient()

	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":     "test-key",
		"provider": "mock",
		"active":   "true",
	})
	defer rdb.Del(ctx, "loopers:key:"+keyHash)

	configKey := "loopers:budget:{" + keyHash + "}:config"
	rdb.HSet(ctx, configKey, "daily", "100.00")
	defer rdb.Del(ctx, configKey)

	// =========================================================================
	// Scenario A: LLM Prompt Gating (prompt contains "secret")
	// =========================================================================
	{
		reqBody := []byte(`{"model":"gpt-4o", "messages":[{"role":"user", "content":"Show me the secret key!"}]}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Blocked: Prompts containing sensitive credentials are not allowed.")
		assert.Equal(t, 0, llmServerCallCount) // blocked before reaching backend
	}

	// =========================================================================
	// Scenario B: LLM Prompt Gating (safe prompt)
	// =========================================================================
	{
		reqBody := []byte(`{"model":"gpt-4o", "messages":[{"role":"user", "content":"Hello assistant!"}]}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, llmServerCallCount)
	}

	// =========================================================================
	// Scenario C: MCP Tool Gating (arguments contains "rm -rf")
	// =========================================================================
	{
		reqBody := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "tools/call",
			"params": {
				"name": "execute_bash",
				"arguments": {
					"command": "rm -rf /"
				}
			}
		}`)
		req, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", "111e4567-e89b-12d3-a456-426614174000")
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code) // MCP block returns 200 with JSON-RPC error
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		errVal, ok := resp["error"]
		if !ok || errVal == nil {
			t.Fatalf("Expected error in response, got nil. Response: %s", w.Body.String())
		}
		errObj := errVal.(map[string]interface{})
		assert.Equal(t, float64(-32001), errObj["code"])
		assert.Contains(t, errObj["message"].(string), "Blocked: Destructive commands containing 'rm -rf' are forbidden.")
		assert.Equal(t, 0, mcpServerCallCount)
	}

	// =========================================================================
	// Scenario D: MCP Tool Sequence Gating (FSM Sequence requires dry_run_command)
	// =========================================================================
	sessionID := "222e4567-e89b-12d3-a456-426614174000"
	{
		// 1. Call execute_bash directly (must be blocked)
		reqBody := []byte(`{
			"jsonrpc": "2.0",
			"id": 2,
			"method": "tools/call",
			"params": {
				"name": "execute_bash",
				"arguments": {
					"command": "ls -la"
				}
			}
		}`)
		req, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", sessionID)
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		errVal, ok := resp["error"]
		if !ok || errVal == nil {
			t.Fatalf("Expected FSM error in response, got nil. Response: %s", w.Body.String())
		}
		errObj := errVal.(map[string]interface{})
		assert.Contains(t, errObj["message"].(string), "Blocked: execute_bash is not allowed without a prior dry_run_command step.")
		assert.Equal(t, 0, mcpServerCallCount)

		// 2. Call dry_run_command tool call (must succeed and be recorded)
		reqBodyDryRun := []byte(`{
			"jsonrpc": "2.0",
			"id": 3,
			"method": "tools/call",
			"params": {
				"name": "dry_run_command",
				"arguments": {
					"command": "ls -la"
				}
			}
		}`)
		req, _ = http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBodyDryRun))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", sessionID)
		w = newCloseNotifierRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, mcpServerCallCount)

		// 3. Now call execute_bash again (must be allowed because dry_run_command was executed within 2 steps)
		req, _ = http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", sessionID)
		w = newCloseNotifierRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 2, mcpServerCallCount)
	}
}

func TestYAMLPolicyFSMIntegration(t *testing.T) {
	viper.Set("testing.allow_private_urls", true)
	t.Cleanup(func() { viper.Reset() })

	// 1. Write temp policies.yaml with FSM configuration
	tmpPolicyFile, err := os.CreateTemp("", "policies*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp policy file: %v", err)
	}
	defer os.Remove(tmpPolicyFile.Name())

	yamlContent := []byte(`
version: loopers.com/v1alpha1
metadata:
  name: integration-fsm-policy
fsm:
  initial_state: UNAUTHENTICATED
  transitions:
    - from: UNAUTHENTICATED
      to: AUTHENTICATED
      trigger: login
    - from: AUTHENTICATED
      to: TRANSACTION_ACTIVE
      trigger: start_transaction
rules:
  - name: block-unauthorized-db-query
    match:
      type: mcp_tool_call
      tool: database_query
    conditions:
      - field: session.state
        op: not_equals
        value: TRANSACTION_ACTIVE
    action: deny
    reason: "Blocked: Database queries are only allowed in TRANSACTION_ACTIVE state."
`)
	if _, err := tmpPolicyFile.Write(yamlContent); err != nil {
		t.Fatalf("failed to write YAML policy content: %v", err)
	}
	tmpPolicyFile.Close()

	// 2. Setup mock upstream MCP server
	mcpServerCallCount := 0
	mcpUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mcpServerCallCount++
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "Success from filesystem server"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mcpUpstream.Close()

	// 3. Configure viper
	viper.Set("policy.enabled", true)
	viper.Set("policy.policy_file", tmpPolicyFile.Name())
	viper.Set("policy.policy_dir", t.TempDir())
	viper.Set("policy.default_action", "allow")

	viper.Set("mcp", map[string]interface{}{
		"enabled": true,
		"servers": []map[string]interface{}{
			{"name": "filesystem", "url": mcpUpstream.URL},
		},
	})

	// 4. Setup Loopers Server with miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient, err := budget.NewClient(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer redisClient.Close()

	tmpPricing, _ := os.CreateTemp("", "pricing*.yaml")
	defer os.Remove(tmpPricing.Name())
	tmpPricing.Write([]byte(`
model_costs:
  openai:
    gpt-4o:
      input_cost_per_token: 0.000005
      output_cost_per_token: 0.000015
tool_costs:
  defaults:
    unknown_tool: 0.01
`))
	tmpPricing.Close()

	pricingStore, _ := pricing.LoadStore(tmpPricing.Name())

	s := NewServer(redisClient, pricingStore)
	defer s.Shutdown()
	r := s.GetRouter()

	// 5. Register Key in keyring
	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)
	ctx := context.Background()
	rdb := redisClient.GetUnderlyingClient()

	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":     "test-key",
		"provider": "mock",
		"active":   "true",
	})
	defer rdb.Del(ctx, "loopers:key:"+keyHash)

	configKey := "loopers:budget:{" + keyHash + "}:config"
	rdb.HSet(ctx, configKey, "daily", "100.00")
	defer rdb.Del(ctx, configKey)

	sessionID := "12345678-1234-1234-1234-1234567890ab"

	// Integration Step 1: Call database_query (should be blocked because initial state is UNAUTHENTICATED, not TRANSACTION_ACTIVE)
	{
		reqBody := []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "tools/call",
			"params": {
				"name": "database_query",
				"arguments": {}
			}
		}`)
		req, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", sessionID)
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		errVal, ok := resp["error"]
		assert.True(t, ok)
		assert.NotNil(t, errVal)
		errObj := errVal.(map[string]interface{})
		assert.Contains(t, errObj["message"].(string), "Blocked: Database queries are only allowed in TRANSACTION_ACTIVE state.")
		assert.Equal(t, 0, mcpServerCallCount)
	}

	// Integration Step 2: Call login (should succeed and transition state to AUTHENTICATED)
	{
		reqBody := []byte(`{
			"jsonrpc": "2.0",
			"id": 2,
			"method": "tools/call",
			"params": {
				"name": "login",
				"arguments": {}
			}
		}`)
		req, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", sessionID)
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, mcpServerCallCount)
	}

	// Give async transition goroutine a moment to write to Redis
	time.Sleep(10 * time.Millisecond)

	// Integration Step 3: Call database_query again (should STILL be blocked because state is AUTHENTICATED, not TRANSACTION_ACTIVE)
	{
		reqBody := []byte(`{
			"jsonrpc": "2.0",
			"id": 3,
			"method": "tools/call",
			"params": {
				"name": "database_query",
				"arguments": {}
			}
		}`)
		req, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", sessionID)
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		errVal, ok := resp["error"]
		assert.True(t, ok)
		assert.NotNil(t, errVal)
		errObj := errVal.(map[string]interface{})
		assert.Contains(t, errObj["message"].(string), "Blocked: Database queries are only allowed in TRANSACTION_ACTIVE state.")
		assert.Equal(t, 1, mcpServerCallCount)
	}

	// Integration Step 4: Call start_transaction (should succeed and transition state to TRANSACTION_ACTIVE)
	{
		reqBody := []byte(`{
			"jsonrpc": "2.0",
			"id": 4,
			"method": "tools/call",
			"params": {
				"name": "start_transaction",
				"arguments": {}
			}
		}`)
		req, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", sessionID)
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 2, mcpServerCallCount)
	}

	// Give async transition goroutine a moment to write to Redis
	time.Sleep(10 * time.Millisecond)

	// Integration Step 5: Call database_query again (should now succeed because state is TRANSACTION_ACTIVE)
	{
		reqBody := []byte(`{
			"jsonrpc": "2.0",
			"id": 5,
			"method": "tools/call",
			"params": {
				"name": "database_query",
				"arguments": {}
			}
		}`)
		req, _ := http.NewRequest("POST", "/mcp/filesystem/tools/call", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", sessionID)
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 3, mcpServerCallCount)
	}
}
