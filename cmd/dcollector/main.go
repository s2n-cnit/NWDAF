package main

import (
	"context"
	"encoding/json"
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
	// pluginMap is the map of plugins we can dispense.
	pluginMap        = map[string]plugin.Plugin{"f5gcollector": &shared2.MetricCollectorPlugin{}}
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
)

func main() {
	configuration.LoadConfig()
	logger.SetLevel(hclog.Trace)
	// We're a host! Start by launching the plugin process.
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
		Cmd:             exec.Command("./plugin/BUILD_free5gc_collector_go"),
		Logger:          logger,
	})
	defer client.Kill()

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		log.Fatal(err)
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("f5gcollector")
	if err != nil {
		log.Fatal(err)
	}

	// We should have a Greeter now! This feels like a normal interface
	// implementation but is in fact over an RPC connection.
	collector := raw.(shared2.MetricCollector)
	collectedData := collector.Collect()

	PublishOnRedis(collectedData)
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
			err := redisClient.Publish(ctx, "metric", jsonData).Err()
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
