package history

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/rathix/command-center/internal/state"
)

// TransitionRecord captures a single health-status transition for a service.
type TransitionRecord struct {
	Timestamp  time.Time          `json:"ts"`
	ServiceKey string             `json:"svc"`
	PrevStatus state.HealthStatus `json:"prev"`
	NextStatus state.HealthStatus `json:"next"`
	HTTPCode   *int               `json:"code"`
	ResponseMs *int64             `json:"ms"`
}

// HistoryWriter persists health-status transition records.
type HistoryWriter interface {
	Record(TransitionRecord) error
	Close() error
}

// FileWriter implements HistoryWriter by appending JSONL to a file.
type FileWriter struct {
	mu     sync.Mutex
	path   string
	file   *os.File
	logger *slog.Logger
}

// NewFileWriter opens (or creates) the file at path for append-only writing.
// If logger is nil, a no-op logger is used.
func NewFileWriter(path string, logger *slog.Logger) (*FileWriter, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &FileWriter{
		path:   path,
		file:   f,
		logger: logger,
	}, nil
}

// reopen closes the current file handle and opens a new one at the same path.
// Must be called while mu is held.
func (w *FileWriter) reopen() error {
	w.file.Close()
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	w.file = f
	return nil
}

// Record marshals the transition record as JSON and appends it as a single line.
func (w *FileWriter) Record(rec TransitionRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	_, err = w.file.Write(data)
	if err != nil {
		w.logger.Error("failed to write history record", "error", err)
	}
	return err
}

// Close closes the underlying file handle.
func (w *FileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// NoopWriter is a HistoryWriter that discards all records.
type NoopWriter struct{}

// Record discards the record and returns nil.
func (NoopWriter) Record(TransitionRecord) error { return nil }

// Close is a no-op and returns nil.
func (NoopWriter) Close() error { return nil }
