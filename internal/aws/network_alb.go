package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	"github.com/DrSkyle/cloudslash/internal/graph"
)

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

// ScanALBs implements "Smart Idle" check.
func (s *ALBScanner) ScanALBs(ctx context.Context) error {
	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(s.Client, &elasticloadbalancingv2.DescribeLoadBalancersInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil { return err }

		for _, lb := range page.LoadBalancers {
			// Only ALBs (Application) for this feature (Redirect/RequestCount).
			// NLBs work differently (FlowCount).
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
			
			// Using ARN as ID for uniqueness (LB names unique per region but graph might be multi-region? ARN is safest).
			// But for consistent readability we usually match conventions.
			// Let's use ARN.
			s.Graph.AddNode(arn, "aws_alb", props)
			
			// 1. Metric: RequestCount
			go s.checkRequests(ctx, arn, props)
			
			// 2. Redirect Awareness
			go s.checkListeners(ctx, arn)
			
			// 3. WAF Wallet Check
			go s.checkWAF(ctx, arn)
		}
	}
	return nil
}

func (s *ALBScanner) checkRequests(ctx context.Context, arn string, props map[string]interface{}) {
	s.Graph.Mu.Lock()
	node, exists := s.Graph.Nodes[arn]
	s.Graph.Mu.Unlock()
	if !exists { return }
	
	// Extract Resource ID for CW (app/lb-name/id) from ARN
	// AWS CW Dimension 'LoadBalancer' expects format 'app/my-load-balancer/50dc6c495c0c9188'
	// ARN: arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/my-load-balancer/50dc6c495c0c9188
	// ID: app/my-load-balancer/50dc6c495c0c9188
	resourceId := ""
	// Simple split by ":loadbalancer/"
	// Note: Be careful with split.
	
	// Better: use manual parsing for robustness.
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
	if err != nil { return }
	
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

func (s *ALBScanner) checkListeners(ctx context.Context, arn string) {
	out, err := s.Client.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{
		LoadBalancerArn: aws.String(arn),
	})
	if err != nil { return }
	
	allRedirects := true
	if len(out.Listeners) == 0 {
		allRedirects = false // No listeners = Empty = Not redirect only (it's useless)
	}
	
	for _, l := range out.Listeners {
		// Check Default Actions
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
	
	s.Graph.Mu.Lock()
	if node, ok := s.Graph.Nodes[arn]; ok {
		node.Properties["IsRedirectOnly"] = allRedirects
	}
	s.Graph.Mu.Unlock()
}

func (s *ALBScanner) checkWAF(ctx context.Context, arn string) {
	// WAFv2 GetWebACLForResource
	// Scope: CLOUDFRONT or REGIONAL. ALB is REGIONAL.
	out, err := s.WAFClient.GetWebACLForResource(ctx, &wafv2.GetWebACLForResourceInput{
		ResourceArn: aws.String(arn),
	})
	
	hasWAF := false
	wafCost := 0.0
	
	if err == nil && out.WebACL != nil {
		hasWAF = true
		// Estimate Cost: $5.00/mo + $1.00 per rule group (approx).
		wafCost = 5.0
	}
	
	s.Graph.Mu.Lock()
	if node, ok := s.Graph.Nodes[arn]; ok {
		node.Properties["HasWAF"] = hasWAF
		node.Properties["WAFCostEst"] = wafCost
	}
	s.Graph.Mu.Unlock()
}
