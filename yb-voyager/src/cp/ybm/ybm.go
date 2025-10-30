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
package ybm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	controlPlane "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
)

const (
	// PAYLOAD_VERSION is the version of the payload format sent to YBM API
	PAYLOAD_VERSION = "1.0"
)

// isRetryableError determines if an error should be retried based on the error message
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Don't retry client errors (4xx except 429)
	if strings.Contains(errStr, "400") || // Bad Request - validation error
		strings.Contains(errStr, "401") || // Unauthorized - invalid API key
		strings.Contains(errStr, "403") { // Forbidden - no permissions
		return false
	}

	// Retry rate limiting (429) and server errors (5xx)
	if strings.Contains(errStr, "429") || // Rate limited - retry with backoff
		strings.Contains(errStr, "500") || // Internal Server Error
		strings.Contains(errStr, "502") || // Bad Gateway
		strings.Contains(errStr, "503") || // Service Unavailable
		strings.Contains(errStr, "504") { // Gateway Timeout
		return true
	}

	// Retry network errors (connection refused, timeout, etc.)
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable") {
		return true
	}

	// Default: retry unknown errors (conservative approach)
	return true
}

type YBM struct {
	sync.Mutex
	config                   *YBMConfig                    // The YBM configuration
	httpClient               *http.Client                  // HTTP client to send requests to YBM API
	voyagerInfo              *controlPlane.VoyagerInstance // The Voyager instance information
	migrationDirectory       string                        // The directory where the migration is being done
	waitGroup                sync.WaitGroup                // Wait group to track pending events
	eventChan                chan MigrationEvent           // Buffered channel to send migration events
	rowCountUpdateEventChan  chan []TableMetrics           // Buffered channel to send table metrics updates in batches
	lastRowCountUpdate       map[string]time.Time          // Tracks last update time per table to throttle row count updates (limit to 1 update per 5 seconds per table)
	latestInvocationSequence int                           // Tracks the latest invocation sequence for a migration phase to avoid duplicates
}

// New creates a new YBM control plane instance
func New(exportDir string, config *YBMConfig) *YBM {
	vi := controlPlane.PrepareVoyagerInstance(exportDir)
	return &YBM{
		config:             config,
		voyagerInfo:        vi,
		migrationDirectory: exportDir,
	}
}

// Init initializes the YBM control plane
func (ybm *YBM) Init() error {
	// Validate configuration
	if err := ybm.validateConfig(); err != nil {
		return err
	}

	// Create buffered channels
	ybm.eventChan = make(chan MigrationEvent, 100)
	ybm.rowCountUpdateEventChan = make(chan []TableMetrics, 200)

	// Initialize HTTP client with timeout and connection pooling
	// - Timeout (30s): Overall request timeout; prevents hanging on slow YBM responses while allowing large payload transmission
	// - MaxIdleConns (10): Supports burst traffic during heavy migrations (100s of tables) and retry attempts; connection reuse is ~100x faster than creating new TCP+TLS connections
	// - IdleConnTimeout (90s): Keeps connections alive long enough for typical migration patterns with gaps between phases
	// - TLSHandshakeTimeout (10s): Sufficient for HTTPS handshake under normal network conditions; prevents hanging on slow/bad SSL
	ybm.httpClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	// Initialize state
	ybm.lastRowCountUpdate = make(map[string]time.Time)
	ybm.latestInvocationSequence = 0

	// Start background goroutines
	go ybm.eventPublisher()
	go ybm.rowCountUpdateEventPublisher()

	log.Infof("YBM control plane initialized for domain: %s", ybm.config.Domain)
	return nil
}

// validateConfig ensures all required configuration is present
func (ybm *YBM) validateConfig() error {
	if ybm.config == nil {
		return fmt.Errorf("YBM configuration is nil")
	}
	if ybm.config.Domain == "" {
		return fmt.Errorf("YBM domain is required")
	}
	if ybm.config.AccountID == "" {
		return fmt.Errorf("YBM account-id is required")
	}
	if ybm.config.ProjectID == "" {
		return fmt.Errorf("YBM project-id is required")
	}
	if ybm.config.ClusterID == "" {
		return fmt.Errorf("YBM cluster-id is required")
	}
	if ybm.config.APIKey == "" {
		return fmt.Errorf("YBM api-key is required")
	}
	return nil
}

