package event

import (
	"context"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/CURSED-ME/loopers-oss/internal/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type BlockEvent struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"` // "budget_block", "loop_block", "rate_limit_block", "circuit_breaker_trip", "blast_radius_block"
	SessionID string    `json:"session_id,omitempty"`
	KeyHash   string    `json:"key_hash,omitempty"`
	Provider  string    `json:"provider,omitempty"`
	Model     string    `json:"model,omitempty"`
	ToolName  string    `json:"tool_name,omitempty"`
	Reason    string    `json:"reason"`
	Detail    string    `json:"detail"`
	RequestID string    `json:"request_id,omitempty"`
}

// EmitBlockEvent logs the block event via zerolog AND emits it as an OTel span event.
func EmitBlockEvent(ctx context.Context, e BlockEvent) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	// 1. Log via zerolog
	logger := logging.Logger.Warn().
		Time("timestamp", e.Timestamp).
		Str("event_type", e.EventType).
		Str("reason", e.Reason).
		Str("detail", e.Detail)

	if e.SessionID != "" {
		logger = logger.Str("session_id", e.SessionID)
	}
	if e.KeyHash != "" {
		logger = logger.Str("key_hash", e.KeyHash)
	}
	if e.Provider != "" {
		logger = logger.Str("provider", e.Provider)
	}
	if e.Model != "" {
		logger = logger.Str("model", e.Model)
	}
	if e.ToolName != "" {
		logger = logger.Str("tool_name", e.ToolName)
	}
	if e.RequestID != "" {
		logger = logger.Str("request_id", e.RequestID)
	}

	logger.Msg("Access blocked")

	// 2. Emit OTel span event
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		otel.PromoteToSampled(span)
		attrs := []attribute.KeyValue{
			attribute.String("event_type", e.EventType),
			attribute.String("reason", e.Reason),
			attribute.String("detail", e.Detail),
		}

		if e.SessionID != "" {
			attrs = append(attrs, attribute.String("session_id", e.SessionID))
		}
		if e.KeyHash != "" {
			attrs = append(attrs, attribute.String("key_hash", e.KeyHash))
		}
		if e.Provider != "" {
			attrs = append(attrs, attribute.String("provider", e.Provider))
		}
		if e.Model != "" {
			attrs = append(attrs, attribute.String("model", e.Model))
		}
		if e.ToolName != "" {
			attrs = append(attrs, attribute.String("tool_name", e.ToolName))
		}

		span.SetAttributes(attribute.String("loopers.enforcement.action", "blocked"))
		span.AddEvent("loopers.enforcement.block", trace.WithAttributes(attrs...))
	}
}
