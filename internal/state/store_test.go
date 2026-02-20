package state

import (
	"sync"
	"testing"
	"time"
)

func TestServiceStructJSONTags(t *testing.T) {
	// Verify Service struct exists and can be instantiated with expected fields
	now := time.Now()
	code := 200
	responseTime := int64(42)
	errSnippet := "connection refused"

	svc := Service{
		Name:            "my-app",
		Namespace:       "default",
		URL:             "https://my-app.example.com",
		Status:          StatusUnknown,
		HTTPCode:        &code,
		ResponseTimeMs:  &responseTime,
		LastChecked:     &now,
		LastStateChange: &now,
		ErrorSnippet:    &errSnippet,
	}

	if svc.Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", svc.Name)
	}
	if svc.Status != StatusUnknown {
		t.Errorf("expected status %q, got %q", StatusUnknown, svc.Status)
	}
}

func TestHealthStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant HealthStatus
		want     string
	}{
		{"healthy", StatusHealthy, "healthy"},
		{"unhealthy", StatusUnhealthy, "unhealthy"},
		{"authBlocked", StatusAuthBlocked, "authBlocked"},
		{"unknown", StatusUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.want {
				t.Errorf("expected %q, got %q", tt.want, tt.constant)
			}
		})
	}
}

func TestNewStore(t *testing.T) {
	store := NewStore()
	if store == nil {
		t.Fatal("NewStore returned nil")
	}

	services := store.All()
	if len(services) != 0 {
		t.Errorf("new store should be empty, got %d services", len(services))
	}
}

func TestStoreAddOrUpdateAndGet(t *testing.T) {
	store := NewStore()

	svc := Service{
		Name:      "web",
		Namespace: "production",
		URL:       "https://web.example.com",
		Status:    StatusUnknown,
	}

	store.AddOrUpdate(svc)

	got, ok := store.Get("production", "web")
	if !ok {
		t.Fatal("expected to find service 'production/web'")
	}
	if got.Name != "web" {
		t.Errorf("expected name 'web', got %q", got.Name)
	}
	if got.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", got.Namespace)
	}
	if got.URL != "https://web.example.com" {
		t.Errorf("expected URL 'https://web.example.com', got %q", got.URL)
	}
	if got.Status != StatusUnknown {
		t.Errorf("expected status %q, got %q", StatusUnknown, got.Status)
	}
}

func TestStoreAddOrUpdateOverwrite(t *testing.T) {
	store := NewStore()

	svc := Service{
		Name:      "api",
		Namespace: "default",
		URL:       "https://api.example.com",
		Status:    StatusUnknown,
	}
	store.AddOrUpdate(svc)

	// Update the service
	code := 200
	svc.Status = StatusHealthy
	svc.HTTPCode = &code
	store.AddOrUpdate(svc)

	got, ok := store.Get("default", "api")
	if !ok {
		t.Fatal("expected to find service 'default/api'")
	}
	if got.Status != StatusHealthy {
		t.Errorf("expected status %q, got %q", StatusHealthy, got.Status)
	}
	if got.HTTPCode == nil || *got.HTTPCode != 200 {
		t.Errorf("expected httpCode 200, got %v", got.HTTPCode)
	}

	// Should still be only one service
	all := store.All()
	if len(all) != 1 {
		t.Errorf("expected 1 service after update, got %d", len(all))
	}
}

func TestStoreGetNotFound(t *testing.T) {
	store := NewStore()

	_, ok := store.Get("default", "nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent service")
	}
}

func TestStoreRemove(t *testing.T) {
	store := NewStore()

	svc := Service{
		Name:      "worker",
		Namespace: "jobs",
		URL:       "https://worker.example.com",
		Status:    StatusUnknown,
	}
	store.AddOrUpdate(svc)

	store.Remove("jobs", "worker")

	_, ok := store.Get("jobs", "worker")
	if ok {
		t.Error("expected service to be removed")
	}

	all := store.All()
	if len(all) != 0 {
		t.Errorf("expected empty store after remove, got %d services", len(all))
	}
}

func TestStoreRemoveNonexistent(t *testing.T) {
	store := NewStore()

	// Should not panic
	store.Remove("default", "nonexistent")
}

func TestStoreAll(t *testing.T) {
	store := NewStore()

	services := []Service{
		{Name: "svc-a", Namespace: "ns1", URL: "https://a.example.com", Status: StatusUnknown},
		{Name: "svc-b", Namespace: "ns1", URL: "https://b.example.com", Status: StatusHealthy},
		{Name: "svc-c", Namespace: "ns2", URL: "https://c.example.com", Status: StatusUnhealthy},
	}

	for _, svc := range services {
		store.AddOrUpdate(svc)
	}

	all := store.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 services, got %d", len(all))
	}

	// Verify All returns a snapshot (modifying returned slice shouldn't affect store)
	all[0].Name = "mutated"
	got, ok := store.Get("ns1", "svc-a")
	if !ok {
		t.Fatal("expected to find service 'ns1/svc-a'")
	}
	if got.Name == "mutated" {
		t.Error("All() should return a snapshot, not a reference to internal data")
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	store := NewStore()
	const goroutines = 100
	const operations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // writers, readers, removers

	// Writers
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range operations {
				svc := Service{
					Name:      "svc",
					Namespace: "ns",
					URL:       "https://svc.example.com",
					Status:    HealthStatus("status-" + string(rune('0'+j%10))),
				}
				_ = id // suppress unused
				store.AddOrUpdate(svc)
			}
		}(i)
	}

	// Readers
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for range operations {
				_ = id
				store.Get("ns", "svc")
				store.All()
			}
		}(i)
	}

	// Removers
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for range operations {
				_ = id
				store.Remove("ns", "svc")
			}
		}(i)
	}

	wg.Wait()
}
