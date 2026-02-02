package heuristics

import (
	"context"
	"fmt"
	"strings"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/DrSkyle/cloudslash/v2/pkg/providers/k8s"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AbandonedFargateHeuristic struct {
	K8sClient *k8s.Client
}

func (h *AbandonedFargateHeuristic) Name() string { return "AbandonedFargateHeuristic" }

func (h *AbandonedFargateHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	// Require K8s.
	if h.K8sClient == nil {
		return nil, nil
	}
	stats := &HeuristicStats{}

	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.GetNodes() {
		if node.TypeStr() != "AWS::EKS::FargateProfile" {
			continue
		}

		profileName, _ := node.Properties["ProfileName"].(string)

		// Exclude system profiles.
		if profileName == "fp-default" || strings.Contains(strings.ToLower(profileName), "coredns") {
			continue
		}

		// Parse selectors.
		selectors, ok := node.Properties["Selectors"].([]types.FargateProfileSelector)
		if !ok || len(selectors) == 0 {
			// Empty profile.
			node.IsWaste = true
			node.RiskScore = 100
			node.Cost = 0
			node.Properties["Reason"] = "Empty Fargate Profile: No selectors defined."
			stats.ItemsFound++
			continue
		}

		// Check activity.
		isProfileActive := false
		var failureReasons []string

		for i, sel := range selectors {
			nsName := *sel.Namespace
			if nsName == "kube-system" {
				// Whitelist kube-system.
				isProfileActive = true
				break
			}

			// Format selector.
			labelSelector := formatLabelSelector(sel.Labels)

			// Check pods.
			pods, err := h.K8sClient.Clientset.CoreV1().Pods(nsName).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})

			if err == nil && len(pods.Items) > 0 {
				isProfileActive = true
				break // Active.
			}

			// Check controllers.
			deployments, err := h.K8sClient.Clientset.AppsV1().Deployments(nsName).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})

			hasActiveController := false
			if err == nil {
				for _, d := range deployments.Items {
					if d.Spec.Replicas != nil && *d.Spec.Replicas > 0 {
						hasActiveController = true
						break
					} else {
						// Inactive.
					}
				}
			}

			if hasActiveController {
				isProfileActive = true
				break // Active.
			}

			// Check StatefulSets.
			sts, err := h.K8sClient.Clientset.AppsV1().StatefulSets(nsName).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err == nil {
				for _, s := range sts.Items {
					if s.Spec.Replicas != nil && *s.Spec.Replicas > 0 {
						hasActiveController = true
						break
					}
				}
			}

			if hasActiveController {
				isProfileActive = true
				break
			}

			// No active workloads.
			failureReasons = append(failureReasons, fmt.Sprintf("Selector #%d ('%s'): No active Pods or Controllers found.", i+1, nsName))
		}

		if !isProfileActive {
			node.IsWaste = true
			node.RiskScore = 60
			node.Cost = 0 // Config debt.

			reason := "Abandoned Fargate Profile:\n"
			for _, r := range failureReasons {
				reason += fmt.Sprintf(" - %s\n", r)
			}
			reason += "Recommendation: Remove unused profile."
			node.Properties["Reason"] = reason
			stats.ItemsFound++
		}
	}

	return stats, nil
}

// formatLabelSelector formats labels.
func formatLabelSelector(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}
