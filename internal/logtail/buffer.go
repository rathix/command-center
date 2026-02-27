package logtail

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const (
	defaultMaxBufferSize = 50 * 1024 * 1024 // 50MB
	defaultBufferDir     = "/data/logs"
	writeChSize          = 10000
)

// LogBuffer provides filesystem-backed log buffering with LRU eviction.
type LogBuffer struct {
	path    string
	maxSize int64
	file    *os.File
	size    int64
	mu      sync.Mutex
	logger  *slog.Logger
	writeCh chan string
	done    chan struct{}
}

// NewLogBuffer creates a new filesystem-backed log buffer.
func NewLogBuffer(dir, sessionID, namespace, pod string, maxSize int64, logger *slog.Logger) (*LogBuffer, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	filename := sessionID + "-" + namespace + "-" + pod + ".log"
	path := filepath.Join(dir, filename)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}

	b := &LogBuffer{
		path:    path,
		maxSize: maxSize,
		file:    file,
		size:    0,
		logger:  logger,
		writeCh: make(chan string, writeChSize),
		done:    make(chan struct{}),
	}

	go b.writerLoop()

	return b, nil
}

// Write queues a line to be written to the buffer. Non-blocking: drops the line if the channel is full.
func (b *LogBuffer) Write(line string) {
	select {
	case b.writeCh <- line:
	default:
		// Channel full, drop the line to avoid blocking the live stream
		b.logger.Debug("log buffer write channel full, dropping line")
	}
}

// ReadAll returns a reader for the entire buffer contents.
func (b *LogBuffer) ReadAll() (io.ReadCloser, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return os.Open(b.path)
}

// Close stops the writer goroutine, closes the file, and deletes the buffer file.
func (b *LogBuffer) Close() error {
	close(b.done)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.file != nil {
		b.file.Close()
		b.file = nil
	}

	if err := os.Remove(b.path); err != nil && !os.IsNotExist(err) {
		b.logger.Warn("failed to remove log buffer file", "error", err)
		return err
	}

	b.logger.Debug("log buffer cleaned up", "path", b.path)
	return nil
}

// writerLoop reads lines from the write channel and appends them to the file.
func (b *LogBuffer) writerLoop() {
	for {
		select {
		case <-b.done:
			return
		case line, ok := <-b.writeCh:
			if !ok {
				return
			}
			b.appendLine(line)
		}
	}
}

// appendLine writes a single line to the buffer file and triggers eviction if needed.
func (b *LogBuffer) appendLine(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.file == nil {
		return
	}

	data := []byte(line + "\n")
	n, err := b.file.Write(data)
	if err != nil {
		b.logger.Warn("log buffer write failed", "error", err)
		return
	}
	b.size += int64(n)

	if b.size >= b.maxSize {
		b.evictLocked()
	}
}

// evictLocked truncates the oldest lines from the buffer. Must be called with mu held.
func (b *LogBuffer) evictLocked() {
	// Target: keep 80% of maxSize
	targetSize := int64(float64(b.maxSize) * 0.8)
	offset := b.size - targetSize

	if offset <= 0 {
		return
	}

	// Close current file for reading
	b.file.Close()

	// Open for reading
	src, err := os.Open(b.path)
	if err != nil {
		b.logger.Warn("eviction: failed to open source", "error", err)
		b.reopenFile()
		return
	}

	// Seek to offset
	if _, err := src.Seek(offset, io.SeekStart); err != nil {
		src.Close()
		b.logger.Warn("eviction: failed to seek", "error", err)
		b.reopenFile()
		return
	}

	// Find next newline boundary
	smallBuf := make([]byte, 4096)
	n, err := src.Read(smallBuf)
	if err != nil && err != io.EOF {
		src.Close()
		b.logger.Warn("eviction: failed to read boundary", "error", err)
		b.reopenFile()
		return
	}

	newlineOffset := 0
	for i := 0; i < n; i++ {
		if smallBuf[i] == '\n' {
			newlineOffset = i + 1
			break
		}
	}

	// Seek to the actual start of the kept content
	actualOffset := offset + int64(newlineOffset)
	if _, err := src.Seek(actualOffset, io.SeekStart); err != nil {
		src.Close()
		b.logger.Warn("eviction: failed to seek to boundary", "error", err)
		b.reopenFile()
		return
	}

	// Write to temp file
	tmpPath := b.path + ".tmp"
	dst, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		src.Close()
		b.logger.Warn("eviction: failed to create temp file", "error", err)
		b.reopenFile()
		return
	}

	// Copy in chunks to avoid loading entire file into memory
	written, err := io.Copy(dst, src)
	src.Close()
	dst.Close()

	if err != nil {
		os.Remove(tmpPath)
		b.logger.Warn("eviction: copy failed", "error", err)
		b.reopenFile()
		return
	}

	// Atomic rename
	if err := os.Rename(tmpPath, b.path); err != nil {
		os.Remove(tmpPath)
		b.logger.Warn("eviction: rename failed", "error", err)
		b.reopenFile()
		return
	}

	b.size = written
	b.reopenFile()
	b.logger.Debug("log buffer eviction complete", "newSize", b.size)
}

// reopenFile reopens the buffer file for appending.
func (b *LogBuffer) reopenFile() {
	file, err := os.OpenFile(b.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		b.logger.Warn("failed to reopen log buffer file", "error", err)
		b.file = nil
		return
	}
	b.file = file
}

// generateSessionID creates a random session ID for buffer file naming.
func generateSessionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
