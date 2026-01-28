package graph

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/sys/intern"
)

type EdgeType string

const (
	EdgeTypeAttachedTo EdgeType = "AttachedTo"
	EdgeTypeSecuredBy  EdgeType = "SecuredBy"
	EdgeTypeContains   EdgeType = "Contains"
	EdgeTypeRuns       EdgeType = "Runs" // Added for ECS Task relationships
	EdgeTypeFlowsTo    EdgeType = "FlowsTo"
	EdgeTypeUses       EdgeType = "Uses"    // Added for Instance->AMI or similar dependencies
	EdgeTypeUnknown    EdgeType = "Unknown"
)

type ReachabilityState string

const (
	ReachabilityUnknown    ReachabilityState = "Unknown"
	ReachabilityReachable  ReachabilityState = "Reachable"
	ReachabilityDarkMatter ReachabilityState = "DarkMatter"
)

type Edge struct {
	TargetID uint32
	Type     EdgeType
	Weight   int
	Metadata map[string]interface{}
}

type Node struct {
	Index          uint32
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
	Reachability   ReachabilityState
}

type GraphMetadata struct {
	Partial      bool
	FailedScopes []ScopeError
}

type ScopeError struct {
	Scope string
	Error string
}

type Graph struct {
	Mu           sync.RWMutex
	Nodes        []*Node
	Edges        [][]Edge
	ReverseEdges [][]Edge
	idMap        map[string]uint32
	Metadata     GraphMetadata
}

func NewGraph() *Graph {
	return &Graph{
		Nodes:        make([]*Node, 0, 1000),
		Edges:        make([][]Edge, 0, 1000),
		ReverseEdges: make([][]Edge, 0, 1000),
		idMap:        make(map[string]uint32),
		Metadata:     GraphMetadata{Partial: false},
	}
}

func (g *Graph) AddError(scope string, err error) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	
	g.Metadata.Partial = true
	g.Metadata.FailedScopes = append(g.Metadata.FailedScopes, ScopeError{
		Scope: scope,
		Error: err.Error(),
	})
}

// GetID returns the internal integer ID for a given string ID (ARN).
// Returns 0, false if not found.
func (g *Graph) GetID(id string) (uint32, bool) {
	// Read lock.
	g.Mu.RLock()
	idx, ok := g.idMap[id]
	g.Mu.RUnlock()
	return idx, ok
}

// GetNode returns the Node pointer for a given string ID.
// Helper method for node retrieval.
func (g *Graph) GetNode(id string) *Node {
	idx, ok := g.GetID(id)
	if !ok {
		return nil
	}
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	if int(idx) < len(g.Nodes) {
		return g.Nodes[idx]
	}
	return nil
}

// GetNodeByID returns the Node pointer for a given internal integer ID.
func (g *Graph) GetNodeByID(idx uint32) *Node {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	if int(idx) < len(g.Nodes) {
		return g.Nodes[idx]
	}
	return nil
}

