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

// TransitionRecord represents a single health state transition.
type TransitionRecord struct {
	Timestamp  time.Time          `json:"ts"`
	ServiceKey string             `json:"svc"`
	PrevStatus state.HealthStatus `json:"prev"`
	NextStatus state.HealthStatus `json:"next"`
	HTTPCode   *int               `json:"code"`
	ResponseMs *int64             `json:"ms"`
}

// HistoryWriter records health state transitions.
type HistoryWriter interface {
	Record(TransitionRecord) error
	Close() error
}

// FileWriter appends transition records to a JSONL file.
type FileWriter struct {
	mu     sync.Mutex
	file   *os.File
	logger *slog.Logger
}

// NewFileWriter creates a new FileWriter that appends to the given path.
func NewFileWriter(path string, logger *slog.Logger) (*FileWriter, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &FileWriter{file: f, logger: logger}, nil
}

// Record appends a transition record as a JSON line.
func (w *FileWriter) Record(rec TransitionRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = w.file.Write(data)
	return err
}

// Close closes the underlying file.
func (w *FileWriter) Close() error {
	return w.file.Close()
}

// NoopWriter is a no-op HistoryWriter for when history is disabled.
type NoopWriter struct{}

func (NoopWriter) Record(TransitionRecord) error { return nil }
func (NoopWriter) Close() error                  { return nil }
