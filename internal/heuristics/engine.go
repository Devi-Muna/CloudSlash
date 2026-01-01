package heuristics

import (
	"context"
	"fmt"
	"sync"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

type WasteConfidence float64

type HeuristicResult struct {
	IsWaste    bool
	Confidence WasteConfidence
	RiskScore  int
	Reason     string
}

type WeightedHeuristic interface {
	Name() string
	Run(ctx context.Context, g *graph.Graph) error
}

type Engine struct {
	heuristics []WeightedHeuristic
}

func NewEngine() *Engine {
	return &Engine{
		heuristics: []WeightedHeuristic{},
	}
}

func (e *Engine) Register(h WeightedHeuristic) {
	e.heuristics = append(e.heuristics, h)
}

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
		// Stop on first error for now
		return err
	}

	return nil
}
