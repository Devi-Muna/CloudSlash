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
		node.Cost = 8.00                                // Manually set cost to test TUI
		node.SourceLocation = "terraform/storage.tf:24" // Manually set source to test TUI
	}

	// 3. Zombie Volume (Attached to stopped instance)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockZombie", "AWS::EC2::Volume", map[string]interface{}{
		"State":               "in-use",
		"AttachedInstanceId":  "i-0mock1234567890",
		"DeleteOnTermination": false,
	})

	// 4. Hollow NAT Gateway (v1.3.0)
	// Triggers NetworkForensicsHeuristic
	natArn := "arn:aws:ec2:us-east-1:123456789012:natgateway/nat-0mock12345"
	s.Graph.AddNode(natArn, "aws_nat_gateway", map[string]interface{}{
		"State": "available",
		"SumConnections7d": 0.0,
		"ActiveUserENIs": 0,
		"EmptySubnets": []string{"subnet-mock-empty-1", "subnet-mock-empty-2"},
		"Region": "us-east-1",
	})
	
	// 5. Mock Snapshot (Time Machine Test)
	// Parent volume is vol-0mock1234567890 (which is waste)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:snapshot/snap-0mockChild", "AWS::EC2::Snapshot", map[string]interface{}{
		"State":      "completed",
		"VolumeId":   "vol-0mock1234567890", // Links to waste vol
		"VolumeSize": 100,
	})

	// 5b. Safe-Release Elastic IP (v1.3.0)
	// Triggers NetworkForensicsHeuristic
	eipArn := "arn:aws:ec2:us-east-1:123456789012:eip/eipalloc-0mock123"
	s.Graph.AddNode(eipArn, "aws_eip", map[string]interface{}{
		"PublicIp": "203.0.113.10",
		"Region":   "us-east-1",
		"AssociationId": "", // Unattached
		"FoundInDNS": false, // Safe
	})
	
	// 5c. Dangerous Zombie EIP (v1.3.0)
	eipDangerArn := "arn:aws:ec2:us-east-1:123456789012:eip/eipalloc-0mockDanger"
	s.Graph.AddNode(eipDangerArn, "aws_eip", map[string]interface{}{
		"PublicIp": "203.0.113.99",
		"Region":   "us-east-1",
		"AssociationId": "",
		"FoundInDNS": true,
		"DNSZone": "production.com",
	})

	// 5d. S3 Iceberg (v1.3.0)
	s.Graph.AddNode("arn:aws:s3:::mock-bucket-iceberg", "AWS::S3::Bucket", map[string]interface{}{
		"Name": "mock-bucket-iceberg",
		"HasAbortLifecycle": false,
	})
	s.Graph.AddNode("arn:aws:s3:::multipart/mock-bucket-iceberg/upload-1", "AWS::S3::MultipartUpload", map[string]interface{}{
		"Initiated": time.Now().Add(-15 * 24 * time.Hour), // 15 days old
		"Bucket": "mock-bucket-iceberg",
	})
	
	// 5e. EBS Modernizer (v1.3.0)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockGp2", "AWS::EC2::Volume", map[string]interface{}{
		"State": "in-use",
		"Size": int32(500), // 500GB
		"VolumeType": "gp2",
		"IsModifying": false,
		"Region": "us-east-1",
	})

	// 5f. Orphaned ELB (Graph-based)
	// Needs to be tagged with a cluster that is also waste (or missing?)
	// ... (Rest of existing mocks)
	
	// ... (Truncating for clarity, keeping required context)
	


	// 5c. Orphaned ELB (Graph-based)
	// Needs to be tagged with a cluster that is also waste (or missing?)
	// ZombieEKS heuristic checks for cluster existence.
	// Let's create a Zombie Cluster first.
	zombieClusterArn := "arn:aws:eks:us-east-1:123456789012:cluster/legacy-dev-cluster"
	s.Graph.AddNode(zombieClusterArn, "AWS::EKS::Cluster", map[string]interface{}{
		"Name":                "legacy-dev-cluster",
		"Status":              "ACTIVE",
		"CreatedAt":           time.Now().Add(-60 * 24 * time.Hour),
		"KarpenterEnabled":    false,
		"HasManagedNodes":     false,
		"HasFargate":          false,
		"HasSelfManagedNodes": false,
		"Region":              "us-east-1",
	})

	s.Graph.AddNode("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/k8s-legacy-service/50dc6c495c0c9188",
		"AWS::ElasticLoadBalancingV2::LoadBalancer", map[string]interface{}{
			"State": "active",
			"Tags": map[string]string{
				"kubernetes.io/cluster/legacy-dev-cluster": "owned",
			},
			"Region": "us-east-1",
		})

	// 5d. Ghost Node Group (Graph-based)
	s.Graph.AddNode("arn:aws:eks:us-east-1:123456789012:nodegroup/production-cluster/failed-scaling-group", "AWS::EKS::NodeGroup", map[string]interface{}{
		"NodegroupName":     "failed-scaling-group",
		"ClusterName":       "production-cluster",
		"Status":            "ACTIVE",
		"NodeCount":         5, // BILLING
		"RealWorkloadCount": 0, // DOING NOTHING
		"Region":            "us-east-1",
	})

	// 5e. Manual Waste Simulation (Metric types where CW is nil)
	// RDS Stopped
	s.Graph.AddNode("arn:aws:rds:us-east-1:123456789012:db:legacy-postgres", "AWS::RDS::DBInstance", map[string]interface{}{
		"DBInstanceIdentifier": "legacy-postgres",
		"Status":               "stopped",
		"Region":               "us-east-1",
	})
	// Note: RDSHeuristic handles "stopped" without CW. It should work if registered.

	// Unused ELB (CW needed -> Manual Waste)
	elbArn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/unused-internal-lb/50dc6c495c0c9999"
	s.Graph.AddNode(elbArn, "AWS::ElasticLoadBalancingV2::LoadBalancer", map[string]interface{}{
		"State":  "active",
		"Region": "us-east-1",
	})
	s.Graph.MarkWaste(elbArn, 70)
	if n, ok := s.Graph.Nodes[elbArn]; ok {
		n.Properties["Reason"] = "ELB unused: Only 2 requests in 7 days"
		n.Cost = 22.50
	}

	// Right-Sizing EC2 (CW needed -> Manual Waste)
	ec2Arn := "arn:aws:ec2:us-east-1:123456789012:instance/i-0mockHuge"
	s.Graph.AddNode(ec2Arn, "AWS::EC2::Instance", map[string]interface{}{
		"InstanceType": "c5.4xlarge",
		"State":        "running",
		"Region":       "us-east-1",
	})
	s.Graph.MarkWaste(ec2Arn, 60)
	if n, ok := s.Graph.Nodes[ec2Arn]; ok {
		n.Properties["Reason"] = "Right-Sizing Opportunity: Max CPU 2.1% < 5% over 7 days"
		n.Cost = 600.00
	}

	// 5. Stale S3 Multipart Upload
	s.Graph.AddNode("arn:aws:s3:::mock-bucket/upload-1", "AWS::S3::MultipartUpload", map[string]interface{}{
		"Initiated": time.Now().Add(-10 * 24 * time.Hour), // 10 days old
	})

	// 7. ECS: Idle Cluster (The Money Saver)
	clusterArn := "arn:aws:ecs:us-east-1:123456789012:cluster/production-unused"
	s.Graph.AddNode(clusterArn, "AWS::ECS::Cluster", map[string]interface{}{
		"Name":                              "production-unused",
		"Status":                            "ACTIVE",
		"RegisteredContainerInstancesCount": 2,
		"RunningTasksCount":                 0,
		"PendingTasksCount":                 0,
		"ActiveServicesCount":               0,
		"Region":                            "us-east-1",
	})
	// Add Container Instance (Old enough to be waste)
	s.Graph.AddNode(clusterArn+"/container-instance/ci-mock-1", "AWS::ECS::ContainerInstance", map[string]interface{}{
		"ClusterArn":   clusterArn,
		"RegisteredAt": time.Now().Add(-24 * time.Hour),
		"Status":       "ACTIVE",
	})
	s.Graph.AddTypedEdge(clusterArn, clusterArn+"/container-instance/ci-mock-1", graph.EdgeType("HAS_INSTANCE"), 1)

	// 8. ECS: Empty Service (Crash Loop)
	serviceArn := "arn:aws:ecs:us-east-1:123456789012:service/frontend-cluster/payment-service-broken"
	s.Graph.AddNode(serviceArn, "AWS::ECS::Service", map[string]interface{}{
		"Name":           "payment-service-broken",
		"ClusterArn":     "arn:aws:ecs:us-east-1:123456789012:cluster/frontend-cluster",
		"Status":         "ACTIVE",
		"DesiredCount":   3,
		"RunningCount":   0,
		"PendingCount":   0,
		"LaunchType":     "FARGATE",
		"TaskDefinition": "arn:aws:ecs:...:task-definition/payment:5",
		"Events": []string{
			"(service payment-service-broken) was unable to place a task because no container instance met all of its requirements.",
			"(service payment-service-broken) has reached a steady state.",
		},
		"Region": "us-east-1",
	})
	// Link to cluster (which is NOT waste, but service IS)
	s.Graph.AddNode("arn:aws:ecs:us-east-1:123456789012:cluster/frontend-cluster", "AWS::ECS::Cluster", map[string]interface{}{
		"Name": "frontend-cluster",
	})
	s.Graph.AddTypedEdge("arn:aws:ecs:us-east-1:123456789012:cluster/frontend-cluster", serviceArn, graph.EdgeTypeContains, 1)

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
