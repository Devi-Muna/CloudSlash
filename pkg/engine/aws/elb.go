package aws

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

type ELBScanner struct {
	Client *elasticloadbalancingv2.Client
	Graph  *graph.Graph
}

func NewELBScanner(cfg aws.Config, g *graph.Graph) *ELBScanner {
	return &ELBScanner{
		Client: elasticloadbalancingv2.NewFromConfig(cfg),
		Graph:  g,
	}
}

func (s *ELBScanner) ScanLoadBalancers(ctx context.Context) error {
	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(s.Client, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to describe load balancers: %v", err)
		}

		for _, lb := range page.LoadBalancers {
			arn := *lb.LoadBalancerArn
			name := *lb.LoadBalancerName

			props := map[string]interface{}{
				"Name":  name,
				"State": lb.State.Code,
				"Type":  string(lb.Type),
			}

			s.Graph.AddNode(arn, "AWS::ElasticLoadBalancingV2::LoadBalancer", props)
		}
	}
	return nil
}
