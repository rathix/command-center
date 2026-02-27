package gitops

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/state"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// mockUpdater records calls to Update for testing.
type mockUpdater struct {
	mu    sync.Mutex
	calls []mockUpdateCall
}

type mockUpdateCall struct {
	Namespace string
	Name      string
	Status    *state.GitOpsStatus
}

func (m *mockUpdater) Update(namespace, name string, fn func(*state.Service)) {
	svc := &state.Service{Namespace: namespace, Name: name}
	fn(svc)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockUpdateCall{
		Namespace: namespace,
		Name:      name,
		Status:    svc.GitOpsStatus,
	})
}

func (m *mockUpdater) All() []state.Service {
	return nil
}

func (m *mockUpdater) getCalls() []mockUpdateCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]mockUpdateCall, len(m.calls))
	copy(result, m.calls)
	return result
}

func TestMapFluxConditionToState(t *testing.T) {
	tests := []struct {
		name       string
		suspended  bool
		conditions []fluxCondition
		want       ReconcileResult
	}{
		{
			name:      "suspended overrides conditions",
			suspended: true,
			conditions: []fluxCondition{
				{Type: "Ready", Status: "True", Reason: "", Message: "Applied revision"},
			},
			want: ReconcileResult{State: state.ReconcilSuspended, Message: "reconciliation suspended"},
		},
		{
			name: "Ready=True -> synced",
			conditions: []fluxCondition{
				{Type: "Ready", Status: "True", Reason: "", Message: "Applied revision: main@sha1:abc123"},
			},
			want: ReconcileResult{State: state.ReconcilSynced, Message: "Applied revision: main@sha1:abc123"},
		},
		{
			name: "Ready=Unknown with Progressing -> progressing",
			conditions: []fluxCondition{
				{Type: "Ready", Status: "Unknown", Reason: "Progressing", Message: "Reconciling"},
			},
			want: ReconcileResult{State: state.ReconcilProgressing, Message: "Reconciling"},
		},
		{
			name: "Ready=False -> failed",
			conditions: []fluxCondition{
				{Type: "Ready", Status: "False", Reason: "ValidationFailed", Message: "kustomization path not found"},
			},
			want: ReconcileResult{State: state.ReconcilFailed, Message: "kustomization path not found"},
		},
		{
			name:       "no conditions -> progressing",
			conditions: nil,
			want:       ReconcileResult{State: state.ReconcilProgressing, Message: "awaiting first reconciliation"},
		},
		{
			name: "Ready=Unknown without Progressing reason -> progressing",
			conditions: []fluxCondition{
				{Type: "Ready", Status: "Unknown", Reason: "SomeOther", Message: "doing something"},
			},
			want: ReconcileResult{State: state.ReconcilProgressing, Message: "doing something"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapFluxConditionToState(tt.suspended, tt.conditions)
			if got.State != tt.want.State {
				t.Errorf("State = %q, want %q", got.State, tt.want.State)
			}
			if got.Message != tt.want.Message {
				t.Errorf("Message = %q, want %q", got.Message, tt.want.Message)
			}
		})
	}
}

func TestExtractFluxStatus_Kustomization(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata": map[string]interface{}{
				"name":      "my-app",
				"namespace": "flux-system",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "True",
						"reason":             "ReconciliationSucceeded",
						"message":            "Applied revision: main@sha1:abc123",
						"lastTransitionTime": "2026-02-27T10:00:00Z",
					},
				},
			},
		},
	}

	result := extractFluxStatus(obj)
	if result.Name != "my-app" {
		t.Errorf("Name = %q, want %q", result.Name, "my-app")
	}
	if result.Namespace != "flux-system" {
		t.Errorf("Namespace = %q, want %q", result.Namespace, "flux-system")
	}
	if result.Result.State != state.ReconcilSynced {
		t.Errorf("State = %q, want %q", result.Result.State, state.ReconcilSynced)
	}
	if result.TransitionTime == nil {
		t.Error("expected non-nil TransitionTime")
	}
}

func TestExtractFluxStatus_HelmReleaseFailed(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name":      "my-helm-app",
				"namespace": "flux-system",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "False",
						"reason":             "InstallFailed",
						"message":            "install retries exhausted",
						"lastTransitionTime": "2026-02-27T09:00:00Z",
					},
				},
			},
		},
	}

	result := extractFluxStatus(obj)
	if result.Result.State != state.ReconcilFailed {
		t.Errorf("State = %q, want %q", result.Result.State, state.ReconcilFailed)
	}
	if result.Result.Message != "install retries exhausted" {
		t.Errorf("Message = %q, want %q", result.Result.Message, "install retries exhausted")
	}
}

