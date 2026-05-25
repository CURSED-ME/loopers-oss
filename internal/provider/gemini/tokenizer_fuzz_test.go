package gemini

import (
	"testing"
)

// FuzzCountGeminiTokensFallback tests that the local fallback token counter
// doesn't panic when provided with arbitrary byte slices (e.g. malformed JSON).
func FuzzCountGeminiTokensFallback(f *testing.F) {
	// Add some seed corpus examples
	f.Add([]byte(`{"contents":[{"role":"user","parts":[{"text":"hello world"}]}]}`))
	f.Add([]byte(`{"contents":[]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		// The function should gracefully return an error for bad data,
		// and never panic.
		_, _ = countGeminiTokensFallback(data)
	})
}
