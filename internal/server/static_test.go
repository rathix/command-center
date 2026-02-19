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
