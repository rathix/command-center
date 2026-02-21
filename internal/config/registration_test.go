package config

import (
	"sync"
	"testing"

	"github.com/rathix/command-center/internal/state"
)

// fakeStore implements StateUpdater with a simple map for testing.
type fakeStore struct {
	mu       sync.Mutex
	services map[string]state.Service
	removed  []string // tracks Remove() calls as "namespace/name"
}

func newFakeStore() *fakeStore {
	return &fakeStore{services: make(map[string]state.Service)}
}

func (f *fakeStore) AddOrUpdate(svc state.Service) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.services[svc.Namespace+"/"+svc.Name] = svc
}

func (f *fakeStore) Remove(namespace, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.services, namespace+"/"+name)
	f.removed = append(f.removed, namespace+"/"+name)
}

func (f *fakeStore) Update(namespace, name string, fn func(*state.Service)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := namespace + "/" + name
	svc, ok := f.services[key]
	if !ok {
		return
	}
	fn(&svc)
	f.services[key] = svc
}

func (f *fakeStore) Get(namespace, name string) (state.Service, bool) {
        f.mu.Lock()
        defer f.mu.Unlock()
        svc, ok := f.services[namespace+"/"+name]
        return svc, ok
}

func (f *fakeStore) All() []state.Service {
        f.mu.Lock()
        defer f.mu.Unlock()
        res := make([]state.Service, 0, len(f.services))
        for _, s := range f.services {
                res = append(res, s)
        }
        return res
}

func TestApplyOverrides_RestoreOriginalValues(t *testing.T) {
        store := newFakeStore()
        // Pre-populate a K8s service with original value
        store.AddOrUpdate(state.Service{
                Name:                "pihole",
                Namespace:           "default",
                DisplayName:         "overridden",
                OriginalDisplayName: "pihole",
                Source:              state.SourceKubernetes,
                Icon:                "old-icon",
        })

        // Config with NO overrides for pihole
        cfg := &Config{
                Overrides: []ServiceOverride{},
        }

        ApplyOverrides(store, cfg)

        svc, _ := store.Get("default", "pihole")
        if svc.DisplayName != "pihole" {
                t.Errorf("expected displayName restored to %q, got %q", "pihole", svc.DisplayName)
        }
        if svc.Icon != "" {
                t.Errorf("expected icon cleared, got %q", svc.Icon)
        }
}
func TestRegisterServices_CustomServicesAppearInStore(t *testing.T) {
	store := newFakeStore()
	cfg := &Config{
		Services: []CustomService{
			{Name: "truenas", URL: "https://truenas.local", Group: "infrastructure", DisplayName: "TrueNAS", Icon: "truenas"},
			{Name: "router", URL: "https://192.168.1.1", Group: "infrastructure"},
		},
	}

	RegisterServices(store, cfg)

	svc, ok := store.Get("custom", "truenas")
	if !ok {
		t.Fatal("expected truenas in store")
	}
	if svc.Source != state.SourceConfig {
		t.Errorf("expected source %q, got %q", state.SourceConfig, svc.Source)
	}
	if svc.Namespace != "custom" {
		t.Errorf("expected namespace %q, got %q", "custom", svc.Namespace)
	}
	if svc.DisplayName != "TrueNAS" {
		t.Errorf("expected displayName %q, got %q", "TrueNAS", svc.DisplayName)
	}
	if svc.Group != "infrastructure" {
		t.Errorf("expected group %q, got %q", "infrastructure", svc.Group)
	}
	if svc.Status != state.StatusUnknown {
		t.Errorf("expected status %q, got %q", state.StatusUnknown, svc.Status)
	}
	if svc.Icon != "truenas" {
		t.Errorf("expected icon %q, got %q", "truenas", svc.Icon)
	}

	// Router should default displayName to name
	router, ok := store.Get("custom", "router")
	if !ok {
		t.Fatal("expected router in store")
	}
	if router.DisplayName != "router" {
		t.Errorf("expected displayName %q (defaulted from name), got %q", "router", router.DisplayName)
	}
}

func TestApplyOverrides_DisplayNameUpdated(t *testing.T) {
	store := newFakeStore()
	// Pre-populate a K8s service
	store.AddOrUpdate(state.Service{
		Name:      "pihole",
		Namespace: "default",
		Source:    state.SourceKubernetes,
		Status:    state.StatusHealthy,
	})

	cfg := &Config{
		Overrides: []ServiceOverride{
			{Match: "default/pihole", DisplayName: "Pi-hole DNS"},
		},
	}

	ApplyOverrides(store, cfg)

	svc, _ := store.Get("default", "pihole")
	if svc.DisplayName != "Pi-hole DNS" {
		t.Errorf("expected displayName %q, got %q", "Pi-hole DNS", svc.DisplayName)
	}
}

