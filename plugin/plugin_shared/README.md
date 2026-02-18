# Plugin Shared Resources

This directory contains the core interfaces, shared utilities, and configuration structures used by all plugins in the NWDAF system.

## Overview

The `plugin_shared` package defines the contracts that plugins must implement to integrate with the NWDAF system. It uses the [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin) framework to enable RPC-based communication between the main application and plugin processes.

## Contents

- **`collector_plugin_interface.go`** - Interface and RPC implementation for collector plugins
- **`analytics_plugin_interface.go`** - Interface and RPC implementation for analytics plugins
- **`5G_cores_env.go`** - Core-type-specific environment variable definitions
- **`analytics_config.go`** - Configuration structures for analytics plugins

## Plugin Interfaces

### 1. MetricCollector Interface (Collector Plugins)

Defined in `collector_plugin_interface.go`, this interface must be implemented by all collector plugins.

#### Interface Definition

```go
type MetricCollector interface {
    // Collect gathers metrics and returns a slice of Metric objects.
    Collect() []models.Metric

    // GetRequiredEnvVars returns a slice of environment variables required by the plugin.
    // If any of these environment variables are not set, the plugin will not be able to run.
    GetRequiredEnvVars() []string
}
```

#### Methods

**`Collect() []models.Metric`**
- Called periodically by the main application to gather metrics
- Returns a slice of `models.Metric` objects containing collected data
- Should handle errors gracefully and log issues
- Must be safe to call concurrently

**`GetRequiredEnvVars() []string`**
- Returns a list of environment variable names required by the plugin
- Called during plugin initialization to validate configuration
- Return an empty slice if no special environment variables are needed
- Common variables (like `LOG_LEVEL`) don't need to be included

#### RPC Architecture

The interface is exposed over RPC using three components:

1. **`MetricCollectorRPC`** - Client-side RPC wrapper that the main application uses
2. **`MetricCollectorRPCServer`** - Server-side RPC handler that wraps the plugin implementation
3. **`MetricCollectorPlugin`** - Plugin implementation that connects the interface to go-plugin

This architecture allows the plugin to run as a separate process while the main application communicates with it over RPC.

### 2. AnalyticsAlgorithm Interface (Analytics Plugins)

Defined in `analytics_plugin_interface.go`, this interface must be implemented by all analytics plugins.

#### Interface Definition

```go
type AnalyticsAlgorithm interface {
    // GetSubscribedMetrics returns the list of metric names this plugin is interested in.
    GetSubscribedMetrics() []string

    // GetMinimumSamples returns the minimum number of samples required before the algorithm can run.
    GetMinimumSamples() int

    // ProcessMetric is called when a new metric matching the subscription list is received.
    // Returns true if the algorithm should be executed after this metric is added.
    ProcessMetric(metric models.Metric) bool

    // Execute runs the analytics algorithm on the accumulated data.
    // Returns a map indexed by metric name, with slices of computed metrics to be published.
    Execute() map[string][]models.Metric

    // GetRequiredEnvVars returns the list of environment variables required by the plugin.
    GetRequiredEnvVars() []string

    // GetName returns a unique name for this analytics plugin.
    GetName() string

    // GetDescription returns a human-readable description of what this plugin does.
    GetDescription() string

    // GetProducedMetrics returns the list of metric names this plugin produces.
    GetProducedMetrics() []string

    // GetMetricDescriptions returns descriptions for subscribed metrics.
    GetMetricDescriptions() map[string]string
}
```

#### Methods

**`GetSubscribedMetrics() []string`**
- Returns metric names the plugin wants to process
- Only metrics with matching names will be sent to this plugin
- Supports exact metric name matching

**`GetMinimumSamples() int`**
- Minimum data points needed before `Execute()` can run
- Return 0 if the algorithm can run with any amount of data
- Used to prevent execution with insufficient data

**`ProcessMetric(metric models.Metric) bool`**
- Called for each new metric matching the subscription list
- Plugin should store the metric in its internal buffer
- Return `true` if the algorithm should execute after adding this metric
- Return `false` to continue accumulating data

**`Execute() map[string][]models.Metric`**
- Runs the analytics algorithm on accumulated data
- Returns a map where keys are metric names and values are slices of computed metrics
- Computed metrics are published to Redis for consumption by other services
- Should clear or manage internal buffers as appropriate

**`GetRequiredEnvVars() []string`**
- Same as collector interface - returns required environment variables

**`GetName() string`**
- Returns a unique identifier for the plugin
- Used for logging and management

**`GetDescription() string`**
- Returns human-readable description of the plugin's purpose
- Displayed in management interfaces

**`GetProducedMetrics() []string`**
- Returns names of metrics this plugin generates
- Helps with metric discovery and documentation

**`GetMetricDescriptions() map[string]string`**
- Returns descriptions for metrics the plugin subscribes to
- Optional - return empty map if not applicable

#### RPC Architecture

Similar to collectors, uses:
1. **`AnalyticsAlgorithmRPC`** - Client-side RPC wrapper
2. **`AnalyticsAlgorithmRPCServer`** - Server-side RPC handler
3. **`AnalyticsAlgorithmPlugin`** - Plugin implementation connector

## How RPC Works with go-plugin

### The Magic Cookie Handshake

For security, go-plugin requires a "magic cookie" - a shared secret between the host application and plugins. This prevents arbitrary executables from being loaded as plugins.

```go
HandshakeConfig{
    ProtocolVersion: 1,
    MagicCookieKey: "NWDAF_PLUGIN",
    MagicCookieValue: "secret_value_here",
}
```

Environment variables used:
- `MAGIC_COOKIE_KEY` - The key name for the cookie
- `MAGIC_COOKIE_VALUE` - The secret value

