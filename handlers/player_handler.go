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

// PlayerHandler handles video player interface
type PlayerHandler struct {
	fileService        *services.FileService
	templates          *template.Template
	cacheService       *services.CacheService
	performanceService *services.PerformanceService
}

// NewPlayerHandler creates a new PlayerHandler instance
func NewPlayerHandler(cfg *config.Config) *PlayerHandler {
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

	templates, err := template.New("").Funcs(funcMap).ParseGlob("views/templates/*.html")
	if err != nil {
		log.Fatalf("Error loading templates: %v", err)
	}

	return &PlayerHandler{
		fileService: fileService,
		templates:   templates,
	}
}

// NewPlayerHandlerWithServices creates a new PlayerHandler instance with enhanced services
func NewPlayerHandlerWithServices(cfg *config.Config, cacheService *services.CacheService,
	performanceService *services.PerformanceService) *PlayerHandler {

	fileService := services.NewFileServiceWithCache(cfg.MediaDir, cacheService, performanceService)

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

	return &PlayerHandler{
		fileService:        fileService,
		templates:          templates,
		cacheService:       cacheService,
		performanceService: performanceService,
	}
}

// HandlePlayer handles video player requests
func (ph *PlayerHandler) HandlePlayer(w http.ResponseWriter, r *http.Request) {
	// Extract path from URL (remove /player/ prefix)
	path := strings.TrimPrefix(r.URL.Path, "/player/")

	// Get file info
	fileInfo, err := ph.fileService.GetFileInfo(path)
	if err != nil {
		ph.handleError(w, r, "File Not Found", err.Error(), http.StatusNotFound)
		return
	}

	// Ensure it's a media file
	if !fileInfo.IsMedia {
		ph.handleError(w, r, "Invalid File Type", "This file is not a supported media file", http.StatusBadRequest)
		return
	}

	// Get directory contents for playlist
	parentDir := utils.GetParentPath(path)
	files, err := ph.fileService.ListDirectory(parentDir)
	if err != nil {
		files = []*models.FileInfo{} // Empty playlist if can't read directory
	}

	// Filter only media files for playlist
	var playlist []*models.FileInfo
	for _, file := range files {
		if file.IsMedia && !file.IsDir {
			playlist = append(playlist, file)
		}
	}

	// Prepare template data
	data := struct {
		Title       string
		CurrentFile *models.FileInfo
		Playlist    []*models.FileInfo
		StreamURL   string
		ParentPath  string
	}{
		Title:       "Media Player - " + fileInfo.Name,
		CurrentFile: fileInfo,
		Playlist:    playlist,
		StreamURL:   "/stream/" + path,
		ParentPath:  parentDir,
	}

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ph.templates.ExecuteTemplate(w, "player.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleLibrary handles the video library interface
func (ph *PlayerHandler) HandleLibrary(w http.ResponseWriter, r *http.Request) {
	// Get all media files recursively
	allFiles, err := ph.getAllMediaFiles("")
	if err != nil {
		ph.handleError(w, r, "Error Loading Library", err.Error(), http.StatusInternalServerError)
		return
	}

	// Group files by type
	videos := []*models.FileInfo{}
	audios := []*models.FileInfo{}
	images := []*models.FileInfo{}

	for _, file := range allFiles {
		switch file.GetMediaType() {
		case "video":
			videos = append(videos, file)
		case "audio":
			audios = append(audios, file)
		case "image":
			images = append(images, file)
		}
	}

	// Prepare template data
	data := struct {
		Title  string
		Videos []*models.FileInfo
		Audios []*models.FileInfo
		Images []*models.FileInfo
	}{
		Title:  "Media Library",
		Videos: videos,
		Audios: audios,
		Images: images,
	}

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ph.templates.ExecuteTemplate(w, "library.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// getAllMediaFiles recursively gets all media files
func (ph *PlayerHandler) getAllMediaFiles(basePath string) ([]*models.FileInfo, error) {
	var allFiles []*models.FileInfo

	files, err := ph.fileService.ListDirectory(basePath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir {
			// Recursively get files from subdirectories
			subFiles, err := ph.getAllMediaFiles(file.Path)
			if err != nil {
				log.Printf("Error reading subdirectory %s: %v", file.Path, err)
				continue
			}
			allFiles = append(allFiles, subFiles...)
		} else if file.IsMedia {
			allFiles = append(allFiles, file)
		}
	}

	return allFiles, nil
}

// handleError renders an error page
func (ph *PlayerHandler) handleError(w http.ResponseWriter, r *http.Request, title, message string, statusCode int) {
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

	if err := ph.templates.ExecuteTemplate(w, "error.html", data); err != nil {
		log.Printf("Error executing error template: %v", err)
		http.Error(w, message, statusCode)
	}
}
