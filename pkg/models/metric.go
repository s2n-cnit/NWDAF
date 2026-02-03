package models

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

// Metric is a struct that represents a metric.
type Metric struct {
	Name        string    `bson:"name" json:"name"`
	Description string    `bson:"description" json:"description"`
	Value       float64   `bson:"value" json:"value"`
	NFid        string    `bson:"nfid" json:"nfid"`
	NFType      string    `bson:"nfType" json:"nfType"`
	ReceivedAt  time.Time `bson:"receivedAt" json:"receivedAt"` // Timestamp when metric was received by darchiver
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
