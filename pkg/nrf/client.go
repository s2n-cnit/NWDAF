// Package nrf provides the models for the Network Repository Function (NRF).
// This has been tested with the NRF in the free5gc project.
package nrf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/free5gc/openapi/models"
	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"net/http"
)

// NRFClient represents a client for interacting with the Network Repository Function (NRF).
type NRFClient struct {
	NRFIp  string       // IP address of the NRF
	Logger hclog.Logger // Logger for logging messages
}

// NRFResponse represents the response from the NRF.
type NRFResponse struct {
	validityPeriod int                // Validity period of the response
	nfInstances    []models.NfProfile // List of NF profiles
}

// RegisterToNRF registers the NF instance to the NRF.
//
// Parameters:
// - nfInstanceID: UUID of the NF instance
// - address: IP address of the NF instance
//
// Returns an error if the registration fails.
func (nrfClient *NRFClient) RegisterToNRF(nfInstanceID uuid.UUID, address models.IpAddress) error {
	profile := models.NfProfile{
		NfInstanceId:  nfInstanceID.String(),
		NfType:        "NWDAF",
		NfStatus:      "REGISTERED",
		Ipv4Addresses: []string{address.Ipv4Addr},
	}
	uri := fmt.Sprintf("http://%v/nnrf-nfm/v1/nf-instances/%v", nrfClient.NRFIp, nfInstanceID.String())

	reqBody, err := json.Marshal(profile)
	if err != nil {
		nrfClient.Logger.Error("Error marshalling profile:", err)
		return err
	}

	req, err := http.NewRequest(http.MethodPut, uri, bytes.NewBuffer(reqBody))
	if err != nil {
		nrfClient.Logger.Error("Error creating PUT request:", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		nrfClient.Logger.Error("Error making PUT request:", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		nrfClient.Logger.Error("Unexpected status code:", resp.StatusCode)
		return errors.New("Unexpected status code")
	}

	nrfClient.Logger.Info("Successfully registered to NRF")
	return nil
}

// DeregisterFromNRF deregisters the NF instance from the NRF.
//
// Parameters:
// - nfInstanceID: UUID of the NF instance
//
// Returns an error if the deregistration fails.
func (nrfClient *NRFClient) DeregisterFromNRF(nfInstanceID uuid.UUID) error {
	uri := fmt.Sprintf("http://%v/nnrf-nfm/v1/nf-instances/%v", nrfClient.NRFIp, nfInstanceID.String())
	req, err := http.NewRequest(http.MethodDelete, uri, nil)
	if err != nil {
		nrfClient.Logger.Error("Error creating DELETE request:", err)
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		nrfClient.Logger.Error("Error making PUT request:", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		nrfClient.Logger.Error("Unexpected status code:", resp.StatusCode)
		return err
	}

	nrfClient.Logger.Info("Successfully DE-registered from NRF")
	return nil
}

// GetNFInstances retrieves NF instances of the specified type from the NRF.
//
// Parameters:
// - nfType: Type of the NF instances to retrieve
//
// Returns a slice of NF profiles and an error if the retrieval fails.
func (nrfClient *NRFClient) GetNFInstances(nfType models.NfType) ([]models.NfProfile, error) {
	if nfType == "" {
		return nil, errors.New("nfType is empty")
	}
	uri := fmt.Sprintf("http://%v/nnrf-disc/v1/nf-instances?target-nf-type=%v&requester-nf-type=NWDAF", nrfClient.NRFIp, nfType)
	resp, err := http.Get(uri)
	if err != nil {
		nrfClient.Logger.Error("Error making GET request:", err)
	}
	defer resp.Body.Close()

	nrfClient.Logger.Trace("Response body:", resp.Body)

	var result NRFResponse
	err_dec := json.NewDecoder(resp.Body).Decode(&result)
	if err_dec != nil {
		nrfClient.Logger.Error("Error decoding JSON response:", err)
	}

	return result.nfInstances, nil
}
