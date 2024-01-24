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
package yugabyted

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	log "github.com/sirupsen/logrus"
	controlPlane "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
)

type YugabyteD struct {
	sync.Mutex
	sync.RWMutex
	migrationDirectory       string
	waitGroup                sync.WaitGroup
	eventChan                chan (VisualizerDBPayload)
	rowCountUpdateEventChan  chan ([]VisualizerTableMetrics)
	conn                     *pgxpool.Pool
	lastRowCountUpdate       map[string]time.Time
	latestInvocationSequence int
}

func New(exportDir string) *YugabyteD {
	return &YugabyteD{migrationDirectory: exportDir}
}

// Initialize the target DB for visualisation metadata
func (cp *YugabyteD) Init() error {

	cp.eventChan = make(chan VisualizerDBPayload, 100)
	cp.rowCountUpdateEventChan = make(chan []VisualizerTableMetrics, 200)

	err := cp.connect()
	if err != nil {
		return err
	}

	err = cp.setupDatabase()
	if err != nil {
		return err
	}

	cp.lastRowCountUpdate = make(map[string]time.Time)
	cp.latestInvocationSequence = 0

	go cp.eventPublisher()
	go cp.rowCountUpdateEventPublisher()

	return nil
}

// Wait for events to publish and Close the connection.
func (cp *YugabyteD) Finalize() {
	cp.waitGroup.Wait()
	cp.disconnect()
}

func (cp *YugabyteD) eventPublisher() {
	defer cp.panicHandler()
	for {
		event := <-cp.eventChan
		err := cp.sendVisualizerDBPayload(event)
		if err != nil {
			log.Warnf("Couldn't send metadata for visualization. %s", err)
		}
		log.Warnf("WaitGroup Done")
		cp.waitGroup.Done()
	}
}

func (cp *YugabyteD) rowCountUpdateEventPublisher() {
	defer cp.panicHandler()
	for {
		event := <-cp.rowCountUpdateEventChan
		err := cp.sendVisualizerTableMetrics(event)
		if err != nil {
			log.Warnf("Couldn't send metadata for visualization. %s", err)
		}
		cp.waitGroup.Done()
	}
}

func (cp *YugabyteD) createAndSendEvent(event *controlPlane.BaseEvent, status string, payload string) {

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	invocationSequence, err := cp.getInvocationSequence(event.MigrationUUID,
		MIGRATION_PHASE_MAP[event.EventType])

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
		return
	}

	dbPayload := VisualizerDBPayload{
		MigrationUUID:       event.MigrationUUID,
		MigrationPhase:      MIGRATION_PHASE_MAP[event.EventType],
		InvocationSequence:  invocationSequence,
		DatabaseName:        event.DatabaseName,
		SchemaName:          strings.Join(event.SchemaName[:], "|"),
		Payload:             payload,
		DBType:              event.DBType,
		Status:              status,
		InvocationTimestamp: timestamp,
	}

	cp.waitGroup.Add(1)
	cp.eventChan <- dbPayload
}

func (cp *YugabyteD) createAndSendUpdateRowCountEvent(events []*controlPlane.BaseUpdateRowCountEvent) {

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	var snapshotMigrateAllTableMetrics []VisualizerTableMetrics

	for _, event := range events {
		snapshotMigrateTableMetrics := VisualizerTableMetrics{
			MigrationUUID:       event.MigrationUUID,
			TableName:           event.TableName,
			Schema:              strings.Join(event.SchemaName[:], "|"),
			MigrationPhase:      MIGRATION_PHASE_MAP[event.EventType],
			Status:              UPDATE_ROW_COUNT_STATUS_STR_TO_INT[event.Status],
			CountLiveRows:       event.CompletedRowCount,
			CountTotalRows:      event.TotalRowCount,
			InvocationTimestamp: timestamp,
		}

		snapshotMigrateAllTableMetrics = append(snapshotMigrateAllTableMetrics,
			snapshotMigrateTableMetrics)
	}

	cp.waitGroup.Add(1)
	cp.rowCountUpdateEventChan <- snapshotMigrateAllTableMetrics
}

