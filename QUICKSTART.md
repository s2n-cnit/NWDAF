# NWDAF Quick Start Guide

Get up and running with NWDAF in minutes using Docker Compose.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Quick Setup](#quick-setup)
- [Step-by-Step Guide](#step-by-step-guide)
- [Verify Installation](#verify-installation)
- [Next Steps](#next-steps)
- [Developing Custom Plugins](#developing-custom-plugins)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before you begin, ensure you have the following installed:

- **Docker** (20.10 or higher)
- **Docker Compose** (v2.0 or higher)
- **Go** (1.20 or higher) - for building plugins
- **Git** - for cloning the repository

### Verify Prerequisites

```bash
# Check Docker
docker --version

# Check Docker Compose
docker compose version

# Check Go
go version
```

---

## Quick Setup

For the impatient, here's the fastest way to get NWDAF running:

```bash
# 1. Clone and navigate to repository
git clone <repository-url>
cd NWDAF

# 2. Build plugins
bash scripts/build_plugins.sh all

# 3. Start infrastructure (Redis, Prometheus)
docker compose -f docker-compose.infra.yml up -d

# 4. Configure NWDAF (use default config_dev.yaml or customize)
nano config/config.yaml

# 5. Start NWDAF
docker compose up -d

# 6. Access the dashboard
# Open http://localhost:8080 in your browser
```

That's it! Skip to [Verify Installation](#verify-installation) to confirm everything is working.

---

## Step-by-Step Guide

### Step 1: Clone the Repository

```bash
git clone <repository-url>
cd NWDAF
```

### Step 2: Build Plugins

NWDAF uses a plugin architecture. You need to build the plugins before starting the containers.

```bash
# Build all plugins (collectors + analytics)
bash scripts/build_plugins.sh all
```

**What this does:**
- Detects the Go binary
- Builds all collector plugins in `plugin/collectors/build/`
- Builds all analytics plugins in `plugin/analytics/build/`
- Makes them executable
- Shows build summary

**Expected output:**
```
=== NWDAF Plugin Builder ===
Project root: /path/to/NWDAF

Using Go from: /home/user/go/go1.25.6/bin/go
Go version: 1.25.6

Building Collector plugins...
  Building: F5GC_amf_collector...
  ...
  ✓

Building Analytics plugins...
  Building: FAKE_moving_average...
  ...
  ✓

Done!
```

**Build specific plugin types:**
```bash
# Only collectors
bash scripts/build_plugins.sh collectors

# Only analytics
bash scripts/build_plugins.sh analytics
```

### Step 3: Start Infrastructure Services

NWDAF requires Redis (for pub/sub messaging) and optionally Prometheus (for monitoring).

```bash
# Start Redis and Prometheus
docker compose -f docker-compose.infra.yml up -d
```

Verify both services are "Up":
```bash
docker compose -f docker-compose.infra.yml ps
```

You should see both services with status "Up".

### Step 4: Configure NWDAF

NWDAF uses a YAML configuration file:

```bash
nano config/config.yaml
```

**Key settings:**
- `core_type: "FAKE"` - Use FAKE plugins for testing
- Plugins must be prefixed with core type (e.g., `FAKE_*`, `HPE_*`, `F5GC_*`)

**Customization options:**

For production or specific core networks, edit `config/config.yaml`:

```yaml
# Example: HPE core network
core_type: "HPE"

slices:
  - id: "production-slice"
    core_endpoint_ip: "10.0.0.1"
    username: "admin"
    password: "secure_password"
    amf_ips:
      - "10.0.0.10"
    smf_ips:
      - "10.0.0.20"
```

**Important:** The `core_type` determines which plugins are loaded. Plugins must be prefixed with the core type (e.g., `FAKE_*`, `HPE_*`, `F5GC_*`).

### Step 5: Start NWDAF

Now start the main NWDAF application and all its microservices:

```bash
docker compose up -d
```

**What this starts:**
- **NWDAF Main** (port 8080): Orchestrator and reverse proxy
- **Data Collector**: Loads and runs collector plugins
- **Data Archiver** (port 2112, 8081): Archives metrics to Prometheus
- **Analytics Engine** (port 8084): Loads and runs analytics plugins

**View logs:**
```bash
# Follow all logs
docker compose logs -f

# Follow specific service
docker compose logs -f nwdaf-app

# View last 100 lines
docker compose logs --tail=100
```

**Check running services:**
```bash
docker compose ps
```

All services should show "Up" status.

---

## Verify Installation

### 1. Check Web Dashboard

Open your browser and navigate to:

```
http://localhost:8080
```

You should see the Material Design dashboard with:
- Real-time metrics display
- Line charts for raw metrics and forecasts
- Metric filtering
- Auto-refresh toggle

### 2. Test REST API Endpoints

```bash
# Get all raw metrics
curl http://localhost:8080/api/metrics | jq

# Get computed/forecasted metrics
curl http://localhost:8080/api/computed-metrics | jq

# Get loaded plugin information
curl http://localhost:8080/api/models | jq

# Get Prometheus metrics (only if started)
curl http://localhost:8080/prometheus/metrics
```

### 3. Check Plugin Loading

View the logs to confirm plugins were loaded:

```bash
docker compose logs nwdaf-app | grep "successfully loaded"
```

You should see messages like:
```
Plugin successfully loaded: FAKE_test_collector
Plugin successfully loaded: FAKE_moving_average
Plugin successfully loaded: FAKE_SARIMA_number_ue
```

### 4. Monitor with Prometheus (Optional)

Open Prometheus UI:
```
http://localhost:9090
```

Query for NWDAF metrics:
- Enter `NWDAF_` in the query box
- Click "Execute"
- Switch to "Graph" tab to visualize

---

## Next Steps

### Access the Services

| Service | URL | Description |
|---------|-----|-------------|
| Web Dashboard | http://localhost:8080 | Real-time metric visualization |
| Prometheus | http://localhost:9090 | Metrics monitoring and querying |
| Data Archiver API | http://localhost:8080/api/metrics | JSON metrics endpoint |
| Analytics Engine API | http://localhost:8080/api/computed-metrics | Computed metrics endpoint |

### Explore the Data

1. **View Raw Metrics**: Click on the "Latest Raw Metrics" card in the dashboard
2. **View Forecasts**: Check the "Latest Forecasts" section for predicted values
3. **Filter Metrics**: Use the search box to find specific metrics by name
4. **Adjust Refresh Rate**: Use the slider to change polling interval (2-60 seconds)

### Stop and Cleanup

```bash
# Stop NWDAF services
docker compose down

# Stop infrastructure
docker compose -f docker-compose.infra.yml down

# Remove volumes (WARNING: deletes all data)
docker compose down -v
docker compose -f docker-compose.infra.yml down -v
```

---

## Developing Custom Plugins

Ready to create your own collector or analytics plugin? Follow these guides:

### Plugin Development Resources

1. **Collector Plugin Guide**: [plugin/collectors/COLLECTOR_PLUGIN_GUIDE.md](plugin/collectors/COLLECTOR_PLUGIN_GUIDE.md)
   - Learn how to collect metrics from network functions
   - See examples: `FAKE_test_collector.go`, `HPE_amf_collector.go`

2. **Analytics Plugin Guide**: [plugin/analytics/ANALYTICS_PLUGIN_GUIDE.md](plugin/analytics/ANALYTICS_PLUGIN_GUIDE.md)
   - Learn how to create forecasting/analytics algorithms
   - See examples: `FAKE_moving_average.go`, `HPE_SARIMA_connected_ue.go`

3. **General Plugin Development**: [plugin/PLUGIN_DEVELOPMENT_GUIDE.md](plugin/PLUGIN_DEVELOPMENT_GUIDE.md)
   - Shared interfaces and best practices

### Quick Plugin Development Workflow

#### 1. Create Your Plugin

Copy an example and customize (see [plugin guides](#plugin-development-resources) for details):

```bash
# Collector example
cp plugin/collectors/FAKE_test_collector.go plugin/collectors/MYCORE_my_collector.go

# Analytics example
cp plugin/analytics/FAKE_moving_average.go plugin/analytics/MYCORE_my_algorithm.go
```

**Naming Convention:** Plugins MUST be prefixed with core type: `<CORE_TYPE>_<plugin_name>`

#### 2. Build Your Plugin

```bash
bash scripts/build_plugins.sh all
```

The script shows the exact build command and creates executable binaries in `plugin/*/build/`.

#### 3. Configure Environment Variables

Add required environment variables (see [plugin guides](#plugin-development-resources) for specifics):

```yaml
# docker-compose.yml
services:
  nwdaf-app:
    environment:
      - MYCORE_API_ENDPOINT=http://mycore:8080
      - MYCORE_API_TOKEN=secret_token
```

Or add to `.env` file.

#### 4. Update Configuration

Update `config/config.yaml` to use your core type:

```yaml
core_type: "MYCORE"  # Must match your plugin prefix

slices:
  - id: "my-slice"
    # Add any slice-specific configuration
```

#### 5. Rebuild and Restart Docker

After building your plugin, restart NWDAF to load it:

```bash
# Rebuild the Docker image (includes new plugins)
docker compose build

# Restart services
docker compose down
docker compose up -d
```

**For faster iteration** (during development):

```bash
# Build plugins
bash scripts/build_plugins.sh all

# Just restart without rebuilding image
docker compose restart
```

The plugin binaries are mounted as volumes, so you can:
1. Build plugin with script
2. Restart container
3. New plugin is loaded automatically

#### 6. Verify Plugin Loading

Check the logs to confirm your plugin was loaded:

```bash
# For collector plugins
docker compose logs nwdaf-app | grep "MYCORE_my_collector"

# For analytics plugins
docker compose logs nwdaf-app | grep "MYCORE_my_algorithm"
```

You should see:
```
Plugin successfully loaded and enabled: MYCORE_my_collector
```

Or:
```
Analytics plugin subscribed to metrics: ["metric_name_1", "metric_name_2"]
```

#### 7. Debug Your Plugin

**Test locally first** (before Docker):

All plugins support a `--debug-locally` flag for standalone testing:

```bash
# Test collector plugin
./plugin/collectors/build/MYCORE_my_collector --debug-locally

# Test analytics plugin
./plugin/analytics/build/MYCORE_my_algorithm --debug-locally
```

This runs the plugin in debug mode:
- Bypasses RPC handshake
- Outputs directly to stdout
- Uses test data if available
- Shows detailed logs

**Check plugin logs in Docker:**

```bash
# View all logs with plugin name
docker compose logs -f | grep "MYCORE"

# View specific microservice logs
docker compose logs -f nwdaf-app
```

### Plugin Development Tips

1. **Start with FAKE plugins**: Copy and modify FAKE examples for testing
2. **Use unique variable names**: Avoid conflicts (e.g., `MyPluginSubscribedMetrics` instead of `SubscribedMetrics`)
3. **Test standalone first**: Use `--debug-locally` flag before Docker testing
4. **Check required env vars**: Plugin won't load if required variables are missing
5. **Match core type prefix**: Plugin filename must start with `CORE_TYPE_`
6. **Use buffered logger**: Prevents RPC handshake issues (see examples)

### Example: Adding a Simple Collector

Here's a complete example of adding a new collector plugin:

```bash
# 1. Create plugin file
cat > plugin/collectors/MYCORE_test_collector.go << 'EOF'
package main

import (
    "os"
    "github.com/hashicorp/go-hclog"
    "github.com/s2n-cnit/nwdaf/pkg/models"
    "github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)

type MyCollector struct {
    logger hclog.Logger
}

func (c *MyCollector) Collect() []models.Metric {
    return []models.Metric{
        {
            Name:        "MYCORE_test_metric",
            Description: "A test metric",
            Value:       42.0,
            NFid:        "test-001",
            NFType:      "TEST",
        },
    }
}

func (c *MyCollector) GetRequiredEnvVars() []string {
    return []string{}
}

func (c *MyCollector) SetEnvironment(debugMode bool) {
    // Environment setup
}

func (c *MyCollector) GetStartupLogs() []plugin_shared.StartupLog {
    return []plugin_shared.StartupLog{}
}

func main() {
    // Standard plugin setup (see existing examples)
}
EOF

# 2. Build
bash scripts/build_plugins.sh collectors

# 3. Update config
sed -i 's/core_type: "FAKE"/core_type: "MYCORE"/' config/config.yaml

# 4. Restart
docker compose restart

# 5. Verify
docker compose logs nwdaf-app | grep "MYCORE_test_collector"
```

---

## Troubleshooting

### Plugins Not Building

```bash
# Check Go version (need 1.20+)
go version

# Try building manually to see errors
cd plugin/collectors
CGO_ENABLED=0 go build -o build/FAKE_test_collector FAKE_test_collector.go
```

### Container Won't Start

```bash
# Check logs for errors
docker compose logs

# Verify infrastructure is running
docker compose -f docker-compose.infra.yml ps

# Remove old containers and retry
docker compose down && docker compose up -d
```

### Plugins Not Loading

**Problem**: Plugins don't appear in logs or API

**Solutions**:

1. **Check core type matches plugin prefix:**
   ```bash
   # config.yaml should have:
   core_type: "FAKE"  # Loads FAKE_* plugins
   ```

2. **Verify plugin is built and executable:**
   ```bash
   ls -la plugin/collectors/build/
   ls -la plugin/analytics/build/
   ```

3. **Check required environment variables:**
   ```bash
   docker compose logs | grep "missing environment"
   ```

4. **Verify plugin naming convention:**
   - Correct: `FAKE_test_collector.go` → `FAKE_test_collector` binary
   - Wrong: `test_collector.go` (no prefix)

### Redis Connection Issues

**Problem**: "Redis URI not set" or connection refused

**Solutions**:
```bash
# Check Redis is running
docker compose -f docker-compose.infra.yml ps redis

# Test Redis connection
docker exec -it $(docker ps -qf name=redis) redis-cli ping
# Should respond: PONG

# Check Redis URI in config
grep -A3 "redis:" config/config.yaml
# Should show: uri: "redis:6379" (for Docker)
```

### No Metrics Appearing

**Problem**: Dashboard shows "No data" or API returns empty arrays

**Solutions**:

1. **Wait for metrics to be generated** (FAKE plugins generate every 60 seconds)

2. **Check collector is running:**
   ```bash
   docker compose logs | grep "Collector"
   ```

3. **Verify metrics are published to Redis:**
   ```bash
   docker exec -it $(docker ps -qf name=redis) redis-cli
   > SUBSCRIBE metrics
   # Watch for incoming messages
   ```

4. **Check Data Archiver is subscribing:**
   ```bash
   docker compose logs | grep "Data Archiver"
   ```

### Web Dashboard Not Loading

**Problem**: Browser shows blank page or errors

**Solutions**:

1. **Check NWDAF Main is running:**
   ```bash
   docker compose ps | grep nwdaf-app
   ```

2. **Verify port 8080 is accessible:**
   ```bash
   curl http://localhost:8080
   ```

3. **Check browser console** (F12) for JavaScript errors

4. **Try different browser** or clear cache

### Plugin-Specific Errors

**Problem**: Plugin crashes or returns errors

**Solutions**:

1. **Test plugin locally first:**
   ```bash
   ./plugin/collectors/build/YOUR_PLUGIN --debug-locally
   ```

2. **Check plugin logs:**
   ```bash
   docker compose logs | grep "YOUR_PLUGIN"
   ```

3. **Verify environment variables are set:**
   ```bash
   docker compose exec nwdaf-app env | grep YOUR_
   ```

4. **Review plugin code** for nil pointer dereferences or missing error handling

---

## Additional Resources

- **Main Documentation**: [README.md](README.md)
- **Web Dashboard Guide**: [web/README.md](web/README.md)
- **Docker Configuration**: [DOCKER.md](DOCKER.md)
- **Plugin Development**: 
  - [plugin/PLUGIN_DEVELOPMENT_GUIDE.md](plugin/PLUGIN_DEVELOPMENT_GUIDE.md)
  - [plugin/collectors/COLLECTOR_PLUGIN_GUIDE.md](plugin/collectors/COLLECTOR_PLUGIN_GUIDE.md)
  - [plugin/analytics/ANALYTICS_PLUGIN_GUIDE.md](plugin/analytics/ANALYTICS_PLUGIN_GUIDE.md)

---

## Quick Reference Commands

```bash
# Build plugins
bash scripts/build_plugins.sh all

# Start infrastructure
docker compose -f docker-compose.infra.yml up -d

# Start NWDAF
docker compose up -d

# View logs
docker compose logs -f

# Restart after plugin changes
bash scripts/build_plugins.sh all && docker compose restart

# Stop everything
docker compose down && docker compose -f docker-compose.infra.yml down

# Clean rebuild
docker compose down -v
docker compose build --no-cache
docker compose up -d
```

---

## Need Help?

If you encounter issues not covered here:

1. Check the main [README.md](README.md) for detailed component documentation
2. Review plugin development guides in `plugin/` directory
3. Check Docker logs: `docker compose logs -f`
4. Verify environment variables: `docker compose config`
5. Test plugins standalone with `--debug-locally` flag

---

**Ready to dive deeper?** Check out the [main README](README.md) for comprehensive documentation on architecture, configuration, and advanced features.

