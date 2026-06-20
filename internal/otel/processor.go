package otel

import (
	"context"
	"encoding/binary"
	"math"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// forceSampleKey is the attribute key used to promote a span to be exported.
const forceSampleKey = attribute.Key("loopers.force_sample")

// PromoteToSampled adds an attribute to the span that ensures it will be exported,
// even if it would have been dropped by the probabilistic sampler.
func PromoteToSampled(span trace.Span) {
	span.SetAttributes(forceSampleKey.Bool(true))
}

// filteringProcessor implements smart sampling. It wraps a downstream processor
// (usually BatchSpanProcessor) and decides which spans actually get exported.
type filteringProcessor struct {
	next  sdktrace.SpanProcessor
	rate  float64
	bound uint64
}

// NewFilteringProcessor creates a processor that drops spans unless they:
// 1. Were explicitly forced via PromoteToSampled()
// 2. Are probabilistically sampled based on the given rate
// 3. Have an upstream parent that was already sampled
func NewFilteringProcessor(next sdktrace.SpanProcessor, rate float64) sdktrace.SpanProcessor {
	bound := uint64(0)
	if rate >= 1.0 {
		bound = math.MaxUint64
	} else if rate > 0.0 {
		bound = uint64(rate * float64(math.MaxUint64))
	}

	return &filteringProcessor{
		next:  next,
		rate:  rate,
		bound: bound,
	}
}

func (f *filteringProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	f.next.OnStart(parent, s)
}

func (f *filteringProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	if f.shouldExport(s) {
		f.next.OnEnd(s)
	}
}

func (f *filteringProcessor) shouldExport(s sdktrace.ReadOnlySpan) bool {
	// 1. Force override takes precedence
	for _, attr := range s.Attributes() {
		if attr.Key == forceSampleKey && attr.Value.AsBool() {
			return true
		}
	}

	// 2. Respect upstream parent sampling decision
	if s.Parent().IsValid() && s.Parent().IsSampled() {
		return true
	}

	// 3. Probabilistic sampling based on TraceID (consistent within a trace)
	if f.rate >= 1.0 {
		return true
	}
	if f.rate <= 0.0 {
		return false
	}

	traceID := s.SpanContext().TraceID()
	// Use lower 64 bits of TraceID (bytes 8-15) for random distribution
	v := binary.BigEndian.Uint64(traceID[8:16])
	return v < f.bound
}

func (f *filteringProcessor) Shutdown(ctx context.Context) error {
	return f.next.Shutdown(ctx)
}

func (f *filteringProcessor) ForceFlush(ctx context.Context) error {
	return f.next.ForceFlush(ctx)
}
