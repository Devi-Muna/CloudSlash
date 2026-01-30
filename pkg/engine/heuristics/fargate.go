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
	// Require K8s connection.
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

		// Exclude default/system profiles.
		if profileName == "fp-default" || strings.Contains(strings.ToLower(profileName), "coredns") {
			continue
		}

		// Parse selectors.
		selectors, ok := node.Properties["Selectors"].([]types.FargateProfileSelector)
		if !ok || len(selectors) == 0 {
			// Empty profile (abandoned).
			node.IsWaste = true
			node.RiskScore = 100
			node.Cost = 0
			node.Properties["Reason"] = "Empty Fargate Profile: No selectors defined."
			continue
		}

		// Check profile activity.
		isProfileActive := false
		var failureReasons []string

		for i, sel := range selectors {
			nsName := *sel.Namespace
			if nsName == "kube-system" {
				// Whitelist kube-system.
				isProfileActive = true
				break
			}


			
			// Format label selector.
			labelSelector := formatLabelSelector(sel.Labels)

			// Check active Pods.
			pods, err := h.K8sClient.Clientset.CoreV1().Pods(nsName).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})

			if err == nil && len(pods.Items) > 0 {
				isProfileActive = true
				break // Active.
			}

			// Check active Workload Controllers.
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
						// Inactive (scaled to 0).
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

			// Selector matches namespace but no active workloads.
			failureReasons = append(failureReasons, fmt.Sprintf("Selector #%d ('%s'): No active Pods or Controllers found.", i+1, nsName))
		}

		if !isProfileActive {
			node.IsWaste = true
			node.RiskScore = 60
			node.Cost = 0 // Risk: Configuration debt.

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

// formatLabelSelector formats labels for K8s API.
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
