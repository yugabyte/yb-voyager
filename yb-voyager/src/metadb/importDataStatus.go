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

type ImportDataStatusRecord struct {
	ErrorPolicySnapshot string `json:"errorPolicySnapshot"`
	ImportDataStarted   bool   `json:"importDataStarted"`
}

const IMPORT_DATA_STATUS_KEY = "import_data_status"

func (m *MetaDB) UpdateImportDataStatusRecord(updateFn func(*ImportDataStatusRecord)) error {
	return UpdateJsonObjectInMetaDB(m, IMPORT_DATA_STATUS_KEY, updateFn)
}

func (m *MetaDB) GetImportDataStatusRecord() (*ImportDataStatusRecord, error) {
	record := new(ImportDataStatusRecord)
	found, err := m.GetJsonObject(nil, IMPORT_DATA_STATUS_KEY, record)
	if err != nil {
		return nil, fmt.Errorf("error while getting import data status record from meta db: %w", err)
	}
	if !found {
		return nil, nil
	}
	return record, nil
}

func (m *MetaDB) InitImportDataStatusRecord() error {
	return m.UpdateImportDataStatusRecord(func(record *ImportDataStatusRecord) {
		// nothing to initialize
	})
}

func (m *MetaDB) GetImportDataErrorPolicySnapshotUsed() (string, error) {
	idsr, err := m.GetImportDataStatusRecord()
	if err != nil {
		return "", fmt.Errorf("error while getting import data status record from meta db: %w", err)
	}
	if idsr == nil {
		return "", fmt.Errorf("import data status record not found")
	}
	return idsr.ErrorPolicySnapshot, nil
}
