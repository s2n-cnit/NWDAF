// Package main contains API response models for the analytics engine
package main

import "github.com/s2n-cnit/nwdaf/pkg/models"

// PluginInfo represents information about a loaded analytics plugin
type PluginInfo struct {
	ID                string          `json:"id"`
	Model             string          `json:"model"`
	Description       string          `json:"description"`
	RequiredMetrics   []models.Metric `json:"required_metrics"`
	ProducedMetrics   []models.Metric `json:"produced_metrics"`
	MinimumSamples    int             `json:"minimum_samples"`
	ExecutionCount    int             `json:"execution_count"`
	LastExecutionTime string          `json:"last_execution_time,omitempty"`
}