// Finalize waits for all pending events to be sent and cleans up
func (ybm *YBM) Finalize() {
	ybm.waitGroup.Wait()
	log.Infof("YBM control plane finalized")
}

// eventPublisher runs in a goroutine and publishes migration events
func (ybm *YBM) eventPublisher() {
	defer ybm.panicHandler()
	for {
		event := <-ybm.eventChan
		err := ybm.sendMigrationEvent(event)
		if err != nil {
			log.Warnf("Couldn't send metadata for visualization to YBM. %s", err)
		}
		ybm.waitGroup.Done()
	}
}

// rowCountUpdateEventPublisher runs in a goroutine and publishes table metrics
func (ybm *YBM) rowCountUpdateEventPublisher() {
	defer ybm.panicHandler()
	for {
		events := <-ybm.rowCountUpdateEventChan
		err := ybm.sendTableMetrics(events)
		if err != nil {
			log.Warnf("Couldn't send table metrics for visualization to YBM. %s", err)
		}
		ybm.waitGroup.Done()
	}
}

// panicHandler recovers from panics in goroutines
func (ybm *YBM) panicHandler() {
	if r := recover(); r != nil {
		log.Errorf(fmt.Sprintf("Panic occurred in YBM event publisher: %v. No further events will be published to YBM.\n"+
			"Stack trace of panic location:\n%s", r, string(debug.Stack())))
	}
}

// createAndSendEvent builds and queues a migration event
func (ybm *YBM) createAndSendEvent(event *controlPlane.BaseEvent, status string, payload string) {
	timestamp := time.Now().Format(time.RFC3339)

	invocationSequence, err := ybm.getInvocationSequence(event.MigrationUUID,
		controlPlane.MIGRATION_PHASE_MAP[event.EventType])
	if err != nil {
		log.Warnf("Unable to get invocation sequence for migration event. Cannot send event to YBM. %s", err)
		return
	}

	// Build host_ip JSON string (same format as yugabyted)
	jsonData := make(map[string]string)
	if controlPlane.IsExportPhase(event.EventType) {
		jsonData["SourceDBIP"] = strings.Join(event.DBIP, "|")
	} else if controlPlane.IsImportPhase(event.EventType) {
		jsonData["TargetDBIP"] = strings.Join(event.DBIP, "|")
	}

	dbIps, err := json.Marshal(jsonData)
	if err != nil {
		log.Warnf("failed to marshal host_ip to JSON format: %s", err)
	}

	// Convert payload string to structured map
	payloadMap := make(map[string]interface{})
	if payload != "" {
		// Try to unmarshal if it's JSON, otherwise leave empty
		if err := json.Unmarshal([]byte(payload), &payloadMap); err != nil {
			log.Debugf("Payload is not JSON, sending empty payload object: %v", err)
			// Keep payloadMap as empty map
		}
	}

	migrationEvent := MigrationEvent{
		MigrationUUID:       event.MigrationUUID,
		MigrationPhase:      controlPlane.MIGRATION_PHASE_MAP[event.EventType],
		InvocationSequence:  invocationSequence,
		DatabaseName:        event.DatabaseName,
		SchemaName:          strings.Join(event.SchemaNames, "|"),
		HostIP:              string(dbIps), // JSON string format: '{"SourceDBIP":"..."}'  or '{"TargetDBIP":"..."}'
		Port:                event.Port,
		DBVersion:           event.DBVersion,
		Payload:             payloadMap,
		PayloadVersion:      PAYLOAD_VERSION,
		VoyagerClientInfo:   *ybm.voyagerInfo,
		DBType:              event.DBType,
		Status:              status,
		InvocationTimestamp: timestamp,
		MigrationDirectory:  ybm.migrationDirectory,
	}

	select {
	case ybm.eventChan <- migrationEvent:
		ybm.waitGroup.Add(1)
	default:
		log.Warnf("Could not publish migration event to YBM (channel full) %v", migrationEvent)
	}
}

