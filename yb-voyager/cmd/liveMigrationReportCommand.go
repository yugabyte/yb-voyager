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

	"github.com/google/uuid"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var liveMigrationReportCmd = &cobra.Command{
	Use: "report",
	Short: "This command will print the report of any live migration workflow.",
	Long: `This command will print the report of the live migration or live migration with fall-forward or live migration with fall-back.`,

	Run: func(cmd *cobra.Command, args []string) {
		validateExportDirFlag()
		migrationStatus, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("error while getting migration status: %w\n", err)
		}
		streamChanges, err := checkWithStreamingMode()
		if err != nil {
			utils.ErrExit("error while checking streaming mode: %w\n", err)
		}
		migrationUUID, err = uuid.Parse(migrationStatus.MigrationUUID)
		if err != nil {
			utils.ErrExit("error while parsing migration UUID: %w\n", err)
		}
		if streamChanges {
			getTargetPassword(cmd)
			migrationStatus.TargetDBConf.Password = tconf.Password
			if migrationStatus.FallForwardEnabled {
				getFallForwardDBPassword(cmd)
				migrationStatus.FallForwardDBConf.Password = tconf.Password
			}
			if migrationStatus.FallbackEnabled {
				getSourceDBPassword(cmd)
				migrationStatus.SourceDBAsTargetConf.Password = source.Password
			}
			liveMigrationStatusCmdFn(migrationStatus)
		} else {
			utils.ErrExit("export-type 'snapshot-and-changes' is not enabled for this migration")
		}
	},
}

type rowData struct {
	TableName        string
	DBType           string
	SnapshotRowCount int64
	InsertsIn        int64
	UpdatesIn        int64
	DeletesIn        int64
	InsertsOut       int64
	UpdatesOut       int64
	DeletesOut       int64
	FinalRowCount    int64
}

var fBEnabled, fFEnabled bool

func liveMigrationStatusCmdFn(msr *metadb.MigrationStatusRecord) {
	fBEnabled = msr.FallbackEnabled
	fFEnabled = msr.FallForwardEnabled
	tableList := msr.TableListExportedFromSource
	reportTable := uitable.New()
	reportTable.MaxColWidth = 50
	addHeader(reportTable, "TABLE", "DB TYPE", "SNAPSHOT ROW COUNT", "INSERTS IN", "UPDATES IN", "DELETES IN", "INSERTS OUT", "UPDATES OUT", "DELETES OUT", "FINAL ROW COUNT")

	exportStatusFilePath := filepath.Join(exportDir, "data", "export_status.json")
	status, err := dbzm.ReadExportStatus(exportStatusFilePath)
	if err != nil {
		utils.ErrExit("Failed to read export status file %s: %v", exportStatusFilePath, err)
	}
	for _, table := range tableList {
		reportTable.AddRow() // blank row

		row := rowData{}
		tableName := strings.Split(table, ".")[1]
		schemaName := strings.Split(table, ".")[0]
		tableExportStatus := status.GetTableStatusByTableName(tableName, schemaName)
		row.SnapshotRowCount = tableExportStatus.ExportedRowCountSnapshot
		source = *msr.SourceDBConf
		sourceSchemaCount := len(strings.Split(source.Schema, "|"))
		row.TableName = table
		if sourceSchemaCount <= 1 {
			schemaName = ""
			row.TableName = tableName
		}
		row.DBType = "Source"
		err := updateRowForOutCounts(&row, tableName, schemaName) //source OUT counts
		if err != nil {
			utils.ErrExit("error while getting OUT counts for source DB: %w\n", err)
		}
		if fBEnabled {
			err = updateRowForInCounts(&row, tableName, schemaName, msr.SourceDBAsTargetConf) //fall back IN counts
			if err != nil {
				utils.ErrExit("error while getting IN counts for source DB in case of fall-back: %w\n", err)
			}
		}

		reportTable.AddRow(row.TableName, row.DBType, row.SnapshotRowCount, row.InsertsIn, row.UpdatesIn, row.DeletesIn, row.InsertsOut, row.UpdatesOut, row.DeletesOut, row.FinalRowCount)
		row = rowData{}
		row.TableName = ""
		row.DBType = "Target"
		err = updateRowForInCounts(&row, tableName, schemaName, msr.TargetDBConf) //target IN counts
		if err != nil {
			utils.ErrExit("error while getting IN counts for target DB: %w\n", err)
		}
		if fFEnabled || fBEnabled {
			err = updateRowForOutCounts(&row, tableName, schemaName) // target OUT counts
			if err != nil {
				utils.ErrExit("error while getting OUT counts for target DB: %w\n", err)
			}
		}
		reportTable.AddRow(row.TableName, row.DBType, row.SnapshotRowCount, row.InsertsIn, row.UpdatesIn, row.DeletesIn, row.InsertsOut, row.UpdatesOut, row.DeletesOut, row.FinalRowCount)

		if fFEnabled {
			row = rowData{}
			row.TableName = ""
			row.DBType = "Fall Forward"
			err = updateRowForInCounts(&row, tableName, schemaName, msr.FallForwardDBConf) //fall forward IN counts
			if err != nil {
				utils.ErrExit("error while getting IN counts for fall-forward DB: %w\n", err)
			}
			reportTable.AddRow(row.TableName, row.DBType, row.SnapshotRowCount, row.InsertsIn, row.UpdatesIn, row.DeletesIn, row.InsertsOut, row.UpdatesOut, row.DeletesOut, row.FinalRowCount)
		}
	}

	if len(tableList) > 0 {
		fmt.Print("\n")
		fmt.Println(reportTable)
		fmt.Print("\n")
	}

}

