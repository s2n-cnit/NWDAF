# Collector Plugin Development Guide

This guide provides step-by-step instructions for creating collector plugins that gather metrics from 5G core network functions.

## Table of Contents

- [What is a Collector Plugin?](#what-is-a-collector-plugin)
- [When to Create a Collector Plugin](#when-to-create-a-collector-plugin)
- [Prerequisites](#prerequisites)
- [Step-by-Step Tutorial](#step-by-step-tutorial)
- [Complete Minimal Example](#complete-minimal-example)
- [Advanced Topics](#advanced-topics)
- [Testing Your Plugin](#testing-your-plugin)
- [Common Patterns](#common-patterns)

## What is a Collector Plugin?

A collector plugin is a standalone program that:

1. **Connects** to a 5G core network function (AMF, SMF, UPF, etc.)
2. **Gathers** performance metrics, statistics, or operational data
3. **Formats** the data as `models.Metric` objects
4. **Returns** the metrics to the main application
5. **Publishes** metrics to Redis (handled by main application)

Collector plugins run as separate processes and communicate with the main application via RPC.

## When to Create a Collector Plugin

Create a collector plugin when you need to:

- Gather metrics from a new 5G core implementation (e.g., a new vendor's core)
- Collect from a different network function (e.g., add UDM metrics)
- Implement a new collection method (e.g., gRPC instead of REST)
- Support a new metric type or data source

## Prerequisites

Before creating a collector plugin, ensure you have:

1. **Go installed** (version 1.20 or later)
2. **Access to the 5G core** you want to collect from
3. **API documentation** for the 5G core
4. **Authentication credentials** if required
5. **Knowledge of the metric schema** (`pkg/models/metric.go`)

## Step-by-Step Tutorial

### Step 1: Choose a Name

Follow the mandatory naming convention:

```
<CORE_TYPE>_<function>_collector.go
```

**Examples:**
- `HPE_amf_collector.go` - HPE AMF collector
- `F5GC_smf_collector.go` - Free5GC SMF collector
- `OAI_upf_collector.go` - OpenAirInterface UPF collector

**Core type prefixes:**
- `HPE_` - HPE 5G core
- `F5GC_` - Free5GC
- `O5GS_` - Open5GS
- `OAI_` - OpenAirInterface
- `FAKE_` - Test/example (doesn't connect to real cores)

### Step 2: Create the File

Create your file in the `plugin/collectors/` directory:

```bash
cd plugin/collectors
touch MY_CORE_amf_collector.go
```

### Step 3: Set Up Package and Imports

```go
package main

import (
    "os"

    "github.com/hashicorp/go-hclog"
    "github.com/hashicorp/go-plugin"
    "github.com/s2n-cnit/nwdaf/pkg/configuration"
    "github.com/s2n-cnit/nwdaf/pkg/models"
    "github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)
```

### Step 4: Define Your Collector Struct

```go
// MyCoreCollector collects metrics from MyCore 5G implementation
type MyCoreCollector struct {
    logger       hclog.Logger
    coreIP       string
    username     string
    password     string
    metricPrefix string
    // Add any other state you need (HTTP client, etc.)
}
```

### Step 5: Define the Handshake Configuration

```go
var (
    // HandshakeConfig is the handshake configuration for this plugin
    HandShakeConfigMyCore = plugin.HandshakeConfig{
        ProtocolVersion: 1,
    }

    // MyCoreCollectorInstance is the global instance
    MyCoreCollectorInstance = &MyCoreCollector{
        logger: hclog.New(&hclog.LoggerOptions{
            Level:      hclog.Debug,
            Output:     os.Stderr,
            JSONFormat: true,
        }),
    }
)
```

### Step 6: Implement Environment Setup

```go
func (c *MyCoreCollector) SetEnvironment(debugMode bool) {
    // Load environment variables
    configuration.LoadEnv()

    // Set log level from environment
    c.logger.SetLevel(hclog.Level(
        configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug)),
    ))

    // Load core-specific environment variables
    c.coreIP = configuration.GetEnv(plugin_shared.EnvMyCoreCoreIP, "")
    c.username = configuration.GetEnv(plugin_shared.EnvMyCoreUsername, "")
    c.password = configuration.GetEnv(plugin_shared.EnvMyCorePassword, "")
    c.metricPrefix = configuration.GetEnv(configuration.EnvMetricPrefix, "NWDAF_")

    // Skip cookie setup in debug mode
    if !debugMode {
        cookieName := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyName)
        cookieValue := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyValue)

        if cookieName == nil || cookieValue == nil {
            c.logger.Error("Missing COOKIE name and value variables for RPC")
            os.Exit(1)
        }

        HandShakeConfigMyCore.MagicCookieKey = *cookieName
        HandShakeConfigMyCore.MagicCookieValue = *cookieValue
    }
}
```

### Step 7: Implement the MetricCollector Interface

#### Collect Method

```go
// Collect gathers metrics from the 5G core and returns them
func (c *MyCoreCollector) Collect() []models.Metric {
    c.logger.Debug("Starting metric collection from MyCore")

    metrics := []models.Metric{}

    // Example: Collect AMF metrics
    amfMetrics := c.collectAMFMetrics()
    metrics = append(metrics, amfMetrics...)

    // You can collect from multiple sources
    // smfMetrics := c.collectSMFMetrics()
    // metrics = append(metrics, smfMetrics...)

    c.logger.Info("Completed metric collection", "count", len(metrics))
    return metrics
}

// Helper method to collect AMF metrics
func (c *MyCoreCollector) collectAMFMetrics() []models.Metric {
    metrics := []models.Metric{}

    // 1. Connect to the 5G core (HTTP, gRPC, etc.)
    // 2. Fetch the metrics
    // 3. Parse the response
    // 4. Convert to models.Metric format

    // Example metric
    metric := models.Metric{
        Name:        c.metricPrefix + "amf_registered_ue_count",
        Description: "Number of registered UEs in AMF",
        Value:       1234.0,  // The actual value from your core
        NFid:        "amf-001",
        NFType:      "AMF",
        ReceivedAt:  time.Now(),
    }

    metrics = append(metrics, metric)

    return metrics
}
```

#### GetRequiredEnvVars Method

```go
// GetRequiredEnvVars returns the list of environment variables required by this plugin
func (c *MyCoreCollector) GetRequiredEnvVars() []string {
    return plugin_shared.MyCoreRequiredEnvVars
}
```

**Note**: You need to define these in `plugin_shared/5G_cores_env.go`:

```go
// In plugin_shared/5G_cores_env.go
const (
    EnvMyCoreCoreIP   = "MYCORE_CORE_IP"
    EnvMyCoreUsername = "MYCORE_USERNAME"
    EnvMyCorePassword = "MYCORE_PASSWORD"
)

var MyCoreRequiredEnvVars = []string{
    EnvMyCoreCoreIP,
    EnvMyCoreUsername,
    EnvMyCorePassword,
}
```

### Step 8: Implement the Main Function

```go
func main() {
    // Check for debug mode
    debug_locally := false
    args := os.Args[1:]
    for _, argument := range args {
        if argument == "--debug-locally" {
            debug_locally = true
        }
    }

    // Set up environment
    MyCoreCollectorInstance.SetEnvironment(debug_locally)

    // Run in debug mode or as plugin
    if debug_locally {
        // Debug mode: run locally
        metrics := MyCoreCollectorInstance.Collect()
        MyCoreCollectorInstance.logger.Info("Collected metrics",
            "count", len(metrics),
            "metrics", metrics)
    } else {
        // Plugin mode: serve via RPC
        pluginMap := map[string]plugin.Plugin{
            "MY_CORE_amf_collector": &plugin_shared.MetricCollectorPlugin{
                Impl: MyCoreCollectorInstance,
            },
        }

        MyCoreCollectorInstance.logger.Info("Starting plugin", "plugins", pluginMap)

        plugin.Serve(&plugin.ServeConfig{
            HandshakeConfig: HandShakeConfigMyCore,
            Plugins:         pluginMap,
        })
    }
}
```

## Complete Minimal Example

Here's a complete, minimal collector plugin that generates test data:

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

// SimpleCollector is a minimal collector example
type SimpleCollector struct {
    logger       hclog.Logger
    metricPrefix string
}

var (
    HandShakeConfig = plugin.HandshakeConfig{
        ProtocolVersion: 1,
    }

    SimpleCollectorInstance = &SimpleCollector{
        logger: hclog.New(&hclog.LoggerOptions{
            Level:      hclog.Debug,
            Output:     os.Stderr,
            JSONFormat: true,
        }),
    }
)

func (c *SimpleCollector) SetEnvironment(debugMode bool) {
    configuration.LoadEnv()
    c.logger.SetLevel(hclog.Level(
        configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug)),
    ))
    c.metricPrefix = configuration.GetEnv(configuration.EnvMetricPrefix, "NWDAF_")

    if !debugMode {
        cookieName := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyName)
        cookieValue := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyValue)
        if cookieName == nil || cookieValue == nil {
            c.logger.Error("Missing COOKIE variables")
            os.Exit(1)
        }
        HandShakeConfig.MagicCookieKey = *cookieName
        HandShakeConfig.MagicCookieValue = *cookieValue
    }
}

// Collect implements the MetricCollector interface
func (c *SimpleCollector) Collect() []models.Metric {
    c.logger.Debug("Collecting metrics")

    metrics := []models.Metric{
        {
            Name:        c.metricPrefix + "simple_metric",
            Description: "A simple test metric",
            Value:       42.0,
            NFid:        "test-nf-001",
            NFType:      "AMF",
            ReceivedAt:  time.Now(),
        },
    }

    c.logger.Info("Collected metrics", "count", len(metrics))
    return metrics
}

// GetRequiredEnvVars implements the MetricCollector interface
func (c *SimpleCollector) GetRequiredEnvVars() []string {
    return []string{} // No special requirements
}

func main() {
    debug_locally := false
    for _, arg := range os.Args[1:] {
        if arg == "--debug-locally" {
            debug_locally = true
        }
    }

    SimpleCollectorInstance.SetEnvironment(debug_locally)

    if debug_locally {
        metrics := SimpleCollectorInstance.Collect()
        SimpleCollectorInstance.logger.Info("Debug output", "metrics", metrics)
    } else {
        pluginMap := map[string]plugin.Plugin{
            "simple_collector": &plugin_shared.MetricCollectorPlugin{
                Impl: SimpleCollectorInstance,
            },
        }

        plugin.Serve(&plugin.ServeConfig{
            HandshakeConfig: HandShakeConfig,
            Plugins:         pluginMap,
        })
    }
}
```

## Advanced Topics

### Using Configuration Files

Plugins can load additional configuration from YAML files stored in `plugin/collectors/config/`. This is useful for managing complex settings like target lists, endpoint configurations, or algorithm parameters that you don't want to hardcode.

#### Configuration File Location and Naming

Configuration files must follow this naming convention:

```
plugin/collectors/config/<PLUGIN_NAME>.yaml
```

**Examples:**
- `FAKE_test_collector.yaml` for `FAKE_test_collector.go`
- `HPE_amf_collector.yaml` for `HPE_amf_collector.go`
- `F5GC_smf_collector.yaml` for `F5GC_smf_collector.go`

The configuration file name **must match** the plugin name exactly.

#### Loading Configuration

Add a `loadConfig()` method to your collector:

```go
import (
    "fmt"
    "os"
    "path/filepath"
    "gopkg.in/yaml.v3"
)

// Define your configuration structure
type MyConfig struct {
    Targets []plugin_shared.Target `yaml:"targets"`
    // Add other configuration fields as needed
}

func (collector *MyCollector) loadConfig() error {
    // Config file path: plugin/collectors/config/MY_PLUGIN.yaml
    configPath := filepath.Join("plugin", "collectors", "config", "MY_PLUGIN.yaml")
    
    collector.logger.Debug("Loading config", "path", configPath)
    
    // Read the file
    data, err := os.ReadFile(configPath)
    if err != nil {
        return fmt.Errorf("failed to read config file: %w", err)
    }
    
    // Parse YAML
    var config MyConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return fmt.Errorf("failed to parse YAML: %w", err)
    }
    
    // Store the configuration
    collector.targets = config.Targets
    return nil
}

func (collector *MyCollector) SetEnvironment(debugMode bool) {
    // ... existing environment setup ...
    
    // Load configuration file
    if err := collector.loadConfig(); err != nil {
        collector.logger.Warn("Failed to load config", "error", err)
        // Decide if you want to continue with defaults or exit
    }
}
```

#### Example: Managing Target Lists

The `plugin_shared.Target` struct is available for managing HTTP/HTTPS endpoints:

```go
type MyCollector struct {
    logger  hclog.Logger
    targets []plugin_shared.Target
}
```

**Configuration file** (`plugin/collectors/config/MY_PLUGIN.yaml`):

```yaml
# Configuration for MY_PLUGIN
targets:
  # Primary AMF instance
  - name: "AMF-Primary"
    url: "https://amf-1.5gcore.local"
    port: 443
    path: "/api/v1/metrics"
    description: "Primary AMF instance"
    enabled: true

  # Secondary AMF instance
  - name: "AMF-Secondary"
    url: "https://amf-2.5gcore.local"
    port: 443
    path: "/api/v1/metrics"
    description: "Secondary AMF instance for redundancy"
    enabled: true

  # Disabled instance (for maintenance)
  - name: "AMF-Test"
    url: "https://amf-test.5gcore.local"
    port: 8443
    path: "/api/v1/metrics"
    description: "Test AMF instance"
    enabled: false
```

**Using targets in your collector:**

```go
func (collector *MyCollector) Collect() []models.Metric {
    metrics := []models.Metric{}
    
    // Iterate only over enabled targets
    enabledTargets := plugin_shared.GetEnabledTargets(collector.targets)
    
    for _, target := range enabledTargets {
        collector.logger.Debug("Collecting from target", 
            "name", target.Name, 
            "url", target.GetFullURL())
        
        targetMetrics := collector.collectFromTarget(target)
        metrics = append(metrics, targetMetrics...)
    }
    
    return metrics
}

func (collector *MyCollector) collectFromTarget(target plugin_shared.Target) []models.Metric {
    // Make HTTP request to target.GetFullURL()
    // Parse response and return metrics
}
```

**Target helper functions:**

```go
// Get only enabled targets
enabled := plugin_shared.GetEnabledTargets(collector.targets)

// Find specific target by name
target := plugin_shared.GetTargetByName(collector.targets, "AMF-Primary")
if target != nil {
    fullURL := target.GetFullURL()  // Returns complete URL with port and path
}
```

#### Example: Custom Configuration Structure

For more complex configurations:

```yaml
# plugin/collectors/config/MY_PLUGIN.yaml
collection:
  interval_seconds: 30
  timeout_seconds: 10
  retry_attempts: 3

endpoints:
  amf:
    base_url: "https://amf.5gcore.local"
    api_version: "v1"
    auth_required: true
  
  smf:
    base_url: "https://smf.5gcore.local"
    api_version: "v2"
    auth_required: true

metrics:
  include:
    - "cpu_usage"
    - "memory_usage"
    - "active_sessions"
  exclude:
    - "debug_counters"
```

**Corresponding Go structures:**

```go
type CollectorConfig struct {
    Collection CollectionSettings `yaml:"collection"`
    Endpoints  EndpointConfig     `yaml:"endpoints"`
    Metrics    MetricFilter       `yaml:"metrics"`
}

type CollectionSettings struct {
    IntervalSeconds int `yaml:"interval_seconds"`
    TimeoutSeconds  int `yaml:"timeout_seconds"`
    RetryAttempts   int `yaml:"retry_attempts"`
}

type EndpointConfig struct {
    AMF EndpointInfo `yaml:"amf"`
    SMF EndpointInfo `yaml:"smf"`
}

type EndpointInfo struct {
    BaseURL      string `yaml:"base_url"`
    APIVersion   string `yaml:"api_version"`
    AuthRequired bool   `yaml:"auth_required"`
}

type MetricFilter struct {
    Include []string `yaml:"include"`
    Exclude []string `yaml:"exclude"`
}
```

#### Error Handling

Decide how to handle missing or invalid configuration:

```go
func (collector *MyCollector) SetEnvironment(debugMode bool) {
    // ... other setup ...
    
    if err := collector.loadConfig(); err != nil {
        if errors.Is(err, os.ErrNotExist) {
            // Config file doesn't exist - use defaults
            collector.logger.Info("No config file found, using defaults")
            collector.useDefaultConfig()
        } else {
            // Other error - log and potentially exit
            collector.logger.Error("Failed to load config", "error", err)
            os.Exit(1)
        }
    }
}
```

#### Benefits of Configuration Files

1. **Separation of concerns**: Configuration separate from code
2. **Easy updates**: Change targets without recompiling
3. **Environment-specific**: Different configs for dev/test/prod
4. **Version control**: Track configuration changes
5. **Validation**: YAML parsing catches syntax errors

### HTTP Client for REST APIs

Many 5G cores expose REST APIs. Here's how to use an HTTP client:

```go
import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type MyCoreCollector struct {
    logger     hclog.Logger
    httpClient *http.Client
    baseURL    string
}

func (c *MyCoreCollector) SetEnvironment(debugMode bool) {
    // ... other setup ...

    // Create HTTP client
    c.httpClient = &http.Client{
        Timeout: 10 * time.Second,
    }
    c.baseURL = fmt.Sprintf("https://%s/api/v1", c.coreIP)
}

func (c *MyCoreCollector) collectAMFMetrics() []models.Metric {
    // Make HTTP request
    req, err := http.NewRequest("GET", c.baseURL+"/amf/metrics", nil)
    if err != nil {
        c.logger.Error("Failed to create request", "error", err)
        return []models.Metric{}
    }

    // Add authentication
    req.SetBasicAuth(c.username, c.password)

    // Send request
    resp, err := c.httpClient.Do(req)
    if err != nil {
        c.logger.Error("Failed to fetch metrics", "error", err)
        return []models.Metric{}
    }
    defer resp.Body.Close()

    // Parse response
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        c.logger.Error("Failed to read response", "error", err)
        return []models.Metric{}
    }

    // Unmarshal JSON (adjust to your API's format)
    var apiResponse struct {
        RegisteredUEs int     `json:"registered_ues"`
        CPUUsage      float64 `json:"cpu_usage"`
    }

    if err := json.Unmarshal(body, &apiResponse); err != nil {
        c.logger.Error("Failed to parse JSON", "error", err)
        return []models.Metric{}
    }

    // Convert to metrics
    metrics := []models.Metric{
        {
            Name:        c.metricPrefix + "amf_registered_ues",
            Description: "Number of registered UEs",
            Value:       float64(apiResponse.RegisteredUEs),
            NFid:        "amf-001",
            NFType:      "AMF",
            ReceivedAt:  time.Now(),
        },
        {
            Name:        c.metricPrefix + "amf_cpu_usage_percent",
            Description: "AMF CPU usage percentage",
            Value:       apiResponse.CPUUsage,
            NFid:        "amf-001",
            NFType:      "AMF",
            ReceivedAt:  time.Now(),
        },
    }

    return metrics
}
```

### Error Handling

Always handle errors gracefully and log them:

```go
func (c *MyCoreCollector) Collect() []models.Metric {
    metrics := []models.Metric{}

    // Try to collect from multiple sources
    amfMetrics, err := c.tryCollectAMF()
    if err != nil {
        c.logger.Error("AMF collection failed", "error", err)
        // Continue with other collections
    } else {
        metrics = append(metrics, amfMetrics...)
    }

    smfMetrics, err := c.tryCollectSMF()
    if err != nil {
        c.logger.Error("SMF collection failed", "error", err)
    } else {
        metrics = append(metrics, smfMetrics...)
    }

    return metrics
}

func (c *MyCoreCollector) tryCollectAMF() ([]models.Metric, error) {
    // Your collection logic
    // Return error instead of panicking
    return nil, fmt.Errorf("not implemented")
}
```

### Multiple Network Functions

Collect from multiple NFs in a single plugin:

```go
func (c *MyCoreCollector) Collect() []models.Metric {
    metrics := []models.Metric{}

    // Collect from AMF
    metrics = append(metrics, c.collectAMFMetrics()...)

    // Collect from SMF
    metrics = append(metrics, c.collectSMFMetrics()...)

    // Collect from UPF
    metrics = append(metrics, c.collectUPFMetrics()...)

    return metrics
}
```

### Caching and Optimization

For performance, consider caching:

```go
type MyCoreCollector struct {
    logger        hclog.Logger
    lastCollected time.Time
    cachedMetrics []models.Metric
    cacheDuration time.Duration
}

func (c *MyCoreCollector) Collect() []models.Metric {
    // Use cache if still valid
    if time.Since(c.lastCollected) < c.cacheDuration {
        c.logger.Debug("Returning cached metrics")
        return c.cachedMetrics
    }

    // Collect fresh metrics
    metrics := c.collectFreshMetrics()

    // Update cache
    c.cachedMetrics = metrics
    c.lastCollected = time.Now()

    return metrics
}
```

## Testing Your Plugin

### 1. Test in Debug Mode

```bash
# Set required environment variables
export MYCORE_CORE_IP=192.168.1.100
export MYCORE_USERNAME=admin
export MYCORE_PASSWORD=secret
export LOG_LEVEL=0  # Debug level

# Run in debug mode
go run MY_CORE_amf_collector.go --debug-locally
```

**Expected output:**
```json
{"@level":"info","@message":"Collected metrics","@timestamp":"...","count":5,"metrics":[...]}
```

### 2. Build the Plugin

```bash
go build -o build/MY_CORE_amf_collector MY_CORE_amf_collector.go
```

### 3. Test with Main Application

Configure the main application to load your plugin and verify it's discovered and called.

## Common Patterns

### Pattern 1: Multiple Metrics from One API Call

```go
func (c *MyCoreCollector) collectFromEndpoint() []models.Metric {
    data := c.fetchData()  // One API call

    metrics := []models.Metric{
        {Name: "metric_1", Value: data.Field1, ...},
        {Name: "metric_2", Value: data.Field2, ...},
        {Name: "metric_3", Value: data.Field3, ...},
    }

    return metrics
}
```

### Pattern 2: Per-Instance Metrics

```go
func (c *MyCoreCollector) collectAMFMetrics() []models.Metric {
    metrics := []models.Metric{}

    // Get list of AMF instances
    instances := c.getAMFInstances()

    // Collect metrics for each instance
    for _, instance := range instances {
        metric := models.Metric{
            Name:   c.metricPrefix + "amf_cpu_usage",
            Value:  instance.CPUUsage,
            NFid:   instance.ID,     // Different for each instance
            NFType: "AMF",
        }
        metrics = append(metrics, metric)
    }

    return metrics
}
```

### Pattern 3: Aggregated Metrics

```go
func (c *MyCoreCollector) collectTotalSessions() models.Metric {
    total := 0.0

    instances := c.getSMFInstances()
    for _, instance := range instances {
        total += instance.ActiveSessions
    }

    return models.Metric{
        Name:        c.metricPrefix + "total_active_sessions",
        Description: "Total active sessions across all SMF instances",
        Value:       total,
        NFid:        "smf-cluster",
        NFType:      "SMF",
    }
}
```

## Next Steps

- Review existing collectors: `FAKE_test_collector.go`, `HPE_amf_collector.go`
- Read the interface documentation: `../plugin_shared/README.md`
- Understand the overall architecture: `../PLUGIN_DEVELOPMENT_GUIDE.md`
- Check the models: `../../pkg/models/metric.go`

## Tips

1. **Start simple**: Begin with a FAKE collector to understand the structure
2. **Test incrementally**: Test after each major change
3. **Use debug mode extensively**: It's faster than full RPC testing
4. **Log everything**: You can reduce verbosity later
5. **Handle errors**: Don't let one failed metric stop all collection
6. **Follow naming conventions**: Makes discovery and management easier
