package models

import (
	"time"
)

// PerformanceMetrics represents system performance metrics
type PerformanceMetrics struct {
	CPUCores        int                           `json:"cpu_cores"`
	GOMAXPROCS      int                           `json:"gomaxprocs"`
	Goroutines      int                           `json:"goroutines"`
	MemoryUsage     uint64                        `json:"memory_usage"`     // bytes
	MemoryTotal     uint64                        `json:"memory_total"`     // bytes
	MemoryPercent   float64                       `json:"memory_percent"`   // percentage
	GCCount         uint32                        `json:"gc_count"`
	StartTime       time.Time                     `json:"start_time"`
	LastUpdated     time.Time                     `json:"last_updated"`
	Uptime          time.Duration                 `json:"uptime"`
	WorkerPools     map[string]WorkerPoolMetrics  `json:"worker_pools"`
}

// WorkerPoolMetrics represents metrics for a worker pool
type WorkerPoolMetrics struct {
	Name                string        `json:"name"`
	Workers             int           `json:"workers"`
	BufferSize          int           `json:"buffer_size"`
	QueueSize           int           `json:"queue_size"`
	ActiveTasks         int           `json:"active_tasks"`
	TotalTasks          int64         `json:"total_tasks"`
	SuccessfulTasks     int64         `json:"successful_tasks"`
	FailedTasks         int64         `json:"failed_tasks"`
	AverageTaskDuration time.Duration `json:"average_task_duration"`
	StartTime           time.Time     `json:"start_time"`
	StopTime            time.Time     `json:"stop_time"`
	LastTaskTime        time.Time     `json:"last_task_time"`
	Uptime              time.Duration `json:"uptime"`
	Status              string        `json:"status"` // starting, running, stopping, stopped
}

// CacheStats represents cache performance statistics
type CacheStats struct {
	Size       int           `json:"size"`
	MaxSize    int           `json:"max_size"`
	HitCount   int64         `json:"hit_count"`
	MissCount  int64         `json:"miss_count"`
	HitRate    float64       `json:"hit_rate"`    // percentage
	Evictions  int64         `json:"evictions"`
	DefaultTTL time.Duration `json:"default_ttl"`
}

// StreamingMetrics represents metrics for file streaming operations
type StreamingMetrics struct {
	ActiveStreams     int     `json:"active_streams"`
	TotalStreams      int64   `json:"total_streams"`
	BytesStreamed     int64   `json:"bytes_streamed"`
	AverageSpeed      float64 `json:"average_speed"`      // bytes per second
	PeakSpeed         float64 `json:"peak_speed"`         // bytes per second
	ConcurrentPeak    int     `json:"concurrent_peak"`    // max concurrent streams
	BufferPoolSize    int     `json:"buffer_pool_size"`
	BufferPoolActive  int     `json:"buffer_pool_active"`
}

// DatabaseMetrics represents metrics for database operations (if applicable)
type DatabaseMetrics struct {
	ActiveConnections int           `json:"active_connections"`
	MaxConnections    int           `json:"max_connections"`
	TotalQueries      int64         `json:"total_queries"`
	SuccessfulQueries int64         `json:"successful_queries"`
	FailedQueries     int64         `json:"failed_queries"`
	AverageQueryTime  time.Duration `json:"average_query_time"`
	SlowQueries       int64         `json:"slow_queries"`
}

// SystemLoad represents current system load information
type SystemLoad struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskPercent   float64 `json:"disk_percent"`
	NetworkIn     int64   `json:"network_in"`     // bytes per second
	NetworkOut    int64   `json:"network_out"`    // bytes per second
	LoadAverage   float64 `json:"load_average"`   // 1-minute load average
}

// GetSuccessRate returns the success rate for worker pool tasks
func (wpm *WorkerPoolMetrics) GetSuccessRate() float64 {
	if wpm.TotalTasks == 0 {
		return 0
	}
	return float64(wpm.SuccessfulTasks) / float64(wpm.TotalTasks) * 100
}

// GetFailureRate returns the failure rate for worker pool tasks
func (wpm *WorkerPoolMetrics) GetFailureRate() float64 {
	if wpm.TotalTasks == 0 {
		return 0
	}
	return float64(wpm.FailedTasks) / float64(wpm.TotalTasks) * 100
}

