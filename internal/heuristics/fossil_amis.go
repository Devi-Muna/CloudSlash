package heuristics

import (
	"context"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

type FossilAMIHeuristic struct{}

func (h *FossilAMIHeuristic) Name() string {
	return "FossilAMIs"
}

func (h *FossilAMIHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	// 1. Collect all Active AMIs
	activeAMIs := make(map[string]bool)
	for _, node := range g.Nodes {
		if node.Type == "AWS::EC2::AMI" {
			activeAMIs[node.ID] = true
		}
	}

	// 2. Scan Snapshots
	for id, node := range g.Nodes {
		if node.Type != "AWS::EC2::Snapshot" {
			continue
		}

		desc, _ := node.Properties["Description"].(string)

		// "Created by CreateImage(...) for ami-12345678"
		// If description says it was created for an AMI, but that AMI is not in our graph...
		// It means the AMI is deregistered (or we failed to scan it, but assuming full scan).

		if strings.Contains(desc, "Created by CreateImage") {
			// Extract AMI ID if possible, or just rely on the fact it's an AMI-snapshot
			// If it's not linked to any existing AMI node in the graph via EdgeTypeContains, it's a candidate.
			// CloudSlash graph edge logic: AMI -> Snapshot (Contains).
			// So we check ReverseEdges of the Snapshot. If no upstream AMI, it's orphaned.

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
				node.Properties["Reason"] = "Fossil Snapshot: Created by an AMI which no longer exists."

				// Estimate Cost ($0.05/GB standard-ish)
				if size, ok := node.Properties["VolumeSize"].(int32); ok {
					node.Cost = float64(size) * 0.05
				}
			}
		}
	}

	return nil
}
