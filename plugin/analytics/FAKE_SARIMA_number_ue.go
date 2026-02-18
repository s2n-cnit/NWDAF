package main

import (
	"fmt"
	"math/rand"
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

var SarimaNueSubscribedMetrics = []string{"NWDAF_cpu_usage_percent", "NWDAF_memory_usage_bytes"}

// SarimaNuePlugin implements a simple SARIMA/ARIMA forecasting algorithm to predict the number of connected UE's'
type SarimaNuePlugin struct {
	logger                 hclog.Logger
	bufferedLogger         *plugin_shared.BufferedLogger // Used during initialization to buffer logs
	config                 *plugin_shared.AnalyticsConfig
	metricBuffer           map[string][]models.Metric
	newSamples             map[string]int
	fittedModels           map[string]*autoarima.Result
	meanSampleTimeDistance map[string]time.Duration
	forecastedValues       map[string][]models.Metric
	lastProcessedMetric    string // Track which metric triggered the execution
}

var (
	// SarimaNueHandShakeConfigAnalytics HandShakeConfigAnalytics is the handshake configuration for analytics plugins.
	SarimaNueHandShakeConfigAnalytics = plugin.HandshakeConfig{
		ProtocolVersion: 1,
	}

	SarimaNueAlgorithm = &SarimaNuePlugin{
		logger: hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Debug,
			Output:     os.Stderr,
			JSONFormat: true,
		}),
		metricBuffer:           make(map[string][]models.Metric),
		newSamples:             make(map[string]int),
		config:                 plugin_shared.NewAnalyticsConfig(),
		fittedModels:           make(map[string]*autoarima.Result),
		meanSampleTimeDistance: make(map[string]time.Duration),
		forecastedValues:       make(map[string][]models.Metric),
	}
)

func (p *SarimaNuePlugin) SetEnvironment(debugMode bool) {
	// Create buffered logger to capture initialization warnings
	// In debug mode, logs go directly to output; in plugin mode, they're buffered
	p.bufferedLogger = plugin_shared.NewBufferedLogger(p.logger, debugMode)

	// Use buffered logger for LoadEnv to prevent warnings during RPC handshake
	configuration.LoadEnvWithBufferedLogger(p.bufferedLogger, "FAKE_SARIMA_number_ue")
	p.logger.SetLevel(hclog.Level(configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug))))

	// Load analytics configuration from environment or use defaults
	p.config = &plugin_shared.AnalyticsConfig{
		WindowSize:          configuration.GetEnvInt(plugin_shared.EnvAnalyticsWindowSize, plugin_shared.DefaultWindowSize),
		SamplesRefreshModel: configuration.GetEnvInt(plugin_shared.EnvAnalyticsSamplesRefreshModel, plugin_shared.DefaultSamplesRefreshModel),
		MinimumSamples:      configuration.GetEnvInt(plugin_shared.EnvAnalyticsMinimumSamples, plugin_shared.DefaultMinimumSamples),
	}

	// Skip cookie setup in debug mode
	if !debugMode {
		cookieName := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyName)
		cookieValue := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyValue)
		if cookieName == nil || cookieValue == nil {
			p.logger.Error("Missing COOKIE name and value variables for RPC")
			os.Exit(1)
		}
		SarimaNueHandShakeConfigAnalytics.MagicCookieKey = *cookieName
		SarimaNueHandShakeConfigAnalytics.MagicCookieValue = *cookieValue
	}

	p.newSamples = make(map[string]int)

	// After initialization, switch buffered logger to normal logging mode
	if p.bufferedLogger != nil {
		p.bufferedLogger.StartNormalLogging()
	}
}

// GetName returns the unique name of the plugin
func (p *SarimaNuePlugin) GetName() string {
	return "SarimaNuePlugin"
}

// GetDescription returns a description of what this plugin does.
func (p *SarimaNuePlugin) GetDescription() string {
	return "FAKE SARIMA forecasting algorithm for testing purposes - generates predictions for CPU and memory metrics"
}

