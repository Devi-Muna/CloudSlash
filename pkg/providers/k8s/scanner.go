package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/graph"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
)

type Scanner struct {
	Client *Client
	Graph  *graph.Graph
}

func NewScanner(client *Client, g *graph.Graph) *Scanner {
	return &Scanner{
		Client: client,
		Graph:  g,
	}
}

func (s *Scanner) Scan(ctx context.Context) error {
	if s.Client == nil {
		return nil // Graceful skip if no client
	}

	// 1. Initialize SharedInformerFactory
	// "Best in the World" generic pattern: Local Cache + Watch
	// Resync every 10 minutes to ensure eventual consistency
	factory := informers.NewSharedInformerFactory(s.Client.Clientset, 10*time.Minute)

	// 2. Initialize Listers (binds Informers to the factory)
	nodeLister := factory.Core().V1().Nodes().Lister()
	podLister := factory.Core().V1().Pods().Lister()

	// 3. Start Informers & Wait for Cache Sync
	// This establishes the Watch connection without blocking the main thread initially
	factory.Start(ctx.Done())

	// Wait for the local cache to fully populate from the API server
	// This prevents "empty list" bugs on startup
	synced := factory.WaitForCacheSync(ctx.Done())
	for kind, ok := range synced {
		if !ok {
			return fmt.Errorf("failed to sync informer for %v", kind)
		}
	}

	// 4. Query Local Cache (0% Load on API Server)
	nodes, err := nodeLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list k8s nodes from cache: %v", err)
	}

	type NodeGroupData struct {
		Name      string // eks.amazonaws.com/nodegroup
		NodeNames []string
		Region    string
		AccountID string
	}

	// Map NodeGroup Name -> Data
	nodeGroups := make(map[string]*NodeGroupData)

	// Note: Lister returns []*corev1.Node (pointers)
	for _, node := range nodes {
		// EKS specific label
		ngName, ok := node.Labels["eks.amazonaws.com/nodegroup"]
		if !ok {
			continue // Not an EKS Node Group node
		}

		if _, exists := nodeGroups[ngName]; !exists {
			nodeGroups[ngName] = &NodeGroupData{
				Name: ngName,
			}
		}
		nodeGroups[ngName].NodeNames = append(nodeGroups[ngName].NodeNames, node.Name)
	}

	// List ALL Pods from local cache
	allPods, err := podLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list all pods from cache: %v", err)
	}

	// Build map: NodeName -> []Pod
	podsByNode := make(map[string][]*corev1.Pod)
	for _, pod := range allPods {
		if pod.Spec.NodeName != "" {
			podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], pod)
		}
	}

	// Process Groups
	for ngName, ng := range nodeGroups {
		realWorkloadCount := 0
		totalNodeCount := len(ng.NodeNames)

		for _, nodeName := range ng.NodeNames {
			pods := podsByNode[nodeName]

			for _, pod := range pods {
				// Zombie Pod Check.
				if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
					continue
				}

				// Infra Check (DaemonSet).
				isDaemonSet := false
				for _, ref := range pod.OwnerReferences {
					if ref.Kind == "DaemonSet" {
						isDaemonSet = true
						break
					}
				}
				if isDaemonSet {
					continue
				}

				// Mirror Check.
				if _, isMirror := pod.Annotations["kubernetes.io/config.mirror"]; isMirror {
					continue
				}

				// Namespace Safety Net.
				if pod.Namespace == "kube-system" {
					continue
				}

				// IT IS SIGNAL
				realWorkloadCount++
			}
		}

		// Add Node Group to Graph.
		id := fmt.Sprintf("arn:aws:eks:unknown:unknown:nodegroup/%s", ngName)

		props := map[string]interface{}{
			"NodeGroupName":     ngName,
			"NodeCount":         totalNodeCount,
			"RealWorkloadCount": realWorkloadCount,
			"ClusterName":       "detected-via-k8s",
		}

		s.Graph.AddNode(id, "AWS::EKS::NodeGroup", props)
	}

	return nil
}
