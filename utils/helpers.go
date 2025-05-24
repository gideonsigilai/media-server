package utils

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FormatFileSize formats a file size in bytes to a human-readable string
func FormatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// SanitizePath ensures a path is safe and doesn't contain directory traversal attempts
func SanitizePath(path string) string {
	// Remove leading slash and clean the path
	path = strings.TrimPrefix(path, "/")
	path = filepath.Clean(path)
	
	// Remove any remaining attempts at directory traversal
	path = strings.ReplaceAll(path, "..", "")
	
	return path
}

// IsValidPath checks if a path is valid and safe
func IsValidPath(requestPath, basePath string) bool {
	// Clean and join the paths
	fullPath := filepath.Join(basePath, requestPath)
	
	// Ensure the resulting path is still within the base directory
	return strings.HasPrefix(fullPath, basePath)
}

// SplitPath splits a path into its components for breadcrumb navigation
func SplitPath(path string) []string {
	if path == "" || path == "." {
		return []string{}
	}
	
	// Clean the path and split by separator
	cleanPath := filepath.Clean(path)
	parts := strings.Split(cleanPath, string(filepath.Separator))
	
	// Filter out empty parts
	var result []string
	for _, part := range parts {
		if part != "" && part != "." {
			result = append(result, part)
		}
	}
	
	return result
}

// JoinPath joins path components safely
func JoinPath(parts ...string) string {
	return filepath.Join(parts...)
}

// GetParentPath returns the parent directory of a given path
func GetParentPath(path string) string {
	if path == "" || path == "." {
		return ""
	}
	
	parent := filepath.Dir(path)
	if parent == "." {
		return ""
	}
	
	return parent
}
