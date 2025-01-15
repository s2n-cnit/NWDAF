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
	"github.com/s2n-cnit/nwdaf/pkg/log"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
)

var (
	metrics_map = make(map[string]prometheus.Collector)

	ctx = context.Background()

	// Define a Prometheus counter metric
	metricCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "metric_number",
			Help: "Total number of metrics received",
		},
		[]string{"app"},
	)

	logger = hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Output: os.Stdout,
		Level:  hclog.Debug,
	})
)

func addGaugeMetric(metric models.Metric) prometheus.Collector {
	if _, exists := metrics_map[metric.Name]; !exists {
		logger.Debug("Adding metric", "name", metric.Name, "description", metric.Description)
		var new_gauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metric.Name,
				Help: metric.Description,
			},
			[]string{"app"},
		)
		metrics_map[metric.Name] = new_gauge
		prometheus.MustRegister(new_gauge)

		// Increment the Prometheus counter
		metricCounter.WithLabelValues("nwdaf").Inc()

		return new_gauge
	} else {
		logger.Debug("Metric is already present, updating it")
	}
	return metrics_map[metric.Name]
}

func getMetric(name string) prometheus.Collector {
	if _, exists := metrics_map[name]; exists {
		return metrics_map[name]
	}
	return nil
}

func removeMetric(name string) {
	prometheus.Unregister(metrics_map[name])
	delete(metrics_map, name)
}

func main() {
	configuration.LoadConfig()
	log.LogSetup(logrus.Level(configuration.LogLevel))
	defer log.LogClose()

	// Initialize Redis client to listen at collected metrics
	rdb := redis.NewClient(&redis.Options{
		Addr: configuration.RedisURI, // Redis server address
	})

	// Subscribe to metric and computed metric topics
	pubsub := rdb.Subscribe(ctx, "metrics", "computedMetrics")
	logger.Debug("Subscribed to Redis topics", "topics", []string{"metrics", "computedMetrics"})

	// Wait for confirmation that subscription is created
	_, err := pubsub.Receive(ctx)
	if err != nil {
		logrus.Fatal(err)
	}

	// Create a channel to receive messages
	ch := pubsub.Channel()

	// Start a HTTP server for exposing Prometheus metrics
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", configuration.PrometheusPort), nil))
	}()

	prometheus.MustRegister(metricCounter)

	// Listen for messages
	for msg := range ch {
		fmt.Println("Received message from topic:", msg.Channel, "Message:", msg.Payload)

		var metric models.Metric
		err := json.Unmarshal([]byte(msg.Payload), &metric)
		if err != nil {
			logrus.Println("Error unmarshalling message payload:", err)
			continue
		}
		fmt.Println("Received metric:", metric)

		var collector = addGaugeMetric(metric)

		vec := collector.(*prometheus.GaugeVec)
		vec.WithLabelValues("nwdaf").Set(metric.Value)
	}
}
