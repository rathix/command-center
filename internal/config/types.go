package config

// Config is the top-level configuration parsed from the YAML config file.
type Config struct {
	Services      []CustomService        `yaml:"services"      json:"services"`
	Overrides     []ServiceOverride      `yaml:"overrides"     json:"overrides"`
	Groups        map[string]GroupConfig `yaml:"groups"        json:"groups"`
	Health        HealthConfig           `yaml:"health"        json:"health"`
	History       HistoryConfig          `yaml:"history"       json:"history"`
	Notifications *NotificationsConfig   `yaml:"notifications" json:"notifications,omitempty"`
	Talos         *TalosConfig           `yaml:"talos"         json:"talos,omitempty"`
	Keyboard      *KeyboardConfig        `yaml:"keyboard"      json:"keyboard,omitempty"`
	Terminal      TerminalConfig         `yaml:"terminal"      json:"terminal"`
}

// TalosConfig configures the Talos gRPC API connection for node management.
type TalosConfig struct {
	Endpoint     string `yaml:"endpoint"     json:"endpoint"`
	PollInterval string `yaml:"pollInterval" json:"pollInterval"`
}

// TerminalConfig controls in-browser terminal settings.
type TerminalConfig struct {
	Enabled         bool     `yaml:"enabled"         json:"enabled"`
	AllowedCommands []string `yaml:"allowedCommands" json:"allowedCommands"`
	IdleTimeout     string   `yaml:"idleTimeout"     json:"idleTimeout"`
	MaxSessions     int      `yaml:"maxSessions"     json:"maxSessions"`
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

// NotificationsConfig configures the notification engine.
type NotificationsConfig struct {
	Adapters []AdapterConfig    `yaml:"adapters" json:"adapters"`
	Rules    []NotificationRule `yaml:"rules"    json:"rules"`
}

// AdapterConfig defines a notification delivery adapter.
type AdapterConfig struct {
	Type     string `yaml:"type"     json:"type"`
	Name     string `yaml:"name"     json:"name"`
	URL      string `yaml:"url"      json:"url"`
	UserKey  string `yaml:"userKey"  json:"userKey"`
	AppToken string `yaml:"appToken" json:"appToken"`
}

// NotificationRule defines per-service routing for notifications.
type NotificationRule struct {
	Services            []string `yaml:"services"            json:"services"`
	Transitions         []string `yaml:"transitions"         json:"transitions"`
	Channels            []string `yaml:"channels"            json:"channels"`
	SuppressionInterval string   `yaml:"suppressionInterval" json:"suppressionInterval"`
	EscalateAfter       string   `yaml:"escalateAfter"       json:"escalateAfter"`
	EscalationChannels  []string `yaml:"escalationChannels"  json:"escalationChannels"`
}

// KeyboardConfig defines custom keyboard shortcut bindings.
type KeyboardConfig struct {
	Mod      string            `yaml:"mod"      json:"mod"`
	Bindings map[string]string `yaml:"bindings" json:"bindings"`
}

