package heuristics

import (
	"context"
	"fmt"
	"math"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// DataForensicsHeuristic identifies underutilized data layer resources.
type DataForensicsHeuristic struct{}

// Name returns the unique identifier for the heuristic.
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
	// Tri-Metric Analysis:
	// Checks for zero operations (hits/misses), low network activity, and idle CPU.
	// This combination strongly indicates an unused cache cluster.
	
	hits := getFloat(node, "SumHits7d")
	misses := getFloat(node, "SumMisses7d")
	netIn := getFloat(node, "SumNetworkBytesIn7d")
	cpu := getFloat(node, "MaxCPU7d")

	totalOps := hits + misses
	netThreshold := 5.0 * 1024 * 1024 // 5MB

	if totalOps == 0 && netIn < netThreshold && cpu < 2.0 {
		node.IsWaste = true
		node.IsWaste = true
		node.RiskScore = 9 // High confidence due to zero activity across all metrics.
		node.Properties["Reason"] = "Idle Cache Cluster: Zero hits/misses, negligible network, and idle CPU."
		// Estimated cost based on node configuration.
		node.Cost = 50.0
	}
}

func (h *DataForensicsHeuristic) analyzeRedshift(node *graph.Node) {
	// Checks if the cluster has processed any queries in the last 24 hours.
	// Zero queries suggests the cluster can be paused to save costs.
	
	queries := getFloat(node, "SumQueries24h")
	conns := getFloat(node, "MaxConnections24h")
	
	if queries == 0 {
		node.IsWaste = true
		node.RiskScore = 8
		
		if conns > 0 {
			node.Properties["Reason"] = "Idle (Connected): Zero queries in 24h, but active connections present. Consideration: Pause cluster."
		} else {
			node.Properties["Reason"] = "Redshift Pause Candidate: Zero queries in 24h. Recommendation: Pause cluster."
		}
		
		node.Cost = 200.0
	}
}

func (h *DataForensicsHeuristic) analyzeDynamoDB(node *graph.Node) {
	// Checks for significantly over-provisioned capacity.
	// Low utilization (< 15%) indicates wasted spend on provisioned throughput.
	
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
			node.Properties["Reason"] = fmt.Sprintf("Auto-Scaling Misconfiguration: Utilization %.1f%%. Recommendation: Lower minimum capacity.", minUtil)
		} else {
			// Suggest switching to On-Demand for low/sporadic usage patterns.
			node.Properties["Reason"] = fmt.Sprintf("Excessive Provisioned Capacity: Utilization %.1f%%. Recommendation: Switch to On-Demand.", minUtil)
		}
		
		node.Cost = 10.0
	}
}

func getFloat(n *graph.Node, key string) float64 {
	if v, ok := n.Properties[key].(float64); ok {
		return v
	}
	return 0.0
}
