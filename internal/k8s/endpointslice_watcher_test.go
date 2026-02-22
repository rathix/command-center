package k8s

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/state"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// fakeEndpointStateUpdater tracks Update calls for testing.
type fakeEndpointStateUpdater struct {
	mu      sync.Mutex
	updates []state.Service
	current map[string]state.Service
}

func (f *fakeEndpointStateUpdater) Update(namespace, name string, fn func(*state.Service)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := namespace + "/" + name
	svc, ok := f.current[key]
	if !ok {
		return
	}
	fn(&svc)
	f.current[key] = svc
	f.updates = append(f.updates, svc)
}

func (f *fakeEndpointStateUpdater) getUpdates() []state.Service {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]state.Service, len(f.updates))
	copy(result, f.updates)
	return result
}

func boolPtr(b bool) *bool {
	return &b
}

func newTestEndpointSlice(name, namespace, serviceName string, readyCount, notReadyCount int) *discoveryv1.EndpointSlice {
	var endpoints []discoveryv1.Endpoint
	for i := range readyCount {
		_ = i
		endpoints = append(endpoints, discoveryv1.Endpoint{
			Conditions: discoveryv1.EndpointConditions{
				Ready: boolPtr(true),
			},
		})
	}
	for i := range notReadyCount {
		_ = i
		endpoints = append(endpoints, discoveryv1.Endpoint{
			Conditions: discoveryv1.EndpointConditions{
				Ready: boolPtr(false),
			},
		})
	}

	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"kubernetes.io/service-name": serviceName,
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   endpoints,
	}
}

func TestAggregateEndpointReadiness(t *testing.T) {
	tests := []struct {
		name      string
		slices    []*discoveryv1.EndpointSlice
		wantReady int
		wantTotal int
	}{
		{
			name:      "no slices",
			slices:    nil,
			wantReady: 0,
			wantTotal: 0,
		},
		{
			name: "all ready",
			slices: []*discoveryv1.EndpointSlice{
				newTestEndpointSlice("es-1", "ns", "svc", 3, 0),
			},
			wantReady: 3,
			wantTotal: 3,
		},
		{
			name: "mixed readiness",
			slices: []*discoveryv1.EndpointSlice{
				newTestEndpointSlice("es-1", "ns", "svc", 2, 1),
			},
			wantReady: 2,
			wantTotal: 3,
		},
		{
			name: "all not ready",
			slices: []*discoveryv1.EndpointSlice{
				newTestEndpointSlice("es-1", "ns", "svc", 0, 3),
			},
			wantReady: 0,
			wantTotal: 3,
		},
		{
			name: "multiple slices",
			slices: []*discoveryv1.EndpointSlice{
				newTestEndpointSlice("es-1", "ns", "svc", 2, 0),
				newTestEndpointSlice("es-2", "ns", "svc", 1, 1),
			},
			wantReady: 3,
			wantTotal: 4,
		},
		{
			name: "nil ready condition counts as not ready",
			slices: []*discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "es-nil", Namespace: "ns"},
					Endpoints: []discoveryv1.Endpoint{
						{Conditions: discoveryv1.EndpointConditions{Ready: nil}},
						{Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)}},
					},
				},
			},
			wantReady: 1,
			wantTotal: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready, total := aggregateEndpointReadiness(tt.slices)
			if ready != tt.wantReady {
				t.Errorf("ready = %d, want %d", ready, tt.wantReady)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
		})
	}
}

func TestExtractNotReadyPodNames(t *testing.T) {
	slices := []*discoveryv1.EndpointSlice{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "es-1", Namespace: "ns"},
			Endpoints: []discoveryv1.Endpoint{
				{
					Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(false)},
					TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: "pod-a"},
				},
				{
					Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(false)},
					TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: "pod-a"}, // duplicate
				},
				{
					Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)},
					TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: "pod-ready"},
				},
				{
					Conditions: discoveryv1.EndpointConditions{Ready: nil}, // nil counts as not-ready
					TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: "pod-nil"},
				},
				{
					Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(false)},
					TargetRef:  &corev1.ObjectReference{Kind: "Service", Name: "not-a-pod"},
				},
			},
		},
	}

	got := extractNotReadyPodNames(slices)
	if len(got) != 2 {
		t.Fatalf("extractNotReadyPodNames() len = %d, want 2 (%v)", len(got), got)
	}
	if got[0] != "pod-a" || got[1] != "pod-nil" {
		t.Fatalf("extractNotReadyPodNames() = %v, want [pod-a pod-nil]", got)
	}
}

