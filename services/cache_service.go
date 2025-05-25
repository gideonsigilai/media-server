package services

import (
	"context"
	"log"
	"media-server/models"
	"runtime"
	"sync"
	"time"
)

// CacheEntry represents a cached item with expiration
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
	AccessCount int64
	LastAccess  time.Time
}

// CacheService provides caching functionality for media metadata and file information
type CacheService struct {
	cache       map[string]*CacheEntry
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	defaultTTL  time.Duration
	maxSize     int
	hitCount    int64
	missCount   int64
	evictions   int64
	memoryUsage int64
	maxMemory   int64
	lastCleanup time.Time
}

// NewCacheService creates a new cache service
func NewCacheService() *CacheService {
	ctx, cancel := context.WithCancel(context.Background())

	return &CacheService{
		cache:       make(map[string]*CacheEntry),
		ctx:         ctx,
		cancel:      cancel,
		defaultTTL:  10 * time.Minute, // Default 10 minutes TTL
		maxSize:     1000,             // Maximum 1000 cached items
		maxMemory:   50 * 1024 * 1024, // 50MB memory limit
		lastCleanup: time.Now(),
	}
}

// StartCleanup starts the background cleanup routine
func (cs *CacheService) StartCleanup() {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	log.Println("Cache cleanup routine started")

	for {
		select {
		case <-cs.ctx.Done():
			log.Println("Cache cleanup routine stopped")
			return
		case <-ticker.C:
			cs.cleanup()
		}
	}
}

// Stop gracefully stops the cache service
func (cs *CacheService) Stop() {
	log.Println("Stopping cache service...")
	cs.cancel()
}

// Set stores a value in the cache with default TTL
func (cs *CacheService) Set(key string, value interface{}) {
	cs.SetWithTTL(key, value, cs.defaultTTL)
}

// SetWithTTL stores a value in the cache with custom TTL
func (cs *CacheService) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// If cache is at max size, evict least recently used item
	if len(cs.cache) >= cs.maxSize {
		cs.evictLRU()
	}

	cs.cache[key] = &CacheEntry{
		Value:       value,
		ExpiresAt:   time.Now().Add(ttl),
		AccessCount: 0,
		LastAccess:  time.Now(),
	}
}

// Get retrieves a value from the cache
func (cs *CacheService) Get(key string) (interface{}, bool) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	entry, exists := cs.cache[key]
	if !exists {
		cs.missCount++
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		delete(cs.cache, key)
		cs.missCount++
		return nil, false
	}

	// Update access statistics
	entry.AccessCount++
	entry.LastAccess = time.Now()
	cs.hitCount++

	return entry.Value, true
}

// GetFileInfo retrieves cached file information
func (cs *CacheService) GetFileInfo(path string) (*models.FileInfo, bool) {
	value, exists := cs.Get("fileinfo:" + path)
	if !exists {
		return nil, false
	}

	fileInfo, ok := value.(*models.FileInfo)
	return fileInfo, ok
}

// SetFileInfo caches file information
func (cs *CacheService) SetFileInfo(path string, fileInfo *models.FileInfo) {
	cs.Set("fileinfo:"+path, fileInfo)
}

// GetDirectoryListing retrieves cached directory listing
func (cs *CacheService) GetDirectoryListing(path string) ([]*models.FileInfo, bool) {
	value, exists := cs.Get("dirlist:" + path)
	if !exists {
		return nil, false
	}

	listing, ok := value.([]*models.FileInfo)
	return listing, ok
}

// SetDirectoryListing caches directory listing
func (cs *CacheService) SetDirectoryListing(path string, listing []*models.FileInfo) {
	// Cache directory listings for shorter time since they can change
	cs.SetWithTTL("dirlist:"+path, listing, 2*time.Minute)
}

// Delete removes a value from the cache
func (cs *CacheService) Delete(key string) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	delete(cs.cache, key)
}

// Clear removes all values from the cache
func (cs *CacheService) Clear() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.cache = make(map[string]*CacheEntry)
	log.Println("Cache cleared")
}

// InvalidateFileCache invalidates all file-related cache entries for a path
func (cs *CacheService) InvalidateFileCache(path string) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	keysToDelete := make([]string, 0)

	for key := range cs.cache {
		if key == "fileinfo:"+path || key == "dirlist:"+path {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(cs.cache, key)
	}

	log.Printf("Invalidated cache for path: %s", path)
}

