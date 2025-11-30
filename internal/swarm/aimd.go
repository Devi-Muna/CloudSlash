package swarm

import (
	"sync"
	"time"
)

// AIMD implements Additive Increase, Multiplicative Decrease
// to manage concurrency levels based on system feedback.
type AIMD struct {
	mu          sync.Mutex
	concurrency int
	minWorkers  int
	maxWorkers  int
	lastChange  time.Time
}

// NewAIMD creates a new AIMD controller.
func NewAIMD(start, min, max int) *AIMD {
	return &AIMD{
		concurrency: start,
		minWorkers:  min,
		maxWorkers:  max,
		lastChange:  time.Now(),
	}
}

// GetConcurrency returns the current allowed concurrency level.
func (a *AIMD) GetConcurrency() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.concurrency
}

// Feedback provides feedback to the AIMD controller.
// latency: the duration of the last operation.
// err: any error encountered (specifically looking for throttling).
func (a *AIMD) Feedback(latency time.Duration, isThrottled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	// Prevent rapid oscillation, only adjust every 100ms at most
	if now.Sub(a.lastChange) < 100*time.Millisecond {
		return
	}

	if isThrottled {
		// Multiplicative Decrease: Cut by 50%
		a.concurrency = a.concurrency / 2
		if a.concurrency < a.minWorkers {
			a.concurrency = a.minWorkers
		}
		a.lastChange = now
		return
	}

	// Additive Increase: +5 workers if latency is low (< 100ms)
	if latency < 100*time.Millisecond {
		a.concurrency += 5
		if a.concurrency > a.maxWorkers {
			a.concurrency = a.maxWorkers
		}
		a.lastChange = now
	}
}
