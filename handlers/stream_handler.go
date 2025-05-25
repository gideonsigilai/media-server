package handlers

import (
	"fmt"
	"io"
	"log"
	"media-server/config"
	"media-server/services"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StreamHandler handles file streaming with optimized performance
type StreamHandler struct {
	fileService        *services.FileService
	fileServer         http.Handler
	adminService       *services.AdminService
	cacheService       *services.CacheService
	performanceService *services.PerformanceService
	bufferPool         *sync.Pool
}

// NewStreamHandler creates a new StreamHandler instance
func NewStreamHandler(cfg *config.Config) *StreamHandler {
	fileService := services.NewFileService(cfg.MediaDir)
	fileServer := http.FileServer(http.Dir(cfg.MediaDir))

	return &StreamHandler{
		fileService: fileService,
		fileServer:  fileServer,
		bufferPool:  createBufferPool(),
	}
}

// NewStreamHandlerWithServices creates a new StreamHandler instance with enhanced services
func NewStreamHandlerWithServices(cfg *config.Config, adminService *services.AdminService,
	cacheService *services.CacheService, performanceService *services.PerformanceService,
	mediaFolderService *services.MediaFolderService) *StreamHandler {

	fileService := services.NewFileServiceWithMediaFolders(cfg.MediaDir, cacheService, performanceService, mediaFolderService)
	fileServer := http.FileServer(http.Dir(cfg.MediaDir))

	return &StreamHandler{
		fileService:        fileService,
		fileServer:         fileServer,
		adminService:       adminService,
		cacheService:       cacheService,
		performanceService: performanceService,
		bufferPool:         createBufferPool(),
	}
}

// createBufferPool creates a pool of buffers for efficient streaming
func createBufferPool() *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			// Create 64KB buffers for optimal streaming performance
			return make([]byte, 64*1024)
		},
	}
}

