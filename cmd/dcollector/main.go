// Description: The main entry point of the Data Collector module.
// This module is responsible for collecting data from the core and sending it to Redis.
package main

import (
	"encoding/json"
	"fmt"
	"log"
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

const PluginFolder = "plugin/collectors/build/"

var (
	// handshakeConfig is the configuration for the plugin handshake.
	handshakeConfig = plugin.HandshakeConfig{
		ProtocolVersion: 1,
	}
	// logger is the global logger for the Data Collector module.
	logger = hclog.New(&hclog.LoggerOptions{
		Name:   "Data Collector",
		Output: os.Stdout,
		Level:  hclog.Debug,
	})
	// redisClient is the Redis client used to publish metrics.
	redisClient *redis_custom.RedisClient
	moduleInfo  = models.Module{
		Name:        "Data Collector",
		Description: "Module for NWDAF runs plugins collecting data from the core and sends it to Redis.",
	}
	rpcClientList           []*plugin.ClientProtocol
	pluginList              []interface{}
	scrapingIntervalSeconds = 10
	scrapingInterval        = time.Duration(scrapingIntervalSeconds) * time.Second
	coreType                *string
	metricsPrefix           string
)

func SetEnvironment() {
	configuration.LoadEnv()
	metricsPrefix = configuration.GetEnv(configuration.EnvMetricPrefix, "NWDAF_")
	logger.SetLevel(hclog.Level(configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug))))

	// Get the core type from the environment variable.
	coreType = configuration.GetEnvStrNoDefault(configuration.EnvCoreType)
	if coreType == nil {
		logger.Error("Environment variable not set", "variable", configuration.EnvCoreType)
		os.Exit(1)
	}

	//Generate handshake config
	handshakeConfig.MagicCookieKey = fmt.Sprintf("%s_PLUGIN_COOKIE_KEY", *coreType)
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
			// If the plugin name is starting with the core type, load it. Because the plugin is compatible with the core type.
			if strings.HasPrefix(file.Name(), *coreType) {
				client := LoadPlugin(fileOrFolder)
				defer func() {
					client.Kill()
				}()
			}
		}
	}
	if len(pluginList) == 0 {
		logger.Error("No compatible plugins found for core type", "coreType", *coreType)
		os.Exit(1)
	}

	//After loading all plugins, for each one, start collecting data and sending it to redis.
	for {
		var collectedData []models.Metric
		for _, loadedPlugin := range pluginList {
			collector := loadedPlugin.(shared2.MetricCollector)
			// Collect data from the plugin and append it to the collected data. If panic data is nil
			collectedDataSinglePlugin := CollectNoPanic(collector)
			collectedData = append(collectedData, collectedDataSinglePlugin...)
		}
		PublishOnRedis(collectedData)

		logger.Info(fmt.Sprintf("Data collected and sent to redis. Waiting %d seconds before collecting data again...", scrapingIntervalSeconds))
		time.Sleep(scrapingInterval)
	}
}

// CollectNoPanic collects data from the specified metric collector and returns the collected metrics.
// If a panic occurs during the collection, the function recovers and returns nil.
//
// Parameters:
// - metricCollector: The metric collector to collect data from.
//
// Returns:
// - A slice of Metric objects representing the collected data.
func CollectNoPanic(metricCollector shared2.MetricCollector) []models.Metric {
	var collectedMetrics []models.Metric
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Recovered from panic")
		}
	}()

	collectedMetrics = metricCollector.Collect()
	return collectedMetrics
}

// LoadPlugin loads a plugin from the specified file and returns the plugin client.
//
// Parameters:
// - file: The file entry representing the plugin to be loaded.
//
// Returns:
// - A pointer to the plugin.Client representing the loaded plugin.
func LoadPlugin(file os.DirEntry) *plugin.Client {
	logger.Info("Found plugin:", "file", file.Name())

	// Create a new plugin client with the specified configuration.
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         map[string]plugin.Plugin{file.Name(): &shared2.MetricCollectorPlugin{}},
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

	//Check if all required environment variables are set for the specific plugin and SET THEM
	plugin_enabled := true
	for _, envVar := range LoadedPlugin.(shared2.MetricCollector).GetRequiredEnvVars() {
		_, exists := configuration.GetEnvNoDefault(envVar)
		if !exists {
			logger.Error("Required environment variable not set for plugin", "plugin", file.Name(), "envVar", envVar)
			plugin_enabled = false
			break
		}
	}
	// If all required environment variables are set, add the plugin to the plugin list.
	if plugin_enabled {
		pluginList = append(pluginList, LoadedPlugin)
	}
	return client
}

func initializeRedis() {
	redisUri := configuration.GetEnvStrNoDefault(configuration.EnvRedisUri)
	if redisUri == nil {
		logger.Error("Redis URI not set. Missing ENV ", configuration.EnvRedisUri)
		os.Exit(1)
	}
	redisPwd := configuration.GetEnvStrNoDefault(configuration.EnvRedisPassword)
	newClient := redis_custom.NewCustomClient("Data Collector", hclog.Debug, *redisUri, redisPwd, &moduleInfo)
	redisClient = &newClient
}

// PublishOnRedis publishes the given metrics to the Redis "metrics" topic.
//
// Parameters:
// - metrics: A slice of Metric objects to be published.
func PublishOnRedis(metrics []models.Metric) {
	// Iterate over each metric and publish it to Redis.
	for _, metric := range metrics {
		// Serialize the metric to JSON.
		jsonData, err := json.Marshal(metric)
		if err != nil {
			log.Println("Error serializing metric to JSON:", err)
		} else {
			logger.Trace("Serialized metric to JSON:", string(jsonData))
			// Publish the serialized metric to the "metrics" topic.
			redisClient.Publish("metrics", string(jsonData))
		}
	}
}

// main is the entry point of the application.
func main() {
	Start()
}