func (cp *YugabyteD) ExportSchemaStarted(exportSchemaEvent *controlPlane.ExportSchemaStartedEvent) {
	cp.createAndSendEvent(&exportSchemaEvent.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteD) ExportSchemaCompleted(exportSchemaEvent *controlPlane.ExportSchemaCompletedEvent) {
	cp.createAndSendEvent(&exportSchemaEvent.BaseEvent, "COMPLETED", "")
}

func (cp *YugabyteD) SchemaAnalysisStarted(schemaAnalysisEvent *controlPlane.SchemaAnalysisStartedEvent) {

	cp.createAndSendEvent(&schemaAnalysisEvent.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteD) SchemaAnalysisIterationCompleted(schemaAnalysisReport *controlPlane.SchemaAnalysisIterationCompletedEvent) {

	jsonBytes, err := json.Marshal(schemaAnalysisReport.AnalysisReport)
	if err != nil {
		panic(err)
	}
	payload := string(jsonBytes)

	cp.createAndSendEvent(&schemaAnalysisReport.BaseEvent, "COMPLETED", payload)
}

func (cp *YugabyteD) SnapshotExportStarted(snapshotExportEvent *controlPlane.SnapshotExportStartedEvent) {
	cp.createAndSendEvent(&snapshotExportEvent.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteD) SnapshotExportCompleted(snapshotExportEvent *controlPlane.SnapshotExportCompletedEvent) {
	cp.createAndSendEvent(&snapshotExportEvent.BaseEvent, "COMPLETED", "")
}

func (cp *YugabyteD) UpdateExportedRowCount(
	snapshotExportTablesMetrics []*controlPlane.UpdateExportedRowCountEvent) {

	var updateExportedRowCountEvents []*controlPlane.BaseUpdateRowCountEvent
	for _, updateExportedRowCountEvent := range snapshotExportTablesMetrics {

		cp.RWMutex.RLock()
		lastRowCountUpdateTime, check := cp.lastRowCountUpdate[updateExportedRowCountEvent.TableName]
		cp.RWMutex.RUnlock()

		if updateExportedRowCountEvent.Status == "COMPLETED" {
			updateExportedRowCountEvents = append(updateExportedRowCountEvents, &updateExportedRowCountEvent.BaseUpdateRowCountEvent)
		} else if !check {
			cp.RWMutex.Lock()
			cp.lastRowCountUpdate[updateExportedRowCountEvent.TableName] = time.Now()
			cp.RWMutex.Unlock()

			updateExportedRowCountEvents = append(updateExportedRowCountEvents, &updateExportedRowCountEvent.BaseUpdateRowCountEvent)
		} else if !lastRowCountUpdateTime.Add(time.Second * 5).After(time.Now()) {
			cp.RWMutex.Lock()
			cp.lastRowCountUpdate[updateExportedRowCountEvent.TableName] = time.Now()
			cp.RWMutex.Unlock()

			updateExportedRowCountEvents = append(updateExportedRowCountEvents, &updateExportedRowCountEvent.BaseUpdateRowCountEvent)
		}

	}
	cp.createAndSendUpdateRowCountEvent(updateExportedRowCountEvents)
}

func (cp *YugabyteD) ImportSchemaStarted(importSchemaEvent *controlPlane.ImportSchemaStartedEvent) {
	cp.createAndSendEvent(&importSchemaEvent.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteD) ImportSchemaCompleted(importSchemaEvent *controlPlane.ImportSchemaCompletedEvent) {
	cp.createAndSendEvent(&importSchemaEvent.BaseEvent, "COMPLETED", "")
}

func (cp *YugabyteD) SnapshotImportStarted(snapshotImportEvent *controlPlane.SnapshotImportStartedEvent) {
	cp.createAndSendEvent(&snapshotImportEvent.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteD) SnapshotImportCompleted(snapshotImportEvent *controlPlane.SnapshotImportCompletedEvent) {
	cp.createAndSendEvent(&snapshotImportEvent.BaseEvent, "COMPLETED", "")
}

func (cp *YugabyteD) UpdateImportedRowCount(
	snapshotImportTableMetrics []*controlPlane.UpdateImportedRowCountEvent) {

	var updateImportedRowCountEvents []*controlPlane.BaseUpdateRowCountEvent
	for _, updateImportedRowCountEvent := range snapshotImportTableMetrics {

		cp.RWMutex.RLock()
		lastRowCountUpdateTime, check := cp.lastRowCountUpdate[updateImportedRowCountEvent.TableName]
		cp.RWMutex.RUnlock()

		if updateImportedRowCountEvent.Status == "COMPLETED" {
			updateImportedRowCountEvents = append(updateImportedRowCountEvents, &updateImportedRowCountEvent.BaseUpdateRowCountEvent)
		} else if !check {
			cp.RWMutex.Lock()
			cp.lastRowCountUpdate[updateImportedRowCountEvent.TableName] = time.Now()
			cp.RWMutex.Unlock()

			updateImportedRowCountEvents = append(updateImportedRowCountEvents, &updateImportedRowCountEvent.BaseUpdateRowCountEvent)
		} else if !lastRowCountUpdateTime.Add(time.Second * 5).After(time.Now()) {
			cp.RWMutex.Lock()
			cp.lastRowCountUpdate[updateImportedRowCountEvent.TableName] = time.Now()
			cp.RWMutex.Unlock()

			updateImportedRowCountEvents = append(updateImportedRowCountEvents, &updateImportedRowCountEvent.BaseUpdateRowCountEvent)
		}

	}
	cp.createAndSendUpdateRowCountEvent(updateImportedRowCountEvents)
}

func (cp *YugabyteD) MigrationEnded(migrationEndedEvent *controlPlane.MigrationEndedEvent) {
}

func (cp *YugabyteD) panicHandler() {
	if r := recover(); r != nil {
		// Handle the panic for eventPublishers
		log.Warnf(fmt.Sprintf("Panic occurred:%v", r))
	}
}

func (cp *YugabyteD) getConn() (*pgxpool.Pool, error) {
	var err error
	err = nil
	if cp.conn == nil {
		err = fmt.Errorf("called visualizer_yugabyte_db.get_conn() " +
			"before visualizer_yugabyte_db.connect()")
	}

	return cp.conn, err
}

func (cp *YugabyteD) reconnect() error {
	var err error
	cp.disconnect()
	for attempt := 1; attempt < 5; attempt++ {
		err = cp.connect()
		if err == nil {
			return nil
		}
		log.Infof("Failed to reconnect to the target database: %s", err)
		time.Sleep(time.Second)
	}
	return fmt.Errorf("reconnect to target db: %w", err)
}

func (cp *YugabyteD) connect() error {
	if cp.conn != nil {
		return nil
	}
	connectionUri := os.Getenv("YUGABYTED_DB_CONN_STRING")

	conn, err := pgxpool.Connect(context.Background(), connectionUri)
	if err != nil {
		return fmt.Errorf("connect to target db: %w", err)
	}

	cp.conn = conn
	return nil
}

func (cp *YugabyteD) disconnect() {
	if cp.conn == nil {
		return
	}

	cp.conn.Close()
	cp.conn = nil
}

const BATCH_METADATA_TABLE_SCHEMA = "ybvoyager_visualizer"

// Set-up YBD database for visualisation metadata
func (cp *YugabyteD) setupDatabase() error {
	err := cp.createVoyagerSchema()
	if err != nil {
		return err
	}

	err = cp.createYugabytedMetadataTable()
	if err != nil {
		return err
	}

	err = cp.createYugabytedTableMetricsTable()
	if err != nil {
		return err
	}

	return nil
}

// Create the visualisation schema
func (cp *YugabyteD) createVoyagerSchema() error {
	cmd := fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s;`, BATCH_METADATA_TABLE_SCHEMA)

	err := cp.executeCmdOnTarget(cmd)
	if err != nil {
		return err
	}

	return nil
}

const YUGABYTED_METADATA_TABLE_NAME = BATCH_METADATA_TABLE_SCHEMA + "." + "ybvoyager_visualizer_metadata"

// Create visualisation metadata table
func (cp *YugabyteD) createYugabytedMetadataTable() error {
	cmd := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			migration_uuid UUID,
			migration_phase INT,
			invocation_sequence INT,
			migration_dir VARCHAR(250),
			database_name VARCHAR(250),
			schema_name VARCHAR(250),
			payload TEXT,
			complexity VARCHAR(30),
			db_type VARCHAR(30),
			status VARCHAR(30),
			invocation_timestamp TIMESTAMPTZ,
			PRIMARY KEY (migration_uuid, migration_phase, invocation_sequence)
			);`, YUGABYTED_METADATA_TABLE_NAME)

	err := cp.executeCmdOnTarget(cmd)
	if err != nil {
		return err
	}

	return nil
}

const YUGABYTED_TABLE_METRICS_TABLE_NAME = BATCH_METADATA_TABLE_SCHEMA + "." + "ybvoyager_visualizer_table_metrics"

// Create table metrics table
func (cp *YugabyteD) createYugabytedTableMetricsTable() error {
	cmd := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			migration_uuid UUID,
			table_name VARCHAR(250),
			schema_name VARCHAR(250),
			migration_phase INT,
			status INT,
			count_live_rows INT,
			count_total_rows INT,
			invocation_timestamp TIMESTAMPTZ,
			PRIMARY KEY (migration_uuid, table_name, migration_phase, schema_name)
			);`, YUGABYTED_TABLE_METRICS_TABLE_NAME)

	err := cp.executeCmdOnTarget(cmd)
	if err != nil {
		return err
	}

	return nil
}

// Get the latest invocation sequence for a given migration_uuid and migration phase
func (cp *YugabyteD) getInvocationSequence(mUUID uuid.UUID, phase int) (int, error) {

	// Increment and return invocation sequence number if already initialised.
	if cp.latestInvocationSequence > 0 {
		cp.latestInvocationSequence += 1
		return cp.latestInvocationSequence, nil
	}

	cmd := fmt.Sprintf(`SELECT MAX(invocation_sequence) AS latest_sequence 
		FROM %s 
		WHERE migration_uuid = '%s' AND migration_phase = %d`, YUGABYTED_METADATA_TABLE_NAME,
		mUUID, phase)

	log.Infof("Executing on target DB: [%s]", cmd)

	var latestSequence *int
	conn, err := cp.getConn()
	if err != nil {
		return 0, err
	}

	err = conn.QueryRow(context.Background(), cmd).Scan(&latestSequence)
	if err != nil {
		if err == pgx.ErrNoRows {
			cp.latestInvocationSequence = 1
			return cp.latestInvocationSequence, nil
		} else {
			return 0, fmt.Errorf("couldn't get the latest sequence number: %w", err)
		}
	}

	if latestSequence != nil {
		cp.latestInvocationSequence = *latestSequence + 1
		return cp.latestInvocationSequence, nil
	} else {
		cp.latestInvocationSequence = 1
		return cp.latestInvocationSequence, nil
	}
}

// Send visualisation metadata
func (cp *YugabyteD) sendVisualizerDBPayload(
	visualizerDBPayload VisualizerDBPayload) error {
	cmd := fmt.Sprintf("INSERT INTO %s ("+
		"migration_uuid, "+
		"migration_phase, "+
		"invocation_sequence, "+
		"migration_dir, "+
		"database_name, "+
		"schema_name, "+
		"payload, "+
		"db_type, "+
		"status, "+
		"invocation_timestamp"+
		") VALUES ("+
		"$1, $2, $3, $4, $5, $6, $7, $8, $9, $10"+
		")", YUGABYTED_METADATA_TABLE_NAME)

	var maxAttempts = 5
	visualizerDBPayload.MigrationDirectory = cp.migrationDirectory

	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = cp.executeInsertQuery(cmd, visualizerDBPayload)
		if err == nil {
			break
		} else {
			if attempt == maxAttempts {
				return err
			}
		}

		invocationSequence, err := cp.getInvocationSequence(visualizerDBPayload.MigrationUUID,
			visualizerDBPayload.MigrationPhase)

		if err != nil {
			log.Warnf("Cannot get invocation sequence for visualization metadata. %s", err)
			return err
		}

		timestamp := time.Now().Format("2006-01-02 15:04:05")

		visualizerDBPayload.InvocationSequence = invocationSequence
		visualizerDBPayload.InvocationTimestamp = timestamp
	}

	return nil
}

// Send Table metrics
func (cp *YugabyteD) sendVisualizerTableMetrics(
	visualizerTableMetricsList []VisualizerTableMetrics) error {

	cmd := fmt.Sprintf("INSERT INTO %s ("+
		"migration_uuid, "+
		"table_name, "+
		"schema_name,"+
		"migration_phase, "+
		"status, "+
		"count_live_rows, "+
		"count_total_rows, "+
		"invocation_timestamp"+
		") VALUES ", YUGABYTED_TABLE_METRICS_TABLE_NAME)

	var valuesList []string
	for _, visualizerTableMetrics := range visualizerTableMetricsList {
		value := fmt.Sprintf("('%s', '%s', '%s', %d, %d, %d, %d, '%s')",
			visualizerTableMetrics.MigrationUUID,
			visualizerTableMetrics.TableName,
			visualizerTableMetrics.Schema,
			visualizerTableMetrics.MigrationPhase,
			visualizerTableMetrics.Status,
			visualizerTableMetrics.CountLiveRows,
			visualizerTableMetrics.CountTotalRows,
			visualizerTableMetrics.InvocationTimestamp)

		valuesList = append(valuesList, value)
	}

	cmd += strings.Join(valuesList, ",")

	cmd += fmt.Sprintf(" ON CONFLICT (migration_uuid, table_name, schema_name, migration_phase) " +
		"DO UPDATE " +
		"SET " +
		"status = EXCLUDED.status," +
		"count_live_rows = EXCLUDED.count_live_rows," +
		"count_total_rows = EXCLUDED.count_total_rows," +
		"invocation_timestamp = EXCLUDED.invocation_timestamp;")

	err := cp.executeCmdOnTarget(cmd)
	if err != nil {
		return err
	}

	return nil
}

func (cp *YugabyteD) executeInsertQuery(cmd string,
	visualizerDBPayload VisualizerDBPayload) error {

	cp.Mutex.Lock()
	defer cp.Mutex.Unlock()

	var err error

	log.Infof("Executing on target DB: [%s] for [%+v]", cmd, visualizerDBPayload)
	conn, err := cp.getConn()
	if err != nil {
		return err
	}

	_, err = conn.Exec(context.Background(), cmd,
		visualizerDBPayload.MigrationUUID,
		visualizerDBPayload.MigrationPhase,
		visualizerDBPayload.InvocationSequence,
		visualizerDBPayload.MigrationDirectory,
		visualizerDBPayload.DatabaseName,
		visualizerDBPayload.SchemaName,
		visualizerDBPayload.Payload,
		visualizerDBPayload.DBType,
		visualizerDBPayload.Status,
		visualizerDBPayload.InvocationTimestamp)

	if err == nil {
		return nil
	}
	log.Warnf("Error while running [%s]: %s", cmd, err)
	time.Sleep(time.Second)
	err2 := cp.reconnect()
	if err2 != nil {
		log.Warnf("Failed to reconnect to the target database: %s", err2)
	}

	if err != nil {
		return fmt.Errorf("couldn't excute command %s on target db. error: %w", cmd, err)
	}

	return nil
}

// Function to execute any cmd on target DB.
func (cp *YugabyteD) executeCmdOnTarget(cmd string) error {
	maxAttempts := 5
	var err error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Infof("Executing on target DB: [%s]", cmd)
		conn, err := cp.getConn()
		if err != nil {
			return err
		}

		_, err = conn.Exec(context.Background(), cmd)
		if err == nil {
			return nil
		}
		log.Warnf("Error while running [%s] attempt %d: %s", cmd, attempt, err)
		time.Sleep(time.Second)
		err2 := cp.reconnect()
		if err2 != nil {
			log.Warnf("Failed to reconnect to the target database: %s", err2)
			break
		}
	}
	if err != nil {
		return fmt.Errorf("couldn't excute command %s on target db. error: %w", cmd, err)
	}
	return nil
}
