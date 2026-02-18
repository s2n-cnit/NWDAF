# NWDAF Docker Deployment Guide

This guide explains how to deploy the NWDAF application using Docker and Docker Compose.

## Table of Contents

- [Quick Start](#quick-start)
- [Architecture Overview](#architecture-overview)
- [Prerequisites](#prerequisites)
- [Building Plugins](#building-plugins)
- [Starting Infrastructure](#starting-infrastructure)
- [Running NWDAF](#running-nwdaf)
- [Plugin Management](#plugin-management)
- [Configuration](#configuration)
- [Logs](#logs)
- [Environment Variables](#environment-variables)
- [Port Mappings](#port-mappings)
- [Advanced Usage](#advanced-usage)
- [Troubleshooting](#troubleshooting)

## Quick Start

```bash
# 1. Build all plugins
./scripts/build_plugins.sh

# 2. Start infrastructure (Redis, MongoDB)
docker-compose -f docker-compose.infra.yml up -d

# 3. Build and start NWDAF
docker-compose up -d --build

# 4. Check status
docker-compose logs -f nwdaf

# 5. Access web dashboard
open http://localhost:8080
```

## Architecture Overview

The Docker deployment separates concerns into two compose files:

```
┌─────────────────────────────────────────────────────────┐
│  docker-compose.infra.yml (Infrastructure)              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │  Redis   │  │ MongoDB  │  │Prometheus│ (optional)  │
│  │  :6379   │  │  :27017  │  │  :9090   │             │
│  └──────────┘  └──────────┘  └──────────┘             │
└─────────────────────────────────────────────────────────┘
                         ▲
                         │ Network / localhost
                         │
┌────────────────────────┴─────────────────────────────────┐
│  docker-compose.yml (Application)                        │
│                                                           │
│  ┌───────────────────────────────────────────┐          │
│  │  NWDAF Container                           │          │
│  │  ┌─────────────────────────────────────┐  │          │
│  │  │ cmd/nwdaf/main.go                  │  │          │
│  │  │  ├─ Data Collector    :8081        │  │          │
│  │  │  ├─ Analytics Engine   :8084        │  │          │
│  │  │  ├─ Data Archiver                   │  │          │
│  │  │  └─ Web Dashboard      :8080        │  │          │
│  │  └─────────────────────────────────────┘  │          │
│  │                                             │          │
│  │  Mounted Plugins (pre-built on host):      │          │
│  │  /app/plugin/collectors/build/              │          │
│  │  /app/plugin/analytics/build/               │          │
│  └───────────────────────────────────────────┘          │
└──────────────────────────────────────────────────────────┘
```

**Key Points:**
- **Dockerfile**: Builds only the NWDAF application (cmd/nwdaf/main.go)
- **Plugins**: Built separately on host, mounted as read-only volumes
- **Infrastructure**: Runs in separate compose file (`docker-compose.infra.yml`)
- **Logs**: Sent to stdout/stderr (visible via `docker logs`)

## Prerequisites

- **Docker Engine** 20.10+
- **Docker Compose** 2.0+
- **Go** 1.24.3+ (for building plugins)
- **Redis** (provided via docker-compose.infra.yml or external)
- At least **2GB free RAM**

## Building Plugins

Plugins **must be built before** starting the NWDAF container.

### Build All Plugins

```bash
./scripts/build_plugins.sh
```

Output example:
```
=== NWDAF Plugin Builder ===
Building Collector plugins...
  Building: FAKE_test_collector... ✓
  Building: F5GC_amf_collector... ✓
  Building: HPE_amf_collector... ✓

Building Analytics plugins...
  Building: FAKE_moving_average... ✓
  Building: FAKE_SARIMA_number_ue... ✓
  Building: HPE_SARIMA_connected_ue... ✓

Done!
```

### Build Specific Plugin Type

```bash
# Only collectors
./scripts/build_plugins.sh collectors

# Only analytics
./scripts/build_plugins.sh analytics
```

### Manual Plugin Build

```bash
# Build a specific collector
cd plugin/collectors
go build -ldflags="-s -w" -o build/MY_PLUGIN MY_PLUGIN.go

# Build a specific analytics plugin
cd plugin/analytics
go build -ldflags="-s -w" -o build/MY_ANALYTICS MY_ANALYTICS.go
```

### Verify Built Plugins

```bash
# List built plugins
ls -lh plugin/collectors/build/
ls -lh plugin/analytics/build/

# Test a plugin in debug mode
./plugin/collectors/build/FAKE_test_collector --debug-locally
```

## Starting Infrastructure

Infrastructure services (Redis, MongoDB) run separately from the application.

### Start All Infrastructure

```bash
docker-compose -f docker-compose.infra.yml up -d
```

### Start with Monitoring

```bash
docker-compose -f docker-compose.infra.yml --profile monitoring up -d
```

This includes:
- **Redis** (required)
- **MongoDB** (optional, for data archiving)
- **Prometheus** (optional, for metrics collection)
- **Grafana** (optional, for visualization)

### Check Infrastructure Status

```bash
docker-compose -f docker-compose.infra.yml ps
docker-compose -f docker-compose.infra.yml logs redis
```

### Stop Infrastructure

```bash
docker-compose -f docker-compose.infra.yml down
```

## Running NWDAF

### Start NWDAF

```bash
# Build and start
docker-compose up -d --build

# Or just start (if already built)
docker-compose up -d
```

### View Logs

Logs are sent to **stdout/stderr** by default:

```bash
# Follow logs (real-time)
docker-compose logs -f nwdaf

# Last 100 lines
docker-compose logs --tail=100 nwdaf

# Logs since timestamp
docker-compose logs --since 2024-02-17T10:00:00 nwdaf
```

### Check Status

```bash
# Container status
docker-compose ps

# Health check
docker inspect nwdaf-app | grep -A 5 Health

# Resource usage
docker stats nwdaf-app
```

### Stop NWDAF

```bash
# Stop container
docker-compose stop

# Stop and remove
docker-compose down
```

## Plugin Management

### How Plugins are Mounted

Plugins are built on the host and mounted as read-only volumes:

```yaml
# docker-compose.yml
volumes:
  # All collector plugins
  - ./plugin/collectors/build:/app/plugin/collectors/build:ro

  # All analytics plugins
  - ./plugin/analytics/build:/app/plugin/analytics/build:ro
```

### Adding New Plugins

1. **Create the plugin** (see [plugin guides](plugin/))

2. **Build the plugin:**
   ```bash
   cd plugin/collectors
   go build -o build/MY_NEW_collector MY_NEW_collector.go
   ```

3. **Restart NWDAF:**
   ```bash
   docker-compose restart nwdaf
   ```

The plugin is automatically discovered on startup.

### Mounting Specific Plugins

Edit `docker-compose.yml` to mount individual plugins:

```yaml
volumes:
  # Mount only specific plugins
  - ./plugin/collectors/build/FAKE_test_collector:/app/plugin/collectors/build/FAKE_test_collector:ro
  - ./plugin/analytics/build/FAKE_moving_average:/app/plugin/analytics/build/FAKE_moving_average:ro
```

### Plugin Discovery

NWDAF discovers plugins based on:
- **Location**: `/app/plugin/collectors/build/` or `/app/plugin/analytics/build/`
- **Naming**: Filename must start with `CORE_TYPE_` prefix
- **Environment**: `CORE_TYPE` must match plugin prefix

Example:
```yaml
# In docker-compose.yml
environment:
  CORE_TYPE: HPE  # Only HPE_* plugins will be loaded
```

## Configuration

### Default Configuration

The container includes a default config at `/app/config/config.yaml`.

### Custom Configuration

Mount your own config file:

```yaml
# docker-compose.yml
volumes:
  - ./my-config.yaml:/app/config/config.yaml:ro
```

Example configuration:

```yaml
# my-config.yaml
server:
  port: 8080
  bind_ip: "0.0.0.0"

redis:
  uri: "localhost:6379"
  password: ""

mongo:
  username: "nwdaf"
  password: "nwdaf_password"
  uri: "mongodb://localhost:27017"
  name: "nwdaf"

core_type: "FAKE"
log_level: 1  # 0=Trace, 1=Debug, 2=Info, 3=Warn, 4=Error
prometheus_port: 2112
```

### Environment Variables

Override config via environment variables in `docker-compose.yml` or `.env` file:

```bash
# .env file
LOG_LEVEL=1
REDIS_URI=localhost:6379
CORE_TYPE=FAKE
METRIC_PREFIX=NWDAF_

# HPE-specific
HPE_CORE_IP=192.168.1.100
HPE_USERNAME=admin
HPE_PASSWORD=secret
```

Docker Compose automatically loads `.env` if present.

## Logs

### Viewing Logs

Logs are sent to **stdout/stderr** and visible via Docker commands:

```bash
# Follow logs (real-time)
docker-compose logs -f nwdaf

# All logs since start
docker logs nwdaf-app

# Export logs to file
docker-compose logs --no-color nwdaf > nwdaf-logs.txt
```

### Log Format

Structured JSON logging:

```json
{
  "@level": "info",
  "@message": "Starting NWDAF HTTP server",
  "@timestamp": "2024-02-17T10:30:45.123Z",
  "address": "0.0.0.0:8080"
}
```

### Log Levels

Control verbosity with `LOG_LEVEL`:

| Value | Level | Usage |
|-------|-------|-------|
| 0 | Trace | Most verbose - all details |
| 1 | Debug | Debugging information |
| 2 | Info | General information (recommended) |
| 3 | Warn | Warnings only |
| 4 | Error | Errors only |

```yaml
# docker-compose.yml
environment:
  LOG_LEVEL: 2  # Info level
```

## Environment Variables

### Core Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `1` | Logging verbosity (0-4) |
| `CORE_TYPE` | `FAKE` | 5G core type (Free5GC, HPE, O5GS, OAI, FAKE) |
| `REDIS_URI` | `localhost:6379` | Redis server address |
| `REDIS_PASSWORD` | `""` | Redis password |
| `METRIC_PREFIX` | `NWDAF_` | Prefix for all metrics |

### Service Ports

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMETHEUS_LOCAL_PORT` | `2112` | Prometheus metrics endpoint |
| `DARCHIVER_API_PORT` | `8081` | Data archiver API port |
| `ANALYTICS_ENGINE_API_PORT` | `8084` | Analytics engine API port |

### Analytics Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `METRIC_EXPIRATION_SECONDS` | `300` | Computed metrics cache duration |
| `ANALYTICS_WINDOW_SIZE` | `100` | Max samples in analytics buffer |
| `ANALYTICS_MINIMUM_SAMPLES` | `15` | Min samples before execution |

### Core-Specific Variables

**HPE Core:**
```bash
HPE_CORE_IP=192.168.1.100
HPE_USERNAME=admin
HPE_PASSWORD=secret
```

**Free5GC:**
```bash
FREE5GC_AMF_IP=192.168.1.100
FREE5GC_METRIC_PREFIX=F5GC_
```

## Port Mappings

### Application Ports (docker-compose.yml)

| Host | Container | Service | Description |
|------|-----------|---------|-------------|
| 8080 | 8080 | NWDAF Main | Web dashboard and HTTP API |
| 8081 | 8081 | Data Archiver | REST API for raw metrics |
| 8084 | 8084 | Analytics Engine | REST API for computed metrics |
| 2112 | 2112 | Prometheus | Metrics endpoint |

### Infrastructure Ports (docker-compose.infra.yml)

| Host | Container | Service |
|------|-----------|---------|
| 6379 | 6379 | Redis |
| 27017 | 27017 | MongoDB |
| 9090 | 9090 | Prometheus UI |
| 3000 | 3000 | Grafana |

### Accessing Services

```bash
# Web dashboard
curl http://localhost:8080/

# Raw metrics API
curl http://localhost:8081/api/metrics

# Computed metrics API
curl http://localhost:8084/api/computed-metrics

# Prometheus metrics
curl http://localhost:2112/metrics
```

## Advanced Usage

### Using External Redis

```yaml
# docker-compose.yml
environment:
  REDIS_URI: external-redis.example.com:6379
  REDIS_PASSWORD: your_password
```

### Using Docker Network

To use a Docker network instead of host mode:

1. Edit `docker-compose.yml` - comment out `network_mode: "host"` and uncomment:
   ```yaml
   networks:
     - nwdaf-net

   networks:
     nwdaf-net:
       external: true
       name: nwdaf-net
   ```

2. Update Redis URI:
   ```yaml
   environment:
     REDIS_URI: nwdaf-redis:6379  # Use container name
   ```

### Resource Limits

```yaml
# docker-compose.yml
services:
  nwdaf:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '1.0'
          memory: 1G
```

### Rebuilding Container

```bash
# Rebuild after code changes
docker-compose build --no-cache

# Rebuild and restart
docker-compose up -d --build
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker-compose logs nwdaf

# Common issues:
# - Redis not running
docker-compose -f docker-compose.infra.yml ps redis

# - Missing plugins
ls -l plugin/collectors/build/ plugin/analytics/build/

# - Port conflicts
netstat -tlnp | grep -E '8080|8081|8084|2112'
```

### Plugins Not Loading

```bash
# Check mounted plugins in container
docker exec nwdaf-app ls -la /app/plugin/collectors/build/
docker exec nwdaf-app ls -la /app/plugin/analytics/build/

# Ensure plugins are executable
chmod +x plugin/collectors/build/*
chmod +x plugin/analytics/build/*

# Check CORE_TYPE matches plugin prefix
docker exec nwdaf-app env | grep CORE_TYPE
```

### Connection to Redis Failed

```bash
# Test Redis from host
redis-cli -h localhost ping

# Check network mode
# If using host mode: REDIS_URI=localhost:6379
# If using Docker network: REDIS_URI=nwdaf-redis:6379
```

### Cannot Access Web Dashboard

```bash
# Test from inside container
docker exec nwdaf-app wget -O- http://localhost:8080/

# Check port binding
docker port nwdaf-app

# Check if port is in use
sudo netstat -tlnp | grep 8080
```

### High Memory Usage

```bash
# Check resource usage
docker stats nwdaf-app

# Solutions:
# - Reduce ANALYTICS_WINDOW_SIZE
# - Reduce METRIC_EXPIRATION_SECONDS
# - Add memory limits
```

## Useful Commands

```bash
# Complete deployment from scratch
./scripts/build_plugins.sh && \
  docker-compose -f docker-compose.infra.yml up -d && \
  docker-compose up -d --build

# Stop everything
docker-compose down
docker-compose -f docker-compose.infra.yml down

# Clean everything (including data)
docker-compose down -v
docker-compose -f docker-compose.infra.yml down -v

# Restart after plugin changes
./scripts/build_plugins.sh
docker-compose restart nwdaf

# Shell into container
docker exec -it nwdaf-app /bin/sh

# Export logs
docker-compose logs --no-color nwdaf > nwdaf-$(date +%Y%m%d).log
```

## Next Steps

- **Plugin Development**: [plugin/PLUGIN_DEVELOPMENT_GUIDE.md](plugin/PLUGIN_DEVELOPMENT_GUIDE.md)
- **Collector Plugins**: [plugin/collectors/COLLECTOR_PLUGIN_GUIDE.md](plugin/collectors/COLLECTOR_PLUGIN_GUIDE.md)
- **Analytics Plugins**: [plugin/analytics/ANALYTICS_PLUGIN_GUIDE.md](plugin/analytics/ANALYTICS_PLUGIN_GUIDE.md)
- **Configuration**: [config/config.yaml](config/config.yaml)
