package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

type MockScanner struct {
	Graph *graph.Graph
}

func NewMockScanner(g *graph.Graph) *MockScanner {
	return &MockScanner{Graph: g}
}

func (s *MockScanner) Scan(ctx context.Context) error {
	// Simulate network latency.
	time.Sleep(100 * time.Millisecond)

	// 1. Stopped Instance (Unused)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:instance/i-0mock1234567890", "AWS::EC2::Instance", map[string]interface{}{
		"State":      "stopped",
		"LaunchTime": time.Now().Add(-60 * 24 * time.Hour), // 60 days old
	})

	// 2. Unattached Volume (Auditor Test)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mock1234567890", "AWS::EC2::Volume", map[string]interface{}{
		"State": "available",
		"Size":  100, // GB
	})
	nodeMockVol := s.Graph.GetNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mock1234567890")
	if nodeMockVol != nil {
		s.Graph.Mu.Lock()
		nodeMockVol.Cost = 8.00
		nodeMockVol.SourceLocation = "terraform/storage.tf:24"
		s.Graph.Mu.Unlock()
	}

	// 3. Unused Volume (Attached to stopped instance)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockZombie", "AWS::EC2::Volume", map[string]interface{}{
		"State":               "in-use",
		"AttachedInstanceId":  "i-0mock1234567890",
		"DeleteOnTermination": false,
	})

	// 4. Idle NAT Gateway
	// Triggers NetworkAnalysisHeuristic
	natArn := "arn:aws:ec2:us-east-1:123456789012:natgateway/nat-0mock12345"
	s.Graph.AddNode(natArn, "aws_nat_gateway", map[string]interface{}{
		"State":            "available",
		"SumConnections7d": 0.0,
		"ActiveUserENIs":   0,
		"EmptySubnets":     []string{"subnet-mock-empty-1", "subnet-mock-empty-2"},
		"Region":           "us-east-1",
	})

	// 5. Mock Snapshot (Time Machine Test)
	// Parent volume is vol-0mock1234567890 (which is waste)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:snapshot/snap-0mockChild", "AWS::EC2::Snapshot", map[string]interface{}{
		"State":      "completed",
		"VolumeId":   "vol-0mock1234567890", // Links to waste vol
		"VolumeSize": 100,
	})

	// 5b. Safe-Release Elastic IP
	// Triggers NetworkAnalysisHeuristic
	eipArn := "arn:aws:ec2:us-east-1:123456789012:eip/eipalloc-0mock123"
	s.Graph.AddNode(eipArn, "aws_eip", map[string]interface{}{
		"PublicIp":      "203.0.113.10",
		"Region":        "us-east-1",
		"AssociationId": "",    // Unattached
		"FoundInDNS":    false, // Safe
	})

	// 5c. Unattached EIP
	eipDangerArn := "arn:aws:ec2:us-east-1:123456789012:eip/eipalloc-0mockDanger"
	s.Graph.AddNode(eipDangerArn, "aws_eip", map[string]interface{}{
		"PublicIp":      "203.0.113.99",
		"Region":        "us-east-1",
		"AssociationId": "",
		"FoundInDNS":    true,
		"DNSZone":       "production.com",
	})

	// 5d. S3 Lifecycle Gap
	s.Graph.AddNode("arn:aws:s3:::mock-bucket-iceberg", "AWS::S3::Bucket", map[string]interface{}{
		"Name":              "mock-bucket-iceberg",
		"HasAbortLifecycle": false,
	})
	s.Graph.AddNode("arn:aws:s3:::multipart/mock-bucket-iceberg/upload-1", "AWS::S3::MultipartUpload", map[string]interface{}{
		"Initiated": time.Now().Add(-15 * 24 * time.Hour), // 15 days old
		"Bucket":    "mock-bucket-iceberg",
	})

	// 5e. Legacy EBS
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockGp2", "AWS::EC2::Volume", map[string]interface{}{
		"State":       "in-use",
		"Size":        int32(500), // 500GB
		"VolumeType":  "gp2",
		"IsModifying": false,
		"Region":      "us-east-1",
	})

	// 5g. Aged AMI (New Heuristic Test)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:image/ami-0mockAged", "AWS::EC2::AMI", map[string]interface{}{
		"Name":         "legacy-server-backup-2023",
		"State":        "available",
		"CreationDate": time.Now().Add(-100 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z"),
		"CreateTime":   time.Now().Add(-100 * 24 * time.Hour), // 100 days old
	})

	// 5h. Ignored AMI (Explicit True) -> SHOULD NOT APPEAR
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:image/ami-0mockIgnoreTrue", "AWS::EC2::AMI", map[string]interface{}{
		"Name":         "important-backup",
		"State":        "available",
		"CreationDate": time.Now().Add(-200 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z"),
		"CreateTime":   time.Now().Add(-200 * 24 * time.Hour), // 200 days old
		"Tags": map[string]string{
			"cloudslash:ignore": "true",
		},
	})

	// 5i. Ignored AMI (Duration Pass) -> SHOULD NOT APPEAR
	// Age: 100d. Ignore: 120d. (100 < 120, so valid/ignored)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:image/ami-0mockIgnoreDurationPass", "AWS::EC2::AMI", map[string]interface{}{
		"Name":         "semi-old-backup",
		"State":        "available",
		"CreationDate": time.Now().Add(-100 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z"),
		"CreateTime":   time.Now().Add(-100 * 24 * time.Hour),
		"Tags": map[string]string{
			"cloudslash:ignore": "120d",
		},
	})

	// 5j. Ignored AMI (Duration Fail) -> SHOULD APPEAR AS WASTE
	// Age: 100d. Ignore: 30d. (100 > 30, so expired/waste)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:image/ami-0mockIgnoreDurationFail", "AWS::EC2::AMI", map[string]interface{}{
		"Name":         "expired-backup",
		"State":        "available",
		"CreationDate": time.Now().Add(-100 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z"),
		"CreateTime":   time.Now().Add(-100 * 24 * time.Hour),
		"Tags": map[string]string{
			"cloudslash:ignore": "30d",
		},
	})

	// 5f. Orphaned ELB (Graph-based)
	// Needs to be tagged with a cluster that is also waste (or missing?)
	// ... (Rest of existing mocks)

	// ... (Truncating for clarity, keeping required context)

	// 5c. Orphaned ELB (Graph-based)
	// Needs to be tagged with a cluster that is also waste (or missing?)
	// UnusedEKS heuristic checks for cluster existence.
	// Let's create an Unused Cluster first.
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
	// RDSHeuristic handles stopped instances without CloudWatch metrics.

	// Unused ELB (Manual Waste Simulation)
	elbArn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/unused-internal-lb/50dc6c495c0c9999"
	s.Graph.AddNode(elbArn, "AWS::ElasticLoadBalancingV2::LoadBalancer", map[string]interface{}{
		"State":  "active",
		"Region": "us-east-1",
	})
	s.Graph.MarkWaste(elbArn, 70)
	nodeElb := s.Graph.GetNode(elbArn)
	if nodeElb != nil {
		s.Graph.Mu.Lock()
		nodeElb.Properties["Reason"] = "ELB unused: Only 2 requests in 7 days"
		nodeElb.Cost = 22.50
		s.Graph.Mu.Unlock()
	}

	// Right-Sizing EC2 (Manual Waste Simulation)
	ec2Arn := "arn:aws:ec2:us-east-1:123456789012:instance/i-0mockHuge"
	s.Graph.AddNode(ec2Arn, "AWS::EC2::Instance", map[string]interface{}{
		"InstanceType": "c5.4xlarge",
		"State":        "running",
		"Region":       "us-east-1",
	})
	s.Graph.MarkWaste(ec2Arn, 60)
	nodeEc2 := s.Graph.GetNode(ec2Arn)
	if nodeEc2 != nil {
		s.Graph.Mu.Lock()
		nodeEc2.Properties["Reason"] = "Right-Sizing Opportunity: Max CPU 2.1% < 5% over 7 days"
		nodeEc2.Cost = 600.00
		s.Graph.Mu.Unlock()
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
	frontendClusterArn := "arn:aws:ecs:us-east-1:123456789012:cluster/frontend-cluster"
	s.Graph.AddNode(frontendClusterArn, "AWS::ECS::Cluster", map[string]interface{}{
		"Name": "frontend-cluster",
	})
	s.Graph.AddTypedEdge(frontendClusterArn, serviceArn, graph.EdgeTypeContains, 1)

	// 9. ECS: Healthy Production Topology (Harmony Test)
	prodMockCluster := "arn:aws:ecs:us-east-1:123456789012:cluster/production-cluster"
	s.Graph.AddNode(prodMockCluster, "AWS::ECS::Cluster", map[string]interface{}{
		"Name":                "production-cluster",
		"Status":              "ACTIVE",
		"RunningTasksCount":   15,
		"ActiveServicesCount": 2,
	})

	// Service A: Auth (Healthy)
	authSvc := "arn:aws:ecs:us-east-1:123456789012:service/production-cluster/auth-service"
	s.Graph.AddNode(authSvc, "AWS::ECS::Service", map[string]interface{}{
		"Name":         "auth-service",
		"DesiredCount": 5,
		"RunningCount": 5,
		"Status":       "ACTIVE",
	})
	s.Graph.AddTypedEdge(prodMockCluster, authSvc, graph.EdgeTypeContains, 1)
	
	// Task A1 (Running)
	taskA1 := "arn:aws:ecs:us-east-1:123456789012:task/production-cluster/auth-task-uuid-1"
	s.Graph.AddNode(taskA1, "AWS::ECS::Task", map[string]interface{}{
		"Name":           "auth-task-uuid-1",
		"LastStatus":     "RUNNING",
		"DesiredStatus":  "RUNNING",
		"TaskDefinition": "arn:aws:ecs:task-def/auth:1",
	})
	s.Graph.AddTypedEdge(authSvc, taskA1, graph.EdgeTypeRuns, 1)

	// Service B: Payment (Healthy)
	paymentSvc := "arn:aws:ecs:us-east-1:123456789012:service/production-cluster/payment-service"
	s.Graph.AddNode(paymentSvc, "AWS::ECS::Service", map[string]interface{}{
		"Name":         "payment-service",
		"DesiredCount": 10,
		"RunningCount": 10,
		"Status":       "ACTIVE",
	})
	s.Graph.AddTypedEdge(prodMockCluster, paymentSvc, graph.EdgeTypeContains, 1)

	// 6. Ignored Resource (Should NOT appear in TUI)
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockIGNORED", "AWS::EC2::Volume", map[string]interface{}{
		"State": "available",
		"Tags": map[string]string{
			"cloudslash:ignore": "true",
		},
	})
	s.Graph.MarkWaste("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockIGNORED", 100)
	// Pre-set costs as Pricing Client is unavailable in mock mode.

	// Let's set costs here on the graph nodes directly.

	// Scenario 10: Inefficient Monolith Fleet simulation for Autonomy Engine.
	// Scenario: 5x m5.large instances running a legacy app.
	// Solver should recommend migrating to c6g.large.
	for i := 0; i < 5; i++ {
		arn := fmt.Sprintf("arn:aws:ec2:us-east-1:123456789012:instance/i-0mockMonolith-%d", i)
		s.Graph.AddNode(arn, "AWS::EC2::Instance", map[string]interface{}{
			"InstanceType": "m5.large", // $0.096/hr
			"State":        "running",
			"Region":       "us-east-1",
			"Zone":         "us-east-1a",
			"Tags": map[string]string{
				"Name": "legacy-monolith-worker",
				"Role": "worker",
			},
		})
		// Manually set cost for reporting
		node := s.Graph.GetNode(arn)
		s.Graph.Mu.Lock()
		node.Cost = 70.08 // roughly monthly
		s.Graph.Mu.Unlock()
	}

	// Scenario 11: High Performance Compute (HPC) simulation for Autonomy Engine.
	// Scenario: 2x c5.4xlarge instances
	for i := 0; i < 2; i++ {
		arn := fmt.Sprintf("arn:aws:ec2:us-east-1:123456789012:instance/i-0mockHPC-%d", i)
		s.Graph.AddNode(arn, "AWS::EC2::Instance", map[string]interface{}{
			"InstanceType": "c5.4xlarge",
			"State":        "running",
			"Region":       "us-east-1",
			"Zone":         "us-east-1b", // Safer zone
		})
		node := s.Graph.GetNode(arn)
		s.Graph.Mu.Lock()
		node.Cost = 600.00
		s.Graph.Mu.Unlock()
	}

	return nil
}
