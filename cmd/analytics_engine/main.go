// Package main implements the Analytics Engine for NWDAF.
//
// The Analytics Engine is responsible for:
//   - Loading analytics plugins compatible with the core type
//   - Subscribing to Redis metrics and computedMetrics topics
//   - Processing incoming metrics with registered plugins
//   - Storing computed metrics in memory with automatic expiration
//   - Publishing computed metrics back to Redis
//   - Exposing HTTP REST API endpoints for retrieving stored computed metrics
//
// Environment variables:
//   - ANALYTICS_ENGINE_API_PORT: Port for the HTTP API server (default: 8084)
//   - METRIC_EXPIRATION_SECONDS: Time in seconds before computed metrics expire (default: 300)
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/pkg/redis_custom"
	"github.com/s2n-cnit/nwdaf/pkg/utils"
	shared2 "github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)

const PluginFolder = "plugin/analytics/build/"

var (
	// handshakeConfig is the configuration for the plugin handshake.
	handshakeConfig = plugin.HandshakeConfig{
		ProtocolVersion: 1,
	}
	// logger is the global logger for the Analytics Engine.
	logger = hclog.New(&hclog.LoggerOptions{
		Name:   "Analytics Engine",
		Output: os.Stdout,
		Level:  hclog.Debug,
	})
	// redisClient is the Redis client used to subscribe to metrics and publish computed metrics.
	redisClient *redis_custom.RedisClient
	moduleInfo  = models.Module{
		Name:        "Analytics Engine",
		Description: "Module for NWDAF that runs analytics plugins to compute derived metrics.",
	}
	rpcClientList []*plugin.ClientProtocol
	pluginList    []AnalyticsPluginWrapper
	coreType      *string
)

// ComputedMetricBuffer stores all computed metrics from a single plugin execution.
type ComputedMetricBuffer struct {
	MetricsByName  map[string][]models.Metric
	LastUpdateTime time.Time
}

// AnalyticsPluginWrapper wraps an analytics plugin with metadata and stores computed metrics.
// The wrapper maintains a buffer of computed metrics that are automatically cleaned up
// after they expire (based on METRIC_EXPIRATION_SECONDS environment variable).
type AnalyticsPluginWrapper struct {
	ID                string                     // Unique identifier for this plugin instance
	Plugin            shared2.AnalyticsAlgorithm // The analytics plugin implementation
	SubscribedMetrics map[string]bool            // Map for fast lookup of subscribed metrics
	MinimumSamples    int                        // Minimum samples required before execution
	Name              string                     // Plugin name
	LastExecutionTime time.Time                  // Last time the plugin executed
	ExecutionCount    int64                      // Total number of executions
	ComputedMetrics   *ComputedMetricBuffer      // Stores all computed metrics from last execution
}

func SetEnvironment() {
	configuration.LoadEnvWithLogger(logger, "Analytics Engine")
	logger.SetLevel(hclog.Level(configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug))))

	// Get the core type from the environment variable.
	coreType = configuration.GetEnvStrNoDefault(configuration.EnvCoreType)
	if coreType == nil {
		logger.Error("Environment variable not set", "variable", configuration.EnvCoreType)
		os.Exit(1)
	}

	// Generate handshake config
	handshakeConfig.MagicCookieKey = fmt.Sprintf("%s_ANALYTICS_PLUGIN_COOKIE_KEY", *coreType)
	handshakeConfig.MagicCookieValue = utils.RandomString(15)
	configuration.SetEnv(shared2.EnvMagicCookieKeyName, handshakeConfig.MagicCookieKey)
	configuration.SetEnv(shared2.EnvMagicCookieKeyValue, handshakeConfig.MagicCookieValue)
}