// createAndSendUpdateRowCountEvent builds and queues table metrics events
func (ybm *YBM) createAndSendUpdateRowCountEvent(events []*controlPlane.BaseUpdateRowCountEvent) {
	timestamp := time.Now().Format(time.RFC3339)

	var tableMetricsList []TableMetrics

	for _, event := range events {
		metrics := TableMetrics{
			MigrationUUID:       event.MigrationUUID,
			TableName:           event.TableName,
			SchemaName:          strings.Join(event.SchemaNames, "|"),
			MigrationPhase:      controlPlane.MIGRATION_PHASE_MAP[event.EventType],
			Status:              controlPlane.UPDATE_ROW_COUNT_STATUS_STR_TO_INT[event.Status],
			CountLiveRows:       event.CompletedRowCount,
			CountTotalRows:      event.TotalRowCount,
			InvocationTimestamp: timestamp,
		}
		tableMetricsList = append(tableMetricsList, metrics)
	}

	select {
	case ybm.rowCountUpdateEventChan <- tableMetricsList:
		ybm.waitGroup.Add(1)
	default:
		log.Warnf("Could not publish row count update event to YBM (channel full) %v", tableMetricsList)
	}
}

// getInvocationSequence retrieves the next invocation sequence for a migration phase
func (ybm *YBM) getInvocationSequence(migrationUUID uuid.UUID, phase int) (int, error) {
	// Optimization: If already initialized, just increment
	if ybm.latestInvocationSequence > 0 {
		ybm.latestInvocationSequence += 1
		return ybm.latestInvocationSequence, nil
	}

	// Query YBM API for max sequence
	maxSeq, err := ybm.getMaxSequenceFromAPI(migrationUUID, phase)
	if err != nil {
		log.Warnf("Failed to get max sequence from YBM API: %v. Falling back to sequence 1", err)
		ybm.latestInvocationSequence = 1
		return ybm.latestInvocationSequence, nil
	}

	ybm.latestInvocationSequence = maxSeq + 1
	return ybm.latestInvocationSequence, nil
}

// OLD IMPLEMENTATION - COMMENTED OUT
// This was the original implementation using /voyager/metadata/max-sequence endpoint
// Replaced with getMaxSequenceFromAPI which uses /voyager/migrations endpoint
/*
func (ybm *YBM) getMaxSequenceFromAPI_OLD(migrationUUID uuid.UUID, phase int) (int, error) {
	urlPath := fmt.Sprintf("%s/api/public/v1/accounts/%s/projects/%s/clusters/%s/voyager/metadata/max-sequence",
		ybm.config.Domain, ybm.config.AccountID, ybm.config.ProjectID, ybm.config.ClusterID)

	params := url.Values{}
	params.Add("migration_uuid", migrationUUID.String())
	params.Add("migration_phase", fmt.Sprintf("%d", phase))

	fullURL := urlPath + "?" + params.Encode()

	var maxAttempts = 5
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var response MaxSequenceResponse
		err := ybm.getJSON(fullURL, &response)
		if err == nil {
			log.Infof("YBM API returned max sequence: %d", response.MaxInvocationSequence)
			return response.MaxInvocationSequence, nil
		}

		// Check if it's a 404 (endpoint not implemented yet or no prior data)
		// TODO: Remove this once the endpoint is implemented
		if strings.Contains(err.Error(), "404") {
			log.Infof("GET max-sequence endpoint returned 404, using sequence 0 (first run or endpoint not available)")
			return 0, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			log.Warnf("Non-retryable error getting max sequence from YBM: %v. Falling back to sequence 0", err)
			return 0, nil
		}

		log.Warnf("Attempt %d/%d failed to get max sequence from YBM: %v", attempt, maxAttempts, err)

		if attempt < maxAttempts {
			// Exponential backoff: 1s, 2s, 4s, 8s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			time.Sleep(backoff)
		}
	}

	// After all retries failed, log warning and fallback to sequence 0
	log.Warnf("Failed to get max sequence from YBM after %d attempts: %v. Falling back to sequence 0", maxAttempts, lastErr)
	return 0, nil
}
*/

