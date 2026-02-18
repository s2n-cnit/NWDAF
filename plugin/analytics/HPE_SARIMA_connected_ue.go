package main

import (
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
	"github.com/sartorproj/goarima/autoarima"
	"github.com/sartorproj/goarima/timeseries"
)

// HPESarimaConnectedUEPlugin implements a SARIMA/ARIMA forecasting algorithm to predict the number of connected UEs
// collected from HPE AMF core
type HPESarimaConnectedUEPlugin struct {
	logger                 hclog.Logger
	bufferedLogger         *plugin_shared.BufferedLogger
	config                 *plugin_shared.AnalyticsConfig
	metricBuffer           []models.Metric
	newSamples             int
	fittedModel            *autoarima.Result
	meanSampleTimeDistance time.Duration
	forecastedValues       []models.Metric
	metricPrefix           string
	subscribedMetricName   string
}

var (
	// HPESarimaConnectedUEHandShakeConfigAnalytics is the handshake configuration for analytics plugins.
	HPESarimaConnectedUEHandShakeConfigAnalytics = plugin.HandshakeConfig{
		ProtocolVersion: 1,
	}

	HPESarimaConnectedUEAlgorithm = &HPESarimaConnectedUEPlugin{
		logger: hclog.New(&hclog.LoggerOptions{
			Name:       "HPE_SARIMA_CONNECTED_UE",
			Level:      hclog.Debug,
			Output:     os.Stderr,
			JSONFormat: true,
		}),
		metricBuffer: make([]models.Metric, 0),
		newSamples:   0,
		config:       plugin_shared.NewAnalyticsConfig(), // Use shared configuration
	}
)

func (p *HPESarimaConnectedUEPlugin) SetEnvironment(debugMode bool) {
	p.bufferedLogger = plugin_shared.NewBufferedLogger(p.logger, debugMode)

	configuration.LoadEnvWithBufferedLogger(p.bufferedLogger, "HPE_SARIMA_connected_ue")
	p.logger.SetLevel(hclog.Level(configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug))))

	// Load analytics configuration from environment or use defaults
	p.config = &plugin_shared.AnalyticsConfig{
		WindowSize:          configuration.GetEnvInt(plugin_shared.EnvAnalyticsWindowSize, plugin_shared.DefaultWindowSize),
		SamplesRefreshModel: configuration.GetEnvInt(plugin_shared.EnvAnalyticsSamplesRefreshModel, plugin_shared.DefaultSamplesRefreshModel),
		MinimumSamples:      configuration.GetEnvInt(plugin_shared.EnvAnalyticsMinimumSamples, plugin_shared.DefaultMinimumSamples),
	}

	// Get the metric prefix from environment variable (default: "NWDAF_")
	p.metricPrefix = configuration.GetEnv(configuration.EnvMetricPrefix, "NWDAF_")
	p.subscribedMetricName = fmt.Sprintf("%sCONN_DEV", p.metricPrefix)
	p.bufferedLogger.Info("Subscribed metric name", "metric", p.subscribedMetricName)

	// Skip cookie setup in debug mode
	if !debugMode {
		cookieName := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyName)
		cookieValue := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyValue)
		if cookieName == nil || cookieValue == nil {
			p.logger.Error("Missing COOKIE name and value variables for RPC")
			os.Exit(1)
		}
		HPESarimaConnectedUEHandShakeConfigAnalytics.MagicCookieKey = *cookieName
		HPESarimaConnectedUEHandShakeConfigAnalytics.MagicCookieValue = *cookieValue
	}

	if p.bufferedLogger != nil {
		p.bufferedLogger.StartNormalLogging()
	}
}

// GetName returns the unique name for this analytics plugin.
func (p *HPESarimaConnectedUEPlugin) GetName() string {
	return "HPESarimaConnectedUEPlugin"
}

// GetDescription returns a description of what this plugin does.
func (p *HPESarimaConnectedUEPlugin) GetDescription() string {
	return "SARIMA forecasting algorithm for predicting number of connected UEs from HPE AMF metrics"
}

// GetProducedMetrics returns the list of metric names this plugin produces.
func (p *HPESarimaConnectedUEPlugin) GetProducedMetrics() []string {
	return []string{fmt.Sprintf("%s_forecasted_value", p.subscribedMetricName)}
}

// GetMetricDescriptions returns descriptions for subscribed metrics.
func (p *HPESarimaConnectedUEPlugin) GetMetricDescriptions() map[string]string {
	return map[string]string{
		p.subscribedMetricName: "Number of connected devices from HPE AMF",
	}
}

