package metadb

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type MigrationStatusRecord struct {
	MigrationUUID string `json:"MigrationUUID"`
	SourceDBType  string `json:"SourceDBType"`
	ExportType    string `json:"ExportType"`
	// FallForwarDBExists                         bool              `json:"FallForwarDBExists"`
	FallForwardEnabled                         bool              `json:"FallForwardEnabled"`
	FallbackEnabled                            bool              `json:"FallbackEnabled"`
	TargetDBConf                               *tgtdb.TargetConf `json:"TargetDBConf"`
	FallForwardDBConf                          *tgtdb.TargetConf `json:"FallForwardDBConf"`
	TableListExportedFromSource                []string          `json:"TableListExportedFromSource"`
	SourceDBConf                               *srcdb.Source     `json:"SourceDBConf"`
	CutoverRequested                           bool              `json:"CutoverRequested"`
	CutoverProcessedBySourceExporter           bool              `json:"CutoverProcessedBySourceExporter"`
	CutoverProcessedByTargetImporter           bool              `json:"CutoverProcessedByTargetImporter"`
	FallForwardSyncStarted                     bool              `json:"FallForwardSyncStarted"`
	FallForwardSwitchRequested                 bool              `json:"FallForwardSwitchRequested"`
	FallForwardSwitchProcessedByTargetExporter bool              `json:"FallForwardSwitchProcessedByTargetExporter"`
	FallForwardSwitchProcessedByFFImporter     bool              `json:"FallForwardSwitchProcessedByFFImporter"`
	FallBackSyncStarted                        bool              `json:"FallBackSyncStarted"`
	FallBackSwitchRequested                    bool              `json:"FallBackSwitchRequested"`
	FallBackSwitchProcessedByTargetExporter    bool              `json:"FallBackSwitchProcessedByTargetExporter"`
	FallBackSwitchProcessedByFBImporter        bool              `json:"FallBackSwitchProcessedByFFImporter"`
	ExportSchemaDone                           bool              `json:"ExportSchemaDone"`
	ExportDataDone                             bool              `json:"ExportDataDone"`
}

const MIGRATION_STATUS_KEY = "migration_status"

func (m *MetaDB) UpdateMigrationStatusRecord(updateFn func(*MigrationStatusRecord)) error {
	return UpdateJsonObjectInMetaDB(m, MIGRATION_STATUS_KEY, updateFn)
}

func (m *MetaDB) GetMigrationStatusRecord() (*MigrationStatusRecord, error) {
	record := new(MigrationStatusRecord)
	found, err := m.GetJsonObject(nil, MIGRATION_STATUS_KEY, record)
	if err != nil {
		return nil, fmt.Errorf("error while getting migration status record from meta db: %w", err)
	}
	if !found {
		return nil, nil
	}
	return record, nil
}

func (m *MetaDB) InitMigrationStatusRecord() error {
	return m.UpdateMigrationStatusRecord(func(record *MigrationStatusRecord) {
		if record != nil && record.MigrationUUID != "" {
			return // already initialized
		}
		record.MigrationUUID = uuid.New().String()
		record.ExportType = utils.SNAPSHOT_ONLY
	})
}
