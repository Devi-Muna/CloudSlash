package swarm

import (
	"context"
	"sync"
	"time"
)

// Task represents a unit of work for the swarm.
type Task func(ctx context.Context) error

// Engine manages the worker pool and concurrency.
type Engine struct {
	aimd   *AIMD
	tasks  chan Task
	wg     sync.WaitGroup
	quit   chan struct{}
	active int
	mu     sync.Mutex
	stats  Stats
}

// Stats holds runtime statistics for the engine.
type Stats struct {
	ActiveWorkers  int
	Concurrency    int
	TasksCompleted int64
}

// NewEngine creates a new Swarm Engine.
func NewEngine() *Engine {
	return &Engine{
		aimd:  NewAIMD(50, 5, 500),   // Start 50, Min 5, Max 500
		tasks: make(chan Task, 1000), // Buffer for tasks
		quit:  make(chan struct{}),
	}
}

// Start begins the worker loop.
func (e *Engine) Start(ctx context.Context) {
	go e.loop(ctx)
}

// Submit adds a task to the queue.
func (e *Engine) Submit(t Task) {
	e.tasks <- t
}

// Wait blocks until all tasks are done (this is a simplified wait,
// in reality we need better tracking of submitted vs completed).
// For now, we'll rely on the caller to manage wait groups for specific sets of tasks
// or use a more complex job tracking system if needed.
// But for the "Swarm", we usually just fire and forget or wait for the channel to drain.
// Let's implement a GracefulStop.
func (e *Engine) Stop() {
	close(e.quit)
	e.wg.Wait()
}

// GetStats returns current engine stats.
func (e *Engine) GetStats() Stats {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Stats{
		ActiveWorkers:  e.active,
		Concurrency:    e.aimd.GetConcurrency(),
		TasksCompleted: e.stats.TasksCompleted,
	}
}

func (e *Engine) loop(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.quit:
			return
		case <-ticker.C:
			// Adjust worker pool based on AIMD concurrency
			target := e.aimd.GetConcurrency()
			current := e.activeCount()

			if current < target {
				// Spawn more workers
				spawn := target - current
				for i := 0; i < spawn; i++ {
					e.wg.Add(1)
					go e.worker(ctx)
				}
			}
			// If current > target, workers will naturally exit when they finish a task
			// and check the limit, or we can implement a "drain" logic.
			// For simplicity, we'll let them finish.
		}
	}
}

func (e *Engine) activeCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active
}

func (e *Engine) worker(ctx context.Context) {
	e.mu.Lock()
	e.active++
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.active--
		e.mu.Unlock()
		e.wg.Done()
	}()

	for {
		// Check if we should scale down
		if e.activeCount() > e.aimd.GetConcurrency() {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-e.quit:
			return
		case task := <-e.tasks:
			start := time.Now()
			err := task(ctx)
			latency := time.Since(start)

			// Check for throttling error (simplified check for now)
			isThrottled := false
			if err != nil {
				// In a real AWS context, we'd check for "ThrottlingException" or 429
				// For now, assume a specific error type or string check would happen here
				// isThrottled = isAwsThrottleError(err)
			}

			e.aimd.Feedback(latency, isThrottled)

			e.mu.Lock()
			e.stats.TasksCompleted++
			e.mu.Unlock()
		default:
			// No tasks, sleep briefly or exit if we have too many idle workers?
			// For now, simple sleep to avoid busy loop
			time.Sleep(10 * time.Millisecond)
		}
	}
}
