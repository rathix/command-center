package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
	ws "nhooyr.io/websocket"
)

// ControlMessage is a JSON message sent over the WebSocket text channel.
type ControlMessage struct {
	Type    string `json:"type"`
	Cols    int    `json:"cols,omitempty"`
	Rows    int    `json:"rows,omitempty"`
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Session represents a single terminal session backed by a PTY.
type Session struct {
	ID           string
	cmd          *exec.Cmd
	ptmx         *os.File
	wsConn       *ws.Conn
	allowlist    *Allowlist
	logger       *slog.Logger
	createdAt    time.Time
	lastActivity atomic.Int64 // Unix nanoseconds
	cancel       context.CancelFunc
	done         chan struct{}
	closeOnce    sync.Once
}

// NewSession creates a new terminal session but does not start it.
func NewSession(id string, wsConn *ws.Conn, allowlist *Allowlist, logger *slog.Logger) *Session {
	now := time.Now()
	s := &Session{
		ID:        id,
		wsConn:    wsConn,
		allowlist: allowlist,
		logger:    logger.With(slog.String("session_id", id)),
		createdAt: now,
		done:      make(chan struct{}),
	}
	s.lastActivity.Store(now.UnixNano())
	return s
}

// Start validates the command, spawns the PTY process, and begins I/O bridging.
func (s *Session) Start(ctx context.Context, commandLine string) error {
	cmd, args, err := s.allowlist.Validate(commandLine)
	if err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	s.cmd = exec.CommandContext(ctx, cmd, args...)
	ptmx, err := pty.Start(s.cmd)
	if err != nil {
		return fmt.Errorf("failed to start PTY: %w", err)
	}
	s.ptmx = ptmx

	ctx, s.cancel = context.WithCancel(ctx)

	// Start I/O bridges
	go s.readPump(ctx)
	go s.writePump(ctx)

	s.logger.Info("terminal session started", slog.String("command", cmd))
	return nil
}

// Close terminates the session: kills the process, closes PTY, closes WS.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}

		// Kill the process
		if s.cmd != nil && s.cmd.Process != nil {
			_ = s.cmd.Process.Signal(os.Interrupt)
			// Give it 3s to exit gracefully
			done := make(chan struct{})
			go func() {
				_ = s.cmd.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				_ = s.cmd.Process.Kill()
			}
		}

		// Close PTY
		if s.ptmx != nil {
			_ = s.ptmx.Close()
		}

		// Close WS
		if s.wsConn != nil {
			_ = s.wsConn.Close(ws.StatusNormalClosure, "session closed")
		}

		close(s.done)
		s.logger.Info("terminal session closed")
	})
}

// Done returns a channel that's closed when the session terminates.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// LastActivity returns the timestamp of the last I/O activity.
func (s *Session) LastActivity() time.Time {
	return time.Unix(0, s.lastActivity.Load())
}

// touchActivity updates the last activity timestamp.
func (s *Session) touchActivity() {
	s.lastActivity.Store(time.Now().UnixNano())
}

// readPump reads PTY output and sends it as binary WS frames.
func (s *Session) readPump(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := s.ptmx.Read(buf)
		if err != nil {
			if err != io.EOF && ctx.Err() == nil {
				s.logger.Warn("PTY read error", slog.String("error", err.Error()))
			}
			s.Close()
			return
		}

		s.touchActivity()

		err = s.wsConn.Write(ctx, ws.MessageBinary, buf[:n])
		if err != nil {
			if ctx.Err() == nil {
				s.logger.Warn("WS write error", slog.String("error", err.Error()))
			}
			s.Close()
			return
		}
	}
}

// writePump reads WS messages and routes them to PTY stdin or handles control messages.
func (s *Session) writePump(ctx context.Context) {
	for {
		msgType, data, err := s.wsConn.Read(ctx)
		if err != nil {
			if ctx.Err() == nil {
				s.logger.Warn("WS read error", slog.String("error", err.Error()))
			}
			s.Close()
			return
		}

		s.touchActivity()

		switch msgType {
		case ws.MessageBinary:
			// Raw PTY stdin
			if _, err := s.ptmx.Write(data); err != nil {
				if ctx.Err() == nil {
					s.logger.Warn("PTY write error", slog.String("error", err.Error()))
				}
				s.Close()
				return
			}

		case ws.MessageText:
			// JSON control message
			var msg ControlMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				s.logger.Warn("invalid control message", slog.String("error", err.Error()))
				continue
			}

			switch msg.Type {
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					if err := pty.Setsize(s.ptmx, &pty.Winsize{
						Cols: uint16(msg.Cols),
						Rows: uint16(msg.Rows),
					}); err != nil {
						s.logger.Warn("resize failed", slog.String("error", err.Error()))
					}
				}
			default:
				s.logger.Warn("unknown control message type", slog.String("type", msg.Type))
			}
		}
	}
}

// sendError sends a JSON error message to the client.
func (s *Session) sendError(ctx context.Context, message string) {
	msg := ControlMessage{Type: "error", Message: message}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_ = s.wsConn.Write(ctx, ws.MessageText, data)
}

// sendTerminated sends a JSON terminated message to the client.
func (s *Session) sendTerminated(ctx context.Context, reason string) {
	msg := ControlMessage{Type: "terminated", Reason: reason}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_ = s.wsConn.Write(ctx, ws.MessageText, data)
}
