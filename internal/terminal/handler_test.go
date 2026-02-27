package terminal

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rathix/command-center/internal/websocket"
)

func TestHandler_DisabledReturns404(t *testing.T) {
	registry := websocket.NewRegistry(slog.Default())
	mgr := NewManager(registry)
	h := NewHandler(mgr, registry, false, slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/terminal", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "Terminal feature is not enabled" {
		t.Errorf("unexpected error message: %s", body["error"])
	}
}

func TestHandler_NoMTLSReturns403(t *testing.T) {
	registry := websocket.NewRegistry(slog.Default())
	mgr := NewManager(registry)
	h := NewHandler(mgr, registry, true, slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/terminal", nil)
	// No TLS info on the request
	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}
