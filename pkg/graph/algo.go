package graph

import (
	"fmt"
)

// TopologicalSort performs a topological sort on the graph logic.
// Returns nodes in dependency order (Leaf -> Root).
// Generates reverse topological order for deletion.
func (g *Graph) TopologicalSort(nodes []*Node) ([]*Node, error) {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	visited := make(map[string]bool)
	tempMark := make(map[string]bool)
	var sorted []*Node
	var cycleError error

	// Sort subset of nodes based on internal dependencies.

	// Create lookup map.
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
				// Traverse dependencies within the target subset.

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
