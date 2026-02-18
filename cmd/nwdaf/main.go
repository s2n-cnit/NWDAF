// Package main implements the Network Data Analytics Function (NWDAF) service.
//
// NWDAF is a 5G core network function that collects and analyzes data from various
// network slices. It manages multiple microservices for data collection and archiving,
// and integrates with the Network Repository Function (NRF) for service registration.
// The actual implementation starts the darchiver and dcollectors microservices, which

package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/free5gc/openapi/models"
	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/s2n-cnit/nwdaf/internal/web"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/nrf"
	"github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
)

var (
	// logger is the global logger for the NWDAF application.
	logger = hclog.New(&hclog.LoggerOptions{Name: "NWDAF", Output: os.Stdout, Level: hclog.Debug})
	// nrfClient is the client used to interact with the NRF.
	nrfClient nrf.ClientNRF
	// nfID is the unique identifier for the NF instance.
	nfID uuid.UUID
	// microservices holds the running microservices.
	microservices = map[string]*exec.Cmd{}
)

// setupReverseProxy creates a reverse proxy handler for a target service.
func setupReverseProxy(port int, serviceName string) http.Handler {
	targetURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	target, err := url.Parse(targetURL)
	if err != nil {
		logger.Error("Failed to parse target URL for reverse proxy", "error", err, "service", serviceName)
		os.Exit(1)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Add custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("Reverse proxy error", "error", err, "url", r.URL, "service", serviceName)
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "%s service unavailable", serviceName)
	}

	return proxy
}

