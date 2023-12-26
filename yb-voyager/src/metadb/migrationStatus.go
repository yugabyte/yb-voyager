package metadb

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type MigrationStatusRecord struct {
	MigrationUUID                                   string            `json:"MigrationUUID"`
	ExportType                                      string            `json:"ExportType"`
	ArchivingEnabled                                bool              `json:"ArchivingEnabled"`
	FallForwardEnabled                              bool              `json:"FallForwardEnabled"`
	FallbackEnabled                                 bool              `json:"FallbackEnabled"`
	TargetDBConf                                    *tgtdb.TargetConf `json:"TargetDBConf"`
	SourceReplicaDBConf                             *tgtdb.TargetConf `json:"SourceReplicaDBConf"`
	SourceDBAsTargetConf                            *tgtdb.TargetConf `json:"SourceDBAsTargetConf"`
	TableListExportedFromSource                     []string          `json:"TableListExportedFromSource"`
	SourceDBConf                                    *srcdb.Source     `json:"SourceDBConf"`
	CutoverToTargetRequested                        bool              `json:"CutoverToTargetRequested"`
	CutoverProcessedBySourceExporter                bool              `json:"CutoverProcessedBySourceExporter"`
	CutoverProcessedByTargetImporter                bool              `json:"CutoverProcessedByTargetImporter"`
	ExportFromTargetFallForwardStarted              bool              `json:"ExportFromTargetFallForwardStarted"`
	CutoverToSourceReplicaRequested                 bool              `json:"CutoverToSourceReplicaRequested"`
	CutoverToSourceReplicaProcessedByTargetExporter bool              `json:"CutoverToSourceReplicaProcessedByTargetExporter"`
	CutoverToSourceReplicaProcessedBySRImporter     bool              `json:"CutoverToSourceReplicaProcessedBySRImporter"`
	ExportFromTargetFallBackStarted                 bool              `json:"ExportFromTargetFallBackStarted"`
	CutoverToSourceRequested                        bool              `json:"CutoverToSourceRequested"`
	CutoverToSourceProcessedByTargetExporter        bool              `json:"CutoverToSourceProcessedByTargetExporter"`
	CutoverToSourceProcessedBySourceImporter        bool              `json:"CutoverToSourceProcessedBySourceImporter"`
	ExportSchemaDone                                bool              `json:"ExportSchemaDone"`
	ExportDataDone                                  bool              `json:"ExportDataDone"`
	YBCDCStreamID                                   string            `json:"YBCDCStreamID"`
	EndMigrationRequested                           bool              `json:"EndMigrationRequested"`
	RenameTablesMap 								string			  `json:"RenameTablesMap"`	
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