// GetSubscribedMetrics returns the list of metrics this plugin subscribes to.
func (p *HPESarimaConnectedUEPlugin) GetSubscribedMetrics() []string {
	return []string{p.subscribedMetricName}
}

// GetMinimumSamples returns the minimum number of samples required.
func (p *HPESarimaConnectedUEPlugin) GetMinimumSamples() int {
	return p.config.MinimumSamples
}

// ProcessMetric accumulates metrics in the buffer and returns true when ready to execute.
func (p *HPESarimaConnectedUEPlugin) ProcessMetric(metric models.Metric) bool {
	p.logger.Debug("Received metric", "name", metric.Name, "value", metric.Value)

	if metric.ReceivedAt.IsZero() {
		p.logger.Error("Metric ReceivedAt is zero. Time of metric is not set", "metric", metric)
		return false
	}

	if metric.Name != p.subscribedMetricName {
		p.logger.Error("Metric not subscribed to this plugin", "metric", metric.Name, "expected", p.subscribedMetricName)
		return false
	}

	// Add metric to buffer
	p.metricBuffer = append(p.metricBuffer, metric)
	p.newSamples++

	// Keep only the last windowSize metrics
	if len(p.metricBuffer) > p.config.WindowSize {
		p.metricBuffer = p.metricBuffer[len(p.metricBuffer)-p.config.WindowSize:]
	}

	p.logger.Trace("Metric added to buffer", "bufferSize", len(p.metricBuffer))

	// Execute if we have enough samples
	if len(p.metricBuffer) >= p.config.MinimumSamples {
		p.logger.Debug("Buffer has enough samples, ready to execute", "count", len(p.metricBuffer))
		return true
	}

	p.logger.Debug("Not enough samples yet", "current", len(p.metricBuffer), "minimum", p.config.MinimumSamples)
	return false
}

func (p *HPESarimaConnectedUEPlugin) GetTimeSeries(metricList []models.Metric) (*timeseries.Series, time.Duration) {
	timestamps := make([]time.Time, len(metricList))
	values := make([]float64, len(metricList))

	var cumulativeTimeDistance time.Duration
	for i, metric := range metricList {
		timestamps[i] = metric.ReceivedAt
		values[i] = metric.Value
		if i == 0 {
			cumulativeTimeDistance = time.Duration(0)
		} else {
			cumulativeTimeDistance += timestamps[i].Sub(timestamps[i-1])
		}
	}

	timeSeries, err := timeseries.NewWithTimestamps(timestamps, values)
	if err != nil {
		p.logger.Error("Failed to create time series", "error", err)
		return nil, time.Duration(-1)
	}

	if len(metricList) < 2 {
		return timeSeries, time.Duration(0)
	}

	return timeSeries, cumulativeTimeDistance / time.Duration(len(metricList)-1)
}

// Execute computes forecasted values for the number of connected UEs
func (p *HPESarimaConnectedUEPlugin) Execute() map[string][]models.Metric {
	p.logger.Debug("Executing HPE SARIMA connected UE forecasting algorithm")

	result := make(map[string][]models.Metric)

	if len(p.metricBuffer) < p.config.MinimumSamples {
		p.logger.Warn("Not enough samples in buffer", "samples", len(p.metricBuffer), "minimum", p.config.MinimumSamples)
		return result
	}

	outputMetricName := fmt.Sprintf("%s_forecasted_value", p.subscribedMetricName)

	// Refresh model if needed
	if p.fittedModel == nil || p.newSamples >= p.config.SamplesRefreshModel {
		p.logger.Debug("Refreshing SARIMA model", "newSamples", p.newSamples)
		timeSeries, meanSampleTimeDistance := p.GetTimeSeries(p.metricBuffer)
		if timeSeries == nil {
			p.logger.Error("Failed to create time series")
			return result
		}

		fittedModel, creationModelError := autoarima.AutoARIMA(timeSeries, nil)
		if creationModelError != nil {
			p.logger.Error("Failed to create ARIMA model", "error", creationModelError)
			p.fittedModel = nil
			return result
		}

		// Check if the model is nil even when no error was returned
		if fittedModel == nil {
			p.logger.Error("ARIMA model is nil despite no error")
			p.fittedModel = nil
			return result
		}

		p.newSamples = 0
		p.fittedModel = fittedModel
		p.meanSampleTimeDistance = meanSampleTimeDistance

		// Generate forecasts using the instance variable (not the local one)
		forecasts, predictErr := p.fittedModel.Predict(p.config.SamplesRefreshModel)
		if predictErr != nil {
			p.logger.Error("Failed to generate forecasts", "error", predictErr)
			return result
		}

		p.forecastedValues = make([]models.Metric, 0, len(forecasts))

		lastMetric := p.metricBuffer[len(p.metricBuffer)-1]
		for index, forecastedValue := range forecasts {
			computedMetric := models.Metric{
				Name:        outputMetricName,
				Description: fmt.Sprintf("Forecasted value of connected UEs from HPE AMF (index=%d)", index),
				Value:       forecastedValue,
				NFid:        lastMetric.NFid,
				NFType:      lastMetric.NFType,
				ReceivedAt:  lastMetric.ReceivedAt.Add(p.meanSampleTimeDistance * time.Duration(index+1)),
			}
			p.forecastedValues = append(p.forecastedValues, computedMetric)
		}

		p.logger.Debug("Generated forecasted values", "count", len(p.forecastedValues))
		result[outputMetricName] = p.forecastedValues
	} else {
		// Return remaining forecasted values
		if p.newSamples >= len(p.forecastedValues) {
			p.logger.Debug("No more forecasted values to return, waiting for model refresh")
			result[outputMetricName] = []models.Metric{}
		} else {
			p.logger.Debug("Returning remaining forecasted values", "remaining", len(p.forecastedValues)-p.newSamples)
			result[outputMetricName] = p.forecastedValues[p.newSamples:]
		}
	}

	return result
}