// GetProducedMetrics returns the list of metric names this plugin produces.
func (p *SarimaNuePlugin) GetProducedMetrics() []string {
	producedMetrics := make([]string, 0, len(SarimaNueSubscribedMetrics))
	for _, metricName := range SarimaNueSubscribedMetrics {
		producedMetrics = append(producedMetrics, fmt.Sprintf("%s_forecasted_value", metricName))
	}
	return producedMetrics
}

// GetMetricDescriptions returns descriptions for subscribed metrics.
func (p *SarimaNuePlugin) GetMetricDescriptions() map[string]string {
	return map[string]string{
		"NWDAF_cpu_usage_percent":  "CPU utilization percentage (fake test data)",
		"NWDAF_memory_usage_bytes": "Memory usage in bytes (fake test data)",
	}
}

// GetSubscribedMetrics returns the list of metrics this plugin subscribes to.
func (p *SarimaNuePlugin) GetSubscribedMetrics() []string {
	return SarimaNueSubscribedMetrics
}

func (p *SarimaNuePlugin) IsMetricSubscribed(metric models.Metric) bool {
	for _, subscribedMetric := range p.GetSubscribedMetrics() {
		if subscribedMetric == metric.Name {
			return true
		}
	}
	return false
}

// GetMinimumSamples returns the minimum number of samples required.
func (p *SarimaNuePlugin) GetMinimumSamples() int {
	return p.config.MinimumSamples
}

// ProcessMetric accumulates metrics in the buffer and returns true when ready to execute.
func (p *SarimaNuePlugin) ProcessMetric(metric models.Metric) bool {
	p.logger.Debug("Received metric", "name", metric.Name, "value", metric.Value)

	if metric.ReceivedAt.IsZero() {
		p.logger.Error("Metric ReceivedAt is zero. Time of metric is not set", "metric", metric)
		return false
	}

	if !p.IsMetricSubscribed(metric) {
		p.logger.Error("Metric not subscribed to this plugin", "metric", metric)
		return false
	}

	// Add metric to buffer
	p.metricBuffer[metric.Name] = append(p.metricBuffer[metric.Name], metric)
	p.newSamples[metric.Name]++

	// Keep only the last windowSize metrics for each metric name
	if len(p.metricBuffer[metric.Name]) > p.config.WindowSize {
		p.metricBuffer[metric.Name] = p.metricBuffer[metric.Name][len(p.metricBuffer[metric.Name])-p.config.WindowSize:]
	}

	p.logger.Trace("Metric added to buffer", "metricBuffer", p.metricBuffer)

	// Check if the current metric has enough samples
	currentMetricCount := len(p.metricBuffer[metric.Name])

	// Execute if we have enough samples for the current metric
	if currentMetricCount >= p.config.MinimumSamples {
		p.logger.Debug("Buffer has enough samples for this metric, ready to execute", "metric", metric.Name, "count", currentMetricCount)
		p.lastProcessedMetric = metric.Name
		return true
	}

	p.logger.Debug("Not enough samples yet", "metric", metric.Name, "current", currentMetricCount, "minimum", p.config.MinimumSamples)
	return false
}

func (p *SarimaNuePlugin) GetTimeSeries(metricList []models.Metric) (*timeseries.Series, time.Duration) {
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
		SarimaNueAlgorithm.logger.Error("Failed to create time series", "error", err)
		return nil, time.Duration(-1)
	}

	if len(metricList) < 2 {
		return timeSeries, time.Duration(0)
	}

	return timeSeries, cumulativeTimeDistance / time.Duration(len(metricList)-1)
}

