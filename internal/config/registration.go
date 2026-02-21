package config

import (
	"strings"

	"github.com/rathix/command-center/internal/state"
)

// StateUpdater is the interface for modifying service state.
// Defined at the consumer per Go convention.
type StateUpdater interface {
	AddOrUpdate(svc state.Service)
	Remove(namespace, name string)
	Update(namespace, name string, fn func(*state.Service))
	Get(namespace, name string) (state.Service, bool)
}

// RegisterServices adds all custom services from config to the state store.
func RegisterServices(store StateUpdater, cfg *Config) {
	if cfg == nil {
		return
	}
	for _, cs := range cfg.Services {
		store.AddOrUpdate(customServiceToState(cs))
	}
}

// ApplyOverrides modifies existing K8s services in the store based on config overrides.
func ApplyOverrides(store StateUpdater, cfg *Config) {
	if cfg == nil {
		return
	}
	for _, ovr := range cfg.Overrides {
		ns, name, ok := parseMatch(ovr.Match)
		if !ok {
			continue
		}
		if _, exists := store.Get(ns, name); !exists {
			continue
		}
		store.Update(ns, name, func(svc *state.Service) {
			applyOverride(svc, ovr)
		})
	}
}

// ReconcileOnReload diffs old vs new config and applies additions, removals, and updates.
func ReconcileOnReload(store StateUpdater, oldCfg, newCfg *Config) (added, removed, updated int) {
	// Parse failures should not blow away the last-known-good config state.
	if newCfg == nil {
		return 0, 0, 0
	}

	oldServices := make(map[string]CustomService)
	if oldCfg != nil {
		for _, cs := range oldCfg.Services {
			oldServices[cs.Name] = cs
		}
	}

	newServices := make(map[string]CustomService)
	for _, cs := range newCfg.Services {
		newServices[cs.Name] = cs
	}

	// Add new services
	for name, cs := range newServices {
		if _, exists := oldServices[name]; !exists {
			store.AddOrUpdate(customServiceToState(cs))
			added++
		}
	}

	// Remove deleted services
	for name := range oldServices {
		if _, exists := newServices[name]; !exists {
			store.Remove("custom", name)
			removed++
		}
	}

	// Update changed services
	for name, newCS := range newServices {
		oldCS, exists := oldServices[name]
		if !exists {
			continue
		}
		if !customServiceEqual(oldCS, newCS) {
			store.Update("custom", name, func(svc *state.Service) {
				svc.DisplayName = newCS.DisplayName
				if svc.DisplayName == "" {
					svc.DisplayName = newCS.Name
				}
				svc.Group = newCS.Group
				svc.URL = newCS.URL
				svc.HealthEndpoint = newCS.HealthEndpoint
				svc.ExpectedStatusCodes = newCS.ExpectedStatusCodes
				svc.Icon = newCS.Icon
			})
			updated++
		}
	}

	// Re-apply all new overrides
	ApplyOverrides(store, newCfg)

	return added, removed, updated
}

func customServiceToState(cs CustomService) state.Service {
	displayName := cs.DisplayName
	if displayName == "" {
		displayName = cs.Name
	}
	return state.Service{
		Name:                cs.Name,
		DisplayName:         displayName,
		Namespace:           "custom",
		Group:               cs.Group,
		URL:                 cs.URL,
		Source:              state.SourceConfig,
		Status:              state.StatusUnknown,
		HealthEndpoint:      cs.HealthEndpoint,
		ExpectedStatusCodes: cs.ExpectedStatusCodes,
		Icon:                cs.Icon,
	}
}

func applyOverride(svc *state.Service, ovr ServiceOverride) {
	if ovr.DisplayName != "" {
		svc.DisplayName = ovr.DisplayName
	}
	if ovr.HealthEndpoint != "" {
		svc.HealthEndpoint = ovr.HealthEndpoint
	}
	if len(ovr.ExpectedStatusCodes) > 0 {
		svc.ExpectedStatusCodes = ovr.ExpectedStatusCodes
	}
	if ovr.Icon != "" {
		svc.Icon = ovr.Icon
	}
}

func parseMatch(match string) (namespace, name string, ok bool) {
	parts := strings.SplitN(match, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func customServiceEqual(a, b CustomService) bool {
	return a.URL == b.URL &&
		a.Group == b.Group &&
		a.DisplayName == b.DisplayName &&
		a.HealthEndpoint == b.HealthEndpoint &&
		a.Icon == b.Icon &&
		intSliceEqual(a.ExpectedStatusCodes, b.ExpectedStatusCodes)
}

func intSliceEqual(a, b []int) bool {
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
