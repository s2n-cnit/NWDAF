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

func deleteGaugeMetric(name string) {
	logger.Debug("Unregistering metric", "name", name)
	prometheus.Unregister(collectorMap[name])
}

func updateGaugeValue(vec *prometheus.GaugeVec, value float64) {
	vec.WithLabelValues("nwdaf").Set(value)
}

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

func DeleteMetric(name string) {
	if _, exists := MetricCollectorMap[name]; exists {
		logger.Debug("Removing metric", "name", name)
		deleteGaugeMetric(name)
		delete(MetricCollectorMap, name)
	}
}

func GetMetric(name string) models.Metric {
	if value, exists := MetricCollectorMap[name]; exists {
		return value.Metric
	}
	return models.Metric{}
}

func GetMetricList() []string {
	metricList := make([]string, 0, len(collectorMap))
	for key := range collectorMap {
		metricList = append(metricList, MetricCollectorMap[key].Metric.Name)
	}
	return metricList
}

func Start() {
	configuration.LoadConfig()
	log.LogSetup(logrus.Level(configuration.LogLevel))
	defer log.LogClose()

	rdb := redis.NewClient(&redis.Options{
		Addr: configuration.RedisURI,
	})

	pubsub := rdb.Subscribe(ctx, "metrics", "computedMetrics")
	logger.Debug("Subscribed to Redis topics", "topics", []string{"metrics", "computedMetrics"})

	if _, err := pubsub.Receive(ctx); err != nil {
		logrus.Fatal(err)
	}

	ch := pubsub.Channel()

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		uri := fmt.Sprintf(":%d", configuration.PrometheusPort)
		logger.Info("Starting Prometheus metric exporter", "uri", uri)
		logrus.Fatal(http.ListenAndServe(uri, nil))
	}()

	prometheus.MustRegister(metricCounter)

	for msg := range ch {
		fmt.Println("Received message from topic:", msg.Channel, "Message:", msg.Payload)

		var metric models.Metric
		if err := json.Unmarshal([]byte(msg.Payload), &metric); err != nil {
			logrus.Println("Error unmarshalling message payload:", err)
			continue
		}
		fmt.Println("Received metric:", metric)
		AddMetric(metric)
	}
}

func main() {
	Start()
}
