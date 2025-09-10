package metadb

import (
	"fmt"
)

type ImportDataStatusRecord struct {
	ErrorPolicySnapshot string `json:"errorPolicySnapshot"`
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
