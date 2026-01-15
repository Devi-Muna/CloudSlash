package graph

// ImpactReport details what will be affected if a node is removed.
type ImpactReport struct {
	TargetNode      *Node
	DirectImpact    []*Node // Nodes directly depending on this
	CascadingImpact []*Node // Nodes reachable through this node.
	TotalRiskScore  int
}

// AnalyzeImpact performs a traversal to find everything that depends on the target node.
// Analyzes dependencies.
func (g *Graph) AnalyzeImpact(nodeID string) *ImpactReport {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	// Lookup node index.
	targetIdx, ok := g.idMap[nodeID]
	if !ok {
		return nil
	}
	targetNode := g.Nodes[targetIdx]

	report := &ImpactReport{
		TargetNode: targetNode,
	}

	// 1. Identify direct dependencies.
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

	// 2. Identify cascading impact.
	// Perform BFS analysis.
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

		// Add to cascading
		if int(currentIdx) < len(g.Nodes) {
			
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
