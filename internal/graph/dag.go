package graph

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type EdgeType string

const (
	EdgeTypeAttachedTo EdgeType = "AttachedTo"
	EdgeTypeSecuredBy  EdgeType = "SecuredBy"
	EdgeTypeContains   EdgeType = "Contains"
	EdgeTypeFlowsTo    EdgeType = "FlowsTo"
	EdgeTypeUnknown    EdgeType = "Unknown"
)

type Edge struct {
	TargetID string
	Type     EdgeType
	Weight   int
}

type Node struct {
	ID             string
	Type           string
	Properties     map[string]interface{}
	IsWaste        bool
	Justified      bool
	Justification  string
	Ignored        bool
	RiskScore      int
	Cost           float64
	SourceLocation string
}

type Graph struct {
	Mu           sync.RWMutex
	Nodes        map[string]*Node
	Edges        map[string][]Edge
	ReverseEdges map[string][]Edge
}

func NewGraph() *Graph {
	return &Graph{
		Nodes:        make(map[string]*Node),
		Edges:        make(map[string][]Edge),
		ReverseEdges: make(map[string][]Edge),
	}
}

func (g *Graph) AddNode(id, resourceType string, props map[string]interface{}) {
	if id == "" {
		return
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	if node, exists := g.Nodes[id]; exists {
		for k, v := range props {
			node.Properties[k] = v
		}
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

func (g *Graph) AddEdge(sourceID, targetID string) {
	g.AddTypedEdge(sourceID, targetID, EdgeTypeUnknown, 1)
}

func (g *Graph) AddTypedEdge(sourceID, targetID string, edgeType EdgeType, weight int) {
	if sourceID == "" || targetID == "" {
		return
	}

	g.Mu.Lock()
	defer g.Mu.Unlock()

	if _, ok := g.Nodes[sourceID]; !ok {
		g.Nodes[sourceID] = &Node{ID: sourceID, Type: "Unknown", Properties: make(map[string]interface{})}
	}
	if _, ok := g.Nodes[targetID]; !ok {
		g.Nodes[targetID] = &Node{ID: targetID, Type: "Unknown", Properties: make(map[string]interface{})}
	}

	// Forward Edge
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

	// Reverse Edge
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

// GetConnectedComponent uses BFS to find all reachable nodes.
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

		for _, edge := range g.Edges[currentID] {
			if !visited[edge.TargetID] {
				queue = append(queue, edge.TargetID)
			}
		}

		for _, edge := range g.ReverseEdges[currentID] {
			if !visited[edge.TargetID] {
				queue = append(queue, edge.TargetID)
			}
		}
	}

	return component
}

func (g *Graph) MarkWaste(id string, score int) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	if node, ok := g.Nodes[id]; ok {
		// cloudslash:ignore logic
		if tags, ok := node.Properties["Tags"].(map[string]string); ok {
			if val, ok := tags["cloudslash:ignore"]; ok {
				val = strings.ToLower(strings.TrimSpace(val))

				if val == "true" {
					return
				}

				if strings.HasPrefix(val, "cost<") {
					limitStr := strings.TrimPrefix(val, "cost<")
					if limit, err := strconv.ParseFloat(limitStr, 64); err == nil {
						if node.Cost < limit {
							return
						}
					}
				}

				if strings.HasPrefix(val, "justified:") {
					node.IsWaste = true
					node.Justified = true
					node.Justification = strings.TrimPrefix(val, "justified:")
					node.RiskScore = score
					return
				}

				if ignoreUntil, err := time.Parse("2006-01-02", val); err == nil {
					if time.Now().Before(ignoreUntil) {
						return
					}
				}

				// Grace period (e.g., "30d")
				if strings.HasSuffix(val, "d") || strings.HasSuffix(val, "h") {
					hours := 0
					var err error

					if strings.HasSuffix(val, "d") {
						daysStr := strings.TrimSuffix(val, "d")
						days, _ := strconv.Atoi(daysStr)
						hours = days * 24
					} else {
						hoursStr := strings.TrimSuffix(val, "h")
						hours, _ = strconv.Atoi(hoursStr)
					}

					if err == nil {
						var launchTime time.Time
						foundTime := false

						for _, key := range []string{"LaunchTime", "CreateTime", "StartTime", "Created"} {
							if tVal, ok := node.Properties[key].(time.Time); ok {
								launchTime = tVal
								foundTime = true
								break
							}
						}

						if foundTime {
							if time.Since(launchTime).Hours() < float64(hours) {
								return
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

func (g *Graph) DumpStats() string {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return fmt.Sprintf("Nodes: %d | Edges: %d", len(g.Nodes), len(g.Edges))
}
