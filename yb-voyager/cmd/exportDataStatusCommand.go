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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
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
		err = InitNameRegistry(exportDir, SOURCE_DB_EXPORTER_ROLE, nil, nil, nil, nil)
		if err != nil {
			utils.ErrExit("initializing name registry: %v", err)
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
	nt, err := namereg.NameReg.LookupTableName(fmt.Sprintf("%s.%s", tableStatus.SchemaName, tableStatus.TableName))
	if err != nil {
		utils.ErrExit("lookup %s in name registry: %v", tableStatus.TableName, err)
	}
	row := &exportTableMigStatusOutputRow{
		tableName:     nt.CurrentName.MinQualified.MinQuoted,
		status:        "DONE",
		exportedCount: tableStatus.ExportedRowCountSnapshot,
	}
	if tableStatus.Sno == InProgressTableSno && dbzm.IsLiveMigrationInSnapshotMode(exportDir) {
		row.status = "EXPORTING"
	}
	return row
}

func runExportDataStatusCmd() error {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("error while getting migration status record: %v", err)
	}
	tableList := msr.TableListExportedFromSource
	source = *msr.SourceDBConf
	sqlname.SourceDBType = source.DBType
	var outputRows []*exportTableMigStatusOutputRow
	exportSnapshotStatusFilePath := filepath.Join(exportDir, "metainfo", "export_snapshot_status.json")
	exportSnapshotStatusFile = jsonfile.NewJsonFile[ExportSnapshotStatus](exportSnapshotStatusFilePath)
	exportStatusSnapshot, err := exportSnapshotStatusFile.Read()
	if err != nil {
		utils.ErrExit("Failed to read export status file %s: %v", exportSnapshotStatusFilePath, err)
	}

	exportedSnapshotRow, exportedSnapshotStatus, err := getExportedSnapshotRowsMap(exportStatusSnapshot)
	if err != nil {
		return fmt.Errorf("error while getting exported snapshot rows map: %v", err)
	}

	leafPartitions := getLeafPartitionsFromRootTable()

	for _, tableName := range tableList {
		finalFullTableName, err := namereg.NameReg.LookupTableName(tableName)
		if err != nil {
			return fmt.Errorf("lookup %s in name registry: %v", tableName, err)
		}
		displayTableName := finalFullTableName.CurrentName.MinQualified.MinQuoted
		partitions := leafPartitions[finalFullTableName.CurrentName.Qualified.MinQuoted]
		if source.DBType == POSTGRESQL && partitions != nil {
			partitions := strings.Join(partitions, ", ")
			displayTableName = fmt.Sprintf("%s (%s)", displayTableName, partitions)
		}
		snapshotStatus, _ := exportedSnapshotStatus.Get(finalFullTableName)
		finalStatus := snapshotStatus[0]
		if len(snapshotStatus) > 1 { // status for root partition wrt leaf partitions
			exportingLeaf := 0
			doneLeaf := 0
			not_started := 0
			for _, status := range snapshotStatus {
				if status == "EXPORTING" {
					exportingLeaf++
				} else if status == "DONE" {
					doneLeaf++
				} else {
					not_started++
				}
			}
			if exportingLeaf > 0 {
				finalStatus = "EXPORTING"
			} else if doneLeaf == len(snapshotStatus) {
				finalStatus = "DONE"
			} else if not_started == len(snapshotStatus) {
				finalStatus = "NOT_STARTED"
			}
		}
		exportedCount, _ := exportedSnapshotRow.Get(finalFullTableName)
		row := &exportTableMigStatusOutputRow{
			tableName:     displayTableName,
			status:        finalStatus,
			exportedCount: exportedCount,
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
