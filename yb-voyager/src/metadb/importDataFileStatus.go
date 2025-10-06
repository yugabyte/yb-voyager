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

type ImportDataFileStatusRecord struct {
	ErrorPolicy         string `json:"errorPolicy"`
	ImportDataStarted   bool   `json:"importDataStarted"`
}

const IMPORT_DATA_FILE_STATUS_KEY = "import_data_file_status"

func (m *MetaDB) UpdateImportDataFileStatusRecord(updateFn func(*ImportDataFileStatusRecord)) error {
	return UpdateJsonObjectInMetaDB(m, IMPORT_DATA_FILE_STATUS_KEY, updateFn)
}

func (m *MetaDB) GetImportDataFileStatusRecord() (*ImportDataFileStatusRecord, error) {
	record := new(ImportDataFileStatusRecord)
	found, err := m.GetJsonObject(nil, IMPORT_DATA_FILE_STATUS_KEY, record)
	if err != nil {
		return nil, fmt.Errorf("error while getting import data file status record from meta db: %w", err)
	}
	if !found {
		return nil, nil
	}
	return record, nil
}

func (m *MetaDB) InitImportDataFileStatusRecord() error {
	return m.UpdateImportDataFileStatusRecord(func(record *ImportDataFileStatusRecord) {
		// nothing to initialize
	})
}

func (m *MetaDB) GetImportDataFileErrorPolicyUsed() (string, error) {
	idfsr, err := m.GetImportDataFileStatusRecord()
	if err != nil {
		return "", fmt.Errorf("error while getting import data file status record from meta db: %w", err)
	}
	if idfsr == nil {
		return "", fmt.Errorf("import data file status record not found")
	}
	return idfsr.ErrorPolicy, nil
}
