package models

import "github.com/prometheus/client_golang/prometheus"

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

type MetricAndCollector struct {
	Metric    Metric
	Collector prometheus.Collector
}
