package otel

import (
	"context"
	"testing"
)

func TestInitDisabled(t *testing.T) {
	cfg := Config{Enabled: false}
	shutdown, err := Init(cfg, "dev")
	if err != nil {
		t.Fatalf("expected no error when disabled, got %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("expected nil error on shutdown, got %v", err)
	}
}

func TestProcessorFiltering(t *testing.T) {
	// A simple test ensuring the filtering processor logic bounds correctly.
	proc := NewFilteringProcessor(nil, 0.5).(*filteringProcessor)
	if proc.rate != 0.5 {
		t.Errorf("expected rate 0.5, got %v", proc.rate)
	}
	if proc.bound == 0 {
		t.Errorf("expected non-zero bound for 0.5 rate")
	}
}
