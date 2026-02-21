package config

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ReloadCallback is invoked when the config file changes.
// cfg is the newly parsed config (nil on parse failure).
// errs contains any validation or parse errors.
type ReloadCallback func(cfg *Config, errs []error)

// Watcher monitors a config file for changes and triggers reloads.
type Watcher struct {
	path     string
	callback ReloadCallback
	logger   *slog.Logger
	debounce time.Duration
}

// WatcherOption configures a Watcher.
type WatcherOption func(*Watcher)

// WithDebounce sets the debounce duration. Default is 1 second.
func WithDebounce(d time.Duration) WatcherOption {
	return func(w *Watcher) {
		w.debounce = d
	}
}

// NewWatcher creates a config file watcher.
func NewWatcher(path string, callback ReloadCallback, logger *slog.Logger, opts ...WatcherOption) *Watcher {
	w := &Watcher{
		path:     path,
		callback: callback,
		logger:   logger,
		debounce: time.Second,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Run watches the config file's parent directory for changes and invokes
// the callback on debounced write/create events. It blocks until ctx is
// cancelled, then returns nil.
func (w *Watcher) Run(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	// Watch the parent directory to catch atomic write patterns (vim, VS Code).
	dir := filepath.Dir(w.path)
	if err := fsw.Add(dir); err != nil {
		return err
	}

	targetName := filepath.Base(w.path)
	reloadCh := make(chan struct{}, 1)
	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return nil

		case event, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			if filepath.Base(event.Name) != targetName {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			// Reset debounce timer
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(w.debounce, func() {
				select {
				case reloadCh <- struct{}{}:
				default:
				}
			})

		case <-reloadCh:
			cfg, errs := Load(w.path)
			w.callback(cfg, errs)

		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			w.logger.Warn("fsnotify error", "error", err)
		}
	}
}
