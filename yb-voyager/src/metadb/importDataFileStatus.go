package metadb

import (
	"fmt"
)

type ImportDataFileStatusRecord struct {
	ErrorPolicy string `json:"errorPolicy"`
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
