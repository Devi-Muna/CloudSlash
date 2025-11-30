package graph

import (
	"sync"
)

// Node represents a resource in the infrastructure graph.
type Node struct {
	ID         string                 // Unique Identifier (ARN)
	Type       string                 // Resource Type (e.g., "AWS::EC2::Instance")
	Properties map[string]interface{} // Resource attributes
	IsWaste    bool                   // Flagged as waste?
	RiskScore  int                    // 0-100
}

// Graph represents the infrastructure topology as a DAG.
type Graph struct {
	Mu           sync.RWMutex
	Nodes        map[string]*Node
	Edges        map[string][]string // ID -> []ID (Flows To)
	ReverseEdges map[string][]string // ID -> []ID (Attached To / Secured By)
}

// NewGraph creates a new empty graph.
func NewGraph() *Graph {
	return &Graph{
		Nodes:        make(map[string]*Node),
		Edges:        make(map[string][]string),
		ReverseEdges: make(map[string][]string),
	}
}

// AddNode adds a resource to the graph.
func (g *Graph) AddNode(id, resourceType string, props map[string]interface{}) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	if _, exists := g.Nodes[id]; !exists {
		g.Nodes[id] = &Node{
			ID:         id,
			Type:       resourceType,
			Properties: props,
		}
	}
}

// AddEdge adds a directed edge from source to target.
func (g *Graph) AddEdge(sourceID, targetID string) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	// Ensure nodes exist (create placeholders if not)
	if _, ok := g.Nodes[sourceID]; !ok {
		g.Nodes[sourceID] = &Node{ID: sourceID, Type: "Unknown"}
	}
	if _, ok := g.Nodes[targetID]; !ok {
		g.Nodes[targetID] = &Node{ID: targetID, Type: "Unknown"}
	}

	g.Edges[sourceID] = append(g.Edges[sourceID], targetID)
	g.ReverseEdges[targetID] = append(g.ReverseEdges[targetID], sourceID)
}

// GetConnectedComponent returns all nodes reachable from startID (BFS).
// Useful for finding all resources in a VPC or related to a specific security group.
func (g *Graph) GetConnectedComponent(startID string) []*Node {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	visited := make(map[string]bool)
	queue := []string{startID}
	var component []*Node

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		if node, ok := g.Nodes[currentID]; ok {
			component = append(component, node)
		}

		// Traverse forward edges
		for _, neighbor := range g.Edges[currentID] {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}

		// Traverse backward edges (undirected connectivity check)
		// If we want strictly downstream, remove this.
		// For "Connected Component" in the waste sense (VPC + everything in it),
		// we usually want full connectivity.
		for _, neighbor := range g.ReverseEdges[currentID] {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}
	}

	return component
}

// MarkWaste flags a node and optionally its dependencies as waste.
func (g *Graph) MarkWaste(id string, score int) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	if node, ok := g.Nodes[id]; ok {
		node.IsWaste = true
		node.RiskScore = score
	}
}
