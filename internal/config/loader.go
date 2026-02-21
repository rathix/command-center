package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a YAML configuration file at path.
// If path does not exist or is empty, it returns an empty Config with no errors.
// If the YAML is malformed, it returns nil config with a parse error.
// For validation errors, it returns a valid config with invalid entries stripped
// plus errors describing what was removed.
func Load(path string) (*Config, []error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, []error{fmt.Errorf("failed to read config file: %w", err)}
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return &Config{}, nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, []error{fmt.Errorf("failed to parse config YAML: %w", err)}
	}

	var validationErrors []error

	// Validate services: name, url, group are required
	validServices := make([]CustomService, 0, len(cfg.Services))
	seenServiceNames := make(map[string]struct{}, len(cfg.Services))
	for i, svc := range cfg.Services {
		valid := true
		name := strings.TrimSpace(svc.Name)
		if name == "" {
			validationErrors = append(validationErrors, fmt.Errorf("services[%d].name: required field missing", i))
			valid = false
		}
		if _, dup := seenServiceNames[name]; name != "" && dup {
			validationErrors = append(validationErrors, fmt.Errorf("services[%d].name: duplicate service name %q", i, name))
			valid = false
		}
		rawURL := strings.TrimSpace(svc.URL)
		if rawURL == "" {
			validationErrors = append(validationErrors, fmt.Errorf("services[%d].url: required field missing", i))
			valid = false
		} else {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				validationErrors = append(validationErrors, fmt.Errorf("services[%d].url: invalid URL %q", i, rawURL))
				valid = false
			}
		}
		if strings.TrimSpace(svc.Group) == "" {
			validationErrors = append(validationErrors, fmt.Errorf("services[%d].group: required field missing", i))
			valid = false
		}
		if valid {
			// Validate optional healthUrl if provided
			if rawHealth := strings.TrimSpace(svc.HealthURL); rawHealth != "" {
				parsed, err := url.Parse(rawHealth)
				if err != nil || parsed.Scheme == "" || parsed.Host == "" {
					validationErrors = append(validationErrors, fmt.Errorf("services[%d].healthUrl: invalid URL %q", i, rawHealth))
					svc.HealthURL = ""
				}
			}
			seenServiceNames[name] = struct{}{}
			validServices = append(validServices, svc)
		}
	}
	cfg.Services = validServices
	// Validate overrides: match is required and must be namespace/name format
	validOverrides := make([]ServiceOverride, 0, len(cfg.Overrides))
	for i, ovr := range cfg.Overrides {
		match := strings.TrimSpace(ovr.Match)
		if match == "" {
			validationErrors = append(validationErrors, fmt.Errorf("overrides[%d].match: required field missing", i))
			continue
		}
		parts := strings.SplitN(match, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			validationErrors = append(validationErrors, fmt.Errorf("overrides[%d].match: must be in namespace/name format, got %q", i, ovr.Match))
			continue
		}
		// Validate optional healthUrl if provided
		if rawHealth := strings.TrimSpace(ovr.HealthURL); rawHealth != "" {
			parsed, err := url.Parse(rawHealth)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				validationErrors = append(validationErrors, fmt.Errorf("overrides[%d].healthUrl: invalid URL %q", i, rawHealth))
				ovr.HealthURL = ""
			}
		}
		validOverrides = append(validOverrides, ovr)
	}
	cfg.Overrides = validOverrides

	// Validate OIDC issuer URL when provided.
	if issuer := strings.TrimSpace(cfg.OIDC.IssuerURL); issuer != "" {
		parsed, err := url.Parse(issuer)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			validationErrors = append(validationErrors, fmt.Errorf("oidc.issuerUrl: invalid URL %q", issuer))
			cfg.OIDC.IssuerURL = ""
		}
	}

	return &cfg, validationErrors
}
