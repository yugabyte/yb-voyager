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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var (
	// Total rows imported during snapshot
	importRowsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_rows_total",
			Help: "Total rows imported during snapshot",
		},
		[]string{"table_name", "schema_name", "importer_role"},
	)

	// Total bytes imported during snapshot
	importBytesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yb_voyager_import_snapshot_bytes_total",
			Help: "Total bytes imported during snapshot",
		},
		[]string{"table_name", "schema_name", "importer_role"},
	)
)

// RecordSnapshotBatchImport records metrics for a completed batch import
func RecordSnapshotBatchImport(tableNameTup sqlname.NameTuple, importerRole string, rows, bytes int64) {
	schemaName, tableName := tableNameTup.ForKeyTableSchema()
	importRowsTotal.WithLabelValues(tableName, schemaName, importerRole).Add(float64(rows))
	importBytesTotal.WithLabelValues(tableName, schemaName, importerRole).Add(float64(bytes))
}

// ResetSnapshotTableMetrics resets metrics for a specific table to zero
// func ResetSnapshotTableMetrics(tableNameTup sqlname.NameTuple, importerRole string) {
// 	schemaName, tableName := tableNameTup.ForKeyTableSchema()
// 	// Reset rows counter
// 	rowsLabels := importRowsTotal.WithLabelValues(tableName, schemaName, importerRole)
// 	currentRows := rowsLabels.Get()
// 	rowsLabels.Add(-currentRows)

// 	// Reset bytes counter
// 	bytesLabels := importBytesTotal.WithLabelValues(tableName, schemaName, importerRole)
// 	currentBytes := bytesLabels.Get()
// 	bytesLabels.Add(-currentBytes)
// }
