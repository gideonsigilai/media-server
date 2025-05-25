package main

import (
	"context"
	"log"
	"media-server/config"
	"media-server/handlers"
	"media-server/middleware"
	"media-server/services"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func main() {
	// Configure CPU and runtime settings for maximum performance
	log.Println("Configuring runtime for optimal performance...")
	configureRuntime()

	// Load configuration
	log.Println("Loading configuration...")
	cfg := config.Load()
	log.Printf("Configuration loaded: MediaDir=%s, Port=%d", cfg.MediaDir, cfg.Port)

	// Initialize performance service
	log.Println("Initializing performance service...")
	performanceService := services.NewPerformanceService()

	// Initialize cache service
	log.Println("Initializing cache service...")
	cacheService := services.NewCacheService()

	// Initialize admin service with performance monitoring
	log.Println("Initializing admin service...")
	adminService := services.NewAdminService()
	adminService.SetPerformanceService(performanceService)
	adminService.SetCacheService(cacheService)

	// Setup middleware
	mux := http.NewServeMux()

	// Setup routes with enhanced services
	log.Println("Setting up routes...")
	handlers.SetupRoutes(mux, cfg, adminService, cacheService, performanceService)

	// Apply middleware (logging and security)
	handler := middleware.Logging(middleware.Security(mux))

	// Create HTTP server with optimized settings
	server := &http.Server{
		Addr:         cfg.GetAddress(),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start performance monitoring
	go performanceService.StartMonitoring()

	// Start cache cleanup routine
	go cacheService.StartCleanup()

	// Start real-time admin dashboard broadcasting
	go func() {
		// Get the admin handler from routes to start broadcasting
		// We need to create a temporary admin handler to start broadcasting
		tempAdminHandler := handlers.NewAdminHandlerWithServices(cfg, adminService, cacheService, performanceService)
		tempAdminHandler.StartRealtimeBroadcast()
	}()

	// Start the server in a goroutine
	go func() {
		log.Printf("Media server starting on http://localhost:%d", cfg.Port)
		log.Printf("Serving media from: %s", cfg.MediaDir)
		log.Printf("Admin dashboard available at: http://localhost:%d/admin/dashboard", cfg.Port)
		log.Printf("CPU cores detected: %d, GOMAXPROCS: %d", runtime.NumCPU(), runtime.GOMAXPROCS(0))
		log.Println("Note: Admin dashboard is accessible from localhost or authorized IPs only")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown services gracefully
	performanceService.Stop()
	cacheService.Stop()

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// configureRuntime optimizes Go runtime settings for maximum performance
func configureRuntime() {
	numCPU := runtime.NumCPU()
	log.Printf("Detected %d CPU cores", numCPU)

	// Set GOMAXPROCS to use all available CPU cores
	runtime.GOMAXPROCS(numCPU)
	log.Printf("Set GOMAXPROCS to %d", numCPU)

	// Set garbage collection target percentage for better performance
	// Lower values mean more frequent GC but lower memory usage
	// Higher values mean less frequent GC but higher memory usage
	// 100 is the default, we'll use 200 for better performance with media streaming
	debug := os.Getenv("GOGC")
	if debug == "" {
		os.Setenv("GOGC", "200")
		log.Println("Set GOGC to 200 for optimized garbage collection")
	}
}
