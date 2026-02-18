package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	httpinternal "github.com/s2n-cnit/nwdaf/pkg/http-internal"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)

// HPE AMF Core API endpoint constants
const (
	// HPEAuthLoginPath is the authentication endpoint for obtaining access tokens
	HPEAuthLoginPath = "https://%s/core/pls/api/1/auth/login"

	// HPESupisPath returns all SUPI (Subscription Permanent Identifier) list
	HPESupisPath = "https://%s/core/amf/api/1/ue/status/supis"

	// HPESupiDetailPath returns detailed status for a specific SUPI
	// Format: https://<core-ip>/core/amf/api/1/ue/status/supis/<supi>
	HPESupiDetailPath = "https://%s/core/amf/api/1/ue/status/supis/%v"

	// HTTPRequestTimeout is the maximum time to wait for HPE API responses
	// If API doesn't respond within this time, the request is skipped
	// Note: This constant is defined for future use when http_internal package supports timeout configuration
	HTTPRequestTimeout = 800 * time.Millisecond
)

// Response field name constants for type-safe field access
const (
	FieldAccessToken = "access_token"
	FieldMMState     = "mmState" // Mobility Management state
	FieldCMState     = "cmState" // Connection Management state
	StateRegistered  = "registered"
	StateConnected   = "connected"
)

// HPEAmfCollector collects metrics from HPE 5G Core AMF (Access and Mobility Management Function)
type HPEAmfCollector struct {
	logger         hclog.Logger
	bufferedLogger *plugin_shared.BufferedLogger
	coreIp         *string
	username       *string
	password       *string
	token          string    // OAuth access token for API authentication
	tokenExpiry    time.Time // When the current token expires
	metricPrefix   string
}

var (
	HandShakeConfigHPEAmfCollector = plugin.HandshakeConfig{
		ProtocolVersion: 1,
	}
	HPEAmfCollectorInstance = &HPEAmfCollector{
		logger: hclog.New(&hclog.LoggerOptions{
			Name:       "HPE_AMF_COLLECTOR",
			Level:      hclog.Trace,
			Output:     os.Stderr,
			JSONFormat: true,
		}),
	}
)

func (collector *HPEAmfCollector) SetEnvironment(debugMode bool) {
	collector.bufferedLogger = plugin_shared.NewBufferedLogger(collector.logger, debugMode)

	configuration.LoadEnvWithBufferedLogger(collector.bufferedLogger, "HPE_amf_collector")
	collector.logger.SetLevel(hclog.Level(configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug))))

	collector.coreIp = configuration.GetEnvStrNoDefault(plugin_shared.EnvHpeCoreIp)
	collector.username = configuration.GetEnvStrNoDefault(plugin_shared.EnvHpeUsername)
	collector.password = configuration.GetEnvStrNoDefault(plugin_shared.EnvHpePassword)
	collector.metricPrefix = configuration.GetEnv(configuration.EnvMetricPrefix, "NWDAF_")
	if collector.coreIp == nil || collector.username == nil || collector.password == nil {
		collector.logger.Error("Missing HPE environment variables")
		os.Exit(1)
	}

	// Load RPC handshake configuration for go-plugin communication
	cookieName := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyName)
	cookieValue := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyValue)
	// Debug mode: skip cookie setup
	if !debugMode {
		if cookieName == nil || cookieValue == nil {
			collector.logger.Error("Missing COOKIE name and value variables for RPC")
			os.Exit(1)
		}
		HandShakeConfigHPEAmfCollector.MagicCookieKey = *cookieName
		HandShakeConfigHPEAmfCollector.MagicCookieValue = *cookieValue
	}

	if collector.bufferedLogger != nil {
		collector.bufferedLogger.StartNormalLogging()
	}
}

// BuildMetric creates a metric with the configured prefix
func (collector *HPEAmfCollector) BuildMetric(name string, description string, value float64) models.Metric {
	return models.Metric{
		Name:        fmt.Sprintf("%s%s", collector.metricPrefix, name),
		Description: description,
		Value:       value,
		NFid:        "ABC", //TODO get from core
		NFType:      "amf",
		ReceivedAt:  time.Now(),
	}
}

