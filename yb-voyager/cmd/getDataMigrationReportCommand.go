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
package cmd

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/gosuri/uitable"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var targetDbPassword string
var sourceReplicaDbPassword string
var sourceDbPassword string

var getDataMigrationReportCmd = &cobra.Command{
	Use:   "data-migration-report",
	Short: "Print the consolidated report of migration of data.",
	Long:  `Print the consolidated report of migration of data among different DBs (source / target / source-replica) when export-type 'snapshot-and-changes' is enabled.`,

	Run: func(cmd *cobra.Command, args []string) {
		migrationStatus, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("error while getting migration status: %w\n", err)
		}
		streamChanges, err := checkStreamingMode()
		if err != nil {
			utils.ErrExit("error while checking streaming mode: %w\n", err)
		}
		migrationUUID, err = uuid.Parse(migrationStatus.MigrationUUID)
		if err != nil {
			utils.ErrExit("error while parsing migration UUID: %w\n", err)
		}
		if streamChanges {
			if migrationStatus.TargetDBConf != nil {
				getTargetPassword(cmd)
				migrationStatus.TargetDBConf.Password = tconf.Password
			}
			if migrationStatus.FallForwardEnabled {
				getSourceReplicaDBPassword(cmd)
				migrationStatus.SourceReplicaDBConf.Password = tconf.Password
			}
			if migrationStatus.FallbackEnabled {
				getSourceDBPassword(cmd)
				migrationStatus.SourceDBAsTargetConf.Password = tconf.Password
			}
			err = InitNameRegistry(exportDir, SOURCE_DB_EXPORTER_ROLE, nil, nil, nil, nil)
			if err != nil {
				utils.ErrExit("initializing name registry: %v", err)
			}
			color.Yellow("Generating data migration report for migration UUID: %s...\n", migrationStatus.MigrationUUID)
			getDataMigrationReportCmdFn(migrationStatus)
		} else {
			utils.ErrExit("Error: Data migration report is only applicable when export-type is 'snapshot-and-changes'(live migration)\nPlease run export data status/import data status commands.")
		}
	},
}

type rowData struct {
	TableName            string
	DBType               string
	ExportedSnapshotRows int64
	ImportedSnapshotRows int64
	ImportedInserts      int64
	ImportedUpdates      int64
	ImportedDeletes      int64
	ExportedInserts      int64
	ExportedUpdates      int64
	ExportedDeletes      int64
}

var fBEnabled, fFEnabled bool
var firstHeader = []string{"TABLE", "DB_TYPE", "EXPORTED", "IMPORTED", "EXPORTED", "EXPORTED", "EXPORTED", "IMPORTED", "IMPORTED", "IMPORTED", "FINAL_ROW_COUNT"}
var secondHeader = []string{"", "", "SNAPSHOT_ROWS", "SNAPSHOT_ROWS", "INSERTS", "UPDATES", "DELETES", "INSERTS", "UPDATES", "DELETES", ""}

