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
	"log"
	"os"
	"os/exec"
	"time"
)

const PluginFolder = "plugin/collectors/build/"

var (
	handshakeConfig = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "NWDAF_PLUGIN_COOKIE_KEY",
		MagicCookieValue: "dsJha6J899JNjudayscn",
	}
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
	rpcClientList           = []*plugin.ClientProtocol{}
	pluginList              = []interface{}{}
	scrapingIntervalSeconds = 60
	scrapingInterval        = time.Duration(scrapingIntervalSeconds) * time.Second
	redisInitialized        = false
)

func Start() {
	configuration.LoadConfig()
	logger.SetLevel(hclog.Trace)

	files, err := os.ReadDir(PluginFolder)
	if err != nil {
		log.Fatal(err)
	}

	for _, fileOrFolder := range files {
		if !fileOrFolder.IsDir() {
			client := LoadPlugin(fileOrFolder)
			defer func() {
				client.Kill()
			}()
		}
	}

	for {
		collectedData := []models.Metric{}
		for _, plugin := range pluginList {
			collector := plugin.(shared2.MetricCollector)
			collectedData = append(collectedData, collector.Collect()...)
		}
		PublishOnRedis(collectedData)

		logger.Info(fmt.Sprintf("Data collected and sent to redis. Waiting %d seconds before collecting data again...", scrapingIntervalSeconds))
		time.Sleep(scrapingInterval)
	}
}

func LoadPlugin(file os.DirEntry) *plugin.Client {
	logger.Info("Found plugin:", "file", file.Name())

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         map[string]plugin.Plugin{file.Name(): &shared2.MetricCollectorPlugin{}},
		Cmd:             exec.Command(PluginFolder + file.Name()),
		Logger:          logger,
	})
	logger.Info("Plugin loaded", "plugin", client)

	rpcClient, err := client.Client()
	if err != nil {
		logger.Error("Error starting RPC for", "plugin", client, "error", err)
	}
	rpcClientList = append(rpcClientList, &rpcClient)

	plugin, err := rpcClient.Dispense(file.Name())
	if err != nil {
		logger.Error("Error loading remote plugin", "plugin", file.Name(), "error", err)
	}
	pluginList = append(pluginList, plugin)
	return client
}

func PublishOnRedis(metrics []models.Metric) {
	if !redisInitialized {
		InitializeRedis()
	}
	for _, metric := range metrics {
		jsonData, err := json.Marshal(metric)
		if err != nil {
			log.Println("Error serializing metric to JSON:", err)
		} else {
			logger.Trace("Serialized metric to JSON:", string(jsonData))
			err := redisClient.Publish(ctx, "metrics", jsonData).Err()
			if err != nil {
				log.Println("Error publishing message:", err)
			}
		}
	}
}

func InitializeRedis() {
	ctx = context.Background()
	redisClient = redis.NewClient(&redis.Options{
		Addr: configuration.RedisURI,
	})
	value, err := json.Marshal(moduleInfo)
	if err == nil {
		err = redisClient.Publish(ctx, "module", value).Err()
		if err != nil {
			log.Println("Error publishing message:", err)
		}
	}
	redisInitialized = true
}

func main() {
	Start()
}