// cleanup removes expired entries from the cache
func (cs *CacheService) cleanup() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)
	memoryFreed := int64(0)

	// Find expired entries
	for key, entry := range cs.cache {
		if now.After(entry.ExpiresAt) {
			expiredKeys = append(expiredKeys, key)
			memoryFreed += cs.estimateEntrySize(entry)
		}
	}

	// Remove expired entries
	for _, key := range expiredKeys {
		delete(cs.cache, key)
		cs.evictions++
	}

	// Update memory usage
	cs.memoryUsage -= memoryFreed

	// Check if we need aggressive cleanup due to memory pressure
	if cs.memoryUsage > cs.maxMemory {
		cs.aggressiveCleanup()
	}

	// Update last cleanup time
	cs.lastCleanup = now

	if len(expiredKeys) > 0 {
		log.Printf("Cache cleanup: removed %d expired entries, freed %d bytes", len(expiredKeys), memoryFreed)
	}

	// Force GC if significant memory was freed
	if memoryFreed > 10*1024*1024 { // 10MB
		go func() {
			runtime.GC()
		}()
	}
}

// aggressiveCleanup removes least recently used items when memory pressure is high
func (cs *CacheService) aggressiveCleanup() {
	// Sort entries by last access time
	type entryInfo struct {
		key        string
		lastAccess time.Time
		size       int64
	}

	entries := make([]entryInfo, 0, len(cs.cache))
	for key, entry := range cs.cache {
		entries = append(entries, entryInfo{
			key:        key,
			lastAccess: entry.LastAccess,
			size:       cs.estimateEntrySize(entry),
		})
	}

	// Sort by last access time (oldest first)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].lastAccess.After(entries[j].lastAccess) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Remove oldest entries until memory usage is acceptable
	removed := 0
	memoryFreed := int64(0)
	targetMemory := cs.maxMemory * 80 / 100 // Target 80% of max memory

	for _, entry := range entries {
		if cs.memoryUsage <= targetMemory {
			break
		}

		delete(cs.cache, entry.key)
		cs.memoryUsage -= entry.size
		memoryFreed += entry.size
		cs.evictions++
		removed++
	}

	if removed > 0 {
		log.Printf("Aggressive cache cleanup: removed %d entries, freed %d bytes", removed, memoryFreed)
	}
}

// estimateEntrySize estimates the memory size of a cache entry
func (cs *CacheService) estimateEntrySize(entry *CacheEntry) int64 {
	// Basic estimation - in a real implementation you might want more accurate sizing
	baseSize := int64(64) // Base overhead for the entry struct

	switch v := entry.Value.(type) {
	case string:
		return baseSize + int64(len(v))
	case []byte:
		return baseSize + int64(len(v))
	case *models.FileInfo:
		// Estimate FileInfo size
		return baseSize + int64(len(v.Name)) + int64(len(v.Path)) + int64(len(v.Extension)) + 64
	default:
		// Default estimation for other types
		return baseSize + 256
	}
}

// evictLRU removes the least recently used item from the cache
func (cs *CacheService) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range cs.cache {
		if oldestKey == "" || entry.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccess
		}
	}

	if oldestKey != "" {
		delete(cs.cache, oldestKey)
		cs.evictions++
	}
}

// GetStats returns cache statistics
func (cs *CacheService) GetStats() models.CacheStats {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	totalRequests := cs.hitCount + cs.missCount
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(cs.hitCount) / float64(totalRequests) * 100
	}

	return models.CacheStats{
		Size:         len(cs.cache),
		MaxSize:      cs.maxSize,
		HitCount:     cs.hitCount,
		MissCount:    cs.missCount,
		HitRate:      hitRate,
		Evictions:    cs.evictions,
		DefaultTTL:   cs.defaultTTL,
	}
}

// GetSize returns the current number of items in the cache
func (cs *CacheService) GetSize() int {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	return len(cs.cache)
}

// SetMaxSize sets the maximum cache size
func (cs *CacheService) SetMaxSize(maxSize int) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.maxSize = maxSize

	// If current size exceeds new max size, evict items
	for len(cs.cache) > cs.maxSize {
		cs.evictLRU()
	}
}

// SetDefaultTTL sets the default time-to-live for cache entries
func (cs *CacheService) SetDefaultTTL(ttl time.Duration) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.defaultTTL = ttl
}
