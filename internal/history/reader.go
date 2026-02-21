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

type ReaderStateWriter interface {
	Get(namespace, name string) (state.Service, bool)
	Update(namespace, name string, fn func(*state.Service))
}

func ReadHistory(path string) (map[string]TransitionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]TransitionRecord), nil
		}
		return nil, err
	}
	defer f.Close()

	records := make(map[string]TransitionRecord)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec TransitionRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if existing, ok := records[rec.ServiceKey]; !ok || rec.Timestamp.After(existing.Timestamp) {
			records[rec.ServiceKey] = rec
		}
	}
	return records, scanner.Err()
}

func RestoreHistory(store ReaderStateWriter, records map[string]TransitionRecord, logger *slog.Logger) *PendingHistory {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	pending := &PendingHistory{records: make(map[string]TransitionRecord)}
	restored := 0
	for key, rec := range records {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		ns, name := parts[0], parts[1]
		if _, ok := store.Get(ns, name); ok {
			rec := rec
			store.Update(ns, name, func(svc *state.Service) {
				ts := rec.Timestamp
				svc.LastStateChange = &ts
				svc.Status = rec.NextStatus
			})
			restored++
		} else {
			pending.mu.Lock()
			pending.records[key] = rec
			pending.mu.Unlock()
		}
	}
	logger.Info("history restored", "restored", restored, "pending", len(pending.records))
	return pending
}

type PendingHistory struct {
	mu      sync.Mutex
	records map[string]TransitionRecord
}

func (p *PendingHistory) ApplyIfPending(store ReaderStateWriter, namespace, name string) {
	key := namespace + "/" + name
	p.mu.Lock()
	rec, ok := p.records[key]
	if ok {
		delete(p.records, key)
	}
	p.mu.Unlock()
	if !ok {
		return
	}
	store.Update(namespace, name, func(svc *state.Service) {
		ts := rec.Timestamp
		svc.LastStateChange = &ts
		svc.Status = rec.NextStatus
	})
}
