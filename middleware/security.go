package middleware

import (
	"net/http"
	"strings"
)

// Security middleware adds security headers and basic protections
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Basic path traversal protection (additional to service layer)
		if strings.Contains(r.URL.Path, "..") {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		
		// Prevent access to hidden files
		pathParts := strings.Split(r.URL.Path, "/")
		for _, part := range pathParts {
			if strings.HasPrefix(part, ".") && part != "." && part != ".." {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}
		}
		
		next.ServeHTTP(w, r)
	})
}
