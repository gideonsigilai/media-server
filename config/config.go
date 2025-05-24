package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds the application configuration
type Config struct {
	MediaDir string
	Port     int
}

// Load loads configuration from environment variables with sensible defaults
func Load() *Config {
	cfg := &Config{
		MediaDir: "./media",
		Port:     8080,
	}

	// Override media directory from environment variable
	if envDir := os.Getenv("MEDIA_DIR"); envDir != "" {
		cfg.MediaDir = envDir
	}

	// Override port from environment variable
	if envPort := os.Getenv("PORT"); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil {
			cfg.Port = port
		} else {
			log.Printf("Invalid PORT value: %s, using default: %d", envPort, cfg.Port)
		}
	}

	// Ensure media directory exists
	if err := cfg.ensureMediaDir(); err != nil {
		log.Fatalf("Failed to setup media directory: %v", err)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(cfg.MediaDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path for media directory: %v", err)
	}
	cfg.MediaDir = absPath

	return cfg
}

// GetAddress returns the server address string
func (c *Config) GetAddress() string {
	return fmt.Sprintf(":%d", c.Port)
}

// ensureMediaDir creates the media directory if it doesn't exist
func (c *Config) ensureMediaDir() error {
	if _, err := os.Stat(c.MediaDir); os.IsNotExist(err) {
		log.Printf("Media directory %s does not exist, creating it...", c.MediaDir)
		return os.MkdirAll(c.MediaDir, 0755)
	}
	return nil
}
