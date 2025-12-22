package graph

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"sync"
)

// EdgeType defines the semantic relationship between resources.
type EdgeType string

const (
	EdgeTypeAttachedTo EdgeType = "AttachedTo" // e.g., EBS -> EC2
	EdgeTypeSecuredBy  EdgeType = "SecuredBy"  // e.g., EC2 -> SG
	EdgeTypeContains   EdgeType = "Contains"   // e.g., VPC -> Subnet
	EdgeTypeFlowsTo    EdgeType = "FlowsTo"    // e.g., ALB -> TargetGroup
	EdgeTypeUnknown    EdgeType = "Unknown"
)

// Edge represents a directed connection with context.
type Edge struct {
	TargetID string
	Type     EdgeType
	Weight   int // 1-100, strength of dependency
}

// Node represents a resource in the infrastructure graph.
type Node struct {
	ID            string                 // Unique Identifier (ARN)
	Type          string                 // Resource Type (e.g., "AWS::EC2::Instance")
	Properties    map[string]interface{} // Resource attributes
	IsWaste       bool                   // Flagged as waste?
	Justified     bool                   // Is this accepted/known waste?
	Justification string                 // Reason for justification
	RiskScore     int                    // 0-100
	Cost          float64                // Monthly cost estimate
	SourceLocation string                // e.g. "storage.tf:24"
}

// Graph represents the infrastructure topology as a Weighted DAG.
type Graph struct {
	Mu           sync.RWMutex
	Nodes        map[string]*Node
	Edges        map[string][]Edge // ID -> []Edge (Forward Dependencies)
	ReverseEdges map[string][]Edge // ID -> []Edge (Reverse Dependencies)
}

// NewGraph creates a new empty graph.
func NewGraph() *Graph {
	return &Graph{
		Nodes:        make(map[string]*Node),
		Edges:        make(map[string][]Edge),
		ReverseEdges: make(map[string][]Edge),
	}
}

// AddNode adds a resource to the graph. Structure is idempotent.
func (g *Graph) AddNode(id, resourceType string, props map[string]interface{}) {
	if id == "" {
		return
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	if node, exists := g.Nodes[id]; exists {
		// Merge properties if node exists (Last Write Wins for conflicts)
		for k, v := range props {
			node.Properties[k] = v
		}
		// Update type if it was unknown
		if node.Type == "Unknown" && resourceType != "Unknown" {
			node.Type = resourceType
		}
	} else {
		g.Nodes[id] = &Node{
			ID:         id,
			Type:       resourceType,
			Properties: props,
		}
	}
}

// AddEdge adds a directed edge from source to target with default type.
// Maintained for backward compatibility.
func (g *Graph) AddEdge(sourceID, targetID string) {
	g.AddTypedEdge(sourceID, targetID, EdgeTypeUnknown, 1)
}

// AddTypedEdge adds a semantic relationship to the graph.
func (g *Graph) AddTypedEdge(sourceID, targetID string, edgeType EdgeType, weight int) {
	if sourceID == "" || targetID == "" {
		return
	}

	g.Mu.Lock()
	defer g.Mu.Unlock()

	// Ensure nodes exist (create placeholders if not)
	if _, ok := g.Nodes[sourceID]; !ok {
		g.Nodes[sourceID] = &Node{ID: sourceID, Type: "Unknown", Properties: make(map[string]interface{})}
	}
	if _, ok := g.Nodes[targetID]; !ok {
		g.Nodes[targetID] = &Node{ID: targetID, Type: "Unknown", Properties: make(map[string]interface{})}
	}

	// Add Forward Edge
	// Check for duplicates to prevent graph explosion
	exists := false
	for _, e := range g.Edges[sourceID] {
		if e.TargetID == targetID && e.Type == edgeType {
			exists = true
			break
		}
	}
	if !exists {
		g.Edges[sourceID] = append(g.Edges[sourceID], Edge{TargetID: targetID, Type: edgeType, Weight: weight})
	}

	// Add Reverse Edge
	revExists := false
	for _, e := range g.ReverseEdges[targetID] {
		if e.TargetID == sourceID && e.Type == edgeType {
			revExists = true
			break
		}
	}
	if !revExists {
		g.ReverseEdges[targetID] = append(g.ReverseEdges[targetID], Edge{TargetID: sourceID, Type: edgeType, Weight: weight})
	}
}

// GetConnectedComponent returns all nodes reachable from startID (BFS).
// Useful for finding all resources in a VPC or related to a specific security group.
func (g *Graph) GetConnectedComponent(startID string) []*Node {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	visited := make(map[string]bool)
	queue := []string{startID}
	var component []*Node

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		if node, ok := g.Nodes[currentID]; ok {
			component = append(component, node)
		}

		// Traverse forward edges
		for _, edge := range g.Edges[currentID] {
			if !visited[edge.TargetID] {
				queue = append(queue, edge.TargetID)
			}
		}

		// Traverse backward edges (undirected connectivity check)
		for _, edge := range g.ReverseEdges[currentID] {
			if !visited[edge.TargetID] {
				queue = append(queue, edge.TargetID)
			}
		}
	}

	return component
}

