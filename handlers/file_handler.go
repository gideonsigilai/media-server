package handlers

import (
	"fmt"
	"html/template"
	"log"
	"media-server/config"
	"media-server/models"
	"media-server/services"
	"media-server/utils"
	"net/http"
	"strconv"
	"strings"
)

// FileHandler handles file listing and navigation
type FileHandler struct {
	fileService        *services.FileService
	templates          *template.Template
	cacheService       *services.CacheService
	performanceService *services.PerformanceService
}

// NewFileHandler creates a new FileHandler instance
func NewFileHandler(cfg *config.Config) *FileHandler {
	log.Println("Creating FileHandler...")
	fileService := services.NewFileService(cfg.MediaDir)

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

	log.Println("Loading templates from views/templates/*.html")
	templates, err := template.New("").Funcs(funcMap).ParseGlob("views/templates/*.html")
	if err != nil {
		log.Fatalf("Error loading templates: %v", err)
	}
	log.Println("Templates loaded successfully")

	return &FileHandler{
		fileService: fileService,
		templates:   templates,
	}
}

// NewFileHandlerWithServices creates a new FileHandler instance with enhanced services
func NewFileHandlerWithServices(cfg *config.Config, cacheService *services.CacheService,
	performanceService *services.PerformanceService, mediaFolderService *services.MediaFolderService) *FileHandler {

	log.Println("Creating FileHandler with enhanced services...")
	fileService := services.NewFileServiceWithMediaFolders(cfg.MediaDir, cacheService, performanceService, mediaFolderService)

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

	log.Println("Loading templates from views/templates/*.html")
	templates, err := template.New("").Funcs(funcMap).ParseGlob("views/templates/*.html")
	if err != nil {
		log.Fatalf("Error loading templates: %v", err)
	}
	log.Println("Templates loaded successfully")

	return &FileHandler{
		fileService:        fileService,
		templates:          templates,
		cacheService:       cacheService,
		performanceService: performanceService,
	}
}

// HandleFileList handles directory listing requests
func (fh *FileHandler) HandleFileList(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Get file info to determine if it's a file or directory
	fileInfo, err := fh.fileService.GetFileInfo(path)
	if err != nil {
		fh.handleError(w, r, "File Not Found", err.Error(), http.StatusNotFound)
		return
	}

	// If it's a file, redirect to appropriate handler
	if !fileInfo.IsDir {
		if fileInfo.IsMedia {
			// Redirect media files to the player
			http.Redirect(w, r, "/player/"+path, http.StatusSeeOther)
		} else {
			// Redirect non-media files to direct streaming
			http.Redirect(w, r, "/stream/"+path, http.StatusSeeOther)
		}
		return
	}

	// List directory contents
	files, err := fh.fileService.ListDirectory(path)
	if err != nil {
		fh.handleError(w, r, "Access Denied", err.Error(), http.StatusForbidden)
		return
	}

	// Prepare template data
	data := struct {
		Title       string
		CurrentPath string
		ParentPath  string
		Files       []*models.FileInfo
	}{
		Title:       fh.getPageTitle(path),
		CurrentPath: path,
		ParentPath:  utils.GetParentPath(path),
		Files:       files,
	}

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := fh.templates.ExecuteTemplate(w, "file_list.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleError renders an error page
func (fh *FileHandler) handleError(w http.ResponseWriter, r *http.Request, title, message string, statusCode int) {
	data := struct {
		Title        string
		ErrorTitle   string
		ErrorMessage string
	}{
		Title:        "Error - Media Server",
		ErrorTitle:   title,
		ErrorMessage: message,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)

	if err := fh.templates.ExecuteTemplate(w, "error.html", data); err != nil {
		log.Printf("Error executing error template: %v", err)
		http.Error(w, message, statusCode)
	}
}

// getPageTitle generates an appropriate page title
func (fh *FileHandler) getPageTitle(path string) string {
	if path == "" {
		return "Media Server"
	}

	// Get the last part of the path for the title
	parts := utils.SplitPath(path)
	if len(parts) > 0 {
		return "Media Server - " + parts[len(parts)-1]
	}

	return "Media Server - " + path
}
