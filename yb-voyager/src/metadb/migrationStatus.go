package metadb

import (
	"fmt"

	"github.com/google/uuid"
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
	MigInfo                                    *MigInfo
	CutoverRequested                           bool
	CutoverProcessedBySourceExporter           bool
	CutoverProcessedByTargetImporter           bool
	FallForwardSyncStarted                     bool
	FallForwardSwitchRequested                 bool
	FallForwardSwitchProcessedByTargetExporter bool
	FallForwardSwitchProcessedByFFImporter     bool
}

type MigInfo struct {
	SourceDBType    string
	SourceDBName    string
	SourceDBSchema  string
	SourceDBVersion string
	SourceDBSid     string
	SourceTNSAlias  string
	ExportDir       string
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
		record.MigInfo = &MigInfo{}
	})
}

func (msr *MigrationStatusRecord) CheckIfTriggerExists(triggerName string) bool {
	switch triggerName {
	case "cutover":
		return msr.CutoverRequested
	case "cutover.source":
		return msr.CutoverProcessedBySourceExporter
	case "cutover.target":
		return msr.CutoverProcessedByTargetImporter
	case "fallforward":
		return msr.FallForwardSwitchRequested
	case "fallforward.target":
		return msr.FallForwardSwitchProcessedByTargetExporter
	case "fallforward.ff":
		return msr.FallForwardSwitchProcessedByFFImporter
	default:
		panic("invalid trigger name")
	}
}

func (msr *MigrationStatusRecord) SetTrigger(triggerName string) {
	switch triggerName {
	case "cutover":
		msr.CutoverRequested = true
	case "cutover.source":
		msr.CutoverProcessedBySourceExporter = true
	case "cutover.target":
		msr.CutoverProcessedByTargetImporter = true
	case "fallforward":
		msr.FallForwardSwitchRequested = true
	case "fallforward.target":
		msr.FallForwardSwitchProcessedByTargetExporter = true
	case "fallforward.ff":
		msr.FallForwardSwitchProcessedByFFImporter = true
	default:
		panic("invalid trigger name")
	}
}
