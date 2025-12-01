/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	log "github.com/sirupsen/logrus"
)

/*
HTTP Client Abstraction Layer

This package provides a reusable HTTP client with built-in retry logic, exponential backoff,
and automatic JSON marshaling/unmarshaling. It wraps hashicorp/go-retryablehttp to provide:

1. Automatic retries on transient errors (network issues, 5xx errors, 429 rate limiting)
2. Exponential backoff with jitter between retries
3. JSON marshaling/unmarshaling handled automatically
4. Observability through structured logging
5. Context support for cancellation and timeouts

Usage Example:
	client := httpclient.NewClient(httpclient.Config{
		BaseURL:      "https://api.example.com",
		Timeout:      30 * time.Second,
		MaxRetries:   5,
		Headers: map[string]string{
			"Authorization": "Bearer token",
		},
	})

	// GET request
	var response MyResponseType
	statusCode, err := client.Get(ctx, "/api/resource", &response)

	// POST request
	payload := MyRequestType{...}
	statusCode, err := client.Post(ctx, "/api/resource", payload)

This abstraction allows control planes (YBM, YBA, etc.) to focus on business logic
while the HTTP layer handles all retry/timeout/error handling concerns.
*/

// Client is a wrapper around go-retryablehttp with automatic retry logic
// and built-in support for JSON marshaling/unmarshaling
type Client struct {
	retryClient *retryablehttp.Client
	baseURL     string
	headers     map[string]string
}

// NewClient creates a new HTTP client with retry capabilities
func NewClient(config Config) *Client {
	// Start with default config and merge provided values
	defaultCfg := DefaultConfig()
	if config.Timeout == 0 {
		config.Timeout = defaultCfg.Timeout
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = defaultCfg.MaxRetries
	}
	if config.RetryWaitMin == 0 {
		config.RetryWaitMin = defaultCfg.RetryWaitMin
	}
	if config.RetryWaitMax == 0 {
		config.RetryWaitMax = defaultCfg.RetryWaitMax
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = defaultCfg.MaxIdleConns
	}
	if config.IdleConnTimeout == 0 {
		config.IdleConnTimeout = defaultCfg.IdleConnTimeout
	}
	if config.TLSHandshakeTimeout == 0 {
		config.TLSHandshakeTimeout = defaultCfg.TLSHandshakeTimeout
	}

	// Create the underlying HTTP client with custom transport
	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        config.MaxIdleConns,
			IdleConnTimeout:     config.IdleConnTimeout,
			TLSHandshakeTimeout: config.TLSHandshakeTimeout,
		},
	}

	// Create retryable HTTP client
	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = httpClient
	retryClient.RetryMax = config.MaxRetries
	retryClient.RetryWaitMin = config.RetryWaitMin
	retryClient.RetryWaitMax = config.RetryWaitMax

	// Disable default logging from retryablehttp (added our own)
	retryClient.Logger = nil

	// Adding custom retry policy and log hook enables us to keep the logs in the format we want.
	// Add custom retry policy with logging
	retryClient.CheckRetry = customRetryPolicy

	// Add hooks for observability
	retryClient.RequestLogHook = requestLogHook

	return &Client{
		retryClient: retryClient,
		baseURL:     strings.TrimSuffix(config.BaseURL, "/"),
		headers:     config.Headers,
	}
}

// Get performs a GET request and unmarshals the JSON response into the provided response object
// Returns the HTTP status code and any error encountered
func (c *Client) Get(ctx context.Context, path string, response interface{}) (int, error) {
	url := c.buildURL(path)

	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create GET request: %w", err)
	}

	c.addHeaders(req)

	startTime := time.Now()
	resp, err := c.retryClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		log.Warnf("GET request failed: url=%s, duration=%s, error=%v", url, duration, err)
		return 0, fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Infof("GET request completed: url=%s, status=%d, duration=%s", url, resp.StatusCode, duration)

	// Check for non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Unmarshal JSON response if response object is provided
	if response != nil {
		if err := json.Unmarshal(body, response); err != nil {
			return resp.StatusCode, fmt.Errorf("failed to unmarshal JSON response: %w", err)
		}
	}

	return resp.StatusCode, nil
}

// Post performs a POST request with the given payload (marshaled as JSON)
// Returns the HTTP status code and any error encountered
func (c *Client) Post(ctx context.Context, path string, payload interface{}) (int, error) {
	return c.doJSONRequest(ctx, "POST", path, payload)
}

// Put performs a PUT request with the given payload (marshaled as JSON)
// Returns the HTTP status code and any error encountered
func (c *Client) Put(ctx context.Context, path string, payload interface{}) (int, error) {
	return c.doJSONRequest(ctx, "PUT", path, payload)
}

// doJSONRequest is a helper method to perform POST/PUT requests with JSON payload
func (c *Client) doJSONRequest(ctx context.Context, method, path string, payload interface{}) (int, error) {
	url := c.buildURL(path)

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("failed to create %s request: %w", method, err)
	}

	c.addHeaders(req)

	startTime := time.Now()
	resp, err := c.retryClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		log.Warnf("%s request failed: url=%s, duration=%s, error=%v", method, url, duration, err)
		return 0, fmt.Errorf("%s request failed: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Infof("%s request completed: url=%s, status=%d, duration=%s", method, url, resp.StatusCode, duration)

	// Check for non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return resp.StatusCode, nil
}

// buildURL combines baseURL and path
func (c *Client) buildURL(path string) string {
	if c.baseURL == "" {
		return path
	}
	return c.baseURL + "/" + strings.TrimPrefix(path, "/")
}

// addHeaders adds common headers to the request
func (c *Client) addHeaders(req *retryablehttp.Request) {
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}
	// Ensure Content-Type is set for JSON requests
	if req.Header.Get("Content-Type") == "" && (req.Method == "POST" || req.Method == "PUT") {
		req.Header.Set("Content-Type", "application/json")
	}
}

// customRetryPolicy determines if a request should be retried
// This wraps the default policy with additional logging
func customRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// Use default retry policy from go-retryablehttp
	shouldRetry, checkErr := retryablehttp.DefaultRetryPolicy(ctx, resp, err)

	if shouldRetry {
		reason := "unknown"
		if err != nil {
			reason = fmt.Sprintf("error: %v", err)
		} else if resp != nil {
			reason = fmt.Sprintf("status: %d", resp.StatusCode)
		}
		log.Debugf("Retrying request due to: %s", reason)
	}

	return shouldRetry, checkErr
}

// requestLogHook logs each request attempt for observability
func requestLogHook(logger retryablehttp.Logger, req *http.Request, attemptNum int) {
	if attemptNum == 0 {
		log.Infof("Attempting request: method=%s, url=%s", req.Method, req.URL.String())
	} else {
		log.Infof("Retrying request: attempt=%d, method=%s, url=%s", attemptNum+1, req.Method, req.URL.String())
	}
}
