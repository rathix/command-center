package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/state"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// fakeStateUpdater records calls to AddOrUpdate and Remove for testing.
type fakeStateUpdater struct {
	mu           sync.Mutex
	added        []state.Service
	removed      []string // "namespace/name"
	current      map[string]state.Service
	k8sConnected bool
	k8sCalls     []bool // history of SetK8sConnected calls
}

func (f *fakeStateUpdater) Get(namespace, name string) (state.Service, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.current == nil {
		return state.Service{}, false
	}
	svc, ok := f.current[namespace+"/"+name]
	return svc, ok
}

func (f *fakeStateUpdater) AddOrUpdate(svc state.Service) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.added = append(f.added, svc)
	if f.current == nil {
		f.current = make(map[string]state.Service)
	}
	f.current[svc.Namespace+"/"+svc.Name] = svc
}

func (f *fakeStateUpdater) Remove(namespace, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, namespace+"/"+name)
	delete(f.current, namespace+"/"+name)
}

func (f *fakeStateUpdater) getAdded() []state.Service {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]state.Service, len(f.added))
	copy(result, f.added)
	return result
}

func (f *fakeStateUpdater) getRemoved() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]string, len(f.removed))
	copy(result, f.removed)
	return result
}

func (f *fakeStateUpdater) SetK8sConnected(connected bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.k8sConnected = connected
	f.k8sCalls = append(f.k8sCalls, connected)
}

func (f *fakeStateUpdater) getK8sCalls() []bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]bool, len(f.k8sCalls))
	copy(result, f.k8sCalls)
	return result
}

func (f *fakeStateUpdater) Update(namespace, name string, fn func(*state.Service)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := namespace + "/" + name
	svc, ok := f.current[key]
	if !ok {
		return
	}
	fn(&svc)
	f.current[key] = svc
	f.added = append(f.added, svc)
}

func (f *fakeStateUpdater) SetConfigErrors(errs []string) {}
func (f *fakeStateUpdater) ConfigErrors() []string        { return nil }

func boolPtr(b bool) *bool {
	return &b
}

func newTestEndpointSlice(name, namespace, serviceName string, readyCount, notReadyCount int) *discoveryv1.EndpointSlice {
	var endpoints []discoveryv1.Endpoint
	for i := 0; i < readyCount; i++ {
		endpoints = append(endpoints, discoveryv1.Endpoint{
			Conditions: discoveryv1.EndpointConditions{
				Ready: boolPtr(true),
			},
		})
	}
	for i := 0; i < notReadyCount; i++ {
		endpoints = append(endpoints, discoveryv1.Endpoint{
			Conditions: discoveryv1.EndpointConditions{
				Ready: boolPtr(false),
			},
			TargetRef: &corev1.ObjectReference{
				Kind: "Pod",
				Name: fmt.Sprintf("pod-%d", i),
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

func newTestIngress(name, namespace, host string, tls bool) *networkingv1.Ingress {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
				},
			},
		},
	}
	if tls {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{Hosts: []string{host}},
		}
	}
	return ingress
}

func newTestIngressWithBackend(name, namespace, host string, tls bool, backendName string, backendPort int32) *networkingv1.Ingress {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: backendName,
											Port: networkingv1.ServiceBackendPort{Number: backendPort},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if tls {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{Hosts: []string{host}},
		}
	}
	return ingress
}

func TestStateUpdaterInterface(t *testing.T) {
	// Verify state.Store satisfies StateUpdater interface (includes SetK8sConnected)
	var _ StateUpdater = &state.Store{}
}

func TestStateUpdaterInterfaceHasSetK8sConnected(t *testing.T) {
	// Compile-time check: StateUpdater requires SetK8sConnected
	var u StateUpdater = &fakeStateUpdater{}
	u.SetK8sConnected(true)
	_ = u
}

func TestNewWatcherWithFakeClient(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))
	if w == nil {
		t.Fatal("NewWatcherWithClient returned nil")
	}
}

