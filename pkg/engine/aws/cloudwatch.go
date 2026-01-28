package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type CloudWatchClient struct {
	Client *cloudwatch.Client
}

func NewCloudWatchClient(cfg aws.Config) *CloudWatchClient {
	return &CloudWatchClient{
		Client: cloudwatch.NewFromConfig(cfg),
	}
}

// GetMetricMax returns the maximum value of a metric over a period.
func (c *CloudWatchClient) GetMetricMax(ctx context.Context, namespace, metricName string, dimensions []types.Dimension, startTime, endTime time.Time) (float64, error) {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		Dimensions: dimensions,
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		Period:     aws.Int32(86400), // Daily granularity
		Statistics: []types.Statistic{types.StatisticMaximum},
	}

	result, err := c.Client.GetMetricStatistics(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to get metric statistics: %v", err)
	}

	maxVal := 0.0
	for _, dp := range result.Datapoints {
		if dp.Maximum != nil && *dp.Maximum > maxVal {
			maxVal = *dp.Maximum
		}
	}

	return maxVal, nil
}

// GetMetricSum returns the sum of a metric over a period.
func (c *CloudWatchClient) GetMetricSum(ctx context.Context, namespace, metricName string, dimensions []types.Dimension, startTime, endTime time.Time) (float64, error) {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		Dimensions: dimensions,
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		Period:     aws.Int32(86400),
		Statistics: []types.Statistic{types.StatisticSum},
	}

	result, err := c.Client.GetMetricStatistics(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to get metric statistics: %v", err)
	}

	sumVal := 0.0
	for _, dp := range result.Datapoints {
		if dp.Sum != nil {
			sumVal += *dp.Sum
		}
	}

	return sumVal, nil
}
