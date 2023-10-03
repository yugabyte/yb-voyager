package metadb

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type MigrationStatusRecord struct {
	MigrationUUID                              string
	SourceDBType                               string
	ExportType                                 string
	FallForwarDBExists                         bool
	TargetDBConf                               *tgtdb.TargetConf
	FallForwardDBConf                          *tgtdb.TargetConf
	TableListExportedFromSource                []string
	SourceDBConf                               *srcdb.Source
	CutoverRequested                           bool
	CutoverProcessedBySourceExporter           bool
	CutoverProcessedByTargetImporter           bool
	FallForwardSyncStarted                     bool
	FallForwardSwitchRequested                 bool
	FallForwardSwitchProcessedByTargetExporter bool
	FallForwardSwitchProcessedByFFImporter     bool
	ExportSchemaDone                           bool
	ExportDataDone                             bool
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
