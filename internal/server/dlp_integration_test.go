package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/try-loopers/loopers-oss/internal/budget"
	"github.com/try-loopers/loopers-oss/internal/keyring"
	"github.com/try-loopers/loopers-oss/internal/pricing"
	"github.com/try-loopers/loopers-oss/internal/riskprofile"
)

type mockDLPProvider struct {
	baseURL string
}

func (m *mockDLPProvider) Name() string                                     { return "mock" }
func (m *mockDLPProvider) BaseURL() string                                  { return m.baseURL }
func (m *mockDLPProvider) InjectAuth(req *http.Request, providerKey string) {}
func (m *mockDLPProvider) RewritePath(originalPath string) string           { return originalPath }
func (m *mockDLPProvider) ParseRequest(req *http.Request, body []byte) (string, bool, int, []byte, error) {
	var payload map[string]interface{}
	isStream := false
	model := "gpt-4o"
	if err := json.Unmarshal(body, &payload); err == nil {
		if m, ok := payload["model"].(string); ok {
			model = m
		}
		if s, ok := payload["stream"].(bool); ok {
			isStream = s
		}
	}
	return model, isStream, 100, body, nil
}
func (m *mockDLPProvider) RewriteModel(req *http.Request, body []byte, fallbackModel string) ([]byte, error) {
	return body, nil
}
func (m *mockDLPProvider) CountInputTokens(ctx context.Context, model string, body []byte, providerKey string) (int, error) {
	return 10, nil
}
func (m *mockDLPProvider) ParseStreamChunk(chunk []byte) (int, int, bool, error) {
	if bytes.Contains(chunk, []byte("[DONE]")) {
		return 0, 0, true, nil
	}
	return 5, 5, false, nil
}
func (m *mockDLPProvider) ParseNonStreamResponse(body []byte) (int, int, error) {
	return 10, 15, nil
}
func (m *mockDLPProvider) FormatBudgetExceededSSE() []byte {
	return []byte("event: error\ndata: {\"error\":\"budget exceeded\"}\n\n")
}

