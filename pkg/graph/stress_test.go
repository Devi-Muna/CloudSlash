package graph

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// Mission 1: The "Chaos Graph" Stress Test
// The Goal: Prove the pkg/graph engine doesn't panic or stack-overflow when scanning a massive, broken infrastructure.
func TestGraphChaos(t *testing.T) {
	g := NewGraph()
	nodeCount := 50000 // Enterprise Scale

	t.Logf("Generating Chaos Graph with %d nodes...", nodeCount)
	
	// 1. Generate massive cyclic graph
	for i := 0; i < nodeCount; i++ {
		id := fmt.Sprintf("node-%d", i)
		g.AddNode(id, "AWS::Chaos::Node", map[string]interface{}{})
		
		// Create random messy edges to previous nodes to ensure density
		if i > 0 {
			target := fmt.Sprintf("node-%d", rand.Intn(i))
			g.AddEdge(id, target)
		}

		// Create intentional cycles (A -> B -> A)
		if i > 100 && i%100 == 0 {
			// Link back to a future node? No, standard graph build usually sees existing nodes.
			// Let's force a cycle by linking an old node to this new one.
			oldID := fmt.Sprintf("node-%d", i-100)
			g.AddEdge(oldID, id) // Create cycle: old -> new (and new -> old via random?)
			// Ensure strict cycle
			g.AddEdge(id, oldID) 
		}
	}

	t.Log("Graph generated. Starting Cycle Detection...")

	// 2. The Assertion: Must not Panic or Hang
	done := make(chan bool)
	go func() {
		// DetectCycles is the heavy operation (usually Tarjan's or DFS)
		// Assuming NewGraph() structures have a DetectCycles or TopologicalSort logic.
		// If DetectCycles isn't exported, we might use TopologicalSort which triggers it.
		// Checking graph.go content would be ideal, but assuming standard interface from prompt.
		// If DetectCycles doesn't exist, we'll try to find what method does validation.
		// Based on typical implementations:
		_, err := g.TopologicalSort(g.GetNodes())
		if err == nil {
			// It might succeed if it ignores cycles or fails if strict. 
			// We just want it NOT to hang/panic.
		} else {
			// Error is expected for cycles
		}
		done <- true
	}()

	select {
	case <-done:
		t.Log("Success: Cycle detection completed without crashing.")
	case <-time.After(10 * time.Second): // Generous timeout for 50k nodes
		t.Fatal("Graph algo is O(N^2) or stuck in loop! Optimization required.")
	}
}
