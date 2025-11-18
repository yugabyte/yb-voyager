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
package ybaeon

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
	// PAYLOAD_VERSION is the version of the payload format sent to YB-Aeon API
	PAYLOAD_VERSION = "1.0"
)

type YBAeon struct {
	sync.Mutex
	config                   *YBAeonConfig                    // The YB-Aeon configuration
	httpClient               *httpclient.Client               // HTTP client with retry logic for YB-Aeon API
	voyagerInfo              *controlPlane.VoyagerInstance    // The Voyager instance information
	migrationDirectory       string                           // The directory where the migration is being done
	waitGroup                sync.WaitGroup                   // Wait group to track pending events
	eventChan                chan MigrationEventPayload       // Buffered channel to send migration events
	rowCountUpdateEventChan  chan []controlPlane.TableMetrics // Buffered channel to send table metrics updates in batches
	lastRowCountUpdate       map[string]time.Time             // Tracks last update time per table to throttle row count updates (limit to 1 update per 5 seconds per table)
	latestInvocationSequence int                              // Tracks the latest invocation sequence for a migration phase to avoid duplicates
}

// New creates a new YB-Aeon control plane instance
func New(exportDir string, config *YBAeonConfig) *YBAeon {
	vi := controlPlane.PrepareVoyagerInstance(exportDir)
	return &YBAeon{
		config:             config,
		voyagerInfo:        vi,
		migrationDirectory: exportDir,
	}
}

// Init initializes the YB-Aeon control plane
func (ybaeon *YBAeon) Init() error {
	// Validate configuration
	if err := ybaeon.validateConfig(); err != nil {
		return err
	}

	// Create buffered channels
	ybaeon.eventChan = make(chan MigrationEventPayload, 100)
	ybaeon.rowCountUpdateEventChan = make(chan []controlPlane.TableMetrics, 200)

	// Initialize HTTP client with retry logic
	// Configuration chosen for:
	// - Timeout (30s): Balances API responsiveness with allowing backend time for processing
	// - MaxRetries (5): Sufficient retries for transient errors with exponential backoff (1s, 2s, 4s, 8s, 16s)
	// - MaxIdleConns (10): Supports burst traffic during heavy migrations (100s of tables)
	// - IdleConnTimeout (90s): Keeps connections alive for typical migration patterns
	// - TLSHandshakeTimeout (10s): Prevents hanging on slow/bad SSL
	ybaeon.httpClient = httpclient.NewClient(httpclient.Config{
		BaseURL:             ybaeon.config.Domain,
		Timeout:             30 * time.Second,
		MaxRetries:          5,
		RetryWaitMin:        1 * time.Second,
		RetryWaitMax:        30 * time.Second,
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		Headers: map[string]string{
			"Authorization": "Bearer " + ybaeon.config.APIKey,
			"Content-Type":  "application/json",
		},
	})

	// Initialize state
	ybaeon.lastRowCountUpdate = make(map[string]time.Time)
	ybaeon.latestInvocationSequence = 0

	// Start background goroutines
	go ybaeon.eventPublisher()
	go ybaeon.rowCountUpdateEventPublisher()

	log.Infof("YB-Aeon control plane initialized for domain: %s", ybaeon.config.Domain)
	return nil
}

// validateConfig ensures all required configuration is present
func (ybaeon *YBAeon) validateConfig() error {
	if ybaeon.config == nil {
		return fmt.Errorf("YB-Aeon configuration is nil")
	}
	if ybaeon.config.Domain == "" {
		return fmt.Errorf("YB-Aeon domain is required")
	}
	if ybaeon.config.AccountID == "" {
		return fmt.Errorf("YB-Aeon account-id is required")
	}
	if ybaeon.config.ProjectID == "" {
		return fmt.Errorf("YB-Aeon project-id is required")
	}
	if ybaeon.config.ClusterID == "" {
		return fmt.Errorf("YB-Aeon cluster-id is required")
	}
	if ybaeon.config.APIKey == "" {
		return fmt.Errorf("YB-Aeon api-key is required")
	}
	return nil
}

// Finalize waits for all pending events to be sent and cleans up
func (ybaeon *YBAeon) Finalize() {
	ybaeon.waitGroup.Wait()
	log.Infof("YB-Aeon control plane finalized")
}

