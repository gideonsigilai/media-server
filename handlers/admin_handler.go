package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"media-server/config"
	"media-server/models"
	"media-server/services"
	"media-server/utils"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AdminHandler handles admin functionality like uploads and settings
type AdminHandler struct {
	config             *config.Config
	templates          *template.Template
	adminService       *services.AdminService
	cacheService       *services.CacheService
	performanceService *services.PerformanceService
	sseClients         map[string]chan []byte
	sseClientsMutex    sync.RWMutex
}

// NewAdminHandler creates a new AdminHandler instance
func NewAdminHandler(cfg *config.Config, adminService *services.AdminService) *AdminHandler {
	// Load templates with custom functions
	funcMap := template.FuncMap{
		"splitPath":      utils.SplitPath,
		"joinPath":       utils.JoinPath,
		"formatFileSize": utils.FormatFileSize,
		"trimPrefix":     strings.TrimPrefix,
		"div": func(a, b interface{}) float64 {
			var aFloat, bFloat float64

			switch v := a.(type) {
			case int64:
				aFloat = float64(v)
			case int:
				aFloat = float64(v)
			case float64:
				aFloat = v
			case string:
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					aFloat = parsed
				}
			}

			switch v := b.(type) {
			case int64:
				bFloat = float64(v)
			case int:
				bFloat = float64(v)
			case float64:
				bFloat = v
			case string:
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					bFloat = parsed
				}
			}

			if bFloat == 0 {
				return 0
			}
			return aFloat / bFloat
		},
		"printf": fmt.Sprintf,
	}

	templates, err := template.New("").Funcs(funcMap).ParseGlob("views/templates/*.html")
	if err != nil {
		log.Fatalf("Error loading templates: %v", err)
	}

	return &AdminHandler{
		config:          cfg,
		templates:       templates,
		adminService:    adminService,
		sseClients:      make(map[string]chan []byte),
		sseClientsMutex: sync.RWMutex{},
	}
}

// NewAdminHandlerWithServices creates a new AdminHandler instance with enhanced services
func NewAdminHandlerWithServices(cfg *config.Config, adminService *services.AdminService,
	cacheService *services.CacheService, performanceService *services.PerformanceService) *AdminHandler {

	// Load templates with custom functions
	funcMap := template.FuncMap{
		"splitPath":      utils.SplitPath,
		"joinPath":       utils.JoinPath,
		"formatFileSize": utils.FormatFileSize,
		"trimPrefix":     strings.TrimPrefix,
		"div": func(a, b interface{}) float64 {
			var aFloat, bFloat float64

			switch v := a.(type) {
			case int64:
				aFloat = float64(v)
			case int:
				aFloat = float64(v)
			case float64:
				aFloat = v
			case string:
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					aFloat = parsed
				}
			}

			switch v := b.(type) {
			case int64:
				bFloat = float64(v)
			case int:
				bFloat = float64(v)
			case float64:
				bFloat = v
			case string:
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					bFloat = parsed
				}
			}

			if bFloat == 0 {
				return 0
			}
			return aFloat / bFloat
		},
		"printf": fmt.Sprintf,
	}

	templates, err := template.New("").Funcs(funcMap).ParseGlob("views/templates/*.html")
	if err != nil {
		log.Fatalf("Error loading templates: %v", err)
	}

	return &AdminHandler{
		config:             cfg,
		templates:          templates,
		adminService:       adminService,
		cacheService:       cacheService,
		performanceService: performanceService,
		sseClients:         make(map[string]chan []byte),
		sseClientsMutex:    sync.RWMutex{},
	}
}

// HandleSettings shows the settings/admin page
func (ah *AdminHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		ah.showSettingsPage(w, r)
	} else if r.Method == "POST" {
		ah.handleSettingsUpdate(w, r)
	}
}

// HandleUpload handles file uploads
func (ah *AdminHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (32MB max)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Get the uploaded files
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	uploadedFiles := []string{}
	errors := []string{}

	for _, fileHeader := range files {
		// Open the uploaded file
		file, err := fileHeader.Open()
		if err != nil {
			errors = append(errors, fmt.Sprintf("Error opening %s: %v", fileHeader.Filename, err))
			continue
		}
		defer file.Close()

		// Validate file type
		if !ah.isValidMediaFile(fileHeader.Filename) {
			errors = append(errors, fmt.Sprintf("Invalid file type: %s", fileHeader.Filename))
			continue
		}

		// Create destination file
		destPath := filepath.Join(ah.config.MediaDir, fileHeader.Filename)
		destFile, err := os.Create(destPath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Error creating %s: %v", fileHeader.Filename, err))
			continue
		}
		defer destFile.Close()

		// Copy file content
		_, err = io.Copy(destFile, file)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Error saving %s: %v", fileHeader.Filename, err))
			os.Remove(destPath) // Clean up partial file
			continue
		}

		uploadedFiles = append(uploadedFiles, fileHeader.Filename)
		log.Printf("File uploaded successfully: %s", fileHeader.Filename)
	}

	// Prepare response data
	data := struct {
		Title         string
		UploadedFiles []string
		Errors        []string
		Success       bool
	}{
		Title:         "Upload Results",
		UploadedFiles: uploadedFiles,
		Errors:        errors,
		Success:       len(uploadedFiles) > 0,
	}

	// Render response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ah.templates.ExecuteTemplate(w, "upload_result.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// showSettingsPage renders the settings page
