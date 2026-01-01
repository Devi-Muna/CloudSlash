package heuristics

import (
	"context"
	"fmt"
	"github.com/DrSkyle/cloudslash/internal/graph"
)

type NetworkForensicsHeuristic struct{}

func (h *NetworkForensicsHeuristic) Name() string { return "NetworkForensics" }

func (h *NetworkForensicsHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	h.Analyze(g)
	return nil
}

func (h *NetworkForensicsHeuristic) Analyze(g *graph.Graph) {
	for _, n := range g.Nodes {
		switch n.Type {
		case "aws_nat_gateway":
			h.analyzeNAT(n, g)
		case "aws_eip":
			h.analyzeEIP(n)
		case "aws_alb":
			h.analyzeALB(n)
		case "aws_vpc_endpoint":
			h.analyzeVPCEP(n)
		}
	}
}

func (h *NetworkForensicsHeuristic) analyzeNAT(n *graph.Node, g *graph.Graph) {
	conns, _ := n.Properties["SumConnections7d"].(float64)
	active, _ := n.Properties["ActiveUserENIs"].(int)

	if conns == 0 && active == 0 {
		n.IsWaste = true
		n.RiskScore = 90
		n.Properties["Reason"] = "Hollow NAT Gateway: Serves subnets with ZERO active instances. Traffic: 0."
		n.Cost = 32.0

		h.topo(g, n)
	}
}

func (h *NetworkForensicsHeuristic) topo(g *graph.Graph, nat *graph.Node) {
	subnets, ok := nat.Properties["EmptySubnets"].([]string)
	if !ok {
		return
	}

	for _, id := range subnets {
		g.Mu.Lock()
		g.Nodes[id] = &graph.Node{
			ID:      id,
			Type:    "aws_subnet",
			IsWaste: true,
			Properties: map[string]interface{}{
				"Reason":   "Empty Subnet (Linked to Hollow NAT)",
				"ParentID": nat.ID,
				"Name":     fmt.Sprintf("Subnet: %s (Empty)", id),
			},
		}
		g.Mu.Unlock()
	}

	if rtbs, ok := nat.Properties["RouteTables"].([]string); ok {
		for _, id := range rtbs {
			g.Mu.Lock()
			g.Nodes[id] = &graph.Node{
				ID:      id,
				Type:    "aws_route_table",
				IsWaste: true,
				Properties: map[string]interface{}{
					"Reason":   "Route Table targeting Hollow NAT",
					"ParentID": nat.ID,
					"Name":     fmt.Sprintf("Route Table: %s", id),
				},
			}
			g.Mu.Unlock()
		}
	}
}

func (h *NetworkForensicsHeuristic) analyzeEIP(n *graph.Node) {
	if assoc, _ := n.Properties["AssociationId"].(string); assoc != "" {
		return
	}

	n.IsWaste = true
	n.Cost = 3.5

	inDNS, _ := n.Properties["FoundInDNS"].(bool)
	if inDNS {
		zone, _ := n.Properties["DNSZone"].(string)
		n.RiskScore = 99
		n.Properties["Reason"] = fmt.Sprintf("DANGEROUS ZOMBIE: EIP %s is unused BUT hardcoded in DNS zone %s. Do NOT release. DNS Conflict.", n.ID, zone)
		return
	}

	n.RiskScore = 20
	n.Properties["Reason"] = "Safe to Release: Unused EIP (Not in Route53)."
	n.Properties["Warning"] = "Verify external DNS manually."
}

func (h *NetworkForensicsHeuristic) analyzeALB(n *graph.Node) {
	reqs, _ := n.Properties["SumRequests7d"].(float64)
	redirect, _ := n.Properties["IsRedirectOnly"].(bool)

	if reqs > 0 || redirect {
		return
	}

	n.IsWaste = true
	n.RiskScore = 60
	n.Properties["Reason"] = "Zombie ALB: 0 Requests in 7 days."
	n.Cost = 16.0

	if hasWAF, _ := n.Properties["HasWAF"].(bool); hasWAF {
		waf, _ := n.Properties["WAFCostEst"].(float64)
		n.Cost += waf
		n.Properties["Reason"] = fmt.Sprintf("Zombie ALB + Attached WAF ($%.2f/mo waste).", n.Cost)
	}
}

func (h *NetworkForensicsHeuristic) analyzeVPCEP(n *graph.Node) {
	bytes, _ := n.Properties["SumBytesProcessed30d"].(float64)
	if bytes == 0 {
		n.IsWaste = true
		n.RiskScore = 70
		n.Properties["Reason"] = "Hidden Leech: VPC Endpoint processed 0 bytes in 30 days."
		n.Cost = 7.0
	}
}
