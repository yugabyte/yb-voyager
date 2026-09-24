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
package metadb

import (
	"fmt"
)

// ExportDataSourceDBExporterStatusRecord holds source-side metadata captured by the
// source DB exporter during `export data` and consumed later by `import data to target`.
//
// It is written by the export process and read by the import process (both share the
// metaDB in the export dir), so it lets import use authoritative *source* facts without
// opening a source connection of its own.
//
// Today it carries only the per-table STORED generated columns; the record is intentionally
// a small extensible bag for other source-exporter-derived state in the future.
type ExportDataSourceDBExporterStatusRecord struct {
	// TableToGeneratedStoredColumns maps a table (by NameTuple.ForKey()) to the names of its
	// STORED generated columns on the source. Names only — whether a column participates in
	// a unique index / primary key is decided at import time from the target catalog (see the
	// hybrid resolution in cmd/live_migration_cdc_partition_strategy.go). A table with no
	// generated columns is simply absent from the map.
	TableToGeneratedStoredColumns map[string][]string `json:"tableToGeneratedStoredColumns"`
}

const EXPORT_DATA_SOURCE_DB_EXPORTER_STATUS_KEY = "export_data_source_db_exporter_status"

func (m *MetaDB) UpdateExportDataSourceDBExporterStatusRecord(updateFn func(*ExportDataSourceDBExporterStatusRecord)) error {
	return UpdateJsonObjectInMetaDB(m, EXPORT_DATA_SOURCE_DB_EXPORTER_STATUS_KEY, updateFn)
}

// GetExportDataSourceDBExporterStatusRecord returns the record, or nil if it was never
// written (older voyager export, or an export that did not run the capture).
func (m *MetaDB) GetExportDataSourceDBExporterStatusRecord() (*ExportDataSourceDBExporterStatusRecord, error) {
	record := new(ExportDataSourceDBExporterStatusRecord)
	found, err := m.GetJsonObject(nil, EXPORT_DATA_SOURCE_DB_EXPORTER_STATUS_KEY, record)
	if err != nil {
		return nil, fmt.Errorf("error while getting export data source db exporter status record from meta db: %w", err)
	}
	if !found {
		return nil, nil
	}
	return record, nil
}
