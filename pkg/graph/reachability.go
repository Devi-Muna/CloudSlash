package graph

// AnalyzeReachability performs the "Void Walker" analysis.
// It marks nodes as ReachabilityReachable or ReachabilityDarkMatter.
func AnalyzeReachability(g *Graph) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	// Identify Roots (Ingress Points).
	var queue []uint32
	for _, node := range g.Nodes {
		// Initialize all nodes as Dark Matter.
		node.Reachability = ReachabilityDarkMatter

		if isRoot(node) {
			node.Reachability = ReachabilityReachable
			queue = append(queue, node.Index)
		}
	}

	// BFS Flood Fill.
	visited := make(map[uint32]bool)
	for _, id := range queue {
		visited[id] = true
	}

	for len(queue) > 0 {
		currentIdx := queue[0]
		queue = queue[1:]

		// Get Neighbors (Downstream)
		// We look at outgoing edges
		if int(currentIdx) < len(g.Edges) {
			edges := g.Edges[currentIdx]
			for _, edge := range edges {
				targetIdx := edge.TargetID
				if visited[targetIdx] {
					continue
				}

				if int(targetIdx) >= len(g.Nodes) {
					continue
				}
				targetNode := g.Nodes[targetIdx]

				// Apply Constraint Check.
				if canTraverse(g.Nodes[currentIdx], targetNode, edge) {
					targetNode.Reachability = ReachabilityReachable
					visited[targetIdx] = true
					queue = append(queue, targetIdx)
				}
			}
		}
	}

	// Reporting phase (optional Debug)
	// countDark := 0
	// for _, n := range g.Nodes {
	// 	if n.Reachability == ReachabilityDarkMatter {
	// 		countDark++
	// 	}
	// }
	// fmt.Printf("Void Walker: Found %d Dark Matter nodes.\n", countDark)
}

// isRoot determines if a node is an Ingress Point (Internet).
func isRoot(n *Node) bool {
	// Check for AWS Ingress Points.
	if n.Type == "AWS::EC2::InternetGateway" {
		return true
	}
	if n.Type == "AWS::EC2::VPNGateway" {
		return true
	}
	// Note: ALBs and other edge services are typically accessed via an IGW.
	return false
}

// canTraverse applies the "Top Notch" logic: Route -> SG -> ACL.
func canTraverse(source, target *Node, edge Edge) bool {
	// Implements simplified traversal logic (Physical -> Route -> Security).

	// 1. Physical/L2 Connectivity
	// Edges of type "Contains" or "AttachedTo" imply physical connectivity.

	// 2. L3/L4 Constraints (Route Tables, Security Groups, NACLs)
	// For this version, we implement basic network segment validation.

	// Check for explicit network isolation.
	if val, ok := target.Properties["NetworkType"]; ok {
		if val == "Private" {
			// Enforce Air-Gap logic: Prevent direct traversal from Internet Gateway to Private nodes.
			if source.Type == "AWS::EC2::InternetGateway" {
				return false
			}
		}
	}

	return true
}

// Helper to check for Dark Matter in downstream analysis
func IsDarkMatter(n *Node) bool {
	return n.Reachability == ReachabilityDarkMatter
}
