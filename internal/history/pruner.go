package history

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

func ReadAllRecords(path string) ([]TransitionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []TransitionRecord
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
		records = append(records, rec)
	}
	return records, scanner.Err()
}

func Prune(path string, retentionDays int, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}
	records, err := ReadAllRecords(path)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	var kept []TransitionRecord
	for _, rec := range records {
		if !rec.Timestamp.Before(cutoff) {
			kept = append(kept, rec)
		}
	}
	if len(kept) == len(records) {
		return nil
	}
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	for _, rec := range kept {
		data, err := json.Marshal(rec)
		if err != nil {
			f.Close()
			os.Remove(tmpPath)
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return err
		}
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	f.Close()
	logger.Info("history pruned", "before", len(records), "after", len(kept), "removed", len(records)-len(kept))
	return os.Rename(tmpPath, path)
}

type Pruner struct {
	path          string
	retentionDays int
	logger        *slog.Logger
}

func NewPruner(path string, retentionDays int, logger *slog.Logger) *Pruner {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Pruner{path: path, retentionDays: retentionDays, logger: logger}
}

func (p *Pruner) Run(ctx context.Context) {
	if err := Prune(p.path, p.retentionDays, p.logger); err != nil {
		p.logger.Warn("history prune failed", "error", err)
	}
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := Prune(p.path, p.retentionDays, p.logger); err != nil {
				p.logger.Warn("history prune failed", "error", err)
			}
		}
	}
}
