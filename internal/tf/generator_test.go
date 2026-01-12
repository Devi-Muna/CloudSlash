package tf

import (
	"os"
	"strings"
	"testing"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

func TestGenerateAtomicNuke(t *testing.T) {
	// 1. Setup Graph: Instance -> Subnet -> VPC
	g := graph.NewGraph()
	g.AddNode("vpc-1", "AWS::EC2::VPC", nil)
	g.AddNode("sub-1", "aws_subnet", nil) // Use alternate type name to test normalization
	g.AddNode("i-1", "AWS::EC2::Instance", nil)

	// Edges: Dependencies.
	// Instance depends on Subnet. Subnet depends on VPC.
	// Graph Edge: Dependent -> Dependency (if strictly dependency)
	// But our graph usually models "Contains" or "AttachedTo".
	// Algo.go TopologicalSort assumes Edge(A, B) means A depends on B (?) or A flows to B?
	// Let's check algo_test.go: Edge("instance", "subnet").
	// Result of TopSort: [VPC, Subnet, Instance] (Root -> Leaf).
	// Generator Reverses this: [Instance, Subnet, VPC].

	g.AddEdge("i-1", "sub-1")
	g.AddEdge("sub-1", "vpc-1")

	// Mark as Waste so they are included
	g.Nodes["vpc-1"].IsWaste = true
	g.Nodes["sub-1"].IsWaste = true
	g.Nodes["i-1"].IsWaste = true

	// 2. Generate Script
	gen := NewGenerator(g, nil)
	tmpFile := "test_atomic_nuke.sh"
	defer os.Remove(tmpFile)

	err := gen.GenerateAtomicNuke(tmpFile)
	if err != nil {
		t.Fatalf("Generation failed: %v", err)
	}

	// 3. Verify Content
	contentBytes, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	content := string(contentBytes)

	// Check for Commands (Loose check to ignore region placement)
	if !strings.Contains(content, "terminate-instances --instance-ids i-1") {
		t.Errorf("Missing instance deletion command")
	}
	if !strings.Contains(content, "delete-subnet --subnet-id sub-1") {
		t.Errorf("Missing subnet deletion command")
	}
	if !strings.Contains(content, "delete-vpc --vpc-id vpc-1") {
		t.Errorf("Missing VPC deletion command")
	}

	// 4. Verify Order (The Gordian Knot Logic)
	// Instance must be deleted BEFORE Subnet
	idxInstance := strings.Index(content, "terminate-instances")
	idxSubnet := strings.Index(content, "delete-subnet")
	idxVPC := strings.Index(content, "delete-vpc")

	if idxInstance == -1 || idxSubnet == -1 {
		t.Fatalf("Commands missing from order check")
	}

	if idxInstance > idxSubnet {
		t.Errorf("ORDER FAIL: Instance (%d) deleted AFTER Subnet (%d)", idxInstance, idxSubnet)
	}
	if idxSubnet > idxVPC {
		t.Errorf("ORDER FAIL: Subnet (%d) deleted AFTER VPC (%d)", idxSubnet, idxVPC)
	}
}
