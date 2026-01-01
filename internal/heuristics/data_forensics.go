package heuristics

import (
	"context"
	"fmt"
	"math"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// DataForensicsHeuristic analyzes Data Layer services.
type DataForensicsHeuristic struct{}

func (h *DataForensicsHeuristic) Name() string {
	return "DataForensics"
}

func (h *DataForensicsHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	h.Analyze(g)
	return nil
}

func (h *DataForensicsHeuristic) Analyze(g *graph.Graph) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	
	for _, node := range g.Nodes {
		switch node.Type {
		case "aws_elasticache_cluster":
			h.analyzeElasticache(node)
		case "aws_redshift_cluster":
			h.analyzeRedshift(node)
		case "aws_dynamodb_table":
			h.analyzeDynamoDB(node)
		}
	}
}

func (h *DataForensicsHeuristic) analyzeElasticache(node *graph.Node) {
	// Tri-Metric Lock:
	// 1. Ops (Hits+Misses) == 0
	// 2. NetworkBytesIn < 5MB/week (Heartbeats)
	// 3. CPU < 2%
	
	hits := getFloat(node, "SumHits7d")
	misses := getFloat(node, "SumMisses7d")
	netIn := getFloat(node, "SumNetworkBytesIn7d")
	cpu := getFloat(node, "MaxCPU7d")

	totalOps := hits + misses
	netThreshold := 5.0 * 1024 * 1024 // 5MB

	if totalOps == 0 && netIn < netThreshold && cpu < 2.0 {
		node.IsWaste = true
		node.RiskScore = 9 // High confidence
		node.Properties["Reason"] = "Ghost Cache: 0 Hits/Misses, Idle CPU, Negligible Network Traffic."
		// Cost calc would utilize Node Config. Assuming generic cost for now unless Pricing API integrated.
		node.Cost = 50.0 // Placeholder
	}
}

func (h *DataForensicsHeuristic) analyzeRedshift(node *graph.Node) {
	// The Pause Button
	// 1. Queries (24h) == 0
	
	queries := getFloat(node, "SumQueries24h")
	conns := getFloat(node, "MaxConnections24h")
	
	if queries == 0 {
		node.IsWaste = true
		node.RiskScore = 8
		
		if conns > 0 {
			node.Properties["Reason"] = "Idle (Connected): 0 Queries in 24h, but active connections detected. Action: PAUSE."
		} else {
			node.Properties["Reason"] = "Redshift Pause Candidate: 0 Queries in 24h. Action: PAUSE."
		}
		
		// TODO: Check 'ClusterAvailabilityStatus' for legacy nodes
		node.Cost = 200.0 // Placeholder
	}
}

func (h *DataForensicsHeuristic) analyzeDynamoDB(node *graph.Node) {
	// Provisioned Bleed
	// Utilization < 15%
	
	rcu := getFloat(node, "ProvisionedRCU")
	wcu := getFloat(node, "ProvisionedWCU")
	
	if rcu == 0 || wcu == 0 { return } // Valid if On-Demand or error
	
	consumedRCU := getFloat(node, "AvgConsumedRCU30d")
	consumedWCU := getFloat(node, "AvgConsumedWCU30d")
	
	utilR := (consumedRCU / rcu) * 100
	utilW := (consumedWCU / wcu) * 100
	
	minUtil := math.Min(utilR, utilW)
	
	// Free Tier Check (< 25 units total provisioned is roughly free tier range)
	if rcu <= 25 && wcu <= 25 {
		return // Ignore
	}

	if minUtil < 15.0 {
		hasAS, _ := node.Properties["HasAutoScaling"].(bool)
		
		node.IsWaste = true
		node.RiskScore = 7
		
		if hasAS {
			node.Properties["Reason"] = fmt.Sprintf("Auto-Scaling Misconfig: Utilization %.1f%%. Lower MinCapacity.", minUtil)
		} else {
			// Breakeven Calc (Simplified)
			// Prov Cost: $0.00065 per RCU-hour -> ~$0.47/month per RCU
			// OnDemand: $1.25 per million units
			// Show savings?
			node.Properties["Reason"] = fmt.Sprintf("Provisioned Bleed: Utilization %.1f%%. Switch to On-Demand.", minUtil)
		}
		
		// Est Cost of waste = (Provisioned - Consumed) * CostPerUnit
		node.Cost = 10.0 // Placeholder
	}
}

func getFloat(n *graph.Node, key string) float64 {
	if v, ok := n.Properties[key].(float64); ok {
		return v
	}
	return 0.0
}
