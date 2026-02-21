package history

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

const defaultRetentionDays = 30

// ReadAllRecords reads all valid TransitionRecords from a JSONL file.
// Malformed and blank lines are skipped. Returns an empty slice if the
// file is missing or empty.
func ReadAllRecords(path string) ([]TransitionRecord, error) {
	records, _, err := parseHistoryFile(path)
	return records, err
}

func parseHistoryFile(path string) ([]TransitionRecord, int, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	defer f.Close()

	var records []TransitionRecord
	nonEmptyLines := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		nonEmptyLines++

		var rec TransitionRecord
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}
	return records, nonEmptyLines, nil
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

	records, nonEmptyLines, err := parseHistoryFile(path)
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

	oldRemoved := len(records) - len(kept)
	malformedRemoved := nonEmptyLines - len(records)
	removed := oldRemoved + malformedRemoved
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
		"removedOld", oldRemoved,
		"removedMalformed", malformedRemoved,
	)
	return nil
}

// Pruner runs periodic history file pruning.
type Pruner struct {
	path          string
	retentionDays int
	logger        *slog.Logger
	writer        *FileWriter
}

// NewPruner creates a Pruner that will prune the file at path, removing
// records older than retentionDays. If writer is non-nil, the pruner
// coordinates with it to prevent writes to a stale file descriptor
// after the atomic rename.
func NewPruner(path string, retentionDays int, writer *FileWriter, logger *slog.Logger) *Pruner {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Pruner{
		path:          path,
		retentionDays: retentionDays,
		logger:        logger,
		writer:        writer,
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
	if p.writer != nil {
		p.writer.mu.Lock()
		defer p.writer.mu.Unlock()
	}
	if err := Prune(p.path, p.retentionDays, p.logger); err != nil {
		p.logger.Warn("history prune failed", "error", err)
		return
	}
	if p.writer != nil {
		if err := p.writer.reopen(); err != nil {
			p.logger.Warn("history writer reopen after prune failed", "error", err)
		}
	}
}
