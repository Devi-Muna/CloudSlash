package heuristics

import (
	"context"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

type AgedAMIHeuristic struct{}

func (h *AgedAMIHeuristic) Name() string {
	return "AgedAMIs"
}

func (h *AgedAMIHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	// Identify candidates.
	// Gather candidates to avoid lock contention.
	g.Mu.RLock()
	var candidates []string

	cutoff := time.Now().AddDate(0, -3, 0)
	timeLayout := "2006-01-02T15:04:05.000Z"

	for id, node := range g.Nodes {
		if node.Type != "AWS::EC2::AMI" {
			continue
		}

		// Check creation age.
		creationTime, ok := node.Properties["CreateTime"].(time.Time)
		if !ok {
			// Fallback: Parse string date.
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
			continue // Less than 90 days old
		}

		// Check usages.
		isUsed := false
		if int(id) < len(g.ReverseEdges) {
			upstream := g.ReverseEdges[id]
			for _, edge := range upstream {
				if edge.Type == graph.EdgeTypeUses {
					isUsed = true
					break
				}
			}
		}

		if !isUsed {
			candidates = append(candidates, node.ID)
		}
	}
	g.Mu.RUnlock()

	// Mark candidates as waste.
	for _, arn := range candidates {
		// MarkWaste respects ignore tags.
		g.MarkWaste(arn, 40)

		// Enrich metadata.
		// Add details if not ignored.
		// Re-acquire lock.
		node := g.GetNode(arn)
		if node != nil {
			g.Mu.Lock()
			if node.IsWaste {
				node.Properties["Reason"] = "Aged Artifact: AMI is > 90 days old and has no active instances."
				node.Cost = 1.00 // Approx storage cost
			}
			g.Mu.Unlock()
		}
	}

	return nil
}
