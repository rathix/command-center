package config

// Config is the top-level configuration parsed from the YAML config file.
type Config struct {
	Services  []CustomService          `yaml:"services"  json:"services"`
	Overrides []ServiceOverride        `yaml:"overrides" json:"overrides"`
	Groups    map[string]GroupConfig   `yaml:"groups"    json:"groups"`
	Health    HealthConfig             `yaml:"health"    json:"health"`
	History   HistoryConfig            `yaml:"history"   json:"history"`
	GitOps    *GitOpsConfig            `yaml:"gitops"    json:"gitops"`
}

// GitOpsConfig configures integration with a GitOps provider (e.g. Flux).
type GitOpsConfig struct {
	Provider      string `yaml:"provider"      json:"provider"`
	Repository    string `yaml:"repository"    json:"repository"`
	Branch        string `yaml:"branch"        json:"branch"`
	FluxNamespace string `yaml:"fluxNamespace" json:"fluxNamespace"`
}

// CustomService defines a non-Kubernetes service to monitor.
type CustomService struct {
	Name                string `yaml:"name"                json:"name"`
	URL                 string `yaml:"url"                 json:"url"`
	Group               string `yaml:"group"               json:"group"`
	DisplayName         string `yaml:"displayName"         json:"displayName"`
	HealthURL           string `yaml:"healthUrl"           json:"healthUrl"`
	ExpectedStatusCodes []int  `yaml:"expectedStatusCodes" json:"expectedStatusCodes"`
	Icon                string `yaml:"icon"                json:"icon"`
}

// ServiceOverride overrides properties of a Kubernetes-discovered service.
type ServiceOverride struct {
	Match               string `yaml:"match"               json:"match"`
	DisplayName         string `yaml:"displayName"         json:"displayName"`
	HealthURL           string `yaml:"healthUrl"           json:"healthUrl"`
	ExpectedStatusCodes []int  `yaml:"expectedStatusCodes" json:"expectedStatusCodes"`
	Icon                string `yaml:"icon"                json:"icon"`
}

// GroupConfig provides metadata for a service group.
type GroupConfig struct {
	DisplayName string `yaml:"displayName" json:"displayName"`
	Icon        string `yaml:"icon"        json:"icon"`
	SortOrder   int    `yaml:"sortOrder"   json:"sortOrder"`
}

// HealthConfig controls health check behavior.
type HealthConfig struct {
	Interval string `yaml:"interval" json:"interval"`
	Timeout  string `yaml:"timeout"  json:"timeout"`
}

// HistoryConfig controls health history retention.
type HistoryConfig struct {
	RetentionDays int `yaml:"retentionDays" json:"retentionDays"`
}

