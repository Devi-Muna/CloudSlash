package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/DrSkyle/cloudslash/pkg/graph"
)

type RedshiftScanner struct {
	Client   *redshift.Client
	CWClient *cloudwatch.Client
	Graph    *graph.Graph
}

func NewRedshiftScanner(cfg aws.Config, g *graph.Graph) *RedshiftScanner {
	return &RedshiftScanner{
		Client:   redshift.NewFromConfig(cfg),
		CWClient: cloudwatch.NewFromConfig(cfg),
		Graph:    g,
	}
}

// ScanClusters discovers clusters and usage metrics.
// Analyzes metrics over a 24-hour window.
func (s *RedshiftScanner) ScanClusters(ctx context.Context) error {
	paginator := redshift.NewDescribeClustersPaginator(s.Client, &redshift.DescribeClustersInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, cluster := range page.Clusters {
			id := *cluster.ClusterIdentifier
			
			props := map[string]interface{}{
				"Service":                   "Redshift",
				"NodeType":                  *cluster.NodeType,
				"ClusterStatus":             *cluster.ClusterStatus, // e.g., "available"
				"ClusterAvailabilityStatus": *cluster.ClusterAvailabilityStatus, 
				"NumberOfNodes":             cluster.NumberOfNodes,
			}

			// Add to Graph
			s.Graph.AddNode(id, "aws_redshift_cluster", props)

			// Enrich with Metrics
			go s.enrichClusterMetrics(ctx, id, props)
			
			// Optimization: Reserve Instance coverage check pending implementation.
			// Default to On-Demand until RI matching is added.
		}
	}
	return nil
}

func (s *RedshiftScanner) enrichClusterMetrics(ctx context.Context, clusterID string, props map[string]interface{}) {
	node := s.Graph.GetNode(clusterID)
	exists := (node != nil)
	if !exists {
		return
	}

	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour) // 24 Hours

	queries := []cwtypes.MetricDataQuery{
		{
			Id: aws.String("m_queries"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/Redshift"),
					MetricName: aws.String("QueriesCompletedPerSecond"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("ClusterIdentifier"), Value: aws.String(clusterID)}},
				},
				Period: aws.Int32(3600), // 1 hour buckets
				Stat:   aws.String("Sum"),
			},
		},
		{
			Id: aws.String("m_conns"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/Redshift"),
					MetricName: aws.String("DatabaseConnections"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("ClusterIdentifier"), Value: aws.String(clusterID)}},
				},
				Period: aws.Int32(3600),
				Stat:   aws.String("Maximum"),
			},
		},
	}

	out, err := s.CWClient.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
		MetricDataQueries: queries,
		StartTime:         &startTime,
		EndTime:           &endTime,
	})

	if err != nil {
		fmt.Printf("Error Redshift metrics %s: %v\n", clusterID, err)
		return
	}

	var totalQueries, maxConns float64
	for _, res := range out.MetricDataResults {
		sum := 0.0
		max := 0.0
		for _, v := range res.Values {
			sum += v
			if v > max { max = v }
		}
		if *res.Id == "m_queries" {
			totalQueries = sum
		} else if *res.Id == "m_conns" {
			maxConns = max
		}
	}

	s.Graph.Mu.Lock()
	node.Properties["SumQueries24h"] = totalQueries
	node.Properties["MaxConnections24h"] = maxConns
	s.Graph.Mu.Unlock()
}
