package k8s

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/state"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// fakeStateUpdater records calls to AddOrUpdate and Remove for testing.
type fakeStateUpdater struct {
	mu       sync.Mutex
	added    []state.Service
	removed  []string // "namespace/name"
	current  map[string]state.Service
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

func TestStateUpdaterInterface(t *testing.T) {
	// Verify state.Store satisfies StateUpdater interface
	var _ StateUpdater = &state.Store{}
}

func TestNewWatcherWithFakeClient(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	updater := &fakeStateUpdater{}
	logger := slog.Default()

	w := NewWatcherWithClient(clientset, updater, logger)
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

	w := NewWatcherWithClient(clientset, updater, logger)

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

	w := NewWatcherWithClient(clientset, updater, logger)

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

	w := NewWatcherWithClient(clientset, updater, logger)

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

	w := NewWatcherWithClient(clientset, updater, logger)

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

	w := NewWatcherWithClient(clientset, updater, logger)

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

	w := NewWatcherWithClient(clientset, updater, logger)

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

	w := NewWatcherWithClient(clientset, updater, logger)

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

	w := NewWatcherWithClient(clientset, updater, logger)

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

	w := NewWatcherWithClient(clientset, updater, logger)

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
