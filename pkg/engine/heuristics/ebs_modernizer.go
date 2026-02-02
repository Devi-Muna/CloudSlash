package heuristics

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

// EBSModernizerHeuristic identifies gp2 -> gp3.
type EBSModernizerHeuristic struct{}

func (h *EBSModernizerHeuristic) Name() string { return "EBSModernizer" }

func (h *EBSModernizerHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	return h.Analyze(g), nil
}

func (h *EBSModernizerHeuristic) Analyze(g *graph.Graph) *HeuristicStats {
	stats := &HeuristicStats{}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::EC2::Volume" {
			if h.analyzeVolume(node) {
				stats.ItemsFound++
				stats.ProjectedSavings += node.Cost
			}
		}
	}
	return stats
}

func (h *EBSModernizerHeuristic) analyzeVolume(n *graph.Node) bool {
	if typ, _ := n.Properties["VolumeType"].(string); typ != "gp2" {
		return false
	}
	if mod, _ := n.Properties["IsModifying"].(bool); mod {
		return false
	}

	// Normalize size.
	raw := n.Properties["Size"]
	sz := 0

	switch v := raw.(type) {
	case int32:
		sz = int(v)
	case int:
		sz = v
	case float64:
		sz = int(v)
	}

	if sz == 0 {
		return false
	}

	// gp2 model.
	curIOPS := sz * 3
	if curIOPS < 100 {
		curIOPS = 100
	}

	// gp3 baseline.
	boost := 0.0
	if curIOPS < 3000 {
		boost = 3000.0 / float64(curIOPS)
	}

	n.IsWaste = true
	// Score.
	n.RiskScore = 3
	n.Cost = float64(sz) * 0.02 // Savings ($0.10 -> $0.08)

	rsn := "EBS Modernizer: Legacy gp2 volume detected."
	if boost > 1.0 {
		rsn += fmt.Sprintf(" [PERFORMANCE] %.1fx Speed Boost (Switch to gp3).", boost)
	}
	rsn += fmt.Sprintf(" Save $%.2f/mo.", n.Cost)

	n.Properties["Reason"] = rsn
	// Remediation.
	n.Properties["FixRecommendation"] = "Run 'cloudslash cleanup' to generate remediation scripts."
	n.Properties["IsGP2"] = true
	return true
}
