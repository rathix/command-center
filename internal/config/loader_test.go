package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Task 3.1: Test valid config with all fields populated
func TestLoad_ValidFullConfig(t *testing.T) {
	yaml := `
services:
  - name: "truenas"
    url: "https://nas.local"
    group: "infrastructure"
    displayName: "TrueNAS"
    healthUrl: "https://nas.local/api/v2.0/system/state"
    expectedStatusCodes: [200, 204]
    icon: "truenas"

overrides:
  - match: "default/radarr"
    displayName: "Radarr HD"
    healthUrl: "https://radarr.local/ping"
    expectedStatusCodes: [200]
    icon: "radarr"

groups:
  infrastructure:
    displayName: "Infrastructure"
    icon: "server"
    sortOrder: 1

health:
  interval: "30s"
  timeout: "10s"

history:
  retentionDays: 30
`
	path := writeTempConfig(t, yaml)
	cfg, errs := Load(path)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	// Services
	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}
	svc := cfg.Services[0]
	if svc.Name != "truenas" {
		t.Errorf("service name = %q, want %q", svc.Name, "truenas")
	}
	if svc.URL != "https://nas.local" {
		t.Errorf("service url = %q, want %q", svc.URL, "https://nas.local")
	}
	if svc.Group != "infrastructure" {
		t.Errorf("service group = %q, want %q", svc.Group, "infrastructure")
	}
	if svc.DisplayName != "TrueNAS" {
		t.Errorf("service displayName = %q, want %q", svc.DisplayName, "TrueNAS")
	}
	if svc.HealthURL != "https://nas.local/api/v2.0/system/state" {
		t.Errorf("service healthUrl = %q, want %q", svc.HealthURL, "https://nas.local/api/v2.0/system/state")
	}
	if len(svc.ExpectedStatusCodes) != 2 || svc.ExpectedStatusCodes[0] != 200 || svc.ExpectedStatusCodes[1] != 204 {
		t.Errorf("service expectedStatusCodes = %v, want [200 204]", svc.ExpectedStatusCodes)
	}
	if svc.Icon != "truenas" {
		t.Errorf("service icon = %q, want %q", svc.Icon, "truenas")
	}

	// Overrides
	if len(cfg.Overrides) != 1 {
		t.Fatalf("expected 1 override, got %d", len(cfg.Overrides))
	}
	ovr := cfg.Overrides[0]
	if ovr.Match != "default/radarr" {
		t.Errorf("override match = %q, want %q", ovr.Match, "default/radarr")
	}
	if ovr.DisplayName != "Radarr HD" {
		t.Errorf("override displayName = %q, want %q", ovr.DisplayName, "Radarr HD")
	}
	if ovr.HealthURL != "https://radarr.local/ping" {
		t.Errorf("override healthUrl = %q, want %q", ovr.HealthURL, "https://radarr.local/ping")
	}

	// Groups
	if len(cfg.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(cfg.Groups))
	}
	grp, ok := cfg.Groups["infrastructure"]
	if !ok {
		t.Fatal("expected 'infrastructure' group")
	}
	if grp.DisplayName != "Infrastructure" {
		t.Errorf("group displayName = %q, want %q", grp.DisplayName, "Infrastructure")
	}
	if grp.Icon != "server" {
		t.Errorf("group icon = %q, want %q", grp.Icon, "server")
	}
	if grp.SortOrder != 1 {
		t.Errorf("group sortOrder = %d, want %d", grp.SortOrder, 1)
	}

	// Health
	if cfg.Health.Interval != "30s" {
		t.Errorf("health interval = %q, want %q", cfg.Health.Interval, "30s")
	}
	if cfg.Health.Timeout != "10s" {
		t.Errorf("health timeout = %q, want %q", cfg.Health.Timeout, "10s")
	}

	// History
	if cfg.History.RetentionDays != 30 {
		t.Errorf("history retentionDays = %d, want %d", cfg.History.RetentionDays, 30)
	}

}

