package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	shared2 "github.com/s2n-cnit/nwdaf/plugin/shared"
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

const (
	PluginFolder = "plugin/collectors/build/"
)

// handshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var (
	handshakeConfig = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "NWDAF_PLUGIN_COOCKIE_KEY",
		MagicCookieValue: "dsJha6J899JNjudayscn",
	}
	redisInitialized = false
	// Create an hclog.Logger
	logger = hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Output: os.Stdout,
		Level:  hclog.Debug,
	})
	ctx         context.Context
	redisClient *redis.Client
	moduleInfo  = models.Module{
		Name:        "Free5GC Collector",
		Description: "Module for NWDAF that collect data from Free5GC and send it to redis 'metric' topic.",
	}
	rpcClientList    = make([]*plugin.ClientProtocol, 0)
	pluginList       = make([]interface{}, 0)
	scrapingInterval = 60 * time.Second
)

func main() {
	configuration.LoadConfig()
	logger.SetLevel(hclog.Trace)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("Starting loading plugins")
	files, err := os.ReadDir(PluginFolder)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if !file.IsDir() {
			// Perform your operations on each file
			logger.Info("Found plugin:", "file", file.Name())

			//Starting plugins
			client := plugin.NewClient(&plugin.ClientConfig{
				HandshakeConfig: handshakeConfig,
				Plugins:         map[string]plugin.Plugin{file.Name(): &shared2.MetricCollectorPlugin{}},
				Cmd:             exec.Command(PluginFolder + file.Name()),
				Logger:          logger,
			})
			defer client.Kill()
			logger.Info("Plugin loaded", "plugin", client)

			// Connect via RPC to the other side
			rpcClient, err := client.Client()
			if err != nil {
				logger.Error("Error starting RPC for", "plugin", client, "error", err)
			}
			rpcClientList = append(rpcClientList, &rpcClient)
			// Requesting the plugin
			plugin, err := rpcClient.Dispense(file.Name())
			if err != nil {
				logger.Error("Error loading remote plugin", "plugin", file.Name(), "error", err)
			}
			pluginList = append(pluginList, plugin)
		}
	}

	for {
		collectedData := make([]models.Metric, 0)
		for _, plugin := range pluginList {
			collector := plugin.(shared2.MetricCollector)
			collectedData = append(collectedData, collector.Collect()...)
		}
		PublishOnRedis(collectedData)

		select {
		case <-ctx.Done():
			logger.Info("Received interrupt signal, shutting down...")
			return
		default:
			logger.Info(fmt.Sprintf("Data collected and sent to redis. Waiting %d seconds before collecting data again...", scrapingInterval))
			time.Sleep(scrapingInterval)
		}
	}
}

func PublishOnRedis(metrics []models.Metric) {
	if !redisInitialized {
		InitializeRedis()
	}
	for _, metric := range metrics {
		jsonData, err := json.Marshal(metric)
		if err != nil {
			logrus.Println("Error serializing metric to JSON:", err)
		} else {
			logger.Trace("Serialized metric to JSON:", string(jsonData))
			err := redisClient.Publish(ctx, "metrics", jsonData).Err()
			if err != nil {
				logrus.Println("Error publishing message:", err)
			}
		}
	}

}

func InitializeRedis() {
	ctx = context.Background()
	redisClient = redis.NewClient(&redis.Options{
		Addr: configuration.RedisURI, // Redis server address
	})
	value, err := json.Marshal(moduleInfo)
	if err == nil {
		err = redisClient.Publish(ctx, "module", value).Err()
		if err != nil {
			logrus.Println("Error publishing message:", err)
		}
	}
	redisInitialized = true
}
