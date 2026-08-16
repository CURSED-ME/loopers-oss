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

	"github.com/CURSED-ME/loopers-oss/internal/budget"
	"github.com/CURSED-ME/loopers-oss/internal/keyring"
	"github.com/CURSED-ME/loopers-oss/internal/pricing"
	"github.com/CURSED-ME/loopers-oss/internal/riskprofile"
	"github.com/alicebob/miniredis/v2"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestRiskProfile_Integration(t *testing.T) {
	viper.Set("testing.allow_private_urls", true)
	t.Cleanup(func() { viper.Reset() })

	// 1. Mock upstream LLM server
	llmUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "chatcmpl-test",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "Hello from mock LLM"}},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		})
	}))
	defer llmUpstream.Close()

	// 2. Mock upstream MCP server
	mcpUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"result": map[string]interface{}{
				"content": []map[string]string{{"type": "text", "text": "Tool executed successfully"}},
			},
			"id": 1,
		})
	}))
	defer mcpUpstream.Close()

	// 3. Configure viper with policy and risk profile
	tmpPolicyFile, err := os.CreateTemp("", "risk_policies*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp policy file: %v", err)
	}
	defer os.Remove(tmpPolicyFile.Name())

	yamlContent := []byte(`
version: loopers.com/v1alpha1
metadata:
  name: risk-profile-test-guardrails
rules:
  - name: block-forbidden-word
    match:
      type: llm_call
    conditions:
      - field: prompt_text
        op: contains
        value: "forbidden_payload"
    action: deny
    reason: "Blocked: forbidden keyword detected in prompt."

  - name: zero-trust-high-risk-gate
    match:
      type: llm_call
    conditions:
      - field: agent_risk.risk_score
        op: greater_than
        value: "75"
    action: deny
    reason: "Blocked: Agent persistent risk score exceeds 75."
`)
	if _, err := tmpPolicyFile.Write(yamlContent); err != nil {
		t.Fatalf("failed to write policy content: %v", err)
	}
	tmpPolicyFile.Close()

	viper.Set("policy", map[string]interface{}{
		"enabled":        true,
		"policy_file":    tmpPolicyFile.Name(),
		"default_action": "allow",
	})
	viper.Set("risk_profile", map[string]interface{}{
		"enabled":                   true,
		"auto_quarantine_threshold": 75,
		"permanent_block_threshold": 90,
	})
	viper.Set("mcp", map[string]interface{}{
		"enabled": true,
		"servers": []map[string]interface{}{
			{"name": "filesystem", "url": mcpUpstream.URL},
		},
		"inspector": map[string]interface{}{
			"enabled": true,
		},
	})

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
  mock:
    gpt-4o:
      input_cost_per_token: 0.000005
      output_cost_per_token: 0.000015
`))
	tmpPricing.Close()

	pricingStore, _ := pricing.LoadStore(tmpPricing.Name())

	s := NewServer(redisClient, pricingStore)
	defer s.Shutdown()
	r := s.GetRouter()

	s.RegisterProviderRoute(&mockProvider{baseURL: llmUpstream.URL})

	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)
	ctx := context.Background()
	rdb := redisClient.GetUnderlyingClient()

	rdb.HSet(ctx, "loopers:key:"+keyHash, map[string]interface{}{
		"name":     "test-risk-agent",
		"provider": "mock",
		"active":   "true",
	})
	defer rdb.Del(ctx, "loopers:key:"+keyHash)

	configKey := "loopers:budget:{" + keyHash + "}:config"
	rdb.HSet(ctx, configKey, "daily", "100.00")
	defer rdb.Del(ctx, configKey)

	// =========================================================================
	// Scenario A: Clean Request -> Passes, Risk Score is 0
	// =========================================================================
	{
		reqBody := []byte(`{"model":"gpt-4o", "messages":[{"role":"user", "content":"Hello assistant"}]}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		rp, err := riskprofile.GetProfile(ctx, rdb, keyHash)
		assert.NoError(t, err)
		assert.Equal(t, 0, rp.RiskScore)
	}

	// =========================================================================
	// Scenario B: Policy Block -> Increments Risk Score by +10
	// =========================================================================
	{
		reqBody := []byte(`{"model":"gpt-4o", "messages":[{"role":"user", "content":"Here is a forbidden_payload"}]}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		// Wait briefly for asynchronous update
		time.Sleep(50 * time.Millisecond)

		rp, err := riskprofile.GetProfile(ctx, rdb, keyHash)
		assert.NoError(t, err)
		assert.Equal(t, 10, rp.RiskScore)
		assert.Equal(t, int64(1), rp.TotalPolicyBlocks)
	}

	// =========================================================================
	// Scenario C: Auto-Quarantine Triggered Dynamically at Score > 75 (e.g. 76)
	// =========================================================================
	{
		// Manually set score to 76
		_, err := riskprofile.UpdateRiskScore(ctx, rdb, keyHash, 66, false, "test_bump")
		assert.NoError(t, err)

		reqBody := []byte(`{"model":"gpt-4o", "messages":[{"role":"user", "content":"Clean request"}]}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "quarantine_active")

		// Verify quarantine key set in Redis
		quarantineExists, _ := rdb.Exists(ctx, "loopers:quarantine:"+keyHash).Result()
		assert.Equal(t, int64(1), quarantineExists)
	}

	// Clear quarantine key for next test
	rdb.Del(ctx, "loopers:quarantine:"+keyHash)

	// =========================================================================
	// Scenario D: Permanent Block Triggered Dynamically at Score > 90 (e.g. 91)
	// =========================================================================
	{
		// Manually set score to 91
		_, err := riskprofile.UpdateRiskScore(ctx, rdb, keyHash, 15, false, "test_bump_permanent")
		assert.NoError(t, err)

		reqBody := []byte(`{"model":"gpt-4o", "messages":[{"role":"user", "content":"Clean request"}]}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "agent_risk_blocked")
	}

	// =========================================================================
	// Scenario E: Cross-Session Persistence
	// =========================================================================
	{
		// Request with a completely different session ID still hits the permanent block
		reqBody := []byte(`{"model":"gpt-4o", "messages":[{"role":"user", "content":"New session attempt"}]}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("X-Loopers-Session-ID", "11111111-2222-3333-4444-555555555555")
		w := newCloseNotifierRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "agent_risk_blocked")
	}
}
