package heuristics

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// StorageOptimizationHeuristic
type StorageOptimizationHeuristic struct{}

func (h *StorageOptimizationHeuristic) Name() string { return "StorageOptimization" }

func (h *StorageOptimizationHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	h.Analyze(g)
	return nil
}

func (h *StorageOptimizationHeuristic) Analyze(g *graph.Graph) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, n := range g.Nodes {
		switch n.Type {
		case "AWS::S3::MultipartUpload":
			h.analyzeMultipart(n)
		}
	}
}

func (h *StorageOptimizationHeuristic) analyzeMultipart(n *graph.Node) {
	init, _ := n.Properties["Initiated"].(time.Time)
	dayz := int(time.Since(init).Hours() / 24)

	if dayz > 7 {
		n.IsWaste = true
		n.RiskScore = 20
		n.Properties["Reason"] = fmt.Sprintf("Stalled Upload: Initiated %d days ago.", dayz)
		n.Properties["FixRecommendation"] = "Add AbortIncompleteMultipartUpload Lifecycle Rule (7 days)."
	}
}
