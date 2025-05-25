package services

import (
	"context"
	"fmt"
	"log"
	"media-server/models"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Task represents a unit of work to be processed by the worker pool
type Task struct {
	ID       string
	Function func() error
	Context  context.Context
	Result   chan error
}

// WorkerPool manages a pool of workers for parallel processing
type WorkerPool struct {
	name         string
	workers      int
	taskQueue    chan Task
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	metrics      models.WorkerPoolMetrics
	metricsLock  sync.RWMutex
	tasksTotal   int64
	tasksSuccess int64
	tasksFailed  int64
	idleWorkers  int64
	busyWorkers  int64
	lastCleanup  time.Time
	cleanupTicker *time.Ticker
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(name string, workers int, bufferSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		name:      name,
		workers:   workers,
		taskQueue: make(chan Task, bufferSize),
		ctx:       ctx,
		cancel:    cancel,
		metrics: models.WorkerPoolMetrics{
			Name:        name,
			Workers:     workers,
			BufferSize:  bufferSize,
			StartTime:   time.Now(),
			Status:      "starting",
		},
		idleWorkers: int64(workers),
		lastCleanup: time.Now(),
	}

	// Start workers
	pool.start()

	// Start cleanup routine
	pool.startCleanup()

	return pool
}

// start initializes and starts all workers
func (wp *WorkerPool) start() {
	wp.metricsLock.Lock()
	wp.metrics.Status = "running"
	wp.metricsLock.Unlock()

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}

	log.Printf("Worker pool '%s' started with %d workers", wp.name, wp.workers)
}

// worker is the main worker goroutine
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	log.Printf("Worker %d started in pool '%s'", id, wp.name)

	for {
		select {
		case <-wp.ctx.Done():
			log.Printf("Worker %d stopping in pool '%s'", id, wp.name)
			return
		case task := <-wp.taskQueue:
			// Mark worker as busy
			atomic.AddInt64(&wp.idleWorkers, -1)
			atomic.AddInt64(&wp.busyWorkers, 1)

			wp.processTask(task)

			// Mark worker as idle
			atomic.AddInt64(&wp.busyWorkers, -1)
			atomic.AddInt64(&wp.idleWorkers, 1)
		}
	}
}

// processTask executes a single task
func (wp *WorkerPool) processTask(task Task) {
	startTime := time.Now()

	// Update active tasks count
	wp.metricsLock.Lock()
	wp.metrics.ActiveTasks++
	wp.metricsLock.Unlock()

	// Execute the task
	err := task.Function()

	// Update metrics
	duration := time.Since(startTime)
	atomic.AddInt64(&wp.tasksTotal, 1)

	if err != nil {
		atomic.AddInt64(&wp.tasksFailed, 1)
		log.Printf("Task %s failed in pool '%s': %v", task.ID, wp.name, err)
	} else {
		atomic.AddInt64(&wp.tasksSuccess, 1)
	}

	// Update metrics
	wp.metricsLock.Lock()
	wp.metrics.ActiveTasks--
	wp.metrics.TotalTasks = atomic.LoadInt64(&wp.tasksTotal)
	wp.metrics.SuccessfulTasks = atomic.LoadInt64(&wp.tasksSuccess)
	wp.metrics.FailedTasks = atomic.LoadInt64(&wp.tasksFailed)
	wp.metrics.AverageTaskDuration = wp.updateAverageTaskDuration(duration)
	wp.metrics.LastTaskTime = time.Now()
	wp.metricsLock.Unlock()

	// Send result back if channel is provided
	if task.Result != nil {
		select {
		case task.Result <- err:
		default:
			// Channel is full or closed, ignore
		}
	}
}

// updateAverageTaskDuration calculates the running average of task durations
func (wp *WorkerPool) updateAverageTaskDuration(newDuration time.Duration) time.Duration {
	if wp.metrics.TotalTasks == 1 {
		return newDuration
	}

	// Simple moving average calculation
	currentAvg := wp.metrics.AverageTaskDuration
	totalTasks := float64(wp.metrics.TotalTasks)
	newAvg := time.Duration(float64(currentAvg)*(totalTasks-1)/totalTasks + float64(newDuration)/totalTasks)

	return newAvg
}

