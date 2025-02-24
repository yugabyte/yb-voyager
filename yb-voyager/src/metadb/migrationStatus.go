package metadb

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type MigrationStatusRecord struct {
	MigrationUUID                             string            `json:"MigrationUUID"`
	VoyagerVersion                            string            `json:"VoyagerVersion"`
	ExportType                                string            `json:"ExportType"`
	ArchivingEnabled                          bool              `json:"ArchivingEnabled"`
	FallForwardEnabled                        bool              `json:"FallForwardEnabled"`
	FallbackEnabled                           bool              `json:"FallbackEnabled"`
	UseYBgRPCConnector                        bool              `json:"UseYBgRPCConnector"`
	TargetDBConf                              *tgtdb.TargetConf `json:"TargetDBConf"`
	SourceReplicaDBConf                       *tgtdb.TargetConf `json:"SourceReplicaDBConf"`
	SourceDBAsTargetConf                      *tgtdb.TargetConf `json:"SourceDBAsTargetConf"`
	TableListExportedFromSource               []string          `json:"TableListExportedFromSource"`
	SourceExportedTableListWithLeafPartitions []string          `json:"SourceExportedTableListWithLeafPartitions"` // will be same as `TableListExportedFromSource` for Oracle and MySQL but will have leaf partitions in case of PG
	TargetExportedTableListWithLeafPartitions []string          `json:"TargetExportedTableListWithLeafPartitions"` // will be the table list for export data from target with leaf partitions

	SourceDBConf *srcdb.Source `json:"SourceDBConf"`

	CutoverToTargetRequested                        bool `json:"CutoverToTargetRequested"`
	CutoverProcessedBySourceExporter                bool `json:"CutoverProcessedBySourceExporter"`
	CutoverProcessedByTargetImporter                bool `json:"CutoverProcessedByTargetImporter"`
	ExportFromTargetFallForwardStarted              bool `json:"ExportFromTargetFallForwardStarted"`
	CutoverToSourceReplicaRequested                 bool `json:"CutoverToSourceReplicaRequested"`
	CutoverToSourceReplicaProcessedByTargetExporter bool `json:"CutoverToSourceReplicaProcessedByTargetExporter"`
	CutoverToSourceReplicaProcessedBySRImporter     bool `json:"CutoverToSourceReplicaProcessedBySRImporter"`
	ExportFromTargetFallBackStarted                 bool `json:"ExportFromTargetFallBackStarted"`
	CutoverToSourceRequested                        bool `json:"CutoverToSourceRequested"`
	CutoverToSourceProcessedByTargetExporter        bool `json:"CutoverToSourceProcessedByTargetExporter"`
	CutoverToSourceProcessedBySourceImporter        bool `json:"CutoverToSourceProcessedBySourceImporter"`

	ExportSchemaDone                bool `json:"ExportSchemaDone"`
	ExportDataDone                  bool `json:"ExportDataDone"` // to be interpreted as export of snapshot data from source is complete
	ExportDataSourceDebeziumStarted bool `json:"ExportDataSourceDebeziumStarted"`
	ExportDataTargetDebeziumStarted bool `json:"ExportDataTargetDebeziumStarted"`

	YBCDCStreamID                    string            `json:"YBCDCStreamID"`
	EndMigrationRequested            bool              `json:"EndMigrationRequested"`
	PGReplicationSlotName            string            `json:"PGReplicationSlotName"` // of the format voyager_<migrationUUID> (with replace "-" -> "_")
	PGPublicationName                string            `json:"PGPublicationName"`     // of the format voyager_<migrationUUID> (with replace "-" -> "_")
	YBReplicationSlotName            string            `json:"YBReplicationSlotName"` // of the format voyager_<migrationUUID> (with replace "-" -> "_")
	YBPublicationName                string            `json:"YBPublicationName"`     // of the format voyager_<migrationUUID> (with replace "-" -> "_")
	SnapshotMechanism                string            `json:"SnapshotMechanism"`     // one of (debezium, pg_dump, ora2pg)
	SourceRenameTablesMap            map[string]string `json:"SourceRenameTablesMap"` // map of source table.Qualified.Unquoted -> table.Qualified.Unquoted for renaming the leaf partitions to root table in case of PG migration
	TargetRenameTablesMap            map[string]string `json:"TargetRenameTablesMap"` // map of target table.Qualified.Unquoted -> table.Qualified.Unquoted for renaming the leaf partitions to root table in case of PG migration
	IsExportTableListSet             bool              `json:"IsExportTableListSet"`
	MigrationAssessmentDone          bool              `json:"MigrationAssessmentDone"`
	AssessmentRecommendationsApplied bool              `json:"AssessmentRecommendationsApplied"`

	ImportDataFileFlagFileTableMapping string `json:"ImportDataFileFlagFileTableMapping"` // Import data file command's file_table_mapping flag
	ImportDataFileFlagDataDir          string `json:"ImportDataFileFlagDataDir"`          // Import data file command's data-dir flag
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
		if record.VoyagerVersion == "" {
			record.VoyagerVersion = utils.YB_VOYAGER_VERSION
		}

		record.MigrationUUID = uuid.New().String()
		record.ExportType = utils.SNAPSHOT_ONLY
	})
}

func (msr *MigrationStatusRecord) IsSnapshotExportedViaDebezium() bool {
	return msr.SnapshotMechanism == "debezium"
}
