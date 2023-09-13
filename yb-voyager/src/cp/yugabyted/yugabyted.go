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
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type YugabyteD struct {
	sync.Mutex
	conn               *pgxpool.Pool
	migrationDirectory string
}

func New(exportDir string) *YugabyteD {
	return &YugabyteD{migrationDirectory: exportDir}
}

// Initialize the target DB for visualisation metadata
func (cp *YugabyteD) Init() error {
	err := cp.connect()
	if err != nil {
		return err
	}

	err = cp.createVoyagerSchema()
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

func (cp *YugabyteD) createAndSendExportSchemaEvent(exportSchemaEvent *cp.ExportSchemaEvent,
	status string) {

	defer utils.WaitGroup.Done()

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	invocationSequence, err := cp.getInvocationSequence(exportSchemaEvent.MigrationUUID,
		MIGRATION_PHASE_MAP["EXPORT SCHEMA"])

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
		return
	}

	dbPayload := CreateVisualzerDBPayload(
		exportSchemaEvent.MigrationUUID,
		MIGRATION_PHASE_MAP["EXPORT SCHEMA"],
		invocationSequence,
		exportSchemaEvent.DatabaseName,
		exportSchemaEvent.SchemaName,
		"",
		exportSchemaEvent.DBType,
		status,
		timestamp)

	err = cp.sendVisualizerDBPayload(dbPayload)

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
	}

}

func (cp *YugabyteD) ExportSchemaStarted(exportSchemaEvent *cp.ExportSchemaEvent) {
	cp.createAndSendExportSchemaEvent(exportSchemaEvent, "IN PROGRESS")
}

func (cp *YugabyteD) ExportSchemaCompleted(exportSchemaEvent *cp.ExportSchemaEvent) {
	cp.createAndSendExportSchemaEvent(exportSchemaEvent, "COMPLETED")
}

func (cp *YugabyteD) SchemaAnalysisStarted(schemaAnalysisEvent *cp.SchemaAnalysisEvent) {

	defer utils.WaitGroup.Done()
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	invocationSequence, err := cp.getInvocationSequence(schemaAnalysisEvent.MigrationUUID,
		MIGRATION_PHASE_MAP["ANALYZE SCHEMA"])

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
		return
	}

	dbPayload := CreateVisualzerDBPayload(
		schemaAnalysisEvent.MigrationUUID,
		MIGRATION_PHASE_MAP["ANALYZE SCHEMA"],
		invocationSequence,
		schemaAnalysisEvent.DatabaseName,
		schemaAnalysisEvent.SchemaName,
		"",
		schemaAnalysisEvent.DBType,
		"IN PROGRESS",
		timestamp)

	err = cp.sendVisualizerDBPayload(dbPayload)

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
	}
}

func (cp *YugabyteD) SubmitSchemaAnalysisReport(schemaAnalysisReport *cp.SchemaAnalysisReport) {

	defer utils.WaitGroup.Done()
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	invocationSequence, err := cp.getInvocationSequence(schemaAnalysisReport.MigrationUUID,
		MIGRATION_PHASE_MAP["ANALYZE SCHEMA"])

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
		return
	}

	dbPayload := CreateVisualzerDBPayload(
		schemaAnalysisReport.MigrationUUID,
		MIGRATION_PHASE_MAP["ANALYZE SCHEMA"],
		invocationSequence,
		schemaAnalysisReport.DatabaseName,
		schemaAnalysisReport.SchemaName,
		schemaAnalysisReport.Payload,
		schemaAnalysisReport.DBType,
		"COMPLETED",
		timestamp)

	err = cp.sendVisualizerDBPayload(dbPayload)

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
	}
}

func (cp *YugabyteD) createAndSendExportSnapshotEvent(snapshotExportEvent *cp.SnapshotExportEvent,
	status string) {

	defer utils.WaitGroup.Done()
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	invocationSequence, err := cp.getInvocationSequence(snapshotExportEvent.MigrationUUID,
		MIGRATION_PHASE_MAP["EXPORT DATA"])

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
		return
	}

	dbPayload := CreateVisualzerDBPayload(
		snapshotExportEvent.MigrationUUID,
		MIGRATION_PHASE_MAP["EXPORT DATA"],
		invocationSequence,
		snapshotExportEvent.DatabaseName,
		snapshotExportEvent.SchemaName,
		"",
		snapshotExportEvent.DBType,
		status,
		timestamp)

	err = cp.sendVisualizerDBPayload(dbPayload)

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
	}
}

func (cp *YugabyteD) SnapshotExportStarted(snapshotExportEvent *cp.SnapshotExportEvent) {
	cp.createAndSendExportSnapshotEvent(snapshotExportEvent, "IN PROGRESS")
}

