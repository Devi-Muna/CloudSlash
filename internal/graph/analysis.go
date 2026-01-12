package graph

// ImpactReport details what will be affected if a node is removed.
type ImpactReport struct {
	TargetNode      *Node
	DirectImpact    []*Node // Nodes directly depending on this
	CascadingImpact []*Node // Nodes strictly reachable only through this (if we did proper dominator analysis, for now just downstream)
	TotalRiskScore  int
}

// AnalyzeImpact performs a traversal to find everything that depends on the target node.
// It uses ReverseEdges (Who points to me?) to find dependencies.
func (g *Graph) AnalyzeImpact(nodeID string) *ImpactReport {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	// Find index via idMap
	targetIdx, ok := g.idMap[nodeID]
	if !ok {
		return nil
	}
	targetNode := g.Nodes[targetIdx]

	report := &ImpactReport{
		TargetNode: targetNode,
	}

	// 1. Direct Dependencies (Reverse Edges: Who needs me? Wait, AnalyzeImpact logic said g.Edges)
	// Original code used: directEdges := g.Edges[nodeID]

	// So "Downstream" (Forward Edges) are the things affected by this node's removal.
	if int(targetIdx) < len(g.Edges) {
		directEdges := g.Edges[targetIdx] // Targets
		for _, edge := range directEdges {
			if int(edge.TargetID) < len(g.Nodes) {
				node := g.Nodes[edge.TargetID]
				report.DirectImpact = append(report.DirectImpact, node)
				report.TotalRiskScore += node.RiskScore
			}
		}
	}

	// 2. Cascading Impact (Recursive downstream)
	// BFS on forward edges
	visited := make(map[uint32]bool)
	queue := []uint32{}

	// Seed queue with direct children indices
	for _, child := range report.DirectImpact {
		visited[child.Index] = true
		queue = append(queue, child.Index)
	}

	// Mark target as visited
	visited[targetIdx] = true

	for len(queue) > 0 {
		currentIdx := queue[0]
		queue = queue[1:]

		// Add to cascading
		if int(currentIdx) < len(g.Nodes) {
			// Already added during seed or loop
		}

		if int(currentIdx) < len(g.Edges) {
			children := g.Edges[currentIdx]
			for _, childEdge := range children {
				if !visited[childEdge.TargetID] {
					visited[childEdge.TargetID] = true
					queue = append(queue, childEdge.TargetID)
					if int(childEdge.TargetID) < len(g.Nodes) {
						node := g.Nodes[childEdge.TargetID]
						report.CascadingImpact = append(report.CascadingImpact, node)
					}
				}
			}
		}
	}

	return report
}
