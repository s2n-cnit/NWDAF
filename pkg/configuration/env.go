package configuration

import (
	"os"
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/joho/godotenv"
	"github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)

const EnvRedisUri = "REDIS_URI"
const EnvCoreType = "CORE_TYPE"
const EnvRedisPassword = "REDIS_PASSWORD"
const EnvPrometheusLocalPort = "PROMETHEUS_LOCAL_PORT"
const EnvLogLevel = "LOG_LEVEL"
const EnvMetricPrefix = "METRIC_PREFIX"
const EnvDArchiverAPIPort = "DARCHIVER_API_PORT"
const EnvAnalyticsEngineAPIPort = "ANALYTICS_ENGINE_API_PORT"
const EnvMetricExpirationSeconds = "METRIC_EXPIRATION_SECONDS"

var logger = hclog.New(&hclog.LoggerOptions{Name: "Environ", Output: os.Stdout, Level: hclog.Debug})

// CommonEnv returns a map of common environment variables used by the NWDAF modules
// The scope of this function is to provide a common set of environment variables that have a **standardized** name.
// The main function (NWDAF) will use this function to set the environment variables for the microservices that
// know how to read them because they will use the same **standardized** names.
func CommonEnv(redisUri string, coreType plugin_shared.CoreType, prometheusLocalPort int, redisPassword *string, logLevel *hclog.Level, metricPrefix *string) map[string]string {
	toReturn := map[string]string{
		EnvRedisUri:            redisUri,
		EnvCoreType:            string(coreType),
		EnvPrometheusLocalPort: strconv.Itoa(prometheusLocalPort),
	}
	if redisPassword != nil {
		toReturn[EnvRedisPassword] = *redisPassword
	}
	if logLevel != nil {
		toReturn[EnvLogLevel] = strconv.Itoa(int(*logLevel))
	}
	if metricPrefix != nil {
		toReturn[EnvMetricPrefix] = *metricPrefix
	}
	return toReturn
}

// LoadEnv loads environment variables from a .env file if it exists
func LoadEnv() {
	// Load .env file if using godotenv injecting environment variables
	err := godotenv.Load()
	if err != nil {
		logger.Error(".env file not found, environment variables should be set directly.")
	}
}

// SetEnv sets an environment variable
// If the variable already exists, it will be overwritten
//
// Parameters:
// - key: the name of the environment variable
// - value: the value of the environment variable
func SetEnv(key, value string) {
	err := os.Setenv(key, value)
	if err != nil {
		logger.Error("Error setting environment variable", key)
	}
}

// GetEnv reads an environment variable or returns a default value if not set
func GetEnv(key, defaultValue string) string {
	value, exists := GetEnvNoDefault(key)
	if !exists {
		return defaultValue
	}
	return value
}

// GetEnvNoDefault reads an environment variable or returns an empty string if not set
func GetEnvNoDefault(key string) (string, bool) {
	value, exists := os.LookupEnv(key)
	if !exists {
		//logger.Warn("Trying to load an environment variable that has not been set", key)
	}
	return value, exists
}

// GetEnvStrNoDefault reads an environment variable or returns nil if not set
func GetEnvStrNoDefault(key string) *string {
	value, exists := GetEnvNoDefault(key)
	if !exists {
		return nil
	}
	return &value
}

// GetEnvInt reads an environment variable, convert it to int, or returns a default value if not set.
// If the value is not an integer, it returns the default value.
func GetEnvInt(key string, defaultValue int) int {
	value := GetEnvIntNoDefault(key)
	if value == nil {
		return defaultValue
	}
	return *value
}

// GetEnvIntNoDefault reads an environment variable and convert it to int, or returns nil if not set.
// If the value is not an integer, it returns nil.
func GetEnvIntNoDefault(key string) *int {
	value, exists := os.LookupEnv(key)
	if !exists {
		// Note: Warning commented out to avoid interfering with plugin handshake
		// logger.Warn("Trying to load an environment variable that has not been set ", key)
		return nil
	}
	valueInt, err := strconv.Atoi(value)
	if err != nil {
		logger.Error("Error converting string to int", key)
		return nil
	}
	return &valueInt
}
