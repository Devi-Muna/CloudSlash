package graph

import (
	"fmt"
)

// TopologicalSort resolves dependency order.
// Returns nodes sorted from independent leaves to dependent roots.
func (g *Graph) TopologicalSort(nodes []*Node) ([]*Node, error) {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	visited := make(map[uint32]bool)
	tempMark := make(map[uint32]bool)
	var sorted []*Node
	var cycleError error

	// Sort subset based on dependencies.
	// Build node set.
	subsetMap := make(map[uint32]bool)
	for _, n := range nodes {
		subsetMap[n.ID] = true
	}

	var visit func(n *Node)
	visit = func(n *Node) {
		if tempMark[n.ID] {
			cycleError = fmt.Errorf("cycle detected involving %s", n.IDStr())
			return
		}
		if visited[n.ID] {
			return
		}

		tempMark[n.ID] = true

		// Retrieve edges.
		edges := g.Store.GetEdges(n.Index)
		for _, edge := range edges {
			// Traverse dependencies.
			// Check if target is in subset.
			// TargetID is uint32
			targetNode := g.Store.GetNode(edge.TargetID)
			if targetNode != nil {
				if subsetMap[targetNode.ID] {
					visit(targetNode)
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

	// Reverse to safe deletion order.
	for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
		sorted[i], sorted[j] = sorted[j], sorted[i]
	}

	return sorted, nil
}
