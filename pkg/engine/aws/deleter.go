package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// Deleter handles destructive actions.
type Deleter struct {
	EC2 *ec2.Client
}

func NewDeleter(cfg aws.Config) *Deleter {
	return &Deleter{
		EC2: ec2.NewFromConfig(cfg),
	}
}

// DeleteVolume deletes an EBS volume.
func (d *Deleter) DeleteVolume(ctx context.Context, id string) error {
	// Parse ID from ARN if needed
	// arn:aws:ec2:region:account:volume/vol-123
	if strings.HasPrefix(id, "arn:") {
		parts := strings.Split(id, "/")
		id = parts[len(parts)-1]
	}

	_, err := d.EC2.DeleteVolume(ctx, &ec2.DeleteVolumeInput{
		VolumeId: aws.String(id),
	})
	return err
}
