package graph

import (
	"testing"
)

func TestGraph_AddNodeAndEdge(t *testing.T) {
	g := NewGraph()

	g.AddNode("node1", "TypeA", nil)
	g.AddNode("node2", "TypeB", nil)
	g.AddEdge("node1", "node2")

	if len(g.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(g.Nodes))
	}

	if len(g.Edges["node1"]) != 1 {
		t.Errorf("Expected 1 edge from node1, got %d", len(g.Edges["node1"]))
	}

	if g.Edges["node1"][0] != "node2" {
		t.Errorf("Expected edge to node2, got %s", g.Edges["node1"][0])
	}
}

func TestGraph_GetConnectedComponent(t *testing.T) {
	g := NewGraph()

	// A -> B -> C
	// D -> E
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("D", "E")

	comp := g.GetConnectedComponent("A")

	// Should find A, B, C (3 nodes)
	// Note: AddEdge creates nodes if they don't exist
	if len(comp) != 3 {
		t.Errorf("Expected component size 3, got %d", len(comp))
	}
}