func getDataMigrationReportCmdFn(msr *metadb.MigrationStatusRecord) {
	fBEnabled = msr.FallbackEnabled
	fFEnabled = msr.FallForwardEnabled
	tableList := msr.TableListExportedFromSource
	tableNts, err := getImportTableList(tableList)
	if err != nil {
		utils.ErrExit("getting name tuples from table list: %v", err)
	}
	uitbl := uitable.New()
	uitbl.MaxColWidth = 50
	uitbl.Wrap = true
	uitbl.Separator = " | "

	maxTablesInOnePage := 10

	addHeader(uitbl, firstHeader...)
	addHeader(uitbl, secondHeader...)
	exportStatusFilePath := filepath.Join(exportDir, "data", "export_status.json")
	dbzmStatus, err := dbzm.ReadExportStatus(exportStatusFilePath)
	if err != nil {
		utils.ErrExit("Failed to read export status file %s: %v", exportStatusFilePath, err)
	}
	ntRowCountMap := utils.NewStructMap[sqlname.NameTuple, int64]()

	for _, tableExportStatus := range dbzmStatus.Tables {
		tableName := fmt.Sprintf("%s.%s", tableExportStatus.SchemaName, tableExportStatus.TableName)
		nt, err := namereg.NameReg.LookupTableName(tableName)
		if err != nil {
			utils.ErrExit("lookup %s in name registry: %v", tableName, err)
		}
		ntRowCountMap.Put(nt, tableExportStatus.ExportedRowCountSnapshot)
	}

	exportSnapshotStatusFilePath := filepath.Join(exportDir, "metainfo", "export_snapshot_status.json")
	exportSnapshotStatusFile = jsonfile.NewJsonFile[ExportSnapshotStatus](exportSnapshotStatusFilePath)
	var exportSnapshotStatus *ExportSnapshotStatus

	source = *msr.SourceDBConf
	if source.DBType == POSTGRESQL {
		exportSnapshotStatus, err = exportSnapshotStatusFile.Read()
		if err != nil {
			utils.ErrExit("Failed to read export status file %s: %v", exportSnapshotStatusFilePath, err)
		}
	}

	sqlname.SourceDBType = source.DBType
	// sourceSchemaCount := len(strings.Split(source.Schema, "|"))
	var exportedPGSnapshotRowsMap *utils.StructMap[sqlname.NameTuple, int64]
	if source.DBType == POSTGRESQL {
		exportedPGSnapshotRowsMap, _, err = getExportedSnapshotRowsMap(exportSnapshotStatus)
		if err != nil {
			utils.ErrExit("error while getting exported snapshot rows: %w\n", err)
		}
	}

	var targetImportedSnapshotRowsMap *utils.StructMap[sqlname.NameTuple, int64]
	if msr.TargetDBConf != nil {
		//TODO: FIX WITH STATS
		targetImportedSnapshotRowsMap, err = getImportedSnapshotRowsMap("target", tableNts)
		if err != nil {
			utils.ErrExit("error while getting imported snapshot rows for target DB: %w\n", err)
		}
	}

	var replicaImportedSnapshotRowsMap *utils.StructMap[sqlname.NameTuple, int64]
	if fFEnabled {
		//TODO: FIX WITH STATS
		replicaImportedSnapshotRowsMap, err = getImportedSnapshotRowsMap("source-replica", tableNts)
		if err != nil {
			utils.ErrExit("error while getting imported snapshot rows for source-replica DB: %w\n", err)
		}
	}

	for i, nt := range tableNts {
		uitbl.AddRow() // blank row

		row := rowData{}
		// tableName := strings.Split(table, ".")[1]
		// schemaName := strings.Split(table, ".")[0]
		updateExportedSnapshotRowsInTheRow(msr, &row, nt, ntRowCountMap, exportedPGSnapshotRowsMap)
		row.ImportedSnapshotRows = 0
		row.TableName = nt.ForKey()
		// if sourceSchemaCount <= 1 && source.DBType != POSTGRESQL { //this check is for Oracle case
		// 	schemaName = ""
		// 	row.TableName = tableName
		// }
		row.DBType = "source"
		err := updateExportedEventsCountsInTheRow(&row, nt.SourceName.Unqualified.Unquoted, nt.SourceName.SchemaName) //source OUT counts
		if err != nil {
			utils.ErrExit("error while getting exported events counts for source DB: %w\n", err)
		}
		if fBEnabled {
			err = updateImportedEventsCountsInTheRow(source.DBType, &row, nt, msr.SourceDBAsTargetConf, nil) //fall back IN counts
			if err != nil {
				utils.ErrExit("error while getting imported events for source DB in case of fall-back: %w\n", err)
			}
		}
		addRowInTheTable(uitbl, row)
		row = rowData{}
		row.TableName = ""
		row.DBType = "target"
		row.ExportedSnapshotRows = 0
		if msr.TargetDBConf != nil { // In case import is not started yet, target DB conf will be nil
			err = updateImportedEventsCountsInTheRow(source.DBType, &row, nt, msr.TargetDBConf, targetImportedSnapshotRowsMap) //target IN counts
			if err != nil {
				utils.ErrExit("error while getting imported events for target DB: %w\n", err)
			}
		}
		if fFEnabled || fBEnabled {
			err = updateExportedEventsCountsInTheRow(&row, nt.SourceName.Unqualified.Unquoted, nt.SourceName.SchemaName) // target OUT counts
			if err != nil {
				utils.ErrExit("error while getting exported events for target DB: %w\n", err)
			}
		}
		addRowInTheTable(uitbl, row)
		if fFEnabled {
			row = rowData{}
			row.TableName = ""
			row.DBType = "source-replica"
			row.ExportedSnapshotRows = 0
			err = updateImportedEventsCountsInTheRow(source.DBType, &row, nt, msr.SourceReplicaDBConf, replicaImportedSnapshotRowsMap) //fall forward IN counts
			if err != nil {
				utils.ErrExit("error while getting imported events for DB %s: %w\n", row.DBType, err)
			}
			addRowInTheTable(uitbl, row)
		}
		if i%maxTablesInOnePage == 0 && i != 0 {
			//multiple table in case of large set of tables
			fmt.Print("\n")
			fmt.Println(uitbl)
			fmt.Print("\n")
			uitbl = uitable.New()
			uitbl.MaxColWidth = 50
			uitbl.Separator = " | "
			addHeader(uitbl, firstHeader...)
			addHeader(uitbl, secondHeader...)
		}
	}
	if uitbl.Rows != nil {
		fmt.Print("\n")
		fmt.Println(uitbl)
		fmt.Print("\n")
	}

}

func addRowInTheTable(uitbl *uitable.Table, row rowData) {
	uitbl.AddRow(row.TableName, row.DBType, row.ExportedSnapshotRows, row.ImportedSnapshotRows, row.ExportedInserts, row.ExportedUpdates, row.ExportedDeletes, row.ImportedInserts, row.ImportedUpdates, row.ImportedDeletes, getFinalRowCount(row))
}

