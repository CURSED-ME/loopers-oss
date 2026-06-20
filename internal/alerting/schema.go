package alerting

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

var Version = "dev" // Injected via -ldflags at build time

type OWASPMetadata struct {
	Category string `json:"owasp_category"`
	Name     string `json:"owasp_name"`
	Severity string `json:"severity"`
}

// SecurityEventEnvelope wraps every security event with traceability fields
// required by EU AI Act Article 14 (human oversight) and Article 12 (record-keeping).
type SecurityEventEnvelope struct {
	SchemaVersion    string        `json:"schema_version"`
	EventID          string        `json:"event_id"`
	LoopersEventType string        `json:"loopers_event_type"`
	TraceID          string        `json:"trace_id,omitempty"`
	SpanID           string        `json:"span_id,omitempty"`
	RequestID        string        `json:"request_id,omitempty"`
	Timestamp        string        `json:"timestamp"`
	Source           EventSource   `json:"source"`
	OWASP            OWASPMetadata `json:"owasp"`
	Regulation       []string      `json:"regulation,omitempty"`
	Action           ActionTaken   `json:"action"`
	Event            string        `json:"event"`
	Details          interface{}   `json:"details"`
}

// EventType implements the AlertEvent interface.
func (e *SecurityEventEnvelope) EventType() string {
	return e.Event
}

type EventSource struct {
	Component string `json:"component"`
	Version   string `json:"version"`
	Instance  string `json:"instance,omitempty"`
}

type ActionTaken struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// Base constructor for envelopes
func newEnvelope(ctx context.Context, requestID, eventType, loopersEventType, actionType, actionReason string, owasp OWASPMetadata, details interface{}) *SecurityEventEnvelope {
	id, err := uuid.NewRandom()
	eventID := ""
	if err != nil {
		eventID = "evt-" + time.Now().Format("20060102150405.000")
	} else {
		eventID = id.String()
	}

	var traceID, spanID string
	if ctx != nil {
		spanCtx := trace.SpanContextFromContext(ctx)
		if spanCtx.HasTraceID() {
			traceID = spanCtx.TraceID().String()
		}
		if spanCtx.HasSpanID() {
			spanID = spanCtx.SpanID().String()
		}
	}

	return &SecurityEventEnvelope{
		SchemaVersion:    "2.0.0",
		EventID:          eventID,
		LoopersEventType: loopersEventType,
		TraceID:          traceID,
		SpanID:           spanID,
		RequestID:        requestID,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
		Source: EventSource{
			Component: "loopers_proxy",
			Version:   Version,
		},
		OWASP:      owasp,
		Regulation: defaultRegulationTags,
		Action: ActionTaken{
			Type:   actionType,
			Reason: actionReason,
		},
		Event:   eventType,
		Details: details,
	}
}

var defaultRegulationTags = []string{"EU_AI_ACT_ART_12", "EU_AI_ACT_ART_14"}

// Event constructors

func NewBudgetBlockEvent(ctx context.Context, requestID string, details interface{}) *SecurityEventEnvelope {
	return newEnvelope(ctx, requestID, "budget_exceeded", "BUDGET_BLOCK", "blocked", "Cost exceeds budget limits", OWASPMetadata{
		Category: "LLM10:2025",
		Name:     "Unbounded Consumption",
		Severity: "critical",
	}, details)
}

func NewBudgetThresholdEvent(ctx context.Context, requestID string, details interface{}) *SecurityEventEnvelope {
	return newEnvelope(ctx, requestID, "budget_threshold", "BUDGET_THRESHOLD", "warned", "Budget threshold reached", OWASPMetadata{
		Category: "LLM10:2025",
		Name:     "Unbounded Consumption",
		Severity: "high",
	}, details)
}

func NewLoopBlockEvent(ctx context.Context, requestID string, details interface{}) *SecurityEventEnvelope {
	return newEnvelope(ctx, requestID, "loop_detected", "LOOP_BLOCK", "blocked", "Autonomous loop detected and blocked", OWASPMetadata{
		Category: "LLM06:2025",
		Name:     "Excessive Agency",
		Severity: "critical",
	}, details)
}

func NewLoopWarnEvent(ctx context.Context, requestID string, details interface{}) *SecurityEventEnvelope {
	return newEnvelope(ctx, requestID, "loop_detected", "LOOP_WARN", "warned", "Potential autonomous loop flagged", OWASPMetadata{
		Category: "LLM06:2025",
		Name:     "Excessive Agency",
		Severity: "medium",
	}, details)
}

func NewAuthFailEvent(ctx context.Context, requestID string, reason string, details interface{}) *SecurityEventEnvelope {
	return newEnvelope(ctx, requestID, "auth_failure", "AUTH_FAIL", "blocked", reason, OWASPMetadata{
		Category: "LLM10:2025",
		Name:     "Unbounded Consumption",
		Severity: "high",
	}, details)
}

func NewFailClosedEvent(ctx context.Context, requestID string, reason string, details interface{}) *SecurityEventEnvelope {
	return newEnvelope(ctx, requestID, "fail_closed", "FAIL_CLOSED", "fail_closed", reason, OWASPMetadata{
		Category: "LLM10:2025", // Availability/fail-closed
		Name:     "Unbounded Consumption",
		Severity: "critical",
	}, details)
}