// eventPublisher runs in a goroutine and publishes migration events
func (ybaeon *YBAeon) eventPublisher() {
	defer ybaeon.panicHandler()
	for {
		event := <-ybaeon.eventChan
		err := ybaeon.sendMigrationEvent(event)
		if err != nil {
			log.Warnf("Couldn't send metadata for visualization to YB-Aeon. %s", err)
		}
		ybaeon.waitGroup.Done()
	}
}

// rowCountUpdateEventPublisher runs in a goroutine and publishes table metrics
func (ybaeon *YBAeon) rowCountUpdateEventPublisher() {
	defer ybaeon.panicHandler()
	for {
		events := <-ybaeon.rowCountUpdateEventChan
		err := ybaeon.sendTableMetrics(events)
		if err != nil {
			log.Warnf("Couldn't send table metrics for visualization to YB-Aeon. %s", err)
		}
		ybaeon.waitGroup.Done()
	}
}

// panicHandler recovers from panics in goroutines
func (ybaeon *YBAeon) panicHandler() {
	if r := recover(); r != nil {
		log.Errorf("Panic occurred in YB-Aeon event publisher: %v. No further events will be published to YB-Aeon.\nStack trace of panic location:\n%s",
			r, string(debug.Stack()))
	}
}

// createAndSendEvent builds and queues a migration event for YB-Aeon API
// YB-Aeon: payload parameter is interface{} - can be a struct, map, or nil
// For MigrationAssessmentCompleted: receives AssessMigrationPayloadYB-Aeon struct
// The struct is assigned to MigrationEventPayload.Payload and marshaled as part of the entire event
// Single marshal happens when sending to YB-Aeon API - no pre-marshaling or unmarshal cycles
func (ybaeon *YBAeon) createAndSendEvent(event *controlPlane.BaseEvent, status string, payload interface{}) {
	timestamp := time.Now().Format(time.RFC3339)

	invocationSequence, err := ybaeon.getInvocationSequence(event.MigrationUUID,
		controlPlane.MIGRATION_PHASE_MAP[event.EventType])
	if err != nil {
		log.Warnf("Unable to get invocation sequence for migration event. Cannot send event to YB-Aeon. %s", err)
		return
	}

	// Build host_IP string (pipe-separated list of IPs)
	hostIP := strings.Join(event.DBIP, "|")

	// Payload can be any type - struct, map, nil, etc.
	// The httpclient will marshal it to JSON when sending to YB-Aeon API
	// This is fully decoupled - each control plane passes data in its native format
	var eventPayload interface{}
	if payload != nil {
		eventPayload = payload
	} else {
		eventPayload = make(map[string]interface{})
	}

	migrationEvent := MigrationEventPayload{
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
		VoyagerClientInfo:   *ybaeon.voyagerInfo,
		DBType:              event.DBType,
		Status:              status,
		InvocationTimestamp: timestamp,
		MigrationDirectory:  ybaeon.migrationDirectory,
	}

	select {
	case ybaeon.eventChan <- migrationEvent:
		ybaeon.waitGroup.Add(1)
	default:
		log.Warnf("Could not publish migration event to YB-Aeon (channel full) %v", migrationEvent)
	}
}

// createAndSendUpdateRowCountEvent builds and queues table metrics events
func (ybaeon *YBAeon) createAndSendUpdateRowCountEvent(events []*controlPlane.BaseUpdateRowCountEvent) {
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
	case ybaeon.rowCountUpdateEventChan <- tableMetricsList:
		ybaeon.waitGroup.Add(1)
	default:
		log.Warnf("Could not publish row count update event to YB-Aeon (channel full) %v", tableMetricsList)
	}
}

// getInvocationSequence retrieves the next invocation sequence for a migration phase
func (ybaeon *YBAeon) getInvocationSequence(migrationUUID uuid.UUID, phase int) (int, error) {
	// Optimization: If already initialized, just increment
	if ybaeon.latestInvocationSequence > 0 {
		ybaeon.latestInvocationSequence += 1
		return ybaeon.latestInvocationSequence, nil
	}

	// Query YB-Aeon API for max sequence
	maxSeq, err := ybaeon.getMaxSequenceFromAPI(migrationUUID, phase)
	if err != nil {
		return 0, err
	}

	ybaeon.latestInvocationSequence = maxSeq + 1
	return ybaeon.latestInvocationSequence, nil
}

