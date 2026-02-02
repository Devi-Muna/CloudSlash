package tf

import (
	"fmt"
	"strings"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

// DriftDetector finds unmanaged resources.
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

// ScanForDrift marks waste.
func (d *DriftDetector) ScanForDrift() {
	managedIDs := d.State.GetManagedResourceIDs()
	d.Graph.Mu.Lock()
	defer d.Graph.Mu.Unlock()

	for _, node := range d.Graph.GetNodes() {
		// Skip checked.
		if node.IsWaste {
			continue
		}

		// Check management.
		id := node.IDStr()

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
			// Identify as Shadow IT.
			node.IsWaste = true
			node.RiskScore = 100
			if node.Properties == nil {
				node.Properties = make(map[string]interface{})
			}
			node.Properties["Reason"] = "Shadow IT: Not found in Terraform State"
			fmt.Printf("Drift Detected: %s (%s)\n", id, node.TypeStr())
		}
	}
}