func (ah *AdminHandler) showSettingsPage(w http.ResponseWriter, r *http.Request) {
	// Get available drives (Windows specific)
	drives := ah.getAvailableDrives()

	data := struct {
		Title       string
		CurrentDir  string
		Drives      []string
		Message     string
	}{
		Title:      "Media Server Settings",
		CurrentDir: ah.config.MediaDir,
		Drives:     drives,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ah.templates.ExecuteTemplate(w, "settings.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleSettingsUpdate processes settings updates
func (ah *AdminHandler) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	newMediaDir := r.FormValue("media_dir")
	if newMediaDir == "" {
		http.Error(w, "Media directory cannot be empty", http.StatusBadRequest)
		return
	}

	// Validate the directory
	if _, err := os.Stat(newMediaDir); os.IsNotExist(err) {
		http.Error(w, "Directory does not exist", http.StatusBadRequest)
		return
	}

	// Update configuration (in a real app, this would persist to a config file)
	ah.config.MediaDir = newMediaDir

	// Redirect back to settings with success message
	http.Redirect(w, r, "/settings?message=Directory updated successfully", http.StatusSeeOther)
}

// isValidMediaFile checks if a file is a valid media file
func (ah *AdminHandler) isValidMediaFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExts := map[string]bool{
		// Video
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
		// Audio
		".mp3": true, ".wav": true, ".aac": true, ".ogg": true,
		".flac": true, ".m4a": true, ".wma": true,
		// Images
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".webp": true,
	}
	return validExts[ext]
}

// getAvailableDrives returns available drives on Windows
func (ah *AdminHandler) getAvailableDrives() []string {
	drives := []string{}
	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		path := string(drive) + ":\\"
		if _, err := os.Stat(path); err == nil {
			drives = append(drives, path)
		}
	}
	return drives
}

// HandleBrowseFolder handles folder browsing for directory selection
func (ah *AdminHandler) HandleBrowseFolder(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "C:\\"
	}

	// Security check
	if !filepath.IsAbs(path) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, "Cannot read directory", http.StatusInternalServerError)
		return
	}

	folders := []map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			folders = append(folders, map[string]string{
				"name": entry.Name(),
				"path": filepath.Join(path, entry.Name()),
			})
		}
	}

	data := struct {
		CurrentPath string
		ParentPath  string
		Folders     []map[string]string
	}{
		CurrentPath: path,
		ParentPath:  filepath.Dir(path),
		Folders:     folders,
	}

	w.Header().Set("Content-Type", "application/json")
	// For simplicity, return HTML snippet instead of JSON
	w.Header().Set("Content-Type", "text/html")

	folderHTML := `<div class="folder-list">`
	if path != filepath.Dir(path) {
		folderHTML += fmt.Sprintf(`<div class="folder-item" data-path="%s">📁 ..</div>`, data.ParentPath)
	}
	for _, folder := range folders {
		folderHTML += fmt.Sprintf(`<div class="folder-item" data-path="%s">📁 %s</div>`, folder["path"], folder["name"])
	}
	folderHTML += `</div>`

	w.Write([]byte(folderHTML))
}

