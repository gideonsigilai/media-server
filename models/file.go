package models

import (
	"os"
	"path/filepath"
	"strings"
)

// FileInfo represents information about a file or directory
type FileInfo struct {
	Name      string `json:"name"`
	IsDir     bool   `json:"is_dir"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
	IsMedia   bool   `json:"is_media"`
}

// NewFileInfo creates a new FileInfo from an os.DirEntry
func NewFileInfo(entry os.DirEntry, basePath string) (*FileInfo, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}

	extension := filepath.Ext(entry.Name())
	isMedia := IsMediaFile(extension)

	return &FileInfo{
		Name:      entry.Name(),
		IsDir:     entry.IsDir(),
		Path:      filepath.Join(basePath, entry.Name()),
		Size:      info.Size(),
		Extension: extension,
		IsMedia:   isMedia,
	}, nil
}

// IsMediaFile checks if a file extension represents a media file
func IsMediaFile(extension string) bool {
	extension = strings.ToLower(extension)
	mediaExtensions := map[string]bool{
		// Video formats
		".mp4":  true,
		".mkv":  true,
		".avi":  true,
		".mov":  true,
		".wmv":  true,
		".flv":  true,
		".webm": true,
		".m4v":  true,
		".3gp":  true,
		".ts":   true,
		".mts":  true,
		".m2ts": true,
		// Audio formats
		".mp3":  true,
		".wav":  true,
		".aac":  true,
		".ogg":  true,
		".flac": true,
		".m4a":  true,
		".wma":  true,
		".opus": true,
		".aiff": true,
		// Image formats
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".bmp":  true,
		".webp": true,
		".svg":  true,
		".tiff": true,
		".ico":  true,
	}
	return mediaExtensions[extension]
}

// GetMediaType returns the general media type (video, audio, image)
func (f *FileInfo) GetMediaType() string {
	if !f.IsMedia {
		return "file"
	}

	ext := strings.ToLower(f.Extension)

	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
		".3gp": true, ".ts": true, ".mts": true, ".m2ts": true,
	}

	audioExts := map[string]bool{
		".mp3": true, ".wav": true, ".aac": true, ".ogg": true,
		".flac": true, ".m4a": true, ".wma": true, ".opus": true,
		".aiff": true,
	}

	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".webp": true, ".svg": true, ".tiff": true,
		".ico": true,
	}

	if videoExts[ext] {
		return "video"
	}
	if audioExts[ext] {
		return "audio"
	}
	if imageExts[ext] {
		return "image"
	}

	return "file"
}

// GetIcon returns an appropriate icon for the file type
func (f *FileInfo) GetIcon() string {
	if f.IsDir {
		return "📁"
	}

	switch f.GetMediaType() {
	case "video":
		return "🎬"
	case "audio":
		return "🎵"
	case "image":
		return "🖼️"
	default:
		return "📄"
	}
}
