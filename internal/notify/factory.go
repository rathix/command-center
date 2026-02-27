package notify

import (
	"fmt"

	"github.com/rathix/command-center/internal/config"
)

// BuildAdapters creates adapters from configuration.
func BuildAdapters(configs []config.AdapterConfig) (map[string]Adapter, error) {
	adapters := make(map[string]Adapter, len(configs))
	for _, cfg := range configs {
		switch cfg.Type {
		case "webhook":
			if cfg.URL == "" {
				return nil, fmt.Errorf("adapter %q: webhook requires url", cfg.Name)
			}
			adapters[cfg.Name] = NewWebhookAdapter(cfg.Name, cfg.URL)
		default:
			return nil, fmt.Errorf("adapter %q: unknown type %q", cfg.Name, cfg.Type)
		}
	}
	return adapters, nil
}
