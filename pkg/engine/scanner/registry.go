package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/DrSkyle/cloudslash/v2/pkg/engine/swarm"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Registry manages a collection of scanners.
type Registry struct {
	scanners []Scanner
}

// NewRegistry creates a new scanner registry.
func NewRegistry() *Registry {
	return &Registry{
		scanners: []Scanner{},
	}
}

// Register adds a scanner to the registry.
func (r *Registry) Register(s Scanner) {
	r.scanners = append(r.scanners, s)
}

// RunAll executes all registered scanners using the provided swarm engine.
func (r *Registry) RunAll(ctx context.Context, g *graph.Graph, pool *swarm.Engine, wg *sync.WaitGroup, region, profile string) {
	for _, s := range r.scanners {
		// Capture closure variable
		scanner := s
		wg.Add(1)
		pool.Submit(func(ctx context.Context) error {
			defer wg.Done()
			return runWithTelemetry(ctx, scanner, g, region, profile)
		})
	}
}

func runWithTelemetry(ctx context.Context, s Scanner, g *graph.Graph, region, profile string) error {
	taskName := s.Name()
	tr := otel.Tracer("cloudslash/scanner")
	ctx, span := tr.Start(ctx, taskName, trace.WithAttributes(
		attribute.String("provider", "aws"),
		attribute.String("region", region),
		attribute.String("aws.profile", profile),
	))
	defer span.End()

	slog.Debug("Starting Scanner", "name", taskName)
	err := s.Scan(ctx, g)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		// Capture partial failure
		scope := fmt.Sprintf("%s:%s [%s]", profile, region, taskName)
		g.AddError(scope, err)
		slog.Error("Scanner encountered error", "name", taskName, "error", err)
	} else {
		slog.Debug("Scanner completed", "name", taskName)
	}
	return err
}