// getMaxSequenceFromAPI calls the GET /voyager/migrations/{migrationId}/phases/{phase}/latest-sequence endpoint
// HTTP client handles retries automatically with exponential backoff
func (ybaeon *YBAeon) getMaxSequenceFromAPI(migrationUUID uuid.UUID, phase int) (int, error) {
	path := fmt.Sprintf("/api/public/v1/accounts/%s/projects/%s/clusters/%s/voyager/migrations/%s/phases/%d/latest-sequence",
		ybaeon.config.AccountID, ybaeon.config.ProjectID, ybaeon.config.ClusterID, migrationUUID.String(), phase)

	var response struct {
		Data struct {
			LatestSequence int `json:"latest_sequence"`
		} `json:"data"`
	}

	statusCode, err := ybaeon.httpClient.Get(context.Background(), path, &response)

	// Special case: 404 means no prior data for this phase
	if statusCode == 404 {
		log.Infof("No prior data for phase %d (404), using sequence 0 (first run)", phase)
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("failed to get max sequence from YB-Aeon API: %w", err)
	}

	maxSeq := response.Data.LatestSequence
	log.Infof("YB-Aeon API returned latest sequence: %d for phase %d", maxSeq, phase)
	return maxSeq, nil
}

// sendMigrationEvent sends a migration event to YB-Aeon API
// HTTP client handles retries automatically with exponential backoff
func (ybaeon *YBAeon) sendMigrationEvent(event MigrationEventPayload) error {
	path := fmt.Sprintf("/api/public/v1/accounts/%s/projects/%s/clusters/%s/voyager/metadata",
		ybaeon.config.AccountID, ybaeon.config.ProjectID, ybaeon.config.ClusterID)

	_, err := ybaeon.httpClient.Post(context.Background(), path, event)
	if err != nil {
		return fmt.Errorf("failed to send migration event: %w", err)
	}

	log.Infof("Successfully sent migration event to YB-Aeon: phase=%d, sequence=%d, status=%s",
		event.MigrationPhase, event.InvocationSequence, event.Status)
	return nil
}

// sendTableMetrics sends table metrics to YB-Aeon API
// HTTP client handles retries automatically with exponential backoff
func (ybaeon *YBAeon) sendTableMetrics(metricsList []controlPlane.TableMetrics) error {
	path := fmt.Sprintf("/api/public/v1/accounts/%s/projects/%s/clusters/%s/voyager/table-metrics",
		ybaeon.config.AccountID, ybaeon.config.ProjectID, ybaeon.config.ClusterID)

	// Send each metric individually (YB-Aeon API expects single metric per PUT)
	for _, metrics := range metricsList {
		_, err := ybaeon.httpClient.Put(context.Background(), path, metrics)
		if err != nil {
			return fmt.Errorf("failed to send table metrics for %s: %w", metrics.TableName, err)
		}
	}

	log.Infof("Successfully sent %d table metrics to YB-Aeon", len(metricsList))
	return nil
}

// ========================= Control Plane Interface Implementation =========================
// All HTTP operations are handled by the httpclient package with built-in retry logic

func (ybaeon *YBAeon) MigrationAssessmentStarted(ev *controlPlane.MigrationAssessmentStartedEvent) {
	ybaeon.createAndSendEvent(&ev.BaseEvent, "IN PROGRESS", nil)
}

func (ybaeon *YBAeon) MigrationAssessmentCompleted(ev *controlPlane.MigrationAssessmentCompletedEvent) {
	// YB-Aeon: ev.Report is an AssessMigrationPayloadYB-Aeon struct (NOT a JSON string)
	// The struct is passed directly and will be marshaled only once by createAndSendEvent
	// when constructing the full MigrationEventPayload for the API call
	// Fully decoupled from Yugabyted - no redundant marshal/unmarshal cycles
	ybaeon.createAndSendEvent(&ev.BaseEvent, "COMPLETED", ev.Report)
}

func (ybaeon *YBAeon) ExportSchemaStarted(event *controlPlane.ExportSchemaStartedEvent) {
	ybaeon.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", nil)
}

func (ybaeon *YBAeon) ExportSchemaCompleted(event *controlPlane.ExportSchemaCompletedEvent) {
	ybaeon.createAndSendEvent(&event.BaseEvent, "COMPLETED", nil)
}

func (ybaeon *YBAeon) SchemaAnalysisStarted(event *controlPlane.SchemaAnalysisStartedEvent) {
	ybaeon.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", nil)
}

