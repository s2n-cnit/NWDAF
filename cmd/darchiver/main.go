package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/pkg/redis_custom"
	"github.com/sirupsen/logrus"
)

var (
	// MetricAndCollectorMap is a map of metric names to their corresponding MetricAndCollector objects.
	MetricAndCollectorMap = make(map[string]models.MetricAndCollector)
	// metricCounter is a Prometheus counter metric for the total number of metrics received.
	metricCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "metric_number",
			Help: "Total number of metrics received",
		},
		[]string{"app"},
	)
	// logger is the global logger for the Data Archiver module.
	logger      hclog.Logger
	redisClient redis_custom.RedisClient
	moduleInfo  = models.Module{
		Name:        "Data Archiver",
		Description: "Module for NWDAF that listen to Redis Events and archive metrics.",
	}
)

// newGaugeCollector generates a new Prometheus gauge metric.
//
// Parameters:
// - metric: The Metric object containing the name and description of the metric.
//
// Returns:
// - A prometheus.Collector representing the newly created gauge metric.
func newGaugeCollector(metric models.Metric) prometheus.Collector {
	newGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metric.Name,
			Help: metric.Description,
		},
		[]string{"app"},
	)
	prometheus.MustRegister(newGauge)
	metricCounter.WithLabelValues("nwdaf").Inc()
	return newGauge
}

// deleteMetric unregisters and deletes a Prometheus metric.
//
// Parameters:
// - name: The name of the metric to be deleted.
func deleteMetric(name string) {
	logger.Debug("Unregistering metric", "name", name)
	metricAndCollector := MetricAndCollectorMap[name]
	prometheus.Unregister(metricAndCollector.Collector)
	delete(MetricAndCollectorMap, name)
}

// updateGaugeValue updates the value of a Prometheus gauge metric.
//
// Parameters:
// - vec: The Prometheus gauge vector to be updated.
// - value: The new value to set for the gauge.
func updateGaugeValue(vec *prometheus.GaugeVec, value float64) {
	vec.WithLabelValues("nwdaf").Set(value)
}

// AddMetric adds a new metric or updates an existing one.
// Sets ReceivedAt timestamp when the metric is received.
//
// Parameters:
// - metric: The Metric object to be added or updated.
//
// Returns:
// - A MetricAndCollector object containing the added or updated metric and its collector.
func AddMetric(metric models.Metric) models.MetricAndCollector {
	// Set the received timestamp
	metric.ReceivedAt = time.Now()

	if value, exists := MetricAndCollectorMap[metric.Name]; exists {
		updateGaugeValue(value.Collector.(*prometheus.GaugeVec), metric.Value)
		// Update the metric with new value and timestamp
		value.Metric = metric
		MetricAndCollectorMap[metric.Name] = value
		return value
	}
	logger.Debug("Adding metric", "name", metric.Name, "description", metric.Description)
	MetricAndCollectorMap[metric.Name] = models.MetricAndCollector{
		Metric:    metric,
		Collector: newGaugeCollector(metric),
	}
	return MetricAndCollectorMap[metric.Name]
}

// GetAllMetrics returns a list of all metrics with their data.
//
// Returns:
// - A slice of Metric objects containing all metrics.
func GetAllMetrics() []models.Metric {
	metricList := make([]models.Metric, 0, len(MetricAndCollectorMap))
	for key := range MetricAndCollectorMap {
		metricList = append(metricList, MetricAndCollectorMap[key].Metric)
	}
	return metricList
}

// CleanupExpiredMetrics removes metrics that haven't been updated for more than 1 hour.
func CleanupExpiredMetrics() {
	expirationDuration := 1 * time.Hour
	now := time.Now()
	expiredCount := 0

	for name, metricAndCollector := range MetricAndCollectorMap {
		if now.Sub(metricAndCollector.Metric.ReceivedAt) > expirationDuration {
			logger.Debug("Metric expired, removing", "name", name, "receivedAt", metricAndCollector.Metric.ReceivedAt)
			deleteMetric(name)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		logger.Info("Cleaned up expired metrics", "count", expiredCount)
	}
}

// startMetricExpirationCleanup starts a goroutine that periodically cleans up expired metrics.
func startMetricExpirationCleanup() {
	go func() {
		// Run cleanup every 10 minutes
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			CleanupExpiredMetrics()
		}
	}()
	logger.Info("Started metric expiration cleanup task (runs every 10 minutes)")
}

