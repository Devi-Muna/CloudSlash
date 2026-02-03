package heuristics

import (
	"context"
	"strings"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

type LogHoardersHeuristic struct{}

func (h *LogHoardersHeuristic) Name() string {
	return "LogHoarders"
}

func (h *LogHoardersHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Store.GetAllNodes() {
		// Respect ignore.
		if node.Ignored {
			continue
		}

		if node.TypeStr() != "AWS::Logs::LogGroup" {
			continue
		}

		id := node.IDStr()
		// Whitelist system.
		if strings.Contains(id, "/aws/lambda/") {
			continue
		}
		// Parse retention.
		retention, _ := node.Properties["Retention"]

		isNeverExpire := false
		if rStr, ok := retention.(string); ok && rStr == "Never" {
			isNeverExpire = true
		}

		storedBytes, _ := node.Properties["StoredBytes"].(int64)
		storedGB := float64(storedBytes) / 1024 / 1024 / 1024

		incomingBytes, _ := node.Properties["IncomingBytes"].(float64)
		// Check activity.

		// 1. Abandoned.
		if storedBytes > 0 && incomingBytes == 0 {
			node.IsWaste = true
			node.RiskScore = 10 // Low risk.
			node.Properties["Reason"] = "Zombie Log: 0 Incoming Bytes in 30 days."
			node.Cost = storedGB * 0.03 // $0.03/GB
			stats.ItemsFound++
			stats.ProjectedSavings += node.Cost
			continue
		}

		// 2. Infinite retention.
		if isNeverExpire && storedGB > 1.0 {
			node.IsWaste = true
			node.RiskScore = 40 // Medium risk.
			node.Properties["Reason"] = "Log Hoarder: Infinite Retention (Active)"
			node.Cost = storedGB * 0.03
			stats.ItemsFound++
			stats.ProjectedSavings += node.Cost
		}
	}

	return stats, nil
}
