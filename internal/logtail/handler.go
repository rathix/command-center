package logtail

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"nhooyr.io/websocket"
)

// PodLogStreamer abstracts the K8s pod log API for testability.
type PodLogStreamer interface {
	StreamPodLogs(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error)
	GetPod(ctx context.Context, namespace, pod string) (*corev1.Pod, error)
}

// Option configures a Handler.
type Option func(*Handler)

// WithBufferSize sets the maximum scanner buffer size for reading log lines.
func WithBufferSize(size int) Option {
	return func(h *Handler) {
		h.bufferSize = size
	}
}

// WithInitialTailLines sets the number of historical lines to fetch on connect.
func WithInitialTailLines(n int64) Option {
	return func(h *Handler) {
		h.initialTail = n
	}
}

// WithMaxBufferSize sets the maximum filesystem buffer size per session.
func WithMaxBufferSize(bytes int64) Option {
	return func(h *Handler) {
		h.maxBufferSize = bytes
	}
}

// WithBufferDir sets the directory for filesystem log buffers.
func WithBufferDir(dir string) Option {
	return func(h *Handler) {
		h.bufferDir = dir
	}
}

// Handler serves WebSocket connections for pod log streaming.
type Handler struct {
	streamer      PodLogStreamer
	logger        *slog.Logger
	bufferSize    int
	initialTail   int64
	maxBufferSize int64
	bufferDir     string
}

// NewHandler creates a log tail WebSocket handler.
func NewHandler(streamer PodLogStreamer, logger *slog.Logger, opts ...Option) *Handler {
	h := &Handler{
		streamer:      streamer,
		logger:        logger,
		bufferSize:    1024 * 1024, // 1MB max line
		initialTail:   1000,
		maxBufferSize: defaultMaxBufferSize,
		bufferDir:     defaultBufferDir,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// ServeHTTP upgrades the connection to WebSocket and streams pod logs.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	pod := r.PathValue("pod")

	if namespace == "" || pod == "" {
		http.Error(w, "namespace and pod are required", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Accept connections from any origin (mTLS handles auth)
	})
	if err != nil {
		h.logger.Warn("websocket accept failed", "error", err)
		return
	}
	defer conn.CloseNow()

	// Set read limit high enough for large log lines and client commands
	conn.SetReadLimit(int64(h.bufferSize) + 4096)

	h.streamLogs(r.Context(), conn, namespace, pod)
}

// streamLogs manages the log streaming lifecycle for a single WS connection.
func (h *Handler) streamLogs(ctx context.Context, conn *websocket.Conn, namespace, pod string) {
	filter := &filterState{}

	// Create filesystem buffer for scrollback
	sessionID := generateSessionID()
	logBuf, err := NewLogBuffer(h.bufferDir, sessionID, namespace, pod, h.maxBufferSize, h.logger)
	if err != nil {
		h.logger.Warn("log buffer creation failed, continuing without buffer", "error", err)
	}
	if logBuf != nil {
		defer logBuf.Close()
	}

	// Start client command reader goroutine
	cmdCtx, cmdCancel := context.WithCancel(ctx)
	defer cmdCancel()
	go readClientCommands(cmdCtx, conn, filter, h.logger)

	tailLines := h.initialTail
	for {
		opts := &corev1.PodLogOptions{
			Follow: true,
		}
		if tailLines > 0 {
			opts.TailLines = &tailLines
		}

		stream, err := h.streamer.StreamPodLogs(ctx, namespace, pod, opts)
		if err != nil {
			h.handleStreamError(ctx, conn, err)
			return
		}

		eof := h.readAndForward(ctx, conn, stream, filter, logBuf)
		stream.Close()

		if !eof {
			// Context was cancelled or connection closed
			return
		}

		// Stream ended (EOF) - check for pod restart
		p, err := h.streamer.GetPod(ctx, namespace, pod)
		if err != nil || p == nil {
			// Pod gone, close connection normally
			conn.Close(websocket.StatusNormalClosure, "pod terminated")
			return
		}

		if p.Status.Phase == corev1.PodRunning {
			// Pod restarted - send control message and reopen stream
			ctrlMsg, _ := json.Marshal(map[string]string{
				"type":  "control",
				"event": "pod-restarted",
			})
			if err := conn.Write(ctx, websocket.MessageText, ctrlMsg); err != nil {
				return
			}
			// Restart without TailLines to get all lines from new instance
			tailLines = 0
			continue
		}

		// Pod not running, close normally
		conn.Close(websocket.StatusNormalClosure, "pod not running")
		return
	}
}

// readAndForward reads log lines from the stream and forwards them to the WS connection.
// Returns true if the stream ended with EOF, false if the context was cancelled.
func (h *Handler) readAndForward(ctx context.Context, conn *websocket.Conn, stream io.Reader, filter *filterState, logBuf *LogBuffer) bool {
	scanner := bufio.NewScanner(stream)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, h.bufferSize)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return false
		}

		line := scanner.Text()

		// Write all lines to buffer (unfiltered)
		if logBuf != nil {
			logBuf.Write(line)
		}

		if !filter.matches(line) {
			continue
		}

		if err := conn.Write(ctx, websocket.MessageText, []byte(line)); err != nil {
			return false
		}
	}

	// Scanner finished - could be EOF or error
	return true
}

