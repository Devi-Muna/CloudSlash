package heuristics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// WasteConfidence level.
type WasteConfidence int

// HeuristicResult is the analysis outcome.
type HeuristicResult struct {
	IsWaste    bool
	Confidence WasteConfidence
	RiskScore  int
	Reason     string
}

// HeuristicStats captures ROI.
type HeuristicStats struct {
	ItemsFound       int     // Number of waste items identified
	ProjectedSavings float64 // Monthly savings in USD
}

// WeightedHeuristic interface.
type WeightedHeuristic interface {
	Name() string
	Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error)
}

// Engine runs heuristics.
type Engine struct {
	heuristics []WeightedHeuristic
}

// NewEngine initializes engine.
func NewEngine() *Engine {
	return &Engine{
		heuristics: []WeightedHeuristic{},
	}
}

// Register heuristic.
func (e *Engine) Register(h WeightedHeuristic) {
	e.heuristics = append(e.heuristics, h)
}

// Run executes heuristics.
func (e *Engine) Run(ctx context.Context, g *graph.Graph) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(e.heuristics))

	tracer := otel.Tracer("cloudslash/heuristics")

	for _, h := range e.heuristics {
		wg.Add(1)
		go func(h WeightedHeuristic) {
			defer wg.Done()

			start := time.Now()
			ctx, span := tracer.Start(ctx, "Heuristic."+h.Name())
			defer span.End()

			stats, err := h.Run(ctx, g)
			if err != nil {
				span.RecordError(err)
				errs <- fmt.Errorf("%s failed: %w", h.Name(), err)
			}

			// Simple metrics in span attributes
			duration := time.Since(start)
			span.SetAttributes(
				attribute.Int64("duration_ms", duration.Milliseconds()),
				attribute.String("heuristic", h.Name()),
			)

			if stats != nil {
				span.SetAttributes(
					attribute.Int("waste_items_found", stats.ItemsFound),
					attribute.Float64("projected_savings_usd", stats.ProjectedSavings),
				)
			}
		}(h)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		// Return first error.
		return err
	}

	return nil
}
