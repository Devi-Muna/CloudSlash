package tf

import (
	"fmt"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// DriftDetector identifies unmanaged resources.
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

// ScanForDrift marks unmanaged resources as waste.
func (d *DriftDetector) ScanForDrift() {
	managedIDs := d.State.GetManagedResourceIDs()
	d.Graph.Mu.Lock()
	defer d.Graph.Mu.Unlock()

	for _, node := range d.Graph.Nodes {
		// Skip already flagged nodes.
		if node.IsWaste {
			continue
		}

		// Check if managed by Terraform.
		//
		//
		id := node.ID

		isManaged := false

		// Check ID match.
		if managedIDs[id] {
			isManaged = true
		} else {
			// Check ARN suffix match.
			
			parts := strings.Split(id, "/")
			if len(parts) > 1 {
				resourceID := parts[len(parts)-1]
				if managedIDs[resourceID] {
					isManaged = true
				}
			}
		}

		if !isManaged {
			// Mark as shadow IT.
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
