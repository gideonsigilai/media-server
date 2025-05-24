package handlers

import (
	"media-server/config"
	"net/http"
)

// SetupRoutes configures all application routes
func SetupRoutes(mux *http.ServeMux, cfg *config.Config) {
	// Create handlers
	fileHandler := NewFileHandler(cfg)
	streamHandler := NewStreamHandler(cfg)
	playerHandler := NewPlayerHandler(cfg)
	adminHandler := NewAdminHandler(cfg)

	// Static file serving
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	mux.Handle("/static/", staticHandler)

	// Admin/Settings interface
	mux.HandleFunc("/settings", adminHandler.HandleSettings)
	mux.HandleFunc("/upload", adminHandler.HandleUpload)
	mux.HandleFunc("/browse-folder", adminHandler.HandleBrowseFolder)

	// Media library interface
	mux.HandleFunc("/library", playerHandler.HandleLibrary)

	// Video player interface
	mux.HandleFunc("/player/", playerHandler.HandlePlayer)

	// File streaming
	mux.HandleFunc("/stream/", streamHandler.HandleStream)

	// File listing (catch-all for directory navigation)
	mux.HandleFunc("/", fileHandler.HandleFileList)
}