// Task 3.2: Test valid config with only required fields
func TestLoad_ValidRequiredFieldsOnly(t *testing.T) {
	yaml := `
services:
  - name: "truenas"
    url: "https://nas.local"
    group: "infrastructure"

overrides:
  - match: "default/radarr"
`
	path := writeTempConfig(t, yaml)
	cfg, errs := Load(path)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}
	if cfg.Services[0].DisplayName != "" {
		t.Errorf("expected empty displayName, got %q", cfg.Services[0].DisplayName)
	}
	if cfg.Services[0].HealthURL != "" {
		t.Errorf("expected empty healthUrl, got %q", cfg.Services[0].HealthURL)
	}
	if len(cfg.Services[0].ExpectedStatusCodes) != 0 {
		t.Errorf("expected empty expectedStatusCodes, got %v", cfg.Services[0].ExpectedStatusCodes)
	}
	if len(cfg.Overrides) != 1 {
		t.Fatalf("expected 1 override, got %d", len(cfg.Overrides))
	}
	if cfg.Overrides[0].Match != "default/radarr" {
		t.Errorf("override match = %q, want %q", cfg.Overrides[0].Match, "default/radarr")
	}
}

// Task 3.3: Test partial failure — mix of valid and invalid service entries
func TestLoad_PartialFailure(t *testing.T) {
	yaml := `
services:
  - name: "truenas"
    url: "https://nas.local"
    group: "infrastructure"
  - name: ""
    url: "https://bad.local"
    group: "infrastructure"
  - name: "plex"
    url: ""
    group: "media"
  - name: "valid-two"
    url: "https://valid.local"
    group: "apps"

overrides:
  - match: "default/radarr"
  - match: ""
  - match: "media/plex"
`
	path := writeTempConfig(t, yaml)
	cfg, errs := Load(path)

	if cfg == nil {
		t.Fatal("expected non-nil config on partial failure")
	}

	// Valid services preserved
	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 valid services, got %d", len(cfg.Services))
	}
	if cfg.Services[0].Name != "truenas" {
		t.Errorf("first valid service = %q, want %q", cfg.Services[0].Name, "truenas")
	}
	if cfg.Services[1].Name != "valid-two" {
		t.Errorf("second valid service = %q, want %q", cfg.Services[1].Name, "valid-two")
	}

	// Valid overrides preserved
	if len(cfg.Overrides) != 2 {
		t.Fatalf("expected 2 valid overrides, got %d", len(cfg.Overrides))
	}

	// Errors reported for invalid entries
	if len(errs) != 3 {
		t.Fatalf("expected 3 validation errors, got %d: %v", len(errs), errs)
	}

	// Check error messages contain field-level detail
	errStrs := make([]string, len(errs))
	for i, e := range errs {
		errStrs[i] = e.Error()
	}
	joined := strings.Join(errStrs, "\n")
	if !strings.Contains(joined, "services[1].name") {
		t.Errorf("expected error for services[1].name, got:\n%s", joined)
	}
	if !strings.Contains(joined, "services[2].url") {
		t.Errorf("expected error for services[2].url, got:\n%s", joined)
	}
	if !strings.Contains(joined, "overrides[1].match") {
		t.Errorf("expected error for overrides[1].match, got:\n%s", joined)
	}
}

// Task 3.4: Test missing config file returns empty config, no errors
func TestLoad_MissingFile(t *testing.T) {
	cfg, errs := Load("/nonexistent/path/config.yaml")
	if len(errs) != 0 {
		t.Fatalf("expected no errors for missing file, got %v", errs)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for missing file")
	}
	if len(cfg.Services) != 0 {
		t.Errorf("expected 0 services, got %d", len(cfg.Services))
	}
	if len(cfg.Overrides) != 0 {
		t.Errorf("expected 0 overrides, got %d", len(cfg.Overrides))
	}
}

// Task 3.5: Test empty config file returns empty config, no errors
func TestLoad_EmptyFile(t *testing.T) {
	path := writeTempConfig(t, "")
	cfg, errs := Load(path)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for empty file, got %v", errs)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for empty file")
	}
	if len(cfg.Services) != 0 {
		t.Errorf("expected 0 services, got %d", len(cfg.Services))
	}
}

