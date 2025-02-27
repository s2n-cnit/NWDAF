package models

import "github.com/prometheus/client_golang/prometheus"

// Metric is a struct that represents a metric.
type Metric struct {
	Name        string  `bson:"name"`
	Description string  `bson:"description"`
	Value       float64 `bson:"value"`
	NFid        string  `bson:"nfid"`
	NFType      string  `bson:"nfType"`
}

// Equal compares two Metric instances for equality.
// It returns true if the Name fields of both Metric instances are equal, otherwise false.
func (m Metric) Equal(other Metric) bool {
	return m.Name == other.Name
}

// MetricAndCollector is a struct that contains a Metric and its corresponding Prometheus Collector.
type MetricAndCollector struct {
	Metric    Metric
	Collector prometheus.Collector
}
