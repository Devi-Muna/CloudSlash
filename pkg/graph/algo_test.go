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

	// Wait for graph build (Pipeline architecture)
	g.CloseAndWait()

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
		names = append(names, n.IDStr())
	}

	expected := []string{"instance", "subnet", "vpc"}
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

	g.CloseAndWait()

	nodes := []*Node{g.GetNode("A"), g.GetNode("B")}

	_, err := g.TopologicalSort(nodes)
	if err == nil {
		t.Errorf("Expected cycle error, got nil")
	}
}

func FuzzTopologicalSort(f *testing.F) {
	// Seed with some initial data
	f.Add([]byte("initial_seed_data"))
	f.Add([]byte{0x1, 0x2, 0x3, 0x4}) // Binary seed

	f.Fuzz(func(t *testing.T, data []byte) {
		g := NewGraph()

		// Use data to deterministically build a graph
		// Strategy:
		// - 1st byte: Number of nodes (mod 50 to keep it fast)
		// - Rest: Edges

		if len(data) < 2 {
			return
		}

		numNodes := int(data[0]) % 50
		if numNodes == 0 {
			numNodes = 2
		}

		// Create Nodes
		for i := 0; i < numNodes; i++ {
			id := string(rune('a' + i)) // "a", "b", "c"...
			g.AddNode(id, "Type", nil)
		}

		// Create Edges from remaining bytes
		// Pairs of bytes define Source -> Target indices
		edgeBytes := data[1:]
		for i := 0; i < len(edgeBytes)-1; i += 2 {
			srcIdx := int(edgeBytes[i]) % numNodes
			tgtIdx := int(edgeBytes[i+1]) % numNodes

			if srcIdx == tgtIdx {
				continue // Self-loops are cycles, handled but boring
			}

			srcID := string(rune('a' + srcIdx))
			tgtID := string(rune('a' + tgtIdx))
			g.AddEdge(srcID, tgtID)
		}

		g.CloseAndWait()

		nodes := g.GetNodes()
		sorted, err := g.TopologicalSort(nodes)

		if err != nil {
			// Cyclic graphs should error, not panic or hang.
			// This is acceptable behavior.
			return
		}

		// Happy Path Verification
		if len(sorted) != len(nodes) {
			t.Errorf("Sorted length %d != Nodes length %d", len(sorted), len(nodes))
		}

		// Verify Order Property: For every edge A->B, A is Dependent, B is Dependency.
		// CloudSlash TopoSort returns: [Dependent, ..., Dependency]
		// So A should appear BEFORE B in the list.

		// Map for fast index lookup
		pos := make(map[uint32]int)
		for i, n := range sorted {
			pos[n.ID] = i
		}

		// Check all edges
		allNodes := g.GetNodes()
		for _, srcNode := range allNodes {
			edges := g.GetEdges(srcNode.Index)

			for _, edge := range edges {
				tgtNode := g.GetNodeByID(edge.TargetID)
				if tgtNode == nil {
					continue
				}

				// Edge: src -> tgt
				// Expectation: src comes BEFORE tgt in sorted list
				pSrc, ok1 := pos[srcNode.ID]
				pTgt, ok2 := pos[tgtNode.ID]

				if ok1 && ok2 {
					if pSrc > pTgt {
						t.Errorf("Topological Violation: %s -> %s, but %s is after %s in sort",
							srcNode.IDStr(), tgtNode.IDStr(), srcNode.IDStr(), tgtNode.IDStr())
					}
				}
			}
		}
	})
}
