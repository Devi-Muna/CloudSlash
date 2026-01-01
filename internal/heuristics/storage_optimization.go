package heuristics

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// StorageOptimizationHeuristic (v1.3.0)
type StorageOptimizationHeuristic struct{}

func (h *StorageOptimizationHeuristic) Name() string { return "StorageOptimization" }

func (h *StorageOptimizationHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	h.Analyze(g)
	return nil
}

func (h *StorageOptimizationHeuristic) Analyze(g *graph.Graph) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, n := range g.Nodes {
		switch n.Type {
		case "AWS::S3::MultipartUpload":
			h.analyzeMultipart(n)
		case "AWS::EC2::Volume":
			h.analyzeVolume(n)
		}
	}
}

func (h *StorageOptimizationHeuristic) analyzeMultipart(n *graph.Node) {
	init, _ := n.Properties["Initiated"].(time.Time)
	dayz := int(time.Since(init).Hours() / 24)

	if dayz > 7 {
		n.IsWaste = true
		n.RiskScore = 20
		n.Properties["Reason"] = fmt.Sprintf("Stalled Upload: Initiated %d days ago.", dayz)
		n.Properties["FixRecommendation"] = "Add AbortIncompleteMultipartUpload Lifecycle Rule (7 days)."
	}
}

func (h *StorageOptimizationHeuristic) analyzeVolume(n *graph.Node) {
	if typ, _ := n.Properties["VolumeType"].(string); typ != "gp2" {
		return
	}
	if mod, _ := n.Properties["IsModifying"].(bool); mod {
		return
	}

	// SDK typed int32, but graph property often unmarshals or is stored as int/float.
	raw := n.Properties["Size"]
	sz := 0

	// Handle SDK type ambiguity
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
	n.RiskScore = 1
	n.Cost = float64(sz) * 0.02 // Savings ($0.10 -> $0.08)

	rsn := "EBS Modernizer: Switch to gp3."
	if boost > 1.0 {
		rsn += fmt.Sprintf(" [PERFORMANCE] %.1fx Speed Boost (3000 IOPS vs %d IOPS).", boost, curIOPS)
	}
	rsn += fmt.Sprintf(" Save $%.2f/mo.", n.Cost)
	n.Properties["Reason"] = rsn
}
