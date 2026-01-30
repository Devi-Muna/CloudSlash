package heuristics

import (
	"context"
	"fmt"     // Added for Sprintf
	"strings" // Added for Split/Join
	"time"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

type IdleEKSClusterHeuristic struct{}

func (h *IdleEKSClusterHeuristic) Name() string { return "IdleEKSClusterHeuristic" }

func (h *IdleEKSClusterHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	// Index ELBs.
	type elbInfo struct {
		Arn  string
		Tags map[string]string
	}
	var elbs []elbInfo
	for _, node := range g.Nodes {
		if node.Type == "AWS::ElasticLoadBalancingV2::LoadBalancer" || node.Type == "AWS::ElasticLoadBalancing::LoadBalancer" {
			tags, _ := node.Properties["Tags"].(map[string]string)
			elbs = append(elbs, elbInfo{Arn: node.ID, Tags: tags})
		}
	}

	for _, node := range g.Nodes {
		if node.Type != "AWS::EKS::Cluster" {
			continue
		}

		// Check: Active status.
		status, _ := node.Properties["Status"].(string)
		if status != "ACTIVE" {
			continue
		}

		// Check: Cluster age (> 7 days).
		createdAt, ok := node.Properties["CreatedAt"].(time.Time)
		if !ok {
			continue
		}
		if time.Since(createdAt) < 7*24*time.Hour {
			continue
		}

		// Check: Karpenter enabled.
		karpenter, _ := node.Properties["KarpenterEnabled"].(bool)
		if karpenter {
			continue // Skip autoscaling clusters.
		}

		// Validate compute resources.
		hasManaged, _ := node.Properties["HasManagedNodes"].(bool)
		hasFargate, _ := node.Properties["HasFargate"].(bool)
		hasSelf, _ := node.Properties["HasSelfManagedNodes"].(bool)

		if !hasManaged && !hasFargate && !hasSelf {
			node.IsWaste = true
			node.RiskScore = 90    // High confidence.
			node.Cost = 0.10 * 730 // Cost estimate.

			reason := "Idle Control Plane: Active EKS cluster with zero compute nodes for > 7 days."

			// Identify orphaned ELBs.
			clusterName := ""
			// Parse cluster name.
			parts := strings.Split(node.ID, "/")
			if len(parts) > 0 {
				clusterName = parts[len(parts)-1]
			}

			if clusterName != "" {
				var orphanedELBs []string
				tagKey := fmt.Sprintf("kubernetes.io/cluster/%s", clusterName)

				for _, elb := range elbs {
					if _, ok := elb.Tags[tagKey]; ok {
						orphanedELBs = append(orphanedELBs, elb.Arn)
					}
				}

				if len(orphanedELBs) > 0 {
					reason += fmt.Sprintf("\nDeleting this cluster will leave %d Orphaned ELBs behind. Here is the CLI command to delete them too:\n", len(orphanedELBs))

					// Generate CLI cleanup commands.
					cmdLines := []string{}
					for _, arn := range orphanedELBs {
						cmdLines = append(cmdLines, fmt.Sprintf("aws elbv2 delete-load-balancer --load-balancer-arn %s", arn))
					}
					reason += strings.Join(cmdLines, "\n")
				}
			}

			node.Properties["Reason"] = reason
		}
	}

	return nil
}
