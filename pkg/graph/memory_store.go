package graph

import (
	"sync"

	"github.com/DrSkyle/cloudslash/v2/pkg/sys/intern"
)

// MemoryStore is an in-memory graph storage.
type MemoryStore struct {
	mu           sync.RWMutex
	nodes        []*Node
	edges        [][]Edge
	reverseEdges [][]Edge
	idMap        map[uint32]uint32 // Interned String ID -> Index
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nodes:        make([]*Node, 0, 1000),
		edges:        make([][]Edge, 0, 1000),
		reverseEdges: make([][]Edge, 0, 1000),
		idMap:        make(map[uint32]uint32),
	}
}

func (s *MemoryStore) AddNode(node *Node) uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check existence.
	if idx, ok := s.idMap[node.ID]; ok {
		return idx
	}

	idx := uint32(len(s.nodes))
	node.Index = idx
	s.nodes = append(s.nodes, node)
	s.edges = append(s.edges, nil)
	s.reverseEdges = append(s.reverseEdges, nil)
	s.idMap[node.ID] = idx
	return idx
}

func (s *MemoryStore) GetNode(index uint32) *Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if int(index) < len(s.nodes) {
		return s.nodes[index]
	}
	return nil
}

func (s *MemoryStore) GetNodeID(stringID string) (uint32, bool) {
	id := intern.Get(stringID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx, ok := s.idMap[id]
	return idx, ok
}

func (s *MemoryStore) GetNodeByStringID(stringID string) *Node {
	id := intern.Get(stringID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx, ok := s.idMap[id]
	if !ok {
		return nil
	}
	if int(idx) < len(s.nodes) {
		return s.nodes[idx]
	}
	return nil
}

func (s *MemoryStore) UpdateNode(index uint32, update func(*Node)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if int(index) < len(s.nodes) {
		update(s.nodes[index])
	}
}

func (s *MemoryStore) NodeCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.nodes)
}

func (s *MemoryStore) GetAllNodes() []*Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return copy.
	result := make([]*Node, len(s.nodes))
	copy(result, s.nodes)
	return result
}

func (s *MemoryStore) AddEdge(sourceIndex uint32, edge Edge) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if int(sourceIndex) >= len(s.edges) {
		return
	}
	if int(edge.TargetID) >= len(s.reverseEdges) {
		return
	}

	// Check duplicates.
	for _, e := range s.edges[sourceIndex] {
		if e.TargetID == edge.TargetID && e.Type == edge.Type {
			return
		}
	}

	s.edges[sourceIndex] = append(s.edges[sourceIndex], edge)

	// Add reverse edge.
	revEdge := Edge{
		TargetID: sourceIndex,
		Type:     edge.Type,
		Weight:   edge.Weight,
		Metadata: edge.Metadata,
	}
	s.reverseEdges[edge.TargetID] = append(s.reverseEdges[edge.TargetID], revEdge)
}

func (s *MemoryStore) GetEdges(sourceIndex uint32) []Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if int(sourceIndex) < len(s.edges) {
		// Return copy.
		res := make([]Edge, len(s.edges[sourceIndex]))
		copy(res, s.edges[sourceIndex])
		return res
	}
	return nil
}

func (s *MemoryStore) GetReverseEdges(targetIndex uint32) []Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if int(targetIndex) < len(s.reverseEdges) {
		res := make([]Edge, len(s.reverseEdges[targetIndex]))
		copy(res, s.reverseEdges[targetIndex])
		return res
	}
	return nil
}
