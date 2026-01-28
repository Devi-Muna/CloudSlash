package heuristics

import (
	"context"
	"fmt"
	"strings"

	"github.com/DrSkyle/cloudslash/pkg/graph"
	"github.com/DrSkyle/cloudslash/pkg/providers/k8s"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AbandonedFargateHeuristic struct {
	K8sClient *k8s.Client
}

func (h *AbandonedFargateHeuristic) Name() string { return "AbandonedFargateHeuristic" }

func (h *AbandonedFargateHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	// Skip heuristic if K8s connection is unavailable.
	if h.K8sClient == nil {
		return nil
	}

	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.Type != "AWS::EKS::FargateProfile" {
			continue
		}

		profileName, _ := node.Properties["ProfileName"].(string)

		// Filter: Exclude default profiles and CoreDNS.
		if profileName == "fp-default" || strings.Contains(strings.ToLower(profileName), "coredns") {
			continue
		}

		// AWS SDK stores Selectors as []types.FargateProfileSelector
		selectors, ok := node.Properties["Selectors"].([]types.FargateProfileSelector)
		if !ok || len(selectors) == 0 {
			// No selectors? It matches nothing. Abandoned.
			node.IsWaste = true
			node.RiskScore = 100
			node.Cost = 0
			node.Properties["Reason"] = "Empty Fargate Profile: No selectors defined."
			continue
		}

		// A Profile is ACTIVE if AT LEAST ONE selector is active (OR Logic).
		isProfileActive := false
		var failureReasons []string

		for i, sel := range selectors {
			nsName := *sel.Namespace
			if nsName == "kube-system" {
				// Whitelist kube-system selectors.
				isProfileActive = true
				break
			}

			// Check if the target namespace exists.
			
			// Define label selector (from profile selectors)
			labelSelector := formatLabelSelector(sel.Labels)

			// Check for active Pods.
			pods, err := h.K8sClient.Clientset.CoreV1().Pods(nsName).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})

			if err == nil && len(pods.Items) > 0 {
				isProfileActive = true
				break // Active pods found.
			}

			// Check for active Workload Controllers (Deployments/StatefulSets).
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
						// Scaled to 0. Treat as inactive.
					}
				}
			}

			if hasActiveController {
				isProfileActive = true
				break // Indicates active intent.
			}

			// Check StatefulSets
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

			// Selector matches a namespace, but no Pods or active Controllers were found.
			failureReasons = append(failureReasons, fmt.Sprintf("Selector #%d ('%s'): No active Pods or Controllers found.", i+1, nsName))
		}

		if !isProfileActive {
			node.IsWaste = true
			node.RiskScore = 60
			node.Cost = 0 // No direct cost, but represents configuration risk.

			reason := "Abandoned Fargate Profile:\n"
			for _, r := range failureReasons {
				reason += fmt.Sprintf(" - %s\n", r)
			}
			reason += "Recommendation: Remove unused profile."
			node.Properties["Reason"] = reason
		}
	}

	return nil
}

// formatLabelSelector converts map[string]string to "key=value,key2=value2"
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
