package logtail

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testBufferLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLogBuffer_WritesLines(t *testing.T) {
	dir := t.TempDir()
	buf, err := NewLogBuffer(dir, "sess1", "default", "pod1", 50*1024*1024, testBufferLogger())
	if err != nil {
		t.Fatalf("NewLogBuffer: %v", err)
	}
	defer buf.Close()

	buf.Write("line1")
	buf.Write("line2")
	buf.Write("line3")

	// Wait for async writes
	time.Sleep(100 * time.Millisecond)

	// Read the file directly
	data, err := os.ReadFile(buf.path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "line1\n") {
		t.Errorf("missing line1 in content: %q", content)
	}
	if !strings.Contains(content, "line2\n") {
		t.Errorf("missing line2 in content: %q", content)
	}
	if !strings.Contains(content, "line3\n") {
		t.Errorf("missing line3 in content: %q", content)
	}
}

func TestLogBuffer_ReadAll(t *testing.T) {
	dir := t.TempDir()
	buf, err := NewLogBuffer(dir, "sess2", "default", "pod1", 50*1024*1024, testBufferLogger())
	if err != nil {
		t.Fatalf("NewLogBuffer: %v", err)
	}
	defer buf.Close()

	buf.Write("alpha")
	buf.Write("beta")
	buf.Write("gamma")

	// Wait for async writes
	time.Sleep(100 * time.Millisecond)

	reader, err := buf.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "alpha\n") {
		t.Errorf("missing alpha in content: %q", content)
	}
	if !strings.Contains(content, "gamma\n") {
		t.Errorf("missing gamma in content: %q", content)
	}
}

func TestLogBuffer_EvictsAtCap(t *testing.T) {
	dir := t.TempDir()
	// Use a small cap for testing: 1KB
	maxSize := int64(1024)
	buf, err := NewLogBuffer(dir, "sess3", "default", "pod1", maxSize, testBufferLogger())
	if err != nil {
		t.Fatalf("NewLogBuffer: %v", err)
	}
	defer buf.Close()

	// Write enough data to exceed the cap
	line := strings.Repeat("x", 100) // 100 bytes per line
	for i := 0; i < 20; i++ {
		buf.Write(line)
	}

	// Wait for writes and eviction
	time.Sleep(300 * time.Millisecond)

	info, err := os.Stat(buf.path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	// File size should be under maxSize (or slightly above due to write timing)
	if info.Size() > maxSize+200 {
		t.Errorf("file size %d exceeds max %d by too much", info.Size(), maxSize)
	}

	// Verify newest lines are preserved
	data, err := os.ReadFile(buf.path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, line) {
		t.Errorf("expected recent lines to be preserved")
	}
}

func TestLogBuffer_EvictionPreservesCompleteLines(t *testing.T) {
	dir := t.TempDir()
	maxSize := int64(500)
	buf, err := NewLogBuffer(dir, "sess4", "default", "pod1", maxSize, testBufferLogger())
	if err != nil {
		t.Fatalf("NewLogBuffer: %v", err)
	}
	defer buf.Close()

	// Write lines of varying length
	for i := 0; i < 20; i++ {
		buf.Write(strings.Repeat("a", 50+i))
	}

	// Wait for writes and eviction
	time.Sleep(300 * time.Millisecond)

	data, err := os.ReadFile(buf.path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Every line should start at the beginning of a line (after eviction)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		// Each line should only contain 'a' characters (not partial lines)
		for _, c := range line {
			if c != 'a' {
				t.Errorf("line %d contains unexpected character %q: %q", i, c, line)
				break
			}
		}
	}
}

func TestLogBuffer_NonBlockingWrite(t *testing.T) {
	dir := t.TempDir()
	buf, err := NewLogBuffer(dir, "sess5", "default", "pod1", 50*1024*1024, testBufferLogger())
	if err != nil {
		t.Fatalf("NewLogBuffer: %v", err)
	}
	defer buf.Close()

	// Fill the write channel by writing many lines rapidly
	// This should NOT block even if the channel is full
	done := make(chan struct{})
	go func() {
		for i := 0; i < 20000; i++ {
			buf.Write("test line")
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - Write did not block
	case <-time.After(2 * time.Second):
		t.Fatal("Write blocked when channel was full")
	}
}

func TestLogBuffer_CloseDeletesFile(t *testing.T) {
	dir := t.TempDir()
	buf, err := NewLogBuffer(dir, "sess6", "default", "pod1", 50*1024*1024, testBufferLogger())
	if err != nil {
		t.Fatalf("NewLogBuffer: %v", err)
	}

	path := buf.path
	buf.Write("some data")
	time.Sleep(50 * time.Millisecond)

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist before close: %v", err)
	}

	buf.Close()

	// Verify file is removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be deleted after close, got error: %v", err)
	}
}

func TestLogBuffer_ConcurrentWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	buf, err := NewLogBuffer(dir, "sess7", "default", "pod1", 50*1024*1024, testBufferLogger())
	if err != nil {
		t.Fatalf("NewLogBuffer: %v", err)
	}
	defer buf.Close()

	var wg sync.WaitGroup

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			buf.Write("concurrent line")
		}
	}()

	// Reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			reader, err := buf.ReadAll()
			if err != nil {
				continue // File might not exist yet
			}
			io.ReadAll(reader)
			reader.Close()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()
}

func TestLogBuffer_DirectoryCreation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", "logs")
	// Directory doesn't exist yet
	buf, err := NewLogBuffer(dir, "sess8", "default", "pod1", 50*1024*1024, testBufferLogger())
	if err != nil {
		t.Fatalf("NewLogBuffer should create directory: %v", err)
	}
	defer buf.Close()

	// Directory should now exist
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("directory should have been created: %v", err)
	}
}
