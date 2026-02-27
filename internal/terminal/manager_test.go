package terminal

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/websocket"
	ws "nhooyr.io/websocket"
)

func setupManagerTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, "ws" + srv.URL[4:]
}

func TestManager_MaxSessions(t *testing.T) {
	registry := websocket.NewRegistry(slog.Default())
	mgr := NewManager(registry,
		WithMaxSessions(2),
		WithAllowedCommands([]string{"sleep"}),
		WithLogger(slog.Default()),
	)

	ctx := context.Background()

	// Create a mock WS connection pair to test session creation
	serverDone := make(chan error, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			serverDone <- err
			return
		}
		_, err = mgr.CreateSession(ctx, c, "sleep 60")
		serverDone <- err
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]

	// Create 2 sessions (should succeed)
	for i := 0; i < 2; i++ {
		c, _, err := ws.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		defer c.CloseNow()

		select {
		case err := <-serverDone:
			if err != nil {
				t.Fatalf("session %d creation failed: %v", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("session %d: timed out", i)
		}
	}

	// Small delay to ensure sessions are registered
	time.Sleep(50 * time.Millisecond)

	if mgr.SessionCount() != 2 {
		t.Fatalf("expected 2 sessions, got %d", mgr.SessionCount())
	}

	// 3rd session should fail
	c, _, err := ws.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial 3: %v", err)
	}
	defer c.CloseNow()

	select {
	case err := <-serverDone:
		if err == nil {
			t.Fatal("expected error for 3rd session, got nil")
		}
		if !strings.Contains(err.Error(), "maximum concurrent sessions reached") {
			t.Fatalf("expected max sessions error, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for 3rd session creation")
	}

	// Cleanup
	mgr.Shutdown(ctx)
}

func TestManager_IdleTimeout(t *testing.T) {
	registry := websocket.NewRegistry(slog.Default())
	mgr := NewManager(registry,
		WithMaxSessions(10),
		WithAllowedCommands([]string{"sleep"}),
		WithIdleTimeout(200*time.Millisecond),
		WithIdleScanInterval(50*time.Millisecond),
		WithLogger(slog.Default()),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessionCreated := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			return
		}
		_, err = mgr.CreateSession(ctx, c, "sleep 60")
		if err != nil {
			return
		}
		close(sessionCreated)
		// Block until context is done
		<-ctx.Done()
	}))
	defer srv.Close()

	c, _, err := ws.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	select {
	case <-sessionCreated:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for session creation")
	}

	// Start idle scanner
	go mgr.RunIdleScanner(ctx)

	// Wait for idle timeout to kick in
	time.Sleep(500 * time.Millisecond)

	if mgr.SessionCount() != 0 {
		t.Fatalf("expected 0 sessions after idle timeout, got %d", mgr.SessionCount())
	}
}

func TestManager_IdleTimeout_ActivityResetsTimer(t *testing.T) {
	registry := websocket.NewRegistry(slog.Default())
	mgr := NewManager(registry,
		WithMaxSessions(10),
		WithAllowedCommands([]string{"sleep"}),
		WithIdleTimeout(300*time.Millisecond),
		WithIdleScanInterval(50*time.Millisecond),
		WithLogger(slog.Default()),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessionCreated := make(chan string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			return
		}
		id, err := mgr.CreateSession(ctx, c, "sleep 60")
		if err != nil {
			return
		}
		sessionCreated <- id
		<-ctx.Done()
	}))
	defer srv.Close()

	c, _, err := ws.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	var sessionID string
	select {
	case sessionID = <-sessionCreated:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for session creation")
	}

	// Start idle scanner
	go mgr.RunIdleScanner(ctx)

	// Keep touching activity to prevent idle timeout
	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond)
		mgr.mu.Lock()
		if s, ok := mgr.sessions[sessionID]; ok {
			s.touchActivity()
		}
		mgr.mu.Unlock()
	}

	// Session should still be alive
	if mgr.SessionCount() != 1 {
		t.Fatalf("expected 1 session (activity should prevent idle timeout), got %d", mgr.SessionCount())
	}

	// Now stop touching — let idle timeout kick in
	time.Sleep(500 * time.Millisecond)

	if mgr.SessionCount() != 0 {
		t.Fatalf("expected 0 sessions after stopping activity, got %d", mgr.SessionCount())
	}
}

func TestManager_SessionCountDecrementOnClose(t *testing.T) {
	registry := websocket.NewRegistry(slog.Default())
	mgr := NewManager(registry,
		WithMaxSessions(4),
		WithAllowedCommands([]string{"sleep"}),
		WithLogger(slog.Default()),
	)

	ctx := context.Background()
	sessionCreated := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			return
		}
		id, err := mgr.CreateSession(ctx, c, "sleep 60")
		if err != nil {
			return
		}
		sessionCreated <- id
	}))
	defer srv.Close()

	c, _, err := ws.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	var id string
	select {
	case id = <-sessionCreated:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	if mgr.SessionCount() != 1 {
		t.Fatalf("expected 1, got %d", mgr.SessionCount())
	}

	mgr.RemoveSession(id)
	time.Sleep(100 * time.Millisecond)

	if mgr.SessionCount() != 0 {
		t.Fatalf("expected 0 after close, got %d", mgr.SessionCount())
	}
}

func TestManager_SessionCount(t *testing.T) {
	registry := websocket.NewRegistry(slog.Default())
	mgr := NewManager(registry,
		WithMaxSessions(10),
		WithAllowedCommands([]string{"echo"}),
	)
	if mgr.SessionCount() != 0 {
		t.Fatalf("expected 0 sessions, got %d", mgr.SessionCount())
	}
}

func TestManager_Shutdown(t *testing.T) {
	registry := websocket.NewRegistry(slog.Default())
	mgr := NewManager(registry,
		WithMaxSessions(10),
		WithAllowedCommands([]string{"sleep"}),
	)

	ctx := context.Background()
	sessionCreated := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			return
		}
		_, err = mgr.CreateSession(ctx, c, "sleep 60")
		if err != nil {
			return
		}
		close(sessionCreated)
		// Block until session ends
		time.Sleep(60 * time.Second)
	}))
	defer srv.Close()

	c, _, err := ws.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	select {
	case <-sessionCreated:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for session creation")
	}

	if mgr.SessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", mgr.SessionCount())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mgr.Shutdown(shutdownCtx)

	if mgr.SessionCount() != 0 {
		t.Fatalf("expected 0 sessions after shutdown, got %d", mgr.SessionCount())
	}
}
