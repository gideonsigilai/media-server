package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"media-server/models"
	"sync"
	"time"
)

// AdminService manages admin users, connections, and access control
type AdminService struct {
	adminUsers         map[string]*models.AdminUser
	connections        map[string]*models.Connection
	activityLogs       []models.ActivityLog
	mediaAccess        map[string]*models.MediaAccess
	blockedIPs         map[string]bool
	totalConnectionsEver int
	startTime          time.Time
	mutex              sync.RWMutex
	logMutex           sync.RWMutex
	performanceService *PerformanceService
	cacheService       *CacheService
	streamingMetrics   models.StreamingMetrics
	streamingMutex     sync.RWMutex
}

// NewAdminService creates a new AdminService instance
func NewAdminService() *AdminService {
	return &AdminService{
		adminUsers:           make(map[string]*models.AdminUser),
		connections:          make(map[string]*models.Connection),
		activityLogs:         make([]models.ActivityLog, 0),
		mediaAccess:          make(map[string]*models.MediaAccess),
		blockedIPs:           make(map[string]bool),
		totalConnectionsEver: 0,
		startTime:            time.Now(),
		streamingMetrics:     models.StreamingMetrics{},
	}
}

// SetPerformanceService sets the performance service for monitoring
func (as *AdminService) SetPerformanceService(ps *PerformanceService) {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	as.performanceService = ps
}

// SetCacheService sets the cache service for monitoring
func (as *AdminService) SetCacheService(cs *CacheService) {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	as.cacheService = cs
}

// AddAdminUser adds a new admin user with IP-based authentication
func (as *AdminService) AddAdminUser(name, ipAddress string) (*models.AdminUser, error) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	// Generate unique ID
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user ID: %w", err)
	}

	admin := &models.AdminUser{
		ID:          id,
		IPAddress:   ipAddress,
		Name:        name,
		CreatedAt:   time.Now(),
		LastSeen:    time.Now(),
		IsActive:    true,
		Permissions: []string{"dashboard", "users", "connections", "media"},
	}

	as.adminUsers[ipAddress] = admin
	as.LogActivity(ipAddress, "admin_user_created", fmt.Sprintf("Admin user %s created", name), "", true, "")

	return admin, nil
}

// IsAdminUser checks if an IP address belongs to an admin user
func (as *AdminService) IsAdminUser(ipAddress string) bool {
	as.mutex.RLock()
	defer as.mutex.RUnlock()

	admin, exists := as.adminUsers[ipAddress]
	return exists && admin.IsActive
}

// GetAdminUser retrieves an admin user by IP address
func (as *AdminService) GetAdminUser(ipAddress string) (*models.AdminUser, bool) {
	as.mutex.RLock()
	defer as.mutex.RUnlock()

	admin, exists := as.adminUsers[ipAddress]
	if exists && admin.IsActive {
		// Update last seen
		admin.LastSeen = time.Now()
		return admin, true
	}
	return nil, false
}

// GetAllAdminUsers returns all admin users
func (as *AdminService) GetAllAdminUsers() []*models.AdminUser {
	as.mutex.RLock()
	defer as.mutex.RUnlock()

	users := make([]*models.AdminUser, 0, len(as.adminUsers))
	for _, user := range as.adminUsers {
		users = append(users, user)
	}
	return users
}

// RemoveAdminUser removes an admin user
func (as *AdminService) RemoveAdminUser(ipAddress string) error {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	admin, exists := as.adminUsers[ipAddress]
	if !exists {
		return fmt.Errorf("admin user not found")
	}

	admin.IsActive = false
	as.LogActivity(ipAddress, "admin_user_removed", fmt.Sprintf("Admin user %s removed", admin.Name), "", true, "")

	return nil
}

// TrackConnection tracks a connection, reusing existing connection for same IP
func (as *AdminService) TrackConnection(ipAddress, userAgent string) *models.Connection {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	// Check if we already have an active connection for this IP
	for _, conn := range as.connections {
		if conn.IPAddress == ipAddress {
			// Update existing connection
			conn.LastActivity = time.Now()
			conn.UserAgent = userAgent // Update user agent in case it changed
			return conn
		}
	}

	// Create new connection only if none exists for this IP
	id, _ := generateID()

	connection := &models.Connection{
		ID:           id,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
		BytesServed:  0,
		RequestCount: 0,
		CurrentSpeed: 0,
		IsBlocked:    as.blockedIPs[ipAddress],
		Location:     as.getLocationFromIP(ipAddress),
	}

	as.connections[id] = connection
	as.totalConnectionsEver++
	as.LogActivity(ipAddress, "connection_established", "New connection established", userAgent, true, "")

	return connection
}

