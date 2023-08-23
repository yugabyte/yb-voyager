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
package visualizer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type VisualizerYugabyteDB struct {
	conn               *pgxpool.Pool
	migrationDirectory string
}

// Initialize the target DB for visualisation metadata
func (db *VisualizerYugabyteDB) Init(exportDir string) error {
	err := db.connect()
	if err != nil {
		return err
	}

	err = db.createVoyagerSchema()
	if err != nil {
		return err
	}

	err = db.createYugabytedMetadataTable()
	if err != nil {
		return err
	}

	db.migrationDirectory = exportDir

	return nil
}

// Destroy the connection
func (db *VisualizerYugabyteDB) Finalize() {
	db.disconnect()
}

func (db *VisualizerYugabyteDB) getConn() *pgxpool.Pool {
	if db.conn == nil {
		utils.ErrExit("Called TargetDB.Conn() before TargetDB.Connect()")
	}
	return db.conn
}

func (db *VisualizerYugabyteDB) reconnect() error {
	var err error
	db.disconnect()
	for attempt := 1; attempt < 5; attempt++ {
		err = db.connect()
		if err == nil {
			return nil
		}
		log.Infof("Failed to reconnect to the target database: %s", err)
		time.Sleep(time.Second)
	}
	return fmt.Errorf("reconnect to target db: %w", err)
}

func (db *VisualizerYugabyteDB) connect() error {
	if db.conn != nil {
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

	db.conn = conn
	return nil
}

func (db *VisualizerYugabyteDB) disconnect() {
	if db.conn == nil {
		return
	}

	db.conn.Close()
	db.conn = nil
}

const BATCH_METADATA_TABLE_SCHEMA = "ybvoyager_visualizer"

// Create the visualisation schema
func (db *VisualizerYugabyteDB) createVoyagerSchema() error {
	cmd := fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s;`, BATCH_METADATA_TABLE_SCHEMA)

	err := db.executeCmdOnTarget(cmd)
	if err != nil {
		return err
	}

	return nil
}

const YUGABYTED_METADATA_TABLE_NAME = BATCH_METADATA_TABLE_SCHEMA + "." + "ybvoyager_visualizer_metadata"

// Create visualisation metadata table
func (db *VisualizerYugabyteDB) createYugabytedMetadataTable() error {
	cmd := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			migration_uuid UUID,
			migration_phase INT,
			invocation_sequence INT,
			migration_dir VARCHAR(250),
			database_name VARCHAR(250),
			schema_name VARCHAR(250),
			payload TEXT,
			status VARCHAR(30),
			invocation_timestamp TIMESTAMPTZ,
			PRIMARY KEY (migration_uuid, migration_phase, invocation_sequence)
			);`, YUGABYTED_METADATA_TABLE_NAME)

	err := db.executeCmdOnTarget(cmd)
	if err != nil {
		return err
	}

	return nil
}

const YUGABYTED_TABLE_METRICS_TABLE_NAME = BATCH_METADATA_TABLE_SCHEMA + "." + "ybvoyager_visualizer_table_metrics"

// Create table metrics table
func (db *VisualizerYugabyteDB) CreateYugabytedTableMetricsTable() error {
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

	err := db.executeCmdOnTarget(cmd)
	if err != nil {
		return err
	}

	return nil
}

// Get the latest invocation sequence for a given migration_uuid and migration phase
func (db *VisualizerYugabyteDB) GetInvocationSequence(mUUID uuid.UUID, phase int) (int, error) {
	cmd := fmt.Sprintf(`SELECT MAX(invocation_sequence) AS latest_sequence 
		FROM %s 
		WHERE migration_uuid = '%s' AND migration_phase = %d`, YUGABYTED_METADATA_TABLE_NAME,
		mUUID, phase)

	log.Infof("Executing on target DB: [%s]", cmd)

	var latestSequence *int
	conn := db.getConn()
	err := conn.QueryRow(context.Background(), cmd).Scan(&latestSequence)
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
func (db *VisualizerYugabyteDB) SendVisualizerDBPayload(
	visualizerDBPayload VisualizerDBPayload) error {
	cmd := fmt.Sprintf("INSERT INTO %s ("+
		"migration_uuid, "+
		"migration_phase, "+
		"invocation_sequence, "+
		"migration_dir, "+
		"database_name, "+
		"schema_name, "+
		"payload, "+
		"status, "+
		"invocation_timestamp"+
		") VALUES ("+
		"'%s', %d, %d, '%s', '%s', '%s', '%s', '%s', '%s'"+
		")", YUGABYTED_METADATA_TABLE_NAME,
		visualizerDBPayload.MigrationUUID,
		visualizerDBPayload.MigrationPhase,
		visualizerDBPayload.InvocationSequence,
		db.migrationDirectory,
		visualizerDBPayload.DatabaseName,
		visualizerDBPayload.SchemaName,
		visualizerDBPayload.Payload,
		visualizerDBPayload.Status,
		visualizerDBPayload.InvocationTimestamp)

	err := db.executeCmdOnTarget(cmd)
	if err != nil {
		return err
	}

	return nil
}

// Send Table metrics
func (db *VisualizerYugabyteDB) SendVisualizerTableMetrics(
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

	err := db.executeCmdOnTarget(cmd)
	if err != nil {
		return err
	}

	return nil
}

// Function to execute any cmd on target DB.
func (db *VisualizerYugabyteDB) executeCmdOnTarget(cmd string) error {
	maxAttempts := 5
	var err error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Infof("Executing on target DB: [%s]", cmd)
		conn := db.getConn()
		_, err = conn.Exec(context.Background(), cmd)
		if err == nil {
			return nil
		}
		log.Warnf("Error while running [%s] attempt %d: %s", cmd, attempt, err)
		time.Sleep(time.Second)
		err2 := db.reconnect()
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
