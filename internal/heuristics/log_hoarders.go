package heuristics

import (
	"context"

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
		if node.Type != "AWS::Logs::LogGroup" {
			continue
		}

		retention, _ := node.Properties["Retention"].(string)
		
		// If retention IS SET (int32 or string != "Never"), we skip
		// Check explicit "Never" marker we set in scanner
		if retention != "Never" {
			continue
		}

		storedBytes, _ := node.Properties["StoredBytes"].(int64)
		storedGB := float64(storedBytes) / 1024 / 1024 / 1024

		// Threshold: > 1GB and No Retention
		if storedGB > 1.0 {
			node.IsWaste = true
			node.RiskScore = 40 // Lower risk, but definitely waste
			node.Properties["Reason"] = "Log Hoarder: >1GB stored with Infinite Retention"
			
			// Cost Estimate: $0.03/GB (Standard logs)
			node.Cost = storedGB * 0.03
		}
	}

	return nil
}
