package aws

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
)

type EKSClient interface {
	ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error)
	DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
	ListFargateProfiles(ctx context.Context, params *eks.ListFargateProfilesInput, optFns ...func(*eks.Options)) (*eks.ListFargateProfilesOutput, error)
	DescribeFargateProfile(ctx context.Context, params *eks.DescribeFargateProfileInput, optFns ...func(*eks.Options)) (*eks.DescribeFargateProfileOutput, error)
}

// EKSEC2Client defines required EC2 operations.
type EKSEC2Client interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

type EKSScanner struct {
	Client    EKSClient
	EC2Client EKSEC2Client // Interface for testing
	Graph     *graph.Graph
}

func NewEKSScanner(cfg aws.Config, g *graph.Graph) *EKSScanner {
	return &EKSScanner{
		Client:    eks.NewFromConfig(cfg),
		EC2Client: ec2.NewFromConfig(cfg),
		Graph:     g,
	}
}

func (s *EKSScanner) ScanClusters(ctx context.Context) error {
	paginator := eks.NewListClustersPaginator(s.Client, &eks.ListClustersInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list eks clusters: %v", err)
		}

		for _, clusterName := range page.Clusters {
			if err := s.processCluster(ctx, clusterName); err != nil {
				// Log non-fatal errors.
				fmt.Printf("Warning: failed to process cluster %s: %v\n", clusterName, err)
			}
		}
	}
	return nil
}

func (s *EKSScanner) processCluster(ctx context.Context, name string) error {
	resp, err := s.Client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &name})
	if err != nil {
		return err
	}
	cluster := resp.Cluster

	// Filter for active clusters.
	if cluster.Status != types.ClusterStatusActive {
		return nil
	}

	arn := *cluster.Arn

	// Check for managed node groups.
	hasManagedNodes, err := s.checkManagedNodes(ctx, name)
	if err != nil {
		return err
	}

	// Check Fargate profiles.
	hasFargate, err := s.scanFargateProfiles(ctx, name, arn)
	if err != nil {
		return err
	}

	// Check self-managed nodes.
	hasSelfManaged, err := s.checkSelfManagedNodes(ctx, name)
	if err != nil {
		return err
	}

	// Detect Karpenter installation.
	karpenterEnabled := false
	if cluster.Tags != nil {
		if _, ok := cluster.Tags["karpenter.sh/discovery"]; ok {
			karpenterEnabled = true
		}
	}

	props := map[string]interface{}{
		"Name":                name,
		"Status":              string(cluster.Status),
		"CreatedAt":           cluster.CreatedAt,
		"HasManagedNodes":     hasManagedNodes,
		"HasFargate":          hasFargate,
		"HasSelfManagedNodes": hasSelfManaged,
		"KarpenterEnabled":    karpenterEnabled,
		"Tags":                cluster.Tags,
	}

	s.Graph.AddNode(arn, "AWS::EKS::Cluster", props)
	return nil
}

func (s *EKSScanner) checkManagedNodes(ctx context.Context, clusterName string) (bool, error) {
	// Check for active nodegroups.
	paginator := eks.NewListNodegroupsPaginator(s.Client, &eks.ListNodegroupsInput{ClusterName: &clusterName})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return false, err
		}

		for _, ngName := range page.Nodegroups {
			ng, err := s.Client.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   &clusterName,
				NodegroupName: &ngName,
			})
			if err != nil {
				return false, err
			}

			if ng.Nodegroup != nil && ng.Nodegroup.ScalingConfig != nil {
				if ng.Nodegroup.ScalingConfig.DesiredSize != nil && *ng.Nodegroup.ScalingConfig.DesiredSize > 0 {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func (s *EKSScanner) scanFargateProfiles(ctx context.Context, clusterName, clusterARN string) (bool, error) {
	paginator := eks.NewListFargateProfilesPaginator(s.Client, &eks.ListFargateProfilesInput{ClusterName: &clusterName})

	hasProfiles := false

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return false, err
		}

		if len(page.FargateProfileNames) > 0 {
			hasProfiles = true
		}

		for _, profileName := range page.FargateProfileNames {
			resp, err := s.Client.DescribeFargateProfile(ctx, &eks.DescribeFargateProfileInput{
				ClusterName:        &clusterName,
				FargateProfileName: &profileName,
			})
			if err != nil {
				// Log warning and continue.
				fmt.Printf("   [Warning] Failed to describe Fargate Profile %s: %v\n", profileName, err)
				continue
			}

			fp := resp.FargateProfile
			if fp == nil {
				continue
			}

			// Add profile to graph.
			props := map[string]interface{}{
				"ProfileName": *fp.FargateProfileName,
				"ClusterName": clusterName,
				"ClusterARN":  clusterARN,
				"CreatedAt":   fp.CreatedAt,
				"Selectors":   fp.Selectors,
			}

			s.Graph.AddNode(*fp.FargateProfileArn, "AWS::EKS::FargateProfile", props)

			// Link profile to cluster.
			s.Graph.AddEdge(*fp.FargateProfileArn, clusterARN)
		}
	}
	return hasProfiles, nil
}

func (s *EKSScanner) checkSelfManagedNodes(ctx context.Context, clusterName string) (bool, error) {
	// Filter instances by cluster ownership tag.
	key := fmt.Sprintf("tag:kubernetes.io/cluster/%s", clusterName)

	input := &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String(key), Values: []string{"owned", "shared"}},
			{Name: aws.String("instance-state-name"), Values: []string{"running", "pending"}},
		},
	}

	// Check for at least one matching instance.
	paginator := ec2.NewDescribeInstancesPaginator(s.EC2Client, input)

	if paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return false, err
		}
		for _, r := range page.Reservations {
			if len(r.Instances) > 0 {
				return true, nil
			}
		}
	}

	return false, nil
}
