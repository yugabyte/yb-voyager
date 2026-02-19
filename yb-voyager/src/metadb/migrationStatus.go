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
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type MigrationStatusRecord struct {
	MigrationUUID                             string            `json:"MigrationUUID"`
	AnonymizerSalt                            string            `json:"AnonymizerSalt"` // salt for anonymization, used to ensure consistent anonymization across runs
	VoyagerVersion                            string            `json:"VoyagerVersion"`
	ExportTypeFromSource                      string            `json:"ExportType"`
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

	//All the cutover requested flags by initiate cutover command
	CutoverToTargetRequested        bool `json:"CutoverToTargetRequested"`
	CutoverToSourceRequested        bool `json:"CutoverToSourceRequested"`
	CutoverToSourceReplicaRequested bool `json:"CutoverToSourceReplicaRequested"`

	//All the cutover detected by importer flags (marked when the cutover event is recieved by the importer)
	CutoverDetectedByTargetImporter        bool `json:"CutoverDetectedByTargetImporter"`
	CutoverDetectedBySourceImporter        bool `json:"CutoverDetectedBySourceImporter"`
	CutoverDetectedBySourceReplicaImporter bool `json:"CutoverDetectedBySourceReplicaImporter"`

	//All the cutover processed by importer/exporter flags - indicating that the cutover is completed by that command.
	CutoverProcessedBySourceExporter                bool `json:"CutoverProcessedBySourceExporter"`
	CutoverToSourceProcessedByTargetExporter        bool `json:"CutoverToSourceProcessedByTargetExporter"`
	CutoverToSourceReplicaProcessedByTargetExporter bool `json:"CutoverToSourceReplicaProcessedByTargetExporter"`
	CutoverProcessedByTargetImporter                bool `json:"CutoverProcessedByTargetImporter"`
	CutoverToSourceReplicaProcessedBySRImporter     bool `json:"CutoverToSourceReplicaProcessedBySRImporter"`
	CutoverToSourceProcessedBySourceImporter        bool `json:"CutoverToSourceProcessedBySourceImporter"`

	ExportFromTargetFallForwardStarted bool `json:"ExportFromTargetFallForwardStarted"`
	ExportFromTargetFallBackStarted    bool `json:"ExportFromTargetFallBackStarted"`

	// Cutover timing data
	CutoverTimings CutoverTimingRecord `json:"CutoverTimings,omitempty"`

	ExportSchemaDone                bool `json:"ExportSchemaDone"`
	ExportDataDone                  bool `json:"ExportDataDone"` // to be interpreted as export of snapshot data from source is complete
	ExportDataSourceDebeziumStarted bool `json:"ExportDataSourceDebeziumStarted"`
	ExportDataTargetDebeziumStarted bool `json:"ExportDataTargetDebeziumStarted"`

	OnPrimaryKeyConflictAction string `json:"OnPrimaryKeyConflictAction"` // only used in import data or import data file commands

	YBCDCStreamID                          string            `json:"YBCDCStreamID"`
	EndMigrationRequested                  bool              `json:"EndMigrationRequested"`
	PGReplicationSlotName                  string            `json:"PGReplicationSlotName"` // of the format voyager_<migrationUUID> (with replace "-" -> "_")
	PGPublicationName                      string            `json:"PGPublicationName"`     // of the format voyager_<migrationUUID> (with replace "-" -> "_")
	YBReplicationSlotName                  string            `json:"YBReplicationSlotName"` // of the format voyager_<migrationUUID> (with replace "-" -> "_")
	YBPublicationName                      string            `json:"YBPublicationName"`     // of the format voyager_<migrationUUID> (with replace "-" -> "_")
	SnapshotMechanism                      string            `json:"SnapshotMechanism"`     // one of (debezium, pg_dump, ora2pg)
	SourceRenameTablesMap                  map[string]string `json:"SourceRenameTablesMap"` // map of source table.Qualified.Unquoted -> table.Qualified.Unquoted for renaming the leaf partitions to root table in case of PG migration
	TargetRenameTablesMap                  map[string]string `json:"TargetRenameTablesMap"` // map of target table.Qualified.Unquoted -> table.Qualified.Unquoted for renaming the leaf partitions to root table in case of PG migration
	IsExportTableListSet                   bool              `json:"IsExportTableListSet"`
	MigrationAssessmentDone                bool              `json:"MigrationAssessmentDone"`
	AssessmentRecommendationsApplied       bool              `json:"AssessmentRecommendationsApplied"`
	MigrationAssessmentDoneViaExportSchema bool              `json:"MigrationAssessmentDoneViaExportSchema"`

	PerformanceComparisonDone bool `json:"PerformanceComparisonDone"`

	ImportDataFileFlagFileTableMapping string `json:"ImportDataFileFlagFileTableMapping"` // Import data file command's file_table_mapping flag
	ImportDataFileFlagDataDir          string `json:"ImportDataFileFlagDataDir"`          // Import data file command's data-dir flag

	SourceColumnToSequenceMapping map[string]string `json:"SourceColumnToSequenceMapping"`
	TargetColumnToSequenceMapping map[string]string `json:"TargetColumnToSequenceMapping"`

	//Common details for per migration
	RestartDataMigrationSourceTargetNextIteration bool   `json:"RestartDataMigrationSourceTargetNextIteration"`
	ParentExportDir                               string `json:"ParentExportDir"`
	IterationNo                                   int    `json:"Iteration"`
	ConfigFile                                    string `json:"ConfigFile"`

	//Parent specific details
	TotalIterations       int `json:"TotalIterations"`
	LatestIterationNumber int `json:"LatestIterationNumber"`
}

