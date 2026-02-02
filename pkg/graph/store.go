package graph

// GraphStore defines graph storage interface.
type GraphStore interface {
	// Node operations.
	AddNode(node *Node) uint32
	GetNode(index uint32) *Node
	GetNodeID(stringID string) (uint32, bool)
	GetNodeByStringID(stringID string) *Node
	UpdateNode(index uint32, update func(*Node))
	NodeCount() int
	GetAllNodes() []*Node // Warning: O(N) operation.

	// Edge operations.
	AddEdge(sourceIndex uint32, edge Edge)
	GetEdges(sourceIndex uint32) []Edge
	GetReverseEdges(targetIndex uint32) []Edge
}