// Start initializes and runs the Analytics Engine.
// It loads plugins, starts the HTTP API server, subscribes to Redis topics,
// and processes incoming metrics.
func Start() {
	SetEnvironment()
	initializeRedis()

	files, err := os.ReadDir(PluginFolder)
	if err != nil {
		log.Fatal(err)
	}

	// Load all plugins COMPATIBLE WITH THE CORE TYPE in the plugin folder.
	for _, fileOrFolder := range files {
		if !fileOrFolder.IsDir() {
			file := fileOrFolder
			// If the plugin name starts with the core type, load it.
			if strings.HasPrefix(file.Name(), *coreType) {
				client := LoadPlugin(fileOrFolder)
				defer func() {
					client.Kill()
				}()
			}
		}
	}

	if len(pluginList) == 0 {
		logger.Warn("No compatible analytics plugins found for core type", "coreType", *coreType)
		logger.Info("Analytics Engine running in passive mode (no plugins loaded)")
	} else {
		logger.Info("Loaded analytics plugins", "count", len(pluginList))
	}

	// Start HTTP API server for retrieving stored computed metrics
	go startHTTPServer()

	// Subscribe to metrics and computedMetrics topics
	pubsub := *redisClient.Subscribe("metrics", "computedMetrics")
	logger.Debug("Subscribed to Redis topics", "topics", []string{"metrics", "computedMetrics"})

	// Wait for the subscription to be established.
	if _, err := pubsub.Receive(redisClient.Ctx); err != nil {
		log.Fatal(err)
	}

	// Get the channel for receiving messages from the subscribed topics.
	ch := pubsub.Channel()

	// Process messages received from the Redis topics.
	for msg := range ch {
		logger.Trace("Received message from topic", "topic", msg.Channel, "message", msg.Payload)

		// Unmarshal the message payload into a Metric object.
		var metric models.Metric
		if err := json.Unmarshal([]byte(msg.Payload), &metric); err != nil {
			log.Println("Error unmarshalling message payload:", err)
			continue
		}

		// Process the metric with each plugin that subscribes to it.
		// This will also store computed metrics and clean up expired ones.
		ProcessMetricWithPlugins(metric)
	}
}