func updateExportedSnapshotRowsInTheRow(msr *metadb.MigrationStatusRecord, row *rowData, nt sqlname.NameTuple, dbzmSnapshotRowCount *utils.StructMap[sqlname.NameTuple, int64], exportedSnapshotPGRowsMap *utils.StructMap[sqlname.NameTuple, int64]) error {
	// TODO: read only from one place(data file descriptor). Right now, data file descriptor does not store schema names.
	if msr.SnapshotMechanism == "debezium" {
		row.ExportedSnapshotRows, _ = dbzmSnapshotRowCount.Get(nt)
	} else {
		row.ExportedSnapshotRows, _ = exportedSnapshotPGRowsMap.Get(nt)
	}
	return nil
}

func updateImportedEventsCountsInTheRow(sourceDBType string, row *rowData, nt sqlname.NameTuple, targetConf *tgtdb.TargetConf, snapshotImportedRowsMap *utils.StructMap[sqlname.NameTuple, int64]) error {
	switch row.DBType {
	case "target":
		importerRole = TARGET_DB_IMPORTER_ROLE
	case "source-replica":
		importerRole = SOURCE_REPLICA_DB_IMPORTER_ROLE
	case "source":
		importerRole = SOURCE_DB_IMPORTER_ROLE
	}
	//reinitialise targetDB
	tconf = *targetConf
	tdb = tgtdb.NewTargetDB(&tconf)
	err := tdb.Init()
	if err != nil {
		return fmt.Errorf("failed to initialize the target DB: %w", err)
	}
	defer tdb.Finalize()
	err = tdb.InitConnPool()
	if err != nil {
		return fmt.Errorf("failed to initialize the target DB connection pool: %w", err)
	}
	state := NewImportDataState(exportDir)

	// if sourceDBType == POSTGRESQL && schemaName != "public" && schemaName != "" { //multiple schema specific
	// 	tableName = schemaName + "." + tableName
	// }

	if importerRole != SOURCE_DB_IMPORTER_ROLE {
		row.ImportedSnapshotRows, _ = snapshotImportedRowsMap.Get(nt) // TODO: FIX table.ForKey()
	}

	// TODO:TABLENAME fix!
	eventCounter, err := state.GetImportedEventsStatsForTable(nt, migrationUUID)
	if err != nil {
		if !strings.Contains(err.Error(), "cannot assign NULL to *int64") &&
			!strings.Contains(err.Error(), "converting NULL to int64") { //TODO: handle better in GetImportedEventsStatsForTable() itself later
			return fmt.Errorf("get imported events stats for table %q for DB type %s: %w", nt, row.DBType, err)
		} else {
			//in case import streaming is not started yet, metadata will not be initialized
			log.Warnf("stream ingestion is not started yet for table %q for DB type %s", nt, row.DBType)
			eventCounter = &tgtdb.EventCounter{
				NumInserts: 0,
				NumUpdates: 0,
				NumDeletes: 0,
			}
		}
	}
	row.ImportedInserts = eventCounter.NumInserts
	row.ImportedUpdates = eventCounter.NumUpdates
	row.ImportedDeletes = eventCounter.NumDeletes
	return nil
}

func updateExportedEventsCountsInTheRow(row *rowData, tableName string, schemaName string) error {
	switch row.DBType {
	case "source":
		exporterRole = SOURCE_DB_EXPORTER_ROLE
	case "target":
		if fFEnabled {
			exporterRole = TARGET_DB_EXPORTER_FF_ROLE
		} else if fBEnabled {
			exporterRole = TARGET_DB_EXPORTER_FB_ROLE
		}
	}
	if len(strings.Split(source.Schema, "|")) <= 1 {
		schemaName = ""
	}
	eventCounter, err := metaDB.GetExportedEventsStatsForTableAndExporterRole(exporterRole, schemaName, tableName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("could not fetch table stats from meta DB: %w", err)
	}
	row.ExportedInserts = eventCounter.NumInserts
	row.ExportedUpdates = eventCounter.NumUpdates
	row.ExportedDeletes = eventCounter.NumDeletes
	return nil
}

func getFinalRowCount(row rowData) int64 {
	if row.DBType == "source" {
		return row.ExportedSnapshotRows + row.ExportedInserts + row.ImportedInserts - row.ExportedDeletes - row.ImportedDeletes
	}
	return row.ImportedSnapshotRows + row.ImportedInserts + row.ExportedInserts - row.ImportedDeletes - row.ExportedDeletes
}

func init() {
	getCommand.AddCommand(getDataMigrationReportCmd)
	registerExportDirFlag(getDataMigrationReportCmd)
	getDataMigrationReportCmd.Flags().StringVar(&sourceReplicaDbPassword, "source-replica-db-password", "",
		"password with which to connect to the target Source-Replica DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_REPLICA_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")

	getDataMigrationReportCmd.Flags().StringVar(&sourceDbPassword, "source-db-password", "",
		"password with which to connect to the target source DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes")

	getDataMigrationReportCmd.Flags().StringVar(&targetDbPassword, "target-db-password", "",
		"password with which to connect to the target YugabyteDB server. Alternatively, you can also specify the password by setting the environment variable TARGET_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")
}
