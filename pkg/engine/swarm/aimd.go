package swarm

import (
	"sync"
	"time"
)

type AIMD struct {
	mu          sync.Mutex
	concurrency int
	minWorkers  int
	maxWorkers  int
	lastChange  time.Time
}

func NewAIMD(start, min, max int) *AIMD {
	return &AIMD{
		concurrency: start,
		minWorkers:  min,
		maxWorkers:  max,
		lastChange:  time.Now(),
	}
}

func (a *AIMD) GetConcurrency() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.concurrency
}

func (a *AIMD) Feedback(lat time.Duration, throttled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	// dampen oscillation
	if now.Sub(a.lastChange) < 100*time.Millisecond {
		return
	}

	if throttled {
		a.concurrency = a.concurrency / 2
		if a.concurrency < a.minWorkers {
			a.concurrency = a.minWorkers
		}
		a.lastChange = now
		return
	}

	// scale up if latency is healthy
	if lat < 100*time.Millisecond {
		a.concurrency += 5
		if a.concurrency > a.maxWorkers {
			a.concurrency = a.maxWorkers
		}
		a.lastChange = now
	}
}