// GetUtilization returns the current utilization percentage of the worker pool
func (wpm *WorkerPoolMetrics) GetUtilization() float64 {
	if wpm.Workers == 0 {
		return 0
	}
	return float64(wpm.ActiveTasks) / float64(wpm.Workers) * 100
}

// GetQueueUtilization returns the current queue utilization percentage
func (wpm *WorkerPoolMetrics) GetQueueUtilization() float64 {
	if wpm.BufferSize == 0 {
		return 0
	}
	return float64(wpm.QueueSize) / float64(wpm.BufferSize) * 100
}

// IsHealthy returns true if the worker pool is in a healthy state
func (wpm *WorkerPoolMetrics) IsHealthy() bool {
	// Consider healthy if:
	// - Status is running
	// - Queue is not completely full
	// - Failure rate is below 10%
	return wpm.Status == "running" && 
		   wpm.GetQueueUtilization() < 95 && 
		   wpm.GetFailureRate() < 10
}

// GetMemoryUsageMB returns memory usage in megabytes
func (pm *PerformanceMetrics) GetMemoryUsageMB() float64 {
	return float64(pm.MemoryUsage) / 1024 / 1024
}

// GetMemoryTotalMB returns total memory in megabytes
func (pm *PerformanceMetrics) GetMemoryTotalMB() float64 {
	return float64(pm.MemoryTotal) / 1024 / 1024
}

// IsMemoryPressure returns true if memory usage is high
func (pm *PerformanceMetrics) IsMemoryPressure() bool {
	return pm.MemoryPercent > 80
}

// IsHighGoroutineCount returns true if goroutine count is unusually high
func (pm *PerformanceMetrics) IsHighGoroutineCount() bool {
	return pm.Goroutines > 1000
}

// GetUptimeString returns uptime as a human-readable string
func (pm *PerformanceMetrics) GetUptimeString() string {
	return pm.Uptime.String()
}

// GetEfficiency returns cache efficiency as a percentage
func (cs *CacheStats) GetEfficiency() float64 {
	return cs.HitRate
}

// GetUsagePercent returns cache usage as a percentage of max size
func (cs *CacheStats) GetUsagePercent() float64 {
	if cs.MaxSize == 0 {
		return 0
	}
	return float64(cs.Size) / float64(cs.MaxSize) * 100
}

// IsNearCapacity returns true if cache is near its capacity
func (cs *CacheStats) IsNearCapacity() bool {
	return cs.GetUsagePercent() > 90
}

// GetThroughput returns streaming throughput in MB/s
func (sm *StreamingMetrics) GetThroughput() float64 {
	return sm.AverageSpeed / 1024 / 1024
}

// GetPeakThroughput returns peak streaming throughput in MB/s
func (sm *StreamingMetrics) GetPeakThroughput() float64 {
	return sm.PeakSpeed / 1024 / 1024
}

// GetTotalStreamedGB returns total bytes streamed in gigabytes
func (sm *StreamingMetrics) GetTotalStreamedGB() float64 {
	return float64(sm.BytesStreamed) / 1024 / 1024 / 1024
}

// GetQuerySuccessRate returns database query success rate
func (dm *DatabaseMetrics) GetQuerySuccessRate() float64 {
	if dm.TotalQueries == 0 {
		return 0
	}
	return float64(dm.SuccessfulQueries) / float64(dm.TotalQueries) * 100
}

// GetConnectionUtilization returns database connection utilization percentage
func (dm *DatabaseMetrics) GetConnectionUtilization() float64 {
	if dm.MaxConnections == 0 {
		return 0
	}
	return float64(dm.ActiveConnections) / float64(dm.MaxConnections) * 100
}

// IsOverloaded returns true if system is under high load
func (sl *SystemLoad) IsOverloaded() bool {
	return sl.CPUPercent > 80 || sl.MemoryPercent > 80 || sl.LoadAverage > 2.0
}

// GetNetworkThroughputMBps returns network throughput in MB/s
func (sl *SystemLoad) GetNetworkThroughputMBps() (float64, float64) {
	inMBps := float64(sl.NetworkIn) / 1024 / 1024
	outMBps := float64(sl.NetworkOut) / 1024 / 1024
	return inMBps, outMBps
}
