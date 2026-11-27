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
	"errors"
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
	rowCountUpdateEventChan chan ([]TableProgress)
	// connMutex guards connPool: both publisher goroutines and Finalize use it,
	// and a failed write replaces it.
	connMutex          sync.Mutex
	connPool           *pgxpool.Pool
	lastRowCountUpdate map[string]time.Time

	// Step claiming (stepClaiming.go). claimingEnv.mode is off by default.
	// stepExecution is guarded by stepExecutionMu; the step execution's own state by its mu.
	claimingEnv       stepClaimingEnv
	heartbeatInterval time.Duration
	stepExecutionMu   sync.Mutex
	stepExecution     *stepExecution
}

func New(exportDir string) *YugabyteDB {
	return &YugabyteDB{
		heartbeatInterval: defaultHeartbeatInterval,
	}
}

func (cp *YugabyteDB) Init() error {
	cp.eventChan = make(chan MigrationEvent, 100)
	cp.rowCountUpdateEventChan = make(chan []TableProgress, 200)

	var err error
	cp.claimingEnv, err = readStepClaimingEnv()
	if err != nil {
		return err
	}
	err = cp.connect()
	if err != nil {
		return err
	}
	err = cp.checkTables()
	if err == nil && cp.StepClaimingEnabled() {
		err = cp.checkStepClaimingTables()
	}
	if err != nil {
		cp.disconnect()
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
	cp.connMutex.Lock()
	defer cp.connMutex.Unlock()
	cp.disconnect()
}

func (cp *YugabyteDB) eventPublisher() {
	defer cp.panicHandler()
	for {
		event := <-cp.eventChan
		err := cp.sendMigrationEvent(event)
		if err != nil {
			log.Errorf("Couldn't send migration event to the control plane. %s", err)
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
			log.Errorf("Couldn't send table data progress to the control plane. %s", err)
		}
		cp.waitGroup.Done()
	}
}

func (cp *YugabyteDB) createAndSendEvent(event *controlPlane.BaseEvent, status string, payload string) {

	migrationEvent := MigrationEvent{
		MigrationUUID: event.MigrationUUID,
		EventType:     event.EventType,
		Payload:       payload,
		Status:        status,
	}

	// Add before the send: the publisher may call Done as soon as it receives.
	cp.waitGroup.Add(1)
	select {
	case cp.eventChan <- migrationEvent:
	default:
		cp.waitGroup.Done()
		log.Warnf("Could not publish migration event %v", migrationEvent)
	}
}

func (cp *YugabyteDB) createAndSendUpdateRowCountEvent(events []*controlPlane.BaseUpdateRowCountEvent) {

	var rowCountUpdateEvent []TableProgress

	for _, event := range events {
		rowCountUpdateEvent = append(rowCountUpdateEvent, TableProgress{
			MigrationUUID:     event.MigrationUUID,
			EventType:         event.EventType,
			SchemaName:        strings.Join(event.SchemaNames, "|"),
			TableName:         event.TableName,
			Status:            event.Status,
			TotalRowCount:     event.TotalRowCount,
			CompletedRowCount: event.CompletedRowCount,
		})
	}

	cp.waitGroup.Add(1)
	select {
	case cp.rowCountUpdateEventChan <- rowCountUpdateEvent:
	default:
		cp.waitGroup.Done()
		log.Warnf("Could not publish row count update event %v", rowCountUpdateEvent)
	}
}

func (cp *YugabyteDB) MigrationAssessmentStarted(ev *controlPlane.MigrationAssessmentStartedEvent) {
	cp.createAndSendEvent(&ev.BaseEvent, STATUS_IN_PROGRESS, "")
}

func (cp *YugabyteDB) MigrationAssessmentCompleted(ev *controlPlane.MigrationAssessmentCompletedEvent) {
	payloadStr, ok := ev.Report.(string)
	if !ok {
		log.Errorf("Assessment report not sent to the yugabytedb control plane: expected a JSON string, got %T", ev.Report)
		return
	}
	cp.createAndSendEvent(&ev.BaseEvent, STATUS_COMPLETED, payloadStr)
}

func (cp *YugabyteDB) ExportSchemaStarted(exportSchemaEvent *controlPlane.ExportSchemaStartedEvent) {
	cp.createAndSendEvent(&exportSchemaEvent.BaseEvent, STATUS_IN_PROGRESS, "")
}

func (cp *YugabyteDB) ExportSchemaCompleted(exportSchemaEvent *controlPlane.ExportSchemaCompletedEvent) {
	cp.createAndSendEvent(&exportSchemaEvent.BaseEvent, STATUS_COMPLETED, "")
}

func (cp *YugabyteDB) SchemaAnalysisStarted(schemaAnalysisEvent *controlPlane.SchemaAnalysisStartedEvent) {
	cp.createAndSendEvent(&schemaAnalysisEvent.BaseEvent, STATUS_IN_PROGRESS, "")
}

func (cp *YugabyteDB) SchemaAnalysisIterationCompleted(schemaAnalysisReport *controlPlane.SchemaAnalysisIterationCompletedEvent) {

	jsonBytes, err := json.Marshal(schemaAnalysisReport.AnalysisReport)
	if err != nil {
		log.Warnf("%v", err)
		return
	}
	payload := string(jsonBytes)

	cp.createAndSendEvent(&schemaAnalysisReport.BaseEvent, STATUS_COMPLETED, payload)
}

func (cp *YugabyteDB) SnapshotExportStarted(snapshotExportEvent *controlPlane.SnapshotExportStartedEvent) {
	cp.createAndSendEvent(&snapshotExportEvent.BaseEvent, STATUS_IN_PROGRESS, "")
}

func (cp *YugabyteDB) SnapshotExportCompleted(snapshotExportEvent *controlPlane.SnapshotExportCompletedEvent) {
	cp.createAndSendEvent(&snapshotExportEvent.BaseEvent, STATUS_COMPLETED, "")
}

func (cp *YugabyteDB) UpdateExportedRowCount(
	snapshotExportTablesMetrics []*controlPlane.UpdateExportedRowCountEvent) {

	cp.Mutex.Lock()
	defer cp.Mutex.Unlock()

	var updateExportedRowCountEvents []*controlPlane.BaseUpdateRowCountEvent
	for _, updateExportedRowCountEvent := range snapshotExportTablesMetrics {

		lastRowCountUpdateTime, check := cp.lastRowCountUpdate[updateExportedRowCountEvent.TableName]

		if updateExportedRowCountEvent.Status == STATUS_COMPLETED {
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
	cp.createAndSendEvent(&importSchemaEvent.BaseEvent, STATUS_IN_PROGRESS, "")
}

func (cp *YugabyteDB) ImportSchemaCompleted(importSchemaEvent *controlPlane.ImportSchemaCompletedEvent) {
	cp.createAndSendEvent(&importSchemaEvent.BaseEvent, STATUS_COMPLETED, "")
}

func (cp *YugabyteDB) SnapshotImportStarted(snapshotImportEvent *controlPlane.SnapshotImportStartedEvent) {
	cp.createAndSendEvent(&snapshotImportEvent.BaseEvent, STATUS_IN_PROGRESS, "")
}

func (cp *YugabyteDB) SnapshotImportCompleted(snapshotImportEvent *controlPlane.SnapshotImportCompletedEvent) {
	cp.createAndSendEvent(&snapshotImportEvent.BaseEvent, STATUS_COMPLETED, "")
}

func (cp *YugabyteDB) UpdateImportedRowCount(
	snapshotImportTableMetrics []*controlPlane.UpdateImportedRowCountEvent) {

	cp.Mutex.Lock()
	defer cp.Mutex.Unlock()

	var updateImportedRowCountEvents []*controlPlane.BaseUpdateRowCountEvent
	for _, updateImportedRowCountEvent := range snapshotImportTableMetrics {

		lastRowCountUpdateTime, check := cp.lastRowCountUpdate[updateImportedRowCountEvent.TableName]

		if updateImportedRowCountEvent.Status == STATUS_COMPLETED {
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

// checkTables fails when a table this control plane writes is missing or lacks
// a privilege the writes use. Voyager does not create them, and a write that
// fails later is only logged, so this is the one point where the command can
// still stop.
func (cp *YugabyteDB) checkTables() error {
	for _, table := range []string{
		QUALIFIED_MIGRATION_STATE_TABLE_NAME,
		QUALIFIED_METADATA_TABLE_NAME,
		QUALIFIED_TABLE_DATA_PROGRESS_TABLE_NAME,
	} {
		// One call per privilege: a comma-separated list is true when any one
		// of them is held.
		var canWrite *bool
		err := cp.connPool.QueryRow(context.Background(),
			`SELECT has_table_privilege(t.oid, 'INSERT')
				AND has_table_privilege(t.oid, 'SELECT')
				AND has_table_privilege(t.oid, 'UPDATE')
			FROM (SELECT to_regclass($1)::oid AS oid) t`, table).Scan(&canWrite)
		if err != nil {
			return fmt.Errorf("check table %s on yugabytedb control plane db: %w", table, err)
		}
		if canWrite == nil {
			return goerrors.Errorf("table %s does not exist on the yugabytedb control plane db", table)
		}
		if !*canWrite {
			return goerrors.Errorf("INSERT, SELECT and UPDATE privileges are needed on table %s of the yugabytedb control plane db", table)
		}
	}
	return nil
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

const STATUS_IN_PROGRESS = "IN PROGRESS"
const STATUS_COMPLETED = "COMPLETED"

// The schema analysis report carries no version of its own, so its document
// format is versioned here. Bump it when utils.SchemaReport changes shape.
const SCHEMA_ANALYSIS_PAYLOAD_VERSION = "1.0"

// migrationPhase is what one voyager command reports: the migration_state
// entries set when it starts and completes and, when it produces a document,
// that document's metadata kind and how to find its version.
type migrationPhase struct {
	startedEntry   string
	doneEntry      string
	metadataKind   string
	payloadVersion func(payload string) (string, error)
}

// Keyed by the event type of controlPlane.BaseEvent.
var migrationPhases = map[string]migrationPhase{
	"ASSESS MIGRATION": {"assessment.started", "assessment.completed", "assessment", payloadVersionFromDocument},
	"EXPORT SCHEMA":    {"schema.export_started", "schema.exported", "", nil},
	"ANALYZE SCHEMA":   {"schema.analysis_started", "schema.analyzed", "schema_analysis", schemaAnalysisPayloadVersion},
	"EXPORT DATA":      {"data.export_started", "data.exported", "", nil},
	"IMPORT SCHEMA":    {"schema.import_started", "schema.imported", "", nil},
	"IMPORT DATA":      {"data.import_started", "data.imported", "", nil},
}

func payloadVersionFromDocument(payload string) (string, error) {
	var document struct {
		PayloadVersion string
	}
	err := json.Unmarshal([]byte(payload), &document)
	if err != nil {
		return "", fmt.Errorf("parse payload: %w", err)
	}
	if document.PayloadVersion == "" {
		return "", goerrors.Errorf("payload has no PayloadVersion")
	}
	return document.PayloadVersion, nil
}

func schemaAnalysisPayloadVersion(string) (string, error) {
	return SCHEMA_ANALYSIS_PAYLOAD_VERSION, nil
}

func (cp *YugabyteDB) sendMigrationEvent(migrationEvent MigrationEvent) error {
	phase, ok := migrationPhases[migrationEvent.EventType]
	if !ok {
		return goerrors.Errorf("unknown migration event type %q", migrationEvent.EventType)
	}
	migrationID := migrationEvent.MigrationUUID.String()

	log.Infof("Sending migration event to the yugabytedb control plane: %s %s for migration %s",
		migrationEvent.EventType, migrationEvent.Status, migrationID)

	// With no open step execution the statements are exactly the unstamped ones, so
	// the step_execution_id column is not touched.
	stepExecutionID := cp.activeStepExecutionID()
	var stampColumn string
	stampValue := func(n int) string {
		if stepExecutionID == nil {
			return ""
		}
		return fmt.Sprintf(", $%d::bigint", n)
	}
	if stepExecutionID != nil {
		stampColumn = ", " + STEP_EXECUTION_ID_COLUMN
	}
	withStamp := func(args ...any) []any {
		if stepExecutionID != nil {
			args = append(args, *stepExecutionID)
		}
		return args
	}

	switch migrationEvent.Status {
	case STATUS_IN_PROGRESS:
		cmd := fmt.Sprintf(`
			WITH started AS (
				INSERT INTO %[1]s (migration_id, name, value%[2]s)
				VALUES ($1::uuid, $2::text, true%[3]s)
			)
			INSERT INTO %[1]s (migration_id, name, value%[2]s)
			SELECT $1::uuid, $3::text, false%[3]s
			-- a re-run: unset the done entry if it is currently set. Newest is by
			-- set_at, since cached sequence blocks mean ids don't follow write order.
			WHERE (SELECT ms.value FROM %[1]s ms
			       WHERE ms.migration_id = $1::uuid
			         AND ms.name = $3::text AND ms.scope = ''
			       ORDER BY ms.set_at DESC, ms.id DESC LIMIT 1)
		`, QUALIFIED_MIGRATION_STATE_TABLE_NAME, stampColumn, stampValue(4))
		return cp.exec(cmd, withStamp(migrationID, phase.startedEntry, phase.doneEntry)...)

	case STATUS_COMPLETED:
		closing := cp.stepExecutionDoneBy(migrationEvent.EventType)
		run := func(cmd string, args []any) error {
			if closing != nil {
				return cp.execAndCloseStepExecution(closing, cmd, args...)
			}
			return cp.exec(cmd, args...)
		}
		if phase.metadataKind == "" {
			cmd := fmt.Sprintf(`
				INSERT INTO %s (migration_id, name, value%s)
				VALUES ($1::uuid, $2::text, true%s)
			`, QUALIFIED_MIGRATION_STATE_TABLE_NAME, stampColumn, stampValue(3))
			return run(cmd, withStamp(migrationID, phase.doneEntry))
		}
		// The done entry and the document are one statement, so a phase is
		// never recorded as done without the document it produced.
		if migrationEvent.Payload == "" {
			return goerrors.Errorf("%s completed without its %s document", migrationEvent.EventType, phase.metadataKind)
		}
		payloadVersion, err := phase.payloadVersion(migrationEvent.Payload)
		if err != nil {
			return fmt.Errorf("payload version of the %s document: %w", phase.metadataKind, err)
		}
		cmd := fmt.Sprintf(`
			WITH done AS (
				INSERT INTO %[1]s (migration_id, name, value%[3]s)
				VALUES ($1::uuid, $2::text, true%[4]s)
			)
			INSERT INTO %[2]s (migration_id, kind, payload, payload_version%[3]s)
			VALUES ($1::uuid, $3::text, $4::jsonb, $5::text%[4]s)
		`, QUALIFIED_MIGRATION_STATE_TABLE_NAME, QUALIFIED_METADATA_TABLE_NAME, stampColumn, stampValue(6))
		return run(cmd, withStamp(migrationID, phase.doneEntry, phase.metadataKind, migrationEvent.Payload, payloadVersion))

	default:
		return goerrors.Errorf("unknown migration event status %q", migrationEvent.Status)
	}
}

// Keyed by the event type of controlPlane.BaseUpdateRowCountEvent.
var tableProgressStreams = map[string]struct{ dbRole, direction string }{
	"EXPORT DATA": {"source", "export"},
	"IMPORT DATA": {"target", "import"},
}

// Keyed by the status of controlPlane.BaseUpdateRowCountEvent.
var tableProgressStatuses = map[string]string{
	"NOT STARTED":      "not_started",
	STATUS_IN_PROGRESS: "in_progress",
	"DONE":             "done",
	STATUS_COMPLETED:   "completed",
}

func (cp *YugabyteDB) sendTableDataProgress(tableProgressList []TableProgress) error {
	// One array per column, so table and schema names are bound, not spliced
	// into the statement.
	cmd := fmt.Sprintf(`
		INSERT INTO %s (
			migration_id, db_role, direction, schema_name, table_name,
			status, snapshot_rows_estimated, snapshot_rows, updated_at
		)
		SELECT m.migration_id::uuid, m.db_role, m.direction, m.schema_name, m.table_name,
			m.status, m.total_rows, m.completed_rows, now()
		FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::text[], $6::text[], $7::bigint[], $8::bigint[])
			AS m (migration_id, db_role, direction, schema_name, table_name, status, total_rows, completed_rows)
		ON CONFLICT (migration_id, db_role, direction, schema_name, table_name)
		DO UPDATE SET
			status = EXCLUDED.status,
			snapshot_rows_estimated = EXCLUDED.snapshot_rows_estimated,
			snapshot_rows = EXCLUDED.snapshot_rows,
			updated_at = EXCLUDED.updated_at
	`, QUALIFIED_TABLE_DATA_PROGRESS_TABLE_NAME)

	n := len(tableProgressList)
	migrationIDs := make([]string, 0, n)
	dbRoles := make([]string, 0, n)
	directions := make([]string, 0, n)
	schemaNames := make([]string, 0, n)
	tableNames := make([]string, 0, n)
	statuses := make([]string, 0, n)
	totalRows := make([]int64, 0, n)
	completedRows := make([]int64, 0, n)
	for _, progress := range tableProgressList {
		stream, ok := tableProgressStreams[progress.EventType]
		if !ok {
			return goerrors.Errorf("unknown row count event type %q for table %s", progress.EventType, progress.TableName)
		}
		status, ok := tableProgressStatuses[progress.Status]
		if !ok {
			return goerrors.Errorf("unknown row count status %q for table %s", progress.Status, progress.TableName)
		}
		migrationIDs = append(migrationIDs, progress.MigrationUUID.String())
		dbRoles = append(dbRoles, stream.dbRole)
		directions = append(directions, stream.direction)
		schemaNames = append(schemaNames, progress.SchemaName)
		tableNames = append(tableNames, progress.TableName)
		statuses = append(statuses, status)
		totalRows = append(totalRows, progress.TotalRowCount)
		completedRows = append(completedRows, progress.CompletedRowCount)
	}

	return cp.exec(cmd, migrationIDs, dbRoles, directions, schemaNames, tableNames, statuses, totalRows, completedRows)
}

// exec runs cmd on the control plane database. A failed connect or statement
// is retried on a fresh connection, and the last error is returned if every
// attempt fails.
func (cp *YugabyteDB) exec(cmd string, args ...any) error {
	return cp.withRetry(cmd, func(connPool *pgxpool.Pool) error {
		_, err := connPool.Exec(context.Background(), cmd, args...)
		return err
	})
}

// withRetry runs fn with the same retry policy as exec. errStepExecutionNotRunning is
// final: retrying cannot change it.
func (cp *YugabyteDB) withRetry(cmd string, fn func(connPool *pgxpool.Pool) error) error {
	cp.connMutex.Lock()
	defer cp.connMutex.Unlock()

	maxAttempts := 5
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var connPool *pgxpool.Pool
		connPool, err = cp.getConnPool()
		if err == nil {
			err = fn(connPool)
			if err == nil {
				return nil
			}
			if errors.Is(err, errStepExecutionNotRunning) {
				return err
			}
		}
		log.Warnf("Error while running [%s] attempt %d: %s", cmd, attempt, err)
		time.Sleep(time.Second)
		// A failed reconnect leaves no pool, so the next attempt connects again.
		reconnectErr := cp.reconnect()
		if reconnectErr != nil {
			log.Warnf("Failed to reconnect to the yugabytedb control plane database: %s", reconnectErr)
		}
	}
	return fmt.Errorf("couldn't execute command %s on yugabytedb control plane db. error: %w", cmd, err)
}
