# Multi-stage Dockerfile for NWDAF Application
# This Dockerfile builds only the NWDAF application
# Plugins must be built separately and mounted at runtime

# Stage 1: Build the application
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code (excluding plugins build directories)
COPY . .

# Build main NWDAF application (runs all services - cmd/nwdaf/main.go)
RUN go build -ldflags="-s -w" -o /build/bin/nwdaf cmd/nwdaf/main.go

# Build individual service executables (required by cmd/nwdaf/main.go)
# Note: Build entire package, not just main.go, to include all files
RUN mkdir -p cmd/dcollector/build cmd/darchiver/build cmd/analytics_engine/build && \
    go build -ldflags="-s -w" -o cmd/dcollector/build/dcollector ./cmd/dcollector && \
    go build -ldflags="-s -w" -o cmd/darchiver/build/darchiver ./cmd/darchiver && \
    go build -ldflags="-s -w" -o cmd/analytics_engine/build/analytics_engine ./cmd/analytics_engine

# Stage 2: Create minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user for security
RUN addgroup -g 1000 nwdaf && \
    adduser -D -u 1000 -G nwdaf nwdaf

# Set working directory
WORKDIR /app

# Create necessary directories for plugin mounting and service binaries
RUN mkdir -p /app/plugin/collectors/build \
             /app/plugin/analytics/build \
             /app/cmd/dcollector/config \
             /app/cmd/dcollector/build \
             /app/cmd/darchiver/build \
             /app/cmd/darchiver/config \
             /app/cmd/analytics_engine/build \
             /app/config \
             /app/web && \
    chown -R nwdaf:nwdaf /app

# Copy main binary from builder
COPY --from=builder --chown=nwdaf:nwdaf /build/bin/nwdaf /app/nwdaf

# Copy service binaries (required by cmd/nwdaf/main.go)
COPY --from=builder --chown=nwdaf:nwdaf /build/cmd/dcollector/build/dcollector /app/cmd/dcollector/build/dcollector
COPY --from=builder --chown=nwdaf:nwdaf /build/cmd/darchiver/build/darchiver /app/cmd/darchiver/build/darchiver
COPY --from=builder --chown=nwdaf:nwdaf /build/cmd/analytics_engine/build/analytics_engine /app/cmd/analytics_engine/build/analytics_engine

# Copy default configuration file
COPY --chown=nwdaf:nwdaf config/config.yaml /app/config/config.yaml
COPY --chown=nwdaf:nwdaf plugin/collectors/config plugin/collectors/config
COPY --chown=nwdaf:nwdaf plugin/analytics/config plugin/analytics/config

# Copy web assets (directory exists in repo)
COPY --chown=nwdaf:nwdaf web /app/web

# Switch to non-root user
USER nwdaf

# Expose ports
# 8080 - Main NWDAF HTTP server (web dashboard)
# 8081 - Data Archiver API
# 8084 - Analytics Engine API
# 2112 - Prometheus metrics
EXPOSE 8080 8081 8084 2112

# Environment variables with defaults
# NOTE: These can be overridden via docker-compose or docker run
ENV LOG_LEVEL=1 \
    REDIS_URI=localhost:6379 \
    REDIS_PASSWORD="" \
    CORE_TYPE=F5GC \
    METRIC_PREFIX=NWDAF_ \
    PROMETHEUS_LOCAL_PORT=2112 \
    DARCHIVER_API_PORT=8081 \
    ANALYTICS_ENGINE_API_PORT=8084 \
    METRIC_EXPIRATION_SECONDS=300

# Health check (verifies web server is responding)
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Default command runs the main NWDAF application (cmd/nwdaf/main.go)
# Logs are sent to stdout/stderr by default
CMD ["/app/nwdaf"]