// getMaxSequenceFromAPI calls the GET /voyager/migrations endpoint with retries
// This endpoint returns the latest migration entry which contains the max invocation_sequence
func (ybm *YBM) getMaxSequenceFromAPI(migrationUUID uuid.UUID, phase int) (int, error) {
	urlPath := fmt.Sprintf("%s/api/public/v1/accounts/%s/projects/%s/clusters/%s/voyager/migrations",
		ybm.config.Domain, ybm.config.AccountID, ybm.config.ProjectID, ybm.config.ClusterID)

	params := url.Values{}
	params.Add("migrationId", migrationUUID.String())
	params.Add("migration_phase", fmt.Sprintf("%d", phase))

	fullURL := urlPath + "?" + params.Encode()

	var maxAttempts = 5
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var response MigrationListResponse
		err := ybm.getJSON(fullURL, &response)
		if err == nil {
			// Check if data array is empty (no prior migrations for this phase)
			if len(response.Data) == 0 {
				log.Infof("YBM API returned empty migration list for phase %d, using sequence 0 (first run)", phase)
				return 0, nil
			}

			// The API returns the latest entry, extract invocation_sequence
			maxSeq := response.Data[0].InvocationSequence
			log.Infof("YBM API returned max sequence: %d for phase %d", maxSeq, phase)
			return maxSeq, nil
		}

		// Check if it's a 404 (no prior data)
		if strings.Contains(err.Error(), "404") {
			log.Infof("GET migrations endpoint returned 404, using sequence 0 (first run or no data)")
			return 0, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			log.Warnf("Non-retryable error getting max sequence from YBM: %v. Falling back to sequence 0", err)
			return 0, nil
		}

		log.Warnf("Attempt %d/%d failed to get max sequence from YBM: %v", attempt, maxAttempts, err)

		if attempt < maxAttempts {
			// Exponential backoff: 1s, 2s, 4s, 8s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			time.Sleep(backoff)
		}
	}

	// After all retries failed, log warning and fallback to sequence 0
	log.Warnf("Failed to get max sequence from YBM after %d attempts: %v. Falling back to sequence 0", maxAttempts, lastErr)
	return 0, nil
}

// sendMigrationEvent sends a migration event to YBM API with retries
func (ybm *YBM) sendMigrationEvent(event MigrationEvent) error {
	urlPath := fmt.Sprintf("%s/api/public/v1/accounts/%s/projects/%s/clusters/%s/voyager/metadata",
		ybm.config.Domain, ybm.config.AccountID, ybm.config.ProjectID, ybm.config.ClusterID)

	var maxAttempts = 5
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := ybm.postJSON(urlPath, event)
		if err == nil {
			log.Infof("Successfully sent migration event to YBM: phase=%d, sequence=%d, status=%s",
				event.MigrationPhase, event.InvocationSequence, event.Status)
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			log.Warnf("Non-retryable error sending migration event to YBM: %v", err)
			return err
		}

		log.Warnf("Attempt %d/%d failed to send migration event to YBM: %v", attempt, maxAttempts, err)

		if attempt < maxAttempts {
			// Exponential backoff: 1s, 2s, 4s, 8s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			time.Sleep(backoff)
			// Update timestamp for retry
			event.InvocationTimestamp = time.Now().Format(time.RFC3339)
		}
	}

	return fmt.Errorf("failed to send migration event after %d attempts: %w", maxAttempts, lastErr)
}

