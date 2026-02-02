package telemetry

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Init configures OpenTelemetry tracing.
func Init(ctx context.Context, serviceName, serviceVersion, explicitEndpoint string) (func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	endpoint := explicitEndpoint
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}

	if endpoint != "" {
		// Configure OTLP HTTP exporter.
		exporter, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		// fmt.Printf(" [Telemtry] Initialized OTLP Exporter -> %s\n", endpoint)
	} else {
		// Configure no-op exporter.
		exporter, err = stdouttrace.New(stdouttrace.WithWriter(io.Discard))
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Register global tracer provider.
	otel.SetTracerProvider(tp)

	// Configure propagation.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// Return shutdown function.
	return tp.Shutdown, nil
}

// Tracer returns a named OTel tracer.
func Tracer(name string) interface{} {
	return otel.Tracer(name)
}