// HandleAdminDashboard handles the main admin dashboard
func (ah *AdminHandler) HandleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	// Get admin stats
	stats := ah.adminService.GetStats()
	activeConnections := ah.adminService.GetActiveConnections()
	adminUsers := ah.adminService.GetAllAdminUsers()

	data := struct {
		Title             string
		Stats             *models.AdminStats
		ActiveConnections []*models.Connection
		AdminUsers        []*models.AdminUser
		IsLocalhost       bool
		Config            *config.Config
	}{
		Title:             "Admin Dashboard",
		Stats:             stats,
		ActiveConnections: activeConnections,
		AdminUsers:        adminUsers,
		IsLocalhost:       r.Context().Value("is_localhost").(bool),
		Config:            ah.config,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ah.templates.ExecuteTemplate(w, "admin_dashboard.html", data); err != nil {
		log.Printf("Error executing admin dashboard template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleAddAdminUser handles adding new admin users
func (ah *AdminHandler) HandleAddAdminUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	ipAddress := r.FormValue("ip_address")

	if name == "" || ipAddress == "" {
		http.Error(w, "Name and IP address are required", http.StatusBadRequest)
		return
	}

	_, err := ah.adminService.AddAdminUser(name, ipAddress)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to add admin user: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
}

// HandleRemoveAdminUser handles removing admin users
func (ah *AdminHandler) HandleRemoveAdminUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ipAddress := r.FormValue("ip_address")
	if ipAddress == "" {
		http.Error(w, "IP address is required", http.StatusBadRequest)
		return
	}

	err := ah.adminService.RemoveAdminUser(ipAddress)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to remove admin user: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
}

// HandleBlockIP handles blocking IP addresses
func (ah *AdminHandler) HandleBlockIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ipAddress := r.FormValue("ip_address")
	reason := r.FormValue("reason")

	if ipAddress == "" {
		http.Error(w, "IP address is required", http.StatusBadRequest)
		return
	}

	if reason == "" {
		reason = "Blocked by admin"
	}

	ah.adminService.BlockIP(ipAddress, reason)

	if r.Header.Get("Content-Type") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	} else {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
	}
}

// HandleUnblockIP handles unblocking IP addresses
func (ah *AdminHandler) HandleUnblockIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ipAddress := r.FormValue("ip_address")
	if ipAddress == "" {
		http.Error(w, "IP address is required", http.StatusBadRequest)
		return
	}

	ah.adminService.UnblockIP(ipAddress)

	if r.Header.Get("Content-Type") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	} else {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
	}
}

// HandleSetMediaPassword handles setting passwords for media files
func (ah *AdminHandler) HandleSetMediaPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mediaPath := r.FormValue("media_path")
	password := r.FormValue("password")
	adminIP := r.Context().Value("admin_ip").(string)

	if mediaPath == "" || password == "" {
		http.Error(w, "Media path and password are required", http.StatusBadRequest)
		return
	}

	err := ah.adminService.SetMediaPassword(mediaPath, password, adminIP)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to set media password: %v", err), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("Content-Type") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	} else {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
	}
}

// HandleRemoveMediaPassword handles removing passwords from media files
func (ah *AdminHandler) HandleRemoveMediaPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mediaPath := r.FormValue("media_path")
	adminIP := r.Context().Value("admin_ip").(string)

	if mediaPath == "" {
		http.Error(w, "Media path is required", http.StatusBadRequest)
		return
	}

	ah.adminService.RemoveMediaPassword(mediaPath, adminIP)

	if r.Header.Get("Content-Type") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	} else {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
	}
}