// UpdateConnection updates connection activity
func (as *AdminService) UpdateConnection(connectionID string, bytesTransferred int64) {
	as.mutex.RLock()
	connection, exists := as.connections[connectionID]
	as.mutex.RUnlock()

	if exists {
		connection.UpdateActivity(bytesTransferred)
	}
}

// GetActiveConnections returns all active connections and cleans up inactive ones
func (as *AdminService) GetActiveConnections() []*models.Connection {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	connections := make([]*models.Connection, 0)
	cutoff := time.Now().Add(-5 * time.Minute) // Consider connections inactive after 5 minutes
	activeConnections := make(map[string]*models.Connection)

	// Clean up inactive connections and collect active ones
	for id, conn := range as.connections {
		if conn.LastActivity.After(cutoff) {
			connections = append(connections, conn)
			activeConnections[id] = conn
		}
	}

	// Update the connections map to only contain active connections
	as.connections = activeConnections

	return connections
}

// BlockIP blocks an IP address
func (as *AdminService) BlockIP(ipAddress, reason string) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	as.blockedIPs[ipAddress] = true

	// Update existing connections
	for _, conn := range as.connections {
		if conn.IPAddress == ipAddress {
			conn.IsBlocked = true
			conn.BlockedReason = reason
		}
	}

	as.LogActivity(ipAddress, "ip_blocked", fmt.Sprintf("IP blocked: %s", reason), "", true, "")
}

// UnblockIP unblocks an IP address
func (as *AdminService) UnblockIP(ipAddress string) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	delete(as.blockedIPs, ipAddress)

	// Update existing connections
	for _, conn := range as.connections {
		if conn.IPAddress == ipAddress {
			conn.IsBlocked = false
			conn.BlockedReason = ""
		}
	}

	as.LogActivity(ipAddress, "ip_unblocked", "IP unblocked", "", true, "")
}

// IsBlocked checks if an IP address is blocked
func (as *AdminService) IsBlocked(ipAddress string) bool {
	as.mutex.RLock()
	defer as.mutex.RUnlock()

	return as.blockedIPs[ipAddress]
}

// SetMediaPassword sets a password for accessing specific media
func (as *AdminService) SetMediaPassword(mediaPath, password, createdBy string) error {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	id, err := generateID()
	if err != nil {
		return fmt.Errorf("failed to generate access ID: %w", err)
	}

	access := &models.MediaAccess{
		ID:           id,
		MediaPath:    mediaPath,
		Password:     password,
		CreatedAt:    time.Now(),
		CreatedBy:    createdBy,
		AccessCount:  0,
		LastAccessed: time.Time{},
		IsActive:     true,
	}

	as.mediaAccess[mediaPath] = access
	as.LogActivity(createdBy, "media_password_set", fmt.Sprintf("Password set for media: %s", mediaPath), "", true, "")

	return nil
}

// CheckMediaPassword checks if the provided password is correct for the media
func (as *AdminService) CheckMediaPassword(mediaPath, password string) bool {
	as.mutex.RLock()
	access, exists := as.mediaAccess[mediaPath]
	as.mutex.RUnlock()

	if !exists || !access.IsActive {
		return true // No password required
	}

	if access.Password == password {
		// Update access statistics
		as.mutex.Lock()
		access.AccessCount++
		access.LastAccessed = time.Now()
		as.mutex.Unlock()
		return true
	}

	return false
}

// RemoveMediaPassword removes password protection from media
func (as *AdminService) RemoveMediaPassword(mediaPath, removedBy string) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	if access, exists := as.mediaAccess[mediaPath]; exists {
		access.IsActive = false
		as.LogActivity(removedBy, "media_password_removed", fmt.Sprintf("Password removed from media: %s", mediaPath), "", true, "")
	}
}

// LogActivity logs user activity
func (as *AdminService) LogActivity(ipAddress, action, resource, userAgent string, success bool, details string) {
	as.logMutex.Lock()
	defer as.logMutex.Unlock()

	id, _ := generateID()

	log := models.ActivityLog{
		ID:        id,
		IPAddress: ipAddress,
		Action:    action,
		Resource:  resource,
		Timestamp: time.Now(),
		UserAgent: userAgent,
		Success:   success,
		Details:   details,
	}

	as.activityLogs = append(as.activityLogs, log)

	// Keep only last 1000 logs
	if len(as.activityLogs) > 1000 {
		as.activityLogs = as.activityLogs[len(as.activityLogs)-1000:]
	}
}

