package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)

// FakeCollector is a fake collector for testing purposes that generates random metrics.
type FakeCollector struct {
	logger       hclog.Logger
	metricPrefix string
	random       *rand.Rand
}

var (
	// HandShakeConfigFakeCollector is the handshake configuration for the fake collector plugin.
	HandShakeConfigFakeCollector = plugin.HandshakeConfig{
		ProtocolVersion: 1,
	}

	FakeTestCollector = &FakeCollector{
		logger: hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Debug,
			Output:     os.Stderr,
			JSONFormat: true,
		}),
		random: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
)

func (collector *FakeCollector) SetEnvironment(debugMode bool) {
	configuration.LoadEnv()
	collector.logger.SetLevel(hclog.Level(configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug))))
	collector.metricPrefix = configuration.GetEnv(configuration.EnvMetricPrefix, "NWDAF_")

	// Skip cookie setup in debug mode
	if !debugMode {
		cookieName := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyName)
		cookieValue := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyValue)
		if cookieName == nil || cookieValue == nil {
			collector.logger.Error("Missing COOKIE name and value variables for RPC")
			os.Exit(1)
		}
		HandShakeConfigFakeCollector.MagicCookieKey = *cookieName
		HandShakeConfigFakeCollector.MagicCookieValue = *cookieValue
	}
}

// main is the entry point for the FakeCollector application.
func main() {
	debug_locally := false
	args := os.Args[1:]
	for _, argument := range args {
		if argument == "--debug-locally" {
			debug_locally = true
		}
	}

	FakeTestCollector.SetEnvironment(debug_locally)

	// If run as a standalone program, collect metrics locally, if loaded as a plugin, serve the plugin
	if debug_locally {
		metrics := FakeTestCollector.Collect()
		FakeTestCollector.logger.Info("Collected metrics", "count", len(metrics), "metrics", metrics)
	} else {
		// pluginMap is the map of plugins we can dispense.
		var pluginMap = map[string]plugin.Plugin{
			"FAKE_test_collector": &plugin_shared.MetricCollectorPlugin{Impl: FakeTestCollector},
		}
		FakeTestCollector.logger.Info("Offered plugins: ", pluginMap)

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: HandShakeConfigFakeCollector,
			Plugins:         pluginMap,
		})
	}
}

// Collect generates fake metrics for testing purposes.
func (collector *FakeCollector) Collect() []models.Metric {
	collector.logger.Debug("Collecting fake metrics for testing")

	metrics := []models.Metric{
		{
			Name:        collector.metricPrefix + "cpu_usage_percent",
			Description: "CPU utilization percentage",
			Value:       collector.randomFloat(20, 80),
			NFid:        "amf-001",
			NFType:      "AMF",
		},
		{
			Name:        collector.metricPrefix + "memory_usage_bytes",
			Description: "Memory usage in bytes",
			Value:       collector.randomFloat(1e9, 4e9),
			NFid:        "amf-001",
			NFType:      "AMF",
		},
		{
			Name:        collector.metricPrefix + "active_sessions",
			Description: "Number of active UE sessions",
			Value:       collector.randomFloat(1000, 2000),
			NFid:        "amf-001",
			NFType:      "AMF",
		},
		{
			Name:        collector.metricPrefix + "registration_requests_total",
			Description: "Total registration requests received",
			Value:       collector.randomFloat(40000, 50000),
			NFid:        "amf-001",
			NFType:      "AMF",
		},
		{
			Name:        collector.metricPrefix + "cpu_usage_percent",
			Description: "CPU utilization percentage",
			Value:       collector.randomFloat(30, 70),
			NFid:        "smf-001",
			NFType:      "SMF",
		},
		{
			Name:        collector.metricPrefix + "pdu_sessions_active",
			Description: "Number of active PDU sessions",
			Value:       collector.randomFloat(500, 1200),
			NFid:        "smf-001",
			NFType:      "SMF",
		},
		{
			Name:        collector.metricPrefix + "throughput_mbps",
			Description: "Network throughput in Mbps",
			Value:       collector.randomFloat(800, 1500),
			NFid:        "upf-001",
			NFType:      "UPF",
		},
		{
			Name:        collector.metricPrefix + "packet_loss_percent",
			Description: "Packet loss percentage",
			Value:       collector.randomFloat(0.05, 0.5),
			NFid:        "upf-001",
			NFType:      "UPF",
		},
		{
			Name:        collector.metricPrefix + "latency_ms",
			Description: "Average network latency in milliseconds",
			Value:       collector.randomFloat(10, 25),
			NFid:        "upf-001",
			NFType:      "UPF",
		},
		{
			Name:        collector.metricPrefix + "authentication_success_rate",
			Description: "Successful authentication rate percentage",
			Value:       collector.randomFloat(95, 99.5),
			NFid:        "ausf-001",
			NFType:      "AUSF",
		},
		{
			Name:        "NWDAF_cpu_usage_percent",
			Description: "CPU usage percentage by NWDAF",
			Value:       collector.randomFloat(0, 5),
			NFid:        "nwdaf-001",
			NFType:      "NWDAF",
		},
		{
			Name:        "NWDAF_memory_usage_bytes",
			Description: "Memory usage in bytes by NWDAF",
			Value:       collector.randomFloat(40, 45.5),
			NFid:        "nwdaf-001",
			NFType:      "NWDAF",
		},
	}

	for metric := range metrics {
		metrics[metric].ReceivedAt = time.Now()
	}

	collector.logger.Debug("Generated fake metrics", "count", len(metrics))
	return metrics
}

// GetRequiredEnvVars returns the list of environment variables required by the plugin.
// The fake collector doesn't require any special environment variables.
func (collector *FakeCollector) GetRequiredEnvVars() []string {
	return []string{}
}

// randomFloat generates a random float64 between min and max.
func (collector *FakeCollector) randomFloat(min, max float64) float64 {
	return min + collector.random.Float64()*(max-min)
}