// Execute computes forecasted values for the metric that triggered execution
func (p *SarimaNuePlugin) Execute() map[string][]models.Metric {
	p.logger.Debug("Executing SARIMA nue algorithm", "triggered by metric", p.lastProcessedMetric)

	result := make(map[string][]models.Metric)

	if len(p.metricBuffer) == 0 {
		return result
	}

	// Only process the metric that triggered the execution
	if p.lastProcessedMetric == "" {
		p.logger.Warn("Execute called but no metric triggered it")
		return result
	}

	metricName := p.lastProcessedMetric
	metrics, exists := p.metricBuffer[metricName]
	if !exists {
		p.logger.Error("Metric that triggered execution not found in buffer", "metric", metricName)
		return result
	}

	// Process only the triggering metric
	{
		p.logger.Trace("Processing metric", "metric", metricName, "samples", len(metrics))
		if len(metrics) < p.config.MinimumSamples {
			p.logger.Warn("Not enough samples for metric", "metric", metricName, "samples", len(metrics), "minimum", p.config.MinimumSamples)
			p.lastProcessedMetric = ""
			return result
		}

		outputMetricName := fmt.Sprintf("%s_forecasted_value", metricName)

		if p.fittedModels[metricName] == nil || p.newSamples[metricName] >= p.config.SamplesRefreshModel {
			p.logger.Debug("Refreshing model for metric", "metric", metricName)
			timeSeries, meanSampleTimeDistance := p.GetTimeSeries(metrics)
			fittedModel, creationModelError := autoarima.AutoARIMA(timeSeries, nil)
			if creationModelError != nil {
				p.logger.Error("Failed to create ARIMA model", "error", creationModelError, "metric", metricName)
				p.fittedModels[metricName] = nil
				p.lastProcessedMetric = ""
				return result
			}
			p.newSamples[metricName] = 0
			p.fittedModels[metricName] = fittedModel
			p.meanSampleTimeDistance[metricName] = meanSampleTimeDistance
			forecasts, _ := fittedModel.Predict(p.config.SamplesRefreshModel)
			p.forecastedValues[metricName] = make([]models.Metric, 0)
			for index, forecastedValue := range forecasts {
				computedMetric := models.Metric{
					Name:        outputMetricName,
					Description: fmt.Sprintf("Forecasted value of %s (index=%d)", metricName, index),
					Value:       forecastedValue,
					NFid:        metrics[0].NFid,
					NFType:      metrics[0].NFType,
					ReceivedAt:  metrics[len(metrics)-1].ReceivedAt.Add(p.meanSampleTimeDistance[metricName] * time.Duration(index+1)),
				}
				p.forecastedValues[metricName] = append(p.forecastedValues[metricName], computedMetric)
			}
			p.logger.Trace("Forecasted values", "metric", metricName, "forecastedValues", p.forecastedValues[metricName])
			result[outputMetricName] = p.forecastedValues[metricName]
		} else {
			if p.newSamples[metricName] >= len(p.forecastedValues[metricName]) {
				result[outputMetricName] = []models.Metric{}
			} else {
				p.logger.Trace("Forecasted values", "metric", metricName, "forecastedValues", p.forecastedValues[metricName])
				result[outputMetricName] = p.forecastedValues[metricName][p.newSamples[metricName]:]
			}
		}
	}

	// Clear the last processed metric
	p.lastProcessedMetric = ""

	return result
}

// GetRequiredEnvVars returns the list of required environment variables.
func (p *SarimaNuePlugin) GetRequiredEnvVars() []string {
	// No special environment variables required for this dummy plugin
	return []string{}
}

// GetStartupLogs returns buffered logs from plugin initialization.
func (p *SarimaNuePlugin) GetStartupLogs() []plugin_shared.StartupLog {
	if p.bufferedLogger == nil {
		return []plugin_shared.StartupLog{}
	}
	return p.bufferedLogger.GetStartupLogs()
}

