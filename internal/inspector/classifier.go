package inspector

import "context"

// ClassificationResult contains the output of a semantic classifier.
type ClassificationResult struct {
	RiskScore float64  // 0.0 to 1.0
	Labels    []string // e.g. ["pii", "secret", "injection", "toxic"]
	Truncate  bool     // if true, response should be truncated
}

// ClassifierHook is a pluggable semantic classification hook.
// Operators may implement this to plug in a local model (LlamaGuard, BERT)
// or an external API. The default implementation is a no-op.
type ClassifierHook interface {
	// Classify is called synchronously for high-risk content.
	// It must be non-blocking and context-aware.
	Classify(ctx context.Context, content string) (ClassificationResult, error)
}

// NoOpClassifier is the default ClassifierHook. It performs no classification.
type NoOpClassifier struct{}

// Classify returns an empty classification result without error.
func (n NoOpClassifier) Classify(_ context.Context, _ string) (ClassificationResult, error) {
	return ClassificationResult{}, nil
}
