package main

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/pkg/redis_custom"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
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
//
// Parameters:
// - metric: The Metric object to be added or updated.
//
// Returns:
// - A MetricAndCollector object containing the added or updated metric and its collector.
func AddMetric(metric models.Metric) models.MetricAndCollector {
	if value, exists := MetricAndCollectorMap[metric.Name]; exists {
		updateGaugeValue(value.Collector.(*prometheus.GaugeVec), metric.Value)
		return value
	}
	logger.Debug("Adding metric", "name", metric.Name, "description", metric.Description)
	MetricAndCollectorMap[metric.Name] = models.MetricAndCollector{
		Metric:    metric,
		Collector: newGaugeCollector(metric),
	}
	return MetricAndCollectorMap[metric.Name]
}

// DeleteMetric deletes a metric from the MetricAndCollectorMap.
//
// Parameters:
// - name: The name of the metric to be deleted.
func DeleteMetric(name string) {
	if _, exists := MetricAndCollectorMap[name]; exists {
		logger.Debug("Removing metric", "name", name)
		deleteMetric(name)
	} else {
		logger.Error("Metric not found", "name", name)
	}
}

// GetMetric retrieves a metric by name.
//
// Parameters:
// - name: The name of the metric to retrieve.
//
// Returns:
// - A Metric object representing the retrieved metric.
func GetMetric(name string) models.Metric {
	if value, exists := MetricAndCollectorMap[name]; exists {
		return value.Metric
	}
	return models.Metric{}
}

// GetMetricList returns a list of all metric names.
//
// Returns:
// - A slice of strings containing the names of all metrics.
func GetMetricList() []string {
	metricList := make([]string, 0, len(MetricAndCollectorMap))
	for key := range MetricAndCollectorMap {
		metricList = append(metricList, MetricAndCollectorMap[key].Metric.Name)
	}
	return metricList
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

	// Register the metric counter with Prometheus.
	prometheus.MustRegister(metricCounter)

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

	// TODO when does a metric expire and must be removed from the published metrics?
}

// main is the entry point of the application.
func main() {
	Start()
}
