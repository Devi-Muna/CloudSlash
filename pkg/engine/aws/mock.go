package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

// MockScanner populates the graph with synthetic data.
type MockScanner struct {
	Graph *graph.Graph
}

func NewMockScanner(g *graph.Graph) *MockScanner {
	return &MockScanner{Graph: g}
}

// Scan generates mock resources with various waste states.
func (s *MockScanner) Scan(ctx context.Context) error {
	// Simulate network latency.
	time.Sleep(100 * time.Millisecond)

	// Create a stopped EC2 instance (potential waste).
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:instance/i-0mock1234567890", "AWS::EC2::Instance", map[string]interface{}{
		"State":      "stopped",
		"LaunchTime": time.Now().Add(-60 * 24 * time.Hour), // 60 days old
	})

	// Create an unattached EBS volume.
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

	// Create an unused volume that is technically "in-use" but by a zombie resource.
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockPseudoUse", "AWS::EC2::Volume", map[string]interface{}{
		"State":               "in-use",
		"AttachedInstanceId":  "i-0mock1234567890",
		"DeleteOnTermination": false,
	})

	// Create an idle NAT Gateway.
	natArn := "arn:aws:ec2:us-east-1:123456789012:natgateway/nat-0mock12345"
	s.Graph.AddNode(natArn, "aws_nat_gateway", map[string]interface{}{
		"State":            "available",
		"SumConnections7d": 0.0,
		"ActiveUserENIs":   0,
		"EmptySubnets":     []string{"subnet-mock-empty-1", "subnet-mock-empty-2"},
		"Region":           "us-east-1",
	})

	// Create a snapshot linked to the waste volume.
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:snapshot/snap-0mockChild", "AWS::EC2::Snapshot", map[string]interface{}{
		"State":      "completed",
		"VolumeId":   "vol-0mock1234567890", // Links to waste vol
		"VolumeSize": 100,
	})

	// Create a properly configured Elastic IP (Safe).
	eipArn := "arn:aws:ec2:us-east-1:123456789012:eip/eipalloc-0mock123"
	s.Graph.AddNode(eipArn, "aws_eip", map[string]interface{}{
		"PublicIp":      "203.0.113.10",
		"Region":        "us-east-1",
		"AssociationId": "",    // Unattached
		"FoundInDNS":    false, // Safe
	})

	// Create an unused Elastic IP (Waste).
	eipDangerArn := "arn:aws:ec2:us-east-1:123456789012:eip/eipalloc-0mockDanger"
	s.Graph.AddNode(eipDangerArn, "aws_eip", map[string]interface{}{
		"PublicIp":      "203.0.113.99",
		"Region":        "us-east-1",
		"AssociationId": "",
		"FoundInDNS":    true,
		"DNSZone":       "production.com",
	})

	// Create an S3 bucket with incomplete multipart uploads.
	s.Graph.AddNode("arn:aws:s3:::mock-bucket-iceberg", "AWS::S3::Bucket", map[string]interface{}{
		"Name":              "mock-bucket-iceberg",
		"HasAbortLifecycle": false,
	})
	s.Graph.AddNode("arn:aws:s3:::multipart/mock-bucket-iceberg/upload-1", "AWS::S3::MultipartUpload", map[string]interface{}{
		"Initiated": time.Now().Add(-15 * 24 * time.Hour), // 15 days old
		"Bucket":    "mock-bucket-iceberg",
	})

	// Create a legacy gp2 volume.
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockGp2", "AWS::EC2::Volume", map[string]interface{}{
		"State":       "in-use",
		"Size":        int32(500), // 500GB
		"VolumeType":  "gp2",
		"IsModifying": false,
		"Region":      "us-east-1",
	})

	// Create an aged AMI.
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:image/ami-0mockAged", "AWS::EC2::AMI", map[string]interface{}{
		"Name":         "legacy-server-backup-2023",
		"State":        "available",
		"CreationDate": time.Now().Add(-100 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z"),
		"CreateTime":   time.Now().Add(-100 * 24 * time.Hour), // 100 days old
	})

	// Create an AMI explicitly ignored via tags.
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:image/ami-0mockIgnoreTrue", "AWS::EC2::AMI", map[string]interface{}{
		"Name":         "important-backup",
		"State":        "available",
		"CreationDate": time.Now().Add(-200 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z"),
		"CreateTime":   time.Now().Add(-200 * 24 * time.Hour), // 200 days old
		"Tags": map[string]string{
			"cloudslash:ignore": "true",
		},
	})

	// Ignored AMI (Pass).
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:image/ami-0mockIgnoreDurationPass", "AWS::EC2::AMI", map[string]interface{}{
		"Name":         "semi-old-backup",
		"State":        "available",
		"CreationDate": time.Now().Add(-100 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z"),
		"CreateTime":   time.Now().Add(-100 * 24 * time.Hour),
		"Tags": map[string]string{
			"cloudslash:ignore": "120d",
		},
	})

	// Ignored AMI (Fail).
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:image/ami-0mockIgnoreDurationFail", "AWS::EC2::AMI", map[string]interface{}{
		"Name":         "expired-backup",
		"State":        "available",
		"CreationDate": time.Now().Add(-100 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z"),
		"CreateTime":   time.Now().Add(-100 * 24 * time.Hour),
		"Tags": map[string]string{
			"cloudslash:ignore": "30d",
		},
	})

	// Orphaned ELB.

	// ... (Truncating for clarity, keeping required context)

	// Create an orphaned Load Balancer attached to a deleted cluster.
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

	// Create a phantom EKS Node Group.
	s.Graph.AddNode("arn:aws:eks:us-east-1:123456789012:nodegroup/production-cluster/failed-scaling-group", "AWS::EKS::NodeGroup", map[string]interface{}{
		"NodegroupName":     "failed-scaling-group",
		"ClusterName":       "production-cluster",
		"Status":            "ACTIVE",
		"NodeCount":         5, // BILLING
		"RealWorkloadCount": 0, // DOING NOTHING
		"Region":            "us-east-1",
	})

	// Create a stopped RDS instance.
	s.Graph.AddNode("arn:aws:rds:us-east-1:123456789012:db:legacy-postgres", "AWS::RDS::DBInstance", map[string]interface{}{
		"DBInstanceIdentifier": "legacy-postgres",
		"Status":               "stopped",
		"Region":               "us-east-1",
	})
	// RDSHeuristic handles stopped instances without CloudWatch metrics.

	// Create an unused Application Load Balancer.
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

	// Create an oversized EC2 instance.
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

	// Create a stale multipart upload.
	s.Graph.AddNode("arn:aws:s3:::mock-bucket/upload-1", "AWS::S3::MultipartUpload", map[string]interface{}{
		"Initiated": time.Now().Add(-10 * 24 * time.Hour), // 10 days old
	})

	// Create an idle ECS Cluster.
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
	// Create a container instance for the idle cluster.
	s.Graph.AddNode(clusterArn+"/container-instance/ci-mock-1", "AWS::ECS::ContainerInstance", map[string]interface{}{
		"ClusterArn":   clusterArn,
		"RegisteredAt": time.Now().Add(-24 * time.Hour),
		"Status":       "ACTIVE",
	})
	s.Graph.AddTypedEdge(clusterArn, clusterArn+"/container-instance/ci-mock-1", graph.EdgeType("HAS_INSTANCE"), 1)

	// Create a service that fails to place tasks (Empty Service).
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
	// Create a placeholder cluster node for linking.
	frontendClusterArn := "arn:aws:ecs:us-east-1:123456789012:cluster/frontend-cluster"
	s.Graph.AddNode(frontendClusterArn, "AWS::ECS::Cluster", map[string]interface{}{
		"Name": "frontend-cluster",
	})
	s.Graph.AddTypedEdge(frontendClusterArn, serviceArn, graph.EdgeTypeContains, 1)

	// Create a healthy ECS topology for contrast.
	prodMockCluster := "arn:aws:ecs:us-east-1:123456789012:cluster/production-cluster"
	s.Graph.AddNode(prodMockCluster, "AWS::ECS::Cluster", map[string]interface{}{
		"Name":                "production-cluster",
		"Status":              "ACTIVE",
		"RunningTasksCount":   15,
		"ActiveServicesCount": 2,
	})

	// Create a healthy Service (A).
	authSvc := "arn:aws:ecs:us-east-1:123456789012:service/production-cluster/auth-service"
	s.Graph.AddNode(authSvc, "AWS::ECS::Service", map[string]interface{}{
		"Name":         "auth-service",
		"DesiredCount": 5,
		"RunningCount": 5,
		"Status":       "ACTIVE",
	})
	s.Graph.AddTypedEdge(prodMockCluster, authSvc, graph.EdgeTypeContains, 1)

	// Create a healthy Task (A1).
	taskA1 := "arn:aws:ecs:us-east-1:123456789012:task/production-cluster/auth-task-uuid-1"
	s.Graph.AddNode(taskA1, "AWS::ECS::Task", map[string]interface{}{
		"Name":           "auth-task-uuid-1",
		"LastStatus":     "RUNNING",
		"DesiredStatus":  "RUNNING",
		"TaskDefinition": "arn:aws:ecs:task-def/auth:1",
	})
	s.Graph.AddTypedEdge(authSvc, taskA1, graph.EdgeTypeRuns, 1)

	// Create a healthy Service (B).
	paymentSvc := "arn:aws:ecs:us-east-1:123456789012:service/production-cluster/payment-service"
	s.Graph.AddNode(paymentSvc, "AWS::ECS::Service", map[string]interface{}{
		"Name":         "payment-service",
		"DesiredCount": 10,
		"RunningCount": 10,
		"Status":       "ACTIVE",
	})
	s.Graph.AddTypedEdge(prodMockCluster, paymentSvc, graph.EdgeTypeContains, 1)

	// Create a volume ignored by tag.
	s.Graph.AddNode("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockIGNORED", "AWS::EC2::Volume", map[string]interface{}{
		"State": "available",
		"Tags": map[string]string{
			"cloudslash:ignore": "true",
		},
	})
	s.Graph.MarkWaste("arn:aws:ec2:us-east-1:123456789012:volume/vol-0mockIGNORED", 100)
	// Set costs.

	// Scenario 10: Monolith Fleet.
	// Simulate 5x m5.large instances running a legacy app.
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
		if node != nil {
			s.Graph.Mu.Lock()
			node.Cost = 70.08 // roughly monthly
			s.Graph.Mu.Unlock()
		}
	}

	// Scenario 11: High Performance Computing (HPC) Simulation.
	// Simulate 2x c5.4xlarge instances.
	for i := 0; i < 2; i++ {
		arn := fmt.Sprintf("arn:aws:ec2:us-east-1:123456789012:instance/i-0mockHPC-%d", i)
		s.Graph.AddNode(arn, "AWS::EC2::Instance", map[string]interface{}{
			"InstanceType": "c5.4xlarge",
			"State":        "running",
			"Region":       "us-east-1",
			"Zone":         "us-east-1b", // Safer zone
		})
		node := s.Graph.GetNode(arn)
		if node != nil {
			s.Graph.Mu.Lock()
			node.Cost = 600.00
			s.Graph.Mu.Unlock()
		}
	}

	return nil
}
