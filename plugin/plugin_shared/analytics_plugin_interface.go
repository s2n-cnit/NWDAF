// Package plugin_shared contains the interface and RPC implementation for analytics plugins.
// Analytics plugins subscribe to metrics from Redis, accumulate temporal data, and produce computed metrics.
package plugin_shared

import (
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/models"
)

// MetricBuffer is a time-series buffer for storing historical metric values.
type MetricBuffer struct {
	Metrics []models.Metric
}

// AnalyticsAlgorithm is the interface that analytics plugins must implement.
// Plugins subscribe to specific metrics, accumulate data, and produce computed metrics.
type AnalyticsAlgorithm interface {
	// GetSubscribedMetrics returns the list of metric names this plugin is interested in.
	// The plugin will only receive metrics with names in this list.
	GetSubscribedMetrics() []string

	// GetMinimumSamples returns the minimum number of samples required before the algorithm can run.
	// Return 0 if the algorithm can run immediately with any number of samples.
	GetMinimumSamples() int

	// ProcessMetric is called when a new metric matching the subscription list is received.
	// The plugin should store the metric in its internal buffer for temporal accumulation.
	// Returns true if the algorithm should be executed after this metric is added.
	ProcessMetric(metric models.Metric) bool

	// Execute runs the analytics algorithm on the accumulated data.
	// Returns a map indexed by metric name, with slices of computed metrics to be published to Redis.
	Execute() map[string][]models.Metric

	// GetRequiredEnvVars returns the list of environment variables required by the plugin.
	GetRequiredEnvVars() []string

	// GetName returns a unique name for this analytics plugin.
	GetName() string
}

// AnalyticsAlgorithmRPC is the RPC client implementation of AnalyticsAlgorithm.
type AnalyticsAlgorithmRPC struct {
	client *rpc.Client
}

// GetSubscribedMetrics calls the remote GetSubscribedMetrics method via RPC.
func (g *AnalyticsAlgorithmRPC) GetSubscribedMetrics() []string {
	var resp []string
	err := g.client.Call("Plugin.GetSubscribedMetrics", new(interface{}), &resp)
	if err != nil {
		panic(err)
	}
	return resp
}

// GetMinimumSamples calls the remote GetMinimumSamples method via RPC.
func (g *AnalyticsAlgorithmRPC) GetMinimumSamples() int {
	var resp int
	err := g.client.Call("Plugin.GetMinimumSamples", new(interface{}), &resp)
	if err != nil {
		panic(err)
	}
	return resp
}

// ProcessMetric calls the remote ProcessMetric method via RPC.
func (g *AnalyticsAlgorithmRPC) ProcessMetric(metric models.Metric) bool {
	var resp bool
	err := g.client.Call("Plugin.ProcessMetric", metric, &resp)
	if err != nil {
		panic(err)
	}
	return resp
}

// Execute calls the remote Execute method via RPC.
func (g *AnalyticsAlgorithmRPC) Execute() map[string][]models.Metric {
	var resp map[string][]models.Metric
	err := g.client.Call("Plugin.Execute", new(interface{}), &resp)
	if err != nil {
		panic(err)
	}
	return resp
}

// GetRequiredEnvVars calls the remote GetRequiredEnvVars method via RPC.
func (g *AnalyticsAlgorithmRPC) GetRequiredEnvVars() []string {
	var resp []string
	err := g.client.Call("Plugin.GetRequiredEnvVars", new(interface{}), &resp)
	if err != nil {
		panic(err)
	}
	return resp
}

// GetName calls the remote GetName method via RPC.
func (g *AnalyticsAlgorithmRPC) GetName() string {
	var resp string
	err := g.client.Call("Plugin.GetName", new(interface{}), &resp)
	if err != nil {
		panic(err)
	}
	return resp
}

// AnalyticsAlgorithmRPCServer is the RPC server that AnalyticsAlgorithmRPC talks to.
type AnalyticsAlgorithmRPCServer struct {
	Impl AnalyticsAlgorithm
}

// GetSubscribedMetrics calls GetSubscribedMetrics on the implementation.
func (s *AnalyticsAlgorithmRPCServer) GetSubscribedMetrics(args interface{}, resp *[]string) error {
	*resp = s.Impl.GetSubscribedMetrics()
	return nil
}

// GetMinimumSamples calls GetMinimumSamples on the implementation.
func (s *AnalyticsAlgorithmRPCServer) GetMinimumSamples(args interface{}, resp *int) error {
	*resp = s.Impl.GetMinimumSamples()
	return nil
}

// ProcessMetric calls ProcessMetric on the implementation.
func (s *AnalyticsAlgorithmRPCServer) ProcessMetric(metric models.Metric, resp *bool) error {
	*resp = s.Impl.ProcessMetric(metric)
	return nil
}

// Execute calls Execute on the implementation.
func (s *AnalyticsAlgorithmRPCServer) Execute(args interface{}, resp *map[string][]models.Metric) error {
	*resp = s.Impl.Execute()
	return nil
}

// GetRequiredEnvVars calls GetRequiredEnvVars on the implementation.
func (s *AnalyticsAlgorithmRPCServer) GetRequiredEnvVars(args interface{}, resp *[]string) error {
	*resp = s.Impl.GetRequiredEnvVars()
	return nil
}

// GetName calls GetName on the implementation.
func (s *AnalyticsAlgorithmRPCServer) GetName(args interface{}, resp *string) error {
	*resp = s.Impl.GetName()
	return nil
}

// AnalyticsAlgorithmPlugin is the plugin implementation for AnalyticsAlgorithm.
type AnalyticsAlgorithmPlugin struct {
	Impl AnalyticsAlgorithm
}

// Server returns an RPC server for the AnalyticsAlgorithm.
func (p *AnalyticsAlgorithmPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &AnalyticsAlgorithmRPCServer{Impl: p.Impl}, nil
}

// Client returns an RPC client for the AnalyticsAlgorithm.
func (AnalyticsAlgorithmPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &AnalyticsAlgorithmRPC{client: c}, nil
}
