package handlers

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"media-server/config"
	"media-server/utils"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// AdminHandler handles admin functionality like uploads and settings
type AdminHandler struct {
	config    *config.Config
	templates *template.Template
}

// NewAdminHandler creates a new AdminHandler instance
func NewAdminHandler(cfg *config.Config) *AdminHandler {
	// Load templates with custom functions
	funcMap := template.FuncMap{
		"splitPath":      utils.SplitPath,
		"joinPath":       utils.JoinPath,
		"formatFileSize": utils.FormatFileSize,
		"trimPrefix":     strings.TrimPrefix,
	}

	templates, err := template.New("").Funcs(funcMap).ParseGlob("views/templates/*.html")
	if err != nil {
		log.Fatalf("Error loading templates: %v", err)
	}

	return &AdminHandler{
		config:    cfg,
		templates: templates,
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
