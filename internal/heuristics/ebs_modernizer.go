package heuristics

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// EBSModernizerHeuristic identifies legacy gp2 volumes that should be upgraded to gp3.
type EBSModernizerHeuristic struct{}

func (h *EBSModernizerHeuristic) Name() string { return "EBSModernizer" }

func (h *EBSModernizerHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	h.Analyze(g)
	return nil
}

func (h *EBSModernizerHeuristic) Analyze(g *graph.Graph) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, n := range g.Nodes {
		if n.Type == "AWS::EC2::Volume" {
			h.analyzeVolume(n)
		}
	}
}

func (h *EBSModernizerHeuristic) analyzeVolume(n *graph.Node) {
	if typ, _ := n.Properties["VolumeType"].(string); typ != "gp2" {
		return
	}
	if mod, _ := n.Properties["IsModifying"].(bool); mod {
		return
	}

	// SDK typed int32, but graph property often unmarshals or is stored as int/float.
	raw := n.Properties["Size"]
	sz := 0

	// Handle type assertions for size, which involves multiple numeric types from the SDK.
	switch v := raw.(type) {
	case int32:
		sz = int(v)
	case int:
		sz = v
	case float64:
		sz = int(v)
	}

	if sz == 0 {
		return
	}

	// gp2: 3 IOPS/GB (min 100, max 3000 bursted)
	curIOPS := sz * 3
	if curIOPS < 100 {
		curIOPS = 100
	}

	// gp3 Baseline: 3000 IOPS flat
	boost := 0.0
	if curIOPS < 3000 {
		boost = 3000.0 / float64(curIOPS)
	}

	n.IsWaste = true
	// Assign a priority score (RiskScore) based on the impact of the optimization.
	n.RiskScore = 3
	n.Cost = float64(sz) * 0.02 // Savings ($0.10 -> $0.08)

	rsn := "EBS Modernizer: Legacy gp2 volume detected."
	if boost > 1.0 {
		rsn += fmt.Sprintf(" [PERFORMANCE] %.1fx Speed Boost (Switch to gp3).", boost)
	}
	rsn += fmt.Sprintf(" Save $%.2f/mo.", n.Cost)

	n.Properties["Reason"] = rsn
	// Suggest remediation action.
	n.Properties["FixRecommendation"] = "Run 'cloudslash cleanup' to generate remediation scripts."
	n.Properties["IsGP2"] = true
}
