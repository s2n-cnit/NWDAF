package main

import (
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	http_internal "github.com/s2n-cnit/nwdaf/pkg/http-internal"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
	"os"
)

type HPEAmfCollector struct {
	logger       hclog.Logger
	coreIp       *string
	username     *string
	password     *string
	token        string
	metricPrefix string
}

var HandShakeConfigHPEAmfCollector = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "NWDAF_PLUGIN_COOKIE_KEY",
	MagicCookieValue: "dsJha6J899JNjudayscn",
}

func (collector *HPEAmfCollector) get_evn() {
	collector.coreIp = configuration.GetEnvStrNoDefault(plugin_shared.EnvHpeCoreIp)
	collector.username = configuration.GetEnvStrNoDefault(plugin_shared.EnvHpeUsername)
	collector.password = configuration.GetEnvStrNoDefault(plugin_shared.EnvHpePassword)
	collector.metricPrefix = configuration.GetEnv(configuration.EnvMetricPrefix, "NWDAF_HPE_")
	if collector.coreIp == nil || collector.username == nil || collector.password == nil {
		collector.logger.Error("Missing HPE environment variables")
		os.Exit(1)
	}
}

func (collector *HPEAmfCollector) BuildMetric(name string, description string, value float64) models.Metric {
	return models.Metric{Name: fmt.Sprintf("%v%v", collector.metricPrefix, name), Description: description, Value: value}
}

// Login authenticates the collector with the HPE core system and retrieves an access token.
func (collector *HPEAmfCollector) Login() {
	// Construct the login URL using the core IP address.
	url := fmt.Sprintf("https://%v/core/pls/api/1/auth/login", *collector.coreIp)

	// Create the request body with the username and password.
	body := map[string]string{
		"username": *collector.username,
		"password": *collector.password,
	}

	// Send the HTTP request and get the response.
	resp, err := http_internal.HttpRequestJsonBodyResp(url, http_internal.POST, body, nil)

	// Type assert the response to a map.
	castedValue := resp.(map[string]interface{})

	// Check for errors in the HTTP request.
	if err != nil {
		collector.logger.Error("Error logging in:", err)
	} else {
		collector.logger.Debug("Response JSON:", resp)

		// Extract the access token from the response and store it in the collector.
		collector.token = castedValue["access_token"].(string)
	}
}

func (collector *HPEAmfCollector) Collect() []models.Metric {
	supiNumMetric := collector.CollectSupiInfo()

	if supiNumMetric == nil {
		collector.logger.Error("SUPI empty info from AMF")
		return []models.Metric{}
	}

	return supiNumMetric
}

func (collector *HPEAmfCollector) GetRequiredEnvVars() []string {
	return plugin_shared.HPERequiredEnvVars
}

func (collector *HPEAmfCollector) CollectSupiInfo() []models.Metric {
	url := fmt.Sprintf("https://%v/core/amf/api/1/ue/status/supis", *collector.coreIp)

	resp, err := http_internal.HttpRequestJsonBodyResp(url, http_internal.GET, nil, &collector.token)

	if err != nil {
		collector.logger.Error("Error getting SUPIs:", err)
		return nil
	} else {
		castedValue := resp.([]interface{})
		SupiNumMetric := collector.BuildMetric("SUPI_NUM", "Number of SUPIs", float64(len(castedValue)))
		registeredDevices := collector.BuildMetric("REGIS_DEV", "Number of registered devices", 0)
		connectedDevices := collector.BuildMetric("CONN_DEV", "Number of connected devices", 0)

		// TODO location info can be retrieved from the SUPIs
		collector.logger.Debug("Getting SUPIs from HPE AMF")
		for _, supi := range castedValue {
			url := fmt.Sprintf("https://%v/core/amf/api/1/ue/status/supis/%v", *collector.coreIp, supi)
			resp, err := http_internal.HttpRequestJsonBodyResp(url, http_internal.GET, nil, &collector.token)
			if err != nil {
				collector.logger.Error("Error getting SUPI info:", err)
			} else {
				supiInfo := resp.(map[string]interface{})
				if supiInfo["mmState"] == "registered" {
					registeredDevices.Value++
				}
				if supiInfo["cmState"] == "connected" {
					connectedDevices.Value++
				}
			}
		}
		return []models.Metric{registeredDevices, connectedDevices, SupiNumMetric}
	}
}

func main() {
	debug_locally := false
	args := os.Args[1:]
	for _, argument := range args {
		if argument == "--debug-locally" {
			debug_locally = true
		}
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:       "HPE_AMF_COLLECTOR",
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	collector := &HPEAmfCollector{
		logger: logger,
	}

	collector.get_evn()
	collector.Login()

	if debug_locally {
		collector.GetRequiredEnvVars()
		collector.Collect()
	} else {
		// pluginMap is the map of plugins we can dispense.
		var pluginMap = map[string]plugin.Plugin{
			"HPE_amf_collector": &plugin_shared.MetricCollectorPlugin{Impl: collector},
		}
		logger.Info("Offered plugins: ", pluginMap)

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: HandShakeConfigHPEAmfCollector,
			Plugins:         pluginMap,
		})
	}
}
