package state

import (
	"sync"
	"time"
)

// HealthStatus represents the health state of a service.
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
)

// Service source constants.
const (
	SourceKubernetes = "kubernetes"
	SourceConfig     = "config"
)

// PodDiagnostic contains pod-level diagnostic information for K8s services.
// Nil for non-K8s services or when pod status is unavailable.
type PodDiagnostic struct {
	Reason       *string `json:"reason"`
	RestartCount int     `json:"restartCount"`
}

// Service represents a discovered service with health information.
type Service struct {
        Name                string       `json:"name"`
        DisplayName         string       `json:"displayName"`
        OriginalDisplayName string       `json:"originalDisplayName,omitempty"`
        Namespace           string       `json:"namespace"`
        Group               string       `json:"group"`
        URL                 string       `json:"url"`
        Icon                string       `json:"icon,omitempty"`
        Source              string       `json:"source"`
        Status              HealthStatus    `json:"status"`
        CompositeStatus     HealthStatus    `json:"compositeStatus"`
        HTTPCode            *int            `json:"httpCode"`
        ResponseTimeMs      *int64       `json:"responseTimeMs"`
        LastChecked         *time.Time   `json:"lastChecked"`
        LastStateChange     *time.Time   `json:"lastStateChange"`
        ErrorSnippet        *string         `json:"errorSnippet"`
        AuthGuarded         bool            `json:"authGuarded"`
        PodDiagnostic       *PodDiagnostic  `json:"podDiagnostic"`
        HealthURL           string          `json:"healthUrl,omitempty"`
        ExpectedStatusCodes []int        `json:"expectedStatusCodes,omitempty"`
        ReadyEndpoints      *int         `json:"readyEndpoints"`
        TotalEndpoints      *int         `json:"totalEndpoints"`
}
// EventType identifies the kind of state mutation.
type EventType int

const (
	EventDiscovered EventType = iota
	EventRemoved
	EventUpdated
	EventK8sStatus
	EventConfigErrors
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
	configErrors []string
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

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

// SetConfigErrors stores config validation errors for SSE broadcasting.
func (s *Store) SetConfigErrors(errs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := make([]string, len(errs))
	copy(next, errs)
	if stringSlicesEqual(s.configErrors, next) {
		return
	}
	s.configErrors = next

	event := Event{Type: EventConfigErrors}
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

// ConfigErrors returns the current config validation errors.
func (s *Store) ConfigErrors() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.configErrors))
	copy(result, s.configErrors)
	return result
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
	if s.PodDiagnostic != nil {
		pd := *s.PodDiagnostic
		if pd.Reason != nil {
			val := *pd.Reason
			pd.Reason = &val
		}
		cp.PodDiagnostic = &pd
	}
	if s.ExpectedStatusCodes != nil {
		cp.ExpectedStatusCodes = make([]int, len(s.ExpectedStatusCodes))
		copy(cp.ExpectedStatusCodes, s.ExpectedStatusCodes)
	}
	if s.ReadyEndpoints != nil {
		val := *s.ReadyEndpoints
		cp.ReadyEndpoints = &val
	}
	if s.TotalEndpoints != nil {
		val := *s.TotalEndpoints
		cp.TotalEndpoints = &val
	}
	return cp
}
