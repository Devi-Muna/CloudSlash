package heuristics

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/v2/pkg/config"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

// GhostNodeGroupHeuristic checks node groups.
type GhostNodeGroupHeuristic struct{}

func (h *GhostNodeGroupHeuristic) Name() string { return "GhostNodeGroupHeuristic" }

func (h *GhostNodeGroupHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Store.GetAllNodes() {
		if node.TypeStr() != "AWS::EKS::NodeGroup" {
			continue
		}

		realWorkloadCount, ok := node.Properties["RealWorkloadCount"].(int)
		if !ok {
			// Skip missing metrics.
			continue
		}

		nodeCount, _ := node.Properties["NodeCount"].(int)

		// Check utilization.
		if realWorkloadCount == 0 && nodeCount > 0 {
			node.IsWaste = true
			node.RiskScore = 95 // High confidence.
			stats.ItemsFound++

			// Est. cost.
			instanceType := "m5.large" // Default instance type.
			if it, ok := node.Properties["InstanceType"].(string); ok && it != "" {
				instanceType = it
			}
			region := config.DefaultRegion
			if r, ok := node.Properties["Region"].(string); ok && r != "" {
				region = r
			}

			estimator := &aws.StaticCostEstimator{}
			estCostPerNode := estimator.GetEstimatedCost(instanceType, region)

			node.Cost = estCostPerNode * float64(nodeCount)
			stats.ProjectedSavings += node.Cost
			node.Properties["Reason"] = fmt.Sprintf("Idle Node Group: %d nodes of type %s running with 0 workloads.", nodeCount, instanceType)
		}
	}

	return stats, nil
}
