package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	freemodels "github.com/free5gc/openapi/models"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)

// F5GAmfCollector is a collector for Free5GC AMF metrics.
type F5GAmfCollector struct {
	logger       hclog.Logger
	metricPrefix string
	amfIP        string
	amfSubURL    string
}

var (
	// HandShakeConfigAmfCollector is the handshake configuration for the AMF collector plugin.
	HandShakeConfigAmfCollector = plugin.HandshakeConfig{
		ProtocolVersion: 1,
	}

	metricPrefix = "NWDAF_"

	amfJsonSubBody string

	F5GCAmfCollector = &F5GAmfCollector{
		logger: hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Debug,
			Output:     os.Stderr,
			JSONFormat: true,
		}),
	}
)

func (collector *F5GAmfCollector) SetEnvironment() {
	configuration.LoadEnv()
	amfIPEnv := configuration.GetEnvStrNoDefault(plugin_shared.EnvFree5GCAmfIp)
	if amfIPEnv == nil {
		collector.logger.Error("Environment variable not set", "variable", plugin_shared.EnvFree5GCAmfIp)
		os.Exit(1)
	}

	collector.logger.SetLevel(hclog.Level(configuration.GetEnvInt(configuration.EnvLogLevel, int(hclog.Debug))))
	metricPrefix = configuration.GetEnv(configuration.EnvMetricPrefix, "NWDAF_")

	cookieName := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyName)
	cookieValue := configuration.GetEnvStrNoDefault(plugin_shared.EnvMagicCookieKeyValue)
	if cookieName == nil || cookieValue == nil {
		collector.logger.Error("Missing COOKIE name and value variables for RPC")
		os.Exit(1)
	}
	HandShakeConfigAmfCollector.MagicCookieKey = *cookieName
	HandShakeConfigAmfCollector.MagicCookieValue = *cookieValue

	F5GCAmfCollector.amfIP = *amfIPEnv
	F5GCAmfCollector.amfSubURL = "http://" + *amfIPEnv + ":31682/namf-evts/v1/subscriptions"
	F5GCAmfCollector.metricPrefix = metricPrefix
}

// main is the entry point for the F5GAmfCollector application.
func main() {
	F5GCAmfCollector.SetEnvironment()

	debug_locally := false
	args := os.Args[1:]
	for _, argument := range args {
		if argument == "--debug-locally" {
			debug_locally = true
		}
	}

	// Read the JSON template for the AMF subscription
	fileContent, err := os.ReadFile("plugin/collectors/templates/F5GC_amf_request.json")
	if err != nil {
		F5GCAmfCollector.logger.Error("Error reading file", "error", err)
	}
	amfJsonSubBody = string(fileContent)

	// If run as a standalone program, collect metrics locally, if loaded as a plugin, serve the plugin
	if debug_locally {
		F5GCAmfCollector.Collect()
	} else {
		// pluginMap is the map of plugins we can dispense.
		var pluginMap = map[string]plugin.Plugin{
			"F5GC_amf_collector": &plugin_shared.MetricCollectorPlugin{Impl: F5GCAmfCollector},
		}
		F5GCAmfCollector.logger.Info("Offered plugins: ", pluginMap)

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: HandShakeConfigAmfCollector,
			Plugins:         pluginMap,
		})
	}
}

// Collect collects metrics from the AMF and returns them as a list of NWDAF metrics.
func (collector *F5GAmfCollector) Collect() []models.Metric {

	// Make a POST request to the AMF to subscribe to events (one time mode = polling)
	resp, err := http.Post(collector.amfSubURL, "application/json", strings.NewReader(amfJsonSubBody))
	if err != nil {
		collector.logger.Error("Error making POST request:", err)
	}
	// When the function terminates, the close is called
	defer resp.Body.Close()

	collector.logger.Debug("Response body:", resp.Body)

	var result freemodels.AmfCreatedEventSubscription
	err_dec := json.NewDecoder(resp.Body).Decode(&result)
	if err_dec != nil {
		collector.logger.Error("Error decoding JSON response:", err)
	}

	// After response is decoded, build the metrics as a standard NWDAF model
	metrics_map := collector.BuildMetrics(result)

	// Preallocate the list with the length of the map for better performance
	list := make([]models.Metric, 0, len(metrics_map))
	for _, value := range metrics_map {
		list = append(list, value)
	}

	return list
}

// GetRequiredEnvVars returns the list of environment variables required by the plugin.
func (collector *F5GAmfCollector) GetRequiredEnvVars() []string {
	return plugin_shared.Free5GCAmfRequiredEnvVars
}

// BuildMetrics builds the metrics from the AMF subscription response.
// The metrics are built as a map of metrics, where the key is the metric name.
//
// Parameters:
// - subscription: The AMF subscription response
//
// Returns:
// - A map of metrics
func (collector *F5GAmfCollector) BuildMetrics(subscription freemodels.AmfCreatedEventSubscription) map[string]models.Metric {
	metricMap := make(map[string]models.Metric)
	reportList := subscription.ReportList
	collector.logger.Debug("Subscription report list", "reportList", reportList)
	for _, report := range reportList {
		switch report.Type {
		case freemodels.AmfEventType_LOCATION_REPORT:
			collector.UpdateLocationReport(&metricMap, report, subscription.Subscription.NfId)
		case freemodels.AmfEventType_ACCESS_TYPE_REPORT:
			collector.UpdateAccessReport(&metricMap, report, subscription.Subscription.NfId)
		}
	}

	return metricMap
}

// UpdateAccessReport updates the metrics map with access type report data.
//
// Parameters:
// - metricMap: The map of metrics to update
// - report: The AMF event report
// - nfID: The NF instance ID
func (collector *F5GAmfCollector) UpdateAccessReport(metricMap *map[string]models.Metric, report freemodels.AmfEventReport, nfID string) {
	accessTypesList := report.AccessTypeList
	for _, accessType := range accessTypesList {
		var key string
		switch accessType {
		case freemodels.AccessType__3_GPP_ACCESS:
			key = "F5GC_3GPP_ACCESS"
		case freemodels.AccessType_NON_3_GPP_ACCESS:
			key = "F5GC_NON_3GPP_ACCESS"
		}
		mapValue := *metricMap
		value, exists := mapValue[key]
		if exists {
			value.Value = value.Value + 1
		} else {
			value = models.Metric{
				Name:        collector.metricPrefix + key,
				Description: "Number of devices in 3GPP Access",
				NFid:        nfID,
				NFType:      "amf",
				Value:       1,
			}
		}
		mapValue[key] = value
	}
}

// UpdateLocationReport updates the metrics map with location report data.
//
// Parameters:
// - metricMap: The map of metrics to update
// - report: The AMF event report
// - nfID: The NF instance ID
func (collector *F5GAmfCollector) UpdateLocationReport(metricMap *map[string]models.Metric, report freemodels.AmfEventReport, nfID string) {
	tac := report.Location.NrLocation.Tai.Tac
	mapValue := *metricMap
	value, exists := mapValue[tac]
	if exists {
		collector.logger.Trace("Incrementing number of devices in the same TAC")
		value.Value = value.Value + 1 // Increment the number of devices in the same TAC
	} else {
		collector.logger.Trace("Creating new metric for TAC")
		value = models.Metric{
			Name:        collector.metricPrefix + "LR_TAC_" + tac,
			Description: "Number of devices in the same TAC",
			NFid:        nfID,
			NFType:      "amf",
			Value:       1,
		}
	}
	mapValue[tac] = value
}
