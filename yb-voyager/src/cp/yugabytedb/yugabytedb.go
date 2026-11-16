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
package yugabytedb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	goerrors "github.com/go-errors/errors"

	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
	controlPlane "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
)

// DB_CONN_STRING_ENV_VAR names the env var holding the control plane DB conn string.
// cmd sets it from the yugabytedb-control-plane.db-conn-string config param.
const DB_CONN_STRING_ENV_VAR = "YUGABYTEDB_CONTROL_PLANE_DB_CONN_STRING"

type YugabyteDB struct {
	sync.Mutex
	waitGroup               sync.WaitGroup
	eventChan               chan (MigrationEvent)
	rowCountUpdateEventChan chan ([]controlPlane.TableMetrics)
	connPool                *pgxpool.Pool
	lastRowCountUpdate      map[string]time.Time
}

func New(exportDir string) *YugabyteDB {
	return &YugabyteDB{}
}

// Initialize the connection to the yugabytedb control plane DB
func (cp *YugabyteDB) Init() error {
	cp.eventChan = make(chan MigrationEvent, 100)
	cp.rowCountUpdateEventChan = make(chan []controlPlane.TableMetrics, 200)

	err := cp.connect()
	if err != nil {
		return err
	}

	cp.lastRowCountUpdate = make(map[string]time.Time)

	go cp.eventPublisher()
	go cp.rowCountUpdateEventPublisher()

	return nil
}

// Wait for events to publish and Close the connection.
func (cp *YugabyteDB) Finalize() {
	cp.waitGroup.Wait()
	cp.disconnect()
}

func (cp *YugabyteDB) eventPublisher() {
	defer cp.panicHandler()
	for {
		event := <-cp.eventChan
		err := cp.sendMigrationEvent(event)
		if err != nil {
			log.Warnf("Couldn't send migration event to the control plane. %s", err)
		}
		cp.waitGroup.Done()
	}
}

func (cp *YugabyteDB) rowCountUpdateEventPublisher() {
	defer cp.panicHandler()
	for {
		event := <-cp.rowCountUpdateEventChan
		err := cp.sendTableDataProgress(event)
		if err != nil {
			log.Warnf("Couldn't send table data progress to the control plane. %s", err)
		}
		cp.waitGroup.Done()
	}
}

// payload is a JSON document, or "" when the event carries none.
func (cp *YugabyteDB) createAndSendEvent(event *controlPlane.BaseEvent, status string, payload string) {

	migrationEvent := MigrationEvent{
		MigrationUUID:  event.MigrationUUID,
		MigrationPhase: controlPlane.MIGRATION_PHASE_MAP[event.EventType],
		Payload:        payload,
		Status:         status,
	}

	select {
	case cp.eventChan <- migrationEvent:
		cp.waitGroup.Add(1)
	default:
		log.Warnf("Could not publish migration event %v", migrationEvent)
	}
}

func (cp *YugabyteDB) createAndSendUpdateRowCountEvent(events []*controlPlane.BaseUpdateRowCountEvent) {

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	var rowCountUpdateEvent []controlPlane.TableMetrics

	for _, event := range events {
		snapshotMigrateTableMetrics := controlPlane.TableMetrics{
			MigrationUUID:       event.MigrationUUID,
			TableName:           event.TableName,
			SchemaName:          strings.Join(event.SchemaNames, "|"),
			MigrationPhase:      controlPlane.MIGRATION_PHASE_MAP[event.EventType],
			Status:              controlPlane.UPDATE_ROW_COUNT_STATUS_STR_TO_INT[event.Status],
			CountLiveRows:       event.CompletedRowCount,
			CountTotalRows:      event.TotalRowCount,
			InvocationTimestamp: timestamp,
		}

		rowCountUpdateEvent = append(rowCountUpdateEvent,
			snapshotMigrateTableMetrics)
	}

	select {
	case cp.rowCountUpdateEventChan <- rowCountUpdateEvent:
		cp.waitGroup.Add(1)
	default:
		log.Warnf("Could not publish row count update event %v", rowCountUpdateEvent)
	}
}

