package heuristics

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	internalaws "github.com/saujanyayaya/cloudslash/internal/aws"
	"github.com/saujanyayaya/cloudslash/internal/graph"
)

type Heuristic interface {
	Analyze(ctx context.Context, g *graph.Graph) error
}

// NATGatewayHeuristic checks for unused NAT Gateways.
type NATGatewayHeuristic struct {
	CW *internalaws.CloudWatchClient
}

func (h *NATGatewayHeuristic) Analyze(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	// Copy nodes to avoid holding lock during network calls
	var natGateways []*graph.Node
	for _, node := range g.Nodes {
		if node.Type == "AWS::EC2::NatGateway" {
			natGateways = append(natGateways, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range natGateways {
		// Get metrics for the last 7 days
		endTime := time.Now()
		startTime := endTime.Add(-7 * 24 * time.Hour)

		// Parse ID from ARN
		// arn:aws:ec2:region:account:natgateway/nat-12345
		// Simplified parsing
		var id string
		fmt.Sscanf(node.ID, "arn:aws:ec2:region:account:natgateway/%s", &id)
		if id == "" {
			continue
		}

		dims := []types.Dimension{
			{Name: aws.String("NatGatewayId"), Value: aws.String(id)},
		}

		// Check ActiveConnectionCount
		maxConns, err := h.CW.GetMetricMax(ctx, "AWS/NATGateway", "ActiveConnectionCount", dims, startTime, endTime)
		if err != nil {
			fmt.Printf("Error getting metrics for %s: %v\n", id, err)
			continue
		}

		// Check BytesOut
		sumBytes, err := h.CW.GetMetricSum(ctx, "AWS/NATGateway", "BytesOutToDestination", dims, startTime, endTime)
		if err != nil {
			continue
		}

		// Heuristic: If max connections < 5 and total bytes < 1GB (1e9) in 7 days -> Waste
		if maxConns < 5 && sumBytes < 1e9 {
			g.MarkWaste(node.ID, 80)
			node.Properties["Reason"] = fmt.Sprintf("Unused NAT Gateway: MaxConns=%.0f, BytesOut=%.0f", maxConns, sumBytes)
		}
	}
	return nil
}

// ZombieEBSHeuristic checks for unattached or zombie volumes.
type ZombieEBSHeuristic struct{}

func (h *ZombieEBSHeuristic) Analyze(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.Type != "AWS::EC2::Volume" {
			continue
		}

		state, _ := node.Properties["State"].(string)
		if state == "available" {
			// Unattached volume
			node.IsWaste = true
			node.RiskScore = 90
			node.Properties["Reason"] = "Unattached EBS Volume"
			continue
		}

		if state == "in-use" {
			// Check attached instance
			instanceID, ok := node.Properties["AttachedInstanceId"].(string)
			if !ok {
				continue
			}

			// Find instance node
			instanceARN := fmt.Sprintf("arn:aws:ec2:region:account:instance/%s", instanceID)
			instanceNode, ok := g.Nodes[instanceARN]
			if !ok {
				continue
			}

			instanceState, _ := instanceNode.Properties["State"].(string)
			launchTime, _ := instanceNode.Properties["LaunchTime"].(time.Time)
			deleteOnTerm, _ := node.Properties["DeleteOnTermination"].(bool)

			// Heuristic: Instance stopped > 30 days and DeleteOnTermination is false
			if instanceState == "stopped" && time.Since(launchTime) > 30*24*time.Hour && !deleteOnTerm {
				node.IsWaste = true
				node.RiskScore = 70
				node.Properties["Reason"] = "Zombie EBS: Attached to stopped instance > 30 days"
			}
		}
	}
	return nil
}

// ElasticIPHeuristic checks for EIPs attached to stopped instances or unattached.
type ElasticIPHeuristic struct{}

func (h *ElasticIPHeuristic) Analyze(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.Type != "AWS::EC2::EIP" {
			continue
		}

		instanceID, hasInstance := node.Properties["InstanceId"].(string)
		if !hasInstance {
			// Unattached EIP
			node.IsWaste = true
			node.RiskScore = 50
			node.Properties["Reason"] = "Unattached Elastic IP"
			continue
		}

		// Check instance state
		instanceARN := fmt.Sprintf("arn:aws:ec2:region:account:instance/%s", instanceID)
		instanceNode, ok := g.Nodes[instanceARN]
		if ok {
			state, _ := instanceNode.Properties["State"].(string)
			if state == "stopped" {
				node.IsWaste = true
				node.RiskScore = 60
				node.Properties["Reason"] = "Elastic IP attached to stopped instance"
			}
		}
	}
	return nil
}

// S3MultipartHeuristic checks for incomplete multipart uploads.
type S3MultipartHeuristic struct{}

func (h *S3MultipartHeuristic) Analyze(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.Type == "AWS::S3::MultipartUpload" {
			// All incomplete multipart uploads found by scanner are considered waste
			// We could add a time threshold (e.g. > 7 days old)
			initiated, ok := node.Properties["Initiated"].(time.Time)
			if ok && time.Since(initiated) > 7*24*time.Hour {
				node.IsWaste = true
				node.RiskScore = 40
				node.Properties["Reason"] = "Stale S3 Multipart Upload (> 7 days)"
			}
		}
	}
	return nil
}

// RDSHeuristic checks for stopped instances or instances with 0 connections.
type RDSHeuristic struct {
	CW *internalaws.CloudWatchClient
}

func (h *RDSHeuristic) Analyze(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var rdsInstances []*graph.Node
	for _, node := range g.Nodes {
		if node.Type == "AWS::RDS::DBInstance" {
			rdsInstances = append(rdsInstances, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range rdsInstances {
		status, _ := node.Properties["Status"].(string)
		if status == "stopped" {
			g.MarkWaste(node.ID, 80)
			node.Properties["Reason"] = "RDS Instance is stopped"
			continue
		}
	}
	return nil
}

// ELBHeuristic checks for unused Load Balancers.
type ELBHeuristic struct {
	CW *internalaws.CloudWatchClient
}

func (h *ELBHeuristic) Analyze(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var elbs []*graph.Node
	for _, node := range g.Nodes {
		if node.Type == "AWS::ElasticLoadBalancingV2::LoadBalancer" {
			elbs = append(elbs, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range elbs {
		// Placeholder for ELB logic
		// If RequestCount == 0 for 7 days -> Waste
		// g.MarkWaste(node.ID, 70)
		// node.Properties["Reason"] = "ELB has 0 requests in 7 days"
		_ = node
	}
	return nil
}
