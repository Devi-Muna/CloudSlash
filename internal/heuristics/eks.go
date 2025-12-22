package heuristics

import (
	"context"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

type ZombieEKSHeuristic struct{}

func (h *ZombieEKSHeuristic) Name() string { return "ZombieEKSHeuristic" }

func (h *ZombieEKSHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.Type != "AWS::EKS::Cluster" {
			continue
		}

		// 1. Status Check
		status, _ := node.Properties["Status"].(string)
		if status != "ACTIVE" {
			continue
		}

		// 2. Age Check (> 7 Days)
		createdAt, ok := node.Properties["CreatedAt"].(time.Time)
		if !ok {
			// If we can't determine age, skip to be safe? Or flag? Safe is better.
			continue
		}
		if time.Since(createdAt) < 7*24*time.Hour {
			continue
		}

		// 3. Karpenter Check (Safety)
		karpenter, _ := node.Properties["KarpenterEnabled"].(bool)
		if karpenter {
			continue // Might be sleeping
		}

		// 4. Compute Check (The Zombie Triad)
		hasManaged, _ := node.Properties["HasManagedNodes"].(bool)
		hasFargate, _ := node.Properties["HasFargate"].(bool)
		hasSelf, _ := node.Properties["HasSelfManagedNodes"].(bool)

		if !hasManaged && !hasFargate && !hasSelf {
			// ZOMBIE IDENTIFIED
			node.IsWaste = true
			node.RiskScore = 90 // High confidence, pure waste
			node.Cost = 0.10 * 730 // ~$73.00/month
			node.Properties["Reason"] = "Zombie Control Plane: Active EKS cluster with zero compute nodes for > 7 days."
		}
	}

	return nil
}
