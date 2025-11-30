package aws

import (
	"context"
	"time"

	"github.com/saujanyayaya/cloudslash/internal/graph"
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

	// 2. Unattached Volume
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mock1234567890", "AWS::EC2::Volume", map[string]interface{}{
		"State": "available",
	})

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
	}

	// 5. Stale S3 Multipart Upload
	s.Graph.AddNode("arn:aws:s3:::mock-bucket/upload-1", "AWS::S3::MultipartUpload", map[string]interface{}{
		"Initiated": time.Now().Add(-10 * 24 * time.Hour), // 10 days old
	})

	return nil
}
