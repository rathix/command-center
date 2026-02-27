package server

import (
	"net/http"
	"strings"
)

// NormalizeBasePath ensures the base path starts and ends with '/'.
func NormalizeBasePath(basePath string) string {
	if basePath == "" {
		return "/"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if !strings.HasSuffix(basePath, "/") {
		basePath = basePath + "/"
	}
	return basePath
}

// BasePathHandler wraps an http.Handler and strips the base path prefix
// from incoming requests, enabling dual access (direct + reverse proxy).
// Requests without the prefix are forwarded unchanged (direct access).
type BasePathHandler struct {
	basePath string
	inner    http.Handler
}

// NewBasePathHandler creates a handler that strips basePath from request URLs
// before forwarding to the inner handler. If basePath is "/", it returns
// the inner handler directly (no-op wrapper).
func NewBasePathHandler(basePath string, inner http.Handler) http.Handler {
	bp := NormalizeBasePath(basePath)
	if bp == "/" {
		return inner
	}
	return &BasePathHandler{
		basePath: bp,
		inner:    inner,
	}
}

func (h *BasePathHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If request matches the base path, strip it
	if strings.HasPrefix(r.URL.Path, h.basePath) {
		stripped := "/" + strings.TrimPrefix(r.URL.Path, h.basePath)
		r2 := r.Clone(r.Context())
		r2.URL.Path = stripped
		r2.URL.RawPath = ""
		h.inner.ServeHTTP(w, r2)
		return
	}
	// Exact base path without trailing content
	if r.URL.Path+"/" == h.basePath {
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		r2.URL.RawPath = ""
		h.inner.ServeHTTP(w, r2)
		return
	}

	// Direct access — forward as-is
	h.inner.ServeHTTP(w, r)
}
