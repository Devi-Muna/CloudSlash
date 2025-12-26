package heuristics

import (
	"context"
	"fmt" // Added for Sprintf
	"strings" // Added for Split/Join
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

type ZombieEKSHeuristic struct{}

func (h *ZombieEKSHeuristic) Name() string { return "ZombieEKSHeuristic" }

func (h *ZombieEKSHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	// 1. Find all ELBs first to avoid O(N*M) lookups inside the EKS loop
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

		// 1. Status Check
		status, _ := node.Properties["Status"].(string)
		if status != "ACTIVE" {
			continue
		}

		// 2. Age Check (> 7 Days)
		createdAt, ok := node.Properties["CreatedAt"].(time.Time)
		if !ok {
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
			
			reason := "Zombie Control Plane: Active EKS cluster with zero compute nodes for > 7 days."

			// 5. Orphaned ELB Check
			clusterName := ""
            // Extract cluster name from ARN: arn:aws:eks:region:account:cluster/ClusterName
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
					
                    // Construct CLI command
					// aws elbv2 delete-load-balancer --load-balancer-arn <ARN>
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
