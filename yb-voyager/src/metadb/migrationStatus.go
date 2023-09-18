package metadb

import (
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type MigrationStatusRecord struct {
	MigrationUUID               string
	SourceDBType                string
	ExportType                  string
	FallForwarDBExists          bool
	TargetDBConf                *tgtdb.TargetConf
	FallForwardDBConf           *tgtdb.TargetConf
	TableListExportedFromSource []string
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

func InitMigrationStatusRecord(migUUID string) error {
	return UpdateMigrationStatusRecord(func(record *MigrationStatusRecord) {
		if record != nil && record.MigrationUUID != "" {
			return // already initialized
		}
		record.MigrationUUID = migUUID
		record.ExportType = utils.SNAPSHOT_ONLY
	})
}
