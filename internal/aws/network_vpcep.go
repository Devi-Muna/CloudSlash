package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/DrSkyle/cloudslash/internal/graph"
)

type VpcEndpointScanner struct {
	Client   *ec2.Client
	CWClient *cloudwatch.Client
	Graph    *graph.Graph
}

func NewVpcEndpointScanner(cfg aws.Config, g *graph.Graph) *VpcEndpointScanner {
	return &VpcEndpointScanner{
		Client:   ec2.NewFromConfig(cfg),
		CWClient: cloudwatch.NewFromConfig(cfg),
		Graph:    g,
	}
}

// ScanEndpoints checks for unused Interface Endpoints.
func (s *VpcEndpointScanner) ScanEndpoints(ctx context.Context) error {
	paginator := ec2.NewDescribeVpcEndpointsPaginator(s.Client, &ec2.DescribeVpcEndpointsInput{
		Filters: []types.Filter{
			{Name: aws.String("vpc-endpoint-type"), Values: []string{"Interface"}}, // Only scan Interface ($$$)
		},
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil { return err }

		for _, ep := range page.VpcEndpoints {
			id := *ep.VpcEndpointId
			
			props := map[string]interface{}{
				"Service": "VpcEndpoint",
				"Type":    "Interface",
				"VpcId":   *ep.VpcId,
				"ServiceName": *ep.ServiceName,
				"State":   string(ep.State),
			}
			
			s.Graph.AddNode(id, "aws_vpc_endpoint", props)
			
			go s.checkFlow(ctx, id, props)
		}
	}
	return nil
}

func (s *VpcEndpointScanner) checkFlow(ctx context.Context, id string, props map[string]interface{}) {
	node := s.Graph.GetNode(id)
	if node == nil { return }
	
	// Metric: BytesProcessed
	// Note: Per Endpoint
	
	endTime := time.Now()
	startTime := endTime.Add(-30 * 24 * time.Hour) // 30 Days
	
	queries := []cwtypes.MetricDataQuery{
		{
			Id: aws.String("m_bytes"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/PrivateLinkEndpoints"), // Correct Namespace
					MetricName: aws.String("BytesProcessed"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("VpcEndpointId"), Value: aws.String(id)}},
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
	if err != nil { return }
	
	totalBytes := 0.0
	for _, res := range out.MetricDataResults {
		for _, v := range res.Values {
			totalBytes += v
		}
	}
	
	s.Graph.Mu.Lock()
	node.Properties["SumBytesProcessed30d"] = totalBytes
	s.Graph.Mu.Unlock()
}
