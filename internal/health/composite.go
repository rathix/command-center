package health

import "github.com/rathix/command-center/internal/state"

// EndpointReadiness represents the K8s readiness signal for a service.
// A nil pointer means no EndpointSlice data is available (fallback to HTTP-only).
type EndpointReadiness struct {
	Ready int // Number of ready endpoints
	Total int // Total number of endpoints
}

// CompositeResult holds the fused health status and auth-guarded flag.
type CompositeResult struct {
	Status      state.HealthStatus
	AuthGuarded bool
}

// isAuthCode returns true for HTTP status codes indicating auth gating (401, 403).
func isAuthCode(code int) bool {
	return code == 401 || code == 403
}

// hasReadyEndpoints returns true if at least one endpoint is ready.
func hasReadyEndpoints(er *EndpointReadiness) bool {
	return er != nil && er.Ready > 0
}

// CompositeHealth fuses an HTTP probe result with K8s EndpointSlice readiness
// to produce a composite health status and auth-guarded flag.
//
// Truth table:
//
//	HTTP Probe       | EndpointSlice | Composite Status  | authGuarded
//	2xx              | Ready         | healthy           | false
//	2xx              | Not ready     | degraded          | false
//	401/403          | Ready         | healthy           | true
//	401/403          | Not ready     | unhealthy         | true
//	5xx/timeout      | Ready         | degraded          | false
//	5xx/timeout      | Not ready     | unhealthy         | false
//	any              | No data       | HTTP-only fallback| false
func CompositeHealth(httpStatus state.HealthStatus, httpCode *int, endpointReadiness *EndpointReadiness) CompositeResult {
	// No K8s data -> HTTP-only fallback (AC #6, #7)
	if endpointReadiness == nil {
		return CompositeResult{Status: httpStatus, AuthGuarded: false}
	}

	ready := hasReadyEndpoints(endpointReadiness)

	// Determine if this is an auth code
	if httpCode != nil && isAuthCode(*httpCode) {
		if ready {
			return CompositeResult{Status: state.StatusHealthy, AuthGuarded: true}
		}
		return CompositeResult{Status: state.StatusUnhealthy, AuthGuarded: true}
	}

	// HTTP healthy (2xx)
	if httpStatus == state.StatusHealthy {
		if ready {
			return CompositeResult{Status: state.StatusHealthy, AuthGuarded: false}
		}
		return CompositeResult{Status: state.StatusDegraded, AuthGuarded: false}
	}

	// HTTP unhealthy (5xx, timeout, connection error, other non-2xx non-auth)
	if ready {
		return CompositeResult{Status: state.StatusDegraded, AuthGuarded: false}
	}
	return CompositeResult{Status: state.StatusUnhealthy, AuthGuarded: false}
}
