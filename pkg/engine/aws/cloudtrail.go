package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
)

// CloudTrailClient queries audit trails.
type CloudTrailClient struct {
	Client *cloudtrail.Client
}

func NewCloudTrailClient(cfg aws.Config) *CloudTrailClient {
	return &CloudTrailClient{
		Client: cloudtrail.NewFromConfig(cfg),
	}
}

// LookupCreator searches CloudTrail for the resource creator (90 days).
func (c *CloudTrailClient) LookupCreator(ctx context.Context, resourceID string) (string, error) {
	// 90-day window.
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -90)

	// Lookup params.
	attrKey := types.LookupAttributeKeyResourceName

	input := &cloudtrail.LookupEventsInput{
		LookupAttributes: []types.LookupAttribute{
			{
				AttributeKey:   attrKey,
				AttributeValue: aws.String(resourceID),
			},
		},
		StartTime:  &startTime,
		EndTime:    &endTime,
		MaxResults: aws.Int32(50),
	}

	// Query CloudTrail.
	paginator := cloudtrail.NewLookupEventsPaginator(c.Client, input)

	// Process first page.
	if paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return "", err
		}

		for _, event := range output.Events {
			// Check for resource creation events.
			eventName := aws.ToString(event.EventName)
			if isCreationEvent(eventName) {
				return aws.ToString(event.Username), nil
			}
		}
	}

	return "", fmt.Errorf("creator not found in CloudTrail (90 days)")
}

func isCreationEvent(name string) bool {
	switch name {
	case "RunInstances", "CreateVolume", "CreateBucket", "CreateDBInstance", "CreateLoadBalancer", "CreateLoadBalancerV2":
		return true
	}
	return false
}
