package heuristics

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

type LambdaHeuristic struct{}

func (h *LambdaHeuristic) Name() string {
	return "LambdaForensics"
}

func (h *LambdaHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	return h.Analyze(g), nil
}

func (h *LambdaHeuristic) Analyze(g *graph.Graph) *HeuristicStats {
	stats := &HeuristicStats{}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.GetNodes() {
		if node.TypeStr() == "aws_lambda_function" {
			if h.analyzeFunction(node) {
				stats.ItemsFound++
				// Lambda cost is harder to project
			}
		}
	}
	return stats
}

func (h *LambdaHeuristic) analyzeFunction(node *graph.Node) bool {
	// Check inactivity.
	invocations := getFloat(node, "SumInvocations90d")
	lastModStr, _ := node.Properties["LastModified"].(string)

	// Parse time.
	lastMod, err := time.Parse("2006-01-02T15:04:05.000+0000", lastModStr)
	if err != nil {
		// Fallback RFC3339.
		lastMod, _ = time.Parse(time.RFC3339, lastModStr)
	}

	isOld := time.Since(lastMod) > 90*24*time.Hour

	if isOld && invocations == 0 {
		node.IsWaste = true
		node.RiskScore = 8
		node.Justification = "Inactive Function: 0 Invocations in 90d. Last Modified > 90d."
	}

	// Identify stale versions.
	versions, _ := node.Properties["AllVersions"].([]string)
	aliases, _ := node.Properties["AliasVersions"].(map[string]bool)

	// Estimate prune.
	pruneCount := 0

	for _, v := range versions {
		if aliases[v] {
			continue
		}
		// Estimate prune count.
		pruneCount++
	}
	if pruneCount > 5 {
		node.IsWaste = true
		wasteGB := (float64(pruneCount) * getFloat(node, "CodeSize")) / (1024 * 1024 * 1024)
		if wasteGB > 0.1 {
			node.Justification += fmt.Sprintf(" Excess storage: %d old versions (%.2f GB) can be pruned.", pruneCount, wasteGB)
		}
		return true // Waste found.
	}
	return false
}
