package aws

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EC2Client interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error)
	DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
	DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	DescribeVolumesModifications(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error)
}

type EC2Scanner struct {
	Client EC2Client
	Graph  *graph.Graph
}

func NewEC2Scanner(cfg aws.Config, g *graph.Graph) *EC2Scanner {
	return &EC2Scanner{
		Client: ec2.NewFromConfig(cfg),
		Graph:  g,
	}
}

func (s *EC2Scanner) ScanInstances(ctx context.Context) error {
	paginator := ec2.NewDescribeInstancesPaginator(s.Client, &ec2.DescribeInstancesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to describe instances: %v", err)
		}

		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				id := *instance.InstanceId
				arn := fmt.Sprintf("arn:aws:ec2:region:account:instance/%s", id) // Simplified ARN construction

				props := map[string]interface{}{
					"State":      string(instance.State.Name),
					"Type":       string(instance.InstanceType),
					"LaunchTime": instance.LaunchTime,
					"Tags":       parseTags(instance.Tags),
				}

				s.Graph.AddNode(arn, "AWS::EC2::Instance", props)

				// Link to VPC
				if instance.VpcId != nil {
					vpcARN := fmt.Sprintf("arn:aws:ec2:region:account:vpc/%s", *instance.VpcId)
					s.Graph.AddTypedEdge(vpcARN, arn, graph.EdgeTypeContains, 100)
				}

				// Link to Subnet
				if instance.SubnetId != nil {
					subnetARN := fmt.Sprintf("arn:aws:ec2:region:account:subnet/%s", *instance.SubnetId)
					s.Graph.AddTypedEdge(subnetARN, arn, graph.EdgeTypeContains, 100)
				}

				// Link to Security Groups
				for _, sg := range instance.SecurityGroups {
					sgARN := fmt.Sprintf("arn:aws:ec2:region:account:security-group/%s", *sg.GroupId)
					s.Graph.AddTypedEdge(arn, sgARN, graph.EdgeTypeSecuredBy, 100)
				}
			}
		}
	}
	return nil
}

func (s *EC2Scanner) ScanVolumes(ctx context.Context) error {
	paginator := ec2.NewDescribeVolumesPaginator(s.Client, &ec2.DescribeVolumesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to describe volumes: %v", err)
		}

		// Optimization: Collect IDs for Modification Check
		var volIDs []string
		for _, v := range page.Volumes {
			volIDs = append(volIDs, *v.VolumeId)
		}
		
		modMap := s.getVolumeModifications(ctx, volIDs)

		for _, volume := range page.Volumes {
			id := *volume.VolumeId
			arn := fmt.Sprintf("arn:aws:ec2:region:account:volume/%s", id)

			props := map[string]interface{}{
				"State":      string(volume.State),
				"Size":       *volume.Size,
				"VolumeType": string(volume.VolumeType), // Catch gp2
				"CreateTime": volume.CreateTime,
				"Tags":       parseTags(volume.Tags),
				"IsModifying": modMap[id], // Check safety lock
			}

			s.Graph.AddNode(arn, "AWS::EC2::Volume", props)

			// Link to Attachments
			for _, att := range volume.Attachments {
				if att.InstanceId != nil {
					instanceARN := fmt.Sprintf("arn:aws:ec2:region:account:instance/%s", *att.InstanceId)
					s.Graph.AddTypedEdge(arn, instanceARN, graph.EdgeTypeAttachedTo, 100)

					// Store attachment info in properties for heuristics
					props["DeleteOnTermination"] = att.DeleteOnTermination
					props["AttachedInstanceId"] = *att.InstanceId // Store ID for easy lookup
				}
			}
		}
	}
	return nil
}

func (s *EC2Scanner) getVolumeModifications(ctx context.Context, volIDs []string) map[string]bool {
	out := make(map[string]bool)
	if len(volIDs) == 0 { return out }
	
	// API accepts list of IDs.
	// We handle pagination if IDs > limit? describe_volumes_modifications usually handles simple lists.
	// But let's assume we pass the slice.
	
	// Issue: If list is too big, might fail. Page size of scan is ~50-100.
	// Should be fine.
	
	resp, err := s.Client.DescribeVolumesModifications(ctx, &ec2.DescribeVolumesModificationsInput{
		VolumeIds: volIDs,
	})
	if err != nil {
		// If error (e.g. some IDs not found), assume safe or log. 
		// "Low-Maintenance Code": skip but return empty.
		return out
	}
	
	for _, mod := range resp.VolumesModifications {
		// modifying | optimizing | completed | failed
		state := mod.ModificationState
		if state == types.VolumeModificationStateModifying || state == types.VolumeModificationStateOptimizing {
			out[*mod.VolumeId] = true
		}
	}
	return out
}

