package heuristics

import (
	"context"
	"testing"
	"time"

	"github.com/saujanyayaya/cloudslash/internal/graph"
)

func TestZombieEBSHeuristic(t *testing.T) {
	g := graph.NewGraph()
	ctx := context.Background()

	// 1. Setup Graph with a Zombie Volume
	// Instance stopped 40 days ago
	// Note: Heuristic constructs ARN as "arn:aws:ec2:region:account:instance/%s"
	g.AddNode("arn:aws:ec2:region:account:instance/i-stopped", "AWS::EC2::Instance", map[string]interface{}{
		"State":      "stopped",
		"LaunchTime": time.Now().Add(-40 * 24 * time.Hour),
	})
	// Volume attached to it
	g.AddNode("vol-zombie", "AWS::EC2::Volume", map[string]interface{}{
		"State":              "in-use",
		"AttachedInstanceId": "i-stopped",
	})

	// 2. Setup Graph with a Healthy Volume
	// Instance running
	g.AddNode("arn:aws:ec2:region:account:instance/i-running", "AWS::EC2::Instance", map[string]interface{}{
		"State":      "running",
		"LaunchTime": time.Now().Add(-10 * 24 * time.Hour),
	})
	// Volume attached to it
	g.AddNode("vol-healthy", "AWS::EC2::Volume", map[string]interface{}{
		"State":              "in-use",
		"AttachedInstanceId": "i-running",
	})

	// 3. Run Heuristic
	h := &ZombieEBSHeuristic{}
	h.Analyze(ctx, g)

	// 4. Assertions
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	// Check Zombie
	if node, ok := g.Nodes["vol-zombie"]; !ok {
		t.Fatal("Zombie volume not found in graph")
	} else {
		if !node.IsWaste {
			t.Error("Expected vol-zombie to be marked as waste, but it wasn't")
		}
		if node.RiskScore != 70 {
			t.Errorf("Expected RiskScore 70, got %d", node.RiskScore)
		}
	}

	// Check Healthy
	if node, ok := g.Nodes["vol-healthy"]; !ok {
		t.Fatal("Healthy volume not found in graph")
	} else {
		if node.IsWaste {
			t.Error("Expected vol-healthy NOT to be marked as waste, but it was")
		}
	}
}

func TestS3MultipartHeuristic(t *testing.T) {
	g := graph.NewGraph()
	ctx := context.Background()

	// 1. Old Upload (Waste)
	g.AddNode("upload-old", "AWS::S3::MultipartUpload", map[string]interface{}{
		"Initiated": time.Now().Add(-8 * 24 * time.Hour), // 8 days old
	})

	// 2. New Upload (Not Waste)
	g.AddNode("upload-new", "AWS::S3::MultipartUpload", map[string]interface{}{
		"Initiated": time.Now().Add(-1 * 24 * time.Hour), // 1 day old
	})

	// 3. Run Heuristic
	h := &S3MultipartHeuristic{}
	h.Analyze(ctx, g)

	// 4. Assertions
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	if node, ok := g.Nodes["upload-old"]; !ok || !node.IsWaste {
		t.Error("Expected upload-old to be waste")
	}

	if node, ok := g.Nodes["upload-new"]; !ok || node.IsWaste {
		t.Error("Expected upload-new NOT to be waste")
	}
}
