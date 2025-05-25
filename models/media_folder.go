package models

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MediaFolder represents a configured media folder
type MediaFolder struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	IsDefault   bool      `json:"is_default"`
	AddedBy     string    `json:"added_by"`
	AddedAt     time.Time `json:"added_at"`
	LastScanned time.Time `json:"last_scanned"`
	FileCount   int       `json:"file_count"`
	TotalSize   int64     `json:"total_size"`
	MediaTypes  []string  `json:"media_types"`
}

// MediaFolderStats represents statistics for a media folder
type MediaFolderStats struct {
	TotalFiles   int                `json:"total_files"`
	TotalSize    int64              `json:"total_size"`
	MediaTypes   map[string]int     `json:"media_types"`
	LastModified time.Time          `json:"last_modified"`
	ScanDuration time.Duration      `json:"scan_duration"`
	Errors       []string           `json:"errors,omitempty"`
}

// MediaFolderRequest represents a request to add a new media folder
type MediaFolderRequest struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
	SetDefault  bool   `json:"set_default"`
}

// ValidateMediaFolder validates a media folder configuration
func ValidateMediaFolder(req *MediaFolderRequest) error {
	if req.Name == "" {
		return NewValidationError("name", "Name is required")
	}

	if req.Path == "" {
		return NewValidationError("path", "Path is required")
	}

	// Check if path exists and is accessible
	info, err := os.Stat(req.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewValidationError("path", "Path does not exist")
		}
		return NewValidationError("path", "Cannot access path: "+err.Error())
	}

	if !info.IsDir() {
		return NewValidationError("path", "Path must be a directory")
	}

	// Check if we can read the directory
	_, err = os.ReadDir(req.Path)
	if err != nil {
		return NewValidationError("path", "Cannot read directory: "+err.Error())
	}

	return nil
}

// GetAbsolutePath returns the absolute path of the media folder
func (mf *MediaFolder) GetAbsolutePath() (string, error) {
	return filepath.Abs(mf.Path)
}

// IsAccessible checks if the media folder is accessible
func (mf *MediaFolder) IsAccessible() bool {
	info, err := os.Stat(mf.Path)
	if err != nil {
		return false
	}

	if !info.IsDir() {
		return false
	}

	// Try to read the directory
	_, err = os.ReadDir(mf.Path)
	return err == nil
}

// GetDisplayPath returns a user-friendly display path
func (mf *MediaFolder) GetDisplayPath() string {
	if absPath, err := mf.GetAbsolutePath(); err == nil {
		return absPath
	}
	return mf.Path
}

// ScanFolder scans the media folder and returns statistics
func (mf *MediaFolder) ScanFolder() (*MediaFolderStats, error) {
	startTime := time.Now()
	stats := &MediaFolderStats{
		MediaTypes: make(map[string]int),
		Errors:     make([]string, 0),
	}

	err := filepath.Walk(mf.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			stats.Errors = append(stats.Errors, err.Error())
			return nil // Continue walking
		}

		if info.IsDir() {
			return nil
		}

		// Count files and sizes
		stats.TotalFiles++
		stats.TotalSize += info.Size()

		// Track last modified
		if info.ModTime().After(stats.LastModified) {
			stats.LastModified = info.ModTime()
		}

		// Categorize by media type
		ext := filepath.Ext(info.Name())
		if IsMediaFile(ext) {
			mediaType := GetMediaType(ext)
			stats.MediaTypes[mediaType]++
		}

		return nil
	})

	stats.ScanDuration = time.Since(startTime)

	if err != nil {
		stats.Errors = append(stats.Errors, err.Error())
	}

	return stats, err
}

// GetMediaType returns the media type for a file extension
func GetMediaType(ext string) string {
	videoExts := []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".3gp", ".ogv"}
	audioExts := []string{".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a", ".opus"}
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".tiff", ".ico"}

	extLower := strings.ToLower(ext)

	for _, videoExt := range videoExts {
		if extLower == videoExt {
			return "video"
		}
	}

	for _, audioExt := range audioExts {
		if extLower == audioExt {
			return "audio"
		}
	}

	for _, imageExt := range imageExts {
		if extLower == imageExt {
			return "image"
		}
	}

	return "other"
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
