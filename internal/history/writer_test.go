package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/state"
)

func ptrInt(v int) *int       { return &v }
func ptrInt64(v int64) *int64 { return &v }

func sampleRecord() TransitionRecord {
	return TransitionRecord{
		Timestamp:  time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
		ServiceKey: "default/my-service",
		PrevStatus: state.StatusHealthy,
		NextStatus: state.StatusUnhealthy,
		HTTPCode:   ptrInt(503),
		ResponseMs: ptrInt64(42),
	}
}

func TestFileWriter_RecordWritesValidJSONL(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history.jsonl")
	w, err := NewFileWriter(path, nil)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}
	defer w.Close()

	rec := sampleRecord()
	if err := w.Record(rec); err != nil {
		t.Fatalf("Record: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got TransitionRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ServiceKey != "default/my-service" {
		t.Errorf("svc = %q, want %q", got.ServiceKey, "default/my-service")
	}
	if got.PrevStatus != state.StatusHealthy {
		t.Errorf("prev = %q, want %q", got.PrevStatus, state.StatusHealthy)
	}
	if got.NextStatus != state.StatusUnhealthy {
		t.Errorf("next = %q, want %q", got.NextStatus, state.StatusUnhealthy)
	}
	if got.HTTPCode == nil || *got.HTTPCode != 503 {
		t.Errorf("code = %v, want 503", got.HTTPCode)
	}
	if got.ResponseMs == nil || *got.ResponseMs != 42 {
		t.Errorf("ms = %v, want 42", got.ResponseMs)
	}
}

func TestFileWriter_TimestampISO8601UTC(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history.jsonl")
	w, err := NewFileWriter(path, nil)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}
	defer w.Close()

	rec := sampleRecord()
	if err := w.Record(rec); err != nil {
		t.Fatalf("Record: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	tsStr := strings.Trim(string(raw["ts"]), `"`)
	parsed, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		t.Fatalf("timestamp %q is not valid RFC3339: %v", tsStr, err)
	}
	if !parsed.Equal(rec.Timestamp) {
		t.Errorf("parsed timestamp = %v, want %v", parsed, rec.Timestamp)
	}
}

func TestFileWriter_CreatesFileIfMissing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "subdir", "history.jsonl")

	// Create the parent directory so OpenFile can create the file.
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	w, err := NewFileWriter(path, nil)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}
	defer w.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file was not created: %v", err)
	}
}

func TestFileWriter_MultipleRecordsAppend(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history.jsonl")
	w, err := NewFileWriter(path, nil)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}
	defer w.Close()

	for i := range 3 {
		rec := sampleRecord()
		rec.HTTPCode = ptrInt(500 + i)
		if err := w.Record(rec); err != nil {
			t.Fatalf("Record[%d]: %v", i, err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}

	for i, line := range lines {
		var got TransitionRecord
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("Unmarshal line %d: %v", i, err)
		}
		wantCode := 500 + i
		if got.HTTPCode == nil || *got.HTTPCode != wantCode {
			t.Errorf("line %d: code = %v, want %d", i, got.HTTPCode, wantCode)
		}
	}
}

func TestFileWriter_ConcurrentWritesNoCorruption(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history.jsonl")
	w, err := NewFileWriter(path, nil)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}
	defer w.Close()

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(idx int) {
			defer wg.Done()
			rec := sampleRecord()
			rec.HTTPCode = ptrInt(idx)
			if err := w.Record(rec); err != nil {
				t.Errorf("Record[%d]: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != numGoroutines {
		t.Fatalf("got %d lines, want %d", len(lines), numGoroutines)
	}

	for i, line := range lines {
		var got TransitionRecord
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestFileWriter_NilOptionalFields(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history.jsonl")
	w, err := NewFileWriter(path, nil)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}
	defer w.Close()

	rec := TransitionRecord{
		Timestamp:  time.Now().UTC(),
		ServiceKey: "kube-system/coredns",
		PrevStatus: state.StatusUnknown,
		NextStatus: state.StatusHealthy,
		HTTPCode:   nil,
		ResponseMs: nil,
	}
	if err := w.Record(rec); err != nil {
		t.Fatalf("Record: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if string(raw["code"]) != "null" {
		t.Errorf("code = %s, want null", raw["code"])
	}
	if string(raw["ms"]) != "null" {
		t.Errorf("ms = %s, want null", raw["ms"])
	}
}

func TestFileWriter_Close(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history.jsonl")
	w, err := NewFileWriter(path, nil)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Writing after close should fail.
	if err := w.Record(sampleRecord()); err == nil {
		t.Error("expected error writing to closed file, got nil")
	}
}

func TestNoopWriter_ImplementsInterface(t *testing.T) {
	t.Parallel()

	var w HistoryWriter = NoopWriter{}

	if err := w.Record(sampleRecord()); err != nil {
		t.Errorf("Record: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestFileWriter_JSONFieldNames(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history.jsonl")
	w, err := NewFileWriter(path, nil)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}
	defer w.Close()

	if err := w.Record(sampleRecord()); err != nil {
		t.Fatalf("Record: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	expectedKeys := []string{"ts", "svc", "prev", "next", "code", "ms"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing expected JSON field %q", key)
		}
	}

	if len(raw) != len(expectedKeys) {
		t.Errorf("got %d JSON fields, want %d", len(raw), len(expectedKeys))
	}
}

func TestFileWriter_CreatesParentDirsAutomatically(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "deeper", "history.jsonl")
	w, err := NewFileWriter(path, nil)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}
	defer w.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected history file to exist, got stat error: %v", err)
	}
}

func TestFileWriter_NewFileHasOwnerOnlyPermissions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history.jsonl")
	w, err := NewFileWriter(path, nil)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}
	defer w.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("file mode = %o, want 600", got)
	}
}
