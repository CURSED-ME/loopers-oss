package alerting

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNewBudgetBlockEvent(t *testing.T) {
	ctx := context.Background()
	reqID := "req-123"
	details := map[string]string{"key": "value"}

	evt := NewBudgetBlockEvent(ctx, reqID, details)

	if evt.SchemaVersion != "2.0.0" {
		t.Errorf("expected schema_version 2.0.0, got %s", evt.SchemaVersion)
	}
	if evt.EventID == "" {
		t.Error("expected non-empty EventID")
	}
	if evt.LoopersEventType != "BUDGET_BLOCK" {
		t.Errorf("expected BUDGET_BLOCK, got %s", evt.LoopersEventType)
	}
	if evt.RequestID != "req-123" {
		t.Errorf("expected request_id req-123, got %s", evt.RequestID)
	}
	if len(evt.Regulation) == 0 {
		t.Error("expected regulation tags")
	}
	if evt.Action.Type != "blocked" {
		t.Errorf("expected action blocked, got %s", evt.Action.Type)
	}

	// Test JSON serialization
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if parsed["loopers_event_type"] != "BUDGET_BLOCK" {
		t.Errorf("expected loopers_event_type in JSON, got %v", parsed["loopers_event_type"])
	}
}

func TestNilContext(t *testing.T) {
	// Should not panic
	evt := NewFailClosedEvent(nil, "", "test", nil)
	if evt.TraceID != "" {
		t.Errorf("expected empty trace id")
	}
}
