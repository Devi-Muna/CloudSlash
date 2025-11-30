package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/saujanyayaya/cloudslash/internal/graph"
)

type EC2Scanner struct {
	Client *ec2.Client
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
				}

				s.Graph.AddNode(arn, "AWS::EC2::Instance", props)

				// Link to VPC
				if instance.VpcId != nil {
					vpcARN := fmt.Sprintf("arn:aws:ec2:region:account:vpc/%s", *instance.VpcId)
					s.Graph.AddEdge(arn, vpcARN)
				}

				// Link to Subnet
				if instance.SubnetId != nil {
					subnetARN := fmt.Sprintf("arn:aws:ec2:region:account:subnet/%s", *instance.SubnetId)
					s.Graph.AddEdge(arn, subnetARN)
				}

				// Link to Security Groups
				for _, sg := range instance.SecurityGroups {
					sgARN := fmt.Sprintf("arn:aws:ec2:region:account:security-group/%s", *sg.GroupId)
					s.Graph.AddEdge(arn, sgARN)
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

		for _, volume := range page.Volumes {
			id := *volume.VolumeId
			arn := fmt.Sprintf("arn:aws:ec2:region:account:volume/%s", id)

			props := map[string]interface{}{
				"State":      string(volume.State),
				"Size":       *volume.Size,
				"CreateTime": volume.CreateTime,
			}

			s.Graph.AddNode(arn, "AWS::EC2::Volume", props)

			// Link to Attachments
			for _, att := range volume.Attachments {
				if att.InstanceId != nil {
					instanceARN := fmt.Sprintf("arn:aws:ec2:region:account:instance/%s", *att.InstanceId)
					s.Graph.AddEdge(arn, instanceARN)

					// Store attachment info in properties for heuristics
					props["DeleteOnTermination"] = att.DeleteOnTermination
					props["AttachedInstanceId"] = *att.InstanceId // Store ID for easy lookup
				}
			}
		}
	}
	return nil
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
