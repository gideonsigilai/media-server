package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"media-server/models"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MediaFolderService manages multiple media folders
type MediaFolderService struct {
	folders      map[string]*models.MediaFolder
	defaultFolder string
	mutex        sync.RWMutex
	scanMutex    sync.Mutex
}

// NewMediaFolderService creates a new MediaFolderService
func NewMediaFolderService(defaultPath string) *MediaFolderService {
	service := &MediaFolderService{
		folders: make(map[string]*models.MediaFolder),
	}

	// Add default folder if provided
	if defaultPath != "" {
		defaultFolder := &models.MediaFolder{
			ID:          service.generateID(),
			Name:        "Default Media Folder",
			Path:        defaultPath,
			Description: "Default media folder configured at startup",
			IsActive:    true,
			IsDefault:   true,
			AddedBy:     "system",
			AddedAt:     time.Now(),
		}

		service.folders[defaultFolder.ID] = defaultFolder
		service.defaultFolder = defaultFolder.ID

		log.Printf("Added default media folder: %s", defaultPath)
	}

	return service
}

// AddFolder adds a new media folder
func (mfs *MediaFolderService) AddFolder(req *models.MediaFolderRequest, addedBy string) (*models.MediaFolder, error) {
	// Validate the request
	if err := models.ValidateMediaFolder(req); err != nil {
		return nil, err
	}

	mfs.mutex.Lock()
	defer mfs.mutex.Unlock()

	// Check if path already exists
	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %v", err)
	}

	for _, folder := range mfs.folders {
		existingAbs, _ := filepath.Abs(folder.Path)
		if existingAbs == absPath {
			return nil, fmt.Errorf("folder path already exists: %s", absPath)
		}
	}

	// Create new media folder
	folder := &models.MediaFolder{
		ID:          mfs.generateID(),
		Name:        req.Name,
		Path:        req.Path,
		Description: req.Description,
		IsActive:    true,
		IsDefault:   false,
		AddedBy:     addedBy,
		AddedAt:     time.Now(),
	}

	// Set as default if requested and no default exists, or if explicitly requested
	if req.SetDefault || mfs.defaultFolder == "" {
		// Remove default flag from existing folders
		for _, existingFolder := range mfs.folders {
			existingFolder.IsDefault = false
		}
		folder.IsDefault = true
		mfs.defaultFolder = folder.ID
	}

	// Add to collection
	mfs.folders[folder.ID] = folder

	// Scan folder in background
	go mfs.scanFolderAsync(folder.ID)

	log.Printf("Added media folder '%s' at path: %s", folder.Name, folder.Path)
	return folder, nil
}

