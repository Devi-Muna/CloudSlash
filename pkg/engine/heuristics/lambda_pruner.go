package heuristics

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

type LambdaHeuristic struct{}

func (h *LambdaHeuristic) Name() string {
	return "LambdaForensics"
}

func (h *LambdaHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	h.Analyze(g)
	return nil
}

func (h *LambdaHeuristic) Analyze(g *graph.Graph) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.Type == "aws_lambda_function" {
			h.analyzeFunction(node)
		}
	}
}

func (h *LambdaHeuristic) analyzeFunction(node *graph.Node) {
	// Check for inactivity (90d).
	invocations := getFloat(node, "SumInvocations90d")
	lastModStr, _ := node.Properties["LastModified"].(string)
	
	// Parse modification time.
	lastMod, err := time.Parse("2006-01-02T15:04:05.000+0000", lastModStr) 
	// Handle timestamp formats.
	if err != nil {
		// Fallback to RFC3339.
		lastMod, _ = time.Parse(time.RFC3339, lastModStr)
	}
	
	isOld := time.Since(lastMod) > 90 * 24 * time.Hour

	if isOld && invocations == 0 {
		node.IsWaste = true
		node.RiskScore = 8
		node.Justification = "Inactive Function: 0 Invocations in 90d. Last Modified > 90d."
	}

	// Identify stale versions.
	versions, _ := node.Properties["AllVersions"].([]string)
	aliases, _ := node.Properties["AliasVersions"].(map[string]bool)
	
	// Note: Conservative pruning estimate.
	
	// Estimate prune count.
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
		wasteGB := (float64(pruneCount) * getFloat(node, "CodeSize")) / (1024*1024*1024)
		if wasteGB > 0.1 {
			node.Justification += fmt.Sprintf(" Excess storage: %d old versions (%.2f GB) can be pruned.", pruneCount, wasteGB)
		}
	}
}
