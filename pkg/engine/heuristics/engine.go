package heuristics

import (
	"context"
	"fmt"
	"sync"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

// WasteConfidence represents the certainty of a finding.
type WasteConfidence int

// HeuristicResult captures the outcome of an analysis.
type HeuristicResult struct {
	IsWaste    bool
	Confidence WasteConfidence
	RiskScore  int
	Reason     string
}

// WeightedHeuristic defines the interface for analysis modules.
type WeightedHeuristic interface {
	Name() string
	Run(ctx context.Context, g *graph.Graph) error
}

// Engine manages and executes heuristics.
type Engine struct {
	heuristics []WeightedHeuristic
}

// NewEngine initializes the heuristic engine.
func NewEngine() *Engine {
	return &Engine{
		heuristics: []WeightedHeuristic{},
	}
}

// Register adds a heuristic.
func (e *Engine) Register(h WeightedHeuristic) {
	e.heuristics = append(e.heuristics, h)
}

// Run executes all registered heuristics concurrently.
func (e *Engine) Run(ctx context.Context, g *graph.Graph) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(e.heuristics))

	for _, h := range e.heuristics {
		wg.Add(1)
		go func(h WeightedHeuristic) {
			defer wg.Done()
			if err := h.Run(ctx, g); err != nil {
				errs <- fmt.Errorf("%s failed: %w", h.Name(), err)
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
