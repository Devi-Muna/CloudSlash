package heuristics

import (
	"context"
	"fmt"
	
	"github.com/DrSkyle/cloudslash/internal/graph"
)

// GhostNodeGroupHeuristic identifies EKS Node Groups that are active but serving 0 user workloads.
type GhostNodeGroupHeuristic struct{}

func (h *GhostNodeGroupHeuristic) Name() string { return "GhostNodeGroupHeuristic" }

func (h *GhostNodeGroupHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.Type != "AWS::EKS::NodeGroup" {
			continue
		}

		realWorkloadCount, ok := node.Properties["RealWorkloadCount"].(int)
		if !ok {
			// If property missing, scanner didn't run or failed. Skip.
			continue
		}
		
		nodeCount, _ := node.Properties["NodeCount"].(int)

		// THE VERDICT
		// If RealWorkloadCount == 0 for EVERY node in the Node Group (Group Level Aggregation was done in Scanner)
		// AND there are actual nodes billing (NodeCount > 0)
		if realWorkloadCount == 0 && nodeCount > 0 {
			// GHOST DETECTED
			node.IsWaste = true
			node.RiskScore = 95 // Extremely High Confidence
			
			// Cost Estimation: Assume generic m5.large (~$70/mo) * nodeCount as a baseline estimate
			// Optimally we'd look up instance type from ASG, but we have "NodeCount".
			estCostPerNode := 70.0
			node.Cost = estCostPerNode * float64(nodeCount)
			
			node.Properties["Reason"] = fmt.Sprintf("👻 GHOST DETECTED: Node Group has %d active nodes but serves EXACTLY ZERO user applications.", nodeCount)
		}
	}

	return nil
}
