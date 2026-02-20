package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func startLocalHTTPServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping network-bound test: cannot bind loopback socket: %v", err)
	}
	srv := httptest.NewUnstartedServer(h)
	srv.Listener = ln
	srv.Start()
	return srv
}

func TestDevProxyHandlerForwardsToTarget(t *testing.T) {
	// Mock Vite dev server
	vite := startLocalHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	vite := startLocalHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