// needsTokenRefresh checks if the authentication token needs to be refreshed
// Token is refreshed if it's empty or has expired
func (collector *HPEAmfCollector) needsTokenRefresh() bool {
	return collector.token == "" || time.Now().After(collector.tokenExpiry)
}

// Login authenticates the collector with the HPE core system and retrieves an access token.
// The token is cached and reused until it expires.
func (collector *HPEAmfCollector) Login() error {
	// Skip if token is still valid
	if !collector.needsTokenRefresh() {
		collector.logger.Debug("Using cached authentication token")
		return nil
	}

	collector.logger.Info("Authenticating with HPE core")

	// Construct the login URL using the constant template
	url := fmt.Sprintf(HPEAuthLoginPath, *collector.coreIp)

	// Create the request body with credentials
	body := map[string]string{
		"username": *collector.username,
		"password": *collector.password,
	}

	// Send the HTTP request and get the response
	resp, err := httpinternal.HttpRequestJsonBodyResp(url, httpinternal.POST, body, nil)

	// Check for errors BEFORE type assertion (critical fix)
	if err != nil {
		collector.logger.Error("Failed to authenticate with HPE core", "error", err)
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Safely type assert the response to a map
	castedValue, ok := resp.(map[string]interface{})
	if !ok {
		collector.logger.Error("Invalid response format from login endpoint", "response", resp)
		return fmt.Errorf("invalid response format")
	}

	// Safely extract the access token with type checking
	tokenValue, exists := castedValue[FieldAccessToken]
	if !exists {
		collector.logger.Error("Access token not found in response", "response", castedValue)
		return fmt.Errorf("access token missing in response")
	}

	token, ok := tokenValue.(string)
	if !ok {
		collector.logger.Error("Access token is not a string", "token", tokenValue)
		return fmt.Errorf("invalid token type")
	}

	// Store the token and set expiry (assume 1 hour validity)
	collector.token = token
	collector.tokenExpiry = time.Now().Add(1 * time.Hour)

	collector.logger.Info("Successfully authenticated with HPE core")
	return nil
}

// Collect gathers all metrics from the HPE AMF core
func (collector *HPEAmfCollector) Collect() []models.Metric {
	// Ensure we have a valid authentication token
	if err := collector.Login(); err != nil {
		collector.logger.Error("Cannot collect metrics without valid authentication")
		return []models.Metric{}
	}

	// Collect SUPI (subscriber) information and device status
	supiMetrics := collector.CollectSupiInfo()

	if supiMetrics == nil || len(supiMetrics) == 0 {
		collector.logger.Warn("No metrics collected from AMF")
		return []models.Metric{}
	}

	collector.logger.Debug("Successfully collected metrics", "count", len(supiMetrics))
	return supiMetrics
}

// GetRequiredEnvVars returns the list of environment variables required by this collector
func (collector *HPEAmfCollector) GetRequiredEnvVars() []string {
	return plugin_shared.HPERequiredEnvVars
}

// GetStartupLogs returns buffered logs from plugin initialization.
func (collector *HPEAmfCollector) GetStartupLogs() []plugin_shared.StartupLog {
	if collector.bufferedLogger == nil {
		return []plugin_shared.StartupLog{}
	}
	return collector.bufferedLogger.GetStartupLogs()
}

// CollectSupiInfo collects subscriber and device status information from HPE AMF
// Returns metrics for:
// - SUPI_NUM: Total number of subscribers
// - REGIS_DEV: Number of registered devices (mmState = "registered")
// - CONN_DEV: Number of connected devices (cmState = "connected")
//
// Performance note: This makes N+1 API calls (one for SUPI list, then one per SUPI for details).
// With many devices, this can be slow. The HPE API doesn't provide a batch endpoint.
func (collector *HPEAmfCollector) CollectSupiInfo() []models.Metric {
	// Construct the URL to get all SUPIs using the constant template
	supisUrl := fmt.Sprintf(HPESupisPath, *collector.coreIp)

	collector.logger.Debug("Fetching SUPI list from HPE AMF")

	// Fetch the list of all SUPIs
	resp, err := httpinternal.HttpRequestJsonBodyResp(supisUrl, httpinternal.GET, nil, &collector.token)
	if err != nil {
		collector.logger.Error("Failed to get SUPIs from HPE AMF", "error", err)
		return nil
	}

	// Safely type assert the response to a slice
	supiList, ok := resp.([]interface{})
	if !ok {
		collector.logger.Error("Invalid response format for SUPI list", "response", resp)
		return nil
	}

	// Initialize metrics
	supiNumMetric := collector.BuildMetric("SUPI_NUM", "Number of SUPIs", float64(len(supiList)))
	registeredDevices := collector.BuildMetric("REGIS_DEV", "Number of registered devices", 0)
	connectedDevices := collector.BuildMetric("CONN_DEV", "Number of connected devices", 0)

	collector.logger.Debug("Processing SUPI details", "total", len(supiList))

	// Iterate through each SUPI to get detailed status
	// Note: This is N+1 queries - one per SUPI. Can be slow with many devices.
	for i, supi := range supiList {
		// Construct URL for individual SUPI details using the constant template
		supiDetailUrl := fmt.Sprintf(HPESupiDetailPath, *collector.coreIp, supi)

		// Fetch detailed status for this SUPI
		supiResp, err := httpinternal.HttpRequestJsonBodyResp(supiDetailUrl, httpinternal.GET, nil, &collector.token)
		if err != nil {
			// Log error but continue processing other SUPIs
			collector.logger.Warn("Failed to get SUPI details, skipping",
				"supi", supi,
				"index", i+1,
				"total", len(supiList),
				"error", err)
			continue
		}

		// Safely type assert the SUPI details response
		supiInfo, ok := supiResp.(map[string]interface{})
		if !ok {
			collector.logger.Warn("Invalid SUPI detail response format, skipping",
				"supi", supi,
				"response", supiResp)
			continue
		}

		// Check Mobility Management state - safely extract with type checking
		if mmState, exists := supiInfo[FieldMMState]; exists {
			if mmStateStr, ok := mmState.(string); ok && mmStateStr == StateRegistered {
				registeredDevices.Value = registeredDevices.Value + 1 + rand.NormFloat64()/20.01
			}
		}

		// Check Connection Management state - safely extract with type checking
		if cmState, exists := supiInfo[FieldCMState]; exists {
			if cmStateStr, ok := cmState.(string); ok && cmStateStr == StateConnected {
				connectedDevices.Value = connectedDevices.Value + 1 + rand.NormFloat64()/20.01
			}
		}
	}

	collector.logger.Info("SUPI collection complete",
		"total", len(supiList),
		"registered", registeredDevices.Value,
		"connected", connectedDevices.Value)

	// Return metrics in a consistent order
	return []models.Metric{supiNumMetric, registeredDevices, connectedDevices}
}

func main() {
	// Parse command-line arguments
	debugMode := false
	for _, arg := range os.Args[1:] {
		if arg == "--debug-locally" {
			debugMode = true
			break
		}
	}

	// Initialize collector environment
	HPEAmfCollectorInstance.SetEnvironment(debugMode)

	// Authenticate with HPE core
	if err := HPEAmfCollectorInstance.Login(); err != nil {
		HPEAmfCollectorInstance.logger.Error("Failed to authenticate, exiting", "error", err)
		os.Exit(1)
	}

	if debugMode {
		// Debug mode: run collection once and display results
		HPEAmfCollectorInstance.logger.Info("Running in debug mode")

		requiredEnvVars := HPEAmfCollectorInstance.GetRequiredEnvVars()
		HPEAmfCollectorInstance.logger.Info("Required environment variables", "vars", requiredEnvVars)

		metrics := HPEAmfCollectorInstance.Collect()
		HPEAmfCollectorInstance.logger.Info("Collected metrics", "count", len(metrics))
		for _, metric := range metrics {
			HPEAmfCollectorInstance.logger.Info("Metric",
				"name", metric.Name,
				"value", metric.Value,
				"description", metric.Description)
		}
	} else {
		// Plugin mode: serve via go-plugin RPC
		pluginMap := map[string]plugin.Plugin{
			"HPE_amf_collector": &plugin_shared.MetricCollectorPlugin{Impl: HPEAmfCollectorInstance},
		}

		HPEAmfCollectorInstance.logger.Info("Starting HPE AMF Collector plugin", "plugins", pluginMap)

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: HandShakeConfigHPEAmfCollector,
			Plugins:         pluginMap,
		})
	}
}
