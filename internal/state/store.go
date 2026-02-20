package state

import (
	"sync"
	"time"
)

// HealthStatus represents the health state of a service.
type HealthStatus string

const (
	StatusHealthy     HealthStatus = "healthy"
	StatusUnhealthy   HealthStatus = "unhealthy"
	StatusAuthBlocked HealthStatus = "authBlocked"
	StatusUnknown     HealthStatus = "unknown"
)

// Service represents a discovered Kubernetes service with health information.
type Service struct {
	Name            string       `json:"name"`
	DisplayName     string       `json:"displayName"`
	Namespace       string       `json:"namespace"`
	URL             string       `json:"url"`
	Status          HealthStatus `json:"status"`
	HTTPCode        *int         `json:"httpCode"`
	ResponseTimeMs  *int64       `json:"responseTimeMs"`
	LastChecked     *time.Time   `json:"lastChecked"`
	LastStateChange *time.Time   `json:"lastStateChange"`
	ErrorSnippet    *string      `json:"errorSnippet"`
}

// EventType identifies the kind of state mutation.
type EventType int

const (
	EventDiscovered EventType = iota
	EventRemoved
	EventUpdated
	EventK8sStatus
)

// Event represents a state mutation notification.
type Event struct {
	Type      EventType
	Service   Service // Populated for Discovered/Updated
	Namespace string  // Populated for Removed
	Name      string  // Populated for Removed
}

// Store is a concurrency-safe in-memory store for discovered services.
// Services are keyed by "namespace/name".
type Store struct {
	mu           sync.RWMutex
	services     map[string]Service
	subs         map[chan Event]struct{}
	k8sConnected bool
	lastK8sEvent time.Time
}

// NewStore creates a new empty Store.
func NewStore() *Store {
	return &Store{
		services: make(map[string]Service),
		subs:     make(map[chan Event]struct{}),
	}
}

// Subscribe returns a read-only channel that receives events for every state mutation.
// The caller is responsible for ensuring the channel is consumed to prevent blocking.
// In a production environment, this should include a way to Unsubscribe.
func (s *Store) Subscribe() <-chan Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan Event, 128) // Increased buffer for better tolerance
	s.subs[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscription channel.
func (s *Store) Unsubscribe(ch <-chan Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Cast back to writable channel for deletion
	for existing := range s.subs {
		if (<-chan Event)(existing) == ch {
			delete(s.subs, existing)
			close(existing)
			return
		}
	}
}

func serviceKey(namespace, name string) string {
	return namespace + "/" + name
}

// AddOrUpdate inserts or replaces a service in the store.
// It sends an EventDiscovered event for new services or an EventUpdated event for existing ones.
func (s *Store) AddOrUpdate(svc Service) {
	s.mu.Lock()
	key := serviceKey(svc.Namespace, svc.Name)
	_, exists := s.services[key]
	
	// Store a deep copy to prevent external mutation of shared pointers
	s.services[key] = svc.DeepCopy()
	
	eventType := EventDiscovered
	if exists {
		eventType = EventUpdated
	}
	
	// Fan-out to all subscribers
	event := Event{Type: eventType, Service: svc.DeepCopy()}
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
			// Buffer full, subscriber is too slow. 
			// In a more robust system, we might flag the client for reconnection.
		}
	}
	s.mu.Unlock()
}

// Remove deletes a service from the store and sends an EventRemoved notification.
func (s *Store) Remove(namespace, name string) {
	s.mu.Lock()
	key := serviceKey(namespace, name)
	if _, exists := s.services[key]; !exists {
		s.mu.Unlock()
		return
	}
	delete(s.services, key)
	
	// Fan-out to all subscribers
	event := Event{Type: EventRemoved, Namespace: namespace, Name: name}
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
	s.mu.Unlock()
}

// Get retrieves a single service by namespace and name.
func (s *Store) Get(namespace, name string) (Service, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	svc, ok := s.services[serviceKey(namespace, name)]
	if !ok {
		return Service{}, false
	}
	return svc.DeepCopy(), true
}

// All returns a snapshot of all services in the store.
func (s *Store) All() []Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Service, 0, len(s.services))
	for _, svc := range s.services {
		result = append(result, svc.DeepCopy())
	}
	return result
}

// Update performs a thread-safe read-modify-write operation on a single service.
// The provided function 'fn' is called with a pointer to the service while the store is locked.
// If the service does not exist, 'fn' is not called.
func (s *Store) Update(namespace, name string, fn func(*Service)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := serviceKey(namespace, name)
	svc, ok := s.services[key]
	if !ok {
		return
	}

	fn(&svc)
	s.services[key] = svc

	// Fan-out to all subscribers
	event := Event{Type: EventUpdated, Service: svc.DeepCopy()}
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

// SetK8sConnected updates the K8s connectivity status and notifies subscribers.
func (s *Store) SetK8sConnected(connected bool) {
	s.mu.Lock()
	s.k8sConnected = connected
	s.lastK8sEvent = time.Now()
	event := Event{Type: EventK8sStatus}
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
	s.mu.Unlock()
}

// K8sConnected returns whether the K8s API is currently reachable.
func (s *Store) K8sConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.k8sConnected
}

// LastK8sEvent returns the time of the last K8s connectivity status change.
func (s *Store) LastK8sEvent() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastK8sEvent
}

// DeepCopy creates a complete copy of the Service, including pointer fields.
func (s Service) DeepCopy() Service {
	cp := s
	if s.HTTPCode != nil {
		val := *s.HTTPCode
		cp.HTTPCode = &val
	}
	if s.ResponseTimeMs != nil {
		val := *s.ResponseTimeMs
		cp.ResponseTimeMs = &val
	}
	if s.LastChecked != nil {
		val := *s.LastChecked
		cp.LastChecked = &val
	}
	if s.LastStateChange != nil {
		val := *s.LastStateChange
		cp.LastStateChange = &val
	}
	if s.ErrorSnippet != nil {
		val := *s.ErrorSnippet
		cp.ErrorSnippet = &val
	}
	return cp
}

