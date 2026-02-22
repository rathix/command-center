package k8s

import (
	"context"
	"log/slog"
	"sync"

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

// serviceWatch tracks a single per-service EndpointSlice informer.
type serviceWatch struct {
	factory     informers.SharedInformerFactory
	cancel      context.CancelFunc
	serviceName string
	namespace   string
	ingressName string
}

// EndpointSliceWatcher manages per-service EndpointSlice informers.
type EndpointSliceWatcher struct {
	clientset kubernetes.Interface
	updater   EndpointStateUpdater
	logger    *slog.Logger
	mu        sync.Mutex
	watches   map[string]*serviceWatch // key: "namespace/ingressName"
}

// NewEndpointSliceWatcher creates a new EndpointSliceWatcher.
func NewEndpointSliceWatcher(clientset kubernetes.Interface, updater EndpointStateUpdater, logger *slog.Logger) *EndpointSliceWatcher {
	return &EndpointSliceWatcher{
		clientset: clientset,
		updater:   updater,
		logger:    logger,
		watches:   make(map[string]*serviceWatch),
	}
}

// Watch starts an EndpointSlice informer for the given backend service,
// keyed by the Ingress name (since the state store keys by Ingress).
func (e *EndpointSliceWatcher) Watch(ingressName, namespace, backendServiceName string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := namespace + "/" + ingressName

	// If already watching, skip.
	if _, exists := e.watches[key]; exists {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create a namespace-scoped factory with a label selector for the specific service.
	tweakOpts := informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = "kubernetes.io/service-name=" + backendServiceName
	})
	factory := informers.NewSharedInformerFactoryWithOptions(
		e.clientset, 0,
		informers.WithNamespace(namespace),
		tweakOpts,
	)

	sw := &serviceWatch{
		factory:     factory,
		cancel:      cancel,
		serviceName: backendServiceName,
		namespace:   namespace,
		ingressName: ingressName,
	}

	esInformer := factory.Discovery().V1().EndpointSlices()
	esInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) { e.onEndpointSliceEvent(sw) },
		UpdateFunc: func(_, _ interface{}) { e.onEndpointSliceEvent(sw) },
		DeleteFunc: func(_ interface{}) { e.onEndpointSliceEvent(sw) },
	})

	e.watches[key] = sw

	factory.Start(ctx.Done())

	e.logger.Info("started EndpointSlice watch",
		"ingress", ingressName,
		"namespace", namespace,
		"backendService", backendServiceName)
}

// Unwatch stops and removes an EndpointSlice watch for the given Ingress.
func (e *EndpointSliceWatcher) Unwatch(ingressName, namespace string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := namespace + "/" + ingressName
	sw, exists := e.watches[key]
	if !exists {
		return
	}

	sw.cancel()
	sw.factory.Shutdown()
	delete(e.watches, key)

	e.logger.Info("stopped EndpointSlice watch",
		"ingress", ingressName,
		"namespace", namespace)
}

// StopAll cancels and shuts down all active EndpointSlice watches.
func (e *EndpointSliceWatcher) StopAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for key, sw := range e.watches {
		sw.cancel()
		sw.factory.Shutdown()
		delete(e.watches, key)
	}

	e.logger.Info("stopped all EndpointSlice watches")
}

// onEndpointSliceEvent is called whenever an EndpointSlice is added, updated, or deleted.
// It re-aggregates readiness and updates the state store.
func (e *EndpointSliceWatcher) onEndpointSliceEvent(sw *serviceWatch) {
	lister := sw.factory.Discovery().V1().EndpointSlices().Lister()
	slices, err := lister.EndpointSlices(sw.namespace).List(labels.Everything())
	if err != nil {
		e.logger.Warn("failed to list EndpointSlices",
			"namespace", sw.namespace,
			"service", sw.serviceName,
			"error", err)
		return
	}

	ready, total := aggregateEndpointReadiness(slices)

	e.updater.Update(sw.namespace, sw.ingressName, func(svc *state.Service) {
		svc.ReadyEndpoints = &ready
		svc.TotalEndpoints = &total
	})
}

// aggregateEndpointReadiness counts ready and total endpoints across all slices.
// A nil Ready condition is treated as not ready.
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
