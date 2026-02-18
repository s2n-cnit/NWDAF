# Plugins

This directory contains the plugin system for the NWDAF project, enabling extensible data collection and analytics through a plugin architecture based on [HashiCorp's go-plugin](https://github.com/hashicorp/go-plugin).

## Overview

The plugin system provides two main types of plugins:

1. **Collector Plugins** (`collectors/`): Extract metrics from various 5G core implementations
2. **Analytics Plugins** (`analytics/`): Process metrics and generate forecasts, predictions, or computed values

Both plugin types communicate with the main application via RPC (Remote Procedure Call), running as separate processes. This architecture provides isolation, stability, and the ability to update plugins independently.

## Directory Structure

```
plugin/
├── README.md                           # This file
├── PLUGIN_DEVELOPMENT_GUIDE.md         # Comprehensive development guide
├── collectors/                         # Collector plugin implementations
│   ├── build/                         # Compiled collector binaries
│   ├── COLLECTOR_PLUGIN_GUIDE.md      # Collector-specific guide
│   ├── FAKE_test_collector.go         # Example/test collector
│   ├── F5GC_amf_collector.go          # Free5GC AMF collector
│   ├── HPE_amf_collector.go           # HPE AMF collector
│   └── templates/                     # Plugin templates
├── analytics/                          # Analytics plugin implementations
│   ├── build/                         # Compiled analytics binaries
│   ├── ANALYTICS_PLUGIN_GUIDE.md      # Analytics-specific guide
│   ├── FAKE_moving_average.go         # Example moving average plugin
│   ├── FAKE_SARIMA_number_ue.go       # Example SARIMA plugin
│   └── HPE_SARIMA_connected_ue.go     # HPE-specific SARIMA plugin
└── plugin_shared/                      # Shared interfaces and utilities
    ├── README.md                       # Interface documentation
    ├── collector_plugin_interface.go   # MetricCollector interface
    ├── analytics_plugin_interface.go   # AnalyticsAlgorithm interface
    ├── 5G_cores_env.go                # Core-specific environment variables
    └── analytics_config.go            # Analytics configuration helpers
```

## Plugin Types and Naming Convention

### Collector Plugins
Collector plugins gather metrics from 5G core network functions. Plugin filenames must follow the pattern:
```
<CORE_TYPE>_<function>_collector.go
```

Examples:
- `HPE_amf_collector.go` - HPE AMF collector
- `F5GC_amf_collector.go` - Free5GC AMF collector
- `FAKE_test_collector.go` - Test collector

### Analytics Plugins
Analytics plugins process metrics and generate computed values, forecasts, or predictions. Plugin filenames must follow the pattern:
```
<CORE_TYPE>_<algorithm>_<metric>.go
```

Examples:
- `HPE_SARIMA_connected_ue.go` - SARIMA forecast for connected UEs
- `FAKE_moving_average.go` - Moving average example

### Core Type Prefixes
- `HPE_` - HPE 5G core
- `F5GC_` - Free5GC core
- `O5GS_` - Open5GS core
- `OAI_` - OpenAirInterface core
- `FAKE_` - Test/example plugins (do not interact with real cores)

## Configuration

Plugins are configured using environment variables. There are three levels of configuration:

### 1. Common Variables (All Plugins)
Defined in `pkg/configuration/env.go`:
- `LOG_LEVEL` - Logging verbosity

### 2. Plugin Type Variables
For RPC communication (defined in `plugin_shared/`):
- `MAGIC_COOKIE_KEY` - RPC handshake key name
- `MAGIC_COOKIE_VALUE` - RPC handshake value

### 3. Core-Specific Variables
Defined in `plugin_shared/5G_cores_env.go`, varies by core type:

**HPE Core:**
- `HPE_CORE_IP` - HPE core IP address
- `HPE_USERNAME` - HPE authentication username
- `HPE_PASSWORD` - HPE authentication password

**Free5GC Core:**
- `FREE5GC_AMF_IP` - AMF IP address
- `FREE5GC_METRIC_PREFIX` - Metric name prefix

See `plugin_shared/5G_cores_env.go` for the complete list of core-specific variables.

## Building Plugins

Plugins are built as standalone executables and placed in the respective `build/` directories:

```bash
# Build a collector plugin
cd plugin/collectors
go build -o build/FAKE_test_collector FAKE_test_collector.go

# Build an analytics plugin
cd plugin/analytics
go build -o build/FAKE_moving_average FAKE_moving_average.go
```

## Getting Started

For detailed information on creating plugins:

- **General Overview**: Read `PLUGIN_DEVELOPMENT_GUIDE.md` for architecture and concepts
- **Collector Plugins**: See `collectors/COLLECTOR_PLUGIN_GUIDE.md` for step-by-step instructions
- **Analytics Plugins**: See `analytics/ANALYTICS_PLUGIN_GUIDE.md` for implementation details
- **Interface Details**: See `plugin_shared/README.md` for interface specifications

## Quick Start Example

The fastest way to understand the plugin system is to examine the example plugins:

1. **Collector Example**: `collectors/FAKE_test_collector.go` - Shows metric collection
2. **Analytics Example**: `analytics/FAKE_moving_average.go` - Shows metric processing

Both can be run in debug mode for local testing:
```bash
go run FAKE_test_collector.go --debug-locally
```

## Plugin Lifecycle

1. **Load**: Main application discovers and loads plugin binaries
2. **Handshake**: RPC connection established using magic cookie
3. **Initialize**: Plugin reads environment variables and configures itself
4. **Execute**:
   - Collectors: `Collect()` called periodically
   - Analytics: Metrics streamed via `ProcessMetric()`, `Execute()` when ready
5. **Shutdown**: Plugin process terminated when no longer needed

## Support and Contribution

When creating new plugins:
- Follow the naming conventions strictly
- Implement all required interface methods
- Declare required environment variables in `GetRequiredEnvVars()`
- Add appropriate logging for debugging
- Test in `--debug-locally` mode before deployment
- Update documentation with core-specific configuration requirements
