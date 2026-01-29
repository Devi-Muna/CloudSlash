package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/DrSkyle/cloudslash/pkg/graph"
)

// ElasticacheScanner handles ElastiCache discovery and metrics.
type ElasticacheScanner struct {
	Client   *elasticache.Client
	CWClient *cloudwatch.Client
	Graph    *graph.Graph
}

// NewElasticacheScanner creates a new scanner instance.
func NewElasticacheScanner(cfg aws.Config, g *graph.Graph) *ElasticacheScanner {
	return &ElasticacheScanner{
		Client:   elasticache.NewFromConfig(cfg),
		CWClient: cloudwatch.NewFromConfig(cfg),
		Graph:    g,
	}
}

// ScanClusters discovers clusters and usage metrics.
// Analyzes metrics over a 7-day window.
func (s *ElasticacheScanner) ScanClusters(ctx context.Context) error {
	paginator := elasticache.NewDescribeCacheClustersPaginator(s.Client, &elasticache.DescribeCacheClustersInput{
		ShowCacheNodeInfo: aws.Bool(true),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, cluster := range page.CacheClusters {
			// Skip memcached nodes that are part of a cluster if processed differently,
			// but DescribeCacheClusters returns all nodes.
			
			// We group by CacheClusterId.
			id := *cluster.CacheClusterId
			
			props := map[string]interface{}{
				"Service":        "Elasticache",
				"Engine":         *cluster.Engine,
				"Status":         *cluster.CacheClusterStatus,
				"NodeType":       *cluster.CacheNodeType,
				"EngineVersion":  *cluster.EngineVersion,
				"NumCacheNodes":  cluster.NumCacheNodes,
			}
			// Fetch comprehensive metrics (Connections, Hits/Misses, CPU, Network).
			
			go s.enrichClusterMetrics(ctx, id, cluster.CacheNodeType, props)
		}
	}
	return nil
}

func (s *ElasticacheScanner) enrichClusterMetrics(ctx context.Context, clusterID string, nodeType *string, props map[string]interface{}) {
	node := s.Graph.GetNode(clusterID)
	// s.Graph.Mu.Unlock() - Removed, GetNode handles lock
	exists := (node != nil)
	if !exists {
		return
	}

	endTime := time.Now()
	startTime := endTime.Add(-7 * 24 * time.Hour) // 7 Days

	// Metrics to fetch
    // Using cwtypes directly
	queries := []cwtypes.MetricDataQuery{
		{
			Id: aws.String("m_conn"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/ElastiCache"),
					MetricName: aws.String("CurrConnections"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("CacheClusterId"), Value: aws.String(clusterID)}},
				},
				Period: aws.Int32(86400), // 1 day buckets
				Stat:   aws.String("Sum"),
			},
		},
		{
			Id: aws.String("m_hits"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/ElastiCache"),
					MetricName: aws.String("CacheHits"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("CacheClusterId"), Value: aws.String(clusterID)}},
				},
				Period: aws.Int32(86400),
				Stat:   aws.String("Sum"),
			},
		},
		{
			Id: aws.String("m_misses"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/ElastiCache"),
					MetricName: aws.String("CacheMisses"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("CacheClusterId"), Value: aws.String(clusterID)}},
				},
				Period: aws.Int32(86400),
				Stat:   aws.String("Sum"),
			},
		},
		{
			Id: aws.String("m_cpu"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/ElastiCache"),
					MetricName: aws.String("CPUUtilization"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("CacheClusterId"), Value: aws.String(clusterID)}},
				},
				Period: aws.Int32(86400),
				Stat:   aws.String("Maximum"),
			},
		},
		{
			Id: aws.String("m_net"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/ElastiCache"),
					MetricName: aws.String("NetworkBytesIn"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("CacheClusterId"), Value: aws.String(clusterID)}},
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
		fmt.Printf("Error fetching metrics for %s: %v\n", clusterID, err)
		return
	}

	// Parse Results
	var totalConn, totalHits, totalMisses, totalNet float64
    var maxCPU float64

	for _, res := range out.MetricDataResults {
		sum := 0.0
        max := 0.0
		for _, val := range res.Values {
			sum += val
            if val > max {
                max = val
            }
		}

		switch *res.Id {
		case "m_conn":
			totalConn = sum
		case "m_hits":
			totalHits = sum
		case "m_misses":
			totalMisses = sum
		case "m_cpu":
			maxCPU = max // Use Max of Max
		case "m_net":
			totalNet = sum
		}
	}

	// Update Node Properties
	s.Graph.Mu.Lock()
	node.Properties["SumConnections7d"] = totalConn
	node.Properties["SumHits7d"] = totalHits
	node.Properties["SumMisses7d"] = totalMisses
	node.Properties["MaxCPU7d"] = maxCPU
	node.Properties["SumNetworkBytesIn7d"] = totalNet
	s.Graph.Mu.Unlock()
}
