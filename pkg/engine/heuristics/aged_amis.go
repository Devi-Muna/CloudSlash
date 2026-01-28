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
	// Phase 1: Identify candidates (Read-Only).
	// Gather candidates first to avoid holding a Lock while calling MarkWaste.
	g.Mu.RLock()
	var candidates []string

	cutoff := time.Now().AddDate(0, -3, 0)
	timeLayout := "2006-01-02T15:04:05.000Z"

	for id, node := range g.Nodes {
		if node.Type != "AWS::EC2::AMI" {
			continue
		}

		// Check Age
		creationTime, ok := node.Properties["CreateTime"].(time.Time)
		if !ok {
			// Try fallback to string if legacy scanners used
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

		// Check Active Usage via Reverse Edges
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

	// PASS 2: Mark Waste (Write)
	for _, arn := range candidates {
		// Use MarkWaste to respect 'cloudslash:ignore' tags
		// This handles internal locking and tag validation
		g.MarkWaste(arn, 40)

		// Phase 3: Enrich Metadata (Write)
		// If MarkWaste succeeded (wasn't ignored), we add details.
		// Re-acquire lock to safely modify node properties.
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