// startHTTPServer starts the HTTP server with reverse proxies to data archiver and analytics engine.
func startHTTPServer(config *configuration.Config) {
	darchiverAPIPort := configuration.GetEnvInt(configuration.EnvDArchiverAPIPort, 8081)
	analyticsEngineAPIPort := configuration.GetEnvInt(configuration.EnvAnalyticsEngineAPIPort, 8084)
	prometheusPort := configuration.GetEnvInt(configuration.EnvPrometheusLocalPort, 2112)

	mux := http.NewServeMux()

	// Proxy /api/metrics* requests to data archiver
	mux.Handle("/api/metrics", setupReverseProxy(darchiverAPIPort, "Data Archiver"))

	// Proxy /api/computed-metrics* requests to analytics engine
	mux.Handle("/api/computed-metrics", setupReverseProxy(analyticsEngineAPIPort, "Analytics Engine"))
	mux.Handle("/api/computed-metrics/", setupReverseProxy(analyticsEngineAPIPort, "Analytics Engine"))

	// Proxy /api/plugins requests to analytics engine
	mux.Handle("/api/plugins", setupReverseProxy(analyticsEngineAPIPort, "Analytics Engine"))
	mux.Handle("/api/models", setupReverseProxy(analyticsEngineAPIPort, "Analytics Engine"))

	// Proxy /prometheus/metrics to Prometheus endpoint (data archiver)
	mux.Handle("/prometheus/metrics", setupReverseProxy(prometheusPort, "Prometheus"))

	// Serve web UI from web directory
	webHandler := web.GetHandler("web")
	mux.Handle("/", webHandler)
	logger.Info("Web dashboard available", "url", fmt.Sprintf("http://%s:%d", config.Server.BindIP, config.Server.Port))

	// Start server
	serverAddr := fmt.Sprintf("%s:%d", config.Server.BindIP, config.Server.Port)
	logger.Info("Starting NWDAF HTTP server", "address", serverAddr)

	go func() {
		if err := http.ListenAndServe(serverAddr, mux); err != nil {
			logger.Error("Failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()
}

// main is the entry point for the NWDAF application.
func main() {
	// Create a channel to receive signals syscall.SIGINT, syscall.SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Starting NWDAF, PID is ", os.Getpid())

	// Load environment variables and configuration
	configuration.LoadEnvWithLogger(logger, "NWDAF Main")
	configur, err := configuration.LoadConfig(nil)
	if err != nil {
		logger.Error("Error loading configuration", "error", err)
		os.Exit(1)
	}

	// RegisterToNRF(configur)

	// Start the data archiver microservice
	darchiverEnv := map[string]string{
		configuration.EnvRedisUri:            configur.Redis.URI,
		configuration.EnvPrometheusLocalPort: strconv.Itoa(configur.PrometheusPort),
	}
	// Pass optional environment variables
	if configur.Redis.Password != "" {
		darchiverEnv[configuration.EnvRedisPassword] = configur.Redis.Password
	}
	if apiPort := configuration.GetEnvIntNoDefault(configuration.EnvDArchiverAPIPort); apiPort != nil {
		darchiverEnv[configuration.EnvDArchiverAPIPort] = strconv.Itoa(*apiPort)
	}
	darchiverEnv[configuration.EnvLogLevel] = strconv.Itoa(int(configur.LogLevel))
	startMicroservice("cmd/darchiver/build/darchiver", darchiverEnv, false, "darchiver")

	// Start the analytics engine microservice
	analyticsEnv := map[string]string{
		configuration.EnvRedisUri:     configur.Redis.URI,
		configuration.EnvCoreType:     string(configur.CoreType),
		configuration.EnvLogLevel:     strconv.Itoa(int(configur.LogLevel)),
		configuration.EnvMetricPrefix: configur.CoreType.String() + "_",
	}
	if configur.Redis.Password != "" {
		analyticsEnv[configuration.EnvRedisPassword] = configur.Redis.Password
	}
	if apiPort := configuration.GetEnvIntNoDefault(configuration.EnvAnalyticsEngineAPIPort); apiPort != nil {
		analyticsEnv[configuration.EnvAnalyticsEngineAPIPort] = strconv.Itoa(*apiPort)
	}
	if expirationSeconds := configuration.GetEnvIntNoDefault(configuration.EnvMetricExpirationSeconds); expirationSeconds != nil {
		analyticsEnv[configuration.EnvMetricExpirationSeconds] = strconv.Itoa(*expirationSeconds)
	}
	startMicroservice("cmd/analytics_engine/build/analytics_engine", analyticsEnv, false, "analytics_engine")

	// Start monitoring slices
	StartMonitorSlices(configur)

	// Start HTTP server with reverse proxy
	startHTTPServer(configur)

	// Wait for a signal to terminate the program
	sig := <-sigChan
	terminate(sig)
}

// StartMonitorSlices starts monitoring for the configured slices.
func StartMonitorSlices(configur *configuration.Config) {
	for _, slice := range configur.Slices {
		logger.Info(fmt.Sprintf("Starting Monitoring for slice %s", slice.ID))
		coreTypeStr := configur.CoreType.String() + "_"
		// Creates a map of COMMON environment variables for each data collector microservice
		commonEnvMap := configuration.CommonEnv(configur.Redis.URI, configur.CoreType, configur.PrometheusPort, &configur.Redis.Password, &configur.LogLevel, &coreTypeStr)

		var envMap map[string]string
		switch configur.CoreType {
		case plugin_shared.CoreTypeHPE:
			envMap = plugin_shared.HPECoreEnv(slice.CoreEndpointIp, slice.Username, slice.Password, slice.ID)
		case plugin_shared.CoreTypeFree5GC:
			envMap = plugin_shared.Free5GCCoreEnv(slice.AmfIPs[0], slice.ID)
		case plugin_shared.CoreTypeFake:
			envMap = plugin_shared.FakeCoreEnv()
		default:
			logger.Error("Invalid core type", "core_type", configur.CoreType)
			os.Exit(1)
		}
		// Add common environment variables to the environment map
		for key, value := range commonEnvMap {
			envMap[key] = value
		}
		startMicroservice("cmd/dcollector/build/dcollector", envMap, false, "dcollectorslice1")
	}
}

// startMicroservice starts a microservice as a separate process.
// It takes the path to the executable, a map of environment variables, a boolean to inherit existing environment variables, and the name of the microservice.
func startMicroservice(path string, envVars map[string]string, inheritEnv bool, name string) *exec.Cmd {
	cmd := exec.Command(path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set environment variables
	if inheritEnv {
		cmd.Env = os.Environ()
	} // inherit existing environment variables
	for key, value := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	if err := (*cmd).Start(); err != nil {
		logger.Error(fmt.Sprintf("Failed to start %s: %v", path, err))
		terminate(syscall.SIGTERM)
	}

	microservices[name] = cmd
	logger.Info(fmt.Sprintf("%s started", path))

	return cmd
}

// waitForMicroservice waits for the specified microservice process to exit.
// It takes an exec.Cmd pointer representing the process and a string name of the microservice.
// If the process exits with an error, it logs the error. Otherwise, it logs that the process has exited.
func waitForMicroservice(cmd *exec.Cmd, name string) {
	if err := cmd.Wait(); err != nil {
		if err.Error() != "signal: killed" {
			logger.Warn(fmt.Sprintf("%s exited with error: %v", name, err))
		}
	}
	logger.Info(fmt.Sprintf("%s exited", name))
}

// terminate terminates the application and all running microservices.
// It takes an os.Signal and logs the shutdown process.
func terminate(sig os.Signal) {
	logger.Info(fmt.Sprintf("Received signal: %v. Shutting down...", sig))
	// Started killing all microservices
	for key, ms := range microservices {
		killProcess(ms, key)
	}
	// Wait for each microservice to be killed
	for key := range microservices {
		waitForMicroservice(microservices[key], key)
		delete(microservices, key)
	}
	DeregisterFromNRF()
	os.Exit(1)
}

// killProcess kills the specified process.
// It takes an exec.Cmd pointer representing the process and a string name of the microservice.
func killProcess(cmd *exec.Cmd, name string) {
	if err := cmd.Process.Kill(); err != nil {
		logger.Error(fmt.Sprintf("Failed to kill PID %s", name), "error", err)
	}
}

// RegisterToNRF registers the NF instance to the NRF.
// It takes a configuration.Config pointer and logs the registration process.
func RegisterToNRF(configur *configuration.Config) {
	nfID = uuid.New()
	logger.Info(fmt.Sprintf("The function generated UUID is: %v", nfID.String()))
	nrfClient = nrf.ClientNRF{NRFIp: configur.NRFIp, Logger: logger}
	if err := nrfClient.RegisterToNRF(nfID, models.IpAddress{Ipv4Addr: ""}); err != nil {
		logger.Error("Error registering to NRF", "error", err)
		os.Exit(1)
	}
}

// DeregisterFromNRF de-registers the NF instance from the NRF.
// It logs an error and exits the program if the de-registration fails.
func DeregisterFromNRF() {
	if err := nrfClient.DeregisterFromNRF(nfID); err != nil {
		logger.Error("Error DE-registering to NRF", "error", err)
		os.Exit(1)
	}
}