func TestExtractFluxStatus_Suspended(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata": map[string]interface{}{
				"name":      "suspended-app",
				"namespace": "flux-system",
			},
			"spec": map[string]interface{}{
				"suspend": true,
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "True",
						"reason":  "ReconciliationSucceeded",
						"message": "Applied revision",
					},
				},
			},
		},
	}

	result := extractFluxStatus(obj)
	if result.Result.State != state.ReconcilSuspended {
		t.Errorf("State = %q, want %q", result.Result.State, state.ReconcilSuspended)
	}
}

func TestExtractFluxStatus_Progressing(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata": map[string]interface{}{
				"name":      "progressing-app",
				"namespace": "flux-system",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "Unknown",
						"reason":  "Progressing",
						"message": "Reconciling",
					},
				},
			},
		},
	}

	result := extractFluxStatus(obj)
	if result.Result.State != state.ReconcilProgressing {
		t.Errorf("State = %q, want %q", result.Result.State, state.ReconcilProgressing)
	}
}

func TestFluxWatcher_HandleObject_UpdatesStore(t *testing.T) {
	updater := &mockUpdater{}
	logger := slog.Default()

	w := &FluxWatcher{
		updater:       updater,
		logger:        logger,
		fluxNamespace: "flux-system",
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata": map[string]interface{}{
				"name":      "test-service",
				"namespace": "flux-system",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "True",
						"reason":             "ReconciliationSucceeded",
						"message":            "Applied revision: main@sha1:abc",
						"lastTransitionTime": "2026-02-27T10:00:00Z",
					},
				},
			},
		},
	}

	w.handleObject(obj, "kustomization")

	calls := updater.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(calls))
	}
	call := calls[0]
	if call.Status == nil {
		t.Fatal("expected non-nil GitOpsStatus")
	}
	if call.Status.ReconciliationState != state.ReconcilSynced {
		t.Errorf("State = %q, want %q", call.Status.ReconciliationState, state.ReconcilSynced)
	}
	if call.Status.SourceType != "kustomization" {
		t.Errorf("SourceType = %q, want %q", call.Status.SourceType, "kustomization")
	}
}

func TestFluxWatcher_HandleObject_StateTransition(t *testing.T) {
	updater := &mockUpdater{}
	logger := slog.Default()

	w := &FluxWatcher{
		updater:       updater,
		logger:        logger,
		fluxNamespace: "flux-system",
	}

	// First: synced
	synced := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata": map[string]interface{}{
				"name":      "transition-svc",
				"namespace": "flux-system",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "True",
						"message": "synced",
					},
				},
			},
		},
	}
	w.handleObject(synced, "kustomization")

	// Then: failed
	failed := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata": map[string]interface{}{
				"name":      "transition-svc",
				"namespace": "flux-system",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "False",
						"message": "build failed",
					},
				},
			},
		},
	}
	w.handleObject(failed, "kustomization")

	calls := updater.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 update calls, got %d", len(calls))
	}
	if calls[0].Status.ReconciliationState != state.ReconcilSynced {
		t.Errorf("first call state = %q, want synced", calls[0].Status.ReconciliationState)
	}
	if calls[1].Status.ReconciliationState != state.ReconcilFailed {
		t.Errorf("second call state = %q, want failed", calls[1].Status.ReconciliationState)
	}
}

func TestExtractFluxConditions(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "True",
						"reason":             "ReconciliationSucceeded",
						"message":            "Applied revision",
						"lastTransitionTime": "2026-02-27T10:00:00Z",
					},
					map[string]interface{}{
						"type":    "Reconciling",
						"status":  "False",
						"reason":  "",
						"message": "",
					},
				},
			},
		},
	}

	conditions, transitionTime := extractConditionsAndTime(obj)
	if len(conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(conditions))
	}
	if conditions[0].Type != "Ready" || conditions[0].Status != "True" {
		t.Errorf("unexpected first condition: %+v", conditions[0])
	}
	if transitionTime == nil {
		t.Error("expected non-nil transitionTime for Ready condition")
	}
	expected := time.Date(2026, 2, 27, 10, 0, 0, 0, time.UTC)
	if !transitionTime.Equal(expected) {
		t.Errorf("transitionTime = %v, want %v", transitionTime, expected)
	}
}
