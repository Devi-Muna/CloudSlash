package graph

// AnalyzeReachability marks internet-accessible resources.
// It marks nodes as ReachabilityReachable vs. ReachabilityDarkMatter (Isolated).
func AnalyzeReachability(g *Graph) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	// Identify Roots (Ingress Points).
	// Identify ingress points.
	var queue []uint32
	// Get copy of current nodes.
	allNodes := g.Store.GetAllNodes()
	for _, node := range allNodes {
		// Reset reachability.
		node.Reachability = ReachabilityDarkMatter

		if isRoot(node) {
			node.Reachability = ReachabilityReachable
			queue = append(queue, node.Index)
		}
	}

	// BFS traversal.
	visited := make(map[uint32]bool)
	for _, id := range queue {
		visited[id] = true
	}

	for len(queue) > 0 {
		currentIdx := queue[0]
		queue = queue[1:]

		// Get downstream neighbors.
		edges := g.Store.GetEdges(currentIdx)
		for _, edge := range edges {
			targetIdx := edge.TargetID
			if visited[targetIdx] {
				continue
			}

			// Lookup target.
			targetNode := g.Store.GetNode(targetIdx)
			if targetNode == nil {
				continue
			}

			// Source Node
			sourceNode := g.Store.GetNode(currentIdx)

			// Apply constraints.
			if canTraverse(sourceNode, targetNode, edge) {
				targetNode.Reachability = ReachabilityReachable
				visited[targetIdx] = true
				queue = append(queue, targetIdx)
			}
		}
	}

}

// isRoot checks for internet ingress.
func isRoot(n *Node) bool {
	// Check AWS ingress.
	if n.TypeStr() == "AWS::EC2::InternetGateway" {
		return true
	}
	if n.TypeStr() == "AWS::EC2::VPNGateway" {
		return true
	}
	return false
}

// canTraverse validates connectivity paths.
func canTraverse(source, target *Node, edge Edge) bool {
	// Check physical connectivity.
	// Edges of type "Contains" or "AttachedTo" imply physical connectivity.

	// Check security constraints.
	// For this version, we implement basic network segment validation.

	// Check network isolation.
	if val, ok := target.Properties["NetworkType"]; ok {
		if val == "Private" {
			// Prevent IGW to Private traversal.
			if source.TypeStr() == "AWS::EC2::InternetGateway" {
				return false
			}
		}
	}

	return true
}

// IsDarkMatter checks network isolation.
func IsDarkMatter(n *Node) bool {
	return n.Reachability == ReachabilityDarkMatter
}
