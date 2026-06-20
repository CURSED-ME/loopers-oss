package otel

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceMiddleware extracts incoming trace context and creates a root span for the proxy request.
func TraceMiddleware(enabled bool) gin.HandlerFunc {
	if !enabled {
		return func(c *gin.Context) { c.Next() }
	}

	tracer := otel.Tracer("loopers-proxy")
	propagator := otel.GetTextMapPropagator()

	return func(c *gin.Context) {
		// Extract incoming trace context from HTTP headers
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// Start the span
		ctx, span := tracer.Start(ctx, "loopers.proxy.request", trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.path", c.Request.URL.Path),
		)

		// Update the request context with the span
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		status := c.Writer.Status()
		if status >= 400 {
			span.SetStatus(codes.Error, "HTTP error status")
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}
}
