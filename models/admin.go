package models

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// AdminUser represents an admin user with IP-based authentication
type AdminUser struct {
	ID          string    `json:"id"`
	IPAddress   string    `json:"ip_address"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeen    time.Time `json:"last_seen"`
	IsActive    bool      `json:"is_active"`
	Permissions []string  `json:"permissions"`
}

// Connection represents an active user connection
type Connection struct {
	ID            string        `json:"id"`
	IPAddress     string        `json:"ip_address"`
	UserAgent     string        `json:"user_agent"`
	ConnectedAt   time.Time     `json:"connected_at"`
	LastActivity  time.Time     `json:"last_activity"`
	BytesServed   int64         `json:"bytes_served"`
	RequestCount  int           `json:"request_count"`
	CurrentSpeed  float64       `json:"current_speed"` // bytes per second
	IsBlocked     bool          `json:"is_blocked"`
	BlockedReason string        `json:"blocked_reason"`
	Location      string        `json:"location"`
	mutex         sync.RWMutex  `json:"-"`
}

// ActivityLog represents user activity history
type ActivityLog struct {
	ID        string    `json:"id"`
	IPAddress string    `json:"ip_address"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent"`
	Success   bool      `json:"success"`
	Details   string    `json:"details"`
}

// MediaAccess represents access control for media files
type MediaAccess struct {
	ID           string    `json:"id"`
	MediaPath    string    `json:"media_path"`
	Password     string    `json:"password"`
	CreatedAt    time.Time `json:"created_at"`
	CreatedBy    string    `json:"created_by"`
	AccessCount  int       `json:"access_count"`
	LastAccessed time.Time `json:"last_accessed"`
	IsActive     bool      `json:"is_active"`
}

// AdminStats represents dashboard statistics
type AdminStats struct {
	TotalConnections    int                  `json:"total_connections"`
	ActiveConnections   int                  `json:"active_connections"`
	BlockedConnections  int                  `json:"blocked_connections"`
	TotalBytesServed    int64                `json:"total_bytes_served"`
	AverageSpeed        float64              `json:"average_speed"`
	TopMediaFiles       []MediaStats         `json:"top_media_files"`
	RecentActivity      []ActivityLog        `json:"recent_activity"`
	SystemUptime        time.Duration        `json:"system_uptime"`
	PerformanceMetrics  *PerformanceMetrics  `json:"performance_metrics,omitempty"`
	StreamingMetrics    *StreamingMetrics    `json:"streaming_metrics,omitempty"`
	CacheStats          *CacheStats          `json:"cache_stats,omitempty"`
}

// MediaStats represents statistics for individual media files
type MediaStats struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	AccessCount int    `json:"access_count"`
	BytesServed int64  `json:"bytes_served"`
	LastAccess  time.Time `json:"last_access"`
}

// UpdateActivity updates the connection's activity and calculates speed
func (c *Connection) UpdateActivity(bytesTransferred int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	timeDiff := now.Sub(c.LastActivity).Seconds()

	if timeDiff > 0 {
		c.CurrentSpeed = float64(bytesTransferred) / timeDiff
	}

	c.LastActivity = now
	c.BytesServed += bytesTransferred
	c.RequestCount++
}

// GetFormattedSpeed returns a human-readable speed string
func (c *Connection) GetFormattedSpeed() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	speed := c.CurrentSpeed
	if speed < 1024 {
		return "< 1 KB/s"
	} else if speed < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", speed/1024)
	} else if speed < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB/s", speed/(1024*1024))
	} else {
		return fmt.Sprintf("%.1f GB/s", speed/(1024*1024*1024))
	}
}

// GetConnectionDuration returns how long the connection has been active
func (c *Connection) GetConnectionDuration() time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return time.Since(c.ConnectedAt)
}

// IsLocalhost checks if the IP address is localhost
func IsLocalhost(ip string) bool {
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return true
	}

	// Parse IP to handle cases like "127.0.0.1:port"
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		host = ip
	}

	parsedIP := net.ParseIP(host)
	if parsedIP == nil {
		return false
	}

	return parsedIP.IsLoopback()
}

// GetClientIP extracts the real client IP from the request
func GetClientIP(remoteAddr, xForwardedFor, xRealIP string) string {
	// Check X-Forwarded-For header first
	if xForwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xRealIP != "" {
		return xRealIP
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}

	return host
}
