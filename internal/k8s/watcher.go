package k8s

import (
	"context"
	"fmt"
	"log/slog"

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
	AddOrUpdate(svc state.Service)
	Remove(namespace, name string)
}

// IngressLister defines the subset of the Kubernetes Ingress lister
// required by the consumer for testability (satisfies AC #5).
type IngressLister interface {
	List(selector labels.Selector) (ret []*networkingv1.Ingress, err error)
	Ingresses(namespace string) networkingv1listers.IngressNamespaceLister
}

// Watcher watches Kubernetes Ingress resources and updates the state store.
type Watcher struct {
	factory informers.SharedInformerFactory
	lister  networkingv1listers.IngressLister
	updater StateUpdater
	logger  *slog.Logger
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
	w.factory.WaitForCacheSync(ctx.Done())
	<-ctx.Done()
	w.factory.Shutdown()
	w.logger.Info("Kubernetes Ingress watcher stopped")
}

func (w *Watcher) onAdd(obj interface{}) {
	ingress, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return
	}

	url, ok := extractServiceURL(ingress)
	if !ok {
		w.logger.Warn("skipping Ingress with no valid host",
			"namespace", ingress.Namespace,
			"name", ingress.Name)
		return
	}

	svc := state.Service{
		Name:      ingress.Name,
		Namespace: ingress.Namespace,
		URL:       url,
		Status:    state.StatusUnknown,
	}
	w.updater.AddOrUpdate(svc)
	w.logger.Info("service discovered",
		"namespace", ingress.Namespace,
		"name", ingress.Name,
		"url", url)
}

func (w *Watcher) onUpdate(oldObj, newObj interface{}) {
	ingress, ok := newObj.(*networkingv1.Ingress)
	if !ok {
		return
	}

	url, ok := extractServiceURL(ingress)
	if !ok {
		w.logger.Warn("skipping Ingress with no valid host after update",
			"namespace", ingress.Namespace,
			"name", ingress.Name)
		return
	}

	svc := state.Service{
		Name:      ingress.Name,
		Namespace: ingress.Namespace,
		URL:       url,
		Status:    state.StatusUnknown,
	}
	w.updater.AddOrUpdate(svc)
	w.logger.Debug("service updated",
		"namespace", ingress.Namespace,
		"name", ingress.Name,
		"url", url)
}

func (w *Watcher) onDelete(obj interface{}) {
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

// extractServiceURL extracts the service URL from an Ingress spec.
// Returns the URL and true if valid, or empty string and false if the
// Ingress has no host defined in its rules or TLS config.
func extractServiceURL(ingress *networkingv1.Ingress) (string, bool) {
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
		return "", false
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

	return scheme + "://" + host, true
}
