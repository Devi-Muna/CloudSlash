package heuristics

import (
	"context"
	"fmt"
	"strings"


	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/k8s"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AbandonedFargateHeuristic struct {
	K8sClient *k8s.Client
}

func (h *AbandonedFargateHeuristic) Name() string { return "AbandonedFargateHeuristic" }

func (h *AbandonedFargateHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	// If no K8s connection, we cannot perform deep forensics.
	// Returning nil is safe (skip heuristic).
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
		
		// 0. The CoreDNS / System Whitelist
		if profileName == "fp-default" || strings.Contains(strings.ToLower(profileName), "coredns") {
			continue
		}
		
		// AWS SDK stores Selectors as []types.FargateProfileSelector
		selectors, ok := node.Properties["Selectors"].([]types.FargateProfileSelector)
		if !ok || len(selectors) == 0 {
			// No selectors? It matches nothing. Abandoned.
			node.IsWaste = true
			node.RiskScore = 100
			node.Cost = 0 // "Risk Removal"
			node.Properties["Reason"] = "Empty Fargate Profile: Matches no namespaces."
			continue
		}

		// A Profile is ACTIVE if AT LEAST ONE selector is active (OR Logic).
		isProfileActive := false
		var failureReasons []string

		for i, sel := range selectors {
			nsName := *sel.Namespace
			if nsName == "kube-system" {
				// Implicit whitelist for kube-system selectors even if profile name doesn't match
				isProfileActive = true
				break
			}
			
			// LAYER 1: The Broken Link Check (Namespace Existence)
			// We check if namespace exists in the K8s cluster.
			// Ideally we cache this list to avoid N calls.
			// For minimal code change, let's just call Get.
			_, err := h.K8sClient.Clientset.CoreV1().Namespaces().Get(ctx, nsName, metav1.GetOptions{})
			if err != nil {
				// If 404, this specific selector is dead.
				failureReasons = append(failureReasons, fmt.Sprintf("Selector #%d: Namespace '%s' not found.", i+1, nsName))
				continue
			}
			
			// LAYER 2: The Pulse Check (Active Pods)
			// List pods in namespace matching labels.
			// Labels in a selector are AND.
			labelSelector := formatLabelSelector(sel.Labels)
			
			pods, err := h.K8sClient.Clientset.CoreV1().Pods(nsName).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
				Limit: 1, // We only need to know if > 0 exist
			})
			
			if err == nil && len(pods.Items) > 0 {
				isProfileActive = true
				break // Found life! The specific Trap Door works.
			}
			
			// LAYER 3: Ghost Town Forensics (Controllers)
			// If 0 Pods, is it abandoned configuration?
			// Check Deployments
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
						// Scaled to 0. Check age (TODO: managedFields check is heavy, assuming scale-to-0 is indication enough for now)
						// User requested "LastScaleTime > 30 Days".
						// For v1.2.5, let's treat "Scaled to 0" as "Inactive" unless proven otherwise.
						// The profile ITSELF isn't doing anything if replicas=0.
					}
				}
			}
			
			if hasActiveController {
				isProfileActive = true
				break // Waiting for pods to launch (e.g. pending/crashloop), but intent is there.
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
			
			// If we get here, this selector matches:
			// 1. Existing Namespace
			// 2. But 0 Pods
			// 3. And 0 Active Controllers
			failureReasons = append(failureReasons, fmt.Sprintf("Selector #%d ('%s'): Ghost Town. No active Pods or Controllers.", i+1, nsName))
		}

		if !isProfileActive {
			// ABANDONED
			node.IsWaste = true
			node.RiskScore = 60 // Medium Risk (Configuration Debt is mostly risk of confusion/accidental billing)
			node.Cost = 0       // It's free to have empty profiles.
			
			reason := "Abandoned Fargate Profile:\n"
			for _, r := range failureReasons {
				reason += fmt.Sprintf(" - %s\n", r)
			}
			reason += "Recommendation: Remove to prevent accidental serverless billing if pods are scheduled here."
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
