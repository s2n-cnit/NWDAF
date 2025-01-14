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

var postURL = "http://192.168.254.193:31682/namf-evts/v1/subscriptions"
var jsonBody = `{
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
       "type": "ACCESS_TYPE_REPORT"
   },
   {
       "type": "REGISTRATION_STATE_REPORT",
			  "immediateFlag": true
   },
   {
       "type": "CONNECTIVITY_STATE_REPORT",
			  "immediateFlag": true
   },
   {
       "type": "REACHABILITY_REPORT"
   },
   {
       "type": "SUBSCRIBED_DATA_REPORT",
			  "immediateFlag": true
   },
   {
       "type": "COMMUNICATION_FAILURE_REPORT"
   },
   {
       "type": "UES_IN_AREA_REPORT",
			  "immediateFlag": true
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

// Here is a real implementation of Greeter
type Free5GCollector struct {
	logger hclog.Logger
}

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "NWDAF_PLUGIN_COOCKIE_KEY",
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

	collector := &Free5GCollector{
		logger: logger,
	}

	if debug_locally {
		collector.Collect()
	} else {
		// pluginMap is the map of plugins we can dispense.
		var pluginMap = map[string]plugin.Plugin{
			"f5gcollector": &shared.MetricCollectorPlugin{Impl: collector},
		}

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: handshakeConfig,
			Plugins:         pluginMap,
		})
	}
}

func (g *Free5GCollector) Collect() []models.Metric {
	resp, err := http.Post(postURL, "application/json", strings.NewReader(jsonBody))
	if err != nil {
		g.logger.Error("Error making POST request:", err)
	}
	defer resp.Body.Close()

	g.logger.Debug("Response body:", resp.Body)

	var result freemodels.AmfCreatedEventSubscription
	err_dec := json.NewDecoder(resp.Body).Decode(&result)
	if err_dec != nil {
		g.logger.Error("Error decoding JSON response:", err)
	}

	metrics_map := g.BuildMetrics(result)

	// Preallocate the slice with the length of the map for better performance
	list := make([]models.Metric, 0, len(metrics_map))
	for _, value := range metrics_map {
		list = append(list, value)
	}

	return list
}

func (g *Free5GCollector) BuildMetrics(subscription freemodels.AmfCreatedEventSubscription) map[string]models.Metric {
	metricList := make(map[string]models.Metric)
	reportList := subscription.ReportList
	g.logger.Debug("Subscription report list", "reportList", reportList)
	for _, report := range reportList {
		switch report.Type {
		case freemodels.AmfEventType_LOCATION_REPORT:
			g.logger.Trace("BEFOR", metricList)
			g.UpdateLocationReport(&metricList, report, subscription.Subscription.NfId)
			g.logger.Trace("AFTER", metricList)
			//case freemodels.AmfEventType_PRESENCE_IN_AOI_REPORT:
		}
	}

	return metricList
}

func (g *Free5GCollector) UpdateLocationReport(metricMap *map[string]models.Metric, report freemodels.AmfEventReport, nfID string) {
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
