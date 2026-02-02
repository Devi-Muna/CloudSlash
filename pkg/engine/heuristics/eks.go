package heuristics

import (
	"context"
	"fmt"     // Added for Sprintf
	"strings" // Added for Split/Join
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

// IdleEKSClusterHeuristic checks idle EKS.
type IdleEKSClusterHeuristic struct{}

// Name returns the name of the heuristic.
func (h *IdleEKSClusterHeuristic) Name() string { return "IdleEKSClusterHeuristic" }

// Run executes the heuristic analysis.
func (h *IdleEKSClusterHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	elbs := h.indexELBs(g)

	for _, node := range g.GetNodes() {
		if node.TypeStr() != "AWS::EKS::Cluster" {
			continue
		}
		if h.analyzeCluster(node, elbs, stats) {
			stats.ItemsFound++
		}
	}

	return stats, nil
}

type elbInfo struct {
	Arn  string
	Tags map[string]string
}

func (h *IdleEKSClusterHeuristic) indexELBs(g *graph.Graph) []elbInfo {
	var elbs []elbInfo
	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::ElasticLoadBalancingV2::LoadBalancer" || node.TypeStr() == "AWS::ElasticLoadBalancing::LoadBalancer" {
			tags, _ := node.Properties["Tags"].(map[string]string)
			elbs = append(elbs, elbInfo{Arn: node.IDStr(), Tags: tags})
		}
	}
	return elbs
}

func (h *IdleEKSClusterHeuristic) analyzeCluster(node *graph.Node, elbs []elbInfo, stats *HeuristicStats) bool {
	// Check status.
	status, _ := node.Properties["Status"].(string)
	if status != "ACTIVE" {
		return false
	}

	// Check age.
	createdAt, ok := node.Properties["CreatedAt"].(time.Time)
	if !ok || time.Since(createdAt) < 7*24*time.Hour {
		return false
	}

	// Check Karpenter.
	karpenter, _ := node.Properties["KarpenterEnabled"].(bool)
	if karpenter {
		return false
	}

	// Check compute.
	hasManaged, _ := node.Properties["HasManagedNodes"].(bool)
	hasFargate, _ := node.Properties["HasFargate"].(bool)
	hasSelf, _ := node.Properties["HasSelfManagedNodes"].(bool)

	if hasManaged || hasFargate || hasSelf {
		return false
	}

	// Mark waste.
	node.IsWaste = true
	node.RiskScore = 90    // High confidence.
	node.Cost = 0.10 * 730 // Est. cost.
	stats.ProjectedSavings += node.Cost

	reason := "Idle Control Plane: Active EKS cluster with zero compute nodes for > 7 days."

	// Check orphaned ELBs.
	h.checkOrphanedELBs(node, elbs, &reason)

	node.Properties["Reason"] = reason
	return true
}

func (h *IdleEKSClusterHeuristic) checkOrphanedELBs(node *graph.Node, elbs []elbInfo, reason *string) {
	clusterName := ""
	parts := strings.Split(node.IDStr(), "/")
	if len(parts) > 0 {
		clusterName = parts[len(parts)-1]
	}

	if clusterName == "" {
		return
	}

	var orphanedELBs []string
	tagKey := fmt.Sprintf("kubernetes.io/cluster/%s", clusterName)

	for _, elb := range elbs {
		if _, ok := elb.Tags[tagKey]; ok {
			orphanedELBs = append(orphanedELBs, elb.Arn)
		}
	}

	if len(orphanedELBs) > 0 {
		*reason += fmt.Sprintf("\nDeleting this cluster will leave %d Orphaned ELBs behind. Here is the CLI command to delete them too:\n", len(orphanedELBs))
		cmdLines := []string{}
		for _, arn := range orphanedELBs {
			cmdLines = append(cmdLines, fmt.Sprintf("aws elbv2 delete-load-balancer --load-balancer-arn %s", arn))
		}
		*reason += strings.Join(cmdLines, "\n")
	}
}
