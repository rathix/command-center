package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"path"
)

// SPAHandler serves static files from an embed.FS and falls back to index.html
// for any extensionless path that doesn't match a static file, enabling SPA
// client-side routing while returning 404 for missing files with extensions.
type SPAHandler struct {
	fileServer http.Handler
	filesystem fs.FS
}

// NewSPAHandler creates a handler that serves files from the given embed.FS,
// stripping the specified prefix from paths. When a requested file is not found,
// it serves index.html for client-side routing (extensionless paths only).
func NewSPAHandler(embedded fs.FS, prefix string) (*SPAHandler, error) {
	sub, err := fs.Sub(embedded, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create sub filesystem: %w", err)
	}
	return &SPAHandler{
		fileServer: http.FileServer(http.FS(sub)),
		filesystem: sub,
	}, nil
}

func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	if urlPath == "/" {
		h.fileServer.ServeHTTP(w, r)
		return
	}

	// Check if the file exists using fs.Stat (avoids opening file content)
	filePath := urlPath[1:]
	if _, err := fs.Stat(h.filesystem, filePath); err == nil {
		h.fileServer.ServeHTTP(w, r)
		return
	}

	// File not found — only serve SPA fallback for extensionless paths.
	// Paths with extensions (e.g., .css, .js, .png) are real file requests
	// and should return 404 to avoid MIME-type mismatches.
	// Note: r.URL.Path is already URL-decoded by Go's HTTP server, so
	// URL-encoded extensions (e.g., %2Ecss) are correctly detected.
	if path.Ext(urlPath) != "" {
		http.NotFound(w, r)
		return
	}

	// SPA fallback — serve index.html for client-side routing
	r.URL.Path = "/"
	h.fileServer.ServeHTTP(w, r)
}
