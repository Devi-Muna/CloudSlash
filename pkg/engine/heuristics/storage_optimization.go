package heuristics

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

// StorageOptimizationHeuristic checks storage.
type StorageOptimizationHeuristic struct{}

func (h *StorageOptimizationHeuristic) Name() string { return "StorageOptimization" }

func (h *StorageOptimizationHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	return h.Analyze(g), nil
}

func (h *StorageOptimizationHeuristic) Analyze(g *graph.Graph) *HeuristicStats {
	stats := &HeuristicStats{}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.GetNodes() {
		switch node.TypeStr() {
		case "AWS::S3::MultipartUpload":
			if h.analyzeMultipart(node) {
				stats.ItemsFound++
				// Minimal cost.
			}
		}
	}
	return stats
}

func (h *StorageOptimizationHeuristic) analyzeMultipart(n *graph.Node) bool {
	init, _ := n.Properties["Initiated"].(time.Time)
	dayz := int(time.Since(init).Hours() / 24)

	if dayz > 7 {
		n.IsWaste = true
		n.RiskScore = 20
		n.Properties["Reason"] = fmt.Sprintf("Incomplete Multipart Upload: Initiated %d days ago.", dayz)
		n.Properties["FixRecommendation"] = "Add AbortIncompleteMultipartUpload Lifecycle Rule (7 days)."
		return true
	}
	return false
}
