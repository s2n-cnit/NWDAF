package nrf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/free5gc/openapi/models"
	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/s2n-cnit/nwdaf/pkg/configuration"
	"net/http"
)

type NRFClient struct {
	NRFUri string
	Logger hclog.Logger
}

type NRFResponse struct {
	validityPeriod int
	nfInstances    []models.NfProfile
}

func (nrfClient *NRFClient) RegisterToNRF(nfInstanceID uuid.UUID, address models.IpAddress) error {
	profile := models.NfProfile{
		NfInstanceId:  nfInstanceID.String(),
		NfType:        "NWDAF",
		NfStatus:      "REGISTERED",
		Ipv4Addresses: []string{address.Ipv4Addr},
	}
	uri := fmt.Sprintf("http://%v/nnrf-nfm/v1/nf-instances/%v", configuration.NrfURI, nfInstanceID.String())

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

func (nrfClient *NRFClient) DeregisterFromNRF(nfInstanceID uuid.UUID) error {
	uri := fmt.Sprintf("http://%v/nnrf-nfm/v1/nf-instances/%v", configuration.NrfURI, nfInstanceID.String())
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

func (nrfClient *NRFClient) GetNFInstances(nfType models.NfType) ([]models.NfProfile, error) {
	//http://192.168.254.193:32147/nnrf-disc/v1/nf-instances?target-nf-type=AMF&requester-nf-type=NWDAF
	if nfType == "" {
		return nil, errors.New("nfType is empty")
	}
	uri := fmt.Sprintf("http://%v/nnrf-disc/v1/nf-instances?target-nf-type=%v&requester-nf-type=NWDAF", configuration.NrfURI, nfType)
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
