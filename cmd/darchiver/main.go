package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/go-hclog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
)

var (
	MetricCollectorMap = make(map[string]models.MetricAndCollector)
	collectorMap       = make(map[string]prometheus.Collector)
	ctx                = context.Background()
	metricCounter      = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "metric_number",
			Help: "Total number of metrics received",
		},
		[]string{"app"},
	)
	logger = hclog.New(&hclog.LoggerOptions{
		Name:   "Data Archiver",
		Output: os.Stdout,
		Level:  hclog.Debug,
	})
)

// addGaugeMetric adds a new Prometheus gauge metric and registers it.
//
// Parameters:
// - metric: The Metric object containing the name and description of the metric.
//
// Returns:
// - A prometheus.Collector representing the newly created gauge metric.
func addGaugeMetric(metric models.Metric) prometheus.Collector {
	logger.Debug("Adding metric", "name", metric.Name, "description", metric.Description)
	newGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metric.Name,
			Help: metric.Description,
		},
		[]string{"app"},
	)
	collectorMap[metric.Name] = newGauge
	prometheus.MustRegister(newGauge)
	metricCounter.WithLabelValues("nwdaf").Inc()
	return newGauge
}

// deleteGaugeMetric unregisters and deletes a Prometheus gauge metric.
//
// Parameters:
// - name: The name of the metric to be deleted.
func deleteGaugeMetric(name string) {
	logger.Debug("Unregistering metric", "name", name)
	prometheus.Unregister(collectorMap[name])
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
	if value, exists := MetricCollectorMap[metric.Name]; exists {
		updateGaugeValue(value.Collector.(*prometheus.GaugeVec), metric.Value)
		return value
	}
	logger.Debug("Adding metric", "name", metric.Name, "description", metric.Description)
	MetricCollectorMap[metric.Name] = models.MetricAndCollector{
		Metric:    metric,
		Collector: addGaugeMetric(metric),
	}
	return MetricCollectorMap[metric.Name]
}

// DeleteMetric deletes a metric from the MetricCollectorMap.
//
// Parameters:
// - name: The name of the metric to be deleted.
func DeleteMetric(name string) {
	if _, exists := MetricCollectorMap[name]; exists {
		logger.Debug("Removing metric", "name", name)
		deleteGaugeMetric(name)
		delete(MetricCollectorMap, name)
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
	if value, exists := MetricCollectorMap[name]; exists {
		return value.Metric
	}
	return models.Metric{}
}

// GetMetricList returns a list of all metric names.
//
// Returns:
// - A slice of strings containing the names of all metrics.
func GetMetricList() []string {
	metricList := make([]string, 0, len(collectorMap))
	for key := range collectorMap {
		metricList = append(metricList, MetricCollectorMap[key].Metric.Name)
	}
	return metricList
}

// Start initializes the configuration, subscribes to Redis topics, and starts the Prometheus metric exporter.
func Start() {
	// Load the configuration settings.
	configuration.LoadConfig()

	// Create a new Redis client with the specified options.
	rdb := redis.NewClient(&redis.Options{
		Addr: configuration.RedisURI,
	})

	// Subscribe to the "metrics" and "computedMetrics" Redis topics.
	pubsub := rdb.Subscribe(ctx, "metrics", "computedMetrics")
	logger.Debug("Subscribed to Redis topics", "topics", []string{"metrics", "computedMetrics"})

	// Wait for the subscription to be established.
	if _, err := pubsub.Receive(ctx); err != nil {
		logrus.Fatal(err)
	}

	// Get the channel for receiving messages from the subscribed topics.
	ch := pubsub.Channel()

	// Start a goroutine to handle Prometheus metrics endpoint.
	go func() {
		// Handle HTTP requests for the "/metrics" endpoint using the Prometheus handler.
		http.Handle("/metrics", promhttp.Handler())
		uri := fmt.Sprintf(":%d", configuration.PrometheusPort)
		logger.Info("Starting Prometheus metric exporter", "uri", uri)
		logrus.Fatal(http.ListenAndServe(uri, nil))
	}()

	// Register the metric counter with Prometheus.
	prometheus.MustRegister(metricCounter)

	// Process messages received from the Redis topics.
	for msg := range ch {
		fmt.Println("Received message from topic:", msg.Channel, "Message:", msg.Payload)

		// Unmarshal the message payload into a Metric object.
		var metric models.Metric
		if err := json.Unmarshal([]byte(msg.Payload), &metric); err != nil {
			logrus.Println("Error unmarshalling message payload:", err)
			continue
		}
		fmt.Println("Received metric:", metric)
		// Add or update the received metric.
		AddMetric(metric)
	}

	// TODO when does a metric expire and must be removed from the published metrics?
}

// main is the entry point of the application.
func main() {
	Start()
}
