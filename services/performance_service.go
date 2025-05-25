package services

import (
	"context"
	"log"
	"media-server/models"
	"runtime"
	"sync"
	"time"
)

// PerformanceService manages system performance monitoring and optimization
type PerformanceService struct {
	metrics     *models.PerformanceMetrics
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	workerPools map[string]*WorkerPool
	gcStats     *GCStats
	gcMutex     sync.RWMutex
	lastGCTime  time.Time
	gcTicker    *time.Ticker
}

// GCStats tracks garbage collection statistics
type GCStats struct {
	LastGCTime    time.Time
	GCFrequency   time.Duration
	GCPauseTotal  time.Duration
	GCPauseAvg    time.Duration
	HeapSize      uint64
	HeapInUse     uint64
	HeapReleased  uint64
	NextGC        uint64
}

// NewPerformanceService creates a new PerformanceService instance
func NewPerformanceService() *PerformanceService {
	ctx, cancel := context.WithCancel(context.Background())

	ps := &PerformanceService{
		metrics: &models.PerformanceMetrics{
			CPUCores:    runtime.NumCPU(),
			GOMAXPROCS:  runtime.GOMAXPROCS(0),
			StartTime:   time.Now(),
		},
		ctx:         ctx,
		cancel:      cancel,
		workerPools: make(map[string]*WorkerPool),
		gcStats:     &GCStats{},
		lastGCTime:  time.Now(),
	}

	// Start GC monitoring
	ps.startGCMonitoring()

	return ps
}

// StartMonitoring begins performance monitoring in a separate goroutine
func (ps *PerformanceService) StartMonitoring() {
	ticker := time.NewTicker(5 * time.Second) // 5-second refresh rate as preferred
	defer ticker.Stop()

	log.Println("Performance monitoring started")

	for {
		select {
		case <-ps.ctx.Done():
			log.Println("Performance monitoring stopped")
			return
		case <-ticker.C:
			ps.updateMetrics()
		}
	}
}

// Stop gracefully stops the performance service
func (ps *PerformanceService) Stop() {
	log.Println("Stopping performance service...")
	ps.cancel()

	// Stop GC monitoring
	if ps.gcTicker != nil {
		ps.gcTicker.Stop()
	}

	// Stop all worker pools
	ps.mutex.Lock()
	for name, pool := range ps.workerPools {
		log.Printf("Stopping worker pool: %s", name)
		pool.Stop()
	}
	ps.mutex.Unlock()

	// Force garbage collection on shutdown
	runtime.GC()
	log.Println("Performance service stopped and garbage collected")
}

// GetMetrics returns current performance metrics
func (ps *PerformanceService) GetMetrics() *models.PerformanceMetrics {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	// Create a copy to avoid race conditions
	metrics := *ps.metrics
	return &metrics
}

// updateMetrics collects current system performance data
func (ps *PerformanceService) updateMetrics() {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	ps.metrics.MemoryUsage = memStats.Alloc
	ps.metrics.MemoryTotal = memStats.Sys
	ps.metrics.GCCount = memStats.NumGC
	ps.metrics.Goroutines = runtime.NumGoroutine()
	ps.metrics.LastUpdated = time.Now()
	ps.metrics.Uptime = time.Since(ps.metrics.StartTime)

	// Calculate memory usage percentage
	if ps.metrics.MemoryTotal > 0 {
		ps.metrics.MemoryPercent = float64(ps.metrics.MemoryUsage) / float64(ps.metrics.MemoryTotal) * 100
	}

	// Update GC statistics
	ps.updateGCStats(&memStats)

	// Update worker pool metrics
	ps.metrics.WorkerPools = make(map[string]models.WorkerPoolMetrics)
	for name, pool := range ps.workerPools {
		ps.metrics.WorkerPools[name] = pool.GetMetrics()
	}

	// Trigger garbage collection if memory usage is high
	if ps.metrics.MemoryPercent > 85 {
		go func() {
			log.Println("High memory usage detected, triggering garbage collection")
			runtime.GC()
		}()
	}
}

