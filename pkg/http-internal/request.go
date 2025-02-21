package http_internal

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"github.com/hashicorp/go-hclog"
	"net/http"
	"os"
)

var logger = hclog.New(&hclog.LoggerOptions{
	Name:       "HTTP_INTERNAL",
	Level:      hclog.Trace,
	Output:     os.Stderr,
	JSONFormat: true,
})

// RequestType represents the type of HTTP request.
type RequestType string

const (
	GET     RequestType = http.MethodGet
	POST    RequestType = http.MethodPost
	PUT     RequestType = http.MethodPut
	DELETE  RequestType = http.MethodDelete
	PATCH   RequestType = http.MethodPatch
	OPTIONS RequestType = http.MethodOptions
	HEAD    RequestType = http.MethodHead
)

// HttpRequestJsonBodyResp sends an HTTP request with a JSON body and returns the response.
//
// Parameters:
// - url: The URL to send the request to.
// - requestType: The type of HTTP request (GET, POST, PUT, DELETE, etc.).
// - body: A map containing the JSON body to be sent with the request.
// - bearerTokenPointer: A pointer to a string containing the bearer token for authorization (optional).
//
// Returns:
// - An interface{} containing the response body, which can be a JSON object or array.
// - An error if there was an issue with the request or response.
func HttpRequestJsonBodyResp(url string, requestType RequestType, body map[string]string, bearerTokenPointer *string) (interface{}, error) {
	// Encode the body map to JSON.
	encoded_body, err := json.Marshal(body)
	if err != nil {
		logger.Error("Error encoding JSON body:", err)
		return nil, err
	}

	// Create a new HTTP request with the specified type and URL.
	req, err := http.NewRequest(string(requestType), url, bytes.NewReader(encoded_body))
	if err != nil {
		logger.Error("Error creating POST request:", err)
		return nil, err
	}

	// Set the Content-Type header to application/json.
	req.Header.Set("Content-Type", "application/json")

	// If a bearer token is provided, set the Authorization header.
	if bearerTokenPointer != nil {
		req.Header.Set("Authorization", "Bearer "+*bearerTokenPointer)
	}

	// Create an HTTP client with a transport that skips TLS verification.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	// Send the HTTP request and get the response.
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Error making POST request:", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the response body into an interface{}.
	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Error("Error decoding JSON response:", err)
		return nil, err
	}

	// Return the decoded response and no error.
	return result, nil
}
