# Analytics Plugin Development Guide

This guide provides step-by-step instructions for creating analytics plugins that process metrics and generate forecasts, predictions, or computed values.

## Table of Contents

- [What is an Analytics Plugin?](#what-is-an-analytics-plugin)
- [When to Create an Analytics Plugin](#when-to-create-an-analytics-plugin)
- [Prerequisites](#prerequisites)
- [How Analytics Plugins Work](#how-analytics-plugins-work)
- [Step-by-Step Tutorial](#step-by-step-tutorial)
- [Complete Minimal Example](#complete-minimal-example)
- [Advanced Topics](#advanced-topics)
- [Testing Your Plugin](#testing-your-plugin)
- [Algorithm Examples](#algorithm-examples)

## What is an Analytics Plugin?

An analytics plugin is a standalone program that:

1. **Subscribes** to specific metrics from Redis
2. **Accumulates** metric data in a time-series buffer
3. **Processes** the data using algorithms (forecasting, smoothing, detection, etc.)
4. **Generates** computed metrics (predictions, anomalies, trends)
5. **Returns** computed metrics to be published to Redis

Analytics plugins are **stateful** - they maintain historical data and run algorithms when enough data is available.

## When to Create an Analytics Plugin

Create an analytics plugin when you need to:

- Forecast future metric values (SARIMA, ARIMA, Prophet, etc.)
- Detect anomalies or outliers in metrics
- Compute rolling statistics (moving average, standard deviation)
- Generate alerts based on metric patterns
- Perform trend analysis or correlation
- Apply machine learning models to metrics

## Prerequisites

Before creating an analytics plugin, ensure you have:

1. **Go installed** (version 1.20 or later)
2. **Understanding of the algorithm** you want to implement
3. **Knowledge of time-series analysis** (for forecasting plugins)
4. **Familiarity with the metric schema** (`pkg/models/metric.go`)
5. **Understanding of the AnalyticsAlgorithm interface**

## How Analytics Plugins Work

### Data Flow

```
┌─────────────────┐
│  Redis Stream   │ Metrics published by collectors
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Analytics Engine│ Main application
└────────┬────────┘
         │
         │ 1. GetSubscribedMetrics() → ["metric_name"]
         ├────────────────────────────────────────────┐
         │                                            │
         │ 2. ProcessMetric(metric) → true/false      │
         ├────────────────────────────────────────────┤
         │         (stores in buffer)                 │
         │                                            │
         │ 3. If true: Execute() → computed metrics   │
         ├────────────────────────────────────────────┤
         │         (runs algorithm)                   │
         │                                            │
         ▼                                            ▼
┌─────────────────────────────────────────────────────┐
│            Analytics Plugin Process                  │
│  ┌────────────┐  ┌──────────┐  ┌───────────────┐  │
│  │   Buffer   │  │Algorithm │  │ Computed      │  │
│  │ [m1, m2,...]│→│ Process  │→│ Metrics       │  │
│  └────────────┘  └──────────┘  └───────────────┘  │
└─────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────┐
│  Redis Stream   │ Computed metrics published
└─────────────────┘
```

### Key Concepts

**Subscription**: Plugin declares which metrics it wants to receive
**Buffering**: Plugin stores historical metric values
**Triggering**: Plugin decides when to run the algorithm
**Execution**: Plugin runs algorithm and generates output
**Publishing**: Main app publishes computed metrics to Redis

## Step-by-Step Tutorial

### Step 1: Choose a Name

Follow the mandatory naming convention:

```
<CORE_TYPE>_<algorithm>_<metric>.go
```

**Examples:**
- `HPE_SARIMA_connected_ue.go` - SARIMA forecast for connected UEs
- `F5GC_moving_average_cpu.go` - Moving average for CPU metrics
- `FAKE_anomaly_detector.go` - Test anomaly detection

**Core type prefixes:**
- `HPE_`, `F5GC_`, `O5GS_`, `OAI_` - For core-specific algorithms
- `FAKE_` - For test/example plugins

### Step 2: Create the File

```bash
cd plugin/analytics
touch MY_algorithm_plugin.go
```

### Step 3: Set Up Package and Imports

```go
package main

import (
    "os"
    "time"

    "github.com/hashicorp/go-hclog"
    "github.com/hashicorp/go-plugin"
    "github.com/s2n-cnit/nwdaf/pkg/configuration"
    "github.com/s2n-cnit/nwdaf/pkg/models"
    "github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)
```

### Step 4: Define Your Plugin Struct

```go
// MyAnalyticsPlugin implements a custom analytics algorithm
type MyAnalyticsPlugin struct {
    logger         hclog.Logger
    metricBuffer   []models.Metric  // Historical data
    windowSize     int               // Max samples to keep
    minimumSamples int               // Min samples before running

    // Add algorithm-specific fields
    // e.g., model parameters, state, etc.
}
```

### Step 5: Define Handshake and Instance

```go
var (
    HandShakeConfigAnalytics = plugin.HandshakeConfig{
        ProtocolVersion: 1,
    }

    MyAnalyticsInstance = &MyAnalyticsPlugin{
        logger: hclog.New(&hclog.LoggerOptions{
            Level:      hclog.Debug,
            Output:     os.Stderr,
            JSONFormat: true,
        }),
        metricBuffer:   make([]models.Metric, 0),
        windowSize:     100,
        minimumSamples: 10,
    }
)
```

### Step 6: Implement Environment Setup

```go
func (p *MyAnalyticsPlugin) SetEnvironment(debugMode bool) {
    configuration.LoadEnv()
    p.logger.SetLevel(hclog.Level(
        configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug)),
    ))

    // Load configuration from environment (optional)
    p.windowSize = configuration.GetEnvInt(
        plugin_shared.EnvAnalyticsWindowSize,
        plugin_shared.DefaultWindowSize,
    )
    p.minimumSamples = configuration.GetEnvInt(
        plugin_shared.EnvAnalyticsMinimumSamples,
        plugin_shared.DefaultMinimumSamples,
    )

    if !debugMode {
        cookieName := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyName)
        cookieValue := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyValue)
        if cookieName == nil || cookieValue == nil {
            p.logger.Error("Missing COOKIE variables")
            os.Exit(1)
        }
        HandShakeConfigAnalytics.MagicCookieKey = *cookieName
        HandShakeConfigAnalytics.MagicCookieValue = *cookieValue
    }
}
```

### Step 7: Implement the AnalyticsAlgorithm Interface

#### GetName

```go
func (p *MyAnalyticsPlugin) GetName() string {
    return "MyAnalyticsPlugin"
}
```

#### GetDescription

```go
func (p *MyAnalyticsPlugin) GetDescription() string {
    return "Performs custom analytics on metric data"
}
```

#### GetSubscribedMetrics

```go
// GetSubscribedMetrics returns the list of metrics this plugin wants to process
func (p *MyAnalyticsPlugin) GetSubscribedMetrics() []string {
    return []string{
        "NWDAF_cpu_usage_percent",
        "NWDAF_memory_usage_bytes",
    }
}
```

#### GetMinimumSamples

```go
// GetMinimumSamples returns how many samples needed before first execution
func (p *MyAnalyticsPlugin) GetMinimumSamples() int {
    return p.minimumSamples
}
```

#### ProcessMetric

```go
// ProcessMetric is called when a new metric arrives
// Returns true if the algorithm should execute
func (p *MyAnalyticsPlugin) ProcessMetric(metric models.Metric) bool {
    p.logger.Debug("Processing metric", "name", metric.Name, "value", metric.Value)

    // Add to buffer
    p.metricBuffer = append(p.metricBuffer, metric)

    // Maintain buffer size (keep only most recent windowSize metrics)
    if len(p.metricBuffer) > p.windowSize {
        p.metricBuffer = p.metricBuffer[len(p.metricBuffer)-p.windowSize:]
    }

    // Execute if we have enough samples
    // You can use other triggers: time-based, value-based, etc.
    if len(p.metricBuffer) >= p.minimumSamples {
        p.logger.Debug("Ready to execute", "bufferSize", len(p.metricBuffer))
        return true
    }

    p.logger.Debug("Not enough samples", "current", len(p.metricBuffer))
    return false
}
```

#### Execute

```go
// Execute runs the analytics algorithm and returns computed metrics
func (p *MyAnalyticsPlugin) Execute() map[string][]models.Metric {
    p.logger.Debug("Executing analytics algorithm", "bufferSize", len(p.metricBuffer))

    result := make(map[string][]models.Metric)

    if len(p.metricBuffer) == 0 {
        return result
    }

    // Group metrics by name
    metricsByName := p.groupMetricsByName()

    // Run algorithm on each metric type
    for metricName, metrics := range metricsByName {
        computedMetrics := p.runAlgorithm(metricName, metrics)
        if len(computedMetrics) > 0 {
            // Key is the output metric name
            outputName := metricName + "_computed"
            result[outputName] = computedMetrics
        }
    }

    return result
}

// Helper to group metrics by name
func (p *MyAnalyticsPlugin) groupMetricsByName() map[string][]models.Metric {
    grouped := make(map[string][]models.Metric)
    for _, metric := range p.metricBuffer {
        grouped[metric.Name] = append(grouped[metric.Name], metric)
    }
    return grouped
}

// Your algorithm implementation
func (p *MyAnalyticsPlugin) runAlgorithm(metricName string, metrics []models.Metric) []models.Metric {
    // Implement your algorithm here
    // Example: simple average
    sum := 0.0
    for _, m := range metrics {
        sum += m.Value
    }
    average := sum / float64(len(metrics))

    computed := models.Metric{
        Name:        metricName + "_average",
        Description: "Average of " + metricName,
        Value:       average,
        NFid:        metrics[0].NFid,
        NFType:      metrics[0].NFType,
        ReceivedAt:  time.Now(),
    }

    return []models.Metric{computed}
}
```

#### GetProducedMetrics

```go
func (p *MyAnalyticsPlugin) GetProducedMetrics() []string {
    return []string{
        "NWDAF_cpu_usage_percent_average",
        "NWDAF_memory_usage_bytes_average",
    }
}
```

#### GetMetricDescriptions

```go
func (p *MyAnalyticsPlugin) GetMetricDescriptions() map[string]string {
    return map[string]string{
        "NWDAF_cpu_usage_percent":   "CPU usage percentage",
        "NWDAF_memory_usage_bytes":  "Memory usage in bytes",
    }
}
```

#### GetRequiredEnvVars

```go
func (p *MyAnalyticsPlugin) GetRequiredEnvVars() []string {
    return []string{} // No special requirements
}
```

### Step 8: Implement Main Function

```go
func main() {
    debug_locally := false
    for _, arg := range os.Args[1:] {
        if arg == "--debug-locally" {
            debug_locally = true
        }
    }

    MyAnalyticsInstance.SetEnvironment(debug_locally)

    if debug_locally {
        // Debug mode: simulate incoming metrics
        p.logger.Info("Running in debug mode")

        testMetrics := []models.Metric{
            {Name: "NWDAF_cpu_usage_percent", Value: 45.0, NFid: "test", NFType: "AMF"},
            {Name: "NWDAF_cpu_usage_percent", Value: 50.0, NFid: "test", NFType: "AMF"},
            {Name: "NWDAF_cpu_usage_percent", Value: 55.0, NFid: "test", NFType: "AMF"},
        }

        for _, metric := range testMetrics {
            shouldExecute := MyAnalyticsInstance.ProcessMetric(metric)
            if shouldExecute {
                computed := MyAnalyticsInstance.Execute()
                p.logger.Info("Computed metrics", "result", computed)
            }
        }
    } else {
        // Plugin mode
        pluginMap := map[string]plugin.Plugin{
            "my_analytics": &plugin_shared.AnalyticsAlgorithmPlugin{
                Impl: MyAnalyticsInstance,
            },
        }

        plugin.Serve(&plugin.ServeConfig{
            HandshakeConfig: HandShakeConfigAnalytics,
            Plugins:         pluginMap,
        })
    }
}
```

## Complete Minimal Example

Here's a complete minimal analytics plugin that computes a simple moving average:

```go
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
)

type SimpleMovingAverage struct {
    logger       hclog.Logger
    buffer       []models.Metric
    windowSize   int
    minSamples   int
}

var (
    HandShake = plugin.HandshakeConfig{ProtocolVersion: 1}
    Instance  = &SimpleMovingAverage{
        logger:     hclog.New(&hclog.LoggerOptions{Level: hclog.Debug, Output: os.Stderr, JSONFormat: true}),
        buffer:     make([]models.Metric, 0),
        windowSize: 5,
        minSamples: 3,
    }
)

func (p *SimpleMovingAverage) SetEnvironment(debug bool) {
    configuration.LoadEnv()
    p.logger.SetLevel(hclog.Level(configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug))))
    if !debug {
        name := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyName)
        value := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyValue)
        if name == nil || value == nil {
            p.logger.Error("Missing cookie")
            os.Exit(1)
        }
        HandShake.MagicCookieKey = *name
        HandShake.MagicCookieValue = *value
    }
}

func (p *SimpleMovingAverage) GetName() string                        { return "SimpleMovingAverage" }
func (p *SimpleMovingAverage) GetDescription() string                 { return "Computes moving average" }
func (p *SimpleMovingAverage) GetSubscribedMetrics() []string         { return []string{"NWDAF_cpu_usage_percent"} }
func (p *SimpleMovingAverage) GetMinimumSamples() int                 { return p.minSamples }
func (p *SimpleMovingAverage) GetProducedMetrics() []string           { return []string{"NWDAF_cpu_usage_percent_ma"} }
func (p *SimpleMovingAverage) GetRequiredEnvVars() []string           { return []string{} }
func (p *SimpleMovingAverage) GetMetricDescriptions() map[string]string { return map[string]string{} }

func (p *SimpleMovingAverage) ProcessMetric(m models.Metric) bool {
    p.buffer = append(p.buffer, m)
    if len(p.buffer) > p.windowSize {
        p.buffer = p.buffer[1:]
    }
    return len(p.buffer) >= p.minSamples
}

func (p *SimpleMovingAverage) Execute() map[string][]models.Metric {
    if len(p.buffer) == 0 {
        return make(map[string][]models.Metric)
    }

    sum := 0.0
    for _, m := range p.buffer {
        sum += m.Value
    }
    avg := sum / float64(len(p.buffer))

    result := make(map[string][]models.Metric)
    result["NWDAF_cpu_usage_percent_ma"] = []models.Metric{
        {
            Name:        "NWDAF_cpu_usage_percent_ma",
            Description: fmt.Sprintf("Moving average (window=%d)", len(p.buffer)),
            Value:       avg,
            NFid:        p.buffer[0].NFid,
            NFType:      p.buffer[0].NFType,
            ReceivedAt:  time.Now(),
        },
    }

    return result
}

func main() {
    debug := false
    for _, arg := range os.Args[1:] {
        if arg == "--debug-locally" {
            debug = true
        }
    }

    Instance.SetEnvironment(debug)

    if debug {
        for i := 0; i < 5; i++ {
            m := models.Metric{Name: "NWDAF_cpu_usage_percent", Value: float64(40 + i*5), NFid: "test", NFType: "AMF"}
            if Instance.ProcessMetric(m) {
                Instance.logger.Info("Result", "metrics", Instance.Execute())
            }
        }
    } else {
        plugin.Serve(&plugin.ServeConfig{
            HandshakeConfig: HandShake,
            Plugins: map[string]plugin.Plugin{
                "simple_ma": &plugin_shared.AnalyticsAlgorithmPlugin{Impl: Instance},
            },
        })
    }
}
```

## Advanced Topics

### Time-Series Buffer Management

Efficient buffer management is crucial:

```go
type TimeSeriesBuffer struct {
    metrics    []models.Metric
    maxSize    int
    maxAge     time.Duration
}

func (b *TimeSeriesBuffer) Add(m models.Metric) {
    b.metrics = append(b.metrics, m)
    b.prune()
}

func (b *TimeSeriesBuffer) prune() {
    // Remove old metrics
    cutoff := time.Now().Add(-b.maxAge)
    i := 0
    for i < len(b.metrics) && b.metrics[i].ReceivedAt.Before(cutoff) {
        i++
    }
    b.metrics = b.metrics[i:]

    // Limit size
    if len(b.metrics) > b.maxSize {
        b.metrics = b.metrics[len(b.metrics)-b.maxSize:]
    }
}
```

### Per-Metric Buffers

Track metrics separately:

```go
type MultiMetricPlugin struct {
    logger  hclog.Logger
    buffers map[string][]models.Metric  // Separate buffer per metric
}

func (p *MultiMetricPlugin) ProcessMetric(m models.Metric) bool {
    if p.buffers == nil {
        p.buffers = make(map[string][]models.Metric)
    }

    // Add to metric-specific buffer
    p.buffers[m.Name] = append(p.buffers[m.Name], m)

    // Check if any buffer is ready
    for _, buffer := range p.buffers {
        if len(buffer) >= p.minimumSamples {
            return true
        }
    }
    return false
}
```

### Model Retraining

Periodically retrain your model:

```go
type ForecastPlugin struct {
    logger            hclog.Logger
    buffer            []models.Metric
    model             interface{}  // Your model
    samplesSinceRetrain int
    retrainInterval   int
}

func (p *ForecastPlugin) Execute() map[string][]models.Metric {
    // Retrain if needed
    if p.samplesSinceRetrain >= p.retrainInterval {
        p.logger.Info("Retraining model")
        p.model = p.trainModel(p.buffer)
        p.samplesSinceRetrain = 0
    }

    // Generate forecast
    forecast := p.generateForecast(p.model)
    p.samplesSinceRetrain++

    return forecast
}
```

### Multi-Step Forecasting

Generate multiple future predictions:

```go
func (p *ForecastPlugin) Execute() map[string][]models.Metric {
    result := make(map[string][]models.Metric)

    // Generate forecast for next 5 time steps
    forecasts := []models.Metric{}
    for step := 1; step <= 5; step++ {
        predictedValue := p.forecast(step)

        forecast := models.Metric{
            Name:        fmt.Sprintf("forecast_step_%d", step),
            Description: fmt.Sprintf("Forecast %d steps ahead", step),
            Value:       predictedValue,
            NFid:        p.buffer[0].NFid,
            NFType:      p.buffer[0].NFType,
            ReceivedAt:  time.Now(),
        }
        forecasts = append(forecasts, forecast)
    }

    result["multi_step_forecast"] = forecasts
    return result
}
```

## Testing Your Plugin

### 1. Debug Mode Testing

```bash
# Run in debug mode
go run MY_analytics.go --debug-locally
```

### 2. Unit Testing

Create a test file `MY_analytics_test.go`:

```go
package main

import (
    "testing"
    "github.com/s2n-cnit/nwdaf/pkg/models"
)

func TestProcessMetric(t *testing.T) {
    plugin := &MyAnalyticsPlugin{
        metricBuffer:   make([]models.Metric, 0),
        minimumSamples: 3,
    }

    // Add metrics
    for i := 0; i < 3; i++ {
        m := models.Metric{Name: "test", Value: float64(i)}
        shouldExecute := plugin.ProcessMetric(m)

        if i < 2 && shouldExecute {
            t.Error("Should not execute before minimum samples")
        }
        if i == 2 && !shouldExecute {
            t.Error("Should execute after minimum samples")
        }
    }
}

func TestExecute(t *testing.T) {
    plugin := &MyAnalyticsPlugin{
        metricBuffer: []models.Metric{
            {Name: "test", Value: 10.0},
            {Name: "test", Value: 20.0},
            {Name: "test", Value: 30.0},
        },
    }

    result := plugin.Execute()
    if len(result) == 0 {
        t.Error("Expected computed metrics")
    }
}
```

Run tests:
```bash
go test -v
```

### 3. Build and Deploy

```bash
go build -o build/MY_analytics MY_analytics.go
```

## Algorithm Examples

### Moving Average

```go
func (p *MovingAveragePlugin) Execute() map[string][]models.Metric {
    sum := 0.0
    for _, m := range p.buffer {
        sum += m.Value
    }
    avg := sum / float64(len(p.buffer))

    result := make(map[string][]models.Metric)
    result["metric_ma"] = []models.Metric{
        {Name: "metric_ma", Value: avg, ...},
    }
    return result
}
```

### Exponential Smoothing

```go
type ExpSmoothingPlugin struct {
    alpha          float64  // Smoothing factor
    lastSmoothed   float64
}

func (p *ExpSmoothingPlugin) Execute() map[string][]models.Metric {
    current := p.buffer[len(p.buffer)-1].Value
    smoothed := p.alpha*current + (1-p.alpha)*p.lastSmoothed
    p.lastSmoothed = smoothed

    result := make(map[string][]models.Metric)
    result["metric_smooth"] = []models.Metric{
        {Name: "metric_smooth", Value: smoothed, ...},
    }
    return result
}
```

### Anomaly Detection (Z-Score)

```go
func (p *AnomalyPlugin) Execute() map[string][]models.Metric {
    // Calculate mean
    sum := 0.0
    for _, m := range p.buffer {
        sum += m.Value
    }
    mean := sum / float64(len(p.buffer))

    // Calculate standard deviation
    variance := 0.0
    for _, m := range p.buffer {
        variance += math.Pow(m.Value-mean, 2)
    }
    stddev := math.Sqrt(variance / float64(len(p.buffer)))

    // Check latest value
    latest := p.buffer[len(p.buffer)-1].Value
    zscore := (latest - mean) / stddev

    isAnomaly := 0.0
    if math.Abs(zscore) > 3.0 {  // 3-sigma rule
        isAnomaly = 1.0
    }

    result := make(map[string][]models.Metric)
    result["anomaly_detected"] = []models.Metric{
        {Name: "anomaly_detected", Value: isAnomaly, ...},
        {Name: "zscore", Value: zscore, ...},
    }
    return result
}
```

## Next Steps

- Review existing analytics plugins: `FAKE_moving_average.go`, `HPE_SARIMA_connected_ue.go`
- Read the interface documentation: `../plugin_shared/README.md`
- Understand the overall architecture: `../PLUGIN_DEVELOPMENT_GUIDE.md`
- Check the models: `../../pkg/models/metric.go`

## Tips

1. **Start with simple algorithms**: Moving average, smoothing before complex forecasting
2. **Test buffer management**: Ensure no memory leaks
3. **Validate output**: Check computed metrics are reasonable
4. **Handle edge cases**: Empty buffers, single data points
5. **Log intermediate steps**: Helps debug algorithm issues
6. **Consider performance**: Some algorithms are O(n²) or worse
7. **Document your algorithm**: Explain the math and parameters
