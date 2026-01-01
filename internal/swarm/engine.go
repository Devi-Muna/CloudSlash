package swarm

import (
	"context"
	"sync"
	"time"
)

type Task func(ctx context.Context) error

type Engine struct {
	aimd   *AIMD
	tasks  chan Task
	wg     sync.WaitGroup
	quit   chan struct{}
	active int
	mu     sync.Mutex
	stats  Stats
}

type Stats struct {
	ActiveWorkers  int
	Concurrency    int
	TasksCompleted int64
}

func NewEngine() *Engine {
	return &Engine{
		aimd:  NewAIMD(50, 5, 500),
		tasks: make(chan Task, 1000),
		quit:  make(chan struct{}),
	}
}

func (e *Engine) Start(ctx context.Context) {
	go e.loop(ctx)
}

func (e *Engine) Submit(t Task) {
	e.tasks <- t
}

func (e *Engine) Stop() {
	close(e.quit)
	e.wg.Wait()
}

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
			target := e.aimd.GetConcurrency()
			current := e.activeCount()

			if current < target {
				spawn := target - current
				for i := 0; i < spawn; i++ {
					e.wg.Add(1)
					go e.worker(ctx)
				}
			}
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
			lat := time.Since(start)

			// Simplified throttle check
			isThrottled := false
			if err != nil {
				// TODO: check for aws.isThrottle(err)
			}

			e.aimd.Feedback(lat, isThrottled)

			e.mu.Lock()
			e.stats.TasksCompleted++
			e.mu.Unlock()
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
