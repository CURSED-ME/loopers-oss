package event

import (
	"context"
	"time"

	"github.com/try-loopers/loopers-oss/internal/logging"
	"github.com/try-loopers/loopers-oss/internal/otel"
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

// EscalationEvent is emitted when a request is suspended pending human or SaaS approval.
type EscalationEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	SessionID  string    `json:"session_id,omitempty"`
	KeyHash    string    `json:"key_hash,omitempty"`
	Provider   string    `json:"provider,omitempty"`
	ToolName   string    `json:"tool_name,omitempty"`
	EscalateTo string    `json:"escalate_to"` // "human" | "saas_control_plane"
	Reason     string    `json:"reason"`
	Evidence   []string  `json:"evidence,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
}

// EmitEscalationEvent logs the escalation event and emits it as an OTel span event.
func EmitEscalationEvent(ctx context.Context, e EscalationEvent) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	logger := logging.Logger.Warn().
		Time("timestamp", e.Timestamp).
		Str("event_type", "escalation").
		Str("escalate_to", e.EscalateTo).
		Str("reason", e.Reason)

	if e.SessionID != "" {
		logger = logger.Str("session_id", e.SessionID)
	}
	if e.KeyHash != "" {
		logger = logger.Str("key_hash", e.KeyHash)
	}
	if e.Provider != "" {
		logger = logger.Str("provider", e.Provider)
	}
	if e.ToolName != "" {
		logger = logger.Str("tool_name", e.ToolName)
	}
	if e.RequestID != "" {
		logger = logger.Str("request_id", e.RequestID)
	}

	logger.Msg("Access suspended for escalation")

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		otel.PromoteToSampled(span)
		attrs := []attribute.KeyValue{
			attribute.String("event_type", "escalation"),
			attribute.String("escalate_to", e.EscalateTo),
			attribute.String("reason", e.Reason),
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
		if e.ToolName != "" {
			attrs = append(attrs, attribute.String("tool_name", e.ToolName))
		}

		span.SetAttributes(attribute.String("loopers.enforcement.action", "escalated"))
		span.AddEvent("loopers.enforcement.escalation", trace.WithAttributes(attrs...))
	}
}

// QuarantineEvent is emitted when an agent is quarantined by policy decision.
type QuarantineEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	KeyHash       string    `json:"key_hash"`
	Reason        string    `json:"reason"`
	QuarantineFor string    `json:"quarantine_for"` // Duration string
	Evidence      []string  `json:"evidence,omitempty"`
	RequestID     string    `json:"request_id,omitempty"`
}

// EmitQuarantineEvent logs the quarantine event and emits it as an OTel span event.
func EmitQuarantineEvent(ctx context.Context, e QuarantineEvent) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	logger := logging.Logger.Warn().
		Time("timestamp", e.Timestamp).
		Str("event_type", "quarantine").
		Str("key_hash", e.KeyHash).
		Str("quarantine_for", e.QuarantineFor).
		Str("reason", e.Reason)

	if e.RequestID != "" {
		logger = logger.Str("request_id", e.RequestID)
	}

	logger.Msg("Agent quarantined")

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		otel.PromoteToSampled(span)
		attrs := []attribute.KeyValue{
			attribute.String("event_type", "quarantine"),
			attribute.String("key_hash", e.KeyHash),
			attribute.String("quarantine_for", e.QuarantineFor),
			attribute.String("reason", e.Reason),
		}

		span.SetAttributes(attribute.String("loopers.enforcement.action", "quarantined"))
		span.AddEvent("loopers.enforcement.quarantine", trace.WithAttributes(attrs...))
	}
}