// handleStreamError sends an error message and closes the WS with appropriate code.
func (h *Handler) handleStreamError(ctx context.Context, conn *websocket.Conn, err error) {
	h.logger.Warn("log stream error", "error", err)

	closeCode := websocket.StatusCode(4500)
	if isNotFound(err) {
		closeCode = 4404
	}

	errMsg, _ := json.Marshal(map[string]string{
		"type":    "error",
		"message": err.Error(),
	})
	_ = conn.Write(ctx, websocket.MessageText, errMsg)
	conn.Close(closeCode, err.Error())
}

// isNotFound checks if the error indicates a pod was not found.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	// Check for our test error type
	if _, ok := err.(*notFoundError); ok {
		return true
	}
	// Check for K8s not found errors by message pattern
	return strings.Contains(err.Error(), "not found")
}

// NotFoundError represents a pod/namespace not found error.
type notFoundError struct {
	namespace string
	pod       string
}

func (e *notFoundError) Error() string {
	return "pod " + e.pod + " not found in namespace " + e.namespace
}

// InternalError represents a K8s API failure.
type internalError struct {
	message string
}

func (e *internalError) Error() string {
	return e.message
}

// clientCommand represents a command sent from the client over the WS connection.
type clientCommand struct {
	Type    string `json:"type"`
	Pattern string `json:"pattern"`
}

// readClientCommands reads commands from the client and applies them.
func readClientCommands(ctx context.Context, conn *websocket.Conn, filter *filterState, logger *slog.Logger) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var cmd clientCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			logger.Debug("invalid client command", "error", err)
			continue
		}

		switch cmd.Type {
		case "filter":
			filter.setPattern(cmd.Pattern)
			logger.Debug("filter updated", "pattern", cmd.Pattern)
		default:
			logger.Debug("unknown command type", "type", cmd.Type)
		}
	}
}

// filterState holds the current filter pattern for a connection.
type filterState struct {
	mu      sync.RWMutex
	pattern string
	regex   *regexp.Regexp
}

// setPattern updates the filter pattern. If the pattern is a valid regex, it is compiled.
// Otherwise, it falls back to substring matching.
func (f *filterState) setPattern(pattern string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pattern = pattern
	if pattern == "" {
		f.regex = nil
		return
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		// Invalid regex, fall back to substring matching
		f.regex = nil
		return
	}
	f.regex = re
}

// matches returns true if the line matches the current filter, or if no filter is set.
func (f *filterState) matches(line string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.pattern == "" {
		return true
	}
	if f.regex != nil {
		return f.regex.MatchString(line)
	}
	return strings.Contains(line, f.pattern)
}
