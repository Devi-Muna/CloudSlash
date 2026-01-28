package tf

import (
	"os"
	"strings"
	"testing"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

func TestGenerateDeletionScript(t *testing.T) {
	// 1. Setup Graph: Instance -> Subnet -> VPC
	g := graph.NewGraph()
	g.AddNode("vpc-1", "AWS::EC2::VPC", nil)
	g.AddNode("sub-1", "aws_subnet", nil) // Use alternate type name to test normalization
	g.AddNode("i-1", "AWS::EC2::Instance", nil)

	// Graph Topology:
	// Instance (AttachedTo) -> Subnet (AttachedTo) -> VPC.
	//
	// Deletion Requirement:
	// Resources must be deleted in reverse dependency order:
	// 1. Instance (dependent on Subnet)
	// 2. Subnet (dependent on VPC)
	// 3. VPC (Root)

	g.AddEdge("i-1", "sub-1")
	g.AddEdge("sub-1", "vpc-1")

	// Mark as Waste so they are included
	g.GetNode("vpc-1").IsWaste = true
	g.GetNode("sub-1").IsWaste = true
	g.GetNode("i-1").IsWaste = true

	// 2. Generate Script
	gen := NewGenerator(g, nil)
	tmpFile := "test_deletion_script.sh"
	defer os.Remove(tmpFile)

	err := gen.GenerateDeletionScript(tmpFile)
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

	// 4. Verify Execution Order
	// Ensure instance command appears before subnet command.
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