// MarkWaste flags a node and optionally its dependencies as waste.
func (g *Graph) MarkWaste(id string, score int) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	if node, ok := g.Nodes[id]; ok {
		// Safe List Logic (cloudslash:ignore)
		if tags, ok := node.Properties["Tags"].(map[string]string); ok {
			if val, ok := tags["cloudslash:ignore"]; ok {
				val = strings.ToLower(strings.TrimSpace(val))
				
				// 1. Ignore Forever
				if val == "true" {
					return
				}
				
				// 2. Cost-Based Ignore (cost<10.50)
				if strings.HasPrefix(val, "cost<") {
					limitStr := strings.TrimPrefix(val, "cost<")
					if limit, err := strconv.ParseFloat(limitStr, 64); err == nil {
						if node.Cost < limit {
							return
						}
					}
				}

				// 3. Justified Waste (justified:compliance)
				if strings.HasPrefix(val, "justified:") {
					node.IsWaste = true
					node.Justified = true
					node.Justification = strings.TrimPrefix(val, "justified:")
					node.RiskScore = score
					return
				}

				// 4. Ignore Until Date (YYYY-MM-DD)
				if ignoreUntil, err := time.Parse("2006-01-02", val); err == nil {
					if time.Now().Before(ignoreUntil) {
						return
					}
				}

				// 5. Grace Period / TTL (e.g., "ignore:3d" => Ignore if younger than 3 days)
				// Useful for ephemeral dev resources that shouldn't be flagged immediately.
				// Requires "LaunchTime" property to be set by scanner.
				if strings.HasSuffix(val, "d") || strings.HasSuffix(val, "h") {
					// Parse "30d" -> 720h manually
					var hours int
					var conversionErr error
					
					if strings.HasSuffix(val, "d") {
						daysStr := strings.TrimSuffix(val, "d")
						days, err := strconv.Atoi(daysStr)
						if err == nil {
							hours = days * 24
						} else {
							conversionErr = err
						}
					} else { // "h"
						hoursStr := strings.TrimSuffix(val, "h")
						h, err := strconv.Atoi(hoursStr)
						if err == nil {
							hours = h
						} else {
							conversionErr = err
						}
					}

					if conversionErr == nil {
						// Look for resource creation time
						// Try common keys: LaunchTime (EC2), CreateTime (EBS/S3), StartTime (RDS)
						var launchTime time.Time
						foundTime := false
						
						timeKeys := []string{"LaunchTime", "CreateTime", "StartTime", "Created"}
						for _, key := range timeKeys {
							if tVal, ok := node.Properties[key].(time.Time); ok {
								launchTime = tVal
								foundTime = true
								break
							}
							// Handle string timestamps if needed? (Scanners usually use native types)
						}

						if foundTime {
							age := time.Since(launchTime)
							if age.Hours() < float64(hours) {
								return // IGNORED: Within grace period
							}
						}
					}
				}
			}
		}

		node.IsWaste = true
		node.RiskScore = score
	}
}

// GetDownstream returns simple string slice of downstream IDs for compatibility.
func (g *Graph) GetDownstream(id string) []string {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	var downstream []string
	if edges, ok := g.Edges[id]; ok {
		for _, e := range edges {
			downstream = append(downstream, e.TargetID)
		}
	}
	return downstream
}

// GetUpstream returns simple string slice of upstream IDs for compatibility.
func (g *Graph) GetUpstream(id string) []string {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	var upstream []string
	if edges, ok := g.ReverseEdges[id]; ok {
		for _, e := range edges {
			upstream = append(upstream, e.TargetID)
		}
	}
	return upstream
}

// DumpStats returns graph statistics for the TUI.
func (g *Graph) DumpStats() string {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return fmt.Sprintf("Nodes: %d | Edges: %d", len(g.Nodes), len(g.Edges))
}
