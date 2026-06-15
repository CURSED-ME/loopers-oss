package pricing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// --- FetchRemotePricing edge cases ---

func TestFetchRemotePricing_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := FetchRemotePricing(server.URL)
	if err == nil {
		t.Fatal("expected error on non-200 response, got nil")
	}
}

func TestFetchRemotePricing_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not-valid-json`))
	}))
	defer server.Close()

	_, err := FetchRemotePricing(server.URL)
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}

func TestFetchRemotePricing_OversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write a body much larger than the 10 MB limit to confirm LimitReader truncates it
		// We write enough to trigger a JSON decode error after limit is hit
		w.Write([]byte(`{"providers": {`))
		chunk := make([]byte, 1024)
		for i := range chunk {
			chunk[i] = 'a'
		}
		for i := 0; i < 12*1024; i++ { // 12 MB total
			w.Write(chunk)
		}
	}))
	defer server.Close()

	// Should fail to decode (truncated JSON), not OOM
	_, err := FetchRemotePricing(server.URL)
	if err == nil {
		t.Fatal("expected error on oversized/truncated body, got nil")
	}
}

func TestFetchRemotePricing_NetworkError(t *testing.T) {
	// Use a port that is not listening
	_, err := FetchRemotePricing("http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error on connection refused, got nil")
	}
}

// --- MergeRemote: additional invariants ---

func TestMergeRemote_Idempotent(t *testing.T) {
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

	remote := map[string]ProviderConfig{
		"openai": {
			Models: map[string]ModelPrice{
				"gpt-4o": {InputPer1M: 2.5, OutputPer1M: 10.0},
			},
		},
	}

	store.MergeRemote(remote)
	store.MergeRemote(remote) // second merge must be a no-op

	inp, out, _ := store.GetModelPrice("openai", "gpt-4o")
	if inp != 2.5 || out != 10.0 {
		t.Errorf("Expected gpt-4o to be 2.5/10.0 after idempotent merge, got %v/%v", inp, out)
	}
	inp, _, _ = store.GetModelPrice("openai", "gpt-4")
	if inp != 10.0 {
		t.Errorf("gpt-4 should remain 10.0 after idempotent merge, got %v", inp)
	}
}

func TestMergeRemote_DefaultMaxOutputTokens_Inherited(t *testing.T) {
	store := &Store{
		config: Config{
			Providers: map[string]ProviderConfig{
				"generic-co": {
					DefaultMaxOutputTokens: 0, // unset locally
					Models:                 map[string]ModelPrice{},
				},
			},
		},
	}

	remote := map[string]ProviderConfig{
		"generic-co": {
			DefaultMaxOutputTokens: 4096,
			Models:                 map[string]ModelPrice{},
		},
	}

	store.MergeRemote(remote)

	_, _, maxOut := store.GetModelPrice("generic-co", "any-model")
	if maxOut != 4096 {
		t.Errorf("Expected DefaultMaxOutputTokens to be inherited as 4096, got %d", maxOut)
	}
}

func TestMergeRemote_DefaultMaxOutputTokens_NotOverwritten(t *testing.T) {
	store := &Store{
		config: Config{
			Providers: map[string]ProviderConfig{
				"openai": {
					DefaultMaxOutputTokens: 8192, // set locally
					Models:                 map[string]ModelPrice{},
				},
			},
		},
	}

	remote := map[string]ProviderConfig{
		"openai": {
			DefaultMaxOutputTokens: 4096,
			Models:                 map[string]ModelPrice{},
		},
	}

	store.MergeRemote(remote)

	_, _, maxOut := store.GetModelPrice("openai", "_fallback")
	if maxOut != 8192 {
		t.Errorf("Expected local DefaultMaxOutputTokens (8192) to NOT be overwritten, got %d", maxOut)
	}
}

func TestMergeRemote_NilProvidersMap_ColdStart(t *testing.T) {
	// Simulates a store loaded from an empty config
	store := &Store{
		config: Config{Providers: nil},
	}

	remote := map[string]ProviderConfig{
		"openrouter": {
			DefaultMaxOutputTokens: 2048,
			Models: map[string]ModelPrice{
				"llama-3": {InputPer1M: 0.5, OutputPer1M: 1.5},
			},
		},
	}

	store.MergeRemote(remote) // must not panic

	inp, _, _ := store.GetModelPrice("openrouter", "llama-3")
	if inp != 0.5 {
		t.Errorf("Expected 0.5, got %v", inp)
	}
}

func TestMergeRemote_Concurrency(t *testing.T) {
	store := &Store{
		config: Config{
			Providers: map[string]ProviderConfig{
				"openai": {Models: map[string]ModelPrice{
					"gpt-4": {InputPer1M: 10.0},
				}},
			},
		},
	}

	remote := map[string]ProviderConfig{
		"openai": {Models: map[string]ModelPrice{
			"gpt-4o": {InputPer1M: 2.5},
		}},
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			store.MergeRemote(remote)
		}()
		go func() {
			defer wg.Done()
			store.GetModelPrice("openai", "gpt-4")
		}()
	}
	wg.Wait()
}

// --- StartRemoteFetcher lifecycle ---

func TestStartRemoteFetcher_CancelStopsGoroutine(t *testing.T) {
	fetchCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fetchCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"providers":{}}`))
	}))
	defer server.Close()

	store := &Store{config: Config{Providers: map[string]ProviderConfig{}}}

	ctx, cancel := context.WithCancel(context.Background())
	store.StartRemoteFetcher(ctx, server.URL, 1) // 1 hour, but initial fetch fires immediately

	// Wait for initial fetch
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	countBeforeCancel := fetchCount
	mu.Unlock()

	if countBeforeCancel < 1 {
		t.Error("Expected at least one initial fetch before cancel")
	}

	cancel() // signal goroutine to stop
	time.Sleep(50 * time.Millisecond)

	// No additional fetches should occur after cancel (ticker is 1hr, so no new tick)
	mu.Lock()
	countAfterCancel := fetchCount
	mu.Unlock()

	if countAfterCancel != countBeforeCancel {
		t.Errorf("Expected no new fetches after cancel, but count went from %d to %d", countBeforeCancel, countAfterCancel)
	}
}

func TestStartRemoteFetcher_EmptyURL_NoOp(t *testing.T) {
	store := &Store{config: Config{}}
	ctx := context.Background()
	// Must not panic and must not start a goroutine
	store.StartRemoteFetcher(ctx, "", 24)
	store.StartRemoteFetcher(ctx, "http://valid.url", 0)   // zero refresh hours
	store.StartRemoteFetcher(ctx, "http://valid.url", -1) // negative refresh hours
}

func TestStartRemoteFetcher_FetchFailure_DoesNotPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	store := &Store{config: Config{Providers: map[string]ProviderConfig{}}}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Must not panic even if the initial fetch fails
	store.StartRemoteFetcher(ctx, server.URL, 1)
	<-ctx.Done()
}
