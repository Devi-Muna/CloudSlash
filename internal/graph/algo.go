package graph

import (
	"fmt"
)

// TopologicalSort performs a topological sort on the graph logic.
// Returns nodes in dependency order (Leaf -> Root).
// This generates the "Reverse Topological Order" needed for deletion.
// (e.g. Delete Instance -> Delete SG -> Delete VPC)
func (g *Graph) TopologicalSort(nodes []*Node) ([]*Node, error) {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	visited := make(map[string]bool)
	tempMark := make(map[string]bool)
	var sorted []*Node
	var cycleError error

	// We only care about sorting the subset of nodes provided (the Waste nodes).
	// However, precise sorting depends on edges relative to EACH OTHER in this subset.
	// We ignore edges pointing to nodes NOT in the subset (since we aren't deleting them).

	// Build a quick lookup for the subset
	subsetMap := make(map[string]bool)
	for _, n := range nodes {
		subsetMap[n.ID] = true
	}

	var visit func(n *Node)
	visit = func(n *Node) {
		if tempMark[n.ID] {
			cycleError = fmt.Errorf("cycle detected involving %s", n.ID)
			return
		}
		if visited[n.ID] {
			return
		}

		tempMark[n.ID] = true


		if int(n.Index) < len(g.Edges) {
			edges := g.Edges[n.Index]
			for _, edge := range edges {
				// Only traverse if target is also in our deletion set
				// AND edge implies dependency (e.g., AttachedTo, Contains).

				// Check if target is in subset (by ID lookup)
				// TargetID is uint32
				if int(edge.TargetID) < len(g.Nodes) {
					targetNode := g.Nodes[edge.TargetID]
					if subsetMap[targetNode.ID] {
						visit(targetNode)
					}
				}
			}
		}

		visited[n.ID] = true
		tempMark[n.ID] = false
		sorted = append(sorted, n)
	}

	for _, n := range nodes {
		if !visited[n.ID] {
			visit(n)
			if cycleError != nil {
				return nil, cycleError
			}
		}
	}

	return sorted, nil
}
