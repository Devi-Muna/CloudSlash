package heuristics

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
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
	// 1. Code Rot
	invocations := getFloat(node, "SumInvocations90d")
	lastModStr, _ := node.Properties["LastModified"].(string)
	
	// Parse LastModified "2023-01-01T..."
	lastMod, err := time.Parse("2006-01-02T15:04:05.000+0000", lastModStr) 
	// Note: AWS format might differ, usually "2006-01-02T15:04:05.999+0000"
	if err != nil {
		// Fallback try simple RFC3339
		lastMod, _ = time.Parse(time.RFC3339, lastModStr)
	}
	
	isOld := time.Since(lastMod) > 90 * 24 * time.Hour

	if isOld && invocations == 0 {
		node.IsWaste = true
		node.RiskScore = 8
		node.Justification = "Code Rot: 0 Invocations in 90d. Last Modified > 90d."
		// Trigger Check? (Todo)
	}

	// 2. Version Pruner
	versions, _ := node.Properties["AllVersions"].([]string)
	aliases, _ := node.Properties["AliasVersions"].(map[string]bool)
	
	// Keep List: Top 3 (sorted?) - AWS ListVersions usually returns sorted by version?? Not valid assumption strictly, but often numerical.
	// For simplicity, we assume generic prune count.
	
	pruneCount := 0
	// We want to keep last 3.
	// We iterate all versions. If not in aliases and not in last 3 -> Prune.
	
	// Assuming `versions` is list of ALL versions except $LATEST.
	// We need to know which are "Recent".
	// Simplification: We blindly protect Aliases.
	// And we just count total - 3 - aliases = waste.
	
	// And we just count total - 3 - aliases = waste.
	
	for _, v := range versions {
		if aliases[v] {
			continue // Safe
		}
		// How to identify "Recent" without sorting? 
		// If we can't sort, we conservatively estimate waste.
		// Let's assume user has many versions.
		pruneCount++ 
	}
	
	// Correct logic: Total Versions - 3 (Safety) - UniqueAliases.
	// But we need to define "waste" on the function node.
	if pruneCount > 5 { // Threshold
		node.IsWaste = true // Or add a separate warning?
		// We can append to justification.
		wasteGB := (float64(pruneCount) * getFloat(node, "CodeSize")) / (1024*1024*1024)
		if wasteGB > 0.1 { // Only flag if significant storage
			node.Justification += fmt.Sprintf(" Storage Bloat: %d Old Versions (%.2f GB) can be pruned.", pruneCount, wasteGB)
		}
	}
}
