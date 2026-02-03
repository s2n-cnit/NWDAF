// Package main implements the Analytics Engine for NWDAF.
// The Analytics Engine loads analytics plugins, subscribes to Redis metrics,
// and publishes computed metrics back to Redis.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/pkg/redis_custom"
	"github.com/s2n-cnit/nwdaf/pkg/utils"
	shared2 "github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
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

// AnalyticsPluginWrapper wraps an analytics plugin with metadata.
type AnalyticsPluginWrapper struct {
	Plugin            shared2.AnalyticsAlgorithm
	SubscribedMetrics map[string]bool // Map for fast lookup
	MinimumSamples    int
	Name              string
	LastExecutionTime time.Time
	ExecutionCount    int64
}

func SetEnvironment() {
	configuration.LoadEnv()
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
			// If the plugin name is starting with the core type, load it.
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

		// Process the metric with each plugin that subscribes to it
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
		logger.Error("Error starting RPC for", "plugin", client, "error", err)
	}
	rpcClientList = append(rpcClientList, &rpcClient)

	// Dispense the plugin and add it to the plugin list.
	LoadedPlugin, err := rpcClient.Dispense(file.Name())
	if err != nil {
		logger.Error("Error loading remote plugin", "plugin", file.Name(), "error", err)
		return client
	}

	// Check if LoadedPlugin is nil
	if LoadedPlugin == nil {
		logger.Error("Plugin dispense returned nil", "plugin", file.Name())
		return client
	}

	// Check if all required environment variables are set for the specific plugin
	plugin_enabled := true
	analyticsPlugin := LoadedPlugin.(shared2.AnalyticsAlgorithm)
	for _, envVar := range analyticsPlugin.GetRequiredEnvVars() {
		_, exists := configuration.GetEnvNoDefault(envVar)
		if !exists {
			logger.Error("Required environment variable not set for plugin", "plugin", file.Name(), "envVar", envVar)
			plugin_enabled = false
			break
		}
	}

	// If all required environment variables are set, wrap and add the plugin
	if plugin_enabled {
		subscribedMetrics := analyticsPlugin.GetSubscribedMetrics()
		subscribedMap := make(map[string]bool)
		for _, metricName := range subscribedMetrics {
			subscribedMap[metricName] = true
		}

		wrapper := AnalyticsPluginWrapper{
			Plugin:            analyticsPlugin,
			SubscribedMetrics: subscribedMap,
			MinimumSamples:    analyticsPlugin.GetMinimumSamples(),
			Name:              analyticsPlugin.GetName(),
			LastExecutionTime: time.Now(),
			ExecutionCount:    0,
		}
		pluginList = append(pluginList, wrapper)

		logger.Info("Analytics plugin enabled",
			"name", wrapper.Name,
			"subscribedMetrics", subscribedMetrics,
			"minimumSamples", wrapper.MinimumSamples)
	}

	return client
}

// ProcessMetricWithPlugins processes a metric with all plugins that subscribe to it.
func ProcessMetricWithPlugins(metric models.Metric) {
	for i := range pluginList {
		wrapper := &pluginList[i]

		// Check if this plugin subscribes to this metric
		if !wrapper.SubscribedMetrics[metric.Name] {
			logger.Trace("Plugin does not subscribe to metric", "plugin", wrapper.Name, "metric", metric.Name)
			continue
		}

		logger.Trace("Processing metric with plugin", "plugin", wrapper.Name, "metric", metric.Name)

		// Let the plugin process the metric
		shouldExecute := wrapper.Plugin.ProcessMetric(metric)

		// Execute the algorithm if the plugin indicates it's ready
		if shouldExecute {
			logger.Debug("Executing analytics algorithm", "plugin", wrapper.Name)

			// Execute the algorithm
			computedMetrics := wrapper.Plugin.Execute()

			// Update wrapper metadata
			wrapper.LastExecutionTime = time.Now()
			wrapper.ExecutionCount++

			// Publish computed metrics to Redis
			if len(computedMetrics) > 0 {
				PublishComputedMetrics(computedMetrics, wrapper.Name)
			}
		}
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

// main is the entry point of the application.
func main() {
	Start()
}
