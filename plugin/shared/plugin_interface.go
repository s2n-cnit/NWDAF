package shared

import (
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"net/rpc"
)

// MetricCollector is the interface that we're exposing as a plugin.
type MetricCollector interface {
	Collect() []models.Metric
}

// Here is an implementation that talks over RPC
type MetricCollectorRPC struct{ client *rpc.Client }

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

// Here is the RPC server that MetricCollectorRPC talks to, conforming to
// the requirements of net/rpc
type MetricCollectorRPCServer struct {
	// This is the real implementation
	Impl MetricCollector
}

func (s *MetricCollectorRPCServer) Collect(args interface{}, resp *[]models.Metric) error {
	*resp = s.Impl.Collect()
	return nil
}

// This is the implementation of plugin.Plugin so we can serve/consume this
//
// This has two methods: Server must return an RPC server for this plugin
// type. We construct a MetricCollectorRPCServer for this.
//
// Client must return an implementation of our interface that communicates
// over an RPC client. We return MetricCollectorRPC for this.
//
// Ignore MuxBroker. That is used to create more multiplexed streams on our
// plugin connection and is a more advanced use case.
type MetricCollectorPlugin struct {
	// Impl Injection
	Impl MetricCollector
}

func (p *MetricCollectorPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &MetricCollectorRPCServer{Impl: p.Impl}, nil
}

func (MetricCollectorPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &MetricCollectorRPC{client: c}, nil
}
