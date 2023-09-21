package cmd

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/slices"
)

type MigrationStatusRecord struct {
	MigrationUUID               string
	ExportType                  string
	FallForwarDBExists          bool
	TargetDBConf                *tgtdb.TargetConf
	FallForwardDBConf           *tgtdb.TargetConf
	TableListExportedFromSource []string
	Triggers                    []string
	MigInfo                     *MigInfo
}

const MIGRATION_STATUS_KEY = "migration_status"

func UpdateMigrationStatusRecord(updateFn func(*MigrationStatusRecord)) error {
	return UpdateJsonObjectInMetaDB(metaDB, MIGRATION_STATUS_KEY, updateFn)
}

func GetMigrationStatusRecord() (*MigrationStatusRecord, error) {
	record := new(MigrationStatusRecord)
	found, err := metaDB.GetJsonObject(nil, MIGRATION_STATUS_KEY, record)
	if err != nil {
		return nil, fmt.Errorf("error while getting migration status record from meta db: %w", err)
	}
	if !found {
		return nil, nil
	}
	return record, nil
}

func InitMigrationStatusRecord() error {
	return UpdateMigrationStatusRecord(func(record *MigrationStatusRecord) {
		if record != nil && record.MigrationUUID != "" {
			return // already initialized
		}
		record.MigrationUUID = uuid.New().String()
		record.ExportType = SNAPSHOT_ONLY
		record.MigInfo = &MigInfo{}
	})
}

func (msr *MigrationStatusRecord) IsTriggerExists(triggerName string) bool {
	return slices.Contains(msr.Triggers, triggerName)
}

// sets the global variable migrationUUID after retrieving it from MigrationStatusRecord
func RetrieveMigrationUUID() error {
	msr, err := GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("retrieving migration status record: %w", err)
	}
	if msr == nil {
		return fmt.Errorf("migration status record not found")
	}

	migrationUUID = uuid.MustParse(msr.MigrationUUID)
	utils.PrintAndLog("migrationID: %s", migrationUUID)
	return nil
}
