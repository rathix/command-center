package terminal

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/rathix/command-center/internal/websocket"
	ws "nhooyr.io/websocket"
)

// ManagerOption is a functional option for the Manager.
type ManagerOption func(*Manager)

// WithLogger sets the logger for the manager.
func WithLogger(l *slog.Logger) ManagerOption {
	return func(m *Manager) { m.logger = l }
}

// WithIdleTimeout sets the idle timeout for sessions.
func WithIdleTimeout(d time.Duration) ManagerOption {
	return func(m *Manager) { m.idleTimeout = d }
}

// WithMaxSessions sets the maximum number of concurrent sessions.
func WithMaxSessions(n int) ManagerOption {
	return func(m *Manager) { m.maxSessions = n }
}

// WithAllowedCommands sets the allowed commands for new sessions.
func WithAllowedCommands(cmds []string) ManagerOption {
	return func(m *Manager) { m.allowlist = NewAllowlist(cmds) }
}

// WithIdleScanInterval sets the interval for idle session scanning (for testing).
func WithIdleScanInterval(d time.Duration) ManagerOption {
	return func(m *Manager) { m.idleScanInterval = d }
}

// Manager manages terminal sessions.
type Manager struct {
	mu               sync.Mutex
	sessions         map[string]*Session
	maxSessions      int
	idleTimeout      time.Duration
	idleScanInterval time.Duration
	allowlist        *Allowlist
	registry         *websocket.ConnectionRegistry
	logger           *slog.Logger
	nextID           int
}

// NewManager creates a new terminal session manager.
func NewManager(registry *websocket.ConnectionRegistry, opts ...ManagerOption) *Manager {
	m := &Manager{
		sessions:         make(map[string]*Session),
		maxSessions:      4,
		idleTimeout:      15 * time.Minute,
		idleScanInterval: 5 * time.Second,
		allowlist:        NewAllowlist(nil),
		registry:         registry,
		logger:           slog.Default(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// CreateSession creates and starts a new terminal session.
// Returns the session ID or an error if the limit is reached.
func (m *Manager) CreateSession(ctx context.Context, wsConn *ws.Conn, commandLine string) (string, error) {
	m.mu.Lock()
	if len(m.sessions) >= m.maxSessions {
		count := len(m.sessions)
		m.mu.Unlock()
		return "", fmt.Errorf("maximum concurrent sessions reached (%d/%d)", count, m.maxSessions)
	}
	m.nextID++
	id := fmt.Sprintf("term-%d", m.nextID)
	session := NewSession(id, wsConn, m.allowlist, m.logger)
	m.sessions[id] = session
	m.mu.Unlock()

	if err := session.Start(ctx, commandLine); err != nil {
		m.mu.Lock()
		delete(m.sessions, id)
		m.mu.Unlock()
		return "", err
	}

	// Auto-remove session when it terminates
	go func() {
		<-session.Done()
		m.RemoveSession(id)
	}()

	return id, nil
}

// RemoveSession removes a session from tracking.
func (m *Manager) RemoveSession(id string) {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()

	if ok && session != nil {
		session.Close()
	}
}

// SessionCount returns the number of active sessions.
func (m *Manager) SessionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// RunIdleScanner starts the idle session scanner. Blocks until ctx is cancelled.
func (m *Manager) RunIdleScanner(ctx context.Context) {
	ticker := time.NewTicker(m.idleScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.scanIdleSessions(ctx)
		}
	}
}

func (m *Manager) scanIdleSessions(ctx context.Context) {
	m.mu.Lock()
	var idle []*Session
	now := time.Now()
	for _, s := range m.sessions {
		if now.Sub(s.LastActivity()) > m.idleTimeout {
			idle = append(idle, s)
		}
	}
	m.mu.Unlock()

	for _, s := range idle {
		m.logger.Info("terminating idle session", slog.String("session_id", s.ID),
			slog.Duration("idle_for", now.Sub(s.LastActivity())))
		s.sendTerminated(ctx, "idle timeout")
		s.Close()
	}
}

// Shutdown terminates all active sessions.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	if len(sessions) == 0 {
		return
	}

	m.logger.Info("shutting down terminal sessions", slog.Int("count", len(sessions)))

	var wg sync.WaitGroup
	for _, s := range sessions {
		wg.Add(1)
		go func(s *Session) {
			defer wg.Done()
			s.sendTerminated(ctx, "server shutdown")
			s.Close()
		}(s)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("all terminal sessions terminated")
	case <-ctx.Done():
		m.logger.Warn("shutdown timeout, some sessions may not have closed cleanly")
	}
}
