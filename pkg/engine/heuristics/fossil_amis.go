package heuristics

import (
	"context"
	"strings"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

type FossilAMIHeuristic struct{}

func (h *FossilAMIHeuristic) Name() string {
	return "FossilAMIs"
}

func (h *FossilAMIHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	// Collect active AMIs.
	activeAMIs := make(map[string]bool)
	for _, node := range g.Nodes {
		if node.Type == "AWS::EC2::AMI" {
			activeAMIs[node.ID] = true
		}
	}

	// Scan snapshots for orphaned references.
	for id, node := range g.Nodes {
		if node.Type != "AWS::EC2::Snapshot" {
			continue
		}

		desc, _ := node.Properties["Description"].(string)

		// Matches standard creation description.
		if strings.Contains(desc, "Created by CreateImage") {
			// Check upstream lineage.
			// Verify graph topology.

			upstream := g.ReverseEdges[id]
			hasAMI := false
			for _, edge := range upstream {
				targetNode := g.GetNodeByID(edge.TargetID)
				if targetNode != nil && (strings.Contains(targetNode.ID, ":image/") || strings.Contains(targetNode.ID, ":ami/")) {
					hasAMI = true
					break
				}
			}

			if !hasAMI {
				node.IsWaste = true
				node.RiskScore = 60
				node.Properties["Reason"] = "Orphaned Snapshot: Created by an AMI which no longer exists."

				// Estimated snapshot cost.
				if size, ok := node.Properties["VolumeSize"].(int32); ok {
					node.Cost = float64(size) * 0.05
				}
			}
		}
	}

	return nil
}
