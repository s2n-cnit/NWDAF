package main

import (
	"encoding/json"
	freemodels "github.com/free5gc/openapi/models"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/s2n-cnit/nwdaf/plugin/shared"
	"net/http"
	"os"
	"strings"
)

var AmfSubURL = "http://192.168.254.193:31682/namf-evts/v1/subscriptions"
var AmfJsonSubBody = `{
 "subscription": {
   "eventNotifyUri": "http://192.168.254.11:5555",
   "nfId": "046b6c7f-0b8a-43b9-b35d-6489e6daee91",
   "eventList": [
   {
       "type": "LOCATION_REPORT",
			  "immediateFlag": true
   },
   {
       "type": "PRESENCE_IN_AOI_REPORT"
   },
   {
       "type": "TIMEZONE_REPORT"
   },
   {
       "type": "ACCESS_TYPE_REPORT",
			  "immediateFlag": true
   },
   {
       "type": "REGISTRATION_STATE_REPORT"
   },
   {
       "type": "CONNECTIVITY_STATE_REPORT"
   },
   {
       "type": "REACHABILITY_REPORT"
   },
   {
       "type": "SUBSCRIBED_DATA_REPORT"
   },
   {
       "type": "COMMUNICATION_FAILURE_REPORT"
   },
   {
       "type": "UES_IN_AREA_REPORT"
   },
   {
       "type": "SUBSCRIPTION_ID_CHANGE"
   },
   {
       "type": "SUBSCRIPTION_ID_ADDITION"
   }
   ],
   "subsChangeNotifyUri": "http://192.168.254.11:5555",
   "anyUE": true,
   "options": {
     "trigger": "ONE_TIME"
   },
   "notifyCorrelationId": "1010"
 }
}`

type F5GAmfCollector struct {
	logger hclog.Logger
}

var HandShakeConfigAmfCollector = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "NWDAF_PLUGIN_COOKIE_KEY",
	MagicCookieValue: "dsJha6J899JNjudayscn",
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
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	collector := &F5GAmfCollector{
		logger: logger,
	}

	// If run as a standalone program, collect metrics locally, if loaded as a plugin, serve the plugin
	if debug_locally {
		collector.Collect()
	} else {
		// pluginMap is the map of plugins we can dispense.
		var pluginMap = map[string]plugin.Plugin{
			"F5GC_amf_collector": &shared.MetricCollectorPlugin{Impl: collector},
		}
		logger.Info("Offered plugins: ", pluginMap)

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: HandShakeConfigAmfCollector,
			Plugins:         pluginMap,
		})
	}
}

func (g *F5GAmfCollector) Collect() []models.Metric {
	// Make a POST request to the AMF to subscribe to events (one time mode = polling)
	resp, err := http.Post(AmfSubURL, "application/json", strings.NewReader(AmfJsonSubBody))
	if err != nil {
		g.logger.Error("Error making POST request:", err)
	}
	// When the function terminate the close is called
	defer resp.Body.Close()

	g.logger.Debug("Response body:", resp.Body)

	var result freemodels.AmfCreatedEventSubscription
	err_dec := json.NewDecoder(resp.Body).Decode(&result)
	if err_dec != nil {
		g.logger.Error("Error decoding JSON response:", err)
	}

	// After response is decoded, build the metrics as a standard NWDAF model
	metrics_map := g.BuildMetrics(result)

	// Preallocate the list with the length of the map for better performance
	list := make([]models.Metric, 0, len(metrics_map))
	for _, value := range metrics_map {
		list = append(list, value)
	}

	return list
}

func (g *F5GAmfCollector) BuildMetrics(subscription freemodels.AmfCreatedEventSubscription) map[string]models.Metric {
	metricMap := make(map[string]models.Metric)
	reportList := subscription.ReportList
	g.logger.Debug("Subscription report list", "reportList", reportList)
	for _, report := range reportList {
		switch report.Type {
		case freemodels.AmfEventType_LOCATION_REPORT:
			g.UpdateLocationReport(&metricMap, report, subscription.Subscription.NfId)
		case freemodels.AmfEventType_ACCESS_TYPE_REPORT:
			g.UpdateAccessReport(&metricMap, report, subscription.Subscription.NfId)
		}
	}

	return metricMap
}

func (g *F5GAmfCollector) UpdateAccessReport(metricMap *map[string]models.Metric, report freemodels.AmfEventReport, nfID string) {
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
				Name:        key,
				Description: "Number of devices in 3GPP Access",
				NFid:        nfID,
				NFType:      "amf",
				Value:       1,
			}
		}
		mapValue[key] = value
	}
}

func (g *F5GAmfCollector) UpdateLocationReport(metricMap *map[string]models.Metric, report freemodels.AmfEventReport, nfID string) {

	tac := report.Location.NrLocation.Tai.Tac
	mapValue := *metricMap
	value, exists := mapValue[tac]
	if exists {
		g.logger.Trace("Incrementing number of devices in the same TAC")
		value.Value = value.Value + 1 // Increment the number of devices in the same TAC
	} else {
		g.logger.Trace("Creating new metric for TAC")
		value = models.Metric{
			Name:        "LR_TAC_" + tac,
			Description: "Number of devices in the same TAC",
			NFid:        nfID,
			NFType:      "amf",
			Value:       1,
		}
	}
	mapValue[tac] = value
}