func (g *Graph) AddNode(id, resourceType string, props map[string]interface{}) {
	if id == "" {
		return
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	// Deduplicate resource type string.
	resourceType = intern.String(resourceType)

	if idx, exists := g.idMap[id]; exists {
		// Update existing node properties.
		node := g.Nodes[idx]
		for k, v := range props {
			node.Properties[k] = v
		}
		if node.Type == intern.String("Unknown") && resourceType != intern.String("Unknown") {
			node.Type = resourceType
		}
	} else {
		// Initialize new node.
		newIdx := uint32(len(g.Nodes))
		g.idMap[id] = newIdx
		g.Nodes = append(g.Nodes, &Node{
			Index:      newIdx,
			ID:         id,
			Type:       resourceType,
			Properties: props,
		})
		// Resize edge slices.
		g.Edges = append(g.Edges, nil)
		g.ReverseEdges = append(g.ReverseEdges, nil)
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

	// Ensure nodes exist in graph.
	srcIdx, ok := g.idMap[sourceID]
	if !ok {
		srcIdx = uint32(len(g.Nodes))
		g.idMap[sourceID] = srcIdx
		g.Nodes = append(g.Nodes, &Node{Index: srcIdx, ID: sourceID, Type: intern.String("Unknown"), Properties: make(map[string]interface{})})
		g.Edges = append(g.Edges, nil)
		g.ReverseEdges = append(g.ReverseEdges, nil)
	}

	dstIdx, ok := g.idMap[targetID]
	if !ok {
		dstIdx = uint32(len(g.Nodes))
		g.idMap[targetID] = dstIdx
		g.Nodes = append(g.Nodes, &Node{Index: dstIdx, ID: targetID, Type: intern.String("Unknown"), Properties: make(map[string]interface{})})
		g.Edges = append(g.Edges, nil)
		g.ReverseEdges = append(g.ReverseEdges, nil)
	}

	// Add forward edge.
	exists := false
	for _, e := range g.Edges[srcIdx] {
		if e.TargetID == dstIdx && e.Type == edgeType {
			exists = true
			break
		}
	}
	if !exists {
		g.Edges[srcIdx] = append(g.Edges[srcIdx], Edge{
			TargetID: dstIdx,
			Type:     edgeType,
			Weight:   weight,
			Metadata: make(map[string]interface{}),
		})
	}

	// Add reverse edge.
	revExists := false
	for _, e := range g.ReverseEdges[dstIdx] {
		if e.TargetID == srcIdx && e.Type == edgeType {
			revExists = true
			break
		}
	}
	if !revExists {
		g.ReverseEdges[dstIdx] = append(g.ReverseEdges[dstIdx], Edge{
			TargetID: srcIdx,
			Type:     edgeType,
			Weight:   weight,
			Metadata: make(map[string]interface{}),
		})
	}
}

// GetConnectedComponent uses BFS to find all reachable nodes.
func (g *Graph) GetConnectedComponent(startID string) []*Node {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	startIdx, ok := g.idMap[startID]
	if !ok {
		return nil
	}

	visited := make(map[uint32]bool)
	queue := []uint32{startIdx}
	var component []*Node

	for len(queue) > 0 {
		currentIdx := queue[0]
		queue = queue[1:]

		if visited[currentIdx] {
			continue
		}
		visited[currentIdx] = true

		if int(currentIdx) < len(g.Nodes) {
			node := g.Nodes[currentIdx]
			component = append(component, node)
		}

		// Forward
		if int(currentIdx) < len(g.Edges) {
			for _, edge := range g.Edges[currentIdx] {
				if !visited[edge.TargetID] {
					queue = append(queue, edge.TargetID)
				}
			}
		}

		// Reverse
		if int(currentIdx) < len(g.ReverseEdges) {
			for _, edge := range g.ReverseEdges[currentIdx] {
				if !visited[edge.TargetID] {
					queue = append(queue, edge.TargetID)
				}
			}
		}
	}

	return component
}

func (g *Graph) MarkWaste(id string, score int) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	idx, ok := g.idMap[id]
	if !ok {
		return
	}
	node := g.Nodes[idx]

	// Check for ignore tags.
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

			// Check grace period.
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

func (g *Graph) GetDownstream(id string) []string {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	startIdx, ok := g.idMap[id]
	if !ok {
		return nil
	}

	var downstream []string
	if int(startIdx) < len(g.Edges) {
		for _, e := range g.Edges[startIdx] {
			// Resolve node IDs.
			if int(e.TargetID) < len(g.Nodes) {
				downstream = append(downstream, g.Nodes[e.TargetID].ID)
			}
		}
	}
	return downstream
}

func (g *Graph) GetUpstream(id string) []string {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	startIdx, ok := g.idMap[id]
	if !ok {
		return nil
	}

	var upstream []string
	if int(startIdx) < len(g.ReverseEdges) {
		for _, e := range g.ReverseEdges[startIdx] {
			if int(e.TargetID) < len(g.Nodes) {
				upstream = append(upstream, g.Nodes[e.TargetID].ID)
			}
		}
	}
	return upstream
}

func (g *Graph) DumpStats() string {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	
	totalEdges := 0
	for _, edges := range g.Edges {
		totalEdges += len(edges)
	}
	return fmt.Sprintf("Nodes: %d | Edges: %d", len(g.Nodes), totalEdges)
}
