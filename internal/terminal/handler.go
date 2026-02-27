package terminal

import (
	"encoding/json"
	"log/slog"
	"net/http"

	appws "github.com/rathix/command-center/internal/websocket"
	ws "nhooyr.io/websocket"
)

// Handler handles WebSocket connections for terminal sessions.
type Handler struct {
	manager  *Manager
	registry *appws.ConnectionRegistry
	enabled  bool
	logger   *slog.Logger
}

// NewHandler creates a terminal HTTP handler.
func NewHandler(manager *Manager, registry *appws.ConnectionRegistry, enabled bool, logger *slog.Logger) *Handler {
	return &Handler{
		manager:  manager,
		registry: registry,
		enabled:  enabled,
		logger:   logger,
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Terminal feature is not enabled",
		})
		return
	}

	// Defense-in-depth mTLS check (transport layer should already enforce this)
	if !appws.CheckMTLS(r) {
		appws.RejectNoMTLS(w)
		return
	}

	// Get command from query parameter
	command := r.URL.Query().Get("command")
	if command == "" {
		command = "kubectl" // default command
	}

	// Accept WebSocket upgrade
	wsConn, err := appws.Accept(w, r, nil)
	if err != nil {
		h.logger.Warn("WebSocket upgrade failed", slog.String("error", err.Error()))
		return
	}

	// Wrap for registry tracking
	wrappedConn := appws.WrapConn(r.Context(), wsConn,
		appws.WithPingInterval(appws.DefaultPingInterval),
		appws.WithPongTimeout(appws.DefaultPongTimeout),
		appws.WithLogger(h.logger),
	)
	h.registry.Register(wrappedConn)
	defer h.registry.Unregister(wrappedConn)

	// Create session
	sessionID, err := h.manager.CreateSession(r.Context(), wsConn, command)
	if err != nil {
		h.logger.Warn("failed to create terminal session", slog.String("error", err.Error()))
		// Send error message before closing
		errMsg := ControlMessage{Type: "error", Message: err.Error()}
		data, _ := json.Marshal(errMsg)
		_ = wsConn.Write(r.Context(), ws.MessageText, data)
		wsConn.Close(ws.StatusPolicyViolation, err.Error())
		return
	}

	h.logger.Info("terminal session created", slog.String("session_id", sessionID))

	// Block until session ends (read/write pumps handle I/O)
	session := h.getSession(sessionID)
	if session != nil {
		<-session.Done()
	}

	wrappedConn.ForceClose()
}

func (h *Handler) getSession(id string) *Session {
	h.manager.mu.Lock()
	defer h.manager.mu.Unlock()
	return h.manager.sessions[id]
}
