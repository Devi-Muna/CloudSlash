package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// CloudWatchClient retrieves metrics.
type CloudWatchClient struct {
	Client *cloudwatch.Client
}

func NewCloudWatchClient(cfg aws.Config) *CloudWatchClient {
	return &CloudWatchClient{
		Client: cloudwatch.NewFromConfig(cfg),
	}
}

// GetMetricHistory retrieves a daily history of maximum values.
func (c *CloudWatchClient) GetMetricHistory(ctx context.Context, namespace, metricName string, dimensions []types.Dimension, startTime, endTime time.Time) ([]float64, error) {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		Dimensions: dimensions,
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		Period:     aws.Int32(86400), // Daily data points
		Statistics: []types.Statistic{types.StatisticMaximum},
	}

	result, err := c.Client.GetMetricStatistics(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric history: %v", err)
	}

	// CloudWatch returns datapoints in random order; sort by timestamp.
	sort.Slice(result.Datapoints, func(i, j int) bool {
		return result.Datapoints[i].Timestamp.Before(*result.Datapoints[j].Timestamp)
	})

	// Extract values
	var history []float64
	for _, dp := range result.Datapoints {
		if dp.Maximum != nil {
			history = append(history, *dp.Maximum)
		}
	}

	return history, nil
}

// GetMetricMax retrieves the single highest value.
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

// GetMetricSum retrieves the total sum.
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
