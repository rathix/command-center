package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func newTestHandler() *SPAHandler {
	fsys := fstest.MapFS{
		"index.html":                   {Data: []byte("<html><body>SPA</body></html>")},
		"_app/immutable/chunks/app.js": {Data: []byte("console.log('app')")},
		"favicon.png":                  {Data: []byte("fakepng")},
	}
	return &SPAHandler{
		fileServer: http.FileServer(http.FS(fsys)),
		filesystem: fsys,
	}
}

func TestRootServesIndexHTML(t *testing.T) {
	handler := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SPA") {
		t.Errorf("expected body to contain 'SPA', got %q", rec.Body.String())
	}
}

func TestStaticFileServedDirectly(t *testing.T) {
	handler := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/favicon.png", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "fakepng" {
		t.Errorf("expected 'fakepng', got %q", rec.Body.String())
	}
}

func TestNestedStaticFileServed(t *testing.T) {
	handler := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/_app/immutable/chunks/app.js", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Errorf("expected JS content, got %q", rec.Body.String())
	}
}

func TestSPAFallbackForUnknownRoute(t *testing.T) {
	handler := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/settings", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SPA") {
		t.Errorf("expected SPA fallback (index.html), got %q", rec.Body.String())
	}
}

func TestSPAFallbackForDeepRoute(t *testing.T) {
	handler := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/services/my-service/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SPA") {
		t.Errorf("expected SPA fallback (index.html), got %q", rec.Body.String())
	}
}

func TestNoFallbackForMissingFileWithExtension(t *testing.T) {
	handler := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/missing.css", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for missing file with extension, got %d", rec.Code)
	}
}

func TestNoFallbackForMissingJSFile(t *testing.T) {
	handler := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/_app/missing-chunk.js", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for missing .js file, got %d", rec.Code)
	}
}

func TestNoFallbackForURLEncodedExtension(t *testing.T) {
	handler := newTestHandler()
	// %2E is URL-encoded "." — /missing%2Ecss should be treated as having .css extension
	req := httptest.NewRequest(http.MethodGet, "/missing%2Ecss", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for URL-encoded extension, got %d", rec.Code)
	}
}

func TestNewSPAHandlerReturnsErrorForInvalidPrefix(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": {Data: []byte("<html></html>")},
	}
	// fs.Sub only errors on invalid paths (e.g., ".." is not a valid fs path)
	_, err := NewSPAHandler(fsys, "..")
	if err == nil {
		t.Error("expected error for invalid prefix, got nil")
	}
}

func TestDirectoryPathFallsBackToSPA(t *testing.T) {
	handler := newTestHandler()
	// Directory paths (e.g., /_app/) are extensionless — should get SPA fallback
	req := httptest.NewRequest(http.MethodGet, "/_app/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SPA") {
		t.Errorf("expected SPA fallback for directory path, got %q", rec.Body.String())
	}
}

func TestNewSPAHandlerSucceedsForValidPrefix(t *testing.T) {
	fsys := fstest.MapFS{
		"web/build/index.html": {Data: []byte("<html></html>")},
	}
	handler, err := NewSPAHandler(fsys, "web/build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler == nil {
		t.Error("expected non-nil handler")
	}
}