// GenerateSeasonalTestData returns a train/test split of synthetic seasonal metrics.
func GenerateSeasonalTestData(seasonality int, mean float64, stdDev float64, numberOfSeasons int, sampleTimeDistance time.Duration) ([]models.Metric, []models.Metric) {
	numberGenerators := make([]func() float64, seasonality)
	for i := 0; i < seasonality; i++ {
		numberGenerators[i] = func() float64 {
			sampleMean := mean + float64(i)*stdDev
			return sampleMean + rand.NormFloat64()*stdDev
		}
	}
	metricSeries := make([]models.Metric, 0, seasonality*(numberOfSeasons+1))
	firstSampleTime := time.Now()
	timemultiplicator := 1
	for i := 0; i < numberOfSeasons+1; i++ {
		for _, generator := range numberGenerators {
			metric := models.Metric{Name: SarimaNueSubscribedMetrics[0], Value: generator(), ReceivedAt: firstSampleTime.Add(sampleTimeDistance * time.Duration(timemultiplicator))}
			timemultiplicator++
			metricSeries = append(metricSeries, metric)
		}
	}
	return metricSeries[:len(metricSeries)-1*seasonality], metricSeries[len(metricSeries)-1*seasonality:]
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

	SarimaNueAlgorithm.SetEnvironment(debugLocally)

	// If run as a standalone program for debugging
	if debugLocally {
		// Test the plugin locally
		SarimaNueAlgorithm.logger.Info("Running in debug mode")

		// Simulate some metrics
		metricSeries, testMetricSeries := GenerateSeasonalTestData(SarimaNueAlgorithm.config.SamplesRefreshModel, 10, 3, 10, time.Second)

		for _, metric := range metricSeries {
			SarimaNueAlgorithm.ProcessMetric(metric)
		}

		forecastMetricsMap := SarimaNueAlgorithm.Execute()

		// Get the forecasted metrics for the first subscribed metric
		forecastedMetricName := fmt.Sprintf("%s_forecasted_value", SarimaNueSubscribedMetrics[0])
		forecastMetrics := forecastMetricsMap[forecastedMetricName]

		sumSquaredError := 0.0
		sumSquaredDeviation := 0.0
		meanForecast := 0.0

		// Calculate mean of forecasts
		for _, forecastMetric := range forecastMetrics {
			meanForecast += forecastMetric.Value
		}
		meanForecast /= float64(len(forecastMetrics))

		// Calculate MSE and Variance
		for index, forecastMetric := range forecastMetrics {
			SarimaNueAlgorithm.logger.Info("Difference", "Forecasted metric", forecastMetric.Value, "Real metric", testMetricSeries[index].Value)

			// MSE calculation
			error := forecastMetric.Value - testMetricSeries[index].Value
			sumSquaredError += error * error

			// Variance calculation
			deviation := forecastMetric.Value - meanForecast
			sumSquaredDeviation += deviation * deviation
		}

		mse := sumSquaredError / float64(len(forecastMetrics))
		variance := sumSquaredDeviation / float64(len(forecastMetrics))

		SarimaNueAlgorithm.logger.Info("Statistics", "MSE", mse, "Variance", variance)

		// Checks that everytime new sample is given the forecast lenght is decremented till the model is to
		// be refreshed at index len(testMetricSeries)-2
		for index, testMetric := range testMetricSeries {
			SarimaNueAlgorithm.ProcessMetric(testMetric)
			metricMap := SarimaNueAlgorithm.Execute()
			metricList := metricMap[forecastedMetricName]
			if len(metricList) > (len(testMetricSeries)-(index+1)) && index < len(testMetricSeries)-2 {
				SarimaNueAlgorithm.logger.Error("Index out of bounds", "index", index, "metricList", metricList)
			}
		}
	} else {
		// Run as a plugin
		pluginMap := map[string]plugin.Plugin{
			"FAKE_SARIMA_number_ue": &plugin_shared.AnalyticsAlgorithmPlugin{Impl: SarimaNueAlgorithm},
		}
		SarimaNueAlgorithm.logger.Info("Offered analytics plugin", "plugins", pluginMap)

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: SarimaNueHandShakeConfigAnalytics,
			Plugins:         pluginMap,
		})
	}
}