// LoadPlugin loads an analytics plugin from the specified file.
func LoadPlugin(file os.DirEntry) *plugin.Client {
	logger.Info("Found analytics plugin:", "file", file.Name())

	// Create a new plugin client with the specified configuration.
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         map[string]plugin.Plugin{file.Name(): &shared2.AnalyticsAlgorithmPlugin{}},
		Cmd:             exec.Command(PluginFolder + file.Name()),
		Logger:          logger,
	})
	logger.Info("Plugin loaded", "plugin", client)

	// Start the RPC client for the plugin.
	rpcClient, err := client.Client()
	if err != nil {
		logger.Error("Error starting RPC for plugin", "plugin", file.Name(), "location", PluginFolder+file.Name(), "error", err)
		client.Kill()
		return client
	}
	rpcClientList = append(rpcClientList, &rpcClient)

	// Dispense the plugin and add it to the plugin list.
	LoadedPlugin, err := rpcClient.Dispense(file.Name())
	if err != nil {
		logger.Error("Error loading remote plugin", "plugin", file.Name(), "error", err)
		client.Kill()
		return client
	}

	// Check if LoadedPlugin is nil
	if LoadedPlugin == nil {
		logger.Error("Plugin dispense returned nil, skipping", "plugin", file.Name())
		client.Kill()
		return client
	}

	// Type assertion with safety check
	analyticsPlugin, ok := LoadedPlugin.(shared2.AnalyticsAlgorithm)
	if !ok {
		logger.Error("Plugin does not implement AnalyticsAlgorithm interface, skipping", "plugin", file.Name())
		client.Kill()
		return client
	}

	// Retrieve and print buffered startup logs from plugin initialization
	startupLogs := analyticsPlugin.GetStartupLogs()
	for _, log := range startupLogs {
		logMessage := fmt.Sprintf("[PLUGIN:%s][STARTUP] %s", file.Name(), log.Message)
		switch log.Level {
		case "WARN":
			logger.Warn(logMessage, "timestamp", log.Timestamp, "fields", log.Fields)
		case "INFO":
			logger.Info(logMessage, "timestamp", log.Timestamp, "fields", log.Fields)
		default:
			logger.Debug(logMessage, "timestamp", log.Timestamp, "fields", log.Fields)
		}
	}

	// Check if all required environment variables are set for the specific plugin
	pluginEnabled := true
	for _, envVar := range analyticsPlugin.GetRequiredEnvVars() {
		_, exists := configuration.GetEnvNoDefault(envVar)
		if !exists {
			logger.Error("Required environment variable not set for plugin", "plugin", file.Name(), "envVar", envVar)
			pluginEnabled = false
			break
		}
	}

	// If all required environment variables are set, wrap and add the plugin
	if pluginEnabled {
		subscribedMetrics := analyticsPlugin.GetSubscribedMetrics()
		subscribedMap := make(map[string]bool)
		for _, metricName := range subscribedMetrics {
			subscribedMap[metricName] = true
		}

		wrapper := AnalyticsPluginWrapper{
			ID:                fmt.Sprintf("PLUGIN_%03d", len(pluginList)+1),
			Plugin:            analyticsPlugin,
			SubscribedMetrics: subscribedMap,
			MinimumSamples:    analyticsPlugin.GetMinimumSamples(),
			Name:              analyticsPlugin.GetName(),
			LastExecutionTime: time.Now(),
			ExecutionCount:    0,
			ComputedMetrics:   nil,
		}
		pluginList = append(pluginList, wrapper)

		logger.Info("Analytics plugin successfully loaded and enabled",
			"plugin", file.Name(),
			"name", wrapper.Name,
			"minimumSamples", wrapper.MinimumSamples)

		// Print subscribed metrics list
		if len(subscribedMetrics) == 0 {
			logger.Warn("Plugin subscribes to NO metrics", "plugin", wrapper.Name)
		} else {
			logger.Info("Plugin subscribed metrics", "plugin", wrapper.Name, "count", len(subscribedMetrics))
			for i, metricName := range subscribedMetrics {
				logger.Info("  Subscribed metric", "plugin", wrapper.Name, "index", i+1, "metric", metricName)
			}
		}
	} else {
		logger.Warn("Analytics plugin loaded but disabled due to missing environment variables", "plugin", file.Name())
		client.Kill()
	}

	return client
}

// ProcessMetricWithPlugins processes a metric with all plugins that subscribe to it.
// It also stores computed metrics in the plugin wrapper and cleans up expired metrics.
func ProcessMetricWithPlugins(metric models.Metric) {
	expirationDuration := time.Duration(configuration.GetEnvInt(configuration.EnvMetricExpirationSeconds, 300)) * time.Second

	for i := range pluginList {
		wrapper := &pluginList[i]

		// Clean up expired computed metrics
		cleanupExpiredMetrics(wrapper, expirationDuration)

		// Check if this plugin subscribes to this metric
		if !wrapper.SubscribedMetrics[metric.Name] {
			logger.Trace("Plugin does not subscribe to metric", "plugin", wrapper.Name, "metric", metric.Name)
			continue
		}

		logger.Trace("Processing metric with plugin", "plugin", wrapper.Name, "metric", metric.Name)

		// Wrap plugin execution in panic recovery to prevent crashes
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Plugin panicked during execution",
						"plugin", wrapper.Name,
						"metric", metric.Name,
						"panic", r)
				}
			}()

			// Let the plugin process the metric
			shouldExecute := wrapper.Plugin.ProcessMetric(metric)

			// Execute the algorithm if the plugin indicates it's ready
			if shouldExecute {
				logger.Debug("Executing analytics algorithm", "plugin", wrapper.Name)

				// Execute the algorithm
				computedMetricsMap := wrapper.Plugin.Execute()
				logger.Trace("Analytics algorithm returned metrics", "plugin", wrapper.Name, "metrics", computedMetricsMap)

				// Update wrapper metadata
				wrapper.LastExecutionTime = time.Now()
				wrapper.ExecutionCount++

				// Store full metric lists by name, but publish only the first element of each list.
				storedMetrics := make(map[string][]models.Metric)
				publishedMetrics := make([]models.Metric, 0)
				for metricName, metricList := range computedMetricsMap {
					if len(metricList) > 0 {
						storedMetrics[metricName] = metricList
						publishedMetrics = append(publishedMetrics, metricList[0])
					}
				}

				// Store computed metrics in wrapper (replace existing)
				if len(storedMetrics) > 0 {
					storeComputedMetrics(wrapper, storedMetrics)
					PublishComputedMetrics(publishedMetrics, wrapper.Name)
				} else {
					logger.Warn("Analytics algorithm returned no metrics", "plugin", wrapper.Name)
				}
			}
		}() // End of panic recovery function
	}
}