// GetRequiredEnvVars returns the list of required environment variables.
func (p *HPESarimaConnectedUEPlugin) GetRequiredEnvVars() []string {
	// The plugin can optionally use METRIC_PREFIX environment variable
	// If not set, it will use the default "NWDAF_"
	return []string{}
}

// GetStartupLogs returns buffered logs from plugin initialization.
func (p *HPESarimaConnectedUEPlugin) GetStartupLogs() []plugin_shared.StartupLog {
	if p.bufferedLogger == nil {
		return []plugin_shared.StartupLog{}
	}
	return p.bufferedLogger.GetStartupLogs()
}

// main is the entry point for the plugin.
func main() {
	debugLocally := false
	args := os.Args[1:]
	for _, argument := range args {
		if argument == "--debug-locally" {
			debugLocally = true
		}
	}

	HPESarimaConnectedUEAlgorithm.SetEnvironment(debugLocally)

	// If run as a standalone program for debugging
	if debugLocally {
		// Test the plugin locally
		HPESarimaConnectedUEAlgorithm.logger.Info("Running in debug mode")

		// Simulate some metrics for connected devices
		// Generate synthetic data for testing
		testMetrics := make([]models.Metric, 30)
		baseTime := time.Now()
		baseValue := 50.0 // Base number of connected UEs

		for i := 0; i < 30; i++ {
			// Simulate a pattern with some variation
			value := baseValue + float64(i%10)*2.0 // Some periodic pattern
			testMetrics[i] = models.Metric{
				Name:        HPESarimaConnectedUEAlgorithm.subscribedMetricName,
				Description: "Number of connected devices",
				Value:       value,
				NFid:        "hpe-amf-001",
				NFType:      "AMF",
				ReceivedAt:  baseTime.Add(time.Second * time.Duration(i*10)),
			}
		}

		HPESarimaConnectedUEAlgorithm.logger.Info("Processing test metrics", "count", len(testMetrics))

		for _, metric := range testMetrics {
			ready := HPESarimaConnectedUEAlgorithm.ProcessMetric(metric)
			if ready {
				forecastMetricsMap := HPESarimaConnectedUEAlgorithm.Execute()
				outputMetricName := fmt.Sprintf("%s_forecasted_value", HPESarimaConnectedUEAlgorithm.subscribedMetricName)
				forecastMetrics := forecastMetricsMap[outputMetricName]
				HPESarimaConnectedUEAlgorithm.logger.Info("Forecast generated", "count", len(forecastMetrics))
				for _, fm := range forecastMetrics {
					HPESarimaConnectedUEAlgorithm.logger.Info("Forecasted value", "value", fm.Value, "time", fm.ReceivedAt)
				}
			}
		}

		HPESarimaConnectedUEAlgorithm.logger.Info("Debug mode completed successfully")
	} else {
		// Run as a plugin
		pluginMap := map[string]plugin.Plugin{
			"HPE_SARIMA_connected_ue": &plugin_shared.AnalyticsAlgorithmPlugin{Impl: HPESarimaConnectedUEAlgorithm},
		}
		HPESarimaConnectedUEAlgorithm.logger.Info("Offered analytics plugin", "plugins", pluginMap)

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: HPESarimaConnectedUEHandShakeConfigAnalytics,
			Plugins:         pluginMap,
		})
	}
}
