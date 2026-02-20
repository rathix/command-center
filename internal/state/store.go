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

// Store is a concurrency-safe in-memory store for discovered services.
// Services are keyed by "namespace/name".
type Store struct {
	mu       sync.RWMutex
	services map[string]Service
}

// NewStore creates a new empty Store.
func NewStore() *Store {
	return &Store{
		services: make(map[string]Service),
	}
}

func serviceKey(namespace, name string) string {
	return namespace + "/" + name
}

// AddOrUpdate inserts or replaces a service in the store.
func (s *Store) AddOrUpdate(svc Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[serviceKey(svc.Namespace, svc.Name)] = svc
}

// Remove deletes a service from the store.
func (s *Store) Remove(namespace, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.services, serviceKey(namespace, name))
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
