package aws

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

type CloudWatchLogsClient struct {
	Client *cloudwatchlogs.Client
	Graph  *graph.Graph
}

func NewCloudWatchLogsClient(cfg aws.Config, g *graph.Graph) *CloudWatchLogsClient {
	return &CloudWatchLogsClient{
		Client: cloudwatchlogs.NewFromConfig(cfg),
		Graph:  g,
	}
}

// ScanLogGroups scans for Log Groups and populates the graph.
func (c *CloudWatchLogsClient) ScanLogGroups(ctx context.Context) error {
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(c.Client, &cloudwatchlogs.DescribeLogGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to describe log groups: %v", err)
		}

		for _, group := range page.LogGroups {
			arn := *group.Arn
			// Strip trailing :* if present (sometimes ARN has :*)
			// arn:aws:logs:region:account:log-group:name:*
			
			props := map[string]interface{}{
				"StoredBytes": group.StoredBytes,
				"Retention":   "Never",
			}

			if group.RetentionInDays != nil {
				props["Retention"] = *group.RetentionInDays
			}

			c.Graph.AddNode(arn, "AWS::Logs::LogGroup", props)
		}
	}
	return nil
}

// DescribeLogGroups helper if needed for direct access
func (c *CloudWatchLogsClient) DescribeLogGroups(ctx context.Context) ([]types.LogGroup, error) {
    // ... logic duplicated above, but simplified for heuristic direct use if we didn't use graph scan
    // But we prefer scanning into graph.
    return nil, nil 
}