// sendTableMetrics sends table metrics to YBM API with retries
func (ybm *YBM) sendTableMetrics(metricsList []TableMetrics) error {
	urlPath := fmt.Sprintf("%s/api/public/v1/accounts/%s/projects/%s/clusters/%s/voyager/table-metrics",
		ybm.config.Domain, ybm.config.AccountID, ybm.config.ProjectID, ybm.config.ClusterID)

	var maxAttempts = 5
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Send each metric individually (YBM API expects single metric per PUT)
		allSuccess := true
		for _, metrics := range metricsList {
			err := ybm.putJSON(urlPath, metrics)
			if err != nil {
				lastErr = err
				allSuccess = false

				// Check if error is retryable
				if !isRetryableError(err) {
					log.Warnf("Non-retryable error sending table metrics for %s: %v", metrics.TableName, err)
					return err
				}

				log.Warnf("Attempt %d/%d failed to send table metrics for %s: %v",
					attempt, maxAttempts, metrics.TableName, err)
				break
			}
		}

		if allSuccess {
			log.Infof("Successfully sent %d table metrics to YBM", len(metricsList))
			return nil
		}

		if attempt < maxAttempts {
			// Exponential backoff
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			time.Sleep(backoff)
			// Update timestamps for retry
			timestamp := time.Now().Format(time.RFC3339)
			for i := range metricsList {
				metricsList[i].InvocationTimestamp = timestamp
			}
		}
	}

	return fmt.Errorf("failed to send table metrics after %d attempts: %w", maxAttempts, lastErr)
}

