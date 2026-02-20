package state

import (
	"fmt"
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

	code := 200
	services := []Service{
		{Name: "svc-a", Namespace: "ns1", URL: "https://a.example.com", Status: StatusUnknown, HTTPCode: &code},
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

	// Verify All returns a snapshot with deep copies (modifying returned pointers shouldn't affect store)
	if all[0].HTTPCode == nil {
		t.Fatal("expected HTTPCode pointer to be populated")
	}
	*all[0].HTTPCode = 500
	
	got, _ := store.Get("ns1", "svc-a")
	if *got.HTTPCode != 200 {
		t.Errorf("All() returned a shallow copy, modifying nested pointer affected store: got %d, want 200", *got.HTTPCode)
	}
}

func TestStoreDeepCopy(t *testing.T) {
	code := 200
	now := time.Now()
	err := "failed"
	
	s := Service{
		Name:         "test",
		HTTPCode:     &code,
		LastChecked:  &now,
		ErrorSnippet: &err,
	}
	
	cp := s.DeepCopy()
	
	// Modify original pointers
	code = 404
	now = now.Add(time.Hour)
	err = "changed"
	
	if *cp.HTTPCode != 200 {
		t.Errorf("DeepCopy failed for HTTPCode: got %d, want 200", *cp.HTTPCode)
	}
	if !cp.LastChecked.Before(now) {
		t.Error("DeepCopy failed for LastChecked")
	}
	if *cp.ErrorSnippet != "failed" {
		t.Errorf("DeepCopy failed for ErrorSnippet: got %q, want \"failed\"", *cp.ErrorSnippet)
	}
}

func TestStoreGetDeepCopy(t *testing.T) {
	store := NewStore()
	code := 200
	store.AddOrUpdate(Service{Name: "svc", Namespace: "ns", HTTPCode: &code})
	
	got, _ := store.Get("ns", "svc")
	*got.HTTPCode = 500
	
	got2, _ := store.Get("ns", "svc")
	if *got2.HTTPCode != 200 {
		t.Error("Get() returned a reference/shallow copy, modifying it affected store")
	}
}

func TestStoreMultipleSubscribers(t *testing.T) {
	store := NewStore()
	
	sub1 := store.Subscribe()
	sub2 := store.Subscribe()
	
	svc := Service{Name: "web", Namespace: "default"}
	store.AddOrUpdate(svc)
	
	// Both should receive the event
	for i, sub := range []<-chan Event{sub1, sub2} {
		select {
		case evt := <-sub:
			if evt.Service.Name != "web" {
				t.Errorf("sub%d: expected 'web', got %q", i+1, evt.Service.Name)
			}
		case <-time.After(time.Second):
			t.Fatalf("sub%d: timed out waiting for event", i+1)
		}
	}
	
	// Unsubscribe sub1
	store.Unsubscribe(sub1)
	
	store.Remove("default", "web")
	
	// sub1 should be closed or not receive anything
	select {
	case _, ok := <-sub1:
		if ok {
			t.Error("sub1 should have been closed after unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
		// Unsubscribe closes the channel in my implementation
	}
	
	// sub2 should still receive the event
	select {
	case evt := <-sub2:
		if evt.Type != EventRemoved {
			t.Errorf("sub2: expected EventRemoved, got %v", evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("sub2: timed out waiting for event after sub1 unsubscribed")
	}
}

func TestSubscribeDiscoveredEvent(t *testing.T) {
	store := NewStore()
	events := store.Subscribe()

	svc := Service{
		Name:      "web",
		Namespace: "default",
		URL:       "https://web.example.com",
		Status:    StatusUnknown,
	}
	store.AddOrUpdate(svc)

	select {
	case evt := <-events:
		if evt.Type != EventDiscovered {
			t.Errorf("expected EventDiscovered, got %v", evt.Type)
		}
		if evt.Service.Name != "web" {
			t.Errorf("expected service name 'web', got %q", evt.Service.Name)
		}
		if evt.Service.Namespace != "default" {
			t.Errorf("expected namespace 'default', got %q", evt.Service.Namespace)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for discovered event")
	}
}

func TestSubscribeUpdatedEvent(t *testing.T) {
	store := NewStore()
	events := store.Subscribe()

	svc := Service{
		Name:      "api",
		Namespace: "prod",
		URL:       "https://api.example.com",
		Status:    StatusUnknown,
	}
	store.AddOrUpdate(svc) // First add → Discovered

	// Drain the discovered event
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial discovered event")
	}

	// Update the same service → Updated
	svc.Status = StatusHealthy
	store.AddOrUpdate(svc)

	select {
	case evt := <-events:
		if evt.Type != EventUpdated {
			t.Errorf("expected EventUpdated, got %v", evt.Type)
		}
		if evt.Service.Status != StatusHealthy {
			t.Errorf("expected status %q, got %q", StatusHealthy, evt.Service.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for updated event")
	}
}

func TestSubscribeRemovedEvent(t *testing.T) {
	store := NewStore()
	events := store.Subscribe()

	svc := Service{
		Name:      "worker",
		Namespace: "jobs",
		URL:       "https://worker.example.com",
		Status:    StatusUnknown,
	}
	store.AddOrUpdate(svc)

	// Drain the discovered event
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for discovered event")
	}

	store.Remove("jobs", "worker")

	select {
	case evt := <-events:
		if evt.Type != EventRemoved {
			t.Errorf("expected EventRemoved, got %v", evt.Type)
		}
		if evt.Namespace != "jobs" {
			t.Errorf("expected namespace 'jobs', got %q", evt.Namespace)
		}
		if evt.Name != "worker" {
			t.Errorf("expected name 'worker', got %q", evt.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for removed event")
	}
}

func TestSubscribeNoEventForMissingRemove(t *testing.T) {
	store := NewStore()
	events := store.Subscribe()

	store.Remove("default", "missing")

	select {
	case evt := <-events:
		t.Fatalf("expected no event for missing remove, got %+v", evt)
	case <-time.After(50 * time.Millisecond):
		// Success: no event emitted.
	}
}

func TestSubscribeNonBlocking(t *testing.T) {
	store := NewStore()
	// Don't read from the subscribe channel — fill the buffer
	_ = store.Subscribe()

	// Send more events than the buffer size (64) without reading
	// This must not block or panic
	done := make(chan struct{})
	go func() {
		for i := range 100 {
			store.AddOrUpdate(Service{
				Name:      fmt.Sprintf("svc-%d", i),
				Namespace: "default",
				URL:       "https://example.com",
				Status:    StatusUnknown,
			})
		}
		close(done)
	}()

	select {
	case <-done:
		// Success — mutations did not block
	case <-time.After(2 * time.Second):
		t.Fatal("mutations blocked due to full event channel")
	}
}

func TestSubscribeEventOrder(t *testing.T) {
	store := NewStore()
	events := store.Subscribe()

	// Add svc-a (discovered), update svc-a (updated), remove svc-a (removed)
	store.AddOrUpdate(Service{Name: "svc-a", Namespace: "ns", URL: "https://a.example.com", Status: StatusUnknown})
	store.AddOrUpdate(Service{Name: "svc-a", Namespace: "ns", URL: "https://a.example.com", Status: StatusHealthy})
	store.Remove("ns", "svc-a")

	expected := []EventType{EventDiscovered, EventUpdated, EventRemoved}
	for i, want := range expected {
		select {
		case evt := <-events:
			if evt.Type != want {
				t.Errorf("event[%d]: expected %v, got %v", i, want, evt.Type)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for event[%d]", i)
		}
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
