# NWDAF Plugin Development Guide

This comprehensive guide explains the plugin architecture for the NWDAF system and provides detailed instructions for developing both collector and analytics plugins.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [How Plugins Work](#how-plugins-work)
- [HashiCorp go-plugin Framework](#hashicorp-go-plugin-framework)
- [Plugin Types](#plugin-types)
- [Development Workflow](#development-workflow)
- [Testing and Debugging](#testing-and-debugging)
- [Building and Deployment](#building-and-deployment)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Architecture Overview

The NWDAF plugin system uses a **process-based architecture** where plugins run as independent processes that communicate with the main application via RPC (Remote Procedure Call).

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Main Application                          │
│  ┌──────────────┐      ┌──────────────┐      ┌───────────┐ │
│  │   Collector  │      │  Analytics   │      │   Redis   │ │
│  │   Manager    │      │   Engine     │      │   Client  │ │
│  └──────┬───────┘      └──────┬───────┘      └─────┬─────┘ │
│         │                     │                    │       │
└─────────┼─────────────────────┼────────────────────┼───────┘
          │ RPC                 │ RPC                │
          │                     │                    │
┌─────────▼─────────┐  ┌────────▼────────┐          │
│  Collector Plugin │  │ Analytics Plugin│          │
│  ┌──────────────┐ │  │ ┌──────────────┐│          │
│  │ Collect()    │ │  │ │ Process()    ││          │
│  │ from 5G Core │ │  │ │ Execute()    ││          │
│  └──────────────┘ │  │ └──────────────┘│          │
└───────────────────┘  └─────────────────┘          │
                                                     │
                       ┌─────────────────────────────▼───┐
                       │        Redis Database            │
                       │  ┌────────────┐  ┌────────────┐ │
                       │  │ Raw Metrics│  │  Computed  │ │
                       │  │            │  │  Metrics   │ │
                       │  └────────────┘  └────────────┘ │
                       └──────────────────────────────────┘
```

### Data Flow

1. **Collector Plugins** → Gather metrics from 5G cores → Publish to Redis
2. **Analytics Engine** → Subscribes to metrics from Redis → Routes to analytics plugins
3. **Analytics Plugins** → Process metrics → Generate computed metrics → Publish to Redis

## How Plugins Work

### Process Isolation

Each plugin runs as an independent OS process, providing:

- **Crash Isolation**: Plugin crashes don't affect the main application
- **Resource Management**: Each plugin has its own memory space
- **Security**: Plugins can't directly access main application memory
- **Flexibility**: Plugins can be started, stopped, and reloaded independently

### Communication via RPC

Plugins communicate with the main application through **RPC (Remote Procedure Call)**:

1. Main application spawns plugin executable
2. Plugin starts an RPC server
3. Main application connects as RPC client
4. Method calls are serialized and sent over RPC
5. Results are returned asynchronously

### Interface-Based Design

All plugins implement well-defined interfaces:

- **Collector plugins**: Implement `MetricCollector` interface
- **Analytics plugins**: Implement `AnalyticsAlgorithm` interface

This ensures:
- Consistent behavior across all plugins
- Easy integration with the main application
- Clear contract between plugin and host

## HashiCorp go-plugin Framework

The NWDAF plugin system uses [HashiCorp's go-plugin](https://github.com/hashicorp/go-plugin), a mature plugin framework used by tools like Terraform, Vault, and Packer.

### Why go-plugin?

1. **Battle-tested**: Used in production by millions of users
2. **Cross-language**: Supports plugins in different programming languages
3. **Secure**: Magic cookie handshake prevents unauthorized plugins
4. **Well-documented**: Extensive documentation and examples
5. **Maintained**: Actively developed by HashiCorp

### The Magic Cookie

For security, go-plugin uses a "magic cookie" - a shared secret that must match between the host and plugin:

```go
HandshakeConfig{
    ProtocolVersion: 1,
    MagicCookieKey:   "NWDAF_PLUGIN",
    MagicCookieValue: "secret_shared_value",
}
```

**How it works:**
- The main application sets the magic cookie as environment variables
- The plugin reads these variables during startup
- If the values don't match, the plugin refuses to connect
- This prevents arbitrary executables from being loaded as plugins

**Environment variables:**
- `MAGIC_COOKIE_KEY` - Name of the cookie key
- `MAGIC_COOKIE_VALUE` - Secret value that must match

### RPC Communication

go-plugin handles the RPC layer automatically:

```go
// Plugin side - implements the interface
type MyCollector struct {}

func (c *MyCollector) Collect() []models.Metric {
    // Your implementation
}

// Main app side - calls the interface
metrics := collectorPlugin.Collect()  // Looks like a local call
```

Behind the scenes:
1. Method call is serialized
2. Sent over RPC connection (stdin/stdout)
3. Plugin deserializes and executes
4. Result is serialized and returned
5. Main app deserializes the result

**You don't need to handle RPC manually** - the framework does it for you!

### Plugin Lifecycle

```
┌──────────────┐
│  Discovery   │  Main app finds plugin binary
└──────┬───────┘
       │
┌──────▼───────┐
│   Startup    │  Spawn plugin process
└──────┬───────┘
       │
┌──────▼───────┐
│  Handshake   │  Verify magic cookie
└──────┬───────┘
       │
┌──────▼───────┐
│ Negotiation  │  Agree on protocol version
└──────┬───────┘
       │
┌──────▼───────┐
│   Running    │  RPC calls back and forth
└──────┬───────┘
       │
┌──────▼───────┐
│  Shutdown    │  Clean termination
└──────────────┘
```

## Plugin Types

### Collector Plugins

**Purpose**: Gather metrics from 5G core network functions

**Key characteristics:**
- Pull-based: Main app calls `Collect()` periodically
- Stateless: Each collection is independent
- Core-specific: Different implementations for different 5G cores

**Interface:**
```go
type MetricCollector interface {
    Collect() []models.Metric
    GetRequiredEnvVars() []string
}
```

**Example use cases:**
- Collecting AMF registration metrics from HPE core
- Gathering UPF throughput from Free5GC
- Monitoring SMF session counts

### Analytics Plugins

**Purpose**: Process metrics and generate forecasts, predictions, or computed values

**Key characteristics:**
- Push-based: Main app sends metrics to plugin
- Stateful: Maintains buffers of historical data
- Algorithm-specific: Implements specific analytics algorithms

**Interface:**
```go
type AnalyticsAlgorithm interface {
    GetSubscribedMetrics() []string
    GetMinimumSamples() int
    ProcessMetric(metric models.Metric) bool
    Execute() map[string][]models.Metric
    GetRequiredEnvVars() []string
    GetName() string
    GetDescription() string
    GetProducedMetrics() []string
    GetMetricDescriptions() map[string]string
}
```

**Example use cases:**
- SARIMA forecasting for UE connection predictions
- Moving average smoothing for CPU usage
- Anomaly detection on latency metrics

## Development Workflow

### Step 1: Choose Plugin Type

Determine whether you need a **collector** or **analytics** plugin:

- **Collector**: If you're gathering data from a 5G core
- **Analytics**: If you're processing existing metrics

### Step 2: Set Up Your File

**Naming convention (mandatory):**

Collectors:
```
<CORE_TYPE>_<function>_collector.go
```

Analytics:
```
<CORE_TYPE>_<algorithm>_<metric>.go
```

**Core type prefixes:**
- `HPE_` - HPE 5G core
- `F5GC_` - Free5GC
- `O5GS_` - Open5GS
- `OAI_` - OpenAirInterface
- `FAKE_` - Test/example

### Step 3: Implement the Interface

See the specific guides:
- **Collectors**: `collectors/COLLECTOR_PLUGIN_GUIDE.md`
- **Analytics**: `analytics/ANALYTICS_PLUGIN_GUIDE.md`

### Step 4: Add Environment Variables

If your plugin needs configuration:

1. Add constants to `plugin_shared/5G_cores_env.go`:
```go
const (
    EnvMyCoreSetting = "MY_CORE_SETTING"
)

var MyCoreRequiredEnvVars = []string{
    EnvMyCoreSetting,
}
```

2. Return them from `GetRequiredEnvVars()`:
```go
func (p *MyPlugin) GetRequiredEnvVars() []string {
    return plugin_shared.MyCoreRequiredEnvVars
}
```

### Step 5: Test Locally

Use debug mode to test without the main application:

```bash
cd plugin/collectors  # or plugin/analytics
go run MY_PLUGIN.go --debug-locally
```

### Step 6: Build

Compile the plugin:

```bash
go build -o build/MY_PLUGIN MY_PLUGIN.go
```

### Step 7: Deploy

Place the binary where the main application can find it and configure environment variables.

## Testing and Debugging

### Debug Mode (`--debug-locally`)

All plugins support a special debug mode that bypasses RPC:

```go
func main() {
    debug_locally := false
    args := os.Args[1:]
    for _, argument := range args {
        if argument == "--debug-locally" {
            debug_locally = true
        }
    }

    if debug_locally {
        // Run plugin logic directly
        metrics := MyPlugin.Collect()
        logger.Info("Collected metrics", "count", len(metrics))
    } else {
        // Run as RPC plugin
        plugin.Serve(&plugin.ServeConfig{...})
    }
}
```

**Benefits:**
- No need for main application
- Direct console output
- Easy to iterate and test
- Validates environment configuration

**Usage:**
```bash
export HPE_CORE_IP=192.168.1.100
export HPE_USERNAME=admin
export HPE_PASSWORD=secret
go run HPE_amf_collector.go --debug-locally
```

### Logging

Use structured logging with `hclog`:

```go
logger := hclog.New(&hclog.LoggerOptions{
    Level:      hclog.Debug,
    Output:     os.Stderr,
    JSONFormat: true,
})

logger.Debug("Processing metric", "name", metric.Name, "value", metric.Value)
logger.Info("Collected metrics", "count", len(metrics))
logger.Error("Failed to connect", "error", err)
```

**Log levels:**
- `Trace`: Very detailed debugging
- `Debug`: Detailed information
- `Info`: General information
- `Warn`: Warning conditions
- `Error`: Error conditions

Set via `LOG_LEVEL` environment variable.

### Common Issues

**Issue**: "magic cookie mismatch"
- **Cause**: Cookie environment variables not set or incorrect
- **Fix**: Ensure `MAGIC_COOKIE_KEY` and `MAGIC_COOKIE_VALUE` are set

**Issue**: "required environment variable not set"
- **Cause**: Missing configuration
- **Fix**: Set all variables returned by `GetRequiredEnvVars()`

**Issue**: Plugin crashes silently
- **Cause**: Panic in plugin code
- **Fix**: Add error handling, use debug mode to see panic

## Building and Deployment

### Build Process

**Manual build:**
```bash
cd plugin/collectors
go build -o build/MY_PLUGIN MY_PLUGIN.go
```

**Optimized build** (smaller binary, no debug symbols):
```bash
go build -ldflags="-s -w" -o build/MY_PLUGIN MY_PLUGIN.go
```

**Build all plugins:**
```bash
# Collectors
for f in plugin/collectors/*.go; do
    name=$(basename "$f" .go)
    go build -o "plugin/collectors/build/$name" "$f"
done

# Analytics
for f in plugin/analytics/*.go; do
    name=$(basename "$f" .go)
    go build -o "plugin/analytics/build/$name" "$f"
done
```

### Deployment

1. **Copy binary** to deployment location
2. **Set permissions**: `chmod +x plugin_binary`
3. **Configure environment variables**
4. **Restart main application** to discover new plugin

### Versioning

Consider adding version information:

```go
const (
    PluginVersion = "1.0.0"
)

func (p *MyPlugin) GetName() string {
    return fmt.Sprintf("MyPlugin v%s", PluginVersion)
}
```

## Best Practices

### Code Organization

1. **Separate concerns**: Keep metric collection logic separate from RPC boilerplate
2. **Use constants**: Define metric names and descriptions as constants
3. **Validate inputs**: Check environment variables and metric data
4. **Handle errors gracefully**: Log errors, don't panic in production code

### Performance

1. **Avoid blocking**: Don't make long-running calls in critical paths
2. **Limit buffer sizes**: Prevent unbounded memory growth in analytics plugins
3. **Use efficient algorithms**: O(n) is better than O(n²)
4. **Pool resources**: Reuse HTTP clients, database connections, etc.

### Security

1. **Validate credentials**: Check environment variables are properly set
2. **Sanitize inputs**: Don't trust external data blindly
3. **Use HTTPS**: Always use secure connections to 5G cores
4. **Don't log secrets**: Avoid logging passwords or tokens

### Documentation

1. **Comment interfaces**: Explain what each method does
2. **Document metrics**: Describe what each metric measures
3. **Provide examples**: Show expected metric values
4. **Explain algorithms**: Document the analytics approach

### Testing

1. **Unit tests**: Test individual functions
2. **Integration tests**: Test with mock 5G cores
3. **Debug mode**: Validate end-to-end functionality
4. **Load testing**: Ensure plugin handles high metric volumes

## Troubleshooting

### Plugin Not Loading

**Symptoms**: Main application doesn't see plugin

**Checks:**
1. Is the binary in the correct `build/` directory?
2. Is the binary executable? (`chmod +x`)
3. Does the filename follow naming conventions?
4. Check main application logs for discovery errors

### RPC Errors

**Symptoms**: "connection refused", "broken pipe"

**Checks:**
1. Are magic cookie environment variables set?
2. Do cookie values match between app and plugin?
3. Is the plugin crashing on startup?
4. Check plugin logs for initialization errors

### Metric Issues

**Symptoms**: No metrics collected or metrics missing

**Checks:**
1. Are all required environment variables set?
2. Can the plugin reach the 5G core? (network connectivity)
3. Are credentials correct?
4. Check plugin debug output

### Analytics Not Executing

**Symptoms**: Analytics plugin doesn't produce results

**Checks:**
1. Is `GetSubscribedMetrics()` returning correct metric names?
2. Do metric names match exactly (case-sensitive)?
3. Has `GetMinimumSamples()` threshold been reached?
4. Is `ProcessMetric()` returning true when ready?

### Memory Issues

**Symptoms**: Plugin memory usage grows unbounded

**Checks:**
1. Is buffer size limited in analytics plugins?
2. Are old metrics being pruned?
3. Is there a memory leak in metric processing?
4. Use `pprof` for memory profiling

## Next Steps

- **Create a collector plugin**: See `collectors/COLLECTOR_PLUGIN_GUIDE.md`
- **Create an analytics plugin**: See `analytics/ANALYTICS_PLUGIN_GUIDE.md`
- **Understand interfaces**: See `plugin_shared/README.md`
- **View examples**: Check `FAKE_test_collector.go` and `FAKE_moving_average.go`

## Additional Resources

- [HashiCorp go-plugin documentation](https://github.com/hashicorp/go-plugin)
- [Go RPC package](https://pkg.go.dev/net/rpc)
- [hclog documentation](https://github.com/hashicorp/go-hclog)
- NWDAF models: `pkg/models/metric.go`
