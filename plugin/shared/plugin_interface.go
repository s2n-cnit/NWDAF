package shared

import (
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"net/rpc"
)

// MetricCollector is the interface that we're exposing as a plugin.
type MetricCollector interface {
	// Collect gathers metrics and returns a slice of Metric objects.
	Collect() []models.Metric
}

// MetricCollectorRPC is the implementation of the MetricCollector interface over RPC.
type MetricCollectorRPC struct{ client *rpc.Client }

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