// postJSON sends a POST request with JSON payload
func (ybm *YBM) postJSON(urlPath string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequest("POST", urlPath, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+ybm.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.Debugf("Calling YBM API POST: %s with payload: %s", urlPath, string(jsonData))

	_, _, err = ybm.executeHTTPRequest(req, "POST")
	return err
}

// putJSON sends a PUT request with JSON payload
func (ybm *YBM) putJSON(urlPath string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequest("PUT", urlPath, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+ybm.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.Debugf("Calling YBM API PUT: %s with payload: %s", urlPath, string(jsonData))

	_, _, err = ybm.executeHTTPRequest(req, "PUT")
	return err
}

// getJSON sends a GET request and decodes the JSON response
func (ybm *YBM) getJSON(urlPath string, response interface{}) error {
	req, err := http.NewRequest("GET", urlPath, nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+ybm.config.APIKey)
	req.Header.Set("Accept", "application/json")

	log.Debugf("Calling YBM API GET: %s", urlPath)

	body, statusCode, err := ybm.executeHTTPRequest(req, "GET")
	if err != nil {
		return err
	}

	// Success - decode response
	if statusCode == 200 {
		if err := json.Unmarshal(body, response); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// executeHTTPRequest executes an HTTP request and handles common response logic
// Returns: response body, status code, error
func (ybm *YBM) executeHTTPRequest(req *http.Request, method string) ([]byte, int, error) {
	resp, err := ybm.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%s request failed: %w", method, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Log response for debugging
	log.Debugf("YBM API %s response: Status=%d, Body=%s", method, resp.StatusCode, string(body))

	// Handle different status codes
	switch resp.StatusCode {
	case 200, 201:
		// Success
		return body, resp.StatusCode, nil
	case 400:
		// Bad request - could be duplicate metadata entry or validation error
		return body, resp.StatusCode, fmt.Errorf("bad request (400): %s", string(body))
	case 401:
		return body, resp.StatusCode, fmt.Errorf("authentication failed (401): invalid API key") // Using the widely accepted error message for 401
	case 403:
		return body, resp.StatusCode, fmt.Errorf("permission denied (403): API key doesn't have access to this resource") // Using the widely accepted error message for 403
	case 404:
		return body, resp.StatusCode, fmt.Errorf("resource not found (404): %s", string(body))
	case 429:
		return body, resp.StatusCode, fmt.Errorf("rate limit exceeded (429): too many requests") // Using the widely accepted error message for 429
	case 500, 502, 503, 504:
		return body, resp.StatusCode, fmt.Errorf("YBM server error (%d): %s", resp.StatusCode, string(body))
	default:
		return body, resp.StatusCode, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}
}

// ControlPlane interface implementation

func (ybm *YBM) MigrationAssessmentStarted(ev *controlPlane.MigrationAssessmentStartedEvent) {
	ybm.createAndSendEvent(&ev.BaseEvent, "IN PROGRESS", "")
}

func (ybm *YBM) MigrationAssessmentCompleted(ev *controlPlane.MigrationAssessmentCompletedEvent) {
	ybm.createAndSendEvent(&ev.BaseEvent, "COMPLETED", ev.Report)
}

func (ybm *YBM) ExportSchemaStarted(event *controlPlane.ExportSchemaStartedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", "")
}

func (ybm *YBM) ExportSchemaCompleted(event *controlPlane.ExportSchemaCompletedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "COMPLETED", "")
}

func (ybm *YBM) SchemaAnalysisStarted(event *controlPlane.SchemaAnalysisStartedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", "")
}

func (ybm *YBM) SchemaAnalysisIterationCompleted(event *controlPlane.SchemaAnalysisIterationCompletedEvent) {
	jsonBytes, err := json.Marshal(event.AnalysisReport)
	if err != nil {
		log.Warnf("Failed to marshal analysis report: %v", err)
		return
	}
	payload := string(jsonBytes)
	ybm.createAndSendEvent(&event.BaseEvent, "COMPLETED", payload)
}

func (ybm *YBM) SnapshotExportStarted(event *controlPlane.SnapshotExportStartedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", "")
}

func (ybm *YBM) SnapshotExportCompleted(event *controlPlane.SnapshotExportCompletedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "COMPLETED", "")
}

func (ybm *YBM) UpdateExportedRowCount(events []*controlPlane.UpdateExportedRowCountEvent) {
	ybm.Mutex.Lock()
	defer ybm.Mutex.Unlock()

	var filteredEvents []*controlPlane.BaseUpdateRowCountEvent

	for _, event := range events {
		lastUpdateTime, exists := ybm.lastRowCountUpdate[event.TableName]

		// Send if: 1) Completed, 2) First update, 3) 5 seconds passed
		if event.Status == "COMPLETED" {
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		} else if !exists {
			ybm.lastRowCountUpdate[event.TableName] = time.Now()
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		} else if time.Since(lastUpdateTime) >= 5*time.Second {
			ybm.lastRowCountUpdate[event.TableName] = time.Now()
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		}
	}

	if len(filteredEvents) > 0 {
		ybm.createAndSendUpdateRowCountEvent(filteredEvents)
	}
}

func (ybm *YBM) ImportSchemaStarted(event *controlPlane.ImportSchemaStartedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", "")
}

func (ybm *YBM) ImportSchemaCompleted(event *controlPlane.ImportSchemaCompletedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "COMPLETED", "")
}

func (ybm *YBM) SnapshotImportStarted(event *controlPlane.SnapshotImportStartedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", "")
}

func (ybm *YBM) SnapshotImportCompleted(event *controlPlane.SnapshotImportCompletedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "COMPLETED", "")
}

func (ybm *YBM) UpdateImportedRowCount(events []*controlPlane.UpdateImportedRowCountEvent) {
	ybm.Mutex.Lock()
	defer ybm.Mutex.Unlock()

	var filteredEvents []*controlPlane.BaseUpdateRowCountEvent

	for _, event := range events {
		lastUpdateTime, exists := ybm.lastRowCountUpdate[event.TableName]

		// Send if: 1) Completed, 2) First update, 3) 5 seconds passed
		if event.Status == "COMPLETED" {
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		} else if !exists {
			ybm.lastRowCountUpdate[event.TableName] = time.Now()
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		} else if time.Since(lastUpdateTime) >= 5*time.Second {
			ybm.lastRowCountUpdate[event.TableName] = time.Now()
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		}
	}

	if len(filteredEvents) > 0 {
		ybm.createAndSendUpdateRowCountEvent(filteredEvents)
	}
}

func (ybm *YBM) MigrationEnded(event *controlPlane.MigrationEndedEvent) {
	// No-op for now, but can be used to send final event to YBM
}
