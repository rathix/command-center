package history

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/state"
)

func writeRecords(t *testing.T, path string, records []TransitionRecord) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, rec := range records {
		data, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
		f.Write([]byte("\n"))
	}
}

func makeRecord(daysAgo int, svc string) TransitionRecord {
	return TransitionRecord{
		Timestamp:  time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour),
		ServiceKey: svc,
		PrevStatus: state.StatusHealthy,
		NextStatus: state.StatusUnhealthy,
	}
}

func TestReadAllRecords(t *testing.T) {
	t.Parallel()

	t.Run("returns all records not just latest per service", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "history.jsonl")

		records := []TransitionRecord{
			makeRecord(5, "svc-a"),
			makeRecord(3, "svc-a"),
			makeRecord(1, "svc-a"),
			makeRecord(2, "svc-b"),
		}
		writeRecords(t, path, records)

		got, err := ReadAllRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 4 {
			t.Fatalf("expected 4 records, got %d", len(got))
		}
	})

	t.Run("missing file returns empty slice", func(t *testing.T) {
		t.Parallel()
		got, err := ReadAllRecords(filepath.Join(t.TempDir(), "nonexistent.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("expected 0 records, got %d", len(got))
		}
	})

	t.Run("empty file returns empty slice", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.jsonl")
		os.WriteFile(path, nil, 0644)

		got, err := ReadAllRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("expected 0 records, got %d", len(got))
		}
	})

	t.Run("skips malformed and blank lines", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "mixed.jsonl")

		rec := makeRecord(1, "svc-a")
		data, _ := json.Marshal(rec)
		content := string(data) + "\n" +
			"not json\n" +
			"\n" +
			string(data) + "\n"
		os.WriteFile(path, []byte(content), 0644)

		got, err := ReadAllRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 records, got %d", len(got))
		}
	})
}

func TestPrune(t *testing.T) {
	t.Parallel()

	t.Run("removes old records keeps new ones", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "history.jsonl")

		records := []TransitionRecord{
			makeRecord(60, "svc-old"),
			makeRecord(45, "svc-also-old"),
			makeRecord(10, "svc-recent"),
			makeRecord(1, "svc-new"),
		}
		writeRecords(t, path, records)

		err := Prune(path, 30, nil)
		if err != nil {
			t.Fatal(err)
		}

		got, err := ReadAllRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 records after prune, got %d", len(got))
		}
		if got[0].ServiceKey != "svc-recent" {
			t.Errorf("expected svc-recent, got %s", got[0].ServiceKey)
		}
		if got[1].ServiceKey != "svc-new" {
			t.Errorf("expected svc-new, got %s", got[1].ServiceKey)
		}
	})

	t.Run("custom retention days", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "history.jsonl")

		records := []TransitionRecord{
			makeRecord(10, "svc-old-for-7d"),
			makeRecord(5, "svc-recent"),
			makeRecord(1, "svc-new"),
		}
		writeRecords(t, path, records)

		err := Prune(path, 7, nil)
		if err != nil {
			t.Fatal(err)
		}

		got, err := ReadAllRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 records after 7-day prune, got %d", len(got))
		}
	})

	t.Run("default 30 day retention when retentionDays zero", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "history.jsonl")

		records := []TransitionRecord{
			makeRecord(60, "svc-old"),
			makeRecord(10, "svc-recent"),
		}
		writeRecords(t, path, records)

		err := Prune(path, 0, nil)
		if err != nil {
			t.Fatal(err)
		}

		got, err := ReadAllRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 record with default retention, got %d", len(got))
		}
		if got[0].ServiceKey != "svc-recent" {
			t.Errorf("expected svc-recent, got %s", got[0].ServiceKey)
		}
	})

	t.Run("default 30 day retention when retentionDays negative", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "history.jsonl")

		records := []TransitionRecord{
			makeRecord(60, "svc-old"),
			makeRecord(10, "svc-recent"),
		}
		writeRecords(t, path, records)

		err := Prune(path, -1, nil)
		if err != nil {
			t.Fatal(err)
		}

		got, err := ReadAllRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 record with default retention, got %d", len(got))
		}
	})

	t.Run("no records removed skips rewrite", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "history.jsonl")

		records := []TransitionRecord{
			makeRecord(1, "svc-new"),
			makeRecord(5, "svc-recent"),
		}
		writeRecords(t, path, records)

		info1, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}

		// Sleep briefly so mod time would differ if file were rewritten.
		time.Sleep(50 * time.Millisecond)

		err = Prune(path, 30, nil)
		if err != nil {
			t.Fatal(err)
		}

		info2, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}

		if !info1.ModTime().Equal(info2.ModTime()) {
			t.Error("file was rewritten despite no records being removed")
		}
	})

	t.Run("empty file produces no error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.jsonl")
		os.WriteFile(path, nil, 0644)

		err := Prune(path, 30, nil)
		if err != nil {
			t.Fatalf("expected no error for empty file, got %v", err)
		}
	})

	t.Run("missing file produces no error", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "nonexistent.jsonl")

		err := Prune(path, 30, nil)
		if err != nil {
			t.Fatalf("expected no error for missing file, got %v", err)
		}
	})

	t.Run("malformed lines are dropped during prune", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "malformed.jsonl")

		rec := makeRecord(1, "svc-good")
		old := makeRecord(60, "svc-old")
		goodData, _ := json.Marshal(rec)
		oldData, _ := json.Marshal(old)
		content := string(goodData) + "\n" +
			"garbage line\n" +
			"{bad json\n" +
			string(oldData) + "\n"
		os.WriteFile(path, []byte(content), 0644)

		err := Prune(path, 30, nil)
		if err != nil {
			t.Fatal(err)
		}

		got, err := ReadAllRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 record after prune (malformed dropped, old removed), got %d", len(got))
		}
		if got[0].ServiceKey != "svc-good" {
			t.Errorf("expected svc-good, got %s", got[0].ServiceKey)
		}
	})

	t.Run("atomic write produces correct file contents", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "history.jsonl")

		records := []TransitionRecord{
			makeRecord(60, "svc-old"),
			makeRecord(5, "svc-a"),
			makeRecord(2, "svc-b"),
		}
		writeRecords(t, path, records)

		err := Prune(path, 30, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Verify no temp file left behind.
		tmpPath := path + ".tmp"
		if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
			t.Error("temp file was not cleaned up after rename")
		}

		got, err := ReadAllRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 records, got %d", len(got))
		}
	})
}

func TestPrunerRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	records := []TransitionRecord{
		makeRecord(60, "svc-old"),
		makeRecord(1, "svc-new"),
	}
	writeRecords(t, path, records)

	ctx, cancel := context.WithCancel(context.Background())

	pruner := NewPruner(path, 30, nil)

	done := make(chan struct{})
	go func() {
		pruner.Run(ctx)
		close(done)
	}()

	// Give Run time to execute the immediate prune.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Pruner.Run did not exit after context cancellation")
	}

	got, err := ReadAllRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 record after pruner run, got %d", len(got))
	}
	if got[0].ServiceKey != "svc-new" {
		t.Errorf("expected svc-new, got %s", got[0].ServiceKey)
	}
}
