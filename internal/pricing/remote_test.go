package pricing

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMergeRemote(t *testing.T) {
	store := &Store{
		config: Config{
			Providers: map[string]ProviderConfig{
				"openai": {
					DefaultMaxOutputTokens: 100,
					Models: map[string]ModelPrice{
						"gpt-4": {InputPer1M: 10.0, OutputPer1M: 30.0},
					},
				},
			},
		},
	}

	remoteConfig := map[string]ProviderConfig{
		"openai": {
			DefaultMaxOutputTokens: 200,
			Models: map[string]ModelPrice{
				"gpt-4":  {InputPer1M: 5.0, OutputPer1M: 15.0}, // Should not overwrite
				"gpt-4o": {InputPer1M: 2.5, OutputPer1M: 10.0}, // Should add
			},
		},
		"anthropic": {
			DefaultMaxOutputTokens: 50,
			Models: map[string]ModelPrice{
				"claude-3": {InputPer1M: 3.0, OutputPer1M: 15.0}, // Should add whole provider
			},
		},
	}

	store.MergeRemote(remoteConfig)

	// Check if openai gpt-4 remained unchanged
	if p, _, _ := store.GetModelPrice("openai", "gpt-4"); p != 10.0 {
		t.Errorf("expected gpt-4 to remain 10.0, got %v", p)
	}

	// Check if openai gpt-4o was added
	if p, _, _ := store.GetModelPrice("openai", "gpt-4o"); p != 2.5 {
		t.Errorf("expected gpt-4o to be added with 2.5, got %v", p)
	}

	// Check if anthropic was added
	if p, _, _ := store.GetModelPrice("anthropic", "claude-3"); p != 3.0 {
		t.Errorf("expected claude-3 to be added with 3.0, got %v", p)
	}
}

func TestFetchRemotePricing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"providers": {"test-prov": {"models": {"test-model": {"input_per_1m": 1.23}}}}}`))
	}))
	defer server.Close()

	config, err := FetchRemotePricing(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Providers["test-prov"].Models["test-model"].InputPer1M != 1.23 {
		t.Errorf("expected 1.23, got %v", config.Providers["test-prov"].Models["test-model"].InputPer1M)
	}
}
