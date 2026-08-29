package otel

import (
	"context"
	"fmt"

	"github.com/try-loopers/loopers-oss/internal/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Config struct {
	Enabled      bool              `mapstructure:"enabled"`
	Endpoint     string            `mapstructure:"endpoint"`
	Protocol     string            `mapstructure:"protocol"`
	Insecure     bool              `mapstructure:"insecure"`
	SamplingRate float64           `mapstructure:"sampling_rate"`
	Headers      map[string]string `mapstructure:"headers"`
}

// Init sets up the global TracerProvider and configures OTLP export.
// Returns a shutdown function that must be called on server exit.
func Init(cfg Config, version string) (func(context.Context) error, error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	if cfg.Protocol == "" {
		cfg.Protocol = "grpc"
	}

	var exporter trace.SpanExporter
	var err error

	if cfg.Protocol == "stdout" {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	} else {
		var client otlptrace.Client
		if cfg.Protocol == "http" {
			opts := []otlptracehttp.Option{
				otlptracehttp.WithEndpoint(cfg.Endpoint),
				otlptracehttp.WithHeaders(cfg.Headers),
			}
			if cfg.Insecure {
				opts = append(opts, otlptracehttp.WithInsecure())
			}
			client = otlptracehttp.NewClient(opts...)
		} else {
			opts := []otlptracegrpc.Option{
				otlptracegrpc.WithEndpoint(cfg.Endpoint),
				otlptracegrpc.WithHeaders(cfg.Headers),
			}
			if cfg.Insecure {
				opts = append(opts, otlptracegrpc.WithInsecure())
			}
			client = otlptracegrpc.NewClient(opts...)
		}

		exporter, err = otlptrace.New(context.Background(), client)
		if err != nil {
			return nil, fmt.Errorf("failed to create otlp exporter: %w", err)
		}
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("loopers-oss"),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	// Create BatchSpanProcessor
	bsp := trace.NewBatchSpanProcessor(exporter)

	// Wrap in our filtering smart sampler processor
	filteringProc := NewFilteringProcessor(bsp, cfg.SamplingRate)

	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()), // 100% sampling at the head, filtered at the tail by filteringProcessor
		trace.WithSpanProcessor(filteringProc),
		trace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	logging.Logger.Info().
		Str("endpoint", cfg.Endpoint).
		Str("protocol", cfg.Protocol).
		Float64("sampling_rate", cfg.SamplingRate).
		Msg("OpenTelemetry initialized")

	return tp.Shutdown, nil
}
