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
	mu       sync.RWMutex
	services map[string]Service
	sub      chan Event
}

// NewStore creates a new empty Store with a buffered event channel.
func NewStore() *Store {
	return &Store{
		services: make(map[string]Service),
		sub:      make(chan Event, 64),
	}
}

// Subscribe returns a read-only channel that receives events for every state mutation.
func (s *Store) Subscribe() <-chan Event {
	return s.sub
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
	s.services[key] = svc
	eventType := EventDiscovered
	if exists {
		eventType = EventUpdated
	}
	// Non-blocking send to prevent slow consumers from blocking state mutations.
	select {
	case s.sub <- Event{Type: eventType, Service: svc}:
	default:
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
	// Non-blocking send to prevent slow consumers from blocking state mutations.
	select {
	case s.sub <- Event{Type: EventRemoved, Namespace: namespace, Name: name}:
	default:
	}
	s.mu.Unlock()
}

// Get retrieves a single service by namespace and name.
func (s *Store) Get(namespace, name string) (Service, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	svc, ok := s.services[serviceKey(namespace, name)]
	return svc, ok
}

// All returns a snapshot of all services in the store.
func (s *Store) All() []Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Service, 0, len(s.services))
	for _, svc := range s.services {
		result = append(result, svc)
	}
	return result
}
