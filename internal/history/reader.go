package history

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/rathix/command-center/internal/state"
)

// StateWriter provides read-modify-write access to the service store for history restoration.
type StateWriter interface {
	Get(namespace, name string) (state.Service, bool)
	Update(namespace, name string, fn func(*state.Service))
}

// PendingHistory holds transition records for services not yet discovered.
// Records are applied when the service first appears via ApplyIfPending.
type PendingHistory struct {
	mu      sync.Mutex
	pending map[string]TransitionRecord
}

// ApplyIfPending checks whether the given service has a pending history record.
// If so, it applies the record to the store and removes it from the pending set.
func (p *PendingHistory) ApplyIfPending(store StateWriter, namespace, name string) {
	key := namespace + "/" + name
	p.mu.Lock()
	rec, ok := p.pending[key]
	p.mu.Unlock()

	if !ok {
		return
	}

	if _, exists := store.Get(namespace, name); !exists {
		// Service is still not present; keep record pending.
		return
	}

	ts := rec.Timestamp
	status := rec.NextStatus
	store.Update(namespace, name, func(svc *state.Service) {
		// Only restore if we haven't performed a fresh health check yet.
		if svc.LastChecked == nil {
			svc.LastStateChange = &ts
			svc.Status = status
		}
	})

	if _, exists := store.Get(namespace, name); !exists {
		// Service disappeared before update could apply; keep it pending.
		return
	}

	p.mu.Lock()
	delete(p.pending, key)
	p.mu.Unlock()
}

// ReadHistory reads a JSONL history file and returns the latest TransitionRecord per service key.
// If the file does not exist, it returns an empty map with no error.
func ReadHistory(path string) (map[string]TransitionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]TransitionRecord), nil
		}
		return nil, err
	}
	defer f.Close()

	return readRecords(f, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func readRecords(r io.Reader, logger *slog.Logger) (map[string]TransitionRecord, error) {
	records := make(map[string]TransitionRecord)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var rec TransitionRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			logger.Debug("skipping malformed history line", "error", err)
			continue
		}

		if existing, ok := records[rec.ServiceKey]; !ok || rec.Timestamp.After(existing.Timestamp) {
			records[rec.ServiceKey] = rec
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

// RestoreHistory applies history records to services already in the store.
// Services not yet discovered are returned in a PendingHistory for later application.
func RestoreHistory(store StateWriter, records map[string]TransitionRecord, logger *slog.Logger) *PendingHistory {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	pending := &PendingHistory{
		pending: make(map[string]TransitionRecord),
	}

	restored := 0
	for key, rec := range records {
		namespace, name, ok := splitServiceKey(key)
		if !ok {
			logger.Debug("skipping record with invalid service key", "key", key)
			continue
		}

		if _, exists := store.Get(namespace, name); exists {
			ts := rec.Timestamp
			status := rec.NextStatus
			store.Update(namespace, name, func(svc *state.Service) {
				// Only restore if we haven't performed a fresh health check yet.
				if svc.LastChecked == nil {
					svc.LastStateChange = &ts
					svc.Status = status
				}
			})
			restored++
		} else {
			pending.pending[key] = rec
		}
	}

	logger.Info("history restoration complete",
		"restored", restored,
		"pending", len(pending.pending),
	)

	return pending
}

// splitServiceKey splits "namespace/name" into its components.
// Returns false if the key does not contain a slash.
func splitServiceKey(key string) (namespace, name string, ok bool) {
	idx := strings.Index(key, "/")
	if idx <= 0 || idx >= len(key)-1 {
		return "", "", false
	}
	if strings.Contains(key[idx+1:], "/") {
		return "", "", false
	}
	return key[:idx], key[idx+1:], true
}
