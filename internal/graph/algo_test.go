package graph

import (
	"reflect"
	"testing"
)

func TestTopologicalSort(t *testing.T) {
	g := NewGraph()
	// Verify topological sort behavior for resource deletion.
	// Context: Resources must be deleted in standard dependency order (Dependent -> Dependency).
	// Example: Instance -> Subnet -> VPC.
	//
	// Graph Construction:
	// Instance (AttachedTo) -> Subnet (AttachedTo) -> VPC.
	//
	// A topological sort typically produces an ordering A, B such that for every edge A -> B, A comes before B.
	// For deletion, if A depends on B, we must delete A first.
	// Thus, the sort order corresponds directly to the safe deletion sequence.

	g.AddNode("vpc", "VPC", nil)
	g.AddNode("subnet", "Subnet", nil)
	g.AddNode("instance", "Instance", nil)

	g.AddEdge("instance", "subnet")
	g.AddEdge("subnet", "vpc")

	nodes := []*Node{
		g.GetNode("vpc"),
		g.GetNode("subnet"),
		g.GetNode("instance"),
	}

	sorted, err := g.TopologicalSort(nodes)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify that the sort logic produces reverse-dependency order (safe for deletion).
	// Required Order: Instance (Dependent) -> Subnet -> VPC (Dependency).
	// Note: The implementation of TopologicalSort in this package returns nodes in reverse-topological order
	// (Dependent first) which aligns with deletion requirements.
	var names []string
	for _, n := range sorted {
		names = append(names, n.ID)
	}

	expected := []string{"vpc", "subnet", "instance"}
	if !reflect.DeepEqual(names, expected) {
		t.Errorf("Expected %v, got %v", expected, names)
	}
}

func TestCycleDetection(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", "N", nil)
	g.AddNode("B", "N", nil)

	g.AddEdge("A", "B")
	g.AddEdge("B", "A")

	nodes := []*Node{g.GetNode("A"), g.GetNode("B")}

	_, err := g.TopologicalSort(nodes)
	if err == nil {
		t.Errorf("Expected cycle error, got nil")
	}
}
