package middleware

import (
	"bufio"
	"context"
	"fmt"
	"media-server/models"
	"media-server/services"
	"net"
	"net/http"
	"strings"
)

// AdminMiddleware provides admin authentication and authorization
type AdminMiddleware struct {
	adminService *services.AdminService
}

// NewAdminMiddleware creates a new AdminMiddleware instance
func NewAdminMiddleware(adminService *services.AdminService) *AdminMiddleware {
	return &AdminMiddleware{
		adminService: adminService,
	}
}

// AdminAuth middleware for admin authentication
func (am *AdminMiddleware) AdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		clientIP := models.GetClientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Real-IP"))

		// Check if accessing from localhost
		isLocalhost := models.IsLocalhost(clientIP)

		// Check if user is in admin whitelist
		isAdmin := am.adminService.IsAdminUser(clientIP)

		// Allow access if from localhost OR if user is whitelisted admin
		if !isLocalhost && !isAdmin {
			am.adminService.LogActivity(clientIP, "admin_access_denied", r.URL.Path, r.UserAgent(), false, "Not authorized for admin access")
			http.Error(w, "Access denied. Admin access is restricted to localhost or authorized IPs.", http.StatusForbidden)
			return
		}

		// Add admin info to context
		ctx := context.WithValue(r.Context(), "admin_ip", clientIP)
		ctx = context.WithValue(ctx, "is_localhost", isLocalhost)

		if isAdmin {
			admin, _ := am.adminService.GetAdminUser(clientIP)
			ctx = context.WithValue(ctx, "admin_user", admin)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ConnectionTracking middleware for tracking user connections and activity
func (am *AdminMiddleware) ConnectionTracking(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		clientIP := models.GetClientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Real-IP"))

		// Check if IP is blocked
		if am.adminService.IsBlocked(clientIP) {
			am.adminService.LogActivity(clientIP, "blocked_access_attempt", r.URL.Path, r.UserAgent(), false, "IP is blocked")
			http.Error(w, "Access denied. Your IP address has been blocked.", http.StatusForbidden)
			return
		}

		// Track connection
		connection := am.adminService.TrackConnection(clientIP, r.UserAgent())

		// Create a custom response writer to track bytes served
		tracker := &responseTracker{
			ResponseWriter: w,
			connection:     connection,
			adminService:   am.adminService,
		}

		// Add connection info to context
		ctx := context.WithValue(r.Context(), "connection_id", connection.ID)
		ctx = context.WithValue(ctx, "client_ip", clientIP)

		// Log the request
		am.adminService.LogActivity(clientIP, "request", r.URL.Path, r.UserAgent(), true, r.Method)

		next.ServeHTTP(tracker, r.WithContext(ctx))
	})
}

// MediaPasswordAuth middleware for password-protected media
func (am *AdminMiddleware) MediaPasswordAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to media streaming requests
		if !strings.HasPrefix(r.URL.Path, "/stream/") && !strings.HasPrefix(r.URL.Path, "/player/") {
			next.ServeHTTP(w, r)
			return
		}

		// Extract media path
		var mediaPath string
		if strings.HasPrefix(r.URL.Path, "/stream/") {
			mediaPath = strings.TrimPrefix(r.URL.Path, "/stream/")
		} else if strings.HasPrefix(r.URL.Path, "/player/") {
			mediaPath = strings.TrimPrefix(r.URL.Path, "/player/")
		}

		// Check if password is required
		password := r.Header.Get("X-Media-Password")
		if password == "" {
			password = r.URL.Query().Get("password")
		}

		if !am.adminService.CheckMediaPassword(mediaPath, password) {
			clientIP := models.GetClientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Real-IP"))
			am.adminService.LogActivity(clientIP, "media_access_denied", mediaPath, r.UserAgent(), false, "Invalid or missing password")

			// Return 401 with custom header to indicate password required
			w.Header().Set("X-Password-Required", "true")
			w.Header().Set("X-Media-Path", mediaPath)
			http.Error(w, "Password required for this media", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// responseTracker wraps http.ResponseWriter to track bytes served
type responseTracker struct {
	http.ResponseWriter
	connection   *models.Connection
	adminService *services.AdminService
	bytesWritten int64
}

// Write tracks bytes written
func (rt *responseTracker) Write(data []byte) (int, error) {
	n, err := rt.ResponseWriter.Write(data)
	rt.bytesWritten += int64(n)

	// Update connection statistics
	if rt.connection != nil && rt.adminService != nil {
		rt.adminService.UpdateConnection(rt.connection.ID, int64(n))
	}

	return n, err
}

// WriteHeader captures the status code
func (rt *responseTracker) WriteHeader(code int) {
	rt.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker interface
func (rt *responseTracker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rt.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

// Flush implements http.Flusher interface
func (rt *responseTracker) Flush() {
	if flusher, ok := rt.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
