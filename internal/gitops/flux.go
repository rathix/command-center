package gitops

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rathix/command-center/internal/state"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

var (
	kustomizationGVR = schema.GroupVersionResource{
		Group:    "kustomize.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "kustomizations",
	}

	helmReleaseGVR = schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}
)

// FluxStateUpdater is the consumer-defined interface for updating GitOps state
// on a service in the state store. Satisfied by *state.Store.
type FluxStateUpdater interface {
	Update(namespace, name string, fn func(*state.Service))
	All() []state.Service
}

// fluxCondition represents a Flux resource condition extracted from unstructured data.
type fluxCondition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

// ReconcileResult holds the mapped reconciliation state and message.
type ReconcileResult struct {
	State   state.ReconciliationState
	Message string
}

// fluxStatusResult holds the extracted status from a Flux resource.
type fluxStatusResult struct {
	Name           string
	Namespace      string
	Result         ReconcileResult
	TransitionTime *time.Time
}

// FluxWatcher watches Flux CRDs (Kustomization, HelmRelease) via dynamic informers
// and updates service GitOps status in the state store.
type FluxWatcher struct {
	dynamicClient dynamic.Interface
	updater       FluxStateUpdater
	logger        *slog.Logger
	fluxNamespace string

	cancel  context.CancelFunc
	factory dynamicinformer.DynamicSharedInformerFactory

	mu      sync.Mutex
	running bool
}

// NewFluxWatcher creates a new FluxWatcher.
func NewFluxWatcher(dynamicClient dynamic.Interface, updater FluxStateUpdater, fluxNamespace string, logger *slog.Logger) *FluxWatcher {
	return &FluxWatcher{
		dynamicClient: dynamicClient,
		updater:       updater,
		logger:        logger,
		fluxNamespace: fluxNamespace,
	}
}

// Run starts the dynamic informers and blocks until context is cancelled.
func (w *FluxWatcher) Run(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true

	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.mu.Unlock()

	w.logger.Info("starting Flux watcher", "namespace", w.fluxNamespace)

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		w.dynamicClient, 0, w.fluxNamespace, nil,
	)

	w.mu.Lock()
	w.factory = factory
	w.mu.Unlock()

	// Set up Kustomization informer
	kustInformer := factory.ForResource(kustomizationGVR).Informer()
	kustInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			w.onEvent(obj, "kustomization")
		},
		UpdateFunc: func(_, newObj interface{}) {
			w.onEvent(newObj, "kustomization")
		},
	})

	// Set up HelmRelease informer
	helmInformer := factory.ForResource(helmReleaseGVR).Informer()
	helmInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			w.onEvent(obj, "helmrelease")
		},
		UpdateFunc: func(_, newObj interface{}) {
			w.onEvent(newObj, "helmrelease")
		},
	})

	factory.Start(ctx.Done())

	// Wait for cache sync (non-blocking on CRD absence)
	synced := factory.WaitForCacheSync(ctx.Done())
	for gvr, ok := range synced {
		if !ok {
			w.logger.Warn("Flux CRD informer failed to sync (CRD may not be installed)",
				"resource", gvr.Resource,
				"group", gvr.Group,
			)
		}
	}

	w.logger.Info("Flux watcher started")
	<-ctx.Done()

	w.mu.Lock()
	w.running = false
	w.mu.Unlock()

	w.logger.Info("Flux watcher stopped")
}

// Stop cancels the watcher context and stops informers.
func (w *FluxWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
}

func (w *FluxWatcher) onEvent(obj interface{}, sourceType string) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		u, ok = tombstone.Obj.(*unstructured.Unstructured)
		if !ok {
			return
		}
	}
	w.handleObject(u, sourceType)
}

