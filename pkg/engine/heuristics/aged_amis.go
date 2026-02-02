package heuristics

import (
	"context"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

type AgedAMIHeuristic struct{}

func (h *AgedAMIHeuristic) Name() string {
	return "AgedAMIs"
}

func (h *AgedAMIHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	// Identify candidates.
	// Avoid lock contention.
	g.Mu.RLock()
	var candidates []string

	cutoff := time.Now().AddDate(0, -3, 0)
	timeLayout := "2006-01-02T15:04:05.000Z"

	for _, node := range g.GetNodes() {
		if node.TypeStr() != "AWS::EC2::AMI" {
			continue
		}

		// Check age.
		creationTime, ok := node.Properties["CreateTime"].(time.Time)
		if !ok {
			// Parse date.
			dateStr, ok := node.Properties["CreationDate"].(string)
			if !ok || dateStr == "" {
				continue
			}
			var err error
			creationTime, err = time.Parse(timeLayout, dateStr)
			if err != nil {
				continue
			}
		}

		if creationTime.After(cutoff) {
			continue
		}

		// Check usages.
		isUsed := false
		upstream := g.GetReverseEdges(node.Index)
		for _, edge := range upstream {
			if edge.Type == graph.EdgeTypeUses {
				isUsed = true
				break
			}
		}

		if !isUsed {
			candidates = append(candidates, node.IDStr())
		}
	}
	g.Mu.RUnlock()

	// Mark waste.
	for _, arn := range candidates {
		// Respect ignore tags.
		g.MarkWaste(arn, 40)

		// Enrich metadata.
		node := g.GetNode(arn)
		if node != nil {
			g.Mu.Lock()
			if node.IsWaste {
				node.Properties["Reason"] = "Aged Artifact: AMI is > 90 days old and has no active instances."
				node.Cost = 1.00 // Approx storage cost
				stats.ItemsFound++
				stats.ProjectedSavings += node.Cost
			}
			g.Mu.Unlock()
		}
	}

	return stats, nil
}
