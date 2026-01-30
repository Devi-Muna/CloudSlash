package aws

import (
	"context"
	"fmt"
    "time"

	"github.com/DrSkyle/cloudslash/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
    cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

type CloudWatchLogsClient struct {
	Client         *cloudwatchlogs.Client
	CWClient       *cloudwatch.Client
	Graph          *graph.Graph
	DisableMetrics bool
}

func NewCloudWatchLogsClient(cfg aws.Config, g *graph.Graph, disableMetrics bool) *CloudWatchLogsClient {
	return &CloudWatchLogsClient{
		Client:         cloudwatchlogs.NewFromConfig(cfg),
		CWClient:       cloudwatch.NewFromConfig(cfg),
		Graph:          g,
		DisableMetrics: disableMetrics,
	}
}

// ScanLogGroups discovers CloudWatch Log Groups.
func (c *CloudWatchLogsClient) ScanLogGroups(ctx context.Context) error {
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(c.Client, &cloudwatchlogs.DescribeLogGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to describe log groups: %v", err)
		}

		for _, group := range page.LogGroups {
			arn := *group.Arn
			// Normalize ARN by removing wildcards.
			// arn:aws:logs:region:account:log-group:name:*
			if len(arn) > 2 && arn[len(arn)-2:] == ":*" {
				arn = arn[:len(arn)-2]
			}

			storedBytes := int64(0)
			if group.StoredBytes != nil {
				storedBytes = *group.StoredBytes
			}

			props := map[string]interface{}{
				"StoredBytes": storedBytes,
				"Retention":   "Never",
			}

			if group.RetentionInDays != nil {
				props["Retention"] = *group.RetentionInDays
			}

			// Check incoming bytes metric.
			incomingBytes := float64(-1)

			if !c.DisableMetrics && storedBytes > 0 {
				// Skip metric retrieval for empty log groups.
				// 30-day analysis window.
				now := time.Now()
				start := now.Add(-30 * 24 * time.Hour)

				metricOut, err := c.CWClient.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
					Namespace:  aws.String("AWS/Logs"),
					MetricName: aws.String("IncomingBytes"),
					Dimensions: []cwtypes.Dimension{
						{Name: aws.String("LogGroupName"), Value: group.LogGroupName},
					},
					StartTime:  &start,
					EndTime:    &now,
					Period:     aws.Int32(30 * 24 * 60 * 60), // Single 30-day datapoint.
					Statistics: []cwtypes.Statistic{cwtypes.StatisticSum},
				})

				if err == nil && len(metricOut.Datapoints) > 0 {
					if metricOut.Datapoints[0].Sum != nil {
						incomingBytes = *metricOut.Datapoints[0].Sum
					}
				} else if err != nil {
					// Metric check failed.
				} else {
					// Missing datapoints imply zero usage.
					incomingBytes = 0
				}
			}
			props["IncomingBytes"] = incomingBytes

			c.Graph.AddNode(arn, "AWS::Logs::LogGroup", props)
		}
	}
	return nil
}

// DescribeLogGroups lists log groups.
func (c *CloudWatchLogsClient) DescribeLogGroups(ctx context.Context) ([]types.LogGroup, error) {
	return nil, nil
}