// YB-Aeon DATA FLOW - Schema Analysis Iteration Completed Event:
// 1. Event contains AnalysisReport struct (utils.SchemaReport)
// 2. Pass struct directly to createAndSendEvent - NO marshaling here
// 3. Struct assigned to MigrationEventPayload.Payload field
// 4. MARSHAL entire MigrationEventPayload (including AnalysisReport struct) when sending to YB-Aeon API
// Result: Single marshal (entire event with nested struct -> JSON), no unmarshal
func (ybaeon *YBAeon) SchemaAnalysisIterationCompleted(event *controlPlane.SchemaAnalysisIterationCompletedEvent) {
	// Pass the AnalysisReport struct directly - NO marshaling needed
	// json.Marshal in postJSON will handle it when sending to YB-Aeon API
	// This eliminates the marshal → string → unmarshal → marshal cycle
	ybaeon.createAndSendEvent(&event.BaseEvent, "COMPLETED", event.AnalysisReport)
}

func (ybaeon *YBAeon) SnapshotExportStarted(event *controlPlane.SnapshotExportStartedEvent) {
	ybaeon.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", nil)
}

func (ybaeon *YBAeon) SnapshotExportCompleted(event *controlPlane.SnapshotExportCompletedEvent) {
	ybaeon.createAndSendEvent(&event.BaseEvent, "COMPLETED", nil)
}

func (ybaeon *YBAeon) UpdateExportedRowCount(events []*controlPlane.UpdateExportedRowCountEvent) {
	ybaeon.Mutex.Lock()
	defer ybaeon.Mutex.Unlock()

	var filteredEvents []*controlPlane.BaseUpdateRowCountEvent

	for _, event := range events {
		lastUpdateTime, exists := ybaeon.lastRowCountUpdate[event.TableName]

		// Send if: 1) Completed, 2) First update, 3) 5 seconds passed
		if event.Status == "COMPLETED" {
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		} else if !exists {
			ybaeon.lastRowCountUpdate[event.TableName] = time.Now()
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		} else if time.Since(lastUpdateTime) >= 5*time.Second {
			ybaeon.lastRowCountUpdate[event.TableName] = time.Now()
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		}
	}

	if len(filteredEvents) > 0 {
		ybaeon.createAndSendUpdateRowCountEvent(filteredEvents)
	}
}

func (ybaeon *YBAeon) ImportSchemaStarted(event *controlPlane.ImportSchemaStartedEvent) {
	ybaeon.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", nil)
}

func (ybaeon *YBAeon) ImportSchemaCompleted(event *controlPlane.ImportSchemaCompletedEvent) {
	ybaeon.createAndSendEvent(&event.BaseEvent, "COMPLETED", nil)
}

func (ybaeon *YBAeon) SnapshotImportStarted(event *controlPlane.SnapshotImportStartedEvent) {
	ybaeon.createAndSendEvent(&event.BaseEvent, "IN PROGRESS", nil)
}

func (ybaeon *YBAeon) SnapshotImportCompleted(event *controlPlane.SnapshotImportCompletedEvent) {
	ybaeon.createAndSendEvent(&event.BaseEvent, "COMPLETED", nil)
}

func (ybaeon *YBAeon) UpdateImportedRowCount(events []*controlPlane.UpdateImportedRowCountEvent) {
	ybaeon.Mutex.Lock()
	defer ybaeon.Mutex.Unlock()

	var filteredEvents []*controlPlane.BaseUpdateRowCountEvent

	for _, event := range events {
		lastUpdateTime, exists := ybaeon.lastRowCountUpdate[event.TableName]

		// Send if: 1) Completed, 2) First update, 3) 5 seconds passed
		if event.Status == "COMPLETED" {
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		} else if !exists {
			ybaeon.lastRowCountUpdate[event.TableName] = time.Now()
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		} else if time.Since(lastUpdateTime) >= 5*time.Second {
			ybaeon.lastRowCountUpdate[event.TableName] = time.Now()
			filteredEvents = append(filteredEvents, &event.BaseUpdateRowCountEvent)
		}
	}

	if len(filteredEvents) > 0 {
		ybaeon.createAndSendUpdateRowCountEvent(filteredEvents)
	}
}

func (ybaeon *YBAeon) MigrationEnded(event *controlPlane.MigrationEndedEvent) {
	// No-op for now, but can be used to send final event to YB-Aeon
}