func (cp *YugabyteD) UpdateExportedRowCount(
	snapshotExportTablesMetrics []*cp.SnapshotExportTableMetrics) {

	defer utils.WaitGroup.Done()

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	var snapshotMigrateAllTableMetrics []VisualizerTableMetrics
	for _, exportTableMetrics := range snapshotExportTablesMetrics {
		snapshotMigrateTableMetrics := CreateVisualzerDBTableMetrics(
			exportTableMetrics.MigrationUUID,
			exportTableMetrics.TableName,
			exportTableMetrics.Schema,
			MIGRATION_PHASE_MAP["EXPORT DATA"],
			exportTableMetrics.Status,
			exportTableMetrics.CountLiveRows,
			exportTableMetrics.CountTotalRows,
			timestamp)
		snapshotMigrateAllTableMetrics = append(snapshotMigrateAllTableMetrics,
			snapshotMigrateTableMetrics)
	}

	err := cp.sendVisualizerTableMetrics(snapshotMigrateAllTableMetrics)

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
	}
}

func (cp *YugabyteD) SnapshotExportCompleted(snapshotExportEvent *cp.SnapshotExportEvent) {
	cp.createAndSendExportSnapshotEvent(snapshotExportEvent, "COMPLETED")
}

func (cp *YugabyteD) createAndSendImportSchemaEvent(importSchemaEvent *cp.ImportSchemaEvent,
	status string) {

	defer utils.WaitGroup.Done()
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	invocationSequence, err := cp.getInvocationSequence(importSchemaEvent.MigrationUUID,
		MIGRATION_PHASE_MAP["IMPORT SCHEMA"])

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
		return
	}

	dbPayload := CreateVisualzerDBPayload(
		importSchemaEvent.MigrationUUID,
		MIGRATION_PHASE_MAP["IMPORT SCHEMA"],
		invocationSequence,
		importSchemaEvent.DatabaseName,
		importSchemaEvent.SchemaName,
		"",
		"",
		status,
		timestamp)

	err = cp.sendVisualizerDBPayload(dbPayload)

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
	}

}

func (cp *YugabyteD) ImportSchemaStarted(importSchemaEvent *cp.ImportSchemaEvent) {
	cp.createAndSendImportSchemaEvent(importSchemaEvent, "IN PROGRESS")
}

func (cp *YugabyteD) ImportSchemaCompleted(importSchemaEvent *cp.ImportSchemaEvent) {
	cp.createAndSendImportSchemaEvent(importSchemaEvent, "COMPLETED")
}

func (cp *YugabyteD) createAndSendImportSnapshotEvent(snapshotImportEvent *cp.SnapshotImportEvent,
	status string) {

	defer utils.WaitGroup.Done()
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	invocationSequence, err := cp.getInvocationSequence(snapshotImportEvent.MigrationUUID,
		MIGRATION_PHASE_MAP["IMPORT DATA"])

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
		return
	}

	dbPayload := CreateVisualzerDBPayload(
		snapshotImportEvent.MigrationUUID,
		MIGRATION_PHASE_MAP["IMPORT DATA"],
		invocationSequence,
		snapshotImportEvent.DatabaseName,
		snapshotImportEvent.SchemaName,
		"",
		"",
		status,
		timestamp)

	err = cp.sendVisualizerDBPayload(dbPayload)

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
	}
}

func (cp *YugabyteD) SnapshotImportStarted(snapshotImportEvent *cp.SnapshotImportEvent) {
	cp.createAndSendImportSnapshotEvent(snapshotImportEvent, "IN PROGRESS")
}

func (cp *YugabyteD) UpdateImportedRowCount(
	snapshotImportTableMetrics []*cp.SnapshotImportTableMetrics) {

	defer utils.WaitGroup.Done()
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// var importTableMetricsList []VisualizerTableMetrics

	var snapshotMigrateAllTableMetrics []VisualizerTableMetrics
	for _, importtableMetrics := range snapshotImportTableMetrics {
		snapshotMigrateTableMetrics := CreateVisualzerDBTableMetrics(
			importtableMetrics.MigrationUUID,
			importtableMetrics.TableName,
			importtableMetrics.Schema,
			MIGRATION_PHASE_MAP["IMPORT DATA"],
			importtableMetrics.Status,
			importtableMetrics.CountLiveRows,
			importtableMetrics.CountTotalRows,
			timestamp)
		snapshotMigrateAllTableMetrics = append(snapshotMigrateAllTableMetrics, snapshotMigrateTableMetrics)
	}

	err := cp.sendVisualizerTableMetrics(snapshotMigrateAllTableMetrics)

	if err != nil {
		log.Warnf("Cannot send metadata for visualization. %s", err)
	}
}

func (cp *YugabyteD) SnapshotImportCompleted(snapshotImportEvent *cp.SnapshotImportEvent) {
	cp.createAndSendImportSnapshotEvent(snapshotImportEvent, "COMPLETED")
}

// Destroy the connection
func (cp *YugabyteD) Finalize() {
	cp.disconnect()
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
	connectionUri := os.Getenv("TARGET_YBDB_CONN_URI")
	if connectionUri == "" {
		log.Warnf("Environment variable TARGET_YBDB_CONN_URI not set. " +
			"Will not be able to visualize in yugabyted UI.")
		return fmt.Errorf("connection uri string not set")
	}

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
			return 1, nil
		} else {
			return 0, fmt.Errorf("couldn't get the latest sequence number: %w", err)
		}
	}

	if latestSequence != nil {
		return *latestSequence + 1, nil
	} else {
		return 1, nil
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
	// err := cp.executeInsertQuery(cmd, visualizerDBPayload)
	// if err != nil {
	// 	return err
	// }

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
