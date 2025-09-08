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
package prometheus

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// sessionID is created on package init and used for all metrics
var sessionID string

func init() {
	// Create a unique session ID based on formatted timestamp
	sessionID = time.Now().Format("20060102-150405")
}

var (
	// Total rows imported during snapshot
	importRowsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_rows_total",
			Help: "Total rows imported during snapshot",
		},
		[]string{"session_id", "importer_role", "table_name", "schema_name"},
	)

	// Total bytes imported during snapshot
	importBytesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_bytes_total",
			Help: "Total bytes imported during snapshot",
		},
		[]string{"session_id", "importer_role", "table_name", "schema_name"},
	)

	// Total number of batches created for import
	snapshotBatchCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_batch_created_total",
			Help: "Total number of batches created for import",
		},
		[]string{"session_id", "importer_role", "table_name", "schema_name"},
	)

	// Total number of batches submitted to worker pool
	snapshotBatchSubmitted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_batch_submitted_total",
			Help: "Total number of batches submitted to worker pool",
		},
		[]string{"session_id", "importer_role", "table_name", "schema_name"},
	)

	// Total number of batches successfully ingested
	snapshotBatchIngested = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_batch_ingested_total",
			Help: "Total number of batches successfully ingested",
		},
		[]string{"session_id", "importer_role", "table_name", "schema_name"},
	)
)

// RecordSnapshotBatchIngested records metrics for a completed batch import
func RecordSnapshotBatchIngested(tableNameTup sqlname.NameTuple, importerRole string, rows, bytes int64) {
	schemaName, tableName := tableNameTup.ForKeyTableSchema()
	importRowsTotal.WithLabelValues(sessionID, importerRole, tableName, schemaName).Add(float64(rows))
	importBytesTotal.WithLabelValues(sessionID, importerRole, tableName, schemaName).Add(float64(bytes))
	snapshotBatchIngested.WithLabelValues(sessionID, importerRole, tableName, schemaName).Inc()
}

// RecordSnapshotBatchCreated records when a batch is created
func RecordSnapshotBatchCreated(tableNameTup sqlname.NameTuple, importerRole string) {
	schemaName, tableName := tableNameTup.ForKeyTableSchema()
	snapshotBatchCreated.WithLabelValues(sessionID, importerRole, tableName, schemaName).Inc()
}

// RecordSnapshotBatchSubmitted records when a batch is submitted to worker pool
func RecordSnapshotBatchSubmitted(tableNameTup sqlname.NameTuple, importerRole string) {
	schemaName, tableName := tableNameTup.ForKeyTableSchema()
	snapshotBatchSubmitted.WithLabelValues(sessionID, importerRole, tableName, schemaName).Inc()
}

// GetSessionID returns the current session ID for debugging purposes
func GetSessionID() string {
	return sessionID
}