### RPC Communication Flow

#### For Collector Plugins:

```
Main Application                    Plugin Process
      |                                   |
      |------ 1. Start plugin process --->|
      |                                   |
      |<----- 2. RPC handshake -----------|
      |                                   |
      |------ 3. Call Collect() --------->|
      |                                   |
      |                                   | 4. Gather metrics
      |                                   |    from 5G core
      |                                   |
      |<----- 5. Return []Metric ---------|
      |                                   |
      | 6. Store metrics in Redis         |
      |                                   |
```

#### For Analytics Plugins:

```
Main Application                    Plugin Process
      |                                   |
      |------ 1. Start plugin process --->|
      |                                   |
      |<----- 2. RPC handshake -----------|
      |                                   |
      |--- 3. GetSubscribedMetrics() ---->|
      |<-- 4. ["metric_name", ...] -------|
      |                                   |
      | (When new metric arrives)         |
      |                                   |
      |--- 5. ProcessMetric(metric) ----->|
      |                                   | 6. Add to buffer
      |<-- 7. Return true/false ----------|
      |                                   |
      | (If true returned)                |
      |                                   |
      |------ 8. Execute() -------------->|
      |                                   | 9. Run algorithm
      |<-- 10. Return computed metrics ---|
      |                                   |
      | 11. Publish to Redis              |
      |                                   |
```

### Why RPC?

The RPC architecture provides several benefits:

1. **Process Isolation**: Plugins run in separate processes, preventing crashes from affecting the main application
2. **Language Agnostic**: Plugins can be written in different languages (though currently all are Go)
3. **Independent Updates**: Plugins can be updated without recompiling the main application
4. **Security**: Magic cookie prevents unauthorized plugin loading
5. **Resource Management**: Plugin processes can be started/stopped independently

## Environment Variables

### Purpose

Environment variables serve multiple purposes in the plugin system:

1. **Configuration**: Provide runtime configuration without code changes
2. **Credentials**: Supply authentication details for 5G cores
3. **Core-Specific Settings**: Different cores require different connection parameters
4. **Standardization**: Centralized naming prevents conflicts and hardcoding

### Structure

Environment variables are organized by core type in `5G_cores_env.go`:

```go
type CoreType string

const (
    CoreTypeFree5GC CoreType = "F5GC"
    CoreTypeHPE     CoreType = "HPE"
    CoreTypeOpen5GS CoreType = "O5GS"
    CoreTypeOpenAirInterface CoreType = "OAI"
    CoreTypeFake    CoreType = "FAKE"
)
```

Each core type has:
- **Constant Definitions**: Environment variable name constants
- **Required Variables List**: Array of mandatory variables
- **Helper Functions**: Functions to create environment maps

### Example: HPE Core

```go
const (
    EnvHpeCoreIp   = "HPE_CORE_IP"
    EnvHpeUsername = "HPE_USERNAME"
    EnvHpePassword = "HPE_PASSWORD"
)

var HPERequiredEnvVars = []string{
    EnvHpeCoreIp,
    EnvHpeUsername,
    EnvHpePassword,
}
```

### Why Centralize?

1. **No Magic Strings**: Variable names are constants, preventing typos
2. **Documentation**: All variables documented in one place
3. **Validation**: `GetRequiredEnvVars()` can reference these lists
4. **Maintainability**: Easy to add new variables for new core types

## Analytics Configuration

The `analytics_config.go` file provides standardized configuration for analytics plugins:

### Default Constants

```go
const (
    DefaultWindowSize = 100          // Max samples in buffer
    DefaultSamplesRefreshModel = 10  // Samples before model retrain
    DefaultMinimumSamples = 15       // Min samples for first execution
)
```

### AnalyticsConfig Structure

```go
type AnalyticsConfig struct {
    WindowSize          int  // Maximum number of samples to keep
    SamplesRefreshModel int  // Number of samples before retraining model
    MinimumSamples      int  // Minimum samples required for execution
}
```

### Usage

Analytics plugins can use these defaults or override via environment variables:

```go
config := plugin_shared.NewAnalyticsConfig()
// or
config := plugin_shared.NewAnalyticsConfigWithDefaults(200, 20, 30)
```

## Best Practices

### For Interface Implementers

1. **Always validate environment variables** in `GetRequiredEnvVars()`
2. **Handle errors gracefully** - log and return empty results rather than panicking
3. **Use structured logging** with appropriate log levels
4. **Make methods thread-safe** if the plugin may be called concurrently
5. **Document metric formats** and expected values
6. **Clean up resources** properly on shutdown

### For Environment Variables

1. **Use the constants** from `5G_cores_env.go`, never hardcode strings
2. **Add new core types** by extending the CoreType enum
3. **Document all variables** with comments explaining their purpose
4. **Provide defaults** where sensible, but require critical values (like credentials)
5. **Use helper functions** to create environment maps

### For Analytics Plugins

1. **Manage buffer size** to prevent unbounded memory growth
2. **Implement windowing** or pruning strategies for time-series data
3. **Return appropriate minimum samples** based on algorithm requirements
4. **Provide meaningful metric descriptions** for monitoring UIs
5. **Consider performance** - `ProcessMetric()` is called frequently

## Debug Mode

All plugins support a `--debug-locally` flag that bypasses RPC and runs the plugin standalone. This is useful for:

- Testing plugin logic without the main application
- Debugging metric collection or computation
- Validating environment variable configuration
- Development and iteration

Example:
```bash
go run FAKE_test_collector.go --debug-locally
```

In debug mode, the magic cookie handshake is skipped, and the plugin executes its main functionality directly.