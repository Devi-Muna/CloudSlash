package heuristics

import (
	"context"
	"fmt"
	"math"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

// DataForensicsHeuristic checks data resources.
type DataForensicsHeuristic struct{}

func (h *DataForensicsHeuristic) Name() string {
	return "DataForensics"
}

func (h *DataForensicsHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	return h.Analyze(g), nil
}

func (h *DataForensicsHeuristic) Analyze(g *graph.Graph) *HeuristicStats {
	stats := &HeuristicStats{}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.GetNodes() {
		var isWaste bool
		switch node.TypeStr() {
		case "aws_elasticache_cluster":
			isWaste = h.analyzeElasticache(node)
		case "aws_redshift_cluster":
			isWaste = h.analyzeRedshift(node)
		case "aws_dynamodb_table":
			isWaste = h.analyzeDynamoDB(node)
		}

		if isWaste {
			stats.ItemsFound++
			stats.ProjectedSavings += node.Cost
		}
	}
	return stats
}

func (h *DataForensicsHeuristic) analyzeElasticache(node *graph.Node) bool {
	// Tri-metric analysis.
	// Checks usage metrics.

	hits := getFloat(node, "SumHits7d")
	misses := getFloat(node, "SumMisses7d")
	netIn := getFloat(node, "SumNetworkBytesIn7d")
	cpu := getFloat(node, "MaxCPU7d")

	totalOps := hits + misses
	netThreshold := 5.0 * 1024 * 1024 // 5MB

	if totalOps == 0 && netIn < netThreshold && cpu < 2.0 {
		node.IsWaste = true
		node.RiskScore = 9 // High confidence due to zero activity across all metrics.
		node.Properties["Reason"] = "Idle Cache Cluster: Zero hits/misses, negligible network, and idle CPU."
		// Est. cost.
		node.Cost = 50.0
		return true
	}
	return false
}

func (h *DataForensicsHeuristic) analyzeRedshift(node *graph.Node) bool {
	// Check queries.

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
		return true
	}
	return false
}

func (h *DataForensicsHeuristic) analyzeDynamoDB(node *graph.Node) bool {
	// Check capacity.
	// < 15% util is waste.

	rcu := getFloat(node, "ProvisionedRCU")
	wcu := getFloat(node, "ProvisionedWCU")

	if rcu == 0 || wcu == 0 {
		return false
	} // Skip On-Demand.

	consumedRCU := getFloat(node, "AvgConsumedRCU30d")
	consumedWCU := getFloat(node, "AvgConsumedWCU30d")

	utilR := (consumedRCU / rcu) * 100
	utilW := (consumedWCU / wcu) * 100

	minUtil := math.Min(utilR, utilW)

	// Skip free tier.
	if rcu <= 25 && wcu <= 25 {
		return false // Ignore
	}

	if minUtil < 15.0 {
		hasAS, _ := node.Properties["HasAutoScaling"].(bool)

		node.IsWaste = true
		node.RiskScore = 7

		if hasAS {
			node.Properties["Reason"] = fmt.Sprintf("Auto-Scaling Misconfiguration: Utilization %.1f%%. Recommendation: Lower minimum capacity.", minUtil)
		} else {
			// Suggest On-Demand.
			node.Properties["Reason"] = fmt.Sprintf("Excessive Provisioned Capacity: Utilization %.1f%%. Recommendation: Switch to On-Demand.", minUtil)
		}

		node.Cost = 10.0
		return true
	}
	return false
}

func getFloat(n *graph.Node, key string) float64 {
	if v, ok := n.Properties[key].(float64); ok {
		return v
	}
	return 0.0
}