func TestOutboundDLP_Integration(t *testing.T) {
	viper.Set("testing.allow_private_urls", true)
	t.Cleanup(func() { viper.Reset() })

	var upstreamResponse string
	var isStreamResp bool

	llmUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isStreamResp {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(upstreamResponse))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(upstreamResponse))
	}))
	defer llmUpstream.Close()

	// Miniredis
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	// Pricing file
	tmpPricing, err := os.CreateTemp("", "pricing*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpPricing.Name())
	_, _ = tmpPricing.WriteString(`
model_costs:
  mock:
    gpt-4o:
      input_cost_per_token: 0.000005
      output_cost_per_token: 0.000015
`)
	_ = tmpPricing.Close()

	pricingStore, err := pricing.LoadStore(tmpPricing.Name())
	assert.NoError(t, err)

	bClient, err := budget.NewClient(mr.Addr(), "", 0)
	assert.NoError(t, err)
	defer bClient.Close()

	rdb := bClient.GetUnderlyingClient()
	rawKey, _ := keyring.GenerateRawKey()
	keyHash := keyring.HashKey(rawKey)

	rdb.HSet(context.Background(), "loopers:key:"+keyHash, map[string]interface{}{
		"name":       "test-agent-key",
		"agent_name": "dlp-agent",
		"provider":   "mock",
		"active":     "true",
	})
	rdb.HSet(context.Background(), "loopers:budget:{"+keyHash+"}:config", "daily", "100.00")

	viper.Set("server.dlp.enabled", true)
	viper.Set("server.dlp.action", "mask")
	viper.Set("server.dlp.scan_secrets", true)
	viper.Set("server.dlp.scan_pii", true)
	viper.Set("server.dlp.quarantine_duration", "1h")
	viper.Set("risk_profile.enabled", true)

	srv := NewServer(bClient, pricingStore)
	defer srv.Shutdown()
	srv.RegisterProviderRoute(&mockDLPProvider{baseURL: llmUpstream.URL})
	router := srv.GetRouter()

	// Test 1: Non-streaming completion with PII (Email & SSN) -> Masked
	t.Run("NonStream_PII_Masked", func(t *testing.T) {
		isStreamResp = false
		upstreamResponse = `{
			"id": "chatcmpl-pii",
			"choices": [
				{
					"message": {
						"role": "assistant",
						"content": "Contact support at admin@secretcorp.com and SSN is 123-45-6789"
					}
				}
			],
			"usage": {"prompt_tokens": 10, "completion_tokens": 15, "total_tokens": 25}
		}`

		reqBody := []byte(`{"model": "gpt-4o", "messages": [{"role": "user", "content": "hello"}]}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("Content-Type", "application/json")

		w := newCloseNotifierRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "true", w.Header().Get("X-Loopers-DLP-Redacted"))

		var respData map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &respData)
		assert.NoError(t, err)

		choices := respData["choices"].([]interface{})
		content := choices[0].(map[string]interface{})["message"].(map[string]interface{})["content"].(string)

		assert.NotContains(t, content, "admin@secretcorp.com")
		assert.NotContains(t, content, "123-45-6789")
		assert.Contains(t, content, "***")
	})

	// Test 2: Non-streaming completion with Secret (AWS key) + Action Quarantine -> 403 & Agent Quarantined
	t.Run("NonStream_Secret_Quarantine", func(t *testing.T) {
		srv.dlpCfg.Action = "quarantine"
		isStreamResp = false
		upstreamResponse = `{
			"id": "chatcmpl-secret",
			"choices": [
				{
					"message": {
						"role": "assistant",
						"content": "Here is the key: AKIAIOSFODNN7EXAMPLE"
					}
				}
			],
			"usage": {"prompt_tokens": 10, "completion_tokens": 15, "total_tokens": 25}
		}`

		reqBody := []byte(`{"model": "gpt-4o", "messages": [{"role": "user", "content": "hello"}]}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("Content-Type", "application/json")

		w := newCloseNotifierRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "true", w.Header().Get("X-Loopers-DLP-Block"))

		// Verify quarantine key set in Redis
		qExists, _ := rdb.Exists(context.Background(), "loopers:quarantine:"+keyHash).Result()
		assert.Equal(t, int64(1), qExists)

		// Wait briefly for async risk score increment
		time.Sleep(50 * time.Millisecond)
		profile, err := riskprofile.GetProfile(context.Background(), rdb, keyHash)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, profile.RiskScore, 30)
	})

	// Test 3: Subsequent request from quarantined agent is rejected at auth layer
	t.Run("Quarantined_Agent_Rejected_At_Auth", func(t *testing.T) {
		reqBody := []byte(`{"model": "gpt-4o", "messages": [{"role": "user", "content": "hello"}]}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("Content-Type", "application/json")

		w := newCloseNotifierRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "quarantined")
	})

	// Test 4: Streaming completion with PII -> Real-time token masking
	t.Run("Stream_PII_Masked", func(t *testing.T) {
		// Clear quarantine for this test
		_ = rdb.Del(context.Background(), "loopers:quarantine:"+keyHash)
		srv.dlpCfg.Action = "mask"

		isStreamResp = true
		upstreamResponse = "data: {\"choices\":[{\"delta\":{\"content\":\"Email: test@corp.net \"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"done\"}}],\"usage\":{\"input_tokens\":5,\"output_tokens\":5}}\n\n" +
			"data: [DONE]\n\n"

		reqBody := []byte(`{"model": "gpt-4o", "messages": [{"role": "user", "content": "stream"}], "stream": true}`)
		req, _ := http.NewRequest("POST", "/mock/v1/chat", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("X-Loopers-Provider-Key", "dummy")
		req.Header.Set("Content-Type", "application/json")

		w := newCloseNotifierRecorder()
		router.ServeHTTP(w, req)

		respBytes, _ := io.ReadAll(w.Body)
		respStr := string(respBytes)

		assert.NotContains(t, respStr, "test@corp.net")
		assert.Contains(t, respStr, "***")
	})
}
