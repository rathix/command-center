package k8s

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rathix/command-center/internal/state"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// EndpointStateUpdater is the consumer-defined interface for updating endpoint
// readiness on a service in the state store.
type EndpointStateUpdater interface {
	Update(namespace, name string, fn func(*state.Service))
}

// EndpointSliceWatcher manages a single cluster-wide EndpointSlice informer
// and updates state for all registered services.
type EndpointSliceWatcher struct {
	clientset      kubernetes.Interface
	updater        EndpointStateUpdater
	logger         *slog.Logger
	podDiagQuerier *PodDiagnosticQuerier

	factory  informers.SharedInformerFactory
	informer cache.SharedIndexInformer
	cancel   context.CancelFunc

	mu sync.RWMutex
	// serviceToIngress maps "namespace/serviceName" to a set of ingress names
	serviceToIngress map[string]map[string]struct{}
}

// NewEndpointSliceWatcher creates a new EndpointSliceWatcher with a cluster-wide informer.
func NewEndpointSliceWatcher(clientset kubernetes.Interface, updater EndpointStateUpdater, logger *slog.Logger) *EndpointSliceWatcher {
	return NewEndpointSliceWatcherWithTweak(clientset, updater, logger, nil)
}

// NewEndpointSliceWatcherWithTweak allows providing a tweak function for the informer factory.
func NewEndpointSliceWatcherWithTweak(clientset kubernetes.Interface, updater EndpointStateUpdater, logger *slog.Logger, tweak func(*metav1.ListOptions)) *EndpointSliceWatcher {
	ctx, cancel := context.WithCancel(context.Background())

	// Cluster-wide informer factory
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithTweakListOptions(tweak))
	informer := factory.Discovery().V1().EndpointSlices().Informer()

	e := &EndpointSliceWatcher{
		clientset:        clientset,
		updater:          updater,
		logger:           logger,
		podDiagQuerier:   NewPodDiagnosticQuerier(clientset, logger),
		factory:          factory,
		informer:         informer,
		cancel:           cancel,
		serviceToIngress: make(map[string]map[string]struct{}),
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    e.onAdd,
		UpdateFunc: e.onUpdate,
		DeleteFunc: e.onDelete,
	})

	factory.Start(ctx.Done())

	return e
}

func (e *EndpointSliceWatcher) onAdd(obj interface{}) {
	e.handleEvent(obj)
}

func (e *EndpointSliceWatcher) onUpdate(_, newObj interface{}) {
	e.handleEvent(newObj)
}

func (e *EndpointSliceWatcher) onDelete(obj interface{}) {
	e.handleEvent(obj)
}

func (e *EndpointSliceWatcher) handleEvent(obj interface{}) {
	slice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		slice, ok = tombstone.Obj.(*discoveryv1.EndpointSlice)
		if !ok {
			return
		}
	}

	serviceName := slice.Labels["kubernetes.io/service-name"]
	if serviceName == "" {
		return
	}

	e.triggerUpdate(slice.Namespace, serviceName)
}

func (e *EndpointSliceWatcher) triggerUpdate(namespace, serviceName string) {
	e.mu.RLock()
	ingresses, ok := e.serviceToIngress[namespace+"/"+serviceName]
	if !ok || len(ingresses) == 0 {
		e.mu.RUnlock()
		return
	}

	// Copy ingress names to avoid holding lock during update
	targets := make([]string, 0, len(ingresses))
	for name := range ingresses {
		targets = append(targets, name)
	}
	e.mu.RUnlock()

	// List all slices for this service to aggregate readiness
	lister := e.factory.Discovery().V1().EndpointSlices().Lister()
	selector := labels.SelectorFromSet(labels.Set{"kubernetes.io/service-name": serviceName})
	slices, err := lister.EndpointSlices(namespace).List(selector)
	if err != nil {
		e.logger.Warn("failed to list EndpointSlices for update", "namespace", namespace, "service", serviceName, "error", err)
		return
	}

	ready, total := aggregateEndpointReadiness(slices)
	notReadyPods := extractNotReadyPodNames(slices)

	for _, ingressName := range targets {
		e.updater.Update(namespace, ingressName, func(svc *state.Service) {
			svc.ReadyEndpoints = &ready
			svc.TotalEndpoints = &total
			// Clear stale diagnostics when there are no currently identified not-ready pods.
			// This also handles 0/0 endpoint transitions where prior pod diagnostics are no longer valid.
			if len(notReadyPods) == 0 {
				svc.PodDiagnostic = nil
			}
		})

		if len(notReadyPods) > 0 {
			go e.queryAndStorePodDiagnostics(namespace, ingressName, notReadyPods)
		}
	}
}

