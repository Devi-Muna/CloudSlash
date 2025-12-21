package aws

import (
	"context"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

type MockScanner struct {
	Graph *graph.Graph
}

func NewMockScanner(g *graph.Graph) *MockScanner {
	return &MockScanner{Graph: g}
}

func (s *MockScanner) Scan(ctx context.Context) error {
	// Simulate network delay
	time.Sleep(100 * time.Millisecond) // Faster for demo

	// 1. Stopped Instance (Zombie)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:instance/i-0mock1234567890", "AWS::EC2::Instance", map[string]interface{}{
		"State":      "stopped",
		"LaunchTime": time.Now().Add(-60 * 24 * time.Hour), // 60 days old
	})

	// 2. Unattached Volume (v1.2 Auditor Test)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mock1234567890", "AWS::EC2::Volume", map[string]interface{}{
		"State": "available",
		"Size":  100, // GB
	})
    if node, ok := s.Graph.Nodes["arn:aws:ec2:us-east-1:123456789012:volume/vol-0mock1234567890"]; ok {
        node.Cost = 8.00 // Manually set cost to test TUI
        node.SourceLocation = "terraform/storage.tf:24" // Manually set source to test TUI
    }

	// 3. Zombie Volume (Attached to stopped instance)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockZombie", "AWS::EC2::Volume", map[string]interface{}{
		"State":               "in-use",
		"AttachedInstanceId":  "i-0mock1234567890",
		"DeleteOnTermination": false,
	})

	// 4. Unused NAT Gateway (Marked as waste manually for demo since we skip CW)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:natgateway/nat-0mock12345", "AWS::EC2::NatGateway", map[string]interface{}{
		"State": "available",
	})
	s.Graph.MarkWaste("arn:aws:ec2:us-east-1:123456789012:natgateway/nat-0mock12345", 80)
	if node, ok := s.Graph.Nodes["arn:aws:ec2:us-east-1:123456789012:natgateway/nat-0mock12345"]; ok {
		node.Properties["Reason"] = "Unused NAT Gateway (Mocked)"
		node.Cost = 32.85 // v1.2 Fixed Rate
	}

    // 5. Mock Snapshot (Time Machine Test)
    // Parent volume is vol-0mock1234567890 (which is waste)
    s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:snapshot/snap-0mockChild", "AWS::EC2::Snapshot", map[string]interface{}{
        "State": "completed",
        "VolumeId": "vol-0mock1234567890", // Links to waste vol
        "VolumeSize": 100,
    })

	// 5. Stale S3 Multipart Upload
	s.Graph.AddNode("arn:aws:s3:::mock-bucket/upload-1", "AWS::S3::MultipartUpload", map[string]interface{}{
		"Initiated": time.Now().Add(-10 * 24 * time.Hour), // 10 days old
	})

    // 6. Ignored Resource (Should NOT appear in TUI)
    s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockIGNORED", "AWS::EC2::Volume", map[string]interface{}{
        "State": "available",
        "Tags": map[string]string{
            "cloudslash:ignore": "true",
        },
    })
    s.Graph.MarkWaste("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockIGNORED", 100)
	// Heuristic runs later and marks waste, but we need to ensure it has cost if we want charts now?
	// The heuristics run in mock mode too (see main.go).
	// However, heuristics calculate cost using Pricing Client which mocks don't hold.
	// So we should manually simulate cost detection or update heuristics to check if Cost is already set?
	// Heuristics overwrite cost usually.
	// But in Mock Mode (main.go), we call heuristics:
	// zombieHeuristic.Analyze(ctx, g) -> calls Pricing if set. Pricing is nil in Mock Mode.
	// So heuristics won't set cost. We must pre-set it here and ensuring heuristics don't overwrite with 0 if Pricing is nil.

	// Let's set costs here on the graph nodes directly.

	return nil
}
