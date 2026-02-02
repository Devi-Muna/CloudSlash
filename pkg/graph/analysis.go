package graph

// ImpactReport details removal impact.
type ImpactReport struct {
	TargetNode      *Node
	DirectImpact    []*Node // Nodes directly depending on this
	CascadingImpact []*Node // Nodes reachable through this node.
	TotalRiskScore  int
}

// AnalyzeImpact quantifies breakage risk.
func (g *Graph) AnalyzeImpact(nodeID string) *ImpactReport {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	// Lookup node index.
	// Lookup node index.
	targetIdx, ok := g.Store.GetNodeID(nodeID)
	if !ok {
		return nil
	}
	targetNode := g.Store.GetNode(targetIdx)

	report := &ImpactReport{
		TargetNode: targetNode,
	}

	// Identify direct dependencies.
	directEdges := g.Store.GetEdges(targetIdx) // Targets
	for _, edge := range directEdges {
		node := g.Store.GetNode(edge.TargetID)
		if node != nil {
			report.DirectImpact = append(report.DirectImpact, node)
			report.TotalRiskScore += node.RiskScore
		}
	}

	// Calculate cascading impact via BFS.
	visited := make(map[uint32]bool)
	queue := []uint32{}

	// Initialize queue.
	for _, child := range report.DirectImpact {
		visited[child.Index] = true
		queue = append(queue, child.Index)
	}

	// Mark processed.
	visited[targetIdx] = true

	for len(queue) > 0 {
		currentIdx := queue[0]
		queue = queue[1:]

		// Add to cascading.

		children := g.Store.GetEdges(currentIdx)
		for _, childEdge := range children {
			if !visited[childEdge.TargetID] {
				visited[childEdge.TargetID] = true
				queue = append(queue, childEdge.TargetID)

				node := g.Store.GetNode(childEdge.TargetID)
				if node != nil {
					report.CascadingImpact = append(report.CascadingImpact, node)
				}
			}
		}
	}

	return report
}
