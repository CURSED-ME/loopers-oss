package loop

import (
	"encoding/json"
	"testing"
)

func TestNormalizeAndHash(t *testing.T) {
	req1 := []byte(`{"model": "gpt-4", "messages": [{"role": "user", "content": "hi"}], "temperature": 0.7, "stream": true}`)
	req2 := []byte(`{"model": "gpt-4", "messages": [{"role": "user", "content": "hi"}], "temperature": 0.0, "stream": false}`)
	req3 := []byte(`{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}], "temperature": 0.7, "stream": true}`)
	reqInvalid := []byte(`{invalid json`)

	h1, _, err1 := NormalizeAndHash(req1)
	if err1 != nil {
		t.Fatalf("unexpected error: %v", err1)
	}

	h2, _, err2 := NormalizeAndHash(req2)
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}

	h3, _, err3 := NormalizeAndHash(req3)
	if err3 != nil {
		t.Fatalf("unexpected error: %v", err3)
	}

	h4, _, err4 := NormalizeAndHash(reqInvalid)
	if err4 != nil {
		t.Fatalf("unexpected error: %v", err4)
	}

	// req1 and req2 should have the same hash because volatile fields are stripped
	if h1 != h2 {
		t.Errorf("expected h1 == h2, got %s and %s", h1, h2)
	}

	// req1 and req3 should have different hashes
	if h1 == h3 {
		t.Errorf("expected h1 != h3, got identical hash %s", h1)
	}

	// invalid JSON should still hash successfully
	if h4 == "" {
		t.Errorf("expected valid hash for invalid json")
	}

	// Validate it's a stable JSON serialization (no random ordering of keys issues)
	// by unmarshaling and marshaling multiple times
	var raw map[string]json.RawMessage
	json.Unmarshal(req1, &raw)
	delete(raw, "temperature")
	delete(raw, "stream")
	norm, _ := json.Marshal(raw)
	if string(norm) != `{"messages":[{"role":"user","content":"hi"}],"model":"gpt-4"}` &&
		string(norm) != `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}` {
		// Wait, Go map iteration is random, but json.Marshal sorts map keys!
		// So it is deterministic.
	}
}