func TestEndpointSliceWatcher_AllReady(t *testing.T) {
	es := newTestEndpointSlice("my-svc-abc", "my-ns", "my-svc", 3, 0)

	clientset := fake.NewSimpleClientset(es)
	updater := &fakeEndpointStateUpdater{
		current: map[string]state.Service{
			"my-ns/my-app": {Name: "my-app", Namespace: "my-ns", Status: state.StatusUnknown},
		},
	}
	logger := slog.Default()

	esw := NewEndpointSliceWatcher(clientset, updater, logger)
	esw.Watch("my-app", "my-ns", "my-svc")
	defer esw.StopAll()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for endpoint update")
		default:
			updates := updater.getUpdates()
			if len(updates) >= 1 {
				latest := updates[len(updates)-1]
				if latest.ReadyEndpoints == nil || latest.TotalEndpoints == nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				if *latest.ReadyEndpoints != 3 {
					t.Errorf("expected ReadyEndpoints=3, got %d", *latest.ReadyEndpoints)
				}
				if *latest.TotalEndpoints != 3 {
					t.Errorf("expected TotalEndpoints=3, got %d", *latest.TotalEndpoints)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestEndpointSliceWatcher_MixedReadiness(t *testing.T) {
	es := newTestEndpointSlice("my-svc-abc", "my-ns", "my-svc", 2, 1)

	clientset := fake.NewSimpleClientset(es)
	updater := &fakeEndpointStateUpdater{
		current: map[string]state.Service{
			"my-ns/my-app": {Name: "my-app", Namespace: "my-ns", Status: state.StatusUnknown},
		},
	}
	logger := slog.Default()

	esw := NewEndpointSliceWatcher(clientset, updater, logger)
	esw.Watch("my-app", "my-ns", "my-svc")
	defer esw.StopAll()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for endpoint update")
		default:
			updates := updater.getUpdates()
			if len(updates) >= 1 {
				latest := updates[len(updates)-1]
				if latest.ReadyEndpoints == nil || latest.TotalEndpoints == nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				if *latest.ReadyEndpoints != 2 {
					t.Errorf("expected ReadyEndpoints=2, got %d", *latest.ReadyEndpoints)
				}
				if *latest.TotalEndpoints != 3 {
					t.Errorf("expected TotalEndpoints=3, got %d", *latest.TotalEndpoints)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestEndpointSliceWatcher_Unwatch(t *testing.T) {
	es := newTestEndpointSlice("my-svc-abc", "my-ns", "my-svc", 3, 0)

	clientset := fake.NewSimpleClientset(es)
	updater := &fakeEndpointStateUpdater{
		current: map[string]state.Service{
			"my-ns/my-app": {Name: "my-app", Namespace: "my-ns", Status: state.StatusUnknown},
		},
	}
	logger := slog.Default()

	esw := NewEndpointSliceWatcher(clientset, updater, logger)
	esw.Watch("my-app", "my-ns", "my-svc")

	// Wait for at least one update
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for initial endpoint update")
		default:
			if len(updater.getUpdates()) >= 1 {
				goto unwatchPhase
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

unwatchPhase:
	esw.Unwatch("my-app", "my-ns")

	esw.mu.Lock()
	watchCount := len(esw.watches)
	esw.mu.Unlock()

	if watchCount != 0 {
		t.Errorf("expected 0 watches after Unwatch, got %d", watchCount)
	}
}

func TestEndpointSliceWatcher_StopAll(t *testing.T) {
	es1 := newTestEndpointSlice("svc-a-abc", "ns-a", "svc-a", 1, 0)
	es2 := newTestEndpointSlice("svc-b-abc", "ns-b", "svc-b", 1, 0)

	clientset := fake.NewSimpleClientset(es1, es2)
	updater := &fakeEndpointStateUpdater{
		current: map[string]state.Service{
			"ns-a/app-a": {Name: "app-a", Namespace: "ns-a", Status: state.StatusUnknown},
			"ns-b/app-b": {Name: "app-b", Namespace: "ns-b", Status: state.StatusUnknown},
		},
	}
	logger := slog.Default()

	esw := NewEndpointSliceWatcher(clientset, updater, logger)
	esw.Watch("app-a", "ns-a", "svc-a")
	esw.Watch("app-b", "ns-b", "svc-b")

	// Brief wait for informers to start
	time.Sleep(200 * time.Millisecond)

	esw.StopAll()

	esw.mu.Lock()
	watchCount := len(esw.watches)
	esw.mu.Unlock()

	if watchCount != 0 {
		t.Errorf("expected 0 watches after StopAll, got %d", watchCount)
	}
}

func TestEndpointSliceWatcher_NilReadyCondition(t *testing.T) {
	// EndpointSlice with nil Ready condition on one endpoint
	es := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc-abc",
			Namespace: "my-ns",
			Labels: map[string]string{
				"kubernetes.io/service-name": "my-svc",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)}},
			{Conditions: discoveryv1.EndpointConditions{Ready: nil}},
			{Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)}},
		},
	}

	clientset := fake.NewSimpleClientset(es)
	updater := &fakeEndpointStateUpdater{
		current: map[string]state.Service{
			"my-ns/my-app": {Name: "my-app", Namespace: "my-ns", Status: state.StatusUnknown},
		},
	}
	logger := slog.Default()

	esw := NewEndpointSliceWatcher(clientset, updater, logger)
	esw.Watch("my-app", "my-ns", "my-svc")
	defer esw.StopAll()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for endpoint update")
		default:
			updates := updater.getUpdates()
			if len(updates) >= 1 {
				latest := updates[len(updates)-1]
				if latest.ReadyEndpoints == nil || latest.TotalEndpoints == nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				if *latest.ReadyEndpoints != 2 {
					t.Errorf("expected ReadyEndpoints=2 (nil counts as not ready), got %d", *latest.ReadyEndpoints)
				}
				if *latest.TotalEndpoints != 3 {
					t.Errorf("expected TotalEndpoints=3, got %d", *latest.TotalEndpoints)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestEndpointStateUpdaterInterface(t *testing.T) {
	// Compile-time check: state.Store satisfies EndpointStateUpdater
	var _ EndpointStateUpdater = &state.Store{}
}
