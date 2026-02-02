package graph

import (
	"sync"
)

// UnionFind implements concurrent DSU.
// Supports amortized O(1) checks.
type UnionFind struct {
	parent []int
	rank   []int
	mu     sync.RWMutex
}

// NewUnionFind initializes DSU.
func NewUnionFind(n int) *UnionFind {
	parent := make([]int, n)
	rank := make([]int, n)
	for i := 0; i < n; i++ {
		parent[i] = i
	}
	return &UnionFind{parent: parent, rank: rank}
}

// Find returns set representative.
func (uf *UnionFind) Find(i int) int {
	uf.mu.Lock() // Lock for path compression
	defer uf.mu.Unlock()
	return uf.findInternal(i)
}

func (uf *UnionFind) findInternal(i int) int {
	if i >= len(uf.parent) {
		return -1 // Should not happen with correct resizing
	}
	if uf.parent[i] != i {
		uf.parent[i] = uf.findInternal(uf.parent[i])
	}
	return uf.parent[i]
}

// Union merges sets.
func (uf *UnionFind) Union(i, j int) {
	uf.mu.Lock()
	defer uf.mu.Unlock()

	rootI := uf.findInternal(i)
	rootJ := uf.findInternal(j)

	if rootI == -1 || rootJ == -1 || rootI == rootJ {
		return
	}

	// Union by rank
	if uf.rank[rootI] < uf.rank[rootJ] {
		uf.parent[rootI] = rootJ
	} else if uf.rank[rootI] > uf.rank[rootJ] {
		uf.parent[rootJ] = rootI
	} else {
		uf.parent[rootJ] = rootI
		uf.rank[rootI]++
	}
}

// Connected checks connectivity.
func (uf *UnionFind) Connected(i, j int) bool {
	return uf.Find(i) == uf.Find(j)
}

// Resize ensures capacity.
func (uf *UnionFind) Resize(newSize int) {
	uf.mu.Lock()
	defer uf.mu.Unlock()

	currentSize := len(uf.parent)
	if newSize <= currentSize {
		return
	}

	for i := currentSize; i < newSize; i++ {
		uf.parent = append(uf.parent, i)
		uf.rank = append(uf.rank, 0)
	}
}
