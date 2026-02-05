# Multi-Core NWDAF (Network Data Analytics Function)

A modular and extensible implementation of the 5G Network Data Analytics Function (NWDAF) supporting multiple 5G core network implementations.

## Table of Contents
- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Components](#components)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Plugin Development](#plugin-development)
- [API Reference](#api-reference)
- [Troubleshooting](#troubleshooting)

---

## Overview

The NWDAF is a 5G core network function that collects, analyzes, and provides insights from network data. This implementation features a plugin-based architecture supporting multiple 5G core vendors and custom analytics algorithms.

**Purpose**: This project is designed for research and innovation in 5G network analytics, providing a flexible platform for developing and testing advanced analytics algorithms.

### Key Capabilities
- **Multi-core Support**: Compatible with Free5GC, HPE, Open5GS, and OpenAirInterface (under development)
- **Modular Design**: Plugin system for data collectors and analytics algorithms
- **Real-time Processing**: Redis pub/sub for low-latency metric distribution
- **Time-Series Analytics**: Temporal data accumulation for advanced algorithms (ARIMA, SARIMA, ML)
- **RESTful API**: 3GPP-compliant service-based interfaces
- **NRF Integration**: Optional service registration/deregistration with Network Repository Function
- **Container-Ready**: Docker deployment with Kubernetes support planned

---

## Features

### Core Features
✅ Multi-vendor 5G core network support  
✅ Dynamic plugin loading for collectors and analytics  
✅ Real-time metrics collection via Redis pub/sub  
✅ Temporal data accumulation for time-series analysis  
✅ RESTful API for metrics exposure  
✅ Prometheus integration for monitoring  
✅ Automatic metric expiration (1-hour TTL)  
✅ Reverse proxy for microservice API aggregation  
✅ Material Design web dashboard for real-time visualization  
✅ Plugin metadata API (`/api/models`, `/api/plugins`)  

### Supported Core Networks

> **Note**: Core network integrations are under active development for research purposes. Full implementation and testing are planned for future releases.

- **Free5GC** (F5GC): AMF metrics via subscription API - *In Development*
- **HPE**: Core metrics via proprietary API - *In Development*
- **Open5GS** (O5GS): Planned support - *Future Work*
- **OpenAirInterface** (OAI): Planned support - *Future Work*
- **FAKE**: Test environment with synthetic metrics - *Fully Implemented*

---

## Architecture

### System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                           NWDAF Main                                  │
│  - Orchestrates all microservices                                     │
│  - HTTP reverse proxy (port 8080)                                     │
│  - NRF registration (optional, configurable)                          │
└───────┬──────────────────┬──────────────────┬────────────────────────┘
        │                  │                  │
        ▼                  ▼                  ▼
┌────────────────┐  ┌────────────────┐  ┌────────────────┐
│ Data Collector │  │ Data Archiver  │  │Analytics Engine│
│                │  │                │  │                │
│ ┌────────────┐ │  │ Subscribes to: │  │ ┌────────────┐ │
│ │ Loads      │ │  │ - metrics      │  │ │ Loads      │ │
│ │ Collector  │ │  │ - computed     │  │ │ Analytics  │ │
│ │ Plugins    │ │  │   Metrics      │  │ │ Plugins    │ │
│ └─────┬──────┘ │  │                │  │ └─────┬──────┘ │
│       │        │  │ Archives to:   │  │       │        │
│   [Plugin 1]   │  │ - Prometheus   │  │   [Plugin A]   │
│   [Plugin 2]   │  │ - REST API     │  │   [Plugin B]   │
│   [Plugin N]   │  │                │  │   [Plugin N]   │
│       │        │  │                │  │       │        │
└───────┼────────┘  └────────▲───────┘  └───────┼────────┘
        │                    │                   │
        │ Publishes          │ Subscribes        │ Publishes
        │                    │                   │
        └────────────────────┼───────────────────┘
                             │
                        ┌────▼─────┐
                        │  Redis   │
                        │ Pub/Sub  │
                        └──────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
         [metrics]   [computedMetrics]  [module]
              │              │
              └──────┬───────┘
                     │
          ┌──────────▼──────────┐
          │  Data Archiver      │
          │  (Subscriber)       │
          └─────────────────────┘
                     │
          ┌──────────┴──────────┐
          ▼                     ▼
    [Prometheus]           [REST API]
    port 2112           /api/metrics
```

### Data Flow

1. **Plugin Loading**:
   - Data Collectors dynamically load collector plugins from `plugin/collectors/build/`
   - Analytics Engine dynamically loads analytics plugins from `plugin/analytics/build/`

2. **Collection**:
   - Collector plugins gather metrics from 5G core NFs (AMF, SMF, UPF, etc.)
   - Data Collectors publish raw metrics to Redis `metrics` topic

3. **Analytics**:
   - Analytics Engine subscribes to Redis `metrics` topic
   - Loaded analytics plugins process metrics (accumulation, computation)
   - Analytics Engine publishes computed metrics to Redis `computedMetrics` topic

4. **Archival**:
   - Data Archiver subscribes to both `metrics` and `computedMetrics` topics
   - Registers/updates metrics in Prometheus
   - Maintains metrics with 1-hour expiration

5. **Exposure**:
   - Prometheus: Metrics available at `http://localhost:2112/metrics`
   - REST API: JSON format at `http://localhost:8080/api/metrics` (via NWDAF proxy)

---

## Components

### 1. NWDAF Main (cmd/nwdaf/)

**Purpose**: Orchestrator that starts and manages all microservices.

**Responsibilities**:
- Load configuration from config/config.yaml
- Start Data Archiver microservice
- Start Analytics Engine microservice
- Start Data Collectors (one per slice)
- Register/deregister with NRF (optional, disabled by default for testing)
- HTTP server with reverse proxy to microservices
- Serve web dashboard

> **Note**: NRF registration is currently commented out in the code to facilitate testing. It can be enabled by uncommenting the `RegisterToNRF()` and `DeregisterFromNRF()` calls in `cmd/nwdaf/main.go`.

**Proxied API Endpoints** (port 8080):
- `/api/metrics` → Data Archiver (8081)
- `/api/computed-metrics` → Analytics Engine (8084)
- `/api/models` → Analytics Engine (8084)
- `/api/plugins` → Analytics Engine (8084)
- `/prometheus/metrics` → Prometheus (2112)
- `/` → Web Dashboard

**Config**: File-based (see Configuration section)

---

### 2. Data Collector (cmd/dcollector/)

**Purpose**: Collects metrics from 5G core network functions using plugins.

**Features**:
- Dynamic plugin loading based on core type
- Periodic collection (60-second interval)
- Redis publishing to metrics topic
- Panic recovery (plugins don't crash collector)

**Plugin System**:
- Location: plugin/collectors/build/
- Naming: <CORE_TYPE>_<plugin_name>
- Example: F5GC_amf_collector, FAKE_test_collector

**Environment Variables**:

| Variable | Description | Required |
|----------|-------------|----------|
| REDIS_URI | Redis server address | Yes |
| REDIS_PASSWORD | Redis password | No |
| CORE_TYPE | Core network type (F5GC, HPE, FAKE) | Yes |
| LOG_LEVEL | Logging level (0=Trace, 1=Debug, 2=Info) | No |
| METRIC_PREFIX | Prefix for metric names | No |

**Core-Specific Variables**:
- **Free5GC**: FREE5GC_AMF_IP
- **HPE**: HPE_CORE_IP, HPE_USERNAME, HPE_PASSWORD
- **FAKE**: None required

**How It Works**:
1. Loads plugins matching CORE_TYPE prefix
2. Validates required environment variables
3. Periodically calls Collect() on each plugin
4. Publishes metrics to Redis metrics topic

---

### 3. Data Archiver (cmd/darchiver/)

**Purpose**: Archives metrics to Prometheus and exposes them via REST API.

**Features**:
- Redis subscription to metrics and computedMetrics
- Prometheus metric registration and updates
- Automatic metric expiration (1-hour TTL, cleanup every 10 minutes)
- REST API for JSON metric retrieval
- Gin web framework for HTTP endpoints

**Endpoints**:
- Prometheus: http://localhost:2112/metrics
- REST API: http://127.0.0.1:8081/api/metrics (loopback only)

**Environment Variables**:

| Variable | Description | Default |
|----------|-------------|---------|
| REDIS_URI | Redis server address | Required |
| REDIS_PASSWORD | Redis password | - |
| PROMETHEUS_LOCAL_PORT | Prometheus endpoint port | 2112 |
| DARCHIVER_API_PORT | REST API port | 8081 |
| LOG_LEVEL | Logging level | 1 (Debug) |

**Metric Lifecycle**:
1. Metric received from Redis
2. ReceivedAt timestamp set to current time
3. Registered in Prometheus (or updated if exists)
4. Available via REST API
5. Expired after 1 hour of no updates
6. Unregistered from Prometheus and removed from API

---

### 4. Analytics Engine (cmd/analytics_engine/)

**Purpose**: Runs analytics plugins to compute derived metrics from collected data.

**Features**:
- Dynamic plugin loading from plugin/analytics/build/
- Selective metric subscription per plugin
- Temporal data accumulation (time-series buffering)
- Minimum sample requirements before execution
- Computed metrics published to Redis computedMetrics
- REST API for computed metrics and plugin info

**API Endpoints** (port 8084, accessed via NWDAF proxy):
- `/api/computed-metrics` - All computed metrics
- `/api/computed-metrics/{name}` - Specific metric
- `/api/models` - Plugin metadata
- `/api/plugins` - Plugin metadata (alias)

**Plugin System**:
- Location: plugin/analytics/build/
- Naming: <CORE_TYPE>_<algorithm_name>
- Example: FAKE_moving_average, F5GC_arima_cpu

**Environment Variables**:

| Variable | Description | Required |
|----------|-------------|----------|
| REDIS_URI | Redis server address | Yes |
| REDIS_PASSWORD | Redis password | No |
| CORE_TYPE | Core network type | Yes |
| LOG_LEVEL | Logging level | No |

**How It Works**:
1. Loads analytics plugins matching CORE_TYPE
2. Subscribes to Redis metrics and computedMetrics topics
3. Routes metrics to plugins based on subscription lists
4. Plugins accumulate metrics in internal buffers
5. Plugin signals when ready (e.g., minimum samples reached)
6. Engine calls Execute() to run algorithm
7. Computed metrics published to computedMetrics topic
8. Data Archiver picks up computed metrics

---

### 5. Web Dashboard (web/)

**Purpose**: Real-time Material Design web interface for monitoring NWDAF metrics.

**Features**:
- Material Design UI (Materialize CSS)
- Real-time metric visualization with Chart.js
- Side-by-side display: raw metrics and forecasts
- Configurable polling interval (2-60 seconds)
- Metric filtering and search
- Toggleable auto-refresh
- Fully responsive (desktop/tablet/mobile)
- Works offline (no CDN dependencies)

**Access**: http://localhost:8080/

**Key Components**:
- `index.html` - Main UI structure
- `styles.css` - Material Design styling
- `app.js` - Application logic and Chart.js integration
- `libs/` - Local copies of all dependencies

**Data Sources**:
- `/api/metrics` - Raw metrics from collectors
- `/api/computed-metrics` - Forecasts from analytics plugins

For detailed documentation, see [web/README.md](web/README.md).

---

## Installation

### Prerequisites

- Go 1.20 or higher
- Redis server
- Docker (recommended for deployment)
- (Optional) Prometheus for monitoring

### Build from Source

```bash
# Clone repository
git clone <repository-url>
cd NWDAF

# Set Go binary path (adjust for your installation)
GO_BIN=/home/paolob/go/go1.25.6/bin/go

# Build all components
$GO_BIN build -o cmd/nwdaf/build/nwdaf ./cmd/nwdaf
$GO_BIN build -o cmd/darchiver/build/darchiver ./cmd/darchiver
$GO_BIN build -o cmd/dcollector/build/dcollector ./cmd/dcollector
$GO_BIN build -o cmd/analytics_engine/build/analytics_engine ./cmd/analytics_engine

# Build example plugins
$GO_BIN build -o plugin/collectors/build/FAKE_test_collector ./plugin/collectors/FAKE_test_collector.go
$GO_BIN build -o plugin/analytics/build/FAKE_moving_average ./plugin/analytics/FAKE_moving_average.go
```

### Quick Start with FAKE Core (Testing)

```bash
# 1. Start Redis
redis-server

# 2. Configure NWDAF for FAKE core
cat > config/config_dev.yaml <<EOF
server:
  port: 8080
  bind_ip: "0.0.0.0"
log_level: 1
prometheus_port: 2112
redis:
  uri: "localhost:6379"
  password: ""
nrf_uri: "localhost:29510"
core_type: "FAKE"
slices:
  - id: "test-slice-1"
EOF

# 3. Run NWDAF
./cmd/nwdaf/build/nwdaf
```

NWDAF will start all microservices automatically:
- Data Archiver on port 2112 (Prometheus) and 8081 (API)
- Analytics Engine loading FAKE plugins
- Data Collector with FAKE test collector
- HTTP proxy on port 8080

### Docker Deployment (Recommended)

> **Note**: Docker Compose configuration is recommended for production deployment. Kubernetes support is planned for future releases.

Docker deployment instructions will be provided in future releases. For now, use the build-from-source method above.

---

## Configuration

### Configuration File: config/config.yaml

```yaml
# HTTP server
server:
  port: 8080           # External API port
  bind_ip: "0.0.0.0"   # Bind address

# TLS (optional)
tls:
  cert_file: "certs/server.crt"
  key_file: "certs/server.key"

# Trusted proxies for reverse proxy
trusted_proxies:
  - "192.168.0.0/16"
  - "127.0.0.1/32"

# Logging
log_level: 1  # 0=Trace, 1=Debug, 2=Info, 3=Warn, 4=Error

# Prometheus
prometheus_port: 2112

# Redis
redis:
  uri: "localhost:6379"
  password: ""

# NRF (Network Repository Function)
nrf_uri: "localhost:29510"

# Core type: F5GC, HPE, O5GS, OAI, FAKE
core_type: "F5GC"

# Network slices to monitor
slices:
  - id: "slice-1"
    core_endpoint_ip: "10.0.0.1"
    username: "admin"
    password: "password"
    amf_ips:
      - "10.0.0.10"
    smf_ips:
      - "10.0.0.20"
```

### Environment Variables

Optional runtime overrides:

```bash
# Override data archiver API port
export DARCHIVER_API_PORT=9090

# Override log level
export LOG_LEVEL=2
```

---

## Usage

### Start NWDAF

```bash
# Using default config (config/config.yaml or config/config_dev.yaml)
./cmd/nwdaf/build/nwdaf

# Using custom config
CONFIG_FILE=config/production.yaml ./cmd/nwdaf/build/nwdaf
```

### Access Metrics

**REST API** (via NWDAF proxy):
```bash
# Get all metrics as JSON
curl http://localhost:8080/api/metrics

# Example response
[
  {
    "name": "NWDAF_cpu_usage_percent",
    "description": "CPU utilization percentage",
    "value": 45.8,
    "nfid": "amf-001",
    "nfType": "AMF",
    "receivedAt": "2026-02-03T10:00:00Z"
  },
  {
    "name": "NWDAF_cpu_usage_percent_moving_avg",
    "description": "Moving average of NWDAF_cpu_usage_percent (window=5)",
    "value": 47.2,
    "nfid": "amf-001",
    "nfType": "AMF",
    "receivedAt": "2026-02-03T10:01:00Z"
  }
]
```

**Prometheus** (direct to Data Archiver):
```bash
# Scrape Prometheus metrics
curl http://localhost:2112/metrics

# Example output
# HELP NWDAF_cpu_usage_percent CPU utilization percentage
# TYPE NWDAF_cpu_usage_percent gauge
NWDAF_cpu_usage_percent{app="nwdaf"} 45.8
```

---

## Plugin Development

**Note:** Plugins with the `FAKE_` prefix are used for testing purposes only and do not interact with real 5G cores. These plugins generate synthetic data and are useful for development and testing without requiring actual network functions.

### Data Collector Plugin

Create a new collector plugin to gather metrics from a specific NF or vendor.

**Interface** (plugin/plugin_shared/plugin_interface.go):
```go
type MetricCollector interface {
    Collect() []models.Metric
    GetRequiredEnvVars() []string
}
```

**Example** (plugin/collectors/F5GC_my_collector.go):
```go
package main

import (
    "github.com/s2n-cnit/nwdaf/pkg/models"
    "github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)

type MyCollector struct {
    // Your collector state
}

func (c *MyCollector) Collect() []models.Metric {
    // Fetch data from NF
    // Return metrics
    return []models.Metric{
        {
            Name:        "NWDAF_my_metric",
            Description: "My custom metric",
            Value:       100.0,
            NFid:        "nf-001",
            NFType:      "AMF",
        },
    }
}

func (c *MyCollector) GetRequiredEnvVars() []string {
    return []string{"MY_NF_IP", "MY_NF_PORT"}
}

func main() {
    // Standard plugin setup (see FAKE_test_collector.go)
}
```

**Build and Deploy**:
```bash
go build -o plugin/collectors/build/F5GC_my_collector ./plugin/collectors/F5GC_my_collector.go
```

---

### Analytics Plugin

Create analytics algorithms that process metrics and produce computed results.

**Interface** (plugin/plugin_shared/analytics_plugin_interface.go):
```go
type AnalyticsAlgorithm interface {
    GetSubscribedMetrics() []string
    GetMinimumSamples() int
    ProcessMetric(metric models.Metric) bool
    Execute() []models.Metric
    GetRequiredEnvVars() []string
    GetName() string
}
```

**Example - ARIMA Forecasting**:
```go
package main

import (
    "github.com/s2n-cnit/nwdaf/pkg/models"
    "github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)

type ARIMAPlugin struct {
    buffer []models.Metric
}

func (p *ARIMAPlugin) GetName() string {
    return "ARIMA_CPU_Forecaster"
}

func (p *ARIMAPlugin) GetSubscribedMetrics() []string {
    return []string{"NWDAF_cpu_usage_percent"}
}

func (p *ARIMAPlugin) GetMinimumSamples() int {
    return 50  // ARIMA needs at least 50 samples
}

func (p *ARIMAPlugin) ProcessMetric(metric models.Metric) bool {
    p.buffer = append(p.buffer, metric)

    // Keep only last 100 samples
    if len(p.buffer) > 100 {
        p.buffer = p.buffer[len(p.buffer)-100:]
    }

    // Execute when we have enough samples
    return len(p.buffer) >= 50
}

func (p *ARIMAPlugin) Execute() []models.Metric {
    // Run ARIMA algorithm
    forecast := runARIMA(p.buffer)

    return []models.Metric{
        {
            Name:        "NWDAF_cpu_usage_percent_forecast",
            Description: "ARIMA forecast for CPU usage",
            Value:       forecast,
            NFid:        p.buffer[0].NFid,
            NFType:      p.buffer[0].NFType,
        },
    }
}

func (p *ARIMAPlugin) GetRequiredEnvVars() []string {
    return []string{}
}

func main() {
    // Standard plugin setup (see FAKE_moving_average.go)
}
```

**Build and Deploy**:
```bash
go build -o plugin/analytics/build/F5GC_arima_cpu ./plugin/analytics/F5GC_arima_cpu.go
```

**Plugin Lifecycle**:
1. Analytics Engine loads plugin
2. Plugin subscribes to specific metrics
3. For each incoming metric:
   - Engine calls ProcessMetric(metric)
   - Plugin stores metric in buffer
   - Plugin returns true when ready to execute
4. Engine calls Execute()
5. Plugin runs algorithm on buffered data
6. Returns computed metrics
7. Engine publishes to Redis computedMetrics

---

## API Reference

### REST API Endpoints

**Base URL**: `http://localhost:8080` (NWDAF main server)

All endpoints are reverse-proxied through the main NWDAF server to the appropriate microservices.

---

#### GET `/api/metrics`

Returns all active raw metrics collected from network functions.

**Response**: JSON array of metrics  
**Proxied to**: Data Archiver (port 8081)

```bash
curl http://localhost:8080/api/metrics
```

---

#### GET `/api/computed-metrics`

Returns all computed/forecasted metrics from analytics plugins.

**Response**: JSON array of computed metric responses  
**Proxied to**: Analytics Engine (port 8084)

```bash
curl http://localhost:8080/api/computed-metrics
```

---

#### GET `/api/computed-metrics/{name}`

Returns computed metrics filtered by metric name.

**Response**: JSON array filtered by metric name  
**Proxied to**: Analytics Engine (port 8084)

```bash
curl http://localhost:8080/api/computed-metrics/HPE_CONN_DEV_forecasted_value
```

---

#### GET `/api/models`

Returns information about loaded analytics plugins/models.

**Response**: JSON array of plugin metadata (ID, model type, description, required/produced metrics)  
**Proxied to**: Analytics Engine (port 8084)

```bash
curl http://localhost:8080/api/models
```

---

#### GET `/api/plugins`

Alias for `/api/models` - returns analytics plugin information.

**Response**: Same as `/api/models`  
**Proxied to**: Analytics Engine (port 8084)

```bash
curl http://localhost:8080/api/plugins
```

---

#### GET `/prometheus/metrics`

Returns metrics in Prometheus exposition format for scraping.

**Response**: Prometheus text format  
**Proxied to**: Prometheus endpoint (port 2112)

```bash
curl http://localhost:8080/prometheus/metrics
```

**Direct access** (not proxied):
```bash
curl http://localhost:2112/metrics
```

---

#### GET `/`

Web dashboard for real-time metric visualization.

**Response**: HTML/CSS/JS Material Design dashboard  
**Features**: Charts, filtering, auto-refresh, responsive design

Open in browser: `http://localhost:8080/`

---


### Prometheus API

**Endpoint**: http://localhost:2112/metrics

Standard Prometheus exposition format for all collected and computed metrics.

**Example**:
```bash
curl http://localhost:2112/metrics
```

---

## Troubleshooting

### Data Archiver fails with "Redis URI not set"
**Solution**: Ensure REDIS_URI is in config or environment variables.

### No plugins loaded
**Solution**:
- Check plugin naming matches core type (e.g., FAKE_*.go for core_type: FAKE)
- Verify plugins are built in plugin/collectors/build/ or plugin/analytics/build/
- Check plugin required environment variables are set

### Metrics not appearing in API
**Solution**:
- Verify Redis is running and accessible
- Check Data Collector is publishing (Redis SUBSCRIBE metrics)
- Ensure metrics haven't expired (>1 hour old)

### Analytics plugins not executing
**Solution**:
- Check plugin subscribed metrics match collector output names
- Verify minimum samples requirement is met
- Review analytics engine logs for plugin errors

---

## Directory Structure

```
NWDAF/
├── cmd/
│   ├── nwdaf/              # Main orchestrator
│   ├── darchiver/          # Data archiver microservice
│   ├── dcollector/         # Data collector microservice
│   └── analytics_engine/   # Analytics engine microservice
├── plugin/
│   ├── collectors/         # Data collector plugins
│   │   └── build/          # Built plugin binaries
│   ├── analytics/          # Analytics plugins
│   │   └── build/          # Built plugin binaries
│   └── plugin_shared/      # Shared plugin interfaces
├── pkg/
│   ├── configuration/      # Config management
│   ├── models/            # Data models
│   ├── redis_custom/      # Redis client wrapper
│   └── utils/             # Utility functions
├── config/
│   ├── config.yaml        # Production config
│   └── config_dev.yaml    # Development config
└── README.md
```

### Running Tests

```bash
# Test collector plugin locally
./plugin/collectors/build/FAKE_test_collector --debug-locally

# Test analytics plugin locally
./plugin/analytics/build/FAKE_moving_average --debug-locally
```

---

## License

License to be determined. This project is currently under active development for research and innovation purposes.

---

## Contributing

[Contribution guidelines if applicable]

---

## Contact

[Contact information or support channels]
