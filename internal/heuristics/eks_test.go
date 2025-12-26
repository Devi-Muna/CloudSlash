package heuristics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

func TestZombieEKSHeuristic_WithOrphanedELBs(t *testing.T) {
	// 1. Setup Graph
	g := graph.NewGraph()
	ctx := context.Background()
	heuristic := &ZombieEKSHeuristic{}

	// Create Zombie EKS Cluster
	clusterArn := "arn:aws:eks:us-east-1:123456789012:cluster/ZombieCluster"
	g.AddNode(clusterArn, "AWS::EKS::Cluster", map[string]interface{}{
		"Status":                "ACTIVE",
		"CreatedAt":             time.Now().Add(-8 * 24 * time.Hour), // 8 days old
		"KarpenterEnabled":      false,
		"HasManagedNodes":       false,
		"HasFargate":            false,
		"HasSelfManagedNodes":   false,
	})

	// Create Orphaned ALB (Matching Tag)
	orphanedAlbArn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/orphaned-alb/50dc6c495c0c9188"
	g.AddNode(orphanedAlbArn, "AWS::ElasticLoadBalancingV2::LoadBalancer", map[string]interface{}{
		"Type": "application",
		"Tags": map[string]string{
			"kubernetes.io/cluster/ZombieCluster": "owned",
		},
	})

	// Create Normal ALB (No Matching Tag)
	normalAlbArn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/other-alb/123456789012"
	g.AddNode(normalAlbArn, "AWS::ElasticLoadBalancingV2::LoadBalancer", map[string]interface{}{
		"Type": "application",
		"Tags": map[string]string{
			"kubernetes.io/cluster/OtherCluster": "owned",
		},
	})

	// 2. Run Heuristic
	err := heuristic.Run(ctx, g)
	if err != nil {
		t.Fatalf("Heuristic run failed: %v", err)
	}

	// 3. Assertions
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	clusterNode, ok := g.Nodes[clusterArn]
	if !ok {
		t.Fatal("Cluster node not found")
	}

	if !clusterNode.IsWaste {
		t.Error("Expected cluster to be marked as waste (Zombie)")
	}

	reason, _ := clusterNode.Properties["Reason"].(string)
	
	// Check for Orphaned ELB Logic
	if !strings.Contains(reason, "Orphaned ELBs") {
		t.Errorf("Expected reason to contain 'Orphaned ELBs', got: %s", reason)
	}

	if !strings.Contains(reason, orphanedAlbArn) {
		t.Errorf("Expected reason to contain orphaned ELB ARN, got: %s", reason)
	}

	if strings.Contains(reason, normalAlbArn) {
		t.Errorf("Expected reason NOT to contain normal ELB ARN, got: %s", reason)
	}
}
