package graph

// AnalyzeReachability performs the "Void Walker" analysis.
// It marks nodes as ReachabilityReachable or ReachabilityDarkMatter.
func AnalyzeReachability(g *Graph) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	// 1. Identify Roots (Ingress Points)
	var queue []uint32
	for _, node := range g.Nodes {
		// Initialize all as Dark Matter (unless potentially ignored/scope?)
		// We'll set them to Unknown first, then sweep.
		node.Reachability = ReachabilityDarkMatter

		if isRoot(node) {
			node.Reachability = ReachabilityReachable
			queue = append(queue, node.Index)
		}
	}

	// 2. BFS Flood Fill
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

				// 3. The Constraint Check
				if canTraverse(g.Nodes[currentIdx], targetNode, edge) {
					targetNode.Reachability = ReachabilityReachable
					visited[targetIdx] = true
					queue = append(queue, targetIdx)
				}
			}
		}
	}

	// 4. Reporting (Optional Debug)
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
	// AWS Ingress Points
	if n.Type == "AWS::EC2::InternetGateway" {
		return true
	}
	if n.Type == "AWS::EC2::VPNGateway" {
		return true
	}
	// Direct Connect?
	// ALB Public? (ALB is usually behind IGW, so IGW is the true root)
	return false
}

// canTraverse applies the "Top Notch" logic: Route -> SG -> ACL.
func canTraverse(source, target *Node, edge Edge) bool {
	// Simplied "Void Walker" Logic for v1.3.6 MVP

	// 1. Physical Connectivity Check
	// If Edge is "Contains" or "AttachedTo", we assume flow is possible at L1/L2.
	// But we need L3 (Route) and L4 (SG).

	// For MVP, we presume the Graph Scanner has only created "FlowsTo" edges
	// if a Route Exists. (This shifts complexity to Scanner, or we check props here).

	// Let's implement a basic check based on properties if available.

	// Example: check if target is a "Private Subnet" (no route to IGW).
	// If source is IGW and target is Subnet, check RouteTable.

	// Assuming the edge represents a "potential" flow.

	// Constraint: Security Groups
	// If target has SG, does it allow traffic from Source?
	// (This requires deep packet analysis or partial evaluation).

	// Void Walker Logic:
	// "If (Route exists) AND (SG allows Port X) AND (NACL allows IP), then Traverse."

	// For simulation purposes (since we don't have full rules engine yet):
	// Check if target is explicitly "Private".
	if val, ok := target.Properties["NetworkType"]; ok {
		if val == "Private" {
			// Only allow if source is also "Private" (Lateral movement)
			// OR if source is a Load Balancer / NAT?
			// This is complex.

			// Simple "Air Gap" Logic:
			// If we are traversing FROM InternetGateway DIRECTLY to a Private Node, BLOCK.
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