func (cp *YugabyteDB) MigrationAssessmentStarted(ev *controlPlane.MigrationAssessmentStartedEvent) {
	cp.createAndSendEvent(&ev.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteDB) MigrationAssessmentCompleted(ev *controlPlane.MigrationAssessmentCompletedEvent) {
	payloadStr, ok := ev.Report.(string)
	if !ok {
		log.Warnf("Expected Report to be a string for YugabyteDB control plane, got %T", ev.Report)
		payloadStr = ""
	}
	cp.createAndSendEvent(&ev.BaseEvent, "COMPLETED", payloadStr)
}

func (cp *YugabyteDB) ExportSchemaStarted(exportSchemaEvent *controlPlane.ExportSchemaStartedEvent) {
	cp.createAndSendEvent(&exportSchemaEvent.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteDB) ExportSchemaCompleted(exportSchemaEvent *controlPlane.ExportSchemaCompletedEvent) {
	cp.createAndSendEvent(&exportSchemaEvent.BaseEvent, "COMPLETED", "")
}

func (cp *YugabyteDB) SchemaAnalysisStarted(schemaAnalysisEvent *controlPlane.SchemaAnalysisStartedEvent) {
	cp.createAndSendEvent(&schemaAnalysisEvent.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteDB) SchemaAnalysisIterationCompleted(schemaAnalysisReport *controlPlane.SchemaAnalysisIterationCompletedEvent) {

	jsonBytes, err := json.Marshal(schemaAnalysisReport.AnalysisReport)
	if err != nil {
		log.Warnf("%v", err)
		return
	}
	payload := string(jsonBytes)

	cp.createAndSendEvent(&schemaAnalysisReport.BaseEvent, "COMPLETED", payload)
}

func (cp *YugabyteDB) SnapshotExportStarted(snapshotExportEvent *controlPlane.SnapshotExportStartedEvent) {
	cp.createAndSendEvent(&snapshotExportEvent.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteDB) SnapshotExportCompleted(snapshotExportEvent *controlPlane.SnapshotExportCompletedEvent) {
	cp.createAndSendEvent(&snapshotExportEvent.BaseEvent, "COMPLETED", "")
}

func (cp *YugabyteDB) UpdateExportedRowCount(
	snapshotExportTablesMetrics []*controlPlane.UpdateExportedRowCountEvent) {

	cp.Mutex.Lock()
	defer cp.Mutex.Unlock()

	var updateExportedRowCountEvents []*controlPlane.BaseUpdateRowCountEvent
	for _, updateExportedRowCountEvent := range snapshotExportTablesMetrics {

		lastRowCountUpdateTime, check := cp.lastRowCountUpdate[updateExportedRowCountEvent.TableName]

		if updateExportedRowCountEvent.Status == "COMPLETED" {
			updateExportedRowCountEvents = append(updateExportedRowCountEvents, &updateExportedRowCountEvent.BaseUpdateRowCountEvent)
		} else if !check {
			cp.lastRowCountUpdate[updateExportedRowCountEvent.TableName] = time.Now()

			updateExportedRowCountEvents = append(updateExportedRowCountEvents, &updateExportedRowCountEvent.BaseUpdateRowCountEvent)
		} else if !lastRowCountUpdateTime.Add(time.Second * 5).After(time.Now()) {
			cp.lastRowCountUpdate[updateExportedRowCountEvent.TableName] = time.Now()

			updateExportedRowCountEvents = append(updateExportedRowCountEvents, &updateExportedRowCountEvent.BaseUpdateRowCountEvent)
		}

	}
	if len(updateExportedRowCountEvents) > 0 {
		cp.createAndSendUpdateRowCountEvent(updateExportedRowCountEvents)
	}
}

func (cp *YugabyteDB) ImportSchemaStarted(importSchemaEvent *controlPlane.ImportSchemaStartedEvent) {
	cp.createAndSendEvent(&importSchemaEvent.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteDB) ImportSchemaCompleted(importSchemaEvent *controlPlane.ImportSchemaCompletedEvent) {
	cp.createAndSendEvent(&importSchemaEvent.BaseEvent, "COMPLETED", "")
}

func (cp *YugabyteDB) SnapshotImportStarted(snapshotImportEvent *controlPlane.SnapshotImportStartedEvent) {
	cp.createAndSendEvent(&snapshotImportEvent.BaseEvent, "IN PROGRESS", "")
}

func (cp *YugabyteDB) SnapshotImportCompleted(snapshotImportEvent *controlPlane.SnapshotImportCompletedEvent) {
	cp.createAndSendEvent(&snapshotImportEvent.BaseEvent, "COMPLETED", "")
}

func (cp *YugabyteDB) UpdateImportedRowCount(
	snapshotImportTableMetrics []*controlPlane.UpdateImportedRowCountEvent) {

	cp.Mutex.Lock()
	defer cp.Mutex.Unlock()

	var updateImportedRowCountEvents []*controlPlane.BaseUpdateRowCountEvent
	for _, updateImportedRowCountEvent := range snapshotImportTableMetrics {

		lastRowCountUpdateTime, check := cp.lastRowCountUpdate[updateImportedRowCountEvent.TableName]

		if updateImportedRowCountEvent.Status == "COMPLETED" {
			updateImportedRowCountEvents = append(updateImportedRowCountEvents, &updateImportedRowCountEvent.BaseUpdateRowCountEvent)
		} else if !check {
			cp.lastRowCountUpdate[updateImportedRowCountEvent.TableName] = time.Now()
			updateImportedRowCountEvents = append(updateImportedRowCountEvents, &updateImportedRowCountEvent.BaseUpdateRowCountEvent)
		} else if !lastRowCountUpdateTime.Add(time.Second * 5).After(time.Now()) {
			cp.lastRowCountUpdate[updateImportedRowCountEvent.TableName] = time.Now()
			updateImportedRowCountEvents = append(updateImportedRowCountEvents, &updateImportedRowCountEvent.BaseUpdateRowCountEvent)
		}

	}
	if len(updateImportedRowCountEvents) > 0 {
		cp.createAndSendUpdateRowCountEvent(updateImportedRowCountEvents)
	}
}

func (cp *YugabyteDB) MigrationEnded(migrationEndedEvent *controlPlane.MigrationEndedEvent) {
}

func (cp *YugabyteDB) panicHandler() {
	if r := recover(); r != nil {
		// Handle the panic for eventPublishers
		log.Errorf("Panic occurred: %v. No further events will be published to yugabytedb control plane DB.\n"+
			"Stack trace of panic location:\n%s", r, string(debug.Stack()))
	}
}

func (cp *YugabyteDB) getConnPool() (*pgxpool.Pool, error) {
	var err error
	err = nil
	if cp.connPool == nil {
		log.Warnf("No Connections to yugabytedb control plane DB found. Creating a connection pool...")
		err = cp.connect()
	}

	return cp.connPool, err
}

func (cp *YugabyteDB) reconnect() error {
	var err error
	cp.disconnect()
	for attempt := 1; attempt < 5; attempt++ {
		err = cp.connect()
		if err == nil {
			return nil
		}
		log.Infof("Failed to reconnect to the yugabytedb control plane database: %s", err)
		time.Sleep(time.Second)
	}
	return fmt.Errorf("reconnect to yugabytedb control plane db: %w", err)
}

func (cp *YugabyteDB) connect() error {
	if cp.connPool != nil {
		return nil
	}
	connectionUri := os.Getenv(DB_CONN_STRING_ENV_VAR)

	connPool, err := pgxpool.New(context.Background(), connectionUri)
	if err != nil {
		return fmt.Errorf("error while connecting to yugabytedb control plane db. error: %w", err)
	}
	// pgxpool.New does not dial; ping so a bad conn string fails here rather than on first use.
	err = connPool.Ping(context.Background())
	if err != nil {
		connPool.Close()
		return fmt.Errorf("error while connecting to yugabytedb control plane db. error: %w", err)
	}

	cp.connPool = connPool
	return nil
}

func (cp *YugabyteDB) disconnect() {
	if cp.connPool == nil {
		return
	}

	cp.connPool.Close()
	cp.connPool = nil
}

// The control plane schema and its tables are created and owned by the control
// plane itself. Voyager only writes rows into them and never issues DDL.
const CONTROL_PLANE_SCHEMA = "voyager_core"
const MIGRATION_STATE_TABLE = "migration_state"
const METADATA_TABLE = "metadata"
const TABLE_DATA_PROGRESS_TABLE = "table_data_progress"

const QUALIFIED_MIGRATION_STATE_TABLE_NAME = CONTROL_PLANE_SCHEMA + "." + MIGRATION_STATE_TABLE
const QUALIFIED_METADATA_TABLE_NAME = CONTROL_PLANE_SCHEMA + "." + METADATA_TABLE
const QUALIFIED_TABLE_DATA_PROGRESS_TABLE_NAME = CONTROL_PLANE_SCHEMA + "." + TABLE_DATA_PROGRESS_TABLE

// Keyed by controlPlane.MIGRATION_PHASE_MAP.
const MIGRATION_PHASES_VALUES = `(VALUES
			(1, 'assessment.started',      'assessment.completed', 'assessment'),
			(2, 'schema.export_started',   'schema.exported',      NULL),
			(3, 'schema.analysis_started', 'schema.analyzed',      'schema_analysis'),
			(4, 'data.export_started',     'data.exported',        NULL),
			(5, 'schema.import_started',   'schema.imported',      NULL),
			(6, 'data.import_started',     'data.imported',        NULL)
		) AS phases (migration_phase, started_entry, done_entry, metadata_kind)`

func (cp *YugabyteDB) sendMigrationEvent(
	migrationEvent MigrationEvent) error {
	cmd := fmt.Sprintf(`
		WITH event AS (
			SELECT
				$1::uuid AS migration_id,
				phases.started_entry,
				phases.done_entry,
				phases.metadata_kind,
				$3::text AS status,
				NULLIF($4::text, '')::jsonb AS payload
			FROM %[3]s
			WHERE phases.migration_phase = $2::int
		), started AS (
			INSERT INTO %[1]s (migration_id, name, value)
			SELECT migration_id, started_entry, true
			FROM event
			WHERE status = 'IN PROGRESS'
		), done_unset AS (
			INSERT INTO %[1]s (migration_id, name, value)
			SELECT migration_id, done_entry, false
			FROM event
			WHERE status = 'IN PROGRESS'
			  -- a re-run: unset the done entry if it is currently set. Newest is by
			  -- set_at, since cached sequence blocks mean ids don't follow write order.
			  AND (SELECT ms.value FROM %[1]s ms
			       WHERE ms.migration_id = event.migration_id
			         AND ms.name = event.done_entry AND ms.scope = ''
			       ORDER BY ms.set_at DESC, ms.id DESC LIMIT 1)
		), done AS (
			INSERT INTO %[1]s (migration_id, name, value)
			SELECT migration_id, done_entry, true
			FROM event
			WHERE status = 'COMPLETED'
		)
		INSERT INTO %[2]s (migration_id, kind, payload, payload_version)
		SELECT migration_id, metadata_kind, payload,
			COALESCE(payload->>'PayloadVersion', payload->>'VoyagerVersion', '')
		FROM event
		WHERE status = 'COMPLETED' AND payload IS NOT NULL AND metadata_kind IS NOT NULL
	`, QUALIFIED_MIGRATION_STATE_TABLE_NAME, QUALIFIED_METADATA_TABLE_NAME, MIGRATION_PHASES_VALUES)

	var maxAttempts = 5

	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = cp.executeInsertQuery(cmd, migrationEvent)
		if err == nil {
			break
		} else {
			if attempt == maxAttempts {
				return goerrors.Errorf("error while sending migration event data to yugabytedb control plane for %d max attempts"+
					" query: %s. migration event data: %v. error: %w", maxAttempts, cmd, migrationEvent, err)
			}
		}
	}

	return nil
}

func (cp *YugabyteDB) sendTableDataProgress(
	tableMetricsList []controlPlane.TableMetrics) error {

	cmd := fmt.Sprintf("INSERT INTO %s ("+
		"migration_id, "+
		"db_role, "+
		"direction, "+
		"schema_name, "+
		"table_name, "+
		"status, "+
		"snapshot_rows_estimated, "+
		"snapshot_rows, "+
		"updated_at"+
		") SELECT "+
		"m.migration_uuid::uuid, "+
		"CASE m.migration_phase WHEN 4 THEN 'source' ELSE 'target' END, "+
		"CASE m.migration_phase WHEN 4 THEN 'export' ELSE 'import' END, "+
		"m.schema_name, "+
		"m.table_name, "+
		"CASE m.status WHEN 1 THEN 'in_progress' WHEN 2 THEN 'done' WHEN 3 THEN 'completed' ELSE 'not_started' END, "+
		"m.count_total_rows::bigint, "+
		"m.count_live_rows::bigint, "+
		"now() "+
		"FROM (VALUES ", QUALIFIED_TABLE_DATA_PROGRESS_TABLE_NAME)

	var valuesList []string
	for _, metrics := range tableMetricsList {
		value := fmt.Sprintf("('%s', '%s', '%s', %d, %d, %d, %d, '%s')",
			metrics.MigrationUUID,
			metrics.TableName,
			metrics.SchemaName,
			metrics.MigrationPhase,
			metrics.Status,
			metrics.CountLiveRows,
			metrics.CountTotalRows,
			metrics.InvocationTimestamp)

		valuesList = append(valuesList, value)
	}

	cmd += strings.Join(valuesList, ",")

	cmd += fmt.Sprintf(") AS m (migration_uuid, table_name, schema_name, migration_phase, status, " +
		"count_live_rows, count_total_rows, invocation_timestamp) " +
		"ON CONFLICT (migration_id, db_role, direction, schema_name, table_name) " +
		"DO UPDATE " +
		"SET " +
		"status = EXCLUDED.status," +
		"snapshot_rows_estimated = EXCLUDED.snapshot_rows_estimated," +
		"snapshot_rows = EXCLUDED.snapshot_rows," +
		"updated_at = EXCLUDED.updated_at;")

	return cp.executeCmdOnTarget(cmd)
}

func (cp *YugabyteDB) executeInsertQuery(cmd string,
	migrationEvent MigrationEvent) error {

	cp.Mutex.Lock()
	defer cp.Mutex.Unlock()

	var err error

	log.Infof("Executing on yugabytedb control plane DB: [%s] for [%+v]", cmd, migrationEvent)
	connPool, err := cp.getConnPool()
	if err != nil {
		return err
	}

	_, err = connPool.Exec(context.Background(), cmd,
		migrationEvent.MigrationUUID,
		migrationEvent.MigrationPhase,
		migrationEvent.Status,
		migrationEvent.Payload)

	if err == nil {
		return nil
	}
	log.Warnf("Error while running [%s]: %s", cmd, err)
	time.Sleep(time.Second)
	err2 := cp.reconnect()
	if err2 != nil {
		log.Warnf("Failed to reconnect to the yugabytedb control plane database: %s", err2)
	}

	if err != nil {
		return fmt.Errorf("couldn't execute command %s on yugabytedb control plane db. error: %w", cmd, err)
	}

	return nil
}

// Function to execute any cmd on yugabytedb control plane DB.
func (cp *YugabyteDB) executeCmdOnTarget(cmd string) error {
	maxAttempts := 5
	var err error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Infof("Executing on yugabytedb control plane DB: [%s]", cmd)
		connPool, err := cp.getConnPool()
		if err != nil {
			return err
		}

		_, err = connPool.Exec(context.Background(), cmd)
		if err == nil {
			return nil
		}
		log.Warnf("Error while running [%s] attempt %d: %s", cmd, attempt, err)
		time.Sleep(time.Second)
		err2 := cp.reconnect()
		if err2 != nil {
			log.Warnf("Failed to reconnect to the yugabytedb control plane database: %s", err2)
			break
		}
	}
	if err != nil {
		return fmt.Errorf("couldn't execute command %s on yugabytedb control plane db. error: %w", cmd, err)
	}
	return nil
}
