package logging

import (
	"io"
	"regexp"
)

var (
	// Matches sk- followed by alphanumeric characters, at least 32 chars
	openaiKeyPattern = regexp.MustCompile(`sk-[A-Za-z0-9]{32,}`)
	// Matches sk-ant-api03- followed by alphanumeric characters, at least 93 chars
	anthropicKeyPattern = regexp.MustCompile(`sk-ant-api03-[A-Za-z0-9]{93,}`)
	// Matches AIza followed by alphanumeric characters, 35 chars
	googleKeyPattern = regexp.MustCompile(`AIza[0-9A-Za-z]{35}`)
)

// RedactWriter wraps an io.Writer and filters out provider API keys.
type RedactWriter struct {
	w io.Writer
}

// NewRedactWriter creates a new RedactWriter.
func NewRedactWriter(w io.Writer) io.Writer {
	return &RedactWriter{w: w}
}

// Write replaces any API keys found in the slice with [REDACTED].
func (rw *RedactWriter) Write(p []byte) (n int, err error) {
	mutated := p
	mutated = openaiKeyPattern.ReplaceAll(mutated, []byte("[REDACTED]"))
	mutated = anthropicKeyPattern.ReplaceAll(mutated, []byte("[REDACTED]"))
	mutated = googleKeyPattern.ReplaceAll(mutated, []byte("[REDACTED]"))

	_, err = rw.w.Write(mutated)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
