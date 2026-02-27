package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/config"
	"github.com/rathix/command-center/internal/state"
)

var _ Adapter = (*WebhookAdapter)(nil)

func TestWebhookAdapter_Send(t *testing.T) {
	var receivedBody []byte
	var receivedContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	adapter := NewWebhookAdapter("test", srv.URL, WithHTTPClient(srv.Client()))

	now := time.Now()
	n := Notification{
		ServiceName: "api-gateway",
		Namespace:   "default",
		PrevState:   state.StatusHealthy,
		NewState:    state.StatusUnhealthy,
		Timestamp:   now,
		Signals:     []string{"http:unhealthy"},
	}

	err := adapter.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", receivedContentType)
	}

	var decoded Notification
	if err := json.Unmarshal(receivedBody, &decoded); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if decoded.ServiceName != "api-gateway" {
		t.Errorf("expected service name 'api-gateway', got %q", decoded.ServiceName)
	}
	if decoded.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", decoded.Namespace)
	}
	if decoded.NewState != state.StatusUnhealthy {
		t.Errorf("expected new state unhealthy, got %v", decoded.NewState)
	}
}

func TestWebhookAdapter_ErrorOnBadResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	adapter := NewWebhookAdapter("test", srv.URL, WithHTTPClient(srv.Client()))

	err := adapter.Send(context.Background(), Notification{ServiceName: "test"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestWebhookAdapter_Name(t *testing.T) {
	adapter := NewWebhookAdapter("my-hook", "http://example.com")
	if adapter.Name() != "my-hook" {
		t.Errorf("expected name 'my-hook', got %q", adapter.Name())
	}
}

func TestBuildAdapters_Webhook(t *testing.T) {
	adapters, err := BuildAdapters([]config.AdapterConfig{
		{Type: "webhook", Name: "my-webhook", URL: "http://example.com/hook"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(adapters) != 1 {
		t.Fatalf("expected 1 adapter, got %d", len(adapters))
	}
	if _, ok := adapters["my-webhook"]; !ok {
		t.Error("expected adapter 'my-webhook'")
	}
}

func TestBuildAdapters_UnknownType(t *testing.T) {
	_, err := BuildAdapters([]config.AdapterConfig{
		{Type: "unknown", Name: "bad"},
	})
	if err == nil {
		t.Fatal("expected error for unknown adapter type")
	}
}

func TestBuildAdapters_Empty(t *testing.T) {
	adapters, err := BuildAdapters(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(adapters) != 0 {
		t.Fatalf("expected 0 adapters, got %d", len(adapters))
	}
}