// RemoveFolder removes a media folder
func (mfs *MediaFolderService) RemoveFolder(folderID string) error {
	mfs.mutex.Lock()
	defer mfs.mutex.Unlock()

	folder, exists := mfs.folders[folderID]
	if !exists {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	// Cannot remove default folder if it's the only one
	if folder.IsDefault && len(mfs.folders) == 1 {
		return fmt.Errorf("cannot remove the only media folder")
	}

	// Remove folder
	delete(mfs.folders, folderID)

	// If this was the default folder, set another as default
	if folder.IsDefault && len(mfs.folders) > 0 {
		for id, remainingFolder := range mfs.folders {
			remainingFolder.IsDefault = true
			mfs.defaultFolder = id
			break
		}
	}

	log.Printf("Removed media folder '%s'", folder.Name)
	return nil
}

// GetFolder returns a specific media folder
func (mfs *MediaFolderService) GetFolder(folderID string) (*models.MediaFolder, error) {
	mfs.mutex.RLock()
	defer mfs.mutex.RUnlock()

	folder, exists := mfs.folders[folderID]
	if !exists {
		return nil, fmt.Errorf("folder not found: %s", folderID)
	}

	return folder, nil
}

// GetAllFolders returns all media folders
func (mfs *MediaFolderService) GetAllFolders() []*models.MediaFolder {
	mfs.mutex.RLock()
	defer mfs.mutex.RUnlock()

	folders := make([]*models.MediaFolder, 0, len(mfs.folders))
	for _, folder := range mfs.folders {
		folders = append(folders, folder)
	}

	return folders
}

// GetActiveFolders returns only active media folders
func (mfs *MediaFolderService) GetActiveFolders() []*models.MediaFolder {
	mfs.mutex.RLock()
	defer mfs.mutex.RUnlock()

	folders := make([]*models.MediaFolder, 0)
	for _, folder := range mfs.folders {
		if folder.IsActive {
			folders = append(folders, folder)
		}
	}

	return folders
}

// GetDefaultFolder returns the default media folder
func (mfs *MediaFolderService) GetDefaultFolder() *models.MediaFolder {
	mfs.mutex.RLock()
	defer mfs.mutex.RUnlock()

	if mfs.defaultFolder != "" {
		return mfs.folders[mfs.defaultFolder]
	}

	return nil
}

// SetDefaultFolder sets a folder as the default
func (mfs *MediaFolderService) SetDefaultFolder(folderID string) error {
	mfs.mutex.Lock()
	defer mfs.mutex.Unlock()

	folder, exists := mfs.folders[folderID]
	if !exists {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	// Remove default flag from all folders
	for _, existingFolder := range mfs.folders {
		existingFolder.IsDefault = false
	}

	// Set new default
	folder.IsDefault = true
	mfs.defaultFolder = folderID

	log.Printf("Set default media folder to '%s'", folder.Name)
	return nil
}

// ToggleFolderActive toggles the active status of a folder
func (mfs *MediaFolderService) ToggleFolderActive(folderID string) error {
	mfs.mutex.Lock()
	defer mfs.mutex.Unlock()

	folder, exists := mfs.folders[folderID]
	if !exists {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	folder.IsActive = !folder.IsActive

	log.Printf("Toggled folder '%s' active status to: %v", folder.Name, folder.IsActive)
	return nil
}

// ScanFolder scans a specific folder for media files
func (mfs *MediaFolderService) ScanFolder(folderID string) (*models.MediaFolderStats, error) {
	mfs.scanMutex.Lock()
	defer mfs.scanMutex.Unlock()

	folder, err := mfs.GetFolder(folderID)
	if err != nil {
		return nil, err
	}

	if !folder.IsAccessible() {
		return nil, fmt.Errorf("folder is not accessible: %s", folder.Path)
	}

	stats, err := folder.ScanFolder()
	if err != nil {
		log.Printf("Error scanning folder '%s': %v", folder.Name, err)
		return stats, err
	}

	// Update folder statistics
	mfs.mutex.Lock()
	folder.LastScanned = time.Now()
	folder.FileCount = stats.TotalFiles
	folder.TotalSize = stats.TotalSize

	// Update media types
	folder.MediaTypes = make([]string, 0, len(stats.MediaTypes))
	for mediaType := range stats.MediaTypes {
		folder.MediaTypes = append(folder.MediaTypes, mediaType)
	}
	mfs.mutex.Unlock()

	log.Printf("Scanned folder '%s': %d files, %s", folder.Name, stats.TotalFiles, formatBytes(stats.TotalSize))
	return stats, nil
}

// scanFolderAsync scans a folder asynchronously
func (mfs *MediaFolderService) scanFolderAsync(folderID string) {
	_, err := mfs.ScanFolder(folderID)
	if err != nil {
		log.Printf("Background scan failed for folder %s: %v", folderID, err)
	}
}

// ScanAllFolders scans all active folders
func (mfs *MediaFolderService) ScanAllFolders() map[string]*models.MediaFolderStats {
	folders := mfs.GetActiveFolders()
	results := make(map[string]*models.MediaFolderStats)

	for _, folder := range folders {
		stats, err := mfs.ScanFolder(folder.ID)
		if err != nil {
			log.Printf("Failed to scan folder '%s': %v", folder.Name, err)
			continue
		}
		results[folder.ID] = stats
	}

	return results
}

// ResolvePath resolves a media path to the actual file system path
func (mfs *MediaFolderService) ResolvePath(mediaPath string) (string, *models.MediaFolder, error) {
	// Try to find the file in any active folder
	folders := mfs.GetActiveFolders()

	for _, folder := range folders {
		fullPath := filepath.Join(folder.Path, mediaPath)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, folder, nil
		}
	}

	// If not found in any folder, try default folder
	defaultFolder := mfs.GetDefaultFolder()
	if defaultFolder != nil {
		fullPath := filepath.Join(defaultFolder.Path, mediaPath)
		return fullPath, defaultFolder, nil
	}

	return "", nil, fmt.Errorf("media file not found: %s", mediaPath)
}

// generateID generates a unique ID for folders
func (mfs *MediaFolderService) generateID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// formatBytes formats bytes into human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
