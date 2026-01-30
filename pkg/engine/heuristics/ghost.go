package heuristics

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/pkg/config"
	"github.com/DrSkyle/cloudslash/pkg/graph"
)

// GhostNodeGroupHeuristic identifies active node groups with no workloads.
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
			// Skip if metrics missing.
			continue
		}

		nodeCount, _ := node.Properties["NodeCount"].(int)

		// Check for zero utilization.
		if realWorkloadCount == 0 && nodeCount > 0 {
			node.IsWaste = true
			node.RiskScore = 95 // High confidence.

			// Dynamic Cost Estimation
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
			node.Properties["Reason"] = fmt.Sprintf("Idle Node Group: %d nodes of type %s running with 0 workloads.", nodeCount, instanceType)
		}
	}

	return nil
}
