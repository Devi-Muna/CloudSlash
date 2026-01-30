package heuristics

import (
	"context"
	"strings"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

type LogHoardersHeuristic struct{}

func (h *LogHoardersHeuristic) Name() string {
	return "LogHoarders"
}

func (h *LogHoardersHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		// Respect ignore flags.
		if node.Ignored {
			continue
		}

		if node.Type != "AWS::Logs::LogGroup" {
			continue
		}

		id := node.ID
		// Whitelist system logs.
		if strings.Contains(id, "/aws/lambda/") {
			continue
		}
		// Parse retention policy.
		retention, _ := node.Properties["Retention"]

		isNeverExpire := false
		if rStr, ok := retention.(string); ok && rStr == "Never" {
			isNeverExpire = true
		}

		storedBytes, _ := node.Properties["StoredBytes"].(int64)
		storedGB := float64(storedBytes) / 1024 / 1024 / 1024

		incomingBytes, _ := node.Properties["IncomingBytes"].(float64)
		// -1: Unknown, 0: Inactive, >0: Active.

		// Case 1: Abandoned Group.
		if storedBytes > 0 && incomingBytes == 0 {
			node.IsWaste = true
			node.RiskScore = 10 // Low risk.
			node.Properties["Reason"] = "Zombie Log: 0 Incoming Bytes in 30 days."
			node.Cost = storedGB * 0.03 // $0.03/GB
			continue
		}

		// Case 2: Infinite Retention Hoarder.
		if isNeverExpire && storedGB > 1.0 {
			node.IsWaste = true
			node.RiskScore = 40 // Medium risk.
			node.Properties["Reason"] = "Log Hoarder: Infinite Retention (Active)"
			node.Cost = storedGB * 0.03
		}
	}

	return nil
}