// Watch registers an interest in EndpointSlices for the given backend service.
func (e *EndpointSliceWatcher) Watch(ingressName, namespace, backendServiceName string) {
	e.mu.Lock()
	key := namespace + "/" + backendServiceName
	if _, ok := e.serviceToIngress[key]; !ok {
		e.serviceToIngress[key] = make(map[string]struct{})
	}
	e.serviceToIngress[key][ingressName] = struct{}{}
	e.mu.Unlock()

	e.logger.Info("started EndpointSlice watch",
		"ingress", ingressName,
		"namespace", namespace,
		"backendService", backendServiceName)

	// Trigger immediate update to populate initial readiness if informer is already synced
	e.triggerUpdate(namespace, backendServiceName)
}

// Unwatch removes registration for the given Ingress.
func (e *EndpointSliceWatcher) Unwatch(ingressName, namespace string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// We don't know the serviceName here, so we must scan.
	// This is infrequent (ingress delete/update).
	for key, ingresses := range e.serviceToIngress {
		if strings.HasPrefix(key, namespace+"/") {
			if _, ok := ingresses[ingressName]; ok {
				delete(ingresses, ingressName)
				if len(ingresses) == 0 {
					delete(e.serviceToIngress, key)
				}
				e.logger.Info("stopped EndpointSlice watch", "ingress", ingressName, "namespace", namespace)
				return
			}
		}
	}
}

// StopAll shuts down the informer factory.
func (e *EndpointSliceWatcher) StopAll() {
	e.cancel()
	e.factory.Shutdown()

	e.mu.Lock()
	e.serviceToIngress = make(map[string]map[string]struct{})
	e.mu.Unlock()

	e.logger.Info("stopped all EndpointSlice watches")
}

// WaitForSync waits for the internal informer to sync.
func (e *EndpointSliceWatcher) WaitForSync(ctx context.Context) bool {
	syncStatus := e.factory.WaitForCacheSync(ctx.Done())
	for _, synced := range syncStatus {
		if !synced {
			return false
		}
	}
	return len(syncStatus) > 0
}

// aggregateEndpointReadiness counts ready and total endpoints across all slices.
func aggregateEndpointReadiness(slices []*discoveryv1.EndpointSlice) (ready, total int) {
	for _, slice := range slices {
		for _, ep := range slice.Endpoints {
			total++
			if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
				ready++
			}
		}
	}
	return ready, total
}

func extractNotReadyPodNames(slices []*discoveryv1.EndpointSlice) []string {
	seen := make(map[string]struct{})
	var names []string

	for _, slice := range slices {
		for _, ep := range slice.Endpoints {
			isReady := ep.Conditions.Ready != nil && *ep.Conditions.Ready
			if isReady || ep.TargetRef == nil {
				continue
			}
			if ep.TargetRef.Kind != "Pod" || ep.TargetRef.Name == "" {
				continue
			}
			if _, exists := seen[ep.TargetRef.Name]; exists {
				continue
			}
			seen[ep.TargetRef.Name] = struct{}{}
			names = append(names, ep.TargetRef.Name)
		}
	}

	return names
}

// queryAndStorePodDiagnostics ...
func (e *EndpointSliceWatcher) queryAndStorePodDiagnostics(namespace, ingressName string, podNames []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	diag := e.podDiagQuerier.QueryForService(ctx, namespace, podNames)
	e.updater.Update(namespace, ingressName, func(svc *state.Service) {
		svc.PodDiagnostic = diag
	})
}

// RegisteredServiceCount returns the number of services being watched.
// Used for internal testing.
func (e *EndpointSliceWatcher) RegisteredServiceCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.serviceToIngress)
}