func TestWatcherDiscoverExistingIngresses(t *testing.T) {
	// Pre-create Ingresses before starting the watcher
	ingress := newTestIngress("my-app", "default", "my-app.example.com", true)

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	// Verify IngressLister interface (AC #5)
	var _ IngressLister = w.lister

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	// Wait for the informer to sync and process events
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for service to be discovered")
		default:
			added := updater.getAdded()
			if len(added) >= 1 {
				svc := added[0]
				if svc.Name != "my-app" {
					t.Errorf("expected name 'my-app', got %q", svc.Name)
				}
				if svc.Namespace != "default" {
					t.Errorf("expected namespace 'default', got %q", svc.Namespace)
				}
				if svc.Group != "default" {
					t.Errorf("expected group 'default', got %q", svc.Group)
				}
				if svc.URL != "https://my-app.example.com" {
					t.Errorf("expected URL 'https://my-app.example.com', got %q", svc.URL)
				}
				if svc.Status != state.StatusUnknown {
					t.Errorf("expected status %q, got %q", state.StatusUnknown, svc.Status)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWatcherHTTPIngress(t *testing.T) {
	// Ingress without TLS should produce http:// URL
	ingress := newTestIngress("http-app", "default", "http-app.example.com", false)

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for service to be discovered")
		default:
			added := updater.getAdded()
			if len(added) >= 1 {
				if added[0].URL != "http://http-app.example.com" {
					t.Errorf("expected URL 'http://http-app.example.com', got %q", added[0].URL)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWatcherTLSFallbackHost(t *testing.T) {
	// Ingress with no rules but TLS hosts should still work
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tls-fallback",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{Hosts: []string{"fallback.example.com"}},
			},
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "fallback-svc",
					Port: networkingv1.ServiceBackendPort{Number: 80},
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for service to be discovered via TLS fallback")
		default:
			added := updater.getAdded()
			if len(added) >= 1 {
				if added[0].URL != "https://fallback.example.com" {
					t.Errorf("expected URL 'https://fallback.example.com', got %q", added[0].URL)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWatcherDeleteIngress(t *testing.T) {
	ingress := newTestIngress("delete-me", "test-ns", "delete-me.example.com", true)

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	// Wait for initial add
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for initial add")
		default:
			if len(updater.getAdded()) >= 1 {
				goto deletePhase
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

deletePhase:
	// Delete the Ingress
	err := clientset.NetworkingV1().Ingresses("test-ns").Delete(ctx, "delete-me", metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("failed to delete ingress: %v", err)
	}

	// Wait for removal event
	deadline = time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for service removal")
		default:
			removed := updater.getRemoved()
			if len(removed) >= 1 {
				if removed[0] != "test-ns/delete-me" {
					t.Errorf("expected removal of 'test-ns/delete-me', got %q", removed[0])
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWatcherUpdateIngress(t *testing.T) {
	ingress := newTestIngress("update-me", "default", "old.example.com", false)

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	// Wait for initial add
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for initial add")
		default:
			if len(updater.getAdded()) >= 1 {
				goto updatePhase
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

updatePhase:
	// Update the Ingress host
	updated := newTestIngress("update-me", "default", "new.example.com", true)
	_, err := clientset.NetworkingV1().Ingresses("default").Update(ctx, updated, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to update ingress: %v", err)
	}

	// Wait for update event (should produce another AddOrUpdate call)
	deadline = time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for update event")
		default:
			added := updater.getAdded()
			if len(added) >= 2 {
				latest := added[len(added)-1]
				if latest.URL != "https://new.example.com" {
					t.Errorf("expected URL 'https://new.example.com', got %q", latest.URL)
				}
				if latest.Group != "default" {
					t.Errorf("expected group 'default', got %q", latest.Group)
				}
				if latest.Group != latest.Namespace {
					t.Errorf("expected group to match namespace, group=%q namespace=%q", latest.Group, latest.Namespace)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWatcherSkipsIngressWithNoRules(t *testing.T) {
	// Ingress with no rules should be skipped
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-rules",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{},
	}

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	// Give time for events to process
	time.Sleep(500 * time.Millisecond)

	added := updater.getAdded()
	if len(added) != 0 {
		t.Errorf("expected no services added for ingress with no rules, got %d", len(added))
	}
}

func TestWatcherSkipsIngressWithNoHost(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-host",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{Host: ""},
			},
		},
	}

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	time.Sleep(500 * time.Millisecond)

	added := updater.getAdded()
	if len(added) != 0 {
		t.Errorf("expected no services added for ingress with no host, got %d", len(added))
	}
}

func TestWatcherContextCancellation(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	// Give informer time to start
	time.Sleep(200 * time.Millisecond)

	// Cancel context
	cancel()

	// Watcher should stop
	select {
	case <-done:
		// Success — watcher stopped
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not stop after context cancellation")
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"grafana.kenny.live", "grafana"},
		{"longhorn.kenny.live", "longhorn"},
		{"trilium.kenny.live", "trilium"},
		{"app.example.com", "app"},
		{"singlehost", "singlehost"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := displayName(tt.host)
			if got != tt.want {
				t.Errorf("displayName(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

func TestExtractServiceURL(t *testing.T) {
	tests := []struct {
		name     string
		ingress  *networkingv1.Ingress
		wantURL  string
		wantHost string
		wantOK   bool
	}{
		{
			name:     "https with TLS",
			ingress:  newTestIngress("app", "ns", "app.example.com", true),
			wantURL:  "https://app.example.com",
			wantHost: "app.example.com",
			wantOK:   true,
		},
		{
			name:     "http without TLS",
			ingress:  newTestIngress("app", "ns", "app.example.com", false),
			wantURL:  "http://app.example.com",
			wantHost: "app.example.com",
			wantOK:   true,
		},
		{
			name: "no rules",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns"},
				Spec:       networkingv1.IngressSpec{},
			},
			wantURL:  "",
			wantHost: "",
			wantOK:   false,
		},
		{
			name: "empty host",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns"},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: ""}},
				},
			},
			wantURL:  "",
			wantHost: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotHost, gotOK := extractServiceURL(tt.ingress)
			if gotOK != tt.wantOK {
				t.Errorf("extractServiceURL() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotURL != tt.wantURL {
				t.Errorf("extractServiceURL() url = %q, want %q", gotURL, tt.wantURL)
			}
			if gotHost != tt.wantHost {
				t.Errorf("extractServiceURL() host = %q, want %q", gotHost, tt.wantHost)
			}
		})
	}
}

func TestWatcherSetsK8sConnectedOnDiscovery(t *testing.T) {
	ingress := newTestIngress("k8s-test", "default", "k8s-test.example.com", true)
	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for k8s connected call")
		default:
			calls := updater.getK8sCalls()
			if len(calls) >= 1 {
				// Should have at least one SetK8sConnected(true) call
				foundTrue := false
				for _, c := range calls {
					if c {
						foundTrue = true
						break
					}
				}
				if !foundTrue {
					t.Error("expected SetK8sConnected(true) to be called")
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWatcherDoesNotSetK8sConnectedWhenCacheSyncIncomplete(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Force WaitForCacheSync to fail immediately

	w.Run(ctx)

	if calls := updater.getK8sCalls(); len(calls) != 0 {
		t.Fatalf("expected no SetK8sConnected calls when cache sync fails, got %v", calls)
	}
}

func TestWatcherGroupMatchesNamespace(t *testing.T) {
	ingress := newTestIngress("media-app", "media", "media-app.example.com", true)

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for service discovery")
		default:
			added := updater.getAdded()
			if len(added) >= 1 {
				svc := added[0]
				if svc.Group != "media" {
					t.Errorf("expected group 'media' (matching namespace), got %q", svc.Group)
				}
				if svc.Group != svc.Namespace {
					t.Errorf("expected group to match namespace, group=%q namespace=%q", svc.Group, svc.Namespace)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestNewWatcherErrorSanitization(t *testing.T) {
	// Call NewWatcher with a non-existent kubeconfig path
	_, err := NewWatcher("/home/user/.kube/nonexistent-config", &fakeStateUpdater{}, slog.Default())
	if err == nil {
		t.Fatal("expected error for non-existent kubeconfig")
	}
	errMsg := err.Error()

	// The error must NOT contain the kubeconfig file path
	sensitivePatterns := []string{
		"/home/user/.kube",
		"nonexistent-config",
		".kube/config",
	}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(errMsg, pattern) {
			t.Errorf("error contains sensitive path info %q: %s", pattern, errMsg)
		}
	}

	// The error should be a generic message
	if !strings.Contains(errMsg, "k8s watcher") {
		t.Errorf("expected generic k8s watcher error, got: %s", errMsg)
	}
}

func TestWatcherPreservesStatusOnUpdate(t *testing.T) {
	ingress := newTestIngress("status-app", "default", "status.example.com", false)

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{
		current: make(map[string]state.Service),
	}
	logger := slog.Default()

	// Pre-populate updater with a healthy status
	updater.AddOrUpdate(state.Service{
		Name:      "status-app",
		Namespace: "default",
		URL:       "http://status.example.com",
		Status:    state.StatusHealthy,
	})

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	// Wait for the update event (triggered by informer sync)
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for update event")
		default:
			added := updater.getAdded()
			// Find the latest update for status-app
			var latest state.Service
			found := false
			for i := len(added) - 1; i >= 0; i-- {
				if added[i].Name == "status-app" {
					latest = added[i]
					found = true
					break
				}
			}

			if found && latest.Status == state.StatusHealthy {
				// Success: status was preserved
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWatcherOnUpdatePreservesRuntimeFields(t *testing.T) {
	es := newTestEndpointSlice("backend-svc-es", "my-ns", "backend-svc", 2, 1)
	clientset := fake.NewSimpleClientset(es)
	updater := &fakeStateUpdater{
		current: make(map[string]state.Service),
	}
	logger := slog.Default()
	w := NewWatcherWithClient(clientset, updater, logger)
	if !w.endpointSliceWatcher.WaitForSync(context.Background()) {
		t.Fatal("EndpointSliceWatcher failed to sync")
	}

	ready := 2
	total := 3
	reason := "CrashLoopBackOff"
	updater.AddOrUpdate(state.Service{
		Name:                "my-app",
		Namespace:           "my-ns",
		Group:               "custom-group",
		URL:                 "https://old.example.com",
		DisplayName:         "Friendly Name",
		OriginalDisplayName: "old",
		Source:              state.SourceKubernetes,
		Status:              state.StatusHealthy,
		AuthGuarded:         true,
		ReadyEndpoints:      &ready,
		TotalEndpoints:      &total,
		PodDiagnostic: &state.PodDiagnostic{
			Reason:       &reason,
			RestartCount: 4,
		},
	})

	updatedIngress := newTestIngressWithBackend("my-app", "my-ns", "new.example.com", true, "backend-svc", 8080)
	w.onUpdate(nil, updatedIngress)

	got, ok := updater.Get("my-ns", "my-app")
	if !ok {
		t.Fatal("expected updated service to exist")
	}

	if got.Status != state.StatusHealthy {
		t.Fatalf("Status = %q, want %q", got.Status, state.StatusHealthy)
	}
	if !got.AuthGuarded {
		t.Fatal("AuthGuarded = false, want true")
	}
	if got.ReadyEndpoints == nil || *got.ReadyEndpoints != 2 {
		t.Fatalf("ReadyEndpoints = %v, want 2", *got.ReadyEndpoints)
	}
	if got.TotalEndpoints == nil || *got.TotalEndpoints != 3 {
		t.Fatalf("TotalEndpoints = %v, want 3", got.TotalEndpoints)
	}

	// Wait for async pod diagnostic query to complete
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for pod diagnostics")
		default:
			got, _ = updater.Get("my-ns", "my-app")
			if got.PodDiagnostic != nil && got.PodDiagnostic.Reason != nil && *got.PodDiagnostic.Reason == "CrashLoopBackOff" {
				goto diagFound
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

diagFound:
	// Preserve explicit display override, but update discovered original display name.
	if got.DisplayName != "Friendly Name" {
		t.Fatalf("DisplayName = %q, want override to be preserved", got.DisplayName)
	}
	if got.OriginalDisplayName != "new" {
		t.Fatalf("OriginalDisplayName = %q, want %q", got.OriginalDisplayName, "new")
	}
	if got.Group != "custom-group" {
		t.Fatalf("Group = %q, want preserved custom-group", got.Group)
	}
	if got.URL != "https://new.example.com" {
		t.Fatalf("URL = %q, want %q", got.URL, "https://new.example.com")
	}
}

func TestWatcherOnUpdateUnwatchesWhenBackendRemoved(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	updater := &fakeStateUpdater{
		current: make(map[string]state.Service),
	}
	logger := slog.Default()
	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	updater.AddOrUpdate(state.Service{
		Name:            "my-app",
		Namespace:       "my-ns",
		URL:             "https://my-app.example.com",
		Status:          state.StatusUnknown,
	})

	w.endpointSliceWatcher.Watch("my-app", "my-ns", "backend-svc")
	defer w.endpointSliceWatcher.StopAll()

	// Manually trigger update for fake informer test
	w.endpointSliceWatcher.triggerUpdate("my-ns", "backend-svc")

	w.endpointSliceWatcher.mu.Lock()
	_, existsBefore := w.endpointSliceWatcher.serviceToIngress["my-ns/backend-svc"]["my-app"]
	w.endpointSliceWatcher.mu.Unlock()
	if !existsBefore {
		t.Fatal("expected initial EndpointSlice watch to exist")
	}

	// Update to an ingress without backend service should clear watch mapping.
	w.onUpdate(nil, newTestIngress("my-app", "my-ns", "my-app.example.com", true))

	w.endpointSliceWatcher.mu.Lock()
	_, existsAfter := w.endpointSliceWatcher.serviceToIngress["my-ns/backend-svc"]["my-app"]
	w.endpointSliceWatcher.mu.Unlock()
	if existsAfter {
		t.Fatal("expected EndpointSlice watch to be removed when backend is missing")
	}
}

func TestExtractBackendServiceName(t *testing.T) {
	tests := []struct {
		name          string
		ingress       *networkingv1.Ingress
		wantService   string
		wantNamespace string
		wantOK        bool
	}{
		{
			name:          "standard ingress with backend",
			ingress:       newTestIngressWithBackend("app", "my-ns", "app.example.com", true, "my-svc", 8080),
			wantService:   "my-svc",
			wantNamespace: "my-ns",
			wantOK:        true,
		},
		{
			name: "no rules",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns"},
				Spec:       networkingv1.IngressSpec{},
			},
			wantService:   "",
			wantNamespace: "",
			wantOK:        false,
		},
		{
			name: "nil HTTP",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns"},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{Host: "app.example.com"}},
				},
			},
			wantService:   "",
			wantNamespace: "",
			wantOK:        false,
		},
		{
			name: "empty service name",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns"},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "app.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "",
													Port: networkingv1.ServiceBackendPort{Number: 80},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantService:   "",
			wantNamespace: "",
			wantOK:        false,
		},
		{
			name: "nil service",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "ns"},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "app.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Backend: networkingv1.IngressBackend{
												Service: nil,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantService:   "",
			wantNamespace: "",
			wantOK:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotService, gotNamespace, gotOK := extractBackendServiceName(tt.ingress)
			if gotOK != tt.wantOK {
				t.Errorf("extractBackendServiceName() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotService != tt.wantService {
				t.Errorf("extractBackendServiceName() service = %q, want %q", gotService, tt.wantService)
			}
			if gotNamespace != tt.wantNamespace {
				t.Errorf("extractBackendServiceName() namespace = %q, want %q", gotNamespace, tt.wantNamespace)
			}
		})
	}
}

func TestWatcherStartsEndpointSliceWatchOnIngressAdd(t *testing.T) {
	ingress := newTestIngressWithBackend("my-app", "my-ns", "my-app.example.com", true, "my-svc", 8080)

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{current: make(map[string]state.Service)}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for service discovery")
		default:
			added := updater.getAdded()
			if len(added) >= 1 && added[0].Name == "my-app" {
				w.endpointSliceWatcher.mu.Lock()
				_, exists := w.endpointSliceWatcher.serviceToIngress["my-ns/my-svc"]["my-app"]
				w.endpointSliceWatcher.mu.Unlock()
				if exists {
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWatcherNoEndpointSliceWatchForIngressWithoutBackend(t *testing.T) {
	ingress := newTestIngress("no-backend", "default", "no-backend.example.com", false)

	clientset := fake.NewSimpleClientset(ingress)
	updater := &fakeStateUpdater{current: make(map[string]state.Service)}
	logger := slog.Default()

	w := NewWatcherWithClientAndESWatcher(clientset, updater, logger, NewEndpointSliceWatcher(clientset, updater, logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for service discovery")
		default:
			added := updater.getAdded()
			if len(added) >= 1 {
				w.endpointSliceWatcher.mu.Lock()
				watchCount := len(w.endpointSliceWatcher.serviceToIngress)
				w.endpointSliceWatcher.mu.Unlock()
				if watchCount != 0 {
					t.Errorf("expected no EndpointSlice watches, got %d", watchCount)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