// storeComputedMetrics stores all computed metrics from a plugin execution, replacing the previous execution's metrics.
func storeComputedMetrics(wrapper *AnalyticsPluginWrapper, metricsByName map[string][]models.Metric) {
	now := time.Now()
	wrapper.ComputedMetrics = &ComputedMetricBuffer{
		MetricsByName:  metricsByName,
		LastUpdateTime: now,
	}
	totalCount := 0
	for _, metricList := range metricsByName {
		totalCount += len(metricList)
	}
	logger.Debug("Stored computed metrics", "plugin", wrapper.Name, "count", totalCount)
}

// cleanupExpiredMetrics removes computed metrics that haven't been updated within the expiration duration.
func cleanupExpiredMetrics(wrapper *AnalyticsPluginWrapper, expirationDuration time.Duration) {
	if wrapper.ComputedMetrics == nil {
		return
	}

	now := time.Now()
	if now.Sub(wrapper.ComputedMetrics.LastUpdateTime) > expirationDuration {
		wrapper.ComputedMetrics = nil
		logger.Debug("Removed expired computed metrics", "plugin", wrapper.Name)
	}
}

// initializeRedis initializes the Redis client.
func initializeRedis() {
	redisUri := configuration.GetEnvStrNoDefault(configuration.EnvRedisUri)
	if redisUri == nil {
		logger.Error("Redis URI not set. Missing ENV ", configuration.EnvRedisUri)
		os.Exit(1)
	}
	redisPwd := configuration.GetEnvStrNoDefault(configuration.EnvRedisPassword)
	newClient := redis_custom.NewCustomClient("Analytics Engine", hclog.Debug, *redisUri, redisPwd, &moduleInfo)
	redisClient = &newClient
}

// PublishComputedMetrics publishes computed metrics to Redis.
func PublishComputedMetrics(metrics []models.Metric, pluginName string) {
	for _, metric := range metrics {
		// Serialize the metric to JSON.
		jsonData, err := json.Marshal(metric)
		if err != nil {
			log.Println("Error serializing metric to JSON:", err)
		} else {
			logger.Debug("Publishing computed metric", "plugin", pluginName, "metric", metric.Name, "value", metric.Value)
			// Publish the serialized metric to the "computedMetrics" topic.
			redisClient.Publish("computedMetrics", string(jsonData))
		}
	}
}

// ComputedMetricResponse represents the API response for computed metrics from a plugin.
type ComputedMetricResponse struct {
	PluginName     string                     `json:"pluginName"`
	MetricsByName  map[string][]models.Metric `json:"metricsByName"`
	LastUpdateTime time.Time                  `json:"lastUpdateTime"`
}

// startHTTPServer starts the HTTP API server for retrieving stored computed metrics.
func startHTTPServer() {
	port := configuration.GetEnvInt(configuration.EnvAnalyticsEngineAPIPort, 8084)

	http.HandleFunc("/api/computed-metrics", handleGetAllComputedMetrics)
	http.HandleFunc("/api/computed-metrics/", handleGetComputedMetricByName)
	http.HandleFunc("/api/models", handleGetPlugins)

	addr := fmt.Sprintf(":%d", port)
	logger.Info("Starting Analytics Engine HTTP API server", "port", port)

	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Error("Failed to start HTTP API server", "error", err)
	}
}

