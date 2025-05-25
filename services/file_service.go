package services

import (
	"fmt"
	"log"
	"media-server/models"
	"media-server/utils"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// FileService handles file operations and business logic with caching and parallel processing
type FileService struct {
	baseDir            string
	cacheService       *CacheService
	performanceService *PerformanceService
	workerPool         *WorkerPool
}

// NewFileService creates a new FileService instance
func NewFileService(baseDir string) *FileService {
	return &FileService{
		baseDir: baseDir,
	}
}

// NewFileServiceWithCache creates a new FileService instance with caching and performance services
func NewFileServiceWithCache(baseDir string, cacheService *CacheService, performanceService *PerformanceService) *FileService {
	fs := &FileService{
		baseDir:            baseDir,
		cacheService:       cacheService,
		performanceService: performanceService,
	}

	// Create worker pool for file operations
	if performanceService != nil {
		fs.workerPool = performanceService.GetOrCreateWorkerPool("file-operations",
			performanceService.GetIOOptimalWorkerCount(), 100)
	}

	return fs
}

// ListDirectory lists the contents of a directory with caching and parallel processing
func (fs *FileService) ListDirectory(requestPath string) ([]*models.FileInfo, error) {
	// Sanitize the path
	cleanPath := utils.SanitizePath(requestPath)

	// Validate the path
	if !utils.IsValidPath(cleanPath, fs.baseDir) {
		return nil, fmt.Errorf("access denied: invalid path")
	}

	// Check cache first if available
	if fs.cacheService != nil {
		if cached, found := fs.cacheService.GetDirectoryListing(cleanPath); found {
			return cached, nil
		}
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

	// Process files in parallel if worker pool is available and there are many files
	var files []*models.FileInfo
	if fs.workerPool != nil && len(entries) > 10 {
		files = fs.processFilesParallel(entries, cleanPath)
	} else {
		files = fs.processFilesSequential(entries, cleanPath)
	}

	// Sort files: directories first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir // directories first
		}
		return files[i].Name < files[j].Name
	})

	// Cache the result if caching is available
	if fs.cacheService != nil {
		fs.cacheService.SetDirectoryListing(cleanPath, files)
	}

	return files, nil
}

// processFilesSequential processes files sequentially (original method)
func (fs *FileService) processFilesSequential(entries []os.DirEntry, cleanPath string) []*models.FileInfo {
	files := make([]*models.FileInfo, 0, len(entries))
	for _, entry := range entries {
		fileInfo, err := models.NewFileInfo(entry, cleanPath)
		if err != nil {
			log.Printf("Error getting file info for %s: %v", entry.Name(), err)
			continue
		}
		files = append(files, fileInfo)
	}
	return files
}

// processFilesParallel processes files in parallel using worker pool
func (fs *FileService) processFilesParallel(entries []os.DirEntry, cleanPath string) []*models.FileInfo {
	files := make([]*models.FileInfo, len(entries))
	var wg sync.WaitGroup
	var mutex sync.Mutex

	// Process files in batches
	batchSize := 10
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}

		batch := entries[i:end]
		wg.Add(1)

		// Submit batch processing task to worker pool
		taskID := fmt.Sprintf("file-batch-%d", i)
		err := fs.workerPool.Submit(taskID, func() error {
			defer wg.Done()

			batchFiles := make([]*models.FileInfo, 0, len(batch))
			for _, entry := range batch {
				fileInfo, err := models.NewFileInfo(entry, cleanPath)
				if err != nil {
					log.Printf("Error getting file info for %s: %v", entry.Name(), err)
					continue
				}
				batchFiles = append(batchFiles, fileInfo)
			}

			// Add batch results to main slice
			mutex.Lock()
			for j, file := range batchFiles {
				if i+j < len(files) {
					files[i+j] = file
				}
			}
			mutex.Unlock()

			return nil
		})

		if err != nil {
			// Fallback to sequential processing for this batch
			wg.Done()
			for _, entry := range batch {
				fileInfo, err := models.NewFileInfo(entry, cleanPath)
				if err != nil {
					log.Printf("Error getting file info for %s: %v", entry.Name(), err)
					continue
				}
				mutex.Lock()
				files = append(files, fileInfo)
				mutex.Unlock()
			}
		}
	}

	// Wait for all batches to complete
	wg.Wait()

	// Filter out nil entries and compact the slice
	compactFiles := make([]*models.FileInfo, 0, len(files))
	for _, file := range files {
		if file != nil {
			compactFiles = append(compactFiles, file)
		}
	}

	return compactFiles
}

// GetFileInfo gets information about a specific file or directory with caching
func (fs *FileService) GetFileInfo(requestPath string) (*models.FileInfo, error) {
	// Sanitize the path
	cleanPath := utils.SanitizePath(requestPath)

	// Validate the path
	if !utils.IsValidPath(cleanPath, fs.baseDir) {
		return nil, fmt.Errorf("access denied: invalid path")
	}

	// Check cache first if available
	if fs.cacheService != nil {
		if cached, found := fs.cacheService.GetFileInfo(cleanPath); found {
			return cached, nil
		}
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

	result := &models.FileInfo{
		Name:      fileInfo.Name(),
		IsDir:     fileInfo.IsDir(),
		Path:      cleanPath,
		Size:      fileInfo.Size(),
		Extension: extension,
		IsMedia:   isMedia,
	}

	// Cache the result if caching is available
	if fs.cacheService != nil {
		fs.cacheService.SetFileInfo(cleanPath, result)
	}

	return result, nil
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