// Submit submits a task to the worker pool
func (wp *WorkerPool) Submit(taskID string, fn func() error) error {
	return wp.SubmitWithContext(context.Background(), taskID, fn)
}

// SubmitWithContext submits a task with context to the worker pool
func (wp *WorkerPool) SubmitWithContext(ctx context.Context, taskID string, fn func() error) error {
	task := Task{
		ID:       taskID,
		Function: fn,
		Context:  ctx,
	}

	select {
	case wp.taskQueue <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-wp.ctx.Done():
		return wp.ctx.Err()
	default:
		return ErrWorkerPoolFull
	}
}

// SubmitAndWait submits a task and waits for its completion
func (wp *WorkerPool) SubmitAndWait(taskID string, fn func() error) error {
	return wp.SubmitAndWaitWithContext(context.Background(), taskID, fn)
}

// SubmitAndWaitWithContext submits a task with context and waits for its completion
func (wp *WorkerPool) SubmitAndWaitWithContext(ctx context.Context, taskID string, fn func() error) error {
	resultChan := make(chan error, 1)

	task := Task{
		ID:       taskID,
		Function: fn,
		Context:  ctx,
		Result:   resultChan,
	}

	select {
	case wp.taskQueue <- task:
		// Task submitted, wait for result
		select {
		case err := <-resultChan:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-wp.ctx.Done():
			return wp.ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-wp.ctx.Done():
		return wp.ctx.Err()
	default:
		return ErrWorkerPoolFull
	}
}

// startCleanup starts the cleanup routine for the worker pool
func (wp *WorkerPool) startCleanup() {
	wp.cleanupTicker = time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes

	go func() {
		for {
			select {
			case <-wp.ctx.Done():
				return
			case <-wp.cleanupTicker.C:
				wp.performCleanup()
			}
		}
	}()
}

// performCleanup performs maintenance tasks for the worker pool
func (wp *WorkerPool) performCleanup() {
	wp.metricsLock.Lock()
	defer wp.metricsLock.Unlock()

	now := time.Now()

	// Reset metrics if pool has been idle for too long
	if wp.metrics.ActiveTasks == 0 && now.Sub(wp.lastCleanup) > 10*time.Minute {
		log.Printf("Performing cleanup for idle worker pool '%s'", wp.name)

		// Reset counters but keep totals
		wp.metrics.ActiveTasks = 0
		wp.lastCleanup = now

		// Force garbage collection if this pool has been idle
		go func() {
			runtime.GC()
		}()
	}
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.metricsLock.Lock()
	wp.metrics.Status = "stopping"
	wp.metricsLock.Unlock()

	log.Printf("Stopping worker pool '%s'", wp.name)

	// Stop cleanup ticker
	if wp.cleanupTicker != nil {
		wp.cleanupTicker.Stop()
	}

	// Cancel context to signal workers to stop
	wp.cancel()

	// Close task queue to prevent new tasks
	close(wp.taskQueue)

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers finished gracefully
	case <-time.After(30 * time.Second):
		log.Printf("Warning: Worker pool '%s' shutdown timeout", wp.name)
	}

	wp.metricsLock.Lock()
	wp.metrics.Status = "stopped"
	wp.metrics.StopTime = time.Now()
	wp.metricsLock.Unlock()

	log.Printf("Worker pool '%s' stopped", wp.name)
}

// GetMetrics returns current worker pool metrics
func (wp *WorkerPool) GetMetrics() models.WorkerPoolMetrics {
	wp.metricsLock.RLock()
	defer wp.metricsLock.RUnlock()

	// Create a copy to avoid race conditions
	metrics := wp.metrics
	metrics.QueueSize = len(wp.taskQueue)
	metrics.Uptime = time.Since(wp.metrics.StartTime)

	return metrics
}

// IsRunning returns true if the worker pool is currently running
func (wp *WorkerPool) IsRunning() bool {
	wp.metricsLock.RLock()
	defer wp.metricsLock.RUnlock()

	return wp.metrics.Status == "running"
}

// GetQueueSize returns the current number of tasks in the queue
func (wp *WorkerPool) GetQueueSize() int {
	return len(wp.taskQueue)
}

// Custom errors
var (
	ErrWorkerPoolFull = fmt.Errorf("worker pool queue is full")
)
