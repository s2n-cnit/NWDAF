# HPE SARIMA Connected UE Forecasting Plugin

## Overview

The `HPE_SARIMA_connected_ue` plugin implements a SARIMA/ARIMA time-series forecasting algorithm to predict the number of connected User Equipment (UEs) based on data collected from the HPE AMF (Access and Mobility Management Function).

This plugin is designed to work with the HPE AMF collector (`HPE_amf_collector`) that monitors the number of connected devices in the HPE 5G core network.

## Features

- **SARIMA/ARIMA Forecasting**: Uses the `goarima` library to automatically select the best ARIMA model for time-series prediction
- **Dynamic Model Refresh**: Automatically retrains the model every 10 new samples to adapt to changing patterns
- **Configurable Window**: Maintains a sliding window of up to 100 samples for model training
- **Metric Prefix Support**: Respects the `METRIC_PREFIX` environment variable to match the collector's metric naming

## Subscribed Metrics

The plugin subscribes to:
- `{METRIC_PREFIX}CONN_DEV` - Number of connected devices from HPE AMF

Where `{METRIC_PREFIX}` defaults to `NWDAF_` if not configured via environment variable.

## Output Metrics

The plugin produces:
- `{METRIC_PREFIX}CONN_DEV_forecasted_value` - Forecasted number of connected devices

The plugin generates 10 forecast values into the future, with timestamps projected based on the mean time distance between samples.

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `METRIC_PREFIX` | No | `NWDAF_` | Prefix for metric names |
| `LOG_LEVEL` | No | Debug | Logging level (Trace, Debug, Info, Warn, Error) |

### Plugin Parameters

| Parameter | Value | Description |
|-----------|-------|-------------|
| Window Size | 100 | Maximum number of samples kept in buffer |
| Minimum Samples | 10 | Minimum samples required before first forecast |
| Model Refresh Interval | 10 | Number of new samples before model is retrained |

## Usage

### Building the Plugin

```bash
cd /path/to/NWDAF
go build -o plugin/analytics/build/HPE_SARIMA_connected_ue ./plugin/analytics/HPE_SARIMA_connected_ue.go
```

### Running as a Plugin

The plugin is loaded by the Analytics Engine (`cmd/analytics_engine/main.go`) and runs as an RPC server:

```bash
# The Analytics Engine will automatically load the plugin
./cmd/analytics_engine/build/analytics_engine
```

### Debug Mode

For testing and debugging, run the plugin standalone:

```bash
./plugin/analytics/build/HPE_SARIMA_connected_ue --debug-locally
```

This will:
1. Generate synthetic test data with 30 samples
2. Process metrics through the forecasting algorithm
3. Display forecasted values with timestamps
4. Log debug information to stderr

## How It Works

1. **Data Collection**: The plugin receives metrics from the Redis pub/sub system, published by the Data Archiver
2. **Buffer Management**: Metrics are stored in a sliding window buffer (max 100 samples)
3. **Model Training**: When 10 new samples are accumulated, the SARIMA model is retrained
4. **Forecasting**: The model generates 10 forecast values into the future
5. **Publishing**: Forecasted metrics are published back to Redis for storage and API access

### Forecast Lifecycle

```
New Metric → Buffer → Ready? → Train Model → Generate Forecasts → Publish
                ↓ No                   ↓ (every 10 samples)
              Wait                 Use Cached Forecasts
```

## Integration with HPE AMF Collector

This plugin is designed to work with the `HPE_amf_collector` plugin:

1. **HPE Collector** queries the HPE 5G core API for connected device information
2. **Data Collector** publishes `NWDAF_CONN_DEV` metrics to Redis
3. **Data Archiver** forwards metrics to the `computedMetrics` channel
4. **Analytics Engine** loads this plugin and subscribes to `NWDAF_CONN_DEV`
5. **This Plugin** processes metrics and generates forecasts
6. **Data Archiver** stores forecasts in Prometheus and exposes via REST API

## Example Output

When running in debug mode, you'll see output like:

```json
{"@level":"info","@message":"Forecasted value","value":68.94736842105263,"time":"2026-02-05T09:32:16+01:00"}
{"@level":"info","@message":"Forecasted value","value":69.89473684210526,"time":"2026-02-05T09:32:26+01:00"}
{"@level":"info","@message":"Forecasted value","value":70.84210526315789,"time":"2026-02-05T09:32:36+01:00"}
```

## Dependencies

- `github.com/hashicorp/go-hclog` - Structured logging
- `github.com/hashicorp/go-plugin` - Go plugin system
- `github.com/sartorproj/goarima` - ARIMA/SARIMA implementation
- `github.com/s2n-cnit/nwdaf/pkg/configuration` - Configuration management
- `github.com/s2n-cnit/nwdaf/pkg/models` - Data models
- `github.com/s2n-cnit/nwdaf/plugin/plugin_shared` - Plugin interfaces

## Troubleshooting

### Plugin Not Receiving Metrics

1. Check that the HPE AMF collector is running and publishing metrics
2. Verify the `METRIC_PREFIX` environment variable matches between collector and plugin
3. Check Redis connectivity and pub/sub channels

### Model Training Failures

- Ensure at least 10 samples are available
- Check for invalid data (NaN, infinity, zero timestamps)
- Review logs for ARIMA model fitting errors

### No Forecasts Generated

- Verify buffer has minimum samples (10)
- Check that `ReceivedAt` timestamps are set on metrics
- Review `newSamples` counter in debug logs

## Performance Considerations

- **Memory**: Plugin maintains a buffer of up to 100 metrics per subscribed metric name
- **CPU**: ARIMA model training is computationally intensive, occurring every 10 samples
- **Latency**: Model training takes ~1ms on typical hardware

## Future Enhancements

- Configurable forecast horizon (currently fixed at 10 steps)
- Support for additional metrics (REGIS_DEV, SUPI_NUM)
- Confidence intervals for forecasts
- Model persistence across restarts
- Performance metrics (RMSE, MAE, R²)