func updateRowForInCounts(row *rowData, tableName string, schemaName string, targetConf *tgtdb.TargetConf) error {
	switch row.DBType {
	case "Target":
		importerRole = TARGET_DB_IMPORTER_ROLE
	case "Fall Forward":
		importerRole = FF_DB_IMPORTER_ROLE
	case "Source":
		importerRole = FB_DB_IMPORTER_ROLE
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
	var dataFileDescriptor *datafile.Descriptor

	state := NewImportDataState(exportDir)
	dataFileDescriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	if utils.FileOrFolderExists(dataFileDescriptorPath) {
		dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	} else {
		return fmt.Errorf("data file descriptor not found at %s for snapshot", dataFileDescriptorPath)
	}

	dataFile := dataFileDescriptor.GetDataFileEntryByTableName(tableName)

	if importerRole != FB_DB_IMPORTER_ROLE {
		row.SnapshotRowCount, err = state.GetImportedRowCount(dataFile.FilePath, dataFile.TableName)
		if err != nil {
			return fmt.Errorf("get imported row count for table %q for DB type %s: %w", row.TableName, row.DBType, err)
		}
	}

	eventCounter, err := state.GetImportedEventsStatsForTable(tableName, migrationUUID)
	if err != nil {
		return fmt.Errorf("get imported events stats for table %q for DB type %s: %w", row.TableName, row.DBType, err)
	}
	if importerRole == FB_DB_IMPORTER_ROLE {
		exportedEventCounter, err := metaDB.GetExportedEventsStatsForTableAndExporterRole(SOURCE_DB_EXPORTER_ROLE, schemaName, tableName)
		if err != nil {
			utils.ErrExit("could not fetch table stats from meta db: %v", err)
		}
		eventCounter.Merge(exportedEventCounter)
	}
	row.InsertsIn = eventCounter.NumInserts
	row.UpdatesIn = eventCounter.NumUpdates
	row.DeletesIn = eventCounter.NumDeletes
	row.FinalRowCount = row.SnapshotRowCount + eventCounter.NumInserts - eventCounter.NumDeletes
	return nil
}

func updateRowForOutCounts(row *rowData, tableName string, schemaName string) error {
	switch row.DBType {
	case "Source":
		exporterRole = SOURCE_DB_EXPORTER_ROLE
	case "Target":
		if fFEnabled {
			exporterRole = TARGET_DB_EXPORTER_FF_ROLE
		} else if fBEnabled {
			exporterRole = TARGET_DB_EXPORTER_FB_ROLE
		}
	}
	eventCounter, err := metaDB.GetExportedEventsStatsForTableAndExporterRole(exporterRole, schemaName, tableName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("could not fetch table stats from meta DB: %w", err)
	}
	row.InsertsOut = eventCounter.NumInserts
	row.UpdatesOut = eventCounter.NumUpdates
	row.DeletesOut = eventCounter.NumDeletes
	row.FinalRowCount = row.SnapshotRowCount + eventCounter.NumInserts - eventCounter.NumDeletes
	return nil
}

func init() {
	liveMigrationCommand.AddCommand(liveMigrationReportCmd)
	registerCommonGlobalFlags(liveMigrationReportCmd)
	liveMigrationReportCmd.Flags().StringVar(&ffDbPassword, "ff-db-password", "",
		"password with which to connect to the target fall-forward DB server. Alternatively, you can also specify the password by setting the environment variable FF_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")

	liveMigrationReportCmd.Flags().StringVar(&sourceDbPassword, "source-db-password", "",
		"password with which to connect to the target source DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes")

	liveMigrationReportCmd.Flags().StringVar(&targetDbPassword, "target-db-password", "",
		"password with which to connect to the target YugabyteDB server. Alternatively, you can also specify the password by setting the environment variable TARGET_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")
}
