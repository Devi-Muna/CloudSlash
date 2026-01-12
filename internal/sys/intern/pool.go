package intern

import "sync"

type Pool struct {
	mu    sync.RWMutex
	store map[string]string
}

var globalPool = &Pool{store: make(map[string]string)}

// String returns the canonical version of s.
// If s is already in the pool, the pooled version is returned.
// Otherwise, s is added to the pool and returned.
func String(s string) string {
	globalPool.mu.RLock()
	if interned, ok := globalPool.store[s]; ok {
		globalPool.mu.RUnlock()
		return interned
	}
	globalPool.mu.RUnlock()

	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()
	// Double-check locking
	if interned, ok := globalPool.store[s]; ok {
		return interned
	}
	globalPool.store[s] = s
	return s
}

// Reset clears the global pool. Useful for testing or aggressive GC.
func Reset() {
	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()
	globalPool.store = make(map[string]string)
}
