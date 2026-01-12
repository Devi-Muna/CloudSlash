package tf

import (
	"fmt"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// DriftDetector identifies resources that exist in the graph but not in the Terraform state.
type DriftDetector struct {
	Graph *graph.Graph
	State *State
}

// NewDriftDetector creates a new detector.
func NewDriftDetector(g *graph.Graph, s *State) *DriftDetector {
	return &DriftDetector{
		Graph: g,
		State: s,
	}
}

// ScanForDrift iterates through the graph and flags resources not found in the state.
func (d *DriftDetector) ScanForDrift() {
	managedIDs := d.State.GetManagedResourceIDs()

	d.Graph.Mu.Lock() // Assuming we added Mu to Graph struct, or use existing mutex
	defer d.Graph.Mu.Unlock()

	for _, node := range d.Graph.Nodes {
		// Skip if node is already marked as waste (optimization)
		if node.IsWaste {
			continue
		}

		// Check if the ID or ARN exists in the managed set
		// Note: The graph ID is usually the ARN. The state might have ID or ARN.
		// We need robust matching.
		id := node.ID

		isManaged := false

		// Direct match
		if managedIDs[id] {
			isManaged = true
		} else {
			// Try to match by resource ID if the graph ID is an ARN
			// e.g. arn:aws:ec2:region:account:instance/i-12345 -> i-12345
			parts := strings.Split(id, "/")
			if len(parts) > 1 {
				resourceID := parts[len(parts)-1]
				if managedIDs[resourceID] {
					isManaged = true
				}
			}
		}

		if !isManaged {
			// SHADOW IT DETECTED
			node.IsWaste = true
			node.RiskScore = 100
			if node.Properties == nil {
				node.Properties = make(map[string]interface{})
			}
			node.Properties["Reason"] = "Shadow IT: Not found in Terraform State"
			fmt.Printf("Drift Detected: %s (%s)\n", id, node.Type)
		}
	}
}