// handleObject extracts Flux status from an unstructured object and updates the state store.
func (w *FluxWatcher) handleObject(obj *unstructured.Unstructured, sourceType string) {
	result := extractFluxStatus(obj)

	w.logger.Debug("Flux reconciliation state changed",
		"resource", result.Name,
		"namespace", result.Namespace,
		"state", result.Result.State,
		"sourceType", sourceType,
		"message", result.Result.Message,
	)

	// Match Flux resource name to state store services by name convention.
	// The Flux resource name (e.g. "my-app") is matched against service names
	// in the state store across all namespaces.
	serviceName := result.Name

	w.updater.Update(result.Namespace, serviceName, func(svc *state.Service) {
		svc.GitOpsStatus = &state.GitOpsStatus{
			ReconciliationState: result.Result.State,
			LastTransitionTime:  result.TransitionTime,
			Message:             result.Result.Message,
			SourceType:          sourceType,
		}
	})

	// Also try to match services in other namespaces by iterating all services.
	for _, svc := range w.updater.All() {
		if svc.Name == serviceName && svc.Namespace != result.Namespace {
			w.updater.Update(svc.Namespace, svc.Name, func(s *state.Service) {
				s.GitOpsStatus = &state.GitOpsStatus{
					ReconciliationState: result.Result.State,
					LastTransitionTime:  result.TransitionTime,
					Message:             result.Result.Message,
					SourceType:          sourceType,
				}
			})
		}
	}
}

// extractFluxStatus extracts the reconciliation status from an unstructured Flux resource.
func extractFluxStatus(obj *unstructured.Unstructured) fluxStatusResult {
	name := obj.GetName()
	namespace := obj.GetNamespace()

	suspended, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend")
	conditions, transitionTime := extractConditionsAndTime(obj)

	result := mapFluxConditionToState(suspended, conditions)

	return fluxStatusResult{
		Name:           name,
		Namespace:      namespace,
		Result:         result,
		TransitionTime: transitionTime,
	}
}

// extractConditionsAndTime extracts conditions and the lastTransitionTime of the Ready condition.
func extractConditionsAndTime(obj *unstructured.Unstructured) ([]fluxCondition, *time.Time) {
	rawConditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return nil, nil
	}

	var conditions []fluxCondition
	var transitionTime *time.Time

	for _, raw := range rawConditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		fc := fluxCondition{}
		if v, ok := cond["type"].(string); ok {
			fc.Type = v
		}
		if v, ok := cond["status"].(string); ok {
			fc.Status = v
		}
		if v, ok := cond["reason"].(string); ok {
			fc.Reason = v
		}
		if v, ok := cond["message"].(string); ok {
			fc.Message = v
		}

		// Extract lastTransitionTime from the Ready condition
		if fc.Type == "Ready" {
			if v, ok := cond["lastTransitionTime"].(string); ok && v != "" {
				if t, err := time.Parse(time.RFC3339, v); err == nil {
					transitionTime = &t
				}
			}
		}

		conditions = append(conditions, fc)
	}

	return conditions, transitionTime
}

// mapFluxConditionToState maps Flux conditions to a ReconcileResult.
// Mapping logic:
//   - spec.suspend=true -> suspended (overrides condition)
//   - Ready=True -> synced
//   - Ready=Unknown -> progressing
//   - Ready=False -> failed
//   - no conditions -> progressing (awaiting first reconciliation)
func mapFluxConditionToState(suspended bool, conditions []fluxCondition) ReconcileResult {
	if suspended {
		return ReconcileResult{State: state.ReconcilSuspended, Message: "reconciliation suspended"}
	}

	for _, c := range conditions {
		if !strings.EqualFold(c.Type, "Ready") {
			continue
		}
		switch c.Status {
		case "True":
			return ReconcileResult{State: state.ReconcilSynced, Message: c.Message}
		case "False":
			return ReconcileResult{State: state.ReconcilFailed, Message: c.Message}
		default: // "Unknown" or any other
			return ReconcileResult{State: state.ReconcilProgressing, Message: c.Message}
		}
	}

	return ReconcileResult{State: state.ReconcilProgressing, Message: "awaiting first reconciliation"}
}
