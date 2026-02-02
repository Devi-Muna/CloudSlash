package graph

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/sys/intern"
)

var ErrGraphClosed = errors.New("graph is closed")

type EdgeType string

const (
	EdgeTypeAttachedTo EdgeType = "AttachedTo"
	EdgeTypeSecuredBy  EdgeType = "SecuredBy"
	EdgeTypeContains   EdgeType = "Contains"
	EdgeTypeRuns       EdgeType = "Runs" // Container orchestration.
	EdgeTypeFlowsTo    EdgeType = "FlowsTo"
	EdgeTypeUses       EdgeType = "Uses" // Dependency.
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
	ID             uint32
	Type           uint32
	Properties     map[string]interface{}
	TypedData      interface{} // Underlying resource struct.
	IsWaste        bool
	WasteReason    string
	Justified      bool
	Justification  string
	Ignored        bool
	RiskScore      int
	Cost           float64
	SourceLocation string
	Reachability   ReachabilityState
}

// IDStr returns the string representation of the Node ID.
func (n *Node) IDStr() string {
	return intern.GetStr(n.ID)
}

// TypeStr returns the string representation of the Node Type.
func (n *Node) TypeStr() string {
	return intern.GetStr(n.Type)
}

type GraphMetadata struct {
	Partial      bool
	FailedScopes []ScopeError
}

type ScopeError struct {
	Scope string
	Error string
}

type GraphOp struct {
	Kind      string // "Node" or "Edge"
	ID        string // For Node ops, the string ID
	Type      string // For Node ops, the string Type
	Props     map[string]interface{}
	TypedData interface{} // For Node ops
	SourceID  string      // For Edge ops, the string SourceID
	TargetID  string      // For Edge ops, the string TargetID
	EdgeType  EdgeType
	Weight    int
}

type Graph struct {
	Mu       sync.RWMutex
	Store    GraphStore
	Metadata GraphMetadata

	// Optimization
	DSU *UnionFind

	// Pipeline
	opChan    chan GraphOp
	buildDone chan struct{}
	quitChan  chan struct{}
	closed    bool
}

func NewGraph() *Graph {
	g := &Graph{
		Store:     NewMemoryStore(),
		Metadata:  GraphMetadata{Partial: false},
		DSU:       NewUnionFind(1024), // Initial capacity
		opChan:    make(chan GraphOp, 10000),
		buildDone: make(chan struct{}),
		quitChan:  make(chan struct{}),
		closed:    false,
	}

	// Start builder goroutine
	go g.builderLoop()
	return g
}

// CloseAndWait finalizes the graph.
// It uses a sentinel value to stop the builder loop safely without closing the channel,
// preventing panics in concurrent writers.
func (g *Graph) CloseAndWait() {
	g.Mu.Lock()
	if g.closed {
		g.Mu.Unlock()
		return
	}
	g.closed = true
	g.Mu.Unlock()

	// Signal exit via dedicated channel (non-blocking)
	close(g.quitChan)
	<-g.buildDone
}

func (g *Graph) builderLoop() {
	defer close(g.buildDone)

	// Op handler closure
	handle := func(op GraphOp) {
		g.Mu.Lock()
		switch op.Kind {
		case "Node":
			g.unsafeAddNode(op.ID, op.Type, op.Props, op.TypedData)
		case "Edge":
			g.unsafeAddEdge(op.SourceID, op.TargetID, op.EdgeType, op.Weight)
		}
		g.Mu.Unlock()
	}

	for {
		select {
		case <-g.quitChan:
			// Drain remaining ops (best effort)
			for len(g.opChan) > 0 {
				select {
				case op := <-g.opChan:
					handle(op)
				default:
					// Empty
				}
			}
			return

		case op := <-g.opChan:
			handle(op)
		}
	}
}

func (g *Graph) AddError(scope string, err error) {
	// Lock required.
	g.Mu.Lock()
	defer g.Mu.Unlock()

	g.Metadata.FailedScopes = append(g.Metadata.FailedScopes, ScopeError{
		Scope: scope,
		Error: err.Error(),
	})
}

func (g *Graph) AddNode(id, resourceType string, props map[string]interface{}) error {
	return g.AddTypedNode(id, resourceType, props, nil)
}

func (g *Graph) AddTypedNode(id, resourceType string, props map[string]interface{}, typedData interface{}) error {
	g.Mu.RLock()
	if g.closed {
		g.Mu.RUnlock()
		return ErrGraphClosed
	}
	g.Mu.RUnlock()

	if id == "" {
		return nil
	}
	// Queue operation; safe for concurrent use.
	g.opChan <- GraphOp{
		Kind:      "Node",
		ID:        id,
		Type:      resourceType,
		Props:     props,
		TypedData: typedData,
	}
	return nil
}

func (g *Graph) AddEdge(sourceID, targetID string) error {
	return g.AddTypedEdge(sourceID, targetID, EdgeTypeUnknown, 1)
}

func (g *Graph) AddTypedEdge(sourceID, targetID string, edgeType EdgeType, weight int) error {
	g.Mu.RLock()
	if g.closed {
		g.Mu.RUnlock()
		return ErrGraphClosed
	}
	g.Mu.RUnlock()

	if sourceID == "" || targetID == "" {
		return nil
	}
	// Queue operation; safe for concurrent use.
	g.opChan <- GraphOp{
		Kind:     "Edge",
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: edgeType,
		Weight:   weight,
	}
	return nil
}