// Task 3.6: Test malformed YAML returns parse error
func TestLoad_MalformedYAML(t *testing.T) {
	path := writeTempConfig(t, "{{{{invalid yaml!!!!")
	cfg, errs := Load(path)
	if cfg != nil {
		t.Error("expected nil config for malformed YAML")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 parse error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "parse") {
		t.Errorf("expected parse error, got: %v", errs[0])
	}
}

// Task 3.7: Test override matching format validation (namespace/name pattern)
func TestLoad_OverrideMatchFormat(t *testing.T) {
	tests := []struct {
		name      string
		match     string
		wantValid bool
	}{
		{"valid namespace/name", "default/radarr", true},
		{"valid with dashes", "my-ns/my-svc", true},
		{"missing slash", "radarr", false},
		{"empty string", "", false},
		{"only slash", "/", false},
		{"missing name", "default/", false},
		{"missing namespace", "/radarr", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := `
overrides:
  - match: "` + tt.match + `"
`
			path := writeTempConfig(t, yaml)
			cfg, errs := Load(path)
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
			if tt.wantValid {
				if len(errs) != 0 {
					t.Errorf("expected no errors for %q, got %v", tt.match, errs)
				}
				if len(cfg.Overrides) != 1 {
					t.Errorf("expected 1 override, got %d", len(cfg.Overrides))
				}
			} else {
				if len(cfg.Overrides) != 0 {
					t.Errorf("expected 0 overrides for invalid %q, got %d", tt.match, len(cfg.Overrides))
				}
			}
		})
	}
}

// Task 3.8: Test all optional sections can be omitted independently
func TestLoad_OptionalSectionsOmitted(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"only services", `
services:
  - name: "test"
    url: "http://test"
    group: "g"
`},
		{"only overrides", `
overrides:
  - match: "ns/svc"
`},
		{"only groups", `
groups:
  infra:
    displayName: "Infra"
`},
		{"only health", `
health:
  interval: "30s"
`},
		{"only history", `
history:
  retentionDays: 7
`},
		{"completely empty sections", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempConfig(t, tt.yaml)
			cfg, errs := Load(path)
			if len(errs) != 0 {
				t.Fatalf("expected no errors, got %v", errs)
			}
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
		})
	}
}

func TestLoad_RequiredFieldsTreatWhitespaceAsMissing(t *testing.T) {
	yaml := `
services:
  - name: "   "
    url: "https://ok.local"
    group: "infra"
  - name: "svc"
    url: "   "
    group: "infra"
  - name: "svc2"
    url: "https://ok2.local"
    group: "   "
overrides:
  - match: "   "
`
	path := writeTempConfig(t, yaml)
	cfg, errs := Load(path)

	if cfg == nil {
		t.Fatal("expected non-nil config on validation failure")
	}
	if len(cfg.Services) != 0 {
		t.Fatalf("expected no valid services, got %d", len(cfg.Services))
	}
	if len(cfg.Overrides) != 0 {
		t.Fatalf("expected no valid overrides, got %d", len(cfg.Overrides))
	}
	if len(errs) != 4 {
		t.Fatalf("expected 4 validation errors, got %d: %v", len(errs), errs)
	}
}

func TestLoad_DuplicateServiceNamesReportedAndDeduplicated(t *testing.T) {
	yaml := `
services:
  - name: "truenas"
    url: "https://nas-a.local"
    group: "infra"
  - name: "truenas"
    url: "https://nas-b.local"
    group: "infra"
  - name: "router"
    url: "https://router.local"
    group: "infra"
`
	path := writeTempConfig(t, yaml)
	cfg, errs := Load(path)

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 valid services after duplicate filtering, got %d", len(cfg.Services))
	}
	if cfg.Services[0].Name != "truenas" || cfg.Services[1].Name != "router" {
		t.Fatalf("unexpected service order/names: %+v", cfg.Services)
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 duplicate-name validation error, got %d: %v", len(errs), errs)
	}
	        if !strings.Contains(errs[0].Error(), "duplicate service name") {
	                t.Fatalf("expected duplicate-name error, got: %v", errs[0])
	        }
	}
	
