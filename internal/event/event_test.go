package event

import (
	"context"
	"testing"
	"time"
)

func TestEmitBlockEvent(t *testing.T) {
	// Emit a block event with empty context to verify it doesn't panic
	e := BlockEvent{
		Timestamp: time.Now(),
		EventType: "test_block",
		Reason:    "testing event logging",
		Detail:    "details here",
	}

	// This should run smoothly and log to standard test logger/stdout
	EmitBlockEvent(context.Background(), e)
}