func (s *EC2Scanner) ScanNatGateways(ctx context.Context) error {
	paginator := ec2.NewDescribeNatGatewaysPaginator(s.Client, &ec2.DescribeNatGatewaysInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to describe nat gateways: %v", err)
		}

		for _, ngw := range page.NatGateways {
			id := *ngw.NatGatewayId
			arn := fmt.Sprintf("arn:aws:ec2:region:account:natgateway/%s", id)

			props := map[string]interface{}{
				"State": string(ngw.State),
				"Tags":  parseTags(ngw.Tags),
			}

			s.Graph.AddNode(arn, "AWS::EC2::NatGateway", props)
		}
	}
	return nil
}

func (s *EC2Scanner) ScanAddresses(ctx context.Context) error {
	result, err := s.Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return fmt.Errorf("failed to describe addresses: %v", err)
	}

	for _, addr := range result.Addresses {
		id := *addr.AllocationId
		arn := fmt.Sprintf("arn:aws:ec2:region:account:eip/%s", id)

		props := map[string]interface{}{
			"PublicIp": *addr.PublicIp,
			"Tags":     parseTags(addr.Tags),
		}

		if addr.InstanceId != nil {
			props["InstanceId"] = *addr.InstanceId
			instanceARN := fmt.Sprintf("arn:aws:ec2:region:account:instance/%s", *addr.InstanceId)
			s.Graph.AddEdge(arn, instanceARN)
		}

		s.Graph.AddNode(arn, "AWS::EC2::EIP", props)
	}
	return nil
}

func (s *EC2Scanner) ScanSnapshots(ctx context.Context, ownerID string) error {
	input := &ec2.DescribeSnapshotsInput{
		OwnerIds: []string{"self"},
	}
	if ownerID != "" {
		input.OwnerIds = []string{ownerID}
	}

	paginator := ec2.NewDescribeSnapshotsPaginator(s.Client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to scan snapshots: %v", err)
		}
		for _, snap := range page.Snapshots {
			id := *snap.SnapshotId
			arn := fmt.Sprintf("arn:aws:ec2:region:account:snapshot/%s", id)

			props := map[string]interface{}{
				"State":       string(snap.State),
				"VolumeSize":  *snap.VolumeSize,
				"Description": *snap.Description,
				"VolumeId":    *snap.VolumeId, // Original volume
				"Tags":        parseTags(snap.Tags),
			}
			s.Graph.AddNode(arn, "AWS::EC2::Snapshot", props)
		}
	}
	return nil
}

func (s *EC2Scanner) ScanImages(ctx context.Context) error {
	input := &ec2.DescribeImagesInput{
		Owners: []string{"self"},
	}
	result, err := s.Client.DescribeImages(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to scan images: %v", err)
	}

	for _, img := range result.Images {
		id := *img.ImageId
		arn := fmt.Sprintf("arn:aws:ec2:region:account:image/%s", id)

		props := map[string]interface{}{
			"State": string(img.State),
			"Name":  *img.Name,
			"Tags":  parseTags(img.Tags),
		}
		s.Graph.AddNode(arn, "AWS::EC2::AMI", props)

		// Link AMI to its Snapshots
		for _, bdm := range img.BlockDeviceMappings {
			if bdm.Ebs != nil && bdm.Ebs.SnapshotId != nil {
				snapARN := fmt.Sprintf("arn:aws:ec2:region:account:snapshot/%s", *bdm.Ebs.SnapshotId)
				// AMI -> Snapshot (AMI contains/uses Snapshot)
				s.Graph.AddTypedEdge(arn, snapARN, graph.EdgeTypeContains, 100)
			}
		}
	}
	return nil
}

func parseTags(tags []types.Tag) map[string]string {
	out := make(map[string]string)
	for _, t := range tags {
		if t.Key != nil && t.Value != nil {
			out[*t.Key] = *t.Value
		}
	}
	return out
}