// startGCMonitoring starts monitoring garbage collection
func (ps *PerformanceService) startGCMonitoring() {
	ps.gcTicker = time.NewTicker(30 * time.Second) // Check GC every 30 seconds

	go func() {
		for {
			select {
			case <-ps.ctx.Done():
				return
			case <-ps.gcTicker.C:
				ps.monitorGC()
			}
		}
	}()
}

// monitorGC monitors garbage collection and triggers it if needed
func (ps *PerformanceService) monitorGC() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	ps.gcMutex.Lock()
	defer ps.gcMutex.Unlock()

	// Check if we need to force GC
	heapInUse := memStats.HeapInuse
	heapIdle := memStats.HeapIdle

	// If heap idle is more than 50% of heap in use, consider GC
	if heapIdle > heapInUse/2 && time.Since(ps.lastGCTime) > 2*time.Minute {
		log.Printf("Triggering garbage collection: HeapInUse=%d, HeapIdle=%d", heapInUse, heapIdle)
		runtime.GC()
		ps.lastGCTime = time.Now()
	}
}

// updateGCStats updates garbage collection statistics
func (ps *PerformanceService) updateGCStats(memStats *runtime.MemStats) {
	ps.gcMutex.Lock()
	defer ps.gcMutex.Unlock()

	now := time.Now()

	// Update GC stats
	ps.gcStats.HeapSize = memStats.HeapSys
	ps.gcStats.HeapInUse = memStats.HeapInuse
	ps.gcStats.HeapReleased = memStats.HeapReleased
	ps.gcStats.NextGC = memStats.NextGC

	// Calculate GC frequency if we have previous data
	if !ps.gcStats.LastGCTime.IsZero() && memStats.NumGC > 0 {
		ps.gcStats.GCFrequency = now.Sub(ps.gcStats.LastGCTime)
	}

	// Update pause statistics
	if len(memStats.PauseNs) > 0 {
		totalPause := time.Duration(0)
		validPauses := 0

		for _, pause := range memStats.PauseNs {
			if pause > 0 {
				totalPause += time.Duration(pause)
				validPauses++
			}
		}

		if validPauses > 0 {
			ps.gcStats.GCPauseTotal = totalPause
			ps.gcStats.GCPauseAvg = totalPause / time.Duration(validPauses)
		}
	}

	ps.gcStats.LastGCTime = now
}

// CreateWorkerPool creates a new worker pool for parallel processing
func (ps *PerformanceService) CreateWorkerPool(name string, workerCount int, bufferSize int) *WorkerPool {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}

	pool := NewWorkerPool(name, workerCount, bufferSize)
	ps.workerPools[name] = pool

	log.Printf("Created worker pool '%s' with %d workers and buffer size %d", name, workerCount, bufferSize)
	return pool
}

// GetWorkerPool returns an existing worker pool by name
func (ps *PerformanceService) GetWorkerPool(name string) *WorkerPool {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	return ps.workerPools[name]
}

// GetOrCreateWorkerPool gets an existing worker pool or creates a new one
func (ps *PerformanceService) GetOrCreateWorkerPool(name string, workerCount int, bufferSize int) *WorkerPool {
	if pool := ps.GetWorkerPool(name); pool != nil {
		return pool
	}
	return ps.CreateWorkerPool(name, workerCount, bufferSize)
}

// OptimizeForLoad adjusts system settings based on current load
func (ps *PerformanceService) OptimizeForLoad() {
	metrics := ps.GetMetrics()

	// If memory usage is high, trigger garbage collection
	if metrics.MemoryPercent > 80 {
		log.Println("High memory usage detected, triggering garbage collection")
		runtime.GC()
	}

	// If goroutine count is very high, log a warning
	if metrics.Goroutines > 1000 {
		log.Printf("Warning: High goroutine count detected: %d", metrics.Goroutines)
	}
}

// GetCPUOptimalWorkerCount returns the optimal number of workers for CPU-intensive tasks
func (ps *PerformanceService) GetCPUOptimalWorkerCount() int {
	return runtime.NumCPU()
}

// GetIOOptimalWorkerCount returns the optimal number of workers for I/O-intensive tasks
func (ps *PerformanceService) GetIOOptimalWorkerCount() int {
	// For I/O intensive tasks, we can use more workers than CPU cores
	return runtime.NumCPU() * 2
}
