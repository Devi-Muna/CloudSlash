package heuristics

import (
	"context"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

type ECRJanitorHeuristic struct{}

func (h *ECRJanitorHeuristic) Name() string {
	return "ECRJanitor"
}

func (h *ECRJanitorHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.Ignored {
			continue
		}

		if node.Type != "AWS::ECR::Repository" {
			continue
		}

		hasPolicy, _ := node.Properties["HasPolicy"].(bool)
		wasteBytes, _ := node.Properties["WasteBytes"].(int64)
		wasteGB := float64(wasteBytes) / 1024 / 1024 / 1024

		// Logic: if NO Policy AND Waste > 0
		if !hasPolicy && wasteBytes > 0 {
			node.IsWaste = true
			node.RiskScore = 20 // Low risk (Untagged + Unpulled)
			node.Properties["Reason"] = "No Lifecycle Policy & Untagged Images > 90d old."

			// Cost: $0.10/GB/month for ECR
			node.Cost = wasteGB * 0.10
		}
	}

	return nil
}
