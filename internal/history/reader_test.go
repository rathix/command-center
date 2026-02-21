package history

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/state"
)

// mockStateWriter implements StateWriter for testing.
type mockStateWriter struct {
	mu       sync.Mutex
	services map[string]state.Service
}

func newMockStateWriter() *mockStateWriter {
	return &mockStateWriter{services: make(map[string]state.Service)}
}

func (m *mockStateWriter) Add(svc state.Service) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services[svc.Namespace+"/"+svc.Name] = svc
}

func (m *mockStateWriter) Get(namespace, name string) (state.Service, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	svc, ok := m.services[namespace+"/"+name]
	return svc, ok
}

func (m *mockStateWriter) Update(namespace, name string, fn func(*state.Service)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := namespace + "/" + name
	svc, ok := m.services[key]
	if !ok {
		return
	}
	fn(&svc)
	m.services[key] = svc
}

func TestReadHistory_ValidJSONL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	ts1 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)
	content := `{"ts":"2025-01-01T10:00:00Z","svc":"default/svc-a","prev":"unknown","next":"healthy","code":200,"ms":50}
{"ts":"2025-01-01T11:00:00Z","svc":"kube-system/svc-b","prev":"healthy","next":"unhealthy","code":503,"ms":120}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	records, err := ReadHistory(path)
	if err != nil {
		t.Fatalf("ReadHistory returned error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	recA := records["default/svc-a"]
	if !recA.Timestamp.Equal(ts1) {
		t.Errorf("svc-a timestamp = %v, want %v", recA.Timestamp, ts1)
	}
	if recA.NextStatus != state.StatusHealthy {
		t.Errorf("svc-a next status = %q, want %q", recA.NextStatus, state.StatusHealthy)
	}

	recB := records["kube-system/svc-b"]
	if !recB.Timestamp.Equal(ts2) {
		t.Errorf("svc-b timestamp = %v, want %v", recB.Timestamp, ts2)
	}
	if recB.NextStatus != state.StatusUnhealthy {
		t.Errorf("svc-b next status = %q, want %q", recB.NextStatus, state.StatusUnhealthy)
	}
}

func TestReadHistory_LatestRecordWins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	content := `{"ts":"2025-01-01T10:00:00Z","svc":"default/svc-a","prev":"unknown","next":"healthy"}
{"ts":"2025-01-01T12:00:00Z","svc":"default/svc-a","prev":"healthy","next":"unhealthy"}
{"ts":"2025-01-01T11:00:00Z","svc":"default/svc-a","prev":"unhealthy","next":"healthy"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	records, err := ReadHistory(path)
	if err != nil {
		t.Fatalf("ReadHistory returned error: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records["default/svc-a"]
	expected := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	if !rec.Timestamp.Equal(expected) {
		t.Errorf("timestamp = %v, want %v (latest)", rec.Timestamp, expected)
	}
	if rec.NextStatus != state.StatusUnhealthy {
		t.Errorf("next status = %q, want %q", rec.NextStatus, state.StatusUnhealthy)
	}
}

func TestReadHistory_MalformedLinesSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	content := `not json at all
{"ts":"2025-01-01T10:00:00Z","svc":"default/svc-a","prev":"unknown","next":"healthy"}
{broken json
{"ts":"2025-01-01T11:00:00Z","svc":"default/svc-b","prev":"unknown","next":"unhealthy"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	records, err := ReadHistory(path)
	if err != nil {
		t.Fatalf("ReadHistory returned error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 valid records, got %d", len(records))
	}

	if _, ok := records["default/svc-a"]; !ok {
		t.Error("expected record for default/svc-a")
	}
	if _, ok := records["default/svc-b"]; !ok {
		t.Error("expected record for default/svc-b")
	}
}

func TestReadHistory_MissingFile(t *testing.T) {
	t.Parallel()
	records, err := ReadHistory("/nonexistent/path/history.jsonl")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected empty map, got %d records", len(records))
	}
}

func TestReadHistory_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	records, err := ReadHistory(path)
	if err != nil {
		t.Fatalf("ReadHistory returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected empty map, got %d records", len(records))
	}
}

func TestRestoreHistory_SetsExistingServices(t *testing.T) {
	t.Parallel()
	store := newMockStateWriter()
	store.Add(state.Service{
		Name:      "svc-a",
		Namespace: "default",
		Status:    state.StatusUnknown,
	})

	ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	records := map[string]TransitionRecord{
		"default/svc-a": {
			Timestamp:  ts,
			ServiceKey: "default/svc-a",
			PrevStatus: state.StatusUnknown,
			NextStatus: state.StatusHealthy,
		},
	}

	pending := RestoreHistory(store, records, nil)

	svc, ok := store.Get("default", "svc-a")
	if !ok {
		t.Fatal("service not found after restore")
	}
	if svc.Status != state.StatusHealthy {
		t.Errorf("status = %q, want %q", svc.Status, state.StatusHealthy)
	}
	if svc.LastStateChange == nil {
		t.Fatal("LastStateChange is nil")
	}
	if !svc.LastStateChange.Equal(ts) {
		t.Errorf("LastStateChange = %v, want %v", *svc.LastStateChange, ts)
	}
	if len(pending.pending) != 0 {
		t.Errorf("expected 0 pending, got %d", len(pending.pending))
	}
}

func TestRestoreHistory_HoldsPendingForMissingServices(t *testing.T) {
	t.Parallel()
	store := newMockStateWriter()

	ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	records := map[string]TransitionRecord{
		"default/svc-missing": {
			Timestamp:  ts,
			ServiceKey: "default/svc-missing",
			PrevStatus: state.StatusUnknown,
			NextStatus: state.StatusUnhealthy,
		},
	}

	pending := RestoreHistory(store, records, nil)

	if len(pending.pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending.pending))
	}
	rec, ok := pending.pending["default/svc-missing"]
	if !ok {
		t.Fatal("expected pending record for default/svc-missing")
	}
	if rec.NextStatus != state.StatusUnhealthy {
		t.Errorf("pending next status = %q, want %q", rec.NextStatus, state.StatusUnhealthy)
	}
}

func TestApplyIfPending_AppliesRecord(t *testing.T) {
	t.Parallel()
	store := newMockStateWriter()
	store.Add(state.Service{
		Name:      "svc-a",
		Namespace: "default",
		Status:    state.StatusUnknown,
	})

	ts := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)
	pending := &PendingHistory{
		pending: map[string]TransitionRecord{
			"default/svc-a": {
				Timestamp:  ts,
				ServiceKey: "default/svc-a",
				PrevStatus: state.StatusUnknown,
				NextStatus: state.StatusHealthy,
			},
		},
	}

	pending.ApplyIfPending(store, "default", "svc-a")

	svc, ok := store.Get("default", "svc-a")
	if !ok {
		t.Fatal("service not found after ApplyIfPending")
	}
	if svc.Status != state.StatusHealthy {
		t.Errorf("status = %q, want %q", svc.Status, state.StatusHealthy)
	}
	if svc.LastStateChange == nil || !svc.LastStateChange.Equal(ts) {
		t.Errorf("LastStateChange = %v, want %v", svc.LastStateChange, ts)
	}
	if len(pending.pending) != 0 {
		t.Errorf("expected pending to be empty after apply, got %d", len(pending.pending))
	}
}

func TestApplyIfPending_NoopForNonPending(t *testing.T) {
	t.Parallel()
	store := newMockStateWriter()
	store.Add(state.Service{
		Name:      "svc-a",
		Namespace: "default",
		Status:    state.StatusUnknown,
	})

	pending := &PendingHistory{
		pending: make(map[string]TransitionRecord),
	}

	pending.ApplyIfPending(store, "default", "svc-a")

	svc, _ := store.Get("default", "svc-a")
	if svc.Status != state.StatusUnknown {
		t.Errorf("status should remain %q, got %q", state.StatusUnknown, svc.Status)
	}
	if svc.LastStateChange != nil {
		t.Error("LastStateChange should remain nil for non-pending service")
	}
}

func TestApplyIfPending_KeepsRecordWhenServiceStillMissing(t *testing.T) {
	t.Parallel()
	store := newMockStateWriter()

	ts := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)
	pending := &PendingHistory{
		pending: map[string]TransitionRecord{
			"default/svc-missing": {
				Timestamp:  ts,
				ServiceKey: "default/svc-missing",
				PrevStatus: state.StatusUnknown,
				NextStatus: state.StatusHealthy,
			},
		},
	}

	pending.ApplyIfPending(store, "default", "svc-missing")

	if len(pending.pending) != 1 {
		t.Fatalf("expected pending record to remain, got %d records", len(pending.pending))
	}
}

func TestSplitServiceKey_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key        string
		wantNS     string
		wantName   string
		wantParsed bool
	}{
		{key: "default/svc-a", wantNS: "default", wantName: "svc-a", wantParsed: true},
		{key: "/svc-a", wantParsed: false},
		{key: "default/", wantParsed: false},
		{key: "default/svc/a", wantParsed: false},
		{key: "noslash", wantParsed: false},
	}

	for _, tt := range tests {
		ns, name, ok := splitServiceKey(tt.key)
		if ok != tt.wantParsed {
			t.Fatalf("splitServiceKey(%q) ok=%v, want %v", tt.key, ok, tt.wantParsed)
		}
		if ok {
			if ns != tt.wantNS || name != tt.wantName {
				t.Fatalf("splitServiceKey(%q) = (%q,%q), want (%q,%q)", tt.key, ns, name, tt.wantNS, tt.wantName)
			}
		}
	}
}