// GetRecentActivity returns recent activity logs
func (as *AdminService) GetRecentActivity(limit int) []models.ActivityLog {
	as.logMutex.RLock()
	defer as.logMutex.RUnlock()

	if limit <= 0 || limit > len(as.activityLogs) {
		limit = len(as.activityLogs)
	}

	// Return last 'limit' entries
	start := len(as.activityLogs) - limit
	if start < 0 {
		start = 0
	}

	logs := make([]models.ActivityLog, limit)
	copy(logs, as.activityLogs[start:])

	// Reverse to show newest first
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	return logs
}

// GetStats returns dashboard statistics
func (as *AdminService) GetStats() *models.AdminStats {
	// Get active connections (this also cleans up inactive ones)
	activeConnections := as.GetActiveConnections()

	as.mutex.RLock()
	totalConnectionsEver := as.totalConnectionsEver
	as.mutex.RUnlock()

	totalBytes := int64(0)
	totalSpeed := float64(0)
	blockedCount := 0

	for _, conn := range activeConnections {
		totalBytes += conn.BytesServed
		totalSpeed += conn.CurrentSpeed
		if conn.IsBlocked {
			blockedCount++
		}
	}

	avgSpeed := float64(0)
	if len(activeConnections) > 0 {
		avgSpeed = totalSpeed / float64(len(activeConnections))
	}

	// Get performance metrics
	var performanceMetrics *models.PerformanceMetrics
	if as.performanceService != nil {
		performanceMetrics = as.performanceService.GetMetrics()
	}

	// Get streaming metrics
	streamingMetrics := as.GetStreamingMetrics()

	// Get cache stats
	var cacheStats *models.CacheStats
	if as.cacheService != nil {
		stats := as.cacheService.GetStats()
		cacheStats = &stats
	}

	return &models.AdminStats{
		TotalConnections:   totalConnectionsEver,
		ActiveConnections:  len(activeConnections),
		BlockedConnections: blockedCount,
		TotalBytesServed:   totalBytes,
		AverageSpeed:       avgSpeed,
		RecentActivity:     as.GetRecentActivity(10),
		SystemUptime:       time.Since(as.startTime),
		PerformanceMetrics: performanceMetrics,
		StreamingMetrics:   &streamingMetrics,
		CacheStats:         cacheStats,
	}
}

// generateID generates a random hex ID
func generateID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// getLocationFromIP returns a location string for an IP (simplified)
func (as *AdminService) getLocationFromIP(ip string) string {
	if models.IsLocalhost(ip) {
		return "Local"
	}
	// In a real implementation, you would use a GeoIP service
	return "Unknown"
}

// StartStream records the start of a new stream
func (as *AdminService) StartStream() {
	as.streamingMutex.Lock()
	defer as.streamingMutex.Unlock()

	as.streamingMetrics.ActiveStreams++
	as.streamingMetrics.TotalStreams++

	if as.streamingMetrics.ActiveStreams > as.streamingMetrics.ConcurrentPeak {
		as.streamingMetrics.ConcurrentPeak = as.streamingMetrics.ActiveStreams
	}
}

// EndStream records the end of a stream
func (as *AdminService) EndStream(bytesStreamed int64, duration time.Duration) {
	as.streamingMutex.Lock()
	defer as.streamingMutex.Unlock()

	as.streamingMetrics.ActiveStreams--
	as.streamingMetrics.BytesStreamed += bytesStreamed

	// Calculate speed for this stream
	if duration > 0 {
		speed := float64(bytesStreamed) / duration.Seconds()

		// Update average speed (simple moving average)
		if as.streamingMetrics.TotalStreams > 1 {
			totalStreams := float64(as.streamingMetrics.TotalStreams)
			as.streamingMetrics.AverageSpeed = (as.streamingMetrics.AverageSpeed*(totalStreams-1) + speed) / totalStreams
		} else {
			as.streamingMetrics.AverageSpeed = speed
		}

		// Update peak speed
		if speed > as.streamingMetrics.PeakSpeed {
			as.streamingMetrics.PeakSpeed = speed
		}
	}
}

// GetStreamingMetrics returns current streaming metrics
func (as *AdminService) GetStreamingMetrics() models.StreamingMetrics {
	as.streamingMutex.RLock()
	defer as.streamingMutex.RUnlock()

	// Return a copy to avoid race conditions
	return as.streamingMetrics
}

// GetPerformanceMetrics returns performance metrics if available
func (as *AdminService) GetPerformanceMetrics() *models.PerformanceMetrics {
	as.mutex.RLock()
	defer as.mutex.RUnlock()

	if as.performanceService != nil {
		return as.performanceService.GetMetrics()
	}
	return nil
}
