package main

import (
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
	"os"
)

// MovingAveragePlugin implements a simple moving average analytics algorithm.
// It subscribes to CPU usage metrics and computes a moving average.
type MovingAveragePlugin struct {
	logger         hclog.Logger
	metricBuffer   []models.Metric
	windowSize     int
	minimumSamples int
}

var (
	// HandShakeConfigAnalytics is the handshake configuration for analytics plugins.
	HandShakeConfigAnalytics = plugin.HandshakeConfig{
		ProtocolVersion: 1,
	}

	MovingAverageAlgorithm = &MovingAveragePlugin{
		logger: hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Debug,
			Output:     os.Stderr,
			JSONFormat: true,
		}),
		metricBuffer:   make([]models.Metric, 0),
		windowSize:     5, // Use last 5 samples for moving average
		minimumSamples: 3, // Need at least 3 samples before computing
	}
)

func (p *MovingAveragePlugin) SetEnvironment(debugMode bool) {
	configuration.LoadEnv()
	p.logger.SetLevel(hclog.Level(configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug))))

	// Skip cookie setup in debug mode
	if !debugMode {
		cookieName := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyName)
		cookieValue := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyValue)
		if cookieName == nil || cookieValue == nil {
			p.logger.Error("Missing COOKIE name and value variables for RPC")
			os.Exit(1)
		}
		HandShakeConfigAnalytics.MagicCookieKey = *cookieName
		HandShakeConfigAnalytics.MagicCookieValue = *cookieValue
	}
}

// GetName returns the unique name for this plugin.
func (p *MovingAveragePlugin) GetName() string {
	return "MovingAveragePlugin"
}

// GetSubscribedMetrics returns the list of metrics this plugin subscribes to.
func (p *MovingAveragePlugin) GetSubscribedMetrics() []string {
	return []string{
		"NWDAF_cpu_usage_percent",
		"NWDAF_memory_usage_bytes",
	}
}

// GetMinimumSamples returns the minimum number of samples required.
func (p *MovingAveragePlugin) GetMinimumSamples() int {
	return p.minimumSamples
}

// ProcessMetric accumulates metrics in the buffer and returns true when ready to execute.
func (p *MovingAveragePlugin) ProcessMetric(metric models.Metric) bool {
	p.logger.Debug("Processing metric", "name", metric.Name, "value", metric.Value)

	// Add metric to buffer
	p.metricBuffer = append(p.metricBuffer, metric)

	// Keep only the last windowSize metrics for each metric name
	// Simple implementation: keep last windowSize overall (should be improved for production)
	if len(p.metricBuffer) > p.windowSize*2 {
		p.metricBuffer = p.metricBuffer[len(p.metricBuffer)-p.windowSize*2:]
	}

	// Execute if we have enough samples
	if len(p.metricBuffer) >= p.minimumSamples {
		p.logger.Debug("Buffer has enough samples, ready to execute", "bufferSize", len(p.metricBuffer))
		return true
	}

	p.logger.Debug("Not enough samples yet", "current", len(p.metricBuffer), "minimum", p.minimumSamples)
	return false
}

// Execute computes the moving average and returns computed metrics.
func (p *MovingAveragePlugin) Execute() []models.Metric {
	p.logger.Debug("Executing moving average algorithm", "bufferSize", len(p.metricBuffer))

	if len(p.metricBuffer) == 0 {
		return []models.Metric{}
	}

	// Group metrics by name
	metricsByName := make(map[string][]models.Metric)
	for _, metric := range p.metricBuffer {
		metricsByName[metric.Name] = append(metricsByName[metric.Name], metric)
	}

	// Compute moving average for each metric type
	computedMetrics := make([]models.Metric, 0)

	for metricName, metrics := range metricsByName {
		if len(metrics) < p.minimumSamples {
			continue
		}

		// Calculate simple moving average
		sum := 0.0
		count := 0
		windowStart := len(metrics) - p.windowSize
		if windowStart < 0 {
			windowStart = 0
		}

		for i := windowStart; i < len(metrics); i++ {
			sum += metrics[i].Value
			count++
		}

		average := sum / float64(count)

		// Create computed metric
		computedMetric := models.Metric{
			Name:        fmt.Sprintf("%s_moving_avg", metricName),
			Description: fmt.Sprintf("Moving average of %s (window=%d)", metricName, count),
			Value:       average,
			NFid:        metrics[0].NFid,
			NFType:      metrics[0].NFType,
		}

		computedMetrics = append(computedMetrics, computedMetric)
		p.logger.Info("Computed moving average",
			"metric", metricName,
			"average", average,
			"samples", count)
	}

	return computedMetrics
}

// GetRequiredEnvVars returns the list of required environment variables.
func (p *MovingAveragePlugin) GetRequiredEnvVars() []string {
	// No special environment variables required for this dummy plugin
	return []string{}
}

// main is the entry point for the plugin.
func main() {
	debug_locally := false
	args := os.Args[1:]
	for _, argument := range args {
		if argument == "--debug-locally" {
			debug_locally = true
		}
	}

	MovingAverageAlgorithm.SetEnvironment(debug_locally)

	// If run as a standalone program for debugging
	if debug_locally {
		// Test the plugin locally
		MovingAverageAlgorithm.logger.Info("Running in debug mode")

		// Simulate some metrics
		testMetrics := []models.Metric{
			{Name: "NWDAF_cpu_usage_percent", Value: 45.0, NFid: "amf-001", NFType: "AMF"},
			{Name: "NWDAF_cpu_usage_percent", Value: 50.0, NFid: "amf-001", NFType: "AMF"},
			{Name: "NWDAF_cpu_usage_percent", Value: 55.0, NFid: "amf-001", NFType: "AMF"},
		}

		for _, metric := range testMetrics {
			shouldExecute := MovingAverageAlgorithm.ProcessMetric(metric)
			if shouldExecute {
				computed := MovingAverageAlgorithm.Execute()
				MovingAverageAlgorithm.logger.Info("Computed metrics", "count", len(computed), "metrics", computed)
			}
		}
	} else {
		// Run as a plugin
		pluginMap := map[string]plugin.Plugin{
			"FAKE_moving_average": &plugin_shared.AnalyticsAlgorithmPlugin{Impl: MovingAverageAlgorithm},
		}
		MovingAverageAlgorithm.logger.Info("Offered analytics plugin", "plugins", pluginMap)

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: HandShakeConfigAnalytics,
			Plugins:         pluginMap,
		})
	}
}
