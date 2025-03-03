// Package plugin_shared contains shared environment variables names and structures.
package plugin_shared

// CoreType represents the type of 5G core network.
type CoreType string

const (
	// CoreTypeFree5GC represents the Free5GC core network type.
	CoreTypeFree5GC CoreType = "F5GC"
	// CoreTypeHPE represents the HPE core network type.
	CoreTypeHPE CoreType = "HPE"
	// CoreTypeOpen5GS represents the Open5GS core network type.
	CoreTypeOpen5GS CoreType = "O5GS"
	// CoreTypeOpenAirInterface represents the OpenAirInterface core network type.
	CoreTypeOpenAirInterface CoreType = "OAI"
)

// String method to convert the enum value to a string.
func (core CoreType) String() string {
	return string(core)
}

// ------------------------ Plugin Common ENV ----------------------

const EnvMagicCookieKeyName = "MAGIC_COOKIE_KEY"
const EnvMagicCookieKeyValue = "MAGIC_COOKIE_VALUE"

// ------------------------ HPE ----------------------

// Environment variable names for HPE core network.
const (
	EnvHpeCoreIp   = "HPE_CORE_IP"
	EnvHpeUsername = "HPE_USERNAME"
	EnvHpePassword = "HPE_PASSWORD"
)

// HPERequiredEnvVars lists the required environment variables for HPE plugins.
var HPERequiredEnvVars = []string{
	EnvHpeCoreIp,
	EnvHpeUsername,
	EnvHpePassword,
}

// HPECoreEnv returns a map of environment variables for the HPE core network.
//
// Parameters:
//   - coreIp: The IP address of the HPE core network.
//   - coreUsername: The username for the HPE core network.
//   - corePassword: The password for the HPE core network.
//   - coreMetricPrefix: The metric prefix for the HPE core network.
//
// Returns a map of environment variable names to their values.
func HPECoreEnv(coreIp string, coreUsername string, corePassword string, coreMetricPrefix string) map[string]string {
	return map[string]string{
		EnvHpeCoreIp:   coreIp,
		EnvHpeUsername: coreUsername,
		EnvHpePassword: corePassword,
	}
}

// ------------------------ Free5GC ----------------------

// Environment variable names for Free5GC core network.
const (
	EnvFree5GCAmfIp        = "FREE5GC_AMF_IP"
	EnvFree5GCMetricPrefix = "FREE5GC_METRIC_PREFIX"
)

// Free5GCAmfRequiredEnvVars lists the required environment variables for Free5GC plugins.
var Free5GCAmfRequiredEnvVars = []string{
	EnvFree5GCAmfIp,
}

// Free5GCCoreEnv returns a map of environment variables for the Free5GC core network.
//
//   - coreUsername: The username for the Free5GC core network.
//   - corePassword: The password for the Free5GC core network.
//
// Returns a map of environment variable names to their values.
func Free5GCCoreEnv(amfIp string, coreMetricPrefix string) map[string]string {
	return map[string]string{
		EnvFree5GCAmfIp:        amfIp,
		EnvFree5GCMetricPrefix: coreMetricPrefix,
	}
}
