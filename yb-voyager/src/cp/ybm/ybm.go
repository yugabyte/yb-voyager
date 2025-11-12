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
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	controlPlane "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/httpclient"
)

const (
	// PAYLOAD_VERSION is the version of the payload format sent to YBM API
	PAYLOAD_VERSION = "1.0"
)

type YBM struct {
	sync.Mutex
	config                   *YBMConfig                       // The YBM configuration
	httpClient               *httpclient.Client               // HTTP client with retry logic for YBM API
	voyagerInfo              *controlPlane.VoyagerInstance    // The Voyager instance information
	migrationDirectory       string                           // The directory where the migration is being done
	waitGroup                sync.WaitGroup                   // Wait group to track pending events
	eventChan                chan MigrationEvent              // Buffered channel to send migration events
	rowCountUpdateEventChan  chan []controlPlane.TableMetrics // Buffered channel to send table metrics updates in batches
	lastRowCountUpdate       map[string]time.Time             // Tracks last update time per table to throttle row count updates (limit to 1 update per 5 seconds per table)
	latestInvocationSequence int                              // Tracks the latest invocation sequence for a migration phase to avoid duplicates
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
	ybm.rowCountUpdateEventChan = make(chan []controlPlane.TableMetrics, 200)

	// Initialize HTTP client with retry logic
	// Configuration chosen for:
	// - Timeout (30s): Balances API responsiveness with allowing backend time for processing
	// - MaxRetries (5): Sufficient retries for transient errors with exponential backoff (1s, 2s, 4s, 8s, 16s)
	// - MaxIdleConns (10): Supports burst traffic during heavy migrations (100s of tables)
	// - IdleConnTimeout (90s): Keeps connections alive for typical migration patterns
	// - TLSHandshakeTimeout (10s): Prevents hanging on slow/bad SSL
	ybm.httpClient = httpclient.NewClient(httpclient.Config{
		BaseURL:             ybm.config.Domain,
		Timeout:             30 * time.Second,
		MaxRetries:          5,
		RetryWaitMin:        1 * time.Second,
		RetryWaitMax:        30 * time.Second,
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		Headers: map[string]string{
			"Authorization": "Bearer " + ybm.config.APIKey,
			"Content-Type":  "application/json",
		},
	})

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

// createAndSendEvent builds and queues a migration event for YBM API
// YBM: payload parameter is interface{} - can be a struct, map, or nil
// For MigrationAssessmentCompleted: receives AssessMigrationPayloadYBM struct
// The struct is assigned to MigrationEvent.Payload and marshaled as part of the entire event
// Single marshal happens when sending to YBM API - no pre-marshaling or unmarshal cycles
func (ybm *YBM) createAndSendEvent(event *controlPlane.BaseEvent, status string, payload interface{}) {
	timestamp := time.Now().Format(time.RFC3339)

	invocationSequence, err := ybm.getInvocationSequence(event.MigrationUUID,
		controlPlane.MIGRATION_PHASE_MAP[event.EventType])
	if err != nil {
		log.Warnf("Unable to get invocation sequence for migration event. Cannot send event to YBM. %s", err)
		return
	}

	// Build host_IP string (pipe-separated list of IPs)
	hostIP := strings.Join(event.DBIP, "|")

	// Payload can be any type - struct, map, nil, etc.
	// The httpclient will marshal it to JSON when sending to YBM API
	// This is fully decoupled - each control plane passes data in its native format
	var eventPayload interface{}
	if payload != nil {
		eventPayload = payload
	} else {
		eventPayload = make(map[string]interface{})
	}

	migrationEvent := MigrationEvent{
		MigrationUUID:       event.MigrationUUID,
		MigrationPhase:      controlPlane.MIGRATION_PHASE_MAP[event.EventType],
		InvocationSequence:  invocationSequence,
		DatabaseName:        event.DatabaseName,
		SchemaName:          strings.Join(event.SchemaNames, "|"),
		HostIP:              hostIP,
		Port:                event.Port,
		DBVersion:           event.DBVersion,
		Payload:             eventPayload,
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

	var tableMetricsList []controlPlane.TableMetrics

	for _, event := range events {
		metrics := controlPlane.TableMetrics{
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

// getMaxSequenceFromAPI calls the GET /voyager/migrations/{migrationId}/phases/{phase}/latest-sequence endpoint
// HTTP client handles retries automatically with exponential backoff
func (ybm *YBM) getMaxSequenceFromAPI(migrationUUID uuid.UUID, phase int) (int, error) {
	path := fmt.Sprintf("/api/public/v1/accounts/%s/projects/%s/clusters/%s/voyager/migrations/%s/phases/%d/latest-sequence",
		ybm.config.AccountID, ybm.config.ProjectID, ybm.config.ClusterID, migrationUUID.String(), phase)

	var response struct {
		Data struct {
			LatestSequence int `json:"latest_sequence"`
		} `json:"data"`
	}

	statusCode, err := ybm.httpClient.Get(context.Background(), path, &response)

	// Special case: 404 means no prior data for this phase
	if statusCode == 404 {
		log.Infof("No prior data for phase %d (404), using sequence 0 (first run)", phase)
		return 0, nil
	}

	if err != nil {
		// After all retries failed, fallback to sequence 0 to allow migration to proceed
		log.Warnf("Failed to get max sequence from YBM: %v. Falling back to sequence 0", err)
		return 0, nil
	}

	maxSeq := response.Data.LatestSequence
	log.Infof("YBM API returned latest sequence: %d for phase %d", maxSeq, phase)
	return maxSeq, nil
}

// sendMigrationEvent sends a migration event to YBM API
// HTTP client handles retries automatically with exponential backoff
func (ybm *YBM) sendMigrationEvent(event MigrationEvent) error {
	path := fmt.Sprintf("/api/public/v1/accounts/%s/projects/%s/clusters/%s/voyager/metadata",
		ybm.config.AccountID, ybm.config.ProjectID, ybm.config.ClusterID)

	_, err := ybm.httpClient.Post(context.Background(), path, event)
	if err != nil {
		return fmt.Errorf("failed to send migration event: %w", err)
	}

	log.Infof("Successfully sent migration event to YBM: phase=%d, sequence=%d, status=%s",
		event.MigrationPhase, event.InvocationSequence, event.Status)
	return nil
}

// sendTableMetrics sends table metrics to YBM API
// HTTP client handles retries automatically with exponential backoff
func (ybm *YBM) sendTableMetrics(metricsList []controlPlane.TableMetrics) error {
	path := fmt.Sprintf("/api/public/v1/accounts/%s/projects/%s/clusters/%s/voyager/table-metrics",
		ybm.config.AccountID, ybm.config.ProjectID, ybm.config.ClusterID)

	// Send each metric individually (YBM API expects single metric per PUT)
	for _, metrics := range metricsList {
		_, err := ybm.httpClient.Put(context.Background(), path, metrics)
		if err != nil {
			return fmt.Errorf("failed to send table metrics for %s: %w", metrics.TableName, err)
		}
	}

	log.Infof("Successfully sent %d table metrics to YBM", len(metricsList))
	return nil
}

// ========================= Control Plane Interface Implementation =========================
// All HTTP operations are handled by the httpclient package with built-in retry logic

func (ybm *YBM) MigrationAssessmentStarted(ev *controlPlane.MigrationAssessmentStartedEvent) {
	ybm.createAndSendEvent(&ev.BaseEvent, "IN PROGRESS", nil)
}

func (ybm *YBM) MigrationAssessmentCompleted(ev *controlPlane.MigrationAssessmentCompletedEvent) {
	// YBM: ev.Report is an AssessMigrationPayloadYBM struct (NOT a JSON string)
	// The struct is passed directly and will be marshaled only once by createAndSendEvent
	// when constructing the full MigrationEvent for the API call
	// Fully decoupled from Yugabyted - no redundant marshal/unmarshal cycles
	ybm.createAndSendEvent(&ev.BaseEvent, "COMPLETED", ev.Report)
}

func (ybm *YBM) ExportSchemaStarted(event *controlPlane.ExportSchemaStartedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", nil)
}

func (ybm *YBM) ExportSchemaCompleted(event *controlPlane.ExportSchemaCompletedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "COMPLETED", nil)
}

func (ybm *YBM) SchemaAnalysisStarted(event *controlPlane.SchemaAnalysisStartedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", nil)
}

// YBM DATA FLOW - Schema Analysis Iteration Completed Event:
// 1. Event contains AnalysisReport struct (utils.SchemaReport)
// 2. Pass struct directly to createAndSendEvent - NO marshaling here
// 3. Struct assigned to MigrationEvent.Payload field
// 4. MARSHAL entire MigrationEvent (including AnalysisReport struct) when sending to YBM API
// Result: Single marshal (entire event with nested struct -> JSON), no unmarshal
func (ybm *YBM) SchemaAnalysisIterationCompleted(event *controlPlane.SchemaAnalysisIterationCompletedEvent) {
	// Pass the AnalysisReport struct directly - NO marshaling needed
	// json.Marshal in postJSON will handle it when sending to YBM API
	// This eliminates the marshal → string → unmarshal → marshal cycle
	ybm.createAndSendEvent(&event.BaseEvent, "COMPLETED", event.AnalysisReport)
}

func (ybm *YBM) SnapshotExportStarted(event *controlPlane.SnapshotExportStartedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", nil)
}

func (ybm *YBM) SnapshotExportCompleted(event *controlPlane.SnapshotExportCompletedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "COMPLETED", nil)
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
	ybm.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", nil)
}

func (ybm *YBM) ImportSchemaCompleted(event *controlPlane.ImportSchemaCompletedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "COMPLETED", nil)
}

func (ybm *YBM) SnapshotImportStarted(event *controlPlane.SnapshotImportStartedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", nil)
}

func (ybm *YBM) SnapshotImportCompleted(event *controlPlane.SnapshotImportCompletedEvent) {
	ybm.createAndSendEvent(&event.BaseEvent, "COMPLETED", nil)
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
