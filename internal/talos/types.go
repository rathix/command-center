package talos

import (
	"context"
	"time"
)

// NodeHealthStatus represents the health state of a Talos node.
type NodeHealthStatus string

const (
	NodeReady       NodeHealthStatus = "ready"
	NodeNotReady    NodeHealthStatus = "not-ready"
	NodeUnreachable NodeHealthStatus = "unreachable"
)

// NodeMetrics holds resource utilization percentages for a node.
type NodeMetrics struct {
	CPUPercent    float64   `json:"cpuPercent"`
	MemoryPercent float64   `json:"memoryPercent"`
	DiskPercent   float64   `json:"diskPercent"`
	Timestamp     time.Time `json:"timestamp"`
}

// UpgradeInfo holds version information for a node upgrade.
type UpgradeInfo struct {
	CurrentVersion string `json:"currentVersion"`
	TargetVersion  string `json:"targetVersion"`
}

// NodeHealth represents the health and role of a single Talos node.
type NodeHealth struct {
	Name     string           `json:"name"`
	Health   NodeHealthStatus `json:"health"`
	Role     string           `json:"role"`
	LastSeen time.Time        `json:"lastSeen"`
	Metrics  *NodeMetrics     `json:"metrics,omitempty"`
}

// NodeClient is the interface consumed by the poller to interact with Talos nodes.
// Production implementations use the Talos gRPC API; tests use a mock.
type NodeClient interface {
	ListNodes(ctx context.Context) ([]NodeHealth, error)
	GetMetrics(ctx context.Context) (map[string]NodeMetrics, error)
	Reboot(ctx context.Context, nodeName string) error
	Upgrade(ctx context.Context, nodeName string, targetVersion string) error
	GetUpgradeInfo(ctx context.Context, nodeName string) (*UpgradeInfo, error)
}
