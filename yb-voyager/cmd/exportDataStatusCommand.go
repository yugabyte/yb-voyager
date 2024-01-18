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
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const exportDataStatusMsg = "Export Data Status for SourceDB\n"

var exportDataStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print status of an ongoing/completed data export.",

	Run: func(cmd *cobra.Command, args []string) {
		streamChanges, err := checkStreamingMode()
		if err != nil {
			utils.ErrExit("error while checking streaming mode: %w\n", err)
		}
		if streamChanges {
			utils.ErrExit("\nNote: Run the following command to get the current report of live migration:\n" +
				color.CyanString("yb-voyager get data-migration-report --export-dir %q\n", exportDir))
		}
		useDebezium = dbzm.IsDebeziumForDataExport(exportDir)
		if useDebezium {
			err = runExportDataStatusCmdDbzm(streamChanges)
		} else {
			err = runExportDataStatusCmd()
		}
		if err != nil {
			utils.ErrExit("error: %s\n", err)
		}
	},
}

func init() {
	exportDataCmd.AddCommand(exportDataStatusCmd)
}

type exportTableMigStatusOutputRow struct {
	tableName     string
	status        string
	exportedCount int64
}

var InProgressTableSno int

// Note that the `export data status` is running in a separate process. It won't have access to the in-memory state
// held in the main `export data` process.
func runExportDataStatusCmdDbzm(streamChanges bool) error {
	exportStatusFilePath := filepath.Join(exportDir, "data", "export_status.json")
	status, err := dbzm.ReadExportStatus(exportStatusFilePath)
	if err != nil {
		utils.ErrExit("Failed to read export status file %s: %v", exportStatusFilePath, err)
	}
	InProgressTableSno = status.InProgressTableSno()
	var rows []*exportTableMigStatusOutputRow
	var row *exportTableMigStatusOutputRow

	for _, tableStatus := range status.Tables {
		row = getSnapshotExportStatusRow(&tableStatus)
		rows = append(rows, row)
	}
	displayExportDataStatus(rows)
	return nil
}

func getSnapshotExportStatusRow(tableStatus *dbzm.TableExportStatus) *exportTableMigStatusOutputRow {
	row := &exportTableMigStatusOutputRow{
		tableName:     tableStatus.TableName,
		status:        "DONE",
		exportedCount: tableStatus.ExportedRowCountSnapshot,
	}
	if tableStatus.Sno == InProgressTableSno && dbzm.IsLiveMigrationInSnapshotMode(exportDir) {
		row.status = "EXPORTING"
	}
	return row
}

func runExportDataStatusCmd() error {
	tableMap := make(map[string]string)
	dataDir := filepath.Join(exportDir, "data")
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("error while getting migration status record: %v", err)
	}
	source := msr.SourceDBConf
	if source.DBType == "postgresql" {
		tableMap = getMappingForTableNameVsTableFileName(dataDir, true)
	} else if source.DBType == "mysql" || source.DBType == "oracle" {
		files, err := filepath.Glob(filepath.Join(dataDir, "*_data.sql"))
		if err != nil {
			return fmt.Errorf("error while checking data directory for export data status: %v", err)
		}
		var fileName string
		for _, file := range files {
			fileName = filepath.Base(file)
			//Sample file name: [tmp_]YB_VOYAGER_TEST_data.sql
			if strings.HasPrefix(fileName, "tmp_") {
				tableMap[fileName[4:]] = fileName
			} else {
				tableMap[fileName] = "tmp_" + fileName
			}
		}
	} else {
		return fmt.Errorf("unable to identify source-db-type")
	}
	var outputRows []*exportTableMigStatusOutputRow
	var finalFullTableName string
	exportStatusSnapshotFilePath := filepath.Join(exportDir, "metainfo", "export_snapshot_status.json")
	exportStatusSnapshot, err := srcdb.ReadExportStatus(exportStatusSnapshotFilePath)
	if err != nil {
		utils.ErrExit("Failed to read export status file %s: %v", exportStatusSnapshotFilePath, err)
	}
	for tableName := range tableMap {
		//"_" is treated as a wildcard character in regex query for Glob
		if tableName == "tmp_postdata.sql" || tableName == "tmp_data.sql" {
			continue
		}
		if strings.HasPrefix(tableName, "public.") {
			finalFullTableName = tableName[7:]
		} else {
			finalFullTableName = tableName
		}
		schemaName := ""
		statusTableName := ""
		if source.DBType == ORACLE || source.DBType == MYSQL {
			finalFullTableName = tableName[:len(tableName)-len("_data.sql")]
			statusTableName = strings.Trim(finalFullTableName,`"`) //for reserved words datafile will be quoted but not in the status file
		}

		if source.DBType == POSTGRESQL {
			schemaName = strings.Split(tableName, ".")[0]
			statusTableName = strings.Split(tableName, ".")[1]
		} else if source.DBType == MYSQL {
			schemaName = source.DBName
		} else if source.DBType == ORACLE {
			schemaName = source.Schema
		}
		tableStatus := exportStatusSnapshot.GetTableExportStatus(statusTableName, schemaName)
		row := &exportTableMigStatusOutputRow{
			tableName:     finalFullTableName,
			status:        tableStatus.Status,
			exportedCount: tableStatus.ExportedRowCountSnapshot,
		}
		outputRows = append(outputRows, row)
	}

	displayExportDataStatus(outputRows)
	return nil
}

func displayExportDataStatus(rows []*exportTableMigStatusOutputRow) {
	color.Cyan(exportDataStatusMsg)
	table := uitable.New()
	addHeader(table, "TABLE", "STATUS", "EXPORTED ROWS")

	// First sort by status and then by table-name.
	sort.Slice(rows, func(i, j int) bool {
		ordStates := map[string]int{"EXPORTING": 1, "DONE": 2, "NOT_STARTED": 3, "STREAMING": 4}
		row1 := rows[i]
		row2 := rows[j]
		if row1.status == row2.status {
			return strings.Compare(row1.tableName, row2.tableName) < 0
		} else {
			return ordStates[row1.status] < ordStates[row2.status]
		}
	})
	for _, row := range rows {
		table.AddRow(row.tableName, row.status, row.exportedCount)
	}
	if len(rows) > 0 {
		fmt.Print("\n")
		fmt.Println(table)
		fmt.Print("\n")
	}
}
