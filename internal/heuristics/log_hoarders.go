package heuristics

import (
	"context"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

type LogHoardersHeuristic struct{}

func (h *LogHoardersHeuristic) Name() string {
	return "LogHoarders"
}

func (h *LogHoardersHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		// Safety Check: Whitelist Ignored Nodes (Interactive)
		if node.Ignored {
			continue
		}

		if node.Type != "AWS::Logs::LogGroup" {
			continue
		}

		id := node.ID
		// Safety Whitelist (Prefix)
		if strings.Contains(id, "/aws/lambda/") {
			continue
		}
		// Check Tags (Simulated, as we didn't always fetch tags for Logs in scanner yet,
		// but if we did/do, we check here. For now rely on prefix/interactive)

		retention, _ := node.Properties["Retention"]
		// Retention logic:
		// scanner sets "Retention" = int32 (days) OR "Never" (string)

		isNeverExpire := false
		if rStr, ok := retention.(string); ok && rStr == "Never" {
			isNeverExpire = true
		}

		storedBytes, _ := node.Properties["StoredBytes"].(int64)
		storedGB := float64(storedBytes) / 1024 / 1024 / 1024

		incomingBytes, _ := node.Properties["IncomingBytes"].(float64)
		// float64(-1) means Unknown (Metrics Disabled)
		// 0 means Dead.
		// >0 means Active.

		// LOGIC 1: ABANDONED (Dead for 30 days)
		// Condition: Stored > 0 AND Incoming == 0
		if storedBytes > 0 && incomingBytes == 0 {
			node.IsWaste = true
			node.RiskScore = 10 // Very Safe
			node.Properties["Reason"] = "Zombie Log: 0 Incoming Bytes in 30 days."
			node.Cost = storedGB * 0.03 // $0.03/GB
			continue
		}

		// LOGIC 2: HOARDER (Infinite Retention)
		// Condition: Retention == "Never" AND Stored > 1GB
		if isNeverExpire && storedGB > 1.0 {
			node.IsWaste = true
			node.RiskScore = 40 // Higher risk (Active but hoardng)
			node.Properties["Reason"] = "Log Hoarder: Infinite Retention (Active)"
			node.Cost = storedGB * 0.03
		}
	}

	return nil
}
