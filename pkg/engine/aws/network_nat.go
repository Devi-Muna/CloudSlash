package aws

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/DrSkyle/cloudslash/pkg/graph"
)

type NATScanner struct {
	Client   *ec2.Client
	CWClient *cloudwatch.Client
	Graph    *graph.Graph
}

func NewNATScanner(cfg aws.Config, g *graph.Graph) *NATScanner {
	return &NATScanner{
		Client:   ec2.NewFromConfig(cfg),
		CWClient: cloudwatch.NewFromConfig(cfg),
		Graph:    g,
	}
}

// ScanNATGateways implements the "Idle NAT" detection.
func (s *NATScanner) ScanNATGateways(ctx context.Context) error {
	paginator := ec2.NewDescribeNatGatewaysPaginator(s.Client, &ec2.DescribeNatGatewaysInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, nat := range page.NatGateways {
			if nat.State != types.NatGatewayStateAvailable {
				continue
			}
			
			id := *nat.NatGatewayId
			vpcId := *nat.VpcId
			
			props := map[string]interface{}{
				"Service": "NATGateway",
				"VpcId":   vpcId,
				"SubnetId": *nat.SubnetId,
				"State": string(nat.State),
				"PublicIp": extractPublicIp(nat.NatGatewayAddresses),
			}
			
			s.Graph.AddNode(id, "aws_nat_gateway", props)
			
			// 1. Check connection metrics.
			go s.checkTraffic(ctx, id, props)
			
			// 2. Analyze network topology.
			go s.checkEmptyRoom(ctx, id, vpcId)
		}
	}
	return nil
}

func extractPublicIp(addrs []types.NatGatewayAddress) string {
	if len(addrs) > 0 && addrs[0].PublicIp != nil {
		return *addrs[0].PublicIp
	}
	return ""
}

func (s *NATScanner) checkTraffic(ctx context.Context, id string, props map[string]interface{}) {
	node := s.Graph.GetNode(id)
	if node == nil { return }

	endTime := time.Now()
	startTime := endTime.Add(-7 * 24 * time.Hour)

	queries := []cwtypes.MetricDataQuery{
		{
			Id: aws.String("m_conns"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/NATGateway"),
					MetricName: aws.String("ConnectionEstablishedCount"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("NatGatewayId"), Value: aws.String(id)}},
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
	
	totalConns := 0.0
	for _, res := range out.MetricDataResults {
		for _, v := range res.Values {
			totalConns += v
		}
	}
	
	s.Graph.Mu.Lock()
	node.Properties["SumConnections7d"] = totalConns
	s.Graph.Mu.Unlock()
}

// checkEmptyRoom verifies if the NAT serves any ACTIVE instance.
func (s *NATScanner) checkEmptyRoom(ctx context.Context, natId string, vpcId string) {
	// 1. Find Route Tables pointing to this NAT
	routeReq := &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcId}},
			{Name: aws.String("route.nat-gateway-id"), Values: []string{natId}},
		},
	}
	
	rtOut, err := s.Client.DescribeRouteTables(ctx, routeReq)
	if err != nil { return }
	
	subnets := make(map[string]bool)
	var rtbIds []string
	for _, rt := range rtOut.RouteTables {
		if rt.RouteTableId != nil {
			rtbIds = append(rtbIds, *rt.RouteTableId)
		}
		for _, assoc := range rt.Associations {
			if assoc.SubnetId != nil {
				subnets[*assoc.SubnetId] = true
			}
		}
	}
	
	// Store RouteTables for visualization
	node := s.Graph.GetNode(natId)
	if node != nil {
		s.Graph.Mu.Lock()
		node.Properties["RouteTables"] = rtbIds
		s.Graph.Mu.Unlock()
	}
	
	if len(subnets) == 0 {
		// Identify NATs with no associated subnets.
		s.updateActiveCount(natId, 0)
		return
	}
	
	// 2. Scan Subnets for Active ENIs
	activeENICount := 0
	var emptySubnetIds []string
	
	for subnetId := range subnets {
		// Describe Network Interfaces in this subnet
		eniReq := &ec2.DescribeNetworkInterfacesInput{
			Filters: []types.Filter{
				{Name: aws.String("subnet-id"), Values: []string{subnetId}},
			},
		}
		
		eniOut, err := s.Client.DescribeNetworkInterfaces(ctx, eniReq)
		if err != nil { continue }
		
		subnetActive := 0
		for _, eni := range eniOut.NetworkInterfaces {
			// Exclude AWS owned (Lambda, NAT itself)
			if eni.InterfaceType == types.NetworkInterfaceTypeNatGateway {
				continue // Self
			}
			
			// Check Attachment
			if eni.Attachment != nil && eni.Attachment.InstanceId != nil {
				instId := *eni.Attachment.InstanceId
				
				instOut, err := s.Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
					InstanceIds: []string{instId},
				})
				
				if err == nil && len(instOut.Reservations) > 0 {
					state := instOut.Reservations[0].Instances[0].State.Name
					if state == types.InstanceStateNameRunning {
						subnetActive++
						activeENICount++
					}
				}
			} else {
				if strings.ToLower(string(eni.InterfaceType)) == "interface" {
                     if eni.RequesterManaged != nil && *eni.RequesterManaged {
                         continue // AWS Managed
                     }
                     if eni.Status == types.NetworkInterfaceStatusInUse {
                         subnetActive++
                         activeENICount++
                     }
				}
			}
		}
		
		if subnetActive == 0 {
			emptySubnetIds = append(emptySubnetIds, subnetId)
		}
	}
	

	nodeRedecl := s.Graph.GetNode(natId)
	if nodeRedecl != nil {
		s.Graph.Mu.Lock()
		nodeRedecl.Properties["ActiveUserENIs"] = activeENICount
		nodeRedecl.Properties["EmptySubnets"] = emptySubnetIds
		s.Graph.Mu.Unlock()
	}
}
func (s *NATScanner) updateActiveCount(natId string, count int) {
	node := s.Graph.GetNode(natId)
	if node != nil {
		s.Graph.Mu.Lock()
		node.Properties["ActiveUserENIs"] = count
		s.Graph.Mu.Unlock()
	}
}
