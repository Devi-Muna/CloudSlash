package heuristics

import (
	"context"
	"fmt"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

type NetworkForensicsHeuristic struct{}

func (h *NetworkForensicsHeuristic) Name() string { return "NetworkForensics" }

func (h *NetworkForensicsHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	return h.Analyze(g), nil
}

func (h *NetworkForensicsHeuristic) Analyze(g *graph.Graph) *HeuristicStats {
	stats := &HeuristicStats{}
	for _, n := range g.GetNodes() {
		var isWaste bool
		switch n.TypeStr() {
		case "aws_nat_gateway":
			isWaste = h.analyzeNAT(n, g)
		case "aws_eip":
			isWaste = h.analyzeEIP(n)
		case "aws_alb":
			isWaste = h.analyzeALB(n)
		case "aws_vpc_endpoint":
			isWaste = h.analyzeVPCEP(n)
		}

		if isWaste {
			stats.ItemsFound++
			stats.ProjectedSavings += n.Cost
		}
	}
	return stats
}

func (h *NetworkForensicsHeuristic) analyzeNAT(n *graph.Node, g *graph.Graph) bool {
	conns, _ := n.Properties["SumConnections7d"].(float64)
	active, _ := n.Properties["ActiveUserENIs"].(int)

	if conns == 0 && active == 0 {
		n.IsWaste = true
		n.RiskScore = 90
		n.Properties["Reason"] = "Unused NAT."
		n.Cost = 32.0

		h.topo(g, n)
		return true
	}
	return false
}

func (h *NetworkForensicsHeuristic) topo(g *graph.Graph, nat *graph.Node) {
	subnets, ok := nat.Properties["EmptySubnets"].([]string)
	if !ok {
		return
	}

	for _, id := range subnets {
		g.AddNode(id, "aws_subnet", map[string]interface{}{
			"Reason":   "Empty Subnet",
			"ParentID": nat.IDStr(),
			"Name":     fmt.Sprintf("Subnet: %s (Empty)", id),
		})
		if node := g.GetNode(id); node != nil {
			node.IsWaste = true
		}
	}

	if rtbs, ok := nat.Properties["RouteTables"].([]string); ok {
		for _, id := range rtbs {
			g.AddNode(id, "aws_route_table", map[string]interface{}{
				"Reason":   "Route Table",
				"ParentID": nat.IDStr(),
				"Name":     fmt.Sprintf("Route Table: %s", id),
			})
			if node := g.GetNode(id); node != nil {
				node.IsWaste = true
			}
		}
	}
}

func (h *NetworkForensicsHeuristic) analyzeEIP(n *graph.Node) bool {
	if assoc, _ := n.Properties["AssociationId"].(string); assoc != "" {
		return false
	}

	n.IsWaste = true
	n.Cost = 3.5

	inDNS, _ := n.Properties["FoundInDNS"].(bool)
	if inDNS {
		zone, _ := n.Properties["DNSZone"].(string)
		n.RiskScore = 99
		n.Properties["Reason"] = fmt.Sprintf("Unused EIP %s referenced in DNS zone %s. Do not release due to DNS conflict.", n.IDStr(), zone)
		return true
	}

	n.RiskScore = 20
	n.Properties["Reason"] = "Safe to Release: Unused EIP (Not in Route53)."
	n.Properties["Warning"] = "Verify external DNS manually."
	return true
}

func (h *NetworkForensicsHeuristic) analyzeALB(n *graph.Node) bool {
	reqs, _ := n.Properties["SumRequests7d"].(float64)
	redirect, _ := n.Properties["IsRedirectOnly"].(bool)

	if reqs > 0 || redirect {
		return false
	}

	n.IsWaste = true
	n.RiskScore = 60
	n.Properties["Reason"] = "Unused ALB: 0 Requests in 7 days."
	n.Cost = 16.0

	if hasWAF, _ := n.Properties["HasWAF"].(bool); hasWAF {
		waf, _ := n.Properties["WAFCostEst"].(float64)
		n.Cost += waf
		n.Properties["Reason"] = fmt.Sprintf("Unused ALB + Attached WAF ($%.2f/mo waste).", n.Cost)
	}
	return true
}

func (h *NetworkForensicsHeuristic) analyzeVPCEP(n *graph.Node) bool {
	bytes, _ := n.Properties["SumBytesProcessed30d"].(float64)
	if bytes == 0 {
		n.IsWaste = true
		n.RiskScore = 70
		n.Properties["Reason"] = "Unused VPC Endpoint: Processed 0 bytes in 30 days."
		n.Cost = 7.0
		return true
	}
	return false
}