func TestLoad_HealthURLValidation(t *testing.T) {
	tests := []struct {
		name     string
		healthUrl string
		valid    bool
	}{
		{"valid absolute URL", "https://monitor.local/health", true},
		{"valid http", "http://monitor.local/status", true},
		{"empty is fine", "", true},
		{"invalid no scheme", "monitor.local/health", false},
		{"invalid no host", "https://", false},
		{"invalid path only", "/api/health", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthLine := ""
			if tt.healthUrl != "" {
				healthLine = `    healthUrl: "` + tt.healthUrl + `"`
			}
			yaml := `
services:
  - name: "test"
    url: "https://site.local"
    group: "infra"
` + healthLine + `
`
			path := writeTempConfig(t, yaml)
			cfg, errs := Load(path)
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
			// Service should always be kept (healthUrl is optional)
			if len(cfg.Services) != 1 {
				t.Fatalf("expected 1 service, got %d", len(cfg.Services))
			}
			if tt.valid {
				if len(errs) != 0 {
					t.Errorf("expected no errors for %q, got %v", tt.healthUrl, errs)
				}
				if cfg.Services[0].HealthURL != tt.healthUrl {
					t.Errorf("expected healthUrl %q, got %q", tt.healthUrl, cfg.Services[0].HealthURL)
				}
			} else {
				if len(errs) == 0 {
					t.Errorf("expected validation warning for invalid healthUrl %q", tt.healthUrl)
				}
				// Invalid healthUrl should be cleared
				if cfg.Services[0].HealthURL != "" {
					t.Errorf("expected healthUrl cleared for invalid %q, got %q", tt.healthUrl, cfg.Services[0].HealthURL)
				}
			}
		})
	}
}

func TestLoad_TerminalConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErrs  int
		errSubstr string
	}{
		{
			name: "valid enabled terminal config",
			yaml: `
terminal:
  enabled: true
  allowedCommands: [kubectl, helm]
  idleTimeout: "15m"
  maxSessions: 4
`,
			wantErrs: 0,
		},
		{
			name: "disabled terminal needs no commands",
			yaml: `
terminal:
  enabled: false
`,
			wantErrs: 0,
		},
		{
			name: "enabled but no commands",
			yaml: `
terminal:
  enabled: true
`,
			wantErrs:  1,
			errSubstr: "allowedCommands",
		},
		{
			name: "invalid idle timeout",
			yaml: `
terminal:
  enabled: true
  allowedCommands: [kubectl]
  idleTimeout: "bad"
`,
			wantErrs:  1,
			errSubstr: "idleTimeout",
		},
		{
			name: "negative max sessions",
			yaml: `
terminal:
  enabled: true
  allowedCommands: [kubectl]
  maxSessions: -1
`,
			wantErrs:  1,
			errSubstr: "maxSessions",
		},
		{
			name:     "omitted terminal section is fine",
			yaml:     `{}`,
			wantErrs: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempConfig(t, tt.yaml)
			cfg, errs := Load(path)
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
			if len(errs) != tt.wantErrs {
				t.Fatalf("expected %d errors, got %d: %v", tt.wantErrs, len(errs), errs)
			}
			if tt.errSubstr != "" && len(errs) > 0 {
				if !strings.Contains(errs[0].Error(), tt.errSubstr) {
					t.Errorf("expected error containing %q, got: %v", tt.errSubstr, errs[0])
				}
			}
		})
	}
}

func TestLoad_URLValidation(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		valid bool
	}{
		{"valid absolute URL", "https://nas.local", true},
		{"valid http", "http://router.local", true},
		{"invalid scheme", "nas.local", false},
		{"empty host", "https://", false},
		{"malformed", "://bad", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := `
services:
  - name: "test"
    url: "` + tt.url + `"
    group: "infra"
`
			path := writeTempConfig(t, yaml)
			cfg, errs := Load(path)
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
			if tt.valid {
				if len(errs) != 0 {
					t.Errorf("expected no errors for %q, got %v", tt.url, errs)
				}
				if len(cfg.Services) != 1 {
					t.Errorf("expected 1 service, got %d", len(cfg.Services))
				}
			} else {
				if len(cfg.Services) != 0 {
					t.Errorf("expected 0 services for invalid URL %q, got %d", tt.url, len(cfg.Services))
				}
				if len(errs) == 0 {
					t.Errorf("expected validation error for invalid URL %q", tt.url)
				}
			}
		})
	}
}
	
