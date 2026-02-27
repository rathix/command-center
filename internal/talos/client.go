package talos

import (
	"context"
	"fmt"
	"time"
)

// GRPCClient implements NodeClient using the Talos gRPC API.
// The actual gRPC calls require siderolabs/talos/pkg/machinery and network
// access to the Talos API endpoint. This implementation dials per-request
// with a timeout to avoid silent connection failures (architecture risk #7).
//
// TODO(spike): Verify Talos gRPC API access from Command Center's network
// position before completing the gRPC implementation.
type GRPCClient struct {
	endpoint string
	timeout  time.Duration
}

// ClientOption configures a GRPCClient.
type ClientOption func(*GRPCClient)

// WithTimeout sets the per-request dial timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *GRPCClient) {
		c.timeout = d
	}
}

// NewGRPCClient creates a new Talos gRPC client for the given endpoint.
func NewGRPCClient(endpoint string, opts ...ClientOption) (*GRPCClient, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("talos endpoint is required")
	}
	c := &GRPCClient{
		endpoint: endpoint,
		timeout:  10 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// ListNodes queries the Talos API for node health information.
// TODO: Implement using siderolabs/talos/pkg/machinery MachineService.
func (c *GRPCClient) ListNodes(ctx context.Context) ([]NodeHealth, error) {
	return nil, fmt.Errorf("talos gRPC client not yet implemented: ListNodes (endpoint: %s)", c.endpoint)
}

// GetMetrics queries the Talos API for node resource metrics.
// TODO: Implement using MachineService.SystemStat, Memory, DiskStats.
func (c *GRPCClient) GetMetrics(ctx context.Context) (map[string]NodeMetrics, error) {
	return nil, fmt.Errorf("talos gRPC client not yet implemented: GetMetrics (endpoint: %s)", c.endpoint)
}

// Reboot sends a reboot command to the specified node.
// TODO: Implement using MachineService.Reboot.
func (c *GRPCClient) Reboot(ctx context.Context, nodeName string) error {
	return fmt.Errorf("talos gRPC client not yet implemented: Reboot (endpoint: %s, node: %s)", c.endpoint, nodeName)
}

// Upgrade sends an upgrade command to the specified node.
// TODO: Implement using MachineService.Upgrade.
func (c *GRPCClient) Upgrade(ctx context.Context, nodeName string, targetVersion string) error {
	return fmt.Errorf("talos gRPC client not yet implemented: Upgrade (endpoint: %s, node: %s, version: %s)", c.endpoint, nodeName, targetVersion)
}

// GetUpgradeInfo queries version information for the specified node.
// TODO: Implement using MachineService.Version.
func (c *GRPCClient) GetUpgradeInfo(ctx context.Context, nodeName string) (*UpgradeInfo, error) {
	return nil, fmt.Errorf("talos gRPC client not yet implemented: GetUpgradeInfo (endpoint: %s, node: %s)", c.endpoint, nodeName)
}
