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
package importdata

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// Prometheus metrics configuration
var (
	IMPORT_DATA_TARGET_PROMETHEUS_METRICS_PORT         = "9101"
	IMPORT_DATA_FILE_PROMETHEUS_METRICS_PORT           = "9102"
	IMPORT_DATA_SOURCE_REPLICA_PROMETHEUS_METRICS_PORT = "9103"
	IMPORT_DATA_SOURCE_PROMETHEUS_METRICS_PORT         = "9104"
)

// promSessionID is created on package init and used for all metrics
var promSessionID string
var promMigrationUUID uuid.UUID

func init() {
	// Create a unique session ID based on formatted timestamp
	promSessionID = time.Now().Format("20060102-150405")

	// override ports from env variables if set
	if port, ok := os.LookupEnv("IMPORT_DATA_TARGET_PROMETHEUS_METRICS_PORT"); ok {
		IMPORT_DATA_TARGET_PROMETHEUS_METRICS_PORT = port
	}
	if port, ok := os.LookupEnv("IMPORT_DATA_FILE_PROMETHEUS_METRICS_PORT"); ok {
		IMPORT_DATA_FILE_PROMETHEUS_METRICS_PORT = port
	}
	if port, ok := os.LookupEnv("IMPORT_DATA_SOURCE_REPLICA_PROMETHEUS_METRICS_PORT"); ok {
		IMPORT_DATA_SOURCE_REPLICA_PROMETHEUS_METRICS_PORT = port
	}
}

func StartPrometheusMetricsServer(importerRole string, migrationUUID uuid.UUID) error {
	promMigrationUUID = migrationUUID
	return utils.StartPrometheusMetricsServer(getPrometheusPort(importerRole))
}

func getPrometheusPort(importerRole string) string {
	switch importerRole {
	case constants.TARGET_DB_IMPORTER_ROLE:
		return IMPORT_DATA_TARGET_PROMETHEUS_METRICS_PORT
	case constants.IMPORT_FILE_ROLE:
		return IMPORT_DATA_FILE_PROMETHEUS_METRICS_PORT
	case constants.SOURCE_REPLICA_DB_IMPORTER_ROLE:
		return IMPORT_DATA_SOURCE_REPLICA_PROMETHEUS_METRICS_PORT
	case constants.SOURCE_DB_IMPORTER_ROLE:
		return IMPORT_DATA_SOURCE_PROMETHEUS_METRICS_PORT
	default:
		panic(fmt.Sprintf("unsupported importer role: %s", importerRole))
	}
}

// ================================= Metrics  ================================= //

var (
	// Total rows imported during snapshot
	importRowsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_rows_total",
			Help: "Total rows imported during snapshot",
		},
		[]string{"migration_uuid", "session_id", "importer_role", "table_name", "schema_name"},
	)

	// Total bytes imported during snapshot
	importBytesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_bytes_total",
			Help: "Total bytes imported during snapshot",
		},
		[]string{"migration_uuid", "session_id", "importer_role", "table_name", "schema_name"},
	)

	// Total number of batches created for import
	snapshotBatchCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_batch_created_total",
			Help: "Total number of batches created for import",
		},
		[]string{"migration_uuid", "session_id", "importer_role", "table_name", "schema_name"},
	)

	// Total number of batches submitted to worker pool
	snapshotBatchSubmitted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_batch_submitted_total",
			Help: "Total number of batches submitted to worker pool",
		},
		[]string{"migration_uuid", "session_id", "importer_role", "table_name", "schema_name"},
	)

	// Total number of batches successfully ingested
	snapshotBatchIngested = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_batch_ingested_total",
			Help: "Total number of batches successfully ingested",
		},
		[]string{"migration_uuid", "session_id", "importer_role", "table_name", "schema_name"},
	)
)

// RecordPrometheusSnapshotBatchIngested records metrics for a completed batch import
func RecordPrometheusSnapshotBatchIngested(tableNameTup sqlname.NameTuple, importerRole string, rows, bytes int64) {
	schemaName, tableName := tableNameTup.ForKeyTableSchema()
	importRowsTotal.WithLabelValues(promMigrationUUID.String(), promSessionID, importerRole, tableName, schemaName).Add(float64(rows))
	importBytesTotal.WithLabelValues(promMigrationUUID.String(), promSessionID, importerRole, tableName, schemaName).Add(float64(bytes))
	snapshotBatchIngested.WithLabelValues(promMigrationUUID.String(), promSessionID, importerRole, tableName, schemaName).Inc()
}

// RecordPrometheusSnapshotBatchCreated records when a batch is created
func RecordPrometheusSnapshotBatchCreated(tableNameTup sqlname.NameTuple, importerRole string) {
	schemaName, tableName := tableNameTup.ForKeyTableSchema()
	snapshotBatchCreated.WithLabelValues(promMigrationUUID.String(), promSessionID, importerRole, tableName, schemaName).Inc()
}

// RecordPrometheusSnapshotBatchSubmitted records when a batch is submitted to worker pool
func RecordPrometheusSnapshotBatchSubmitted(tableNameTup sqlname.NameTuple, importerRole string) {
	schemaName, tableName := tableNameTup.ForKeyTableSchema()
	snapshotBatchSubmitted.WithLabelValues(promMigrationUUID.String(), promSessionID, importerRole, tableName, schemaName).Inc()
}

// GetSessionID returns the current session ID for debugging purposes
func GetSessionID() string {
	return promSessionID
}
