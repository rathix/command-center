package health

import (
	"testing"

	"github.com/rathix/command-center/internal/state"
)

func intPtr(v int) *int {
	return &v
}

func TestCompositeHealth_TruthTable(t *testing.T) {
	tests := []struct {
		name           string
		httpStatus     state.HealthStatus
		httpCode       *int
		er             *EndpointReadiness
		wantStatus     state.HealthStatus
		wantAuthGuard  bool
	}{
		// Row 1: 2xx + Ready → healthy, false
		{
			name:          "2xx_ready",
			httpStatus:    state.StatusHealthy,
			httpCode:      intPtr(200),
			er:            &EndpointReadiness{Ready: 2, Total: 2},
			wantStatus:    state.StatusHealthy,
			wantAuthGuard: false,
		},
		// Row 2: 2xx + Not ready → degraded, false
		{
			name:          "2xx_not_ready",
			httpStatus:    state.StatusHealthy,
			httpCode:      intPtr(200),
			er:            &EndpointReadiness{Ready: 0, Total: 2},
			wantStatus:    state.StatusDegraded,
			wantAuthGuard: false,
		},
		// Row 3: 401 + Ready → healthy, true
		{
			name:          "401_ready",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      intPtr(401),
			er:            &EndpointReadiness{Ready: 1, Total: 1},
			wantStatus:    state.StatusHealthy,
			wantAuthGuard: true,
		},
		// Row 4: 401 + Not ready → unhealthy, true
		{
			name:          "401_not_ready",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      intPtr(401),
			er:            &EndpointReadiness{Ready: 0, Total: 1},
			wantStatus:    state.StatusUnhealthy,
			wantAuthGuard: true,
		},
		// Row 5: 403 + Ready → healthy, true
		{
			name:          "403_ready",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      intPtr(403),
			er:            &EndpointReadiness{Ready: 3, Total: 3},
			wantStatus:    state.StatusHealthy,
			wantAuthGuard: true,
		},
		// Row 6: 403 + Not ready → unhealthy, true
		{
			name:          "403_not_ready",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      intPtr(403),
			er:            &EndpointReadiness{Ready: 0, Total: 3},
			wantStatus:    state.StatusUnhealthy,
			wantAuthGuard: true,
		},
		// Row 7: 500 + Ready → degraded, false
		{
			name:          "500_ready",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      intPtr(500),
			er:            &EndpointReadiness{Ready: 1, Total: 2},
			wantStatus:    state.StatusDegraded,
			wantAuthGuard: false,
		},
		// Row 8: 500 + Not ready → unhealthy, false
		{
			name:          "500_not_ready",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      intPtr(500),
			er:            &EndpointReadiness{Ready: 0, Total: 2},
			wantStatus:    state.StatusUnhealthy,
			wantAuthGuard: false,
		},
		// Row 9: 2xx + No data → healthy (HTTP-only fallback), false
		{
			name:          "2xx_no_data",
			httpStatus:    state.StatusHealthy,
			httpCode:      intPtr(200),
			er:            nil,
			wantStatus:    state.StatusHealthy,
			wantAuthGuard: false,
		},
		// Row 10: 401 + No data → unhealthy (HTTP-only fallback), false
		{
			name:          "401_no_data",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      intPtr(401),
			er:            nil,
			wantStatus:    state.StatusUnhealthy,
			wantAuthGuard: false,
		},
		// Row 11: 500 + No data → unhealthy (HTTP-only fallback), false
		{
			name:          "500_no_data",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      intPtr(500),
			er:            nil,
			wantStatus:    state.StatusUnhealthy,
			wantAuthGuard: false,
		},
		// Row 12: timeout (no HTTP code) + Ready → degraded, false
		{
			name:          "timeout_ready",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      nil,
			er:            &EndpointReadiness{Ready: 1, Total: 1},
			wantStatus:    state.StatusDegraded,
			wantAuthGuard: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompositeHealth(tt.httpStatus, tt.httpCode, tt.er)
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.AuthGuarded != tt.wantAuthGuard {
				t.Errorf("AuthGuarded = %v, want %v", got.AuthGuarded, tt.wantAuthGuard)
			}
		})
	}
}

func TestCompositeHealth_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		httpStatus     state.HealthStatus
		httpCode       *int
		er             *EndpointReadiness
		wantStatus     state.HealthStatus
		wantAuthGuard  bool
	}{
		// Timeout with no endpoints → unhealthy
		{
			name:          "timeout_no_endpoints",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      nil,
			er:            &EndpointReadiness{Ready: 0, Total: 0},
			wantStatus:    state.StatusUnhealthy,
			wantAuthGuard: false,
		},
		// Timeout with no data → unhealthy (HTTP-only)
		{
			name:          "timeout_no_data",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      nil,
			er:            nil,
			wantStatus:    state.StatusUnhealthy,
			wantAuthGuard: false,
		},
		// Partial readiness (2 of 5) with 2xx → healthy
		{
			name:          "partial_readiness_2xx",
			httpStatus:    state.StatusHealthy,
			httpCode:      intPtr(200),
			er:            &EndpointReadiness{Ready: 2, Total: 5},
			wantStatus:    state.StatusHealthy,
			wantAuthGuard: false,
		},
		// Zero total endpoints with 2xx → degraded (Ready=0)
		{
			name:          "zero_total_2xx",
			httpStatus:    state.StatusHealthy,
			httpCode:      intPtr(200),
			er:            &EndpointReadiness{Ready: 0, Total: 0},
			wantStatus:    state.StatusDegraded,
			wantAuthGuard: false,
		},
		// Unknown HTTP status with ready endpoints → degraded
		{
			name:          "unknown_status_ready",
			httpStatus:    state.StatusUnknown,
			httpCode:      nil,
			er:            &EndpointReadiness{Ready: 1, Total: 1},
			wantStatus:    state.StatusDegraded,
			wantAuthGuard: false,
		},
		// Unknown HTTP status with no ready endpoints → unhealthy
		{
			name:          "unknown_status_not_ready",
			httpStatus:    state.StatusUnknown,
			httpCode:      nil,
			er:            &EndpointReadiness{Ready: 0, Total: 1},
			wantStatus:    state.StatusUnhealthy,
			wantAuthGuard: false,
		},
		// 404 (not auth code) with ready → degraded
		{
			name:          "404_ready",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      intPtr(404),
			er:            &EndpointReadiness{Ready: 1, Total: 1},
			wantStatus:    state.StatusDegraded,
			wantAuthGuard: false,
		},
		// 301 redirect with ready → degraded
		{
			name:          "301_ready",
			httpStatus:    state.StatusUnhealthy,
			httpCode:      intPtr(301),
			er:            &EndpointReadiness{Ready: 1, Total: 1},
			wantStatus:    state.StatusDegraded,
			wantAuthGuard: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompositeHealth(tt.httpStatus, tt.httpCode, tt.er)
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.AuthGuarded != tt.wantAuthGuard {
				t.Errorf("AuthGuarded = %v, want %v", got.AuthGuarded, tt.wantAuthGuard)
			}
		})
	}
}
