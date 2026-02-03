package heuristics

import (
	"context"
	"strings"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

type FossilAMIHeuristic struct{}

func (h *FossilAMIHeuristic) Name() string {
	return "FossilAMIs"
}

func (h *FossilAMIHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	// Collect active AMIs.
	activeAMIs := make(map[string]bool)
	for _, node := range g.Store.GetAllNodes() {
		if node.TypeStr() == "AWS::EC2::AMI" {
			activeAMIs[node.IDStr()] = true
		}
	}

	// Scan snapshots.
	for _, node := range g.Store.GetAllNodes() {
		if node.TypeStr() != "AWS::EC2::Snapshot" {
			continue
		}

		desc, _ := node.Properties["Description"].(string)

		// Match description.
		if strings.Contains(desc, "Created by CreateImage") {
			// Check upstream.

			up := g.GetReverseEdges(node.Index)
			hasAMI := false
			for _, edge := range up {
				targetNode := g.GetNodeByID(edge.TargetID)
				if targetNode != nil && (strings.Contains(targetNode.IDStr(), ":image/") || strings.Contains(targetNode.IDStr(), ":ami/")) {
					hasAMI = true
					break
				}
			}

			if !hasAMI {
				node.IsWaste = true
				node.RiskScore = 60
				node.Properties["Reason"] = "Orphaned Snapshot: Created by an AMI which no longer exists."
				stats.ItemsFound++

				// Est. cost.
				if size, ok := node.Properties["VolumeSize"].(int32); ok {
					node.Cost = float64(size) * 0.05
					stats.ProjectedSavings += node.Cost
				}
			}
		}
	}

	return stats, nil
}
