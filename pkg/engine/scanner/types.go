package scanner

import (
	"context"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

// Scanner defines the interface for resource discovery modules.
type Scanner interface {
	Name() string
	// Scan performs the analysis and injects nodes into the graph.
	// It returns an error only for fatal failures; partials should be logged/added to graph metadata.
	Scan(ctx context.Context, g *graph.Graph) error
}
