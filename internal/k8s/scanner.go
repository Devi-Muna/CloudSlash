package k8s

import (
    "context"
    "fmt"

    "github.com/DrSkyle/cloudslash/internal/graph"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

    // --- STEP 1: THE SOURCE (List Nodes & Group by EKS NodeGroup) ---
    nodes, err := s.Client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
    if err != nil {
        return fmt.Errorf("failed to list k8s nodes: %v", err)
    }

    type NodeGroupData struct {
        Name       string // eks.amazonaws.com/nodegroup
        NodeNames  []string
        Region     string
        AccountID  string
    }
    
    // Map NodeGroup Name -> Data
    nodeGroups := make(map[string]*NodeGroupData)

    for _, node := range nodes.Items {
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

    // Optimized STEP 2: List ALL Pods once
    allPods, err := s.Client.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
    if err != nil {
         return fmt.Errorf("failed to list all pods: %v", err)
    }
    
    // Build map: NodeName -> []Pod
    podsByNode := make(map[string][]corev1.Pod)
    for _, pod := range allPods.Items {
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
                // --- STEP 3: THE GREAT FILTER ---
                
                // 1. Zombie Pod Check
                if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
                    continue
                }

                // 2. Infra Check (DaemonSet)
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

                // 3. Mirror Check
                if _, isMirror := pod.Annotations["kubernetes.io/config.mirror"]; isMirror {
                    continue
                }

                // 4. Namespace Safety Net
                if pod.Namespace == "kube-system" {
                    continue
                }

                // IT IS SIGNAL
                realWorkloadCount++
            }
        }
        
        // --- ADD TO GRAPH ---
        id := fmt.Sprintf("arn:aws:eks:unknown:unknown:nodegroup/%s", ngName)
        
        props := map[string]interface{}{
            "NodeGroupName": ngName,
            "NodeCount": totalNodeCount,
            "RealWorkloadCount": realWorkloadCount,
            "ClusterName": "detected-via-k8s",
        }
        
        s.Graph.AddNode(id, "AWS::EKS::NodeGroup", props)
    }

    return nil
}
