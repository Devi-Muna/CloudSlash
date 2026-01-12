package graph

import (
	"reflect"
	"testing"
)

func TestTopologicalSort(t *testing.T) {
	g := NewGraph()
	// Setup Dependency Chain: Leaf -> Middle -> Root
	// Waste is usually Root -> Middle -> Leaf in generic terms?
	// But our graph edges are:
	// Instance (AttachedTo) Subnet (AttachedTo) VPC
	// Deletion Order: Instance, Subnet, VPC.
	//
	// Edge: Instance -> Subnet
	// TopSort(DFS Post Order): Subnet added first, then Instance.
	// Result: [Subnet, Instance].
	// This is REVERSE of what we want?
	// Wait.
	// If A -> B (A depends on B). e.g. Instance -> Subnet.
	// We visit A.
	//   Visit B.
	//     B has no children.
	//     Add B to list.
	//   Add A to list.
	// Result: [B, A] i.e. [Subnet, Instance].
	//
	// Deletion Order:
	// Can I delete Subnet first? NO. Instance is in it.
	// I must delete Instance first.
	// So I want [Instance, Subnet] == [A, B].
	//
	// So DFS Post-Order gives [Dependency, Dependent].
	// We want [Dependent, Dependency].
	//
	// The `algo.go` implementation appends to list at end of visit.
	// That is Post-Order.
	// So `algo.go` produces [Subnet, Instance].
	// This is WRONG for deletion order if Edge means Dependency.
	//
	// Let's verify what "Edge" means in our graph.
	// "AttachedTo" -> Target is the thing we are attached to (The Dependency).
	// So Instance -> Subnet.
	//
	// We need to Reverse the list returned by Post-Order DFS?
	// Or use Pre-Order?

	// Let's test precisely.

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

	// Expectation: Post-Order DFS
	// If we start at Instance:
	//   Visit Subnet
	//     Visit VPC
	//       Add VPC
	//     Add Subnet
	//   Add Instance
	// Result: [VPC, Subnet, Instance]
	//
	// Deletion Order Required: Instance, Subnet, VPC.
	// (Delete Instance first to free Subnet, Delete Subnet to free VPC).
	//
	// So the Algo result [VPC, Subnet, Instance] is ROOT FIRST.
	// We want LEAF FIRST (if Leaf depends on Root).
	//
	// So we need to REVERSE the result of `TopologicalSort` in the generator
	// OR modify `TopologicalSort` to prepend?
	// Or `TopologicalSort` returns "Construction Order".
	// "Reverse Topological Sort" usually means reverse of TopSort.

	// I'll test that it returns [VPC, Subnet, Instance].
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
