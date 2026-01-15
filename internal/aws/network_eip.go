package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/DrSkyle/cloudslash/internal/graph"
)

type EIPScanner struct {
	Client   *ec2.Client
	R53Client *route53.Client
	Graph    *graph.Graph
}

func NewEIPScanner(cfg aws.Config, g *graph.Graph) *EIPScanner {
	return &EIPScanner{
		Client:    ec2.NewFromConfig(cfg),
		R53Client: route53.NewFromConfig(cfg),
		Graph:     g,
	}
}

// ScanAddresses discovers Elastic IPs.
func (s *EIPScanner) ScanAddresses(ctx context.Context) error {
	out, err := s.Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return err
	}

	for _, addr := range out.Addresses {
		ip := *addr.PublicIp
		id := *addr.AllocationId // or PublicIp if EC2-Classic (unlikely)

		props := map[string]interface{}{
			"Service": "EIP",
			"PublicIp": ip,
			"AllocationId": id,
		}

		if addr.AssociationId != nil {
			props["AssociationId"] = *addr.AssociationId
			if addr.InstanceId != nil {
				props["InstanceId"] = *addr.InstanceId
				// Could check instance state here, but Heuristic does it.
			}
		}

		s.Graph.AddNode(id, "aws_eip", props)
		
		// Verify DNS usage to prevent conflicts.
		go s.checkDNS(ctx, id, ip)
	}
	return nil
}

func (s *EIPScanner) checkDNS(ctx context.Context, id, ip string) {
	// 1. List Hosted Zones
	zonesPaginator := route53.NewListHostedZonesPaginator(s.R53Client, &route53.ListHostedZonesInput{})
	
	foundInDNS := false
	badZone := ""
	
	for zonesPaginator.HasMorePages() {
		page, err := zonesPaginator.NextPage(ctx)
		if err != nil { break }
		
		for _, zone := range page.HostedZones {
			// 2. Search Records in Zone
			// List all record sets.
			// Scan all records as Route53 API lacks value-based filtering.
			
			recPaginator := route53.NewListResourceRecordSetsPaginator(s.R53Client, &route53.ListResourceRecordSetsInput{
				HostedZoneId: zone.Id,
			})
			
			for recPaginator.HasMorePages() {
				recPage, err := recPaginator.NextPage(ctx)
				if err != nil { break }
				
				for _, rec := range recPage.ResourceRecordSets {
					// Check ResourceRecords
					for _, rr := range rec.ResourceRecords {
						if rr.Value != nil && strings.Contains(*rr.Value, ip) {
							foundInDNS = true
							badZone = *zone.Name
							break
						}
					}
					if foundInDNS { break }
				}
				if foundInDNS { break }
			}
			if foundInDNS { break }
		}
		if foundInDNS { break }
	}
	

	node := s.Graph.GetNode(id)
	if node != nil {
		s.Graph.Mu.Lock()
		node.Properties["FoundInDNS"] = foundInDNS
		if foundInDNS {
			node.Properties["DNSZone"] = badZone
		}
		s.Graph.Mu.Unlock()
	}
}
