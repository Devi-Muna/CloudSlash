package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	aaTypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoDBScanner struct {
	Client   *dynamodb.Client
	AAClient *applicationautoscaling.Client
	CWClient *cloudwatch.Client
	Graph    *graph.Graph
}

func NewDynamoDBScanner(cfg aws.Config, g *graph.Graph) *DynamoDBScanner {
	return &DynamoDBScanner{
		Client:   dynamodb.NewFromConfig(cfg),
		AAClient: applicationautoscaling.NewFromConfig(cfg),
		CWClient: cloudwatch.NewFromConfig(cfg),
		Graph:    g,
	}
}

// ScanTables scans tables and analyzes metrics.
func (s *DynamoDBScanner) ScanTables(ctx context.Context) error {
	paginator := dynamodb.NewListTablesPaginator(s.Client, &dynamodb.ListTablesInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, tableName := range page.TableNames {
			// Describe table.
			desc, err := s.Client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(tableName)})
			if err != nil {
				continue
			}
			table := desc.Table

			// Check billing mode.
			isProvisioned := false
			if table.BillingModeSummary != nil {
				if table.BillingModeSummary.BillingMode == types.BillingModeProvisioned {
					isProvisioned = true
				}
			} else {
				// Handle legacy defaults where BillingModeSummary matches nil for Provisioned.
				if table.ProvisionedThroughput != nil {
					isProvisioned = true
				}
			}

			if !isProvisioned {
				continue // Skip On-Demand tables as they do not incur idle costs.
			}

			// Get capacity.
			readCap := *table.ProvisionedThroughput.ReadCapacityUnits
			writeCap := *table.ProvisionedThroughput.WriteCapacityUnits

			props := map[string]interface{}{
				"Service":            "DynamoDB",
				"BillingMode":        "PROVISIONED",
				"ProvisionedRCU":     float64(readCap),
				"ProvisionedWCU":     float64(writeCap),
				"TableSizeBytes":     table.TableSizeBytes,
				"GlobalTableVersion": table.GlobalTableVersion,
			}

			s.Graph.AddNode(tableName, "aws_dynamodb_table", props)

			// Check auto-scaling.
			go s.checkAutoScaling(ctx, tableName, props)

			// Enrich metrics.
			go s.enrichTableMetrics(ctx, tableName, props)
		}
	}
	return nil
}

// checkAutoScaling checks for scaling policies.
func (s *DynamoDBScanner) checkAutoScaling(ctx context.Context, tableName string, props map[string]interface{}) {
	// Resource ID.
	resourceId := fmt.Sprintf("table/%s", tableName)

	out, err := s.AAClient.DescribeScalingPolicies(ctx, &applicationautoscaling.DescribeScalingPoliciesInput{
		ServiceNamespace: aaTypes.ServiceNamespaceDynamodb,
		ResourceId:       aws.String(resourceId),
	})

	hasAS := false
	if err == nil {
		if len(out.ScalingPolicies) > 0 {
			hasAS = true
		}
	}

	// Update node.
	node := s.Graph.GetNode(tableName)
	if node != nil {
		s.Graph.Mu.Lock()
		node.Properties["HasAutoScaling"] = hasAS
		s.Graph.Mu.Unlock()
	}
}

// enrichTableMetrics adds CloudWatch metrics.
func (s *DynamoDBScanner) enrichTableMetrics(ctx context.Context, tableName string, props map[string]interface{}) {
	// Get node.
	node := s.Graph.GetNode(tableName)

	if node == nil {
		return
	}
	exists := true
	if !exists {
		return
	}

	endTime := time.Now()
	startTime := endTime.Add(-30 * 24 * time.Hour) // 30 Days

	queries := []cwtypes.MetricDataQuery{
		{
			Id: aws.String("m_consumed_read"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/DynamoDB"),
					MetricName: aws.String("ConsumedReadCapacityUnits"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("TableName"), Value: aws.String(tableName)}},
				},
				Period: aws.Int32(86400),
				Stat:   aws.String("Sum"), // Sum over day
			},
		},
		{
			Id: aws.String("m_consumed_write"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/DynamoDB"),
					MetricName: aws.String("ConsumedWriteCapacityUnits"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("TableName"), Value: aws.String(tableName)}},
				},
				Period: aws.Int32(86400),
				Stat:   aws.String("Sum"),
			},
		},
	}

	out, err := s.CWClient.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
		MetricDataQueries: queries,
		StartTime:         &startTime,
		EndTime:           &endTime,
	})
	if err != nil {
		return
	}

	var avgConsumedRCU, avgConsumedWCU float64
	// Calc avg daily.

	for _, res := range out.MetricDataResults {
		totalSum := 0.0
		count := 0.0
		for _, v := range res.Values {
			totalSum += v
			count++
		}

		avgDailySum := 0.0
		if count > 0 {
			avgDailySum = totalSum / count
		}

		avgPerSec := avgDailySum / 86400.0

		if *res.Id == "m_consumed_read" {
			avgConsumedRCU = avgPerSec
		} else {
			avgConsumedWCU = avgPerSec
		}
	}

	s.Graph.Mu.Lock()
	node.Properties["AvgConsumedRCU30d"] = avgConsumedRCU
	node.Properties["AvgConsumedWCU30d"] = avgConsumedWCU
	s.Graph.Mu.Unlock()
}
