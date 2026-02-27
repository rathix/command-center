package websocket

import (
	"context"
	"log/slog"
	"sync"

	ws "nhooyr.io/websocket"
)

// ConnectionRegistry tracks active WebSocket connections for graceful shutdown.
type ConnectionRegistry struct {
	mu    sync.Mutex
	conns map[*Conn]struct{}
	log   *slog.Logger
}

// NewRegistry creates a new ConnectionRegistry.
func NewRegistry(logger *slog.Logger) *ConnectionRegistry {
	if logger == nil {
		logger = slog.Default()
	}
	return &ConnectionRegistry{
		conns: make(map[*Conn]struct{}),
		log:   logger,
	}
}

// Register adds a connection to the registry.
func (r *ConnectionRegistry) Register(c *Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conns[c] = struct{}{}
}

// Unregister removes a connection from the registry.
func (r *ConnectionRegistry) Unregister(c *Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns, c)
}

// Count returns the number of active connections.
func (r *ConnectionRegistry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.conns)
}

// CloseAll sends a close frame to every registered connection.
// It waits for each close to complete or for the context to expire.
func (r *ConnectionRegistry) CloseAll(ctx context.Context) {
	r.mu.Lock()
	snapshot := make([]*Conn, 0, len(r.conns))
	for c := range r.conns {
		snapshot = append(snapshot, c)
	}
	r.mu.Unlock()

	if len(snapshot) == 0 {
		return
	}

	r.log.Info("closing all WebSocket connections", slog.Int("count", len(snapshot)))

	var wg sync.WaitGroup
	for _, c := range snapshot {
		wg.Add(1)
		go func(c *Conn) {
			defer wg.Done()
			_ = c.CloseWithContext(ctx, ws.StatusGoingAway, "server shutting down")
		}(c)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.log.Info("all WebSocket connections closed")
	case <-ctx.Done():
		r.log.Warn("shutdown timeout reached, some WebSocket connections may not have closed cleanly")
	}
}
