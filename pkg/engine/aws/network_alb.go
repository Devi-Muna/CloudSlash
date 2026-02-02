package aws

import (
	"context"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
)

// ALBScanner scans Application Load Balancers.
type ALBScanner struct {
	Client    *elasticloadbalancingv2.Client
	CWClient  *cloudwatch.Client
	WAFClient *wafv2.Client
	Graph     *graph.Graph
}

func NewALBScanner(cfg aws.Config, g *graph.Graph) *ALBScanner {
	return &ALBScanner{
		Client:    elasticloadbalancingv2.NewFromConfig(cfg),
		CWClient:  cloudwatch.NewFromConfig(cfg),
		WAFClient: wafv2.NewFromConfig(cfg),
		Graph:     g,
	}
}

// ScanALBs scans ALBs and checks traffic.
func (s *ALBScanner) ScanALBs(ctx context.Context) error {
	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(s.Client, &elasticloadbalancingv2.DescribeLoadBalancersInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, lb := range page.LoadBalancers {
			// Filter ALBs.
			if lb.Type != elbv2types.LoadBalancerTypeEnumApplication {
				continue
			}

			arn := *lb.LoadBalancerArn

			props := map[string]interface{}{
				"Service": "ALB",
				"Arn":     arn,
				"DNS":     *lb.DNSName,
				"State":   string(lb.State.Code),
			}

			s.Graph.AddNode(arn, "aws_alb", props)

			// Check traffic metrics.
			go s.checkRequests(ctx, arn, props)

			// Analyze listener configurations.
			go s.checkListeners(ctx, arn)

			// Verify WAF association.
			go s.checkWAF(ctx, arn)
		}
	}
	return nil
}

// checkRequests queries request counts (7 days).
func (s *ALBScanner) checkRequests(ctx context.Context, arn string, props map[string]interface{}) {
	node := s.Graph.GetNode(arn)
	if node == nil {
		return
	}

	// Parse ResourceID.
	// Format: app/load-balancer-name/id

	// Manual parsing.
	var resourceId string
	parts := -1
	for i := 0; i < len(arn)-13; i++ {
		if arn[i:i+13] == "loadbalancer/" {
			parts = i + 13
			break
		}
	}
	if parts != -1 {
		resourceId = arn[parts:]
	} else {
		return // Can't parse
	}

	endTime := time.Now()
	startTime := endTime.Add(-7 * 24 * time.Hour)

	queries := []cwtypes.MetricDataQuery{
		{
			Id: aws.String("m_reqs"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/ApplicationELB"),
					MetricName: aws.String("RequestCount"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("LoadBalancer"), Value: aws.String(resourceId)}},
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

	sumReqs := 0.0
	for _, res := range out.MetricDataResults {
		for _, v := range res.Values {
			sumReqs += v
		}
	}

	s.Graph.Mu.Lock()
	node.Properties["SumRequests7d"] = sumReqs
	s.Graph.Mu.Unlock()
}

// checkListeners checks for redirection-only status.
func (s *ALBScanner) checkListeners(ctx context.Context, arn string) {
	out, err := s.Client.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{
		LoadBalancerArn: aws.String(arn),
	})
	if err != nil {
		return
	}

	allRedirects := true
	if len(out.Listeners) == 0 {
		allRedirects = false
	}

	for _, l := range out.Listeners {
		// Check actions.
		isRedirect := false
		for _, act := range l.DefaultActions {
			if act.Type == elbv2types.ActionTypeEnumRedirect {
				isRedirect = true
			}
		}
		if !isRedirect {
			allRedirects = false
			break
		}
	}

	node := s.Graph.GetNode(arn)
	if node != nil {
		s.Graph.Mu.Lock()
		node.Properties["IsRedirectOnly"] = allRedirects
		s.Graph.Mu.Unlock()
	}
}

// checkWAF checks Web ACL association.
func (s *ALBScanner) checkWAF(ctx context.Context, arn string) {
	// Check WAF (Regional).
	out, err := s.WAFClient.GetWebACLForResource(ctx, &wafv2.GetWebACLForResourceInput{
		ResourceArn: aws.String(arn),
	})

	hasWAF := false
	wafCost := 0.0

	if err == nil && out.WebACL != nil {
		hasWAF = true
		// Est. cost.
		wafCost = 5.0
	}

	node := s.Graph.GetNode(arn)
	if node != nil {
		s.Graph.Mu.Lock()
		node.Properties["HasWAF"] = hasWAF
		node.Properties["WAFCostEst"] = wafCost
		s.Graph.Mu.Unlock()
	}
}
