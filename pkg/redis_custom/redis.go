package redis_custom

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/go-hclog"
	"github.com/s2n-cnit/nwdaf/pkg/models"
)

func NewCustomClient(clientName string, loggingLevel hclog.Level, redisUri string, redisPwd *string, moduleInfo *models.Module) RedisClient {
	var redisPwdCopy *string
	if redisPwd != nil {
		pwd := strings.Clone(*redisPwd)
		redisPwdCopy = &pwd
	}
	client := RedisClient{
		clientName:  clientName,
		logger:      nil,
		loggerLevel: loggingLevel,
		redisUri:    redisUri,
		redisPwd:    redisPwdCopy,
		Ctx:         nil,
		moduleInfo:  moduleInfo,
	}
	client = client.initializeRedis()
	return client
}

type RedisClient struct {
	Client            *redis.Client
	clientInitialized bool
	clientName        string
	logger            hclog.Logger
	loggerLevel       hclog.Level
	redisUri          string
	redisPwd          *string
	Ctx               context.Context
	moduleInfo        *models.Module
}

// IsInitialized returns whether the Redis Client has been initialized.
func (client RedisClient) IsInitialized() bool {
	return client.clientInitialized
}

// InitializeRedis initializes the Redis Client and publishes module information.
//
// This function sets up the Redis Client with the configuration specified in the
// configuration package and publishes the module information to the "module" topic.
func (client RedisClient) initializeRedis() RedisClient {
	client.logger = hclog.New(&hclog.LoggerOptions{
		Name:   client.clientName,
		Output: os.Stdout,
		Level:  client.loggerLevel,
	})

	// Create a new context for Redis operations.
	client.Ctx = context.Background()
	// Create a new Redis Client with the specified options.
	client.Client = redis.NewClient(&redis.Options{
		Addr: client.redisUri,
	})
	client.logger.Debug("Redis client initialized on URI:", client.redisUri)
	// Set the Redis password if it is specified in the configuration.
	if client.redisPwd != nil {
		client.Client.Options().Password = *client.redisPwd
		client.logger.Debug("with password:", *client.redisPwd)
	}

	//Serialize the module information to JSON.
	if client.moduleInfo != nil {
		value, err := json.Marshal(*client.moduleInfo)
		if err == nil {
			// Publish the module information to the "module" topic.
			client.Publish("module", string(value))
		}
	}
	// Mark Redis as initialized.
	client.clientInitialized = true
	return client
}

func (client RedisClient) Publish(topic string, message string) {
	// Publish the message to the specified topic.
	err := client.Client.Publish(client.Ctx, topic, message).Err()
	if err != nil {
		client.logger.Error("Error publishing message:", err)
	}
}

// Subscribe pubsub := *redisClient.Client.Subscribe(Ctx, "metrics", "computedMetrics")
func (client RedisClient) Subscribe(topics ...string) *redis.PubSub {
	pubsub := client.Client.Subscribe(client.Ctx, topics...)
	return pubsub
}
