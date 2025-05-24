package main

import (
	"log"
	"media-server/config"
	"media-server/handlers"
	"media-server/middleware"
	"net/http"
)

func main() {
	// Load configuration
	log.Println("Loading configuration...")
	cfg := config.Load()
	log.Printf("Configuration loaded: MediaDir=%s, Port=%d", cfg.MediaDir, cfg.Port)

	// Setup middleware
	mux := http.NewServeMux()

	// Setup routes
	log.Println("Setting up routes...")
	handlers.SetupRoutes(mux, cfg)

	// Apply middleware
	handler := middleware.Logging(middleware.Security(mux))

	// Start the server
	log.Printf("Media server starting on http://localhost:%d", cfg.Port)
	log.Printf("Serving media from: %s", cfg.MediaDir)
	log.Fatal(http.ListenAndServe(cfg.GetAddress(), handler))
}