// unsafeAddNode delegates to Store.
func (g *Graph) unsafeAddNode(idStr, resourceTypeStr string, props map[string]interface{}, typedData interface{}) {
	id := intern.Get(idStr)
	resourceType := intern.Get(resourceTypeStr)

	// Check existence via Store
	if existingID, ok := g.Store.GetNodeID(idStr); ok {
		g.Store.UpdateNode(existingID, func(n *Node) {
			for k, v := range props {
				n.Properties[k] = v
			}
			if typedData != nil {
				n.TypedData = typedData
			}
			if n.Type == intern.Get("Unknown") && resourceType != intern.Get("Unknown") {
				n.Type = resourceType
			}
		})
	} else {
		// Create new
		node := &Node{
			ID:         id,
			Type:       resourceType,
			Properties: props,
			TypedData:  typedData,
		}
		idx := g.Store.AddNode(node)

		// Ensure DSU capacity
		g.DSU.Resize(int(idx) + 1)
	}
}

// unsafeAddEdge delegates to Store.
func (g *Graph) unsafeAddEdge(sourceIDStr, targetIDStr string, edgeType EdgeType, weight int) {
	// Auto-vivify missing nodes.

	var srcIdx, dstIdx uint32

	if idx, ok := g.Store.GetNodeID(sourceIDStr); ok {
		srcIdx = idx
	} else {
		sid := intern.Get(sourceIDStr)
		node := &Node{ID: sid, Type: intern.Get("Unknown"), Properties: make(map[string]interface{})}
		srcIdx = g.Store.AddNode(node)
		g.DSU.Resize(int(srcIdx) + 1)
	}

	if idx, ok := g.Store.GetNodeID(targetIDStr); ok {
		dstIdx = idx
	} else {
		tid := intern.Get(targetIDStr)
		node := &Node{ID: tid, Type: intern.Get("Unknown"), Properties: make(map[string]interface{})}
		dstIdx = g.Store.AddNode(node)
		g.DSU.Resize(int(dstIdx) + 1)
	}

	edge := Edge{
		TargetID: dstIdx,
		Type:     edgeType,
		Weight:   weight,
		Metadata: make(map[string]interface{}),
	}
	g.Store.AddEdge(srcIdx, edge)

	// Validated merge.
	g.DSU.Union(int(srcIdx), int(dstIdx))
}

// GetNodes returns all nodes.
func (g *Graph) GetNodes() []*Node {
	g.Mu.RLock()
	defer g.Mu.RUnlock()
	return g.Store.GetAllNodes()
}

// GetID delegates to Store.
func (g *Graph) GetID(internedID uint32) (uint32, bool) {
	// Get ID by interned value.
	return g.Store.GetNodeID(intern.GetStr(internedID))
}

// GetNode delegates.
func (g *Graph) GetNode(idStr string) *Node {
	return g.Store.GetNodeByStringID(idStr)
}

// GetNodeByID delegates.
func (g *Graph) GetNodeByID(idx uint32) *Node {
	return g.Store.GetNode(idx)
}

func (g *Graph) GetEdges(nodeIdx uint32) []Edge {
	return g.Store.GetEdges(nodeIdx)
}

func (g *Graph) GetReverseEdges(nodeIdx uint32) []Edge {
	return g.Store.GetReverseEdges(nodeIdx)
}

// GetConnectedComponent uses BFS.
func (g *Graph) GetConnectedComponent(startIDStr string) []*Node {
	// Use store edges.
	startNode := g.Store.GetNodeByStringID(startIDStr)
	if startNode == nil {
		return nil
	}

	startIdx := startNode.Index
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

		node := g.Store.GetNode(currentIdx)
		if node != nil {
			component = append(component, node)
		}

		// Forward
		for _, edge := range g.Store.GetEdges(currentIdx) {
			if !visited[edge.TargetID] {
				queue = append(queue, edge.TargetID)
			}
		}

		// Reverse
		for _, edge := range g.Store.GetReverseEdges(currentIdx) {
			if !visited[edge.TargetID] {
				queue = append(queue, edge.TargetID)
			}
		}
	}
	return component
}

// AreConnected checks connectivity via DSU.
func (g *Graph) AreConnected(id1, id2 string) bool {
	idx1, ok1 := g.Store.GetNodeID(id1)
	idx2, ok2 := g.Store.GetNodeID(id2)

	if !ok1 || !ok2 {
		return false
	}

	return g.DSU.Connected(int(idx1), int(idx2))
}

func (g *Graph) MarkWaste(idStr string, score int) {
	// Mutex required for thread-safe store updates during concurrent heuristic analysis.
	g.Mu.Lock()
	defer g.Mu.Unlock()

	idx, ok := g.Store.GetNodeID(idStr)
	if !ok {
		return
	}

	g.Store.UpdateNode(idx, func(node *Node) {
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
			}
		}
		node.IsWaste = true
		node.RiskScore = score
	})
}

func (g *Graph) GetDownstream(id string) []string {
	idx, ok := g.Store.GetNodeID(id)
	if !ok {
		return nil
	}

	var downstream []string
	for _, e := range g.Store.GetEdges(idx) {
		if node := g.Store.GetNode(e.TargetID); node != nil {
			downstream = append(downstream, node.IDStr())
		}
	}
	return downstream
}

func (g *Graph) GetUpstream(id string) []string {
	idx, ok := g.Store.GetNodeID(id)
	if !ok {
		return nil
	}

	var upstream []string
	for _, e := range g.Store.GetReverseEdges(idx) {
		if node := g.Store.GetNode(e.TargetID); node != nil {
			upstream = append(upstream, node.IDStr())
		}
	}
	return upstream
}

func (g *Graph) DumpStats() string {
	// Basic stats.
	count := g.Store.NodeCount()
	return fmt.Sprintf("Nodes: %d | Storage: Memory", count)
}