func TestReconcileOnReload_AddedServiceTriggersAddOrUpdate(t *testing.T) {
	store := newFakeStore()
	oldCfg := &Config{}
	newCfg := &Config{
		Services: []CustomService{
			{Name: "newservice", URL: "https://new.local", Group: "apps"},
		},
	}

	added, removed, updated := ReconcileOnReload(store, oldCfg, newCfg)

	if added != 1 {
		t.Errorf("expected 1 added, got %d", added)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
	if updated != 0 {
		t.Errorf("expected 0 updated, got %d", updated)
	}
	if _, ok := store.Get("custom", "newservice"); !ok {
		t.Error("expected newservice in store")
	}
}

func TestReconcileOnReload_RemovedServiceTriggersRemove(t *testing.T) {
	store := newFakeStore()
	store.AddOrUpdate(state.Service{Name: "old", Namespace: "custom", Source: state.SourceConfig})

	oldCfg := &Config{
		Services: []CustomService{
			{Name: "old", URL: "https://old.local", Group: "apps"},
		},
	}
	newCfg := &Config{}

	added, removed, updated := ReconcileOnReload(store, oldCfg, newCfg)

	if added != 0 {
		t.Errorf("expected 0 added, got %d", added)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if updated != 0 {
		t.Errorf("expected 0 updated, got %d", updated)
	}
	if _, ok := store.Get("custom", "old"); ok {
		t.Error("expected old service to be removed from store")
	}
}

func TestReconcileOnReload_ChangedServiceTriggersUpdate(t *testing.T) {
	store := newFakeStore()
	store.AddOrUpdate(state.Service{
		Name: "svc", Namespace: "custom", URL: "https://old.local", Group: "apps",
		Source: state.SourceConfig,
	})

	oldCfg := &Config{
		Services: []CustomService{
			{Name: "svc", URL: "https://old.local", Group: "apps"},
		},
	}
	newCfg := &Config{
		Services: []CustomService{
			{Name: "svc", URL: "https://new.local", Group: "apps"},
		},
	}

	added, removed, updated := ReconcileOnReload(store, oldCfg, newCfg)

	if added != 0 {
		t.Errorf("expected 0 added, got %d", added)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
	if updated != 1 {
		t.Errorf("expected 1 updated, got %d", updated)
	}

	svc, _ := store.Get("custom", "svc")
	if svc.URL != "https://new.local" {
		t.Errorf("expected URL %q, got %q", "https://new.local", svc.URL)
	}
}

func TestReconcileOnReload_NilNewConfigNoOp(t *testing.T) {
	store := newFakeStore()
	store.AddOrUpdate(state.Service{Name: "svc", Namespace: "custom", URL: "https://old.local", Source: state.SourceConfig})
	oldCfg := &Config{
		Services: []CustomService{
			{Name: "svc", URL: "https://old.local", Group: "apps"},
		},
	}

	added, removed, updated := ReconcileOnReload(store, oldCfg, nil)
	if added != 0 || removed != 0 || updated != 0 {
		t.Fatalf("expected no-op counters for nil new config, got added=%d removed=%d updated=%d", added, removed, updated)
	}
	if _, ok := store.Get("custom", "svc"); !ok {
		t.Fatal("expected existing service to remain in store")
	}
}

func TestRegisterServices_EmptyConfig(t *testing.T) {
	store := newFakeStore()
	RegisterServices(store, &Config{})

	if len(store.services) != 0 {
		t.Errorf("expected empty store, got %d services", len(store.services))
	}
}

func TestRegisterServices_NilConfig(t *testing.T) {
	store := newFakeStore()
	RegisterServices(store, nil)

	if len(store.services) != 0 {
		t.Errorf("expected empty store, got %d services", len(store.services))
	}
}

func TestApplyOverrides_NonExistentService(t *testing.T) {
	store := newFakeStore()
	cfg := &Config{
		Overrides: []ServiceOverride{
			{Match: "default/missing", DisplayName: "Won't Apply"},
		},
	}

	// Should not panic
	ApplyOverrides(store, cfg)

	if len(store.services) != 0 {
		t.Errorf("expected empty store, got %d services", len(store.services))
	}
}

func TestApplyOverrides_HealthEndpointAndExpectedStatusCodes(t *testing.T) {
	store := newFakeStore()
	store.AddOrUpdate(state.Service{
		Name: "svc", Namespace: "default", Source: state.SourceKubernetes,
	})

	cfg := &Config{
		Overrides: []ServiceOverride{
			{
				Match:               "default/svc",
				HealthEndpoint:      "https://svc.local/health",
				ExpectedStatusCodes: []int{200, 401},
				Icon:                "radarr",
			},
		},
	}

	ApplyOverrides(store, cfg)

	svc, _ := store.Get("default", "svc")
	if svc.HealthEndpoint != "https://svc.local/health" {
		t.Errorf("expected healthEndpoint %q, got %q", "https://svc.local/health", svc.HealthEndpoint)
	}
	if len(svc.ExpectedStatusCodes) != 2 || svc.ExpectedStatusCodes[0] != 200 || svc.ExpectedStatusCodes[1] != 401 {
		t.Errorf("expected expectedStatusCodes [200, 401], got %v", svc.ExpectedStatusCodes)
	}
	if svc.Icon != "radarr" {
		t.Errorf("expected icon %q, got %q", "radarr", svc.Icon)
	}
}
