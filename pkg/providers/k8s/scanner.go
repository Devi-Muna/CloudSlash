package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
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

func (s *Scanner) Name() string { return "K8sScanner" }

func (s *Scanner) Scan(ctx context.Context, g *graph.Graph) error {
	if s.Client == nil {
		return nil
	}

	// Initialize SharedInformerFactory.
	// Resync every 10m for consistency.
	factory := informers.NewSharedInformerFactory(s.Client.Clientset, 10*time.Minute)

	// Initialize Listers.
	nodeLister := factory.Core().V1().Nodes().Lister()
	podLister := factory.Core().V1().Pods().Lister()

	// Start Informers.
	factory.Start(ctx.Done())

	// Wait for cache sync.
	synced := factory.WaitForCacheSync(ctx.Done())
	for kind, ok := range synced {
		if !ok {
			return fmt.Errorf("failed to sync informer for %v", kind)
		}
	}

	// Query Local Cache.
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

	// Iterate nodes.
	for _, node := range nodes {
		// EKS specific label
		ngName, ok := node.Labels["eks.amazonaws.com/nodegroup"]
		if !ok {
			continue
		}

		if _, exists := nodeGroups[ngName]; !exists {
			nodeGroups[ngName] = &NodeGroupData{
				Name: ngName,
			}
		}
		nodeGroups[ngName].NodeNames = append(nodeGroups[ngName].NodeNames, node.Name)
	}

	// List all pods.
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

	// Process Groups.
	for ngName, ng := range nodeGroups {
		realWorkloadCount := 0
		totalNodeCount := len(ng.NodeNames)

		for _, nodeName := range ng.NodeNames {
			pods := podsByNode[nodeName]

			for _, pod := range pods {
				// Skip finished pods.
				if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
					continue
				}

				// Skip DaemonSets.
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

				// Skip static mirror pods.
				if _, isMirror := pod.Annotations["kubernetes.io/config.mirror"]; isMirror {
					continue
				}

				// Skip system namespace.
				if pod.Namespace == "kube-system" {
					continue
				}

				// Active workload.
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
