// Package plugin_shared contains shared configuration and constants for analytics plugins.
package plugin_shared

// Default configuration constants for SARIMA/ARIMA analytics plugins
const (
	// DefaultWindowSize is the default maximum number of samples to keep in the metric buffer
	DefaultWindowSize = 100

	// DefaultSamplesRefreshModel is the default number of new samples before retraining the model
	DefaultSamplesRefreshModel = 10

	// DefaultMinimumSamples is the default minimum number of samples required before first execution
	DefaultMinimumSamples = 15
)

// Environment variable names for analytics plugin configuration
const (
	// EnvAnalyticsWindowSize allows overriding the window size via environment variable
	EnvAnalyticsWindowSize = "ANALYTICS_WINDOW_SIZE"

	// EnvAnalyticsSamplesRefreshModel allows overriding the model refresh interval via environment variable
	EnvAnalyticsSamplesRefreshModel = "ANALYTICS_SAMPLES_REFRESH_MODEL"

	// EnvAnalyticsMinimumSamples allows overriding the minimum samples via environment variable
	EnvAnalyticsMinimumSamples = "ANALYTICS_MINIMUM_SAMPLES"
)

// AnalyticsConfig holds configuration parameters for analytics plugins
type AnalyticsConfig struct {
	// WindowSize is the maximum number of samples to keep in the metric buffer
	WindowSize int

	// SamplesRefreshModel is the number of new samples before retraining the model
	SamplesRefreshModel int

	// MinimumSamples is the minimum number of samples required before first execution
	MinimumSamples int
}

// NewAnalyticsConfig creates a new AnalyticsConfig with default values
func NewAnalyticsConfig() *AnalyticsConfig {
	return &AnalyticsConfig{
		WindowSize:          DefaultWindowSize,
		SamplesRefreshModel: DefaultSamplesRefreshModel,
		MinimumSamples:      DefaultMinimumSamples,
	}
}

// NewAnalyticsConfigWithDefaults creates a new AnalyticsConfig with custom default values
func NewAnalyticsConfigWithDefaults(windowSize, samplesRefreshModel, minimumSamples int) *AnalyticsConfig {
	return &AnalyticsConfig{
		WindowSize:          windowSize,
		SamplesRefreshModel: samplesRefreshModel,
		MinimumSamples:      minimumSamples,
	}
}
