package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/rathix/command-center/internal/state"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// StateUpdater is the interface the watcher uses to update service state.
// Defined here at the consumer, not in the state package.
type StateUpdater interface {
	Get(namespace, name string) (state.Service, bool)
	AddOrUpdate(svc state.Service)
	Remove(namespace, name string)
	SetK8sConnected(connected bool)
}

// IngressLister defines the subset of the Kubernetes Ingress lister
// required by the consumer for testability (satisfies AC #5).
type IngressLister interface {
	List(selector labels.Selector) (ret []*networkingv1.Ingress, err error)
	Ingresses(namespace string) networkingv1listers.IngressNamespaceLister
}

// Watcher watches Kubernetes Ingress resources and updates the state store.
type Watcher struct {
	factory      informers.SharedInformerFactory
	lister       IngressLister
	updater      StateUpdater
	logger       *slog.Logger
	k8sConnected atomic.Bool
}

// NewWatcher creates a Watcher from a kubeconfig path. Supports both
// external kubeconfig files and in-cluster config (when kubeconfigPath is "").
func NewWatcher(kubeconfigPath string, updater StateUpdater, logger *slog.Logger) (*Watcher, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("k8s watcher: build config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("k8s watcher: create clientset: %w", err)
	}

	return NewWatcherWithClient(clientset, updater, logger), nil
}

// NewWatcherWithClient creates a Watcher from an existing Kubernetes clientset.
// This is primarily used for testing with fake clientsets.
func NewWatcherWithClient(clientset kubernetes.Interface, updater StateUpdater, logger *slog.Logger) *Watcher {
	factory := informers.NewSharedInformerFactory(clientset, 0)
	ingressInformer := factory.Networking().V1().Ingresses()

	w := &Watcher{
		factory: factory,
		lister:  ingressInformer.Lister(),
		updater: updater,
		logger:  logger,
	}

	if err := ingressInformer.Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		w.logger.Warn("k8s API watch error", "error", err)
		if w.k8sConnected.CompareAndSwap(true, false) {
			w.updater.SetK8sConnected(false)
		}
	}); err != nil {
		w.logger.Warn("failed to set k8s watch error handler", "error", err)
	}

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.onAdd,
		UpdateFunc: w.onUpdate,
		DeleteFunc: w.onDelete,
	})

	return w
}

// Run starts the informer factory and blocks until the context is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	w.logger.Info("Starting Kubernetes Ingress watcher")
	w.factory.Start(ctx.Done())
	syncStatus := w.factory.WaitForCacheSync(ctx.Done())
	allSynced := len(syncStatus) > 0
	for _, synced := range syncStatus {
		if !synced {
			allSynced = false
			break
		}
	}

	if allSynced {
		w.markK8sConnected()
		w.logger.Info("Kubernetes Ingress watcher synced")
	} else {
		w.logger.Warn("Kubernetes Ingress watcher cache sync incomplete")
	}

	<-ctx.Done()
	w.factory.Shutdown()
	w.logger.Info("Kubernetes Ingress watcher stopped")
}

func (w *Watcher) markK8sConnected() {
	if w.k8sConnected.CompareAndSwap(false, true) {
		w.updater.SetK8sConnected(true)
		w.logger.Info("k8s API connection recovered")
	}
}

func (w *Watcher) onAdd(obj interface{}) {
	w.markK8sConnected()

	ingress, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return
	}

	url, host, ok := extractServiceURL(ingress)
	if !ok {
		w.logger.Warn("skipping Ingress with no valid host",
			"namespace", ingress.Namespace,
			"name", ingress.Name)
		return
	}

	svc := state.Service{
		Name:        ingress.Name,
		DisplayName: displayName(host),
		Namespace:   ingress.Namespace,
		URL:         url,
		Status:      state.StatusUnknown,
	}
	w.updater.AddOrUpdate(svc)
	w.logger.Info("service discovered",
		"namespace", ingress.Namespace,
		"name", ingress.Name,
		"url", url)
}

func (w *Watcher) onUpdate(oldObj, newObj interface{}) {
	w.markK8sConnected()

	ingress, ok := newObj.(*networkingv1.Ingress)
	if !ok {
		return
	}

	url, host, ok := extractServiceURL(ingress)
	if !ok {
		w.logger.Warn("skipping Ingress with no valid host after update",
			"namespace", ingress.Namespace,
			"name", ingress.Name)
		return
	}

	// Preserve health status if service already exists
	status := state.StatusUnknown
	if existing, ok := w.updater.Get(ingress.Namespace, ingress.Name); ok {
		status = existing.Status
	}

	svc := state.Service{
		Name:        ingress.Name,
		DisplayName: displayName(host),
		Namespace:   ingress.Namespace,
		URL:         url,
		Status:      status,
	}
	w.updater.AddOrUpdate(svc)
	w.logger.Info("service updated",
		"namespace", ingress.Namespace,
		"name", ingress.Name,
		"url", url)
}

func (w *Watcher) onDelete(obj interface{}) {
	w.markK8sConnected()

	ingress, ok := obj.(*networkingv1.Ingress)
	if !ok {
		// Handle DeletedFinalStateUnknown
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		ingress, ok = tombstone.Obj.(*networkingv1.Ingress)
		if !ok {
			return
		}
	}

	w.updater.Remove(ingress.Namespace, ingress.Name)
	w.logger.Info("service removed",
		"namespace", ingress.Namespace,
		"name", ingress.Name)
}

// displayName extracts a human-friendly display name from a hostname
// by taking the prefix before the first dot.
func displayName(host string) string {
	if i := strings.IndexByte(host, '.'); i > 0 {
		return host[:i]
	}
	return host
}

// extractServiceURL extracts the service URL and raw host from an Ingress spec.
// Returns the URL, host, and true if valid, or empty strings and false if the
// Ingress has no host defined in its rules or TLS config.
func extractServiceURL(ingress *networkingv1.Ingress) (string, string, bool) {
	var host string

	// 1. Try to get host from first rule
	if len(ingress.Spec.Rules) > 0 {
		host = ingress.Spec.Rules[0].Host
	}

	// 2. Fallback to first TLS host if rule host is empty
	if host == "" && len(ingress.Spec.TLS) > 0 && len(ingress.Spec.TLS[0].Hosts) > 0 {
		host = ingress.Spec.TLS[0].Hosts[0]
	}

	if host == "" {
		return "", "", false
	}

	scheme := "http"
	for _, tls := range ingress.Spec.TLS {
		for _, tlsHost := range tls.Hosts {
			if tlsHost == host {
				scheme = "https"
				break
			}
		}
		if scheme == "https" {
			break
		}
	}

	return scheme + "://" + host, host, true
}
