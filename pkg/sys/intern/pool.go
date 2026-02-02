package intern

import "sync"

type Pool struct {
	mu      sync.RWMutex
	store   map[string]uint32
	reverse []string
}

var globalPool = &Pool{
	store:   make(map[string]uint32),
	reverse: make([]string, 0, 1000),
}

const InvalidID uint32 = 0

// Get returns the unique ID for s, allocating a new one if necessary.
// IDs are 1-based to allow 0 to be a sentinel.
func Get(s string) uint32 {
	if s == "" {
		return InvalidID
	}

	globalPool.mu.RLock()
	id, ok := globalPool.store[s]
	globalPool.mu.RUnlock()
	if ok {
		return id
	}

	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()

	// Double-check
	if id, ok := globalPool.store[s]; ok {
		return id
	}

	// 1-based index (0 is reserve for empty/invalid)
	// reverse is 0-indexed, so we append, then index is len-1.
	// But we went 1-based ID?
	// Let's make ID = index + 1.
	// reverse[0] -> ID 1.
	// reverse[id-1] -> string.

	globalPool.reverse = append(globalPool.reverse, s)
	id = uint32(len(globalPool.reverse))
	globalPool.store[s] = id
	return id
}

// GetStr returns the string for the given ID.
func GetStr(id uint32) string {
	if id == InvalidID {
		return ""
	}
	globalPool.mu.RLock()
	defer globalPool.mu.RUnlock()

	idx := int(id) - 1
	if idx < 0 || idx >= len(globalPool.reverse) {
		return ""
	}
	return globalPool.reverse[idx]
}

// Reset clears the global pool.
func Reset() {
	globalPool.mu.Lock()
	defer globalPool.mu.Unlock()
	globalPool.store = make(map[string]uint32)
	globalPool.reverse = make([]string, 0, 1000)
}
