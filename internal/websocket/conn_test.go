package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ws "nhooyr.io/websocket"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, "ws" + srv.URL[4:] // http -> ws
}

func TestPingPong_ClosesOnTimeout(t *testing.T) {
	serverDone := make(chan struct{})

	_, wsURL := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept error: %v", err)
			return
		}

		// Use CloseRead to start background reader for pong processing
		ctx := c.CloseRead(context.Background())

		conn := WrapConn(ctx, c,
			WithPingInterval(50*time.Millisecond),
			WithPongTimeout(100*time.Millisecond),
		)

		// Wait for ping loop to exit (pong timeout will kill it)
		select {
		case <-conn.done:
			// Expected: timed out waiting for pong
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for ping loop to exit")
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Client dials but does NOT read, so pongs won't be sent
	c, _, err := ws.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer c.CloseNow()

	select {
	case <-serverDone:
	case <-time.After(5 * time.Second):
		t.Error("timed out waiting for server handler to exit")
	}
}

func TestPingPong_ConnectionStaysAliveWhenPonging(t *testing.T) {
	serverDone := make(chan struct{})

	_, wsURL := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept error: %v", err)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		// Use CloseRead to start background reader for pong processing
		readCtx := c.CloseRead(ctx)

		conn := WrapConn(readCtx, c,
			WithPingInterval(50*time.Millisecond),
			WithPongTimeout(2*time.Second),
		)

		// Keep connection alive for 300ms — multiple pings should succeed
		time.Sleep(300 * time.Millisecond)

		// Verify ping loop is still running (not timed out)
		select {
		case <-conn.done:
			t.Error("ping loop exited unexpectedly — pong timeout?")
		default:
			// Good — still alive
		}

		cancel()
		<-conn.done
		c.Close(ws.StatusNormalClosure, "test done")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := ws.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	// Client needs an active read loop to respond to pings
	go func() {
		for {
			_, _, err := c.Read(ctx)
			if err != nil {
				return
			}
		}
	}()

	select {
	case <-serverDone:
	case <-time.After(5 * time.Second):
		t.Error("timed out waiting for server to close connection")
	}
	c.CloseNow()
}

func TestConn_Close(t *testing.T) {
	serverDone := make(chan struct{})

	_, wsURL := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept error: %v", err)
			return
		}
		conn := WrapConn(context.Background(), c,
			WithPingInterval(10*time.Second),
			WithPongTimeout(10*time.Second),
		)
		conn.Close(ws.StatusNormalClosure, "test close")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := ws.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	// Client reads so close handshake works
	go func() {
		for {
			_, _, err := c.Read(ctx)
			if err != nil {
				return
			}
		}
	}()

	select {
	case <-serverDone:
	case <-time.After(5 * time.Second):
		t.Error("timed out waiting for close")
	}
	c.CloseNow()
}

func TestConn_DoubleCloseIsNoop(t *testing.T) {
	serverDone := make(chan struct{})

	_, wsURL := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept error: %v", err)
			return
		}
		conn := WrapConn(context.Background(), c,
			WithPingInterval(10*time.Second),
			WithPongTimeout(10*time.Second),
		)
		conn.Close(ws.StatusNormalClosure, "first")
		if err := conn.Close(ws.StatusNormalClosure, "second"); err != nil {
			t.Errorf("second close should not error: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := ws.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	go func() {
		for {
			_, _, err := c.Read(ctx)
			if err != nil {
				return
			}
		}
	}()

	select {
	case <-serverDone:
	case <-time.After(5 * time.Second):
		t.Error("timed out waiting for close")
	}
	c.CloseNow()
}
