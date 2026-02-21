package history

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"time"
)

const defaultRetentionDays = 30

// ReadAllRecords reads all valid TransitionRecords from a JSONL file.
// Malformed and blank lines are skipped. Returns an empty slice if the
// file is missing or empty.
func ReadAllRecords(path string) ([]TransitionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []TransitionRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec TransitionRecord
		if json.Unmarshal(line, &rec) != nil {
			continue
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return records, err
	}
	return records, nil
}

// Prune removes records older than retentionDays from the JSONL file at path.
// It uses an atomic temp-file-and-rename strategy to avoid data loss. If
// retentionDays is <= 0 the default of 30 days is used.
func Prune(path string, retentionDays int, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if retentionDays <= 0 {
		retentionDays = defaultRetentionDays
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

	removed := len(records) - len(kept)
	if removed == 0 {
		logger.Info("history prune: nothing to remove",
			"total", len(records),
		)
		return nil
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	for _, rec := range kept {
		data, merr := json.Marshal(rec)
		if merr != nil {
			f.Close()
			os.Remove(tmpPath)
			return merr
		}
		data = append(data, '\n')
		if _, werr := f.Write(data); werr != nil {
			f.Close()
			os.Remove(tmpPath)
			return werr
		}
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	logger.Info("history prune complete",
		"before", len(records),
		"after", len(kept),
		"removed", removed,
	)
	return nil
}

// Pruner runs periodic history file pruning.
type Pruner struct {
	path          string
	retentionDays int
	logger        *slog.Logger
}

// NewPruner creates a Pruner that will prune the file at path, removing
// records older than retentionDays.
func NewPruner(path string, retentionDays int, logger *slog.Logger) *Pruner {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Pruner{
		path:          path,
		retentionDays: retentionDays,
		logger:        logger,
	}
}

// Run executes Prune immediately, then repeats every 24 hours until
// ctx is cancelled.
func (p *Pruner) Run(ctx context.Context) {
	p.runOnce()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.runOnce()
		}
	}
}

func (p *Pruner) runOnce() {
	if err := Prune(p.path, p.retentionDays, p.logger); err != nil {
		p.logger.Warn("history prune failed", "error", err)
	}
}
