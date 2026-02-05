# Environment Variables

This document summarizes environment variables used by NWDAF components and plugins.

## Global/Shared

These apply across multiple components or are required by microservices and plugins.

- `LOG_LEVEL`: Log verbosity (int). Used by cmd modules and plugins.
- `REDIS_URI`: Redis address used by collectors, archiver, analytics engine.
- `REDIS_PASSWORD`: Optional Redis password.
- `CORE_TYPE`: Core type used by collector/analytics to select compatible plugins.
- `METRIC_PREFIX`: Prefix for metric names used by collectors (set by `cmd/nwdaf` when launching them).
- `MAGIC_COOKIE_KEY`, `MAGIC_COOKIE_VALUE`: RPC cookie for plugins (set by host).

## cmd/nwdaf

Used to launch microservices and reverse-proxy to them.

- `DARCHIVER_API_PORT`: Port for darchiver API (proxy target).
- `ANALYTICS_ENGINE_API_PORT`: Port for analytics engine API (proxy target).
- `METRIC_EXPIRATION_SECONDS`: Passed to analytics engine.

## cmd/darchiver

- `DARCHIVER_API_PORT`: Gin API listen port (loopback).
- `PROMETHEUS_LOCAL_PORT`: Prometheus exporter port.
- `LOG_LEVEL`: Log level (shared).
- `REDIS_URI`, `REDIS_PASSWORD`: Redis connection.

## cmd/analytics_engine

- `ANALYTICS_ENGINE_API_PORT`: HTTP API port.
- `METRIC_EXPIRATION_SECONDS`: In-memory computed metrics TTL.
- `CORE_TYPE`: Selects compatible analytics plugins.
- `LOG_LEVEL`, `REDIS_URI`, `REDIS_PASSWORD`: Shared.

## cmd/dcollector

- `CORE_TYPE`: Selects compatible collector plugins.
- `METRIC_PREFIX`: Used to prefix emitted metrics (inherited from `cmd/nwdaf`).
- `LOG_LEVEL`, `REDIS_URI`, `REDIS_PASSWORD`: Shared.

## Collector Plugins

- HPE collector:
  - `HPE_CORE_IP`, `HPE_USERNAME`, `HPE_PASSWORD`.
- Free5GC collector:
  - `FREE5GC_AMF_IP`.
- Fake collector:
  - No plugin-specific variables.

Defaults for `METRIC_PREFIX` are only used when running a plugin standalone (debug mode).