// HandleStream handles file streaming requests with proper range support
func (sh *StreamHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	// Extract path from URL (remove /stream/ prefix)
	path := strings.TrimPrefix(r.URL.Path, "/stream/")
	log.Printf("Streaming request for path: %s, method: %s, range: %s", path, r.Method, r.Header.Get("Range"))

	// Handle OPTIONS requests for CORS
	if r.Method == "OPTIONS" {
		sh.setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Validate the file path
	fullPath, err := sh.fileService.ValidateFilePath(path)
	if err != nil {
		log.Printf("File validation failed for path %s: %v", path, err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	log.Printf("Serving file: %s", fullPath)

	// Get file info
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		log.Printf("Error getting file info for %s: %v", fullPath, err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Set basic streaming headers
	sh.setBasicStreamingHeaders(w, path, fileInfo.Size())

	// Handle HEAD requests
	if r.Method == "HEAD" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Handle range requests for progressive streaming
	if r.Header.Get("Range") != "" {
		log.Printf("Handling range request: %s", r.Header.Get("Range"))
		sh.handleRangeRequest(w, r, fullPath, fileInfo.Size())
	} else {
		log.Printf("Serving complete file")
		sh.serveCompleteFile(w, r, fullPath)
	}
}

// setBasicStreamingHeaders sets basic headers for media file streaming
func (sh *StreamHandler) setBasicStreamingHeaders(w http.ResponseWriter, path string, fileSize int64) {
	// Get file extension and set proper content type
	ext := strings.ToLower(filepath.Ext(path))
	contentType := sh.getContentType(ext)
	w.Header().Set("Content-Type", contentType)

	// Enable range requests for media files (essential for video streaming)
	w.Header().Set("Accept-Ranges", "bytes")

	// Set content disposition to inline for streaming (not download)
	w.Header().Set("Content-Disposition", "inline")

	// Set cache headers for better performance
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Set content length
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))

	// Add CORS headers for cross-origin requests
	sh.setCORSHeaders(w)
}

// setCORSHeaders sets CORS headers
func (sh *StreamHandler) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
}

// getContentType returns the appropriate MIME type for media files
func (sh *StreamHandler) getContentType(ext string) string {
	// Try to get MIME type from Go's built-in mime package first
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}

	// Fallback to manual mapping for media files
	switch ext {
	// Video formats
	case ".mp4":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".flv":
		return "video/x-flv"
	case ".webm":
		return "video/webm"
	case ".m4v":
		return "video/x-m4v"
	case ".3gp":
		return "video/3gpp"
	case ".ts":
		return "video/mp2t"
	case ".mts", ".m2ts":
		return "video/mp2t"
	// Audio formats
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".aac":
		return "audio/aac"
	case ".ogg":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".m4a":
		return "audio/mp4"
	case ".wma":
		return "audio/x-ms-wma"
	case ".opus":
		return "audio/opus"
	case ".aiff":
		return "audio/aiff"
	// Image formats
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".tiff":
		return "image/tiff"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

// handleRangeRequest handles HTTP range requests for progressive streaming
func (sh *StreamHandler) handleRangeRequest(w http.ResponseWriter, r *http.Request, filePath string, fileSize int64) {
	rangeHeader := r.Header.Get("Range")

	// Parse range header (format: "bytes=start-end")
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		log.Printf("Invalid range header format: %s", rangeHeader)
		http.Error(w, "Invalid range header", http.StatusBadRequest)
		return
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	ranges := strings.Split(rangeSpec, ",")

	// Handle only the first range for simplicity
	if len(ranges) == 0 {
		log.Printf("No ranges found in header: %s", rangeHeader)
		http.Error(w, "Invalid range header", http.StatusBadRequest)
		return
	}

	rangeParts := strings.Split(strings.TrimSpace(ranges[0]), "-")
	if len(rangeParts) != 2 {
		log.Printf("Invalid range format: %s", ranges[0])
		http.Error(w, "Invalid range header", http.StatusBadRequest)
		return
	}

	var start, end int64
	var err error

	// Parse start position
	if rangeParts[0] != "" {
		start, err = strconv.ParseInt(rangeParts[0], 10, 64)
		if err != nil {
			log.Printf("Invalid range start: %s", rangeParts[0])
			http.Error(w, "Invalid range start", http.StatusBadRequest)
			return
		}
	} else {
		start = 0
	}

	// Parse end position
	if rangeParts[1] != "" {
		end, err = strconv.ParseInt(rangeParts[1], 10, 64)
		if err != nil {
			log.Printf("Invalid range end: %s", rangeParts[1])
			http.Error(w, "Invalid range end", http.StatusBadRequest)
			return
		}
	} else {
		// If no end specified, serve to end of file
		end = fileSize - 1
	}

	// Validate range
	if start < 0 || end >= fileSize || start > end {
		log.Printf("Range not satisfiable: %d-%d (file size: %d)", start, end, fileSize)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		http.Error(w, "Range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	log.Printf("Serving range: %d-%d/%d", start, end, fileSize)

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file %s: %v", filePath, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Seek to start position
	_, err = file.Seek(start, 0)
	if err != nil {
		log.Printf("Error seeking file %s: %v", filePath, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set range response headers (override content-length from basic headers)
	contentLength := end - start + 1
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.WriteHeader(http.StatusPartialContent)

	// Copy the requested range using optimized buffer
	written, err := sh.copyNWithBuffer(w, file, contentLength)
	if err != nil {
		log.Printf("Error copying file range %s (wrote %d bytes): %v", filePath, written, err)
	} else {
		log.Printf("Successfully served %d bytes", written)
	}
}

// serveCompleteFile serves the complete file for non-range requests with optimized streaming
func (sh *StreamHandler) serveCompleteFile(w http.ResponseWriter, r *http.Request, filePath string) {
	startTime := time.Now()

	// Track streaming start
	if sh.adminService != nil {
		sh.adminService.StartStream()
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file %s: %v", filePath, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set status OK and copy file content using optimized buffer
	w.WriteHeader(http.StatusOK)
	written, err := sh.copyWithBuffer(w, file)

	// Track streaming end
	duration := time.Since(startTime)
	if sh.adminService != nil {
		sh.adminService.EndStream(written, duration)
	}

	if err != nil {
		log.Printf("Error copying file %s (wrote %d bytes): %v", filePath, written, err)
	} else {
		log.Printf("Successfully served complete file: %d bytes in %v", written, duration)
	}
}

// copyWithBuffer copies data using a pooled buffer for better performance
func (sh *StreamHandler) copyWithBuffer(dst io.Writer, src io.Reader) (int64, error) {
	// Get buffer from pool
	buffer := sh.bufferPool.Get().([]byte)
	defer sh.bufferPool.Put(buffer)

	return io.CopyBuffer(dst, src, buffer)
}

// copyNWithBuffer copies N bytes using a pooled buffer for better performance
func (sh *StreamHandler) copyNWithBuffer(dst io.Writer, src io.Reader, n int64) (int64, error) {
	// Get buffer from pool
	buffer := sh.bufferPool.Get().([]byte)
	defer sh.bufferPool.Put(buffer)

	// Use a limited reader to ensure we don't read more than n bytes
	limitedReader := io.LimitReader(src, n)
	return io.CopyBuffer(dst, limitedReader, buffer)
}
