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

// ExportDataTargetDBExporterStatusRecord holds state written by the target DB exporter
// (`export data from target`, fall-back / fall-forward).
type ExportDataTargetDBExporterStatusRecord struct {
	// ExcludedColumns is the target exporter's effective excluded columns, recorded on its first
	// run: the source exporter's ExcludedColumns plus its own --exclude-column-list, as root table
	// (NameTuple.ForKey()) -> excluded column names. Nil means the target exporter has not
	// recorded one yet.
	ExcludedColumns map[string][]string `json:"excludedColumns"`
}

const EXPORT_DATA_TARGET_DB_EXPORTER_STATUS_KEY = "export_data_target_db_exporter_status"

func (m *MetaDB) UpdateExportDataTargetDBExporterStatusRecord(updateFn func(*ExportDataTargetDBExporterStatusRecord)) error {
	return UpdateJsonObjectInMetaDB(m, EXPORT_DATA_TARGET_DB_EXPORTER_STATUS_KEY, updateFn)
}

// GetExportDataTargetDBExporterStatusRecord returns the record, or nil if it was never written.
func (m *MetaDB) GetExportDataTargetDBExporterStatusRecord() (*ExportDataTargetDBExporterStatusRecord, error) {
	record := new(ExportDataTargetDBExporterStatusRecord)
	found, err := m.GetJsonObject(nil, EXPORT_DATA_TARGET_DB_EXPORTER_STATUS_KEY, record)
	if err != nil {
		return nil, fmt.Errorf("error while getting export data target db exporter status record from meta db: %w", err)
	}
	if !found {
		return nil, nil
	}
	return record, nil
}