// setupAPIRouter sets up the Gin router with API endpoints.
//
// Returns:
// - A configured Gin Engine.
func setupAPIRouter() *gin.Engine {
	// Set Gin to release mode based on log level
	if logger.GetLevel() > hclog.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// API endpoints
	api := router.Group("/api")
	{
		api.GET("/metrics", func(c *gin.Context) {
			metrics := GetAllMetrics()
			c.JSON(http.StatusOK, metrics)
		})
	}

	return router
}

// Start initializes the configuration, subscribes to Redis topics, and starts the Prometheus metric exporter.
func Start() {
	// Load the configuration settings.
	configuration.LoadEnv()
	// Set the logging level based on the environment variable.
	logLevel := hclog.Level(configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug)))
	logger = hclog.New(&hclog.LoggerOptions{
		Name:   "Data Archiver",
		Output: os.Stdout,
		Level:  logLevel,
	})

	// Create a new Redis client with the specified options.
	redisUri := configuration.GetEnvStrNoDefault(configuration.EnvRedisUri)
	if redisUri == nil {
		logger.Error("Redis URI not set. Missing ENV ", configuration.EnvRedisUri)
		os.Exit(1)
	}
	redis_pwd := configuration.GetEnvStrNoDefault(configuration.EnvRedisPassword)
	redisClient = redis_custom.NewCustomClient("Data Archiver", logLevel, *redisUri, redis_pwd, &moduleInfo)

	// Subscribe to the "metrics" and "computedMetrics" Redis topics.
	pubsub := *redisClient.Subscribe("metrics", "computedMetrics")
	logger.Debug("Subscribed to Redis topics", "topics", []string{"metrics", "computedMetrics"})

	// Wait for the subscription to be established.
	if _, err := pubsub.Receive(redisClient.Ctx); err != nil {
		logrus.Fatal(err)
	}

	// Get the channel for receiving messages from the subscribed topics.
	ch := pubsub.Channel()

	// Start a goroutine to handle Prometheus metrics endpoint.
	go func() {
		prometheusPort := configuration.GetEnvInt(configuration.EnvPrometheusLocalPort, 2112)
		// Handle HTTP requests for the "/metrics" endpoint using the Prometheus handler.
		http.Handle("/metrics", promhttp.Handler())
		uri := fmt.Sprintf(":%d", prometheusPort)
		logger.Info("Starting Prometheus metric exporter", "uri", uri)
		logrus.Fatal(http.ListenAndServe(uri, nil))
	}()

	// Start a goroutine to handle API endpoints using Gin.
	go func() {
		apiPort := configuration.GetEnvInt(configuration.EnvDArchiverAPIPort, 8081)
		router := setupAPIRouter()
		uri := fmt.Sprintf("127.0.0.1:%d", apiPort)
		logger.Info("Starting API server on loopback", "uri", uri)
		if err := router.Run(uri); err != nil {
			logrus.Fatal("Failed to start API server:", err)
		}
	}()

	// Register the metric counter with Prometheus.
	prometheus.MustRegister(metricCounter)

	// Start the metric expiration cleanup task
	startMetricExpirationCleanup()

	// Process messages received from the Redis topics.
	for msg := range ch {
		logger.Debug("Received message from topic", "topic", msg.Channel, "message", msg.Payload)

		// Unmarshal the message payload into a Metric object.
		var metric models.Metric
		if err := json.Unmarshal([]byte(msg.Payload), &metric); err != nil {
			logrus.Println("Error unmarshalling message payload:", err)
			continue
		}
		logger.Debug("Metric Detected", "metric", metric)
		// Add or update the received metric.
		AddMetric(metric)
	}
}

// main is the entry point of the application.
func main() {
	Start()
}