// handleGetAllComputedMetrics returns all stored computed metrics from all plugins.
func handleGetAllComputedMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var response []ComputedMetricResponse

	for _, wrapper := range pluginList {
		if wrapper.ComputedMetrics != nil {
			response = append(response, ComputedMetricResponse{
				PluginName:     wrapper.Name,
				MetricsByName:  wrapper.ComputedMetrics.MetricsByName,
				LastUpdateTime: wrapper.ComputedMetrics.LastUpdateTime,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("Failed to encode response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleGetComputedMetricByName returns stored computed metrics filtered by a specific metric name across all plugins.
func handleGetComputedMetricByName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract metric name from URL path
	metricName := strings.TrimPrefix(r.URL.Path, "/api/v1/computed-metrics/")
	if metricName == "" {
		http.Error(w, "Metric name is required", http.StatusBadRequest)
		return
	}

	var response []ComputedMetricResponse
	found := false

	for _, wrapper := range pluginList {
		if wrapper.ComputedMetrics != nil {
			metricList, exists := wrapper.ComputedMetrics.MetricsByName[metricName]
			if exists && len(metricList) > 0 {
				found = true
				response = append(response, ComputedMetricResponse{
					PluginName:     wrapper.Name,
					MetricsByName:  map[string][]models.Metric{metricName: metricList},
					LastUpdateTime: wrapper.ComputedMetrics.LastUpdateTime,
				})
			}
		}
	}

	if !found {
		http.Error(w, "Metric not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("Failed to encode response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleGetPlugins returns information about all loaded analytics plugins.
func handleGetPlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Debug("GET /api/plugins - Fetching loaded plugins information")

	pluginInfoList := make([]PluginInfo, 0, len(pluginList))

	for _, wrapper := range pluginList {
		// Get plugin metadata via RPC
		description := wrapper.Plugin.GetDescription()
		producedMetricNames := wrapper.Plugin.GetProducedMetrics()
		metricDescriptions := wrapper.Plugin.GetMetricDescriptions()

		// Build required metrics list
		subscribedMetricNames := wrapper.Plugin.GetSubscribedMetrics()
		requiredMetrics := make([]models.Metric, 0, len(subscribedMetricNames))
		for _, metricName := range subscribedMetricNames {
			// Get description from plugin, or use default
			desc, exists := metricDescriptions[metricName]
			if !exists {
				desc = "Metric monitored by " + wrapper.Name
			}
			requiredMetrics = append(requiredMetrics, models.Metric{
				Name:        metricName,
				Description: desc,
			})
		}

		// Build produced metrics list
		producedMetrics := make([]models.Metric, 0, len(producedMetricNames))
		for _, metricName := range producedMetricNames {
			// Use a generic description for produced metrics
			producedDesc := "Computed metric produced by " + wrapper.Name
			producedMetrics = append(producedMetrics, models.Metric{
				Name:        metricName,
				Description: producedDesc,
			})
		}

		// Format last execution time
		lastExecTime := ""
		if !wrapper.LastExecutionTime.IsZero() {
			lastExecTime = wrapper.LastExecutionTime.Format("2006-01-02T15:04:05Z07:00")
		}

		pluginInfo := PluginInfo{
			ID:                wrapper.ID,
			Model:             wrapper.Name,
			Description:       description,
			RequiredMetrics:   requiredMetrics,
			ProducedMetrics:   producedMetrics,
			MinimumSamples:    wrapper.MinimumSamples,
			ExecutionCount:    int(wrapper.ExecutionCount),
			LastExecutionTime: lastExecTime,
		}

		pluginInfoList = append(pluginInfoList, pluginInfo)
	}

	logger.Info("Returning plugin information", "count", len(pluginInfoList))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pluginInfoList); err != nil {
		logger.Error("Failed to encode response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// main is the entry point of the application.
func main() {
	Start()
}
