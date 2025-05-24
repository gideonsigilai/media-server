package services

import (
	"fmt"
	"log"
	"media-server/models"
	"media-server/utils"
	"os"
	"path/filepath"
	"sort"
)

// FileService handles file operations and business logic
type FileService struct {
	baseDir string
}

// NewFileService creates a new FileService instance
func NewFileService(baseDir string) *FileService {
	return &FileService{
		baseDir: baseDir,
	}
}

// ListDirectory lists the contents of a directory
func (fs *FileService) ListDirectory(requestPath string) ([]*models.FileInfo, error) {
	// Sanitize the path
	cleanPath := utils.SanitizePath(requestPath)
	
	// Validate the path
	if !utils.IsValidPath(cleanPath, fs.baseDir) {
		return nil, fmt.Errorf("access denied: invalid path")
	}
	
	// Build full path
	fullPath := filepath.Join(fs.baseDir, cleanPath)
	
	// Check if path exists and is a directory
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path not found")
		}
		return nil, fmt.Errorf("error accessing path: %w", err)
	}
	
	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}
	
	// Read directory contents
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %w", err)
	}
	
	// Convert to FileInfo structures
	files := make([]*models.FileInfo, 0, len(entries))
	for _, entry := range entries {
		fileInfo, err := models.NewFileInfo(entry, cleanPath)
		if err != nil {
			log.Printf("Error getting file info for %s: %v", entry.Name(), err)
			continue
		}
		files = append(files, fileInfo)
	}
	
	// Sort files: directories first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir // directories first
		}
		return files[i].Name < files[j].Name
	})
	
	return files, nil
}

// GetFileInfo gets information about a specific file or directory
func (fs *FileService) GetFileInfo(requestPath string) (*models.FileInfo, error) {
	// Sanitize the path
	cleanPath := utils.SanitizePath(requestPath)
	
	// Validate the path
	if !utils.IsValidPath(cleanPath, fs.baseDir) {
		return nil, fmt.Errorf("access denied: invalid path")
	}
	
	// Build full path
	fullPath := filepath.Join(fs.baseDir, cleanPath)
	
	// Get file info
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found")
		}
		return nil, fmt.Errorf("error accessing file: %w", err)
	}
	
	// Create FileInfo
	extension := filepath.Ext(fileInfo.Name())
	isMedia := models.IsMediaFile(extension)
	
	return &models.FileInfo{
		Name:      fileInfo.Name(),
		IsDir:     fileInfo.IsDir(),
		Path:      cleanPath,
		Size:      fileInfo.Size(),
		Extension: extension,
		IsMedia:   isMedia,
	}, nil
}

// ValidateFilePath validates that a file path is safe and accessible
func (fs *FileService) ValidateFilePath(requestPath string) (string, error) {
	// Sanitize the path
	cleanPath := utils.SanitizePath(requestPath)
	
	// Validate the path
	if !utils.IsValidPath(cleanPath, fs.baseDir) {
		return "", fmt.Errorf("access denied: invalid path")
	}
	
	// Build full path
	fullPath := filepath.Join(fs.baseDir, cleanPath)
	
	// Check if file exists
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found")
		}
		return "", fmt.Errorf("error accessing file: %w", err)
	}
	
	return fullPath, nil
}
