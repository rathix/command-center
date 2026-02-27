package websocket

import (
	"context"
	"log/slog"
	"sync"
	"time"

	ws "nhooyr.io/websocket"
)

// Conn wraps a nhooyr.io/websocket.Conn with server-side ping/pong keepalive
// and graceful close-frame logic.
type Conn struct {
	inner  *ws.Conn
	opts   Options
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
	closed bool
}

// WrapConn wraps an accepted WebSocket connection with ping/pong keepalive.
// The returned Conn starts a background goroutine that pings the peer at the
// configured interval. Call Close to stop the goroutine and close the connection.
//
// IMPORTANT: The caller must have an active Read loop on the connection for
// pong responses to be processed (nhooyr.io/websocket v1.x requirement).
func WrapConn(ctx context.Context, c *ws.Conn, options ...Option) *Conn {
	opts := applyOptions(options)
	ctx, cancel := context.WithCancel(ctx)
	conn := &Conn{
		inner:  c,
		opts:   opts,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go conn.pingLoop(ctx)
	return conn
}

// Inner returns the underlying nhooyr.io/websocket.Conn for direct read/write.
func (c *Conn) Inner() *ws.Conn {
	return c.inner
}

// Close sends a close frame and shuts down the connection.
func (c *Conn) Close(code ws.StatusCode, reason string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	c.cancel()
	<-c.done
	return c.inner.Close(code, reason)
}

// CloseWithContext sends a close frame within the given context deadline.
func (c *Conn) CloseWithContext(ctx context.Context, code ws.StatusCode, reason string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	c.cancel()

	// Wait for ping loop to finish, but respect ctx deadline
	select {
	case <-c.done:
	case <-ctx.Done():
	}
	return c.inner.Close(code, reason)
}

// ForceClose immediately closes the underlying connection without sending a close frame.
// Used when the connection is already broken.
func (c *Conn) ForceClose() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()
	c.cancel()
	c.inner.CloseNow()
}

func (c *Conn) pingLoop(ctx context.Context) {
	defer close(c.done)

	ticker := time.NewTicker(c.opts.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(ctx, c.opts.PongTimeout)
			err := c.inner.Ping(pingCtx)
			pingCancel()
			if err != nil {
				if ctx.Err() != nil {
					// Parent context cancelled — not a pong timeout
					return
				}
				c.opts.Logger.Warn("pong timeout, closing connection", slog.String("error", err.Error()))
				// Use CloseNow to avoid blocking on close handshake with unresponsive peer
				c.inner.CloseNow()
				return
			}
		}
	}
}
