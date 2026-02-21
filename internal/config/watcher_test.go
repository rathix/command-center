package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// testCallback captures reload invocations for assertions.
type testCallback struct {
	mu      sync.Mutex
	calls   []callRecord
	callsCh chan struct{}
}

type callRecord struct {
	cfg  *Config
	errs []error
}

func newTestCallback() *testCallback {
	return &testCallback{callsCh: make(chan struct{}, 100)}
}

func (tc *testCallback) fn(cfg *Config, errs []error) {
	tc.mu.Lock()
	tc.calls = append(tc.calls, callRecord{cfg: cfg, errs: errs})
	tc.mu.Unlock()
	tc.callsCh <- struct{}{}
}

func (tc *testCallback) waitForCall(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-tc.callsCh:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for callback")
	}
}

func (tc *testCallback) count() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return len(tc.calls)
}

func (tc *testCallback) last() callRecord {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.calls[len(tc.calls)-1]
}

func writeConfigFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
}

const validConfig = `services:
  - name: test-svc
    url: https://example.com
    group: infra
`

const validConfig2 = `services:
  - name: test-svc-2
    url: https://example2.com
    group: apps
`

const malformedYAML = `services:
  - name: [invalid yaml
`

func TestWatcher_FileModificationTriggersCallback(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeConfigFile(t, cfgPath, validConfig)

	cb := newTestCallback()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(cfgPath, cb.fn, logger, WithDebounce(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	writeConfigFile(t, cfgPath, validConfig2)
	cb.waitForCall(t, 2*time.Second)

	rec := cb.last()
	if rec.cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(rec.cfg.Services) != 1 || rec.cfg.Services[0].Name != "test-svc-2" {
		t.Errorf("expected service test-svc-2, got %+v", rec.cfg.Services)
	}
	if len(rec.errs) != 0 {
		t.Errorf("expected no errors, got %v", rec.errs)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

func TestWatcher_DebounceRapidWrites(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeConfigFile(t, cfgPath, validConfig)

	cb := newTestCallback()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(cfgPath, cb.fn, logger, WithDebounce(100*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// Rapid writes — should debounce to one callback
	writeConfigFile(t, cfgPath, validConfig)
	time.Sleep(20 * time.Millisecond)
	writeConfigFile(t, cfgPath, validConfig2)
	time.Sleep(20 * time.Millisecond)
	writeConfigFile(t, cfgPath, validConfig)

	cb.waitForCall(t, 2*time.Second)

	// Wait a bit more to confirm no extra callbacks fire
	time.Sleep(300 * time.Millisecond)

	if count := cb.count(); count != 1 {
		t.Errorf("expected exactly 1 callback (debounced), got %d", count)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

func TestWatcher_FileCreatedAfterStart(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// Do NOT create the file initially

	cb := newTestCallback()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(cfgPath, cb.fn, logger, WithDebounce(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// Create the file
	writeConfigFile(t, cfgPath, validConfig)
	cb.waitForCall(t, 2*time.Second)

	rec := cb.last()
	if rec.cfg == nil {
		t.Fatal("expected non-nil config after file creation")
	}
	if len(rec.cfg.Services) != 1 || rec.cfg.Services[0].Name != "test-svc" {
		t.Errorf("unexpected services: %+v", rec.cfg.Services)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

func TestWatcher_AtomicRenameTriggersCallback(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeConfigFile(t, cfgPath, validConfig)

	cb := newTestCallback()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(cfgPath, cb.fn, logger, WithDebounce(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// Simulate editor atomic save: write temp file, then rename over target.
	tmpPath := filepath.Join(dir, "config.yaml.tmp")
	writeConfigFile(t, tmpPath, validConfig2)
	if err := os.Rename(tmpPath, cfgPath); err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	cb.waitForCall(t, 2*time.Second)
	rec := cb.last()
	if rec.cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(rec.cfg.Services) != 1 || rec.cfg.Services[0].Name != "test-svc-2" {
		t.Errorf("expected service test-svc-2, got %+v", rec.cfg.Services)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

func TestWatcher_ContextCancellationReturnsNil(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeConfigFile(t, cfgPath, validConfig)

	cb := newTestCallback()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(cfgPath, cb.fn, logger, WithDebounce(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Run() should return nil on context cancel, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}

func TestWatcher_CallbackReceivesValidationErrors(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// Config missing required 'url' field
	writeConfigFile(t, cfgPath, `services:
  - name: bad-svc
    group: infra
`)

	cb := newTestCallback()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(cfgPath, cb.fn, logger, WithDebounce(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// Touch the file to trigger reload
	writeConfigFile(t, cfgPath, `services:
  - name: bad-svc
    group: infra
`)
	cb.waitForCall(t, 2*time.Second)

	rec := cb.last()
	if rec.cfg == nil {
		t.Fatal("expected non-nil config (partial parse)")
	}
	if len(rec.errs) == 0 {
		t.Error("expected validation errors for missing url")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

func TestWatcher_MalformedYAMLTriggersCallbackWithNilConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeConfigFile(t, cfgPath, validConfig)

	cb := newTestCallback()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(cfgPath, cb.fn, logger, WithDebounce(50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// Write malformed YAML
	writeConfigFile(t, cfgPath, malformedYAML)
	cb.waitForCall(t, 2*time.Second)

	rec := cb.last()
	if rec.cfg != nil {
		t.Errorf("expected nil config for malformed YAML, got %+v", rec.cfg)
	}
	if len(rec.errs) == 0 {
		t.Error("expected parse error for malformed YAML")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}
