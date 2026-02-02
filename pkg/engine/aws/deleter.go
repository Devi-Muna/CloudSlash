package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Deleter handles resource removal.
type Deleter struct {
	EC2 *ec2.Client
}

func NewDeleter(cfg aws.Config) *Deleter {
	return &Deleter{
		EC2: ec2.NewFromConfig(cfg),
	}
}

// CreateSnapshot creates a final snapshot before deletion.
func (d *Deleter) CreateSnapshot(ctx context.Context, volID, desc string) (string, error) {
	// Parse ID if ARN is provided.
	if strings.HasPrefix(volID, "arn:") {
		parts := strings.Split(volID, "/")
		volID = parts[len(parts)-1]
	}

	resp, err := d.EC2.CreateSnapshot(ctx, &ec2.CreateSnapshotInput{
		VolumeId:    aws.String(volID),
		Description: aws.String(desc),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSnapshot,
				Tags: []types.Tag{
					{Key: aws.String("CreatedBy"), Value: aws.String("CloudSlash")},
					{Key: aws.String("SourceVolume"), Value: aws.String(volID)},
				},
			},
		},
	})
	if err != nil {
		return "", err
	}
	return *resp.SnapshotId, nil
}

// DeleteVolume deletes an EBS volume.
func (d *Deleter) DeleteVolume(ctx context.Context, id string) error {
	// Handle ARN.
	if strings.HasPrefix(id, "arn:") {
		parts := strings.Split(id, "/")
		id = parts[len(parts)-1]
	}

	_, err := d.EC2.DeleteVolume(ctx, &ec2.DeleteVolumeInput{
		VolumeId: aws.String(id),
	})
	return err
}
