package main

import (
	"fmt"
	"github.com/free5gc/openapi/models"
	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"github.com/s2n-cnit/nwdaf/pkg/nrf"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

var (
	logger        = hclog.New(&hclog.LoggerOptions{Name: "NWDAF", Output: os.Stdout, Level: hclog.Debug})
	darchiverCmd  *exec.Cmd
	dcollectorCmd *exec.Cmd
	nrfClient     nrf.NRFClient
	nfID          uuid.UUID
)

func main() {
	configuration.LoadConfig()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	RegisterToNRF()

	startMicroservice("cmd/darchiver/build/darchiver", &darchiverCmd)
	startMicroservice("cmd/dcollector/build/dcollector", &dcollectorCmd)

	sig := <-sigChan
	terminate(sig)

	waitForMicroservice(darchiverCmd, "darchiver")
	waitForMicroservice(dcollectorCmd, "dcollector")
}

// startMicroservice starts a microservice as a separate process.
// It takes the path to the executable and a pointer to an exec.Cmd pointer.
// The function logs an error and exits the program if the process fails to start.
func startMicroservice(path string, cmd **exec.Cmd) {
	*cmd = exec.Command(path)
	(*cmd).Stdout = os.Stdout
	(*cmd).Stderr = os.Stderr
	if err := (*cmd).Start(); err != nil {
		log.Fatalf("Failed to start %s: %v", path, err)
	}
	logger.Info(fmt.Sprintf("%s started", path))
}

// waitForMicroservice waits for the specified microservice process to exit.
// It takes an exec.Cmd pointer representing the process and a string name of the microservice.
// If the process exits with an error, it logs the error. Otherwise, it logs that the process has exited.
func waitForMicroservice(cmd *exec.Cmd, name string) {
	if err := cmd.Wait(); err != nil {
		logger.Error(fmt.Sprintf("%s exited with error: %v", name, err))
	}
	logger.Info(fmt.Sprintf("%s exited", name))
}

func terminate(sig os.Signal) {
	logger.Info(fmt.Sprintf("Received signal: %v. Shutting down...", sig))
	killProcess(darchiverCmd, "darchiver")
	killProcess(dcollectorCmd, "dcollector")
	DeregisterFromNRF()
}

func killProcess(cmd *exec.Cmd, name string) {
	if err := cmd.Process.Kill(); err != nil {
		logger.Error(fmt.Sprintf("Failed to kill %s process", name), "error", err)
	}
}

func RegisterToNRF() {
	nfID = uuid.New()
	logger.Info(fmt.Sprintf("The function generated UUID is: %v", nfID.String()))
	nrfClient = nrf.NRFClient{NRFUri: configuration.NrfURI, Logger: logger}
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
