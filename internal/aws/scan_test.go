package aws

import (
	"context"
	"testing"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// MockEC2Client implements EC2Client for testing.
type MockEC2Client struct {
	DescribeVolumesFunc func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	// Add other mock functions if needed
}

func (m *MockEC2Client) DescribeVolumesModifications(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error) {
	return &ec2.DescribeVolumesModificationsOutput{}, nil
}

func (m *MockEC2Client) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	if m.DescribeVolumesFunc != nil {
		return m.DescribeVolumesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeVolumesOutput{}, nil
}

// Stubs for other interface methods
func (m *MockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}
func (m *MockEC2Client) DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return &ec2.DescribeNatGatewaysOutput{}, nil
}
func (m *MockEC2Client) DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{}, nil
}

func (m *MockEC2Client) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{}, nil
}

func (m *MockEC2Client) DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return &ec2.DescribeSnapshotsOutput{}, nil
}

func TestScanVolumes(t *testing.T) {
	tests := []struct {
		name          string
		volumes       []types.Volume
		wantNodeCount int
		checkNode     func(*testing.T, *graph.Graph)
	}{
		{
			name: "Zombie Volume",
			volumes: []types.Volume{
				{
					VolumeId:   aws.String("vol-zombie"),
					State:      types.VolumeStateAvailable,
					Size:       aws.Int32(50),
					CreateTime: aws.Time(time.Now()),
				},
			},
			wantNodeCount: 1,
			checkNode: func(t *testing.T, g *graph.Graph) {
				g.Mu.RLock()
				defer g.Mu.RUnlock()
				node, ok := g.Nodes["arn:aws:ec2:region:account:volume/vol-zombie"]
				if !ok {
					t.Fatal("Zombie volume not found in graph")
				}
				if node.Properties["State"] != "available" {
					t.Errorf("Expected state available, got %v", node.Properties["State"])
				}
			},
		},
		{
			name: "Clean Volume",
			volumes: []types.Volume{
				{
					VolumeId:   aws.String("vol-inuse"),
					State:      types.VolumeStateInUse,
					Size:       aws.Int32(100),
					CreateTime: aws.Time(time.Now()),
					Attachments: []types.VolumeAttachment{
						{
							InstanceId:          aws.String("i-12345"),
							State:               types.VolumeAttachmentStateAttached,
							DeleteOnTermination: aws.Bool(true),
						},
					},
				},
			},
			wantNodeCount: 2, // Volume + attached Instance placeholder
			checkNode: func(t *testing.T, g *graph.Graph) {
				g.Mu.RLock()
				defer g.Mu.RUnlock()
				node, ok := g.Nodes["arn:aws:ec2:region:account:volume/vol-inuse"]
				if !ok {
					t.Fatal("Clean volume not found in graph")
				}
				// Verify edge to instance exists
				foundEdge := false
				edges, _ := g.Edges[node.ID]
				for _, edge := range edges {
					if edge.TargetID == "arn:aws:ec2:region:account:instance/i-12345" {
						foundEdge = true
						break
					}
				}
				if !foundEdge {
					t.Error("Expected edge to instance i-12345")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.NewGraph()
			mock := &MockEC2Client{
				DescribeVolumesFunc: func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
					return &ec2.DescribeVolumesOutput{
						Volumes: tt.volumes,
					}, nil
				},
			}

			scanner := &EC2Scanner{
				Client: mock,
				Graph:  g,
			}

			err := scanner.ScanVolumes(context.Background())
			if err != nil {
				t.Fatalf("ScanVolumes failed: %v", err)
			}

			if len(g.Nodes) != tt.wantNodeCount {
				t.Errorf("Expected %d nodes, got %d", tt.wantNodeCount, len(g.Nodes))
			}

			if tt.checkNode != nil {
				tt.checkNode(t, g)
			}
		})
	}
}