type CutoverTimingRecord struct {
	// Request timestamps
	ToTargetRequestedAt        time.Time `json:"ToTargetRequestedAt,omitempty"`
	ToSourceRequestedAt        time.Time `json:"ToSourceRequestedAt,omitempty"`
	ToSourceReplicaRequestedAt time.Time `json:"ToSourceReplicaRequestedAt,omitempty"`

	// Detection timestamps
	DetectedByTargetImporterAt        time.Time `json:"DetectedByTargetImporterAt,omitempty"`
	DetectedBySourceImporterAt        time.Time `json:"DetectedBySourceImporterAt,omitempty"`
	DetectedBySourceReplicaImporterAt time.Time `json:"DetectedBySourceReplicaImporterAt,omitempty"`

	// Processing timestamps
	ProcessedBySourceExporterAt                time.Time `json:"ProcessedBySourceExporterAt,omitempty"`
	ProcessedByTargetImporterAt                time.Time `json:"ProcessedByTargetImporterAt,omitempty"`
	ToSourceReplicaProcessedByTargetExporterAt time.Time `json:"ToSourceReplicaProcessedByTargetExporterAt,omitempty"`
	ToSourceProcessedByTargetExporterAt        time.Time `json:"ToSourceProcessedByTargetExporterAt,omitempty"`
	ToSourceReplicaProcessedBySRImporterAt     time.Time `json:"ToSourceReplicaProcessedBySRImporterAt,omitempty"`
	ToSourceProcessedBySourceImporterAt        time.Time `json:"ToSourceProcessedBySourceImporterAt,omitempty"`

	// Export from target timestamps
	ExportFromTargetFallForwardStartedAt time.Time `json:"ExportFromTargetFallForwardStartedAt,omitempty"`
	ExportFromTargetFallBackStartedAt    time.Time `json:"ExportFromTargetFallBackStartedAt,omitempty"`
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

func (m *MetaDB) InitMigrationStatusRecord(cfgFile string) error {
	return m.UpdateMigrationStatusRecord(func(record *MigrationStatusRecord) {
		if record != nil && record.MigrationUUID != "" {
			return // already initialized
		}
		if record.VoyagerVersion == "" {
			record.VoyagerVersion = utils.YB_VOYAGER_VERSION
		}

		if record.ConfigFile == "" {
			record.ConfigFile = cfgFile
		}

		record.MigrationUUID = uuid.New().String()
		record.ExportTypeFromSource = utils.SNAPSHOT_ONLY
	})
}

func (msr *MigrationStatusRecord) IsSnapshotExportedViaDebezium() bool {
	return msr.SnapshotMechanism == "debezium"
}

func (msr *MigrationStatusRecord) IsParentMigration() bool {
	return msr.ParentExportDir == ""
}

func (msr *MigrationStatusRecord) IsIteration() bool {
	return msr.ParentExportDir != ""
}

func (msr *MigrationStatusRecord) GetParentMetaDB() (*MetaDB, error) {
	return NewMetaDB(msr.ParentExportDir)
}

func (msr *MigrationStatusRecord) GetIterationsDir(exportDir string) string {
	if msr.IsParentMigration() {
		return filepath.Join(exportDir, "live-data-migration-iterations")
	}
	return filepath.Join(msr.ParentExportDir, "live-data-migration-iterations")
}
