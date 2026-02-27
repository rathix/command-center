package terminal

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ws "nhooyr.io/websocket"
)

func TestSession_StartAndClose(t *testing.T) {
	al := NewAllowlist([]string{"echo"})
	sessionDone := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(sessionDone)
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}

		s := NewSession("test-1", c, al, slog.Default())
		if err := s.Start(context.Background(), "echo hello"); err != nil {
			t.Errorf("start: %v", err)
			return
		}

		// Wait for session to finish (echo exits quickly)
		select {
		case <-s.Done():
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for session to close")
			s.Close()
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := ws.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	// Read output from echo command
	_, data, err := c.Read(ctx)
	if err != nil {
		// echo might close before we read, which is acceptable
		t.Logf("read: %v (may be expected if echo exited quickly)", err)
	} else {
		t.Logf("received: %q", string(data))
	}

	select {
	case <-sessionDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server handler")
	}
}

func TestSession_DisallowedCommand(t *testing.T) {
	al := NewAllowlist([]string{"kubectl"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer c.Close(ws.StatusNormalClosure, "")

		s := NewSession("test-2", c, al, slog.Default())
		err = s.Start(context.Background(), "rm -rf /")
		if err == nil {
			t.Error("expected error for disallowed command")
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := ws.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()
}

func TestSession_LastActivity(t *testing.T) {
	al := NewAllowlist([]string{"echo"})
	s := NewSession("test-3", nil, al, slog.Default())

	before := s.LastActivity()
	time.Sleep(10 * time.Millisecond)
	s.touchActivity()
	after := s.LastActivity()

	if !after.After(before) {
		t.Error("expected lastActivity to advance after touchActivity")
	}
}
