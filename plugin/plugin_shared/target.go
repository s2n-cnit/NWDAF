package plugin_shared

import "fmt"

// Target represents a remote HTTP/HTTPS server endpoint
type Target struct {
	Name        string `yaml:"name"`        // Friendly name for the target
	URL         string `yaml:"url"`         // Base URL (e.g., "https://example.com")
	Port        int    `yaml:"port"`        // Port number (e.g., 443 for HTTPS, 80 for HTTP)
	Path        string `yaml:"path"`        // API path (e.g., "/api/v1/metrics")
	Description string `yaml:"description"` // Optional description
	Enabled     bool   `yaml:"enabled"`     // Whether this target is active
}

// TargetConfig represents the YAML configuration structure for targets
type TargetConfig struct {
	Targets []Target `yaml:"targets"`
}

// GetFullURL returns the complete URL for the target
func (t *Target) GetFullURL() string {
	if t.Port > 0 {
		return fmt.Sprintf("%s:%d%s", t.URL, t.Port, t.Path)
	}
	return fmt.Sprintf("%s%s", t.URL, t.Path)
}

// GetEnabledTargets returns a list of all enabled targets from a slice
func GetEnabledTargets(targets []Target) []Target {
	enabled := []Target{}
	for _, target := range targets {
		if target.Enabled {
			enabled = append(enabled, target)
		}
	}
	return enabled
}

// GetTargetByName returns a target by its name from a slice, or nil if not found
func GetTargetByName(targets []Target, name string) *Target {
	for i := range targets {
		if targets[i].Name == name {
			return &targets[i]
		}
	}
	return nil
}
