package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "/"},
		{"/", "/"},
		{"command-center", "/command-center/"},
		{"/command-center", "/command-center/"},
		{"/command-center/", "/command-center/"},
		{"command-center/", "/command-center/"},
	}

	for _, tc := range tests {
		got := NormalizeBasePath(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeBasePath(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestBasePathHandlerDirectAccess(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	})

	handler := NewBasePathHandler("/command-center/", inner)

	// Direct access to /api/events should pass through unchanged
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "/api/events" {
		t.Errorf("direct access: expected path /api/events, got %q", rec.Body.String())
	}
}

func TestBasePathHandlerProxiedAccess(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	})

	handler := NewBasePathHandler("/command-center/", inner)

	// Proxied access to /command-center/api/events should strip prefix
	req := httptest.NewRequest(http.MethodGet, "/command-center/api/events", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "/api/events" {
		t.Errorf("proxied access: expected path /api/events, got %q", rec.Body.String())
	}
}

func TestBasePathHandlerProxiedRoot(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	})

	handler := NewBasePathHandler("/command-center/", inner)

	// /command-center/ -> /
	req := httptest.NewRequest(http.MethodGet, "/command-center/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "/" {
		t.Errorf("proxied root: expected path /, got %q", rec.Body.String())
	}
}

func TestBasePathHandlerProxiedRootWithoutTrailingSlash(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	})

	handler := NewBasePathHandler("/command-center/", inner)

	// /command-center -> /
	req := httptest.NewRequest(http.MethodGet, "/command-center", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "/" {
		t.Errorf("proxied root without slash: expected path /, got %q", rec.Body.String())
	}
}

func TestBasePathHandlerStaticFile(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	})

	handler := NewBasePathHandler("/command-center/", inner)

	// /command-center/index.html -> /index.html
	req := httptest.NewRequest(http.MethodGet, "/command-center/index.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "/index.html" {
		t.Errorf("static file: expected path /index.html, got %q", rec.Body.String())
	}
}

func TestBasePathHandlerDefaultIsNoOp(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	})

	handler := NewBasePathHandler("/", inner)

	// Should be a no-op pass-through
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "/api/events" {
		t.Errorf("no-op handler: expected path /api/events, got %q", rec.Body.String())
	}
}

func TestBasePathHandlerSPAFallback(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	})

	handler := NewBasePathHandler("/command-center/", inner)

	// /command-center/nonexistent-route -> /nonexistent-route (for SPA fallback)
	req := httptest.NewRequest(http.MethodGet, "/command-center/nonexistent-route", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "/nonexistent-route" {
		t.Errorf("SPA fallback: expected path /nonexistent-route, got %q", rec.Body.String())
	}
}
