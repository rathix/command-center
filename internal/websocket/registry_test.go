package websocket

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ws "nhooyr.io/websocket"
)

func TestRegistry_RegisterUnregister(t *testing.T) {
	reg := NewRegistry(slog.Default())

	serverDone := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		c, err := ws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		conn := WrapConn(ctx, c,
			WithPingInterval(10*time.Second),
			WithPongTimeout(10*time.Second),
		)
		reg.Register(conn)

		if got := reg.Count(); got != 1 {
			t.Errorf("expected count 1, got %d", got)
		}

		reg.Unregister(conn)

		if got := reg.Count(); got != 0 {
			t.Errorf("expected count 0 after unregister, got %d", got)
		}

		cancel()
		<-conn.done
		c.Close(ws.StatusNormalClosure, "done")
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := ws.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Keep reading for close handshake
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
		t.Fatal("timed out")
	}
	c.CloseNow()
}

func TestRegistry_CloseAll(t *testing.T) {
	reg := NewRegistry(slog.Default())
	const numConns = 3

	var acceptedCount atomic.Int32
	allAccepted := make(chan struct{})
	serversDone := make(chan struct{})
	var serverWg sync.WaitGroup

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverWg.Add(1)
		defer serverWg.Done()

		c, err := ws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		conn := WrapConn(context.Background(), c,
			WithPingInterval(10*time.Second),
			WithPongTimeout(10*time.Second),
		)
		reg.Register(conn)

		if acceptedCount.Add(1) == numConns {
			close(allAccepted)
		}

		// Block until connection is closed by CloseAll
		for {
			_, _, err := c.Read(context.Background())
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	go func() {
		serverWg.Wait()
		close(serversDone)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Dial numConns connections, each with an active reader for close handshake
	clients := make([]*ws.Conn, numConns)
	for i := 0; i < numConns; i++ {
		c, _, err := ws.Dial(ctx, "ws"+srv.URL[4:], nil)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		clients[i] = c
		// Active read loop for close handshake
		go func(c *ws.Conn) {
			for {
				_, _, err := c.Read(ctx)
				if err != nil {
					return
				}
			}
		}(c)
	}

	// Wait for all server connections to be accepted
	select {
	case <-allAccepted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for all connections to be accepted")
	}

	if got := reg.Count(); got != numConns {
		t.Fatalf("expected %d connections, got %d", numConns, got)
	}

	// CloseAll
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCancel()
	reg.CloseAll(closeCtx)

	// Wait for all server handlers to exit
	select {
	case <-serversDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server handlers to exit")
	}

	for _, c := range clients {
		c.CloseNow()
	}
}

func TestRegistry_CloseAllEmpty(t *testing.T) {
	reg := NewRegistry(slog.Default())
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	reg.CloseAll(ctx)
}

func TestNewRegistry_NilLogger(t *testing.T) {
	reg := NewRegistry(nil)
	if reg.log == nil {
		t.Fatal("expected non-nil logger when nil passed to NewRegistry")
	}
}
