package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
)

type CloudTrailClient struct {
	Client *cloudtrail.Client
}

func NewCloudTrailClient(cfg aws.Config) *CloudTrailClient {
	return &CloudTrailClient{
		Client: cloudtrail.NewFromConfig(cfg),
	}
}

// LookupCreator attempts to find the IAM identity that created a resource.
// It searches CloudTrail events for the past 90 days.
func (c *CloudTrailClient) LookupCreator(ctx context.Context, resourceID string) (string, error) {
	// 1. Define window (CloudTrail Lookup is limited)
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -90) // 90 days ago

	// 2. Define Lookup Attribute
	// We use ResourceName because ResourceID is not always indexed, but ResourceName often matches ID for EC2/EBS.
	attrKey := types.LookupAttributeKeyResourceName
	
	input := &cloudtrail.LookupEventsInput{
		LookupAttributes: []types.LookupAttribute{
			{
				AttributeKey:   attrKey,
				AttributeValue: aws.String(resourceID),
			},
		},
		StartTime: &startTime,
		EndTime:   &endTime,
		MaxResults: aws.Int32(50), 
	}

	// 3. Query
	paginator := cloudtrail.NewLookupEventsPaginator(c.Client, input)
	
	// We only need the first page usually.
	if paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return "", err
		}

		for _, event := range output.Events {
			// Look for "creation" like events
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
