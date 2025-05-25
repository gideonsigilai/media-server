package handlers

import (
	"media-server/config"
	"media-server/middleware"
	"media-server/services"
	"net/http"
)

// SetupRoutes configures all application routes
func SetupRoutes(mux *http.ServeMux, cfg *config.Config, adminService *services.AdminService,
	cacheService *services.CacheService, performanceService *services.PerformanceService) {
	// Create handlers with enhanced services
	fileHandler := NewFileHandlerWithServices(cfg, cacheService, performanceService)
	streamHandler := NewStreamHandlerWithServices(cfg, adminService, cacheService, performanceService)
	playerHandler := NewPlayerHandlerWithServices(cfg, cacheService, performanceService)
	adminHandler := NewAdminHandlerWithServices(cfg, adminService, cacheService, performanceService)

	// Create admin middleware
	adminMiddleware := middleware.NewAdminMiddleware(adminService)

	// Static file serving
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	mux.Handle("/static/", staticHandler)

	// Admin dashboard routes (protected by admin auth)
	mux.Handle("/admin/dashboard", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleAdminDashboard)))
	mux.Handle("/admin/add-user", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleAddAdminUser)))
	mux.Handle("/admin/remove-user", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleRemoveAdminUser)))
	mux.Handle("/admin/block-ip", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleBlockIP)))
	mux.Handle("/admin/unblock-ip", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleUnblockIP)))
	mux.Handle("/admin/set-media-password", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleSetMediaPassword)))
	mux.Handle("/admin/remove-media-password", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleRemoveMediaPassword)))

	// Admin API routes (protected by admin auth)
	mux.Handle("/admin/api/connections", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleConnectionsAPI)))
	mux.Handle("/admin/api/stats", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleStatsAPI)))
	mux.Handle("/admin/api/activity", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleActivityAPI)))
	mux.Handle("/admin/api/performance", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandlePerformanceAPI)))
	mux.Handle("/admin/api/streaming", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleStreamingAPI)))
	mux.Handle("/admin/api/cache", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleCacheAPI)))
	mux.Handle("/admin/api/worker-pools", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleWorkerPoolsAPI)))
	mux.Handle("/admin/api/realtime", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleRealtimeSSE)))

	// Admin/Settings interface (protected by admin auth)
	mux.Handle("/settings", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleSettings)))
	mux.Handle("/upload", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleUpload)))
	mux.Handle("/browse-folder", adminMiddleware.AdminAuth(http.HandlerFunc(adminHandler.HandleBrowseFolder)))

	// Media library interface (with connection tracking and media password protection)
	mux.Handle("/library", adminMiddleware.ConnectionTracking(http.HandlerFunc(playerHandler.HandleLibrary)))

	// Video player interface (with connection tracking and media password protection)
	mux.Handle("/player/", adminMiddleware.MediaPasswordAuth(adminMiddleware.ConnectionTracking(http.HandlerFunc(playerHandler.HandlePlayer))))

	// File streaming (with connection tracking and media password protection)
	mux.Handle("/stream/", adminMiddleware.MediaPasswordAuth(adminMiddleware.ConnectionTracking(http.HandlerFunc(streamHandler.HandleStream))))

	// File listing (with connection tracking)
	mux.Handle("/", adminMiddleware.ConnectionTracking(http.HandlerFunc(fileHandler.HandleFileList)))
}
