package aws

import (
	"context"
	"testing"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/DrSkyle/cloudslash/v2/pkg/resource"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockEC2Client implements EC2Client for testing purposes.
type mockEC2Client struct {
	// Hooks to inject custom behavior
	DescribeInstancesFunc     func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeInstanceTypesFunc func(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)
	DescribeVolumesFunc       func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.DescribeInstancesFunc != nil {
		return m.DescribeInstancesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

func (m *mockEC2Client) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	if m.DescribeVolumesFunc != nil {
		return m.DescribeVolumesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeVolumesOutput{}, nil
}

func (m *mockEC2Client) DescribeInstanceTypes(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
	if m.DescribeInstanceTypesFunc != nil {
		return m.DescribeInstanceTypesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstanceTypesOutput{}, nil
}

// Stubs for other interface methods
func (m *mockEC2Client) DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return &ec2.DescribeNatGatewaysOutput{}, nil
}
func (m *mockEC2Client) DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{}, nil
}
func (m *mockEC2Client) DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return &ec2.DescribeSnapshotsOutput{}, nil
}
func (m *mockEC2Client) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{}, nil
}
func (m *mockEC2Client) DescribeVolumesModifications(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error) {
	return &ec2.DescribeVolumesModificationsOutput{}, nil
}

func TestEC2Scanner_ScanInstances_Mocked(t *testing.T) {
	g := graph.NewGraph()
	mockClient := &mockEC2Client{
		DescribeInstancesFunc: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:   aws.String("i-mock123"),
								InstanceType: types.InstanceType("m5.large"),
								State: &types.InstanceState{
									Name: types.InstanceStateNameRunning,
								},
								LaunchTime: aws.Time(time.Now().Add(-24 * time.Hour)),
								VpcId:      aws.String("vpc-123"),
								Tags: []types.Tag{
									{Key: aws.String("Name"), Value: aws.String("Production-Web")},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	scanner := &EC2Scanner{
		Client: mockClient,
		Graph:  g,
	}

	// Run Scan
	if err := scanner.ScanInstances(context.Background()); err != nil {
		t.Fatalf("ScanInstances failed: %v", err)
	}

	// Wait for graph builder to finish processing
	g.CloseAndWait()

	// Verification
	// 1. Check Node Creation
	nodeID := "arn:aws:ec2:region:account:instance/i-mock123"
	node := g.GetNode(nodeID)
	if node == nil {
		t.Fatalf("Expected node %s not found in graph", nodeID)
	}

	// 2. Check Translation
	if node.TypeStr() != "AWS::EC2::Instance" {
		t.Errorf("Expected Type AWS::EC2::Instance, got %s", node.TypeStr())
	}
	if node.Properties["State"] != "running" {
		t.Errorf("Expected State running, got %v", node.Properties["State"])
	}
	if node.Properties["Type"] != "m5.large" {
		t.Errorf("Expected Type m5.large, got %v", node.Properties["Type"])
	}

	// 3. Strict Type Verification (Dual-Write Check)
	if node.TypedData == nil {
		t.Fatal("TypedData is nil! Dual-write failed.")
	}
	ec2Node, ok := node.TypedData.(*resource.EC2Instance)
	if !ok {
		t.Fatal("TypedData is not *resource.EC2Instance")
	}
	if ec2Node.InstanceType != "m5.large" {
		t.Errorf("TypedData.InstanceType mismatch: got %s, want m5.large", ec2Node.InstanceType)
	}
	if ec2Node.VpcID != "vpc-123" {
		t.Errorf("TypedData.VpcID mismatch: got %s, want vpc-123", ec2Node.VpcID)
	}

	// 4. Check Topology (Edge to VPC)
	vpcNodeID := "arn:aws:ec2:region:account:vpc/vpc-123"
	if g.GetNode(vpcNodeID) == nil {
		t.Error("Synthesized VPC node not found")
	}

	// Verify edge exists
	// We can't easily check edges in current graph API without iterating,
	// but we can check if upstream of node includes vpc (or downstream of vpc includes node)
	// Graph.AddTypedEdge(vpcARN, arn, graph.EdgeTypeContains, 100)
	// So VPC --CONTAINS--> Instance

	downstream := g.GetDownstream(vpcNodeID)
	found := false
	for _, childID := range downstream {
		if childID == nodeID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected edge VPC -> Instance not found")
	}
}

func TestEC2Scanner_ScanVolumes_Mocked(t *testing.T) {
	g := graph.NewGraph()
	mockClient := &mockEC2Client{
		DescribeVolumesFunc: func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			return &ec2.DescribeVolumesOutput{
				Volumes: []types.Volume{
					{
						VolumeId:   aws.String("vol-mock999"),
						Size:       aws.Int32(100),
						State:      types.VolumeStateAvailable,
						VolumeType: types.VolumeTypeGp3,
						CreateTime: aws.Time(time.Now()),
						Tags: []types.Tag{
							{Key: aws.String("Env"), Value: aws.String("Dev")},
						},
					},
				},
			}, nil
		},
	}

	scanner := &EC2Scanner{
		Client: mockClient,
		Graph:  g,
	}

	if err := scanner.ScanVolumes(context.Background()); err != nil {
		t.Fatalf("ScanVolumes failed: %v", err)
	}

	g.CloseAndWait()

	volID := "arn:aws:ec2:region:account:volume/vol-mock999"
	node := g.GetNode(volID)
	if node == nil {
		t.Fatalf("Expected volume node %s not found", volID)
	}

	if node.Properties["Size"] != int32(100) {
		t.Errorf("Expected Size 100, got %v", node.Properties["Size"])
	}
	if node.Properties["State"] != "available" {
		t.Errorf("Expected State available, got %v", node.Properties["State"])
	}
}
