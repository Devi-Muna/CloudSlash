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

// Init configures OpenTelemetry.
// Checks OTEL_EXPORTER_OTLP_ENDPOINT.
// Configures OTLP HTTP or fallback.
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
		// Mode: Enterprise (OTLP HTTP).
		exporter, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		// fmt.Printf(" [Telemtry] Initialized OTLP Exporter -> %s\n", endpoint)
	} else {
		// Mode: Privacy (Local/No-Op).
		// Discard output by default.
		exporter, err = stdouttrace.New(stdouttrace.WithWriter(io.Discard))
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Register global providers.
	otel.SetTracerProvider(tp)
	
	// Configure W3C propagation.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// Return cleanup function.
	return tp.Shutdown, nil
}

// Tracer returns a named tracer.
func Tracer(name string) interface{} {
	return otel.Tracer(name)
}
