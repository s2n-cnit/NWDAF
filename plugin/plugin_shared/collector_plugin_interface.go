// Package plugin_shared Description: This file contains the interface that the plugin exposes, the RPC implementation of the interface, and the plugin implementation for the interface.
package plugin_shared

import (
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/models"
)

// MetricCollector is the interface that we're exposing as a plugin.
// This interface is implemented by the plugin and called by the plugin caller
type MetricCollector interface {
	// Collect gathers metrics and returns a slice of Metric objects.
	Collect() []models.Metric
	// GetRequiredEnvVars returns a slice of environment variables required by the plugin. If any of these
	// environment variables are not set, the plugin will not be able to run.
	GetRequiredEnvVars() []string
	// GetStartupLogs returns buffered logs from plugin initialization.
	// This is called by the host after RPC handshake completes to retrieve
	// any warnings or info messages that occurred during plugin setup.
	// The buffer is cleared after this call.
	GetStartupLogs() []StartupLog
}

// MetricCollectorRPC is the implementation of the MetricCollector interface over RPC.
type MetricCollectorRPC struct {
	client *rpc.Client
}

// Collect calls the remote Collect method via RPC and returns the collected metrics.
func (g *MetricCollectorRPC) Collect() []models.Metric {
	var resp []models.Metric
	err := g.client.Call("Plugin.Collect", new(interface{}), &resp)
	if err != nil {
		// You usually want your interfaces to return errors. If they don't,
		// there isn't much other choice here.
		panic(err)
	}

	return resp
}

// GetRequiredEnvVars calls the remote GetRequiredEnvVars method via RPC and returns the required environment variables.
func (g *MetricCollectorRPC) GetRequiredEnvVars() []string {
	var resp []string
	err := g.client.Call("Plugin.GetRequiredEnvVars", new(interface{}), &resp)
	if err != nil {
		panic(err)
	}

	return resp
}

// GetStartupLogs calls the remote GetStartupLogs method via RPC.
func (g *MetricCollectorRPC) GetStartupLogs() []StartupLog {
	var resp []StartupLog
	err := g.client.Call("Plugin.GetStartupLogs", new(interface{}), &resp)
	if err != nil {
		// Return empty slice if method not implemented (backward compatibility)
		return []StartupLog{}
	}
	return resp
}

// MetricCollectorRPCServer is the RPC server that MetricCollectorRPC talks to,
// conforming to the requirements of net/rpc.
type MetricCollectorRPCServer struct {
	// Impl is the real implementation of the MetricCollector interface.
	Impl MetricCollector
}

// Collect calls the Collect method on the real implementation and sets the response.
func (s *MetricCollectorRPCServer) Collect(args interface{}, resp *[]models.Metric) error {
	*resp = s.Impl.Collect()
	return nil
}

// GetRequiredEnvVars Collect calls the RequiredEnvVars method on the real implementation and sets the response.
func (s *MetricCollectorRPCServer) GetRequiredEnvVars(args interface{}, resp *[]string) error {
	print("RequiredEnvVars")
	*resp = s.Impl.GetRequiredEnvVars()
	return nil
}

// GetStartupLogs calls GetStartupLogs on the implementation.
func (s *MetricCollectorRPCServer) GetStartupLogs(args interface{}, resp *[]StartupLog) error {
	*resp = s.Impl.GetStartupLogs()
	return nil
}

// MetricCollectorPlugin is the plugin implementation for MetricCollector.
type MetricCollectorPlugin struct {
	// Impl is the injected implementation of the MetricCollector interface.
	Impl MetricCollector
}

// Server returns an RPC server for the MetricCollector.
func (p *MetricCollectorPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &MetricCollectorRPCServer{Impl: p.Impl}, nil
}

// Client returns an RPC client for the MetricCollector.
func (MetricCollectorPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &MetricCollectorRPC{client: c}, nil
}