// HandleConnectionsAPI provides JSON API for connections data
func (ah *AdminHandler) HandleConnectionsAPI(w http.ResponseWriter, r *http.Request) {
	connections := ah.adminService.GetActiveConnections()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(connections); err != nil {
		log.Printf("Error encoding connections JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleStatsAPI provides JSON API for dashboard stats
func (ah *AdminHandler) HandleStatsAPI(w http.ResponseWriter, r *http.Request) {
	stats := ah.adminService.GetStats()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Error encoding stats JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleActivityAPI provides JSON API for activity logs
func (ah *AdminHandler) HandleActivityAPI(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default

	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	activity := ah.adminService.GetRecentActivity(limit)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(activity); err != nil {
		log.Printf("Error encoding activity JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandlePerformanceAPI provides JSON API for performance metrics
func (ah *AdminHandler) HandlePerformanceAPI(w http.ResponseWriter, r *http.Request) {
	if ah.performanceService == nil {
		http.Error(w, "Performance service not available", http.StatusServiceUnavailable)
		return
	}

	metrics := ah.performanceService.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Printf("Error encoding performance metrics JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleStreamingAPI provides JSON API for streaming metrics
func (ah *AdminHandler) HandleStreamingAPI(w http.ResponseWriter, r *http.Request) {
	streamingMetrics := ah.adminService.GetStreamingMetrics()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(streamingMetrics); err != nil {
		log.Printf("Error encoding streaming metrics JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleCacheAPI provides JSON API for cache statistics
func (ah *AdminHandler) HandleCacheAPI(w http.ResponseWriter, r *http.Request) {
	if ah.cacheService == nil {
		http.Error(w, "Cache service not available", http.StatusServiceUnavailable)
		return
	}

	cacheStats := ah.cacheService.GetStats()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cacheStats); err != nil {
		log.Printf("Error encoding cache stats JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleWorkerPoolsAPI provides JSON API for worker pool metrics
func (ah *AdminHandler) HandleWorkerPoolsAPI(w http.ResponseWriter, r *http.Request) {
	if ah.performanceService == nil {
		http.Error(w, "Performance service not available", http.StatusServiceUnavailable)
		return
	}

	metrics := ah.performanceService.GetMetrics()
	workerPools := metrics.WorkerPools

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(workerPools); err != nil {
		log.Printf("Error encoding worker pools JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleRealtimeSSE handles Server-Sent Events for real-time admin dashboard updates
func (ah *AdminHandler) HandleRealtimeSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Create a unique client ID
	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())

	// Create a channel for this client
	clientChan := make(chan []byte, 10)

	// Register the client
	ah.sseClientsMutex.Lock()
	ah.sseClients[clientID] = clientChan
	ah.sseClientsMutex.Unlock()

	// Clean up when client disconnects
	defer func() {
		ah.sseClientsMutex.Lock()
		delete(ah.sseClients, clientID)
		close(clientChan)
		ah.sseClientsMutex.Unlock()
		log.Printf("SSE client %s disconnected", clientID)
	}()

	log.Printf("SSE client %s connected", clientID)

	// Send initial data
	ah.sendInitialData(w)

	// Listen for client disconnect and data updates
	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-clientChan:
			// Send data to client
			fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

// sendInitialData sends the initial dashboard data to a new SSE client
func (ah *AdminHandler) sendInitialData(w http.ResponseWriter) {
	stats := ah.adminService.GetStats()
	data, err := json.Marshal(map[string]interface{}{
		"type": "initial",
		"data": stats,
	})
	if err != nil {
		log.Printf("Error marshaling initial data: %v", err)
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", data)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// StartRealtimeBroadcast starts broadcasting real-time updates to all connected SSE clients
func (ah *AdminHandler) StartRealtimeBroadcast() {
	ticker := time.NewTicker(1 * time.Second) // Real-time updates every second
	defer ticker.Stop()

	log.Println("Started real-time admin dashboard broadcasting")

	// Cleanup ticker for removing stale clients
	cleanupTicker := time.NewTicker(30 * time.Second)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ticker.C:
			ah.broadcastUpdates()
		case <-cleanupTicker.C:
			ah.cleanupStaleClients()
		}
	}
}

// cleanupStaleClients removes clients that haven't been active
func (ah *AdminHandler) cleanupStaleClients() {
	ah.sseClientsMutex.Lock()
	defer ah.sseClientsMutex.Unlock()

	staleClients := make([]string, 0)

	for clientID, clientChan := range ah.sseClients {
		// Check if channel is closed or full (indicating stale client)
		select {
		case clientChan <- []byte("ping"):
			// Client is responsive
		default:
			// Client channel is full or closed
			staleClients = append(staleClients, clientID)
		}
	}

	// Remove stale clients
	for _, clientID := range staleClients {
		if clientChan, exists := ah.sseClients[clientID]; exists {
			close(clientChan)
			delete(ah.sseClients, clientID)
			log.Printf("Removed stale SSE client: %s", clientID)
		}
	}

	// Force garbage collection if many clients were removed
	if len(staleClients) > 5 {
		go func() {
			runtime.GC()
		}()
	}
}

// broadcastUpdates sends current stats to all connected SSE clients
func (ah *AdminHandler) broadcastUpdates() {
	ah.sseClientsMutex.RLock()
	if len(ah.sseClients) == 0 {
		ah.sseClientsMutex.RUnlock()
		return
	}
	ah.sseClientsMutex.RUnlock()

	// Get current stats
	stats := ah.adminService.GetStats()
	connections := ah.adminService.GetActiveConnections()

	// Prepare update data
	updateData := map[string]interface{}{
		"type":        "update",
		"timestamp":   time.Now().Unix(),
		"stats":       stats,
		"connections": connections,
	}

	data, err := json.Marshal(updateData)
	if err != nil {
		log.Printf("Error marshaling broadcast data: %v", err)
		return
	}

	// Send to all connected clients
	ah.sseClientsMutex.RLock()
	for clientID, clientChan := range ah.sseClients {
		select {
		case clientChan <- data:
			// Successfully sent
		default:
			// Channel is full, skip this client
			log.Printf("SSE client %s channel full, skipping update", clientID)
		}
	}
	ah.sseClientsMutex.RUnlock()
}

// GetConnectedClientsCount returns the number of connected SSE clients
func (ah *AdminHandler) GetConnectedClientsCount() int {
	ah.sseClientsMutex.RLock()
	defer ah.sseClientsMutex.RUnlock()
	return len(ah.sseClients)
}
