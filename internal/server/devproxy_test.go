package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDevProxyHandlerForwardsToTarget(t *testing.T) {
	// Mock Vite dev server
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("vite:" + r.URL.Path))
	}))
	defer vite.Close()

	handler, err := NewDevProxyHandler(vite.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "vite:/" {
		t.Errorf("expected 'vite:/', got %q", rec.Body.String())
	}
}

func TestDevProxyHandlerPreservesPath(t *testing.T) {
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("vite:" + r.URL.Path))
	}))
	defer vite.Close()

	handler, err := NewDevProxyHandler(vite.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/settings", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "vite:/dashboard/settings" {
		t.Errorf("expected 'vite:/dashboard/settings', got %q", rec.Body.String())
	}
}

func TestDevProxyHandlerInvalidURL(t *testing.T) {
	_, err := NewDevProxyHandler("://invalid")
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}
