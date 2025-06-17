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
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
)

const importDataStatusMsg = "Import Data Status for TargetDB\n"

var importDataStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print status of an ongoing/completed import data.",
	PreRun: func(cmd *cobra.Command, args []string) {
		validateReportOutputFormat(migrationReportFormats, reportOrStatusCmdOutputFormat)
	},
	Run: func(cmd *cobra.Command, args []string) {
		streamChanges, err := checkStreamingMode()
		if err != nil {
			utils.ErrExit("error while checking streaming mode: %w\n", err)
		}
		if streamChanges {
			utils.ErrExit("\nNote: Run the following command to get the report of live migration:\n" +
				color.CyanString("yb-voyager get data-migration-report --export-dir %q\n", exportDir))
		}
		dataFileDescriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
		if utils.FileOrFolderExists(dataFileDescriptorPath) {
			importerRole = TARGET_DB_IMPORTER_ROLE
		} else {
			importerRole = IMPORT_FILE_ROLE
		}

		err = InitNameRegistry(exportDir, "", nil, nil, nil, nil, false)
		if err != nil {
			utils.ErrExit("initialize name registry: %v", err)
		}
		err = runImportDataStatusCmd()
		if err != nil {
			utils.ErrExit("error running import data status: %s\n", err)
		}
	},
}
var reportOrStatusCmdOutputFormat string

func init() {
	importDataCmd.AddCommand(importDataStatusCmd)
	importDataStatusCmd.Flags().StringVar(&reportOrStatusCmdOutputFormat, "output-format", "table",
		"format in which report will be generated: (table, json)")
	importDataStatusCmd.Flags().MarkHidden("output-format") //confirm this if should be hidden or not
}

// totalCount and importedCount store row-count for import data command and byte-count for import data file command.
type tableMigStatusOutputRow struct {
	TableName          string  `json:"table_name"`
	FileName           string  `json:"file_name,omitempty"`
	Status             string  `json:"status"`
	TotalCount         int64   `json:"total_count"`
	ImportedCount      int64   `json:"imported_count"`
	ErroredCount       int64   `json:"errored_count"`
	PercentageComplete float64 `json:"percentage_complete"`
}

// Note that the `import data status` is running in a separate process. It won't have access to the in-memory state
// held in the main `import data` process.
func runImportDataStatusCmd() error {
	if !dataIsExported() {
		return fmt.Errorf("cannot run `import data status` before data export is done")
	}
	rows, err := prepareImportDataStatusTable()
	if err != nil {
		return fmt.Errorf("prepare import data status table: %w", err)
	}
	if reportOrStatusCmdOutputFormat == "json" {
		// Print the report in json format.
		reportFilePath := filepath.Join(exportDir, "reports", "import-data-status-report.json")
		reportFile := jsonfile.NewJsonFile[[]*tableMigStatusOutputRow](reportFilePath)
		err := reportFile.Create(&rows)
		if err != nil {
			utils.ErrExit("creating into json file: %s: %v", reportFilePath, err)
		}
		fmt.Print(color.GreenString("Import data status report is written to %s\n", reportFilePath))
		return nil
	}

	color.Cyan(importDataStatusMsg)
	uiTable := uitable.New()
	for i, row := range rows {
		perc := fmt.Sprintf("%.2f", row.PercentageComplete)
		if reportProgressInBytes {
			if i == 0 {
				addHeader(uiTable, "TABLE", "FILE", "STATUS", "TOTAL SIZE", "IMPORTED SIZE", "ERRORED SIZE", "PERCENTAGE")
			}
			// case of importDataFileCommand where file size is available not row counts
			totalCount := utils.HumanReadableByteCount(row.TotalCount)
			importedCount := utils.HumanReadableByteCount(row.ImportedCount)
			erroredCount := utils.HumanReadableByteCount(row.ErroredCount)

			uiTable.AddRow(row.TableName, row.FileName, row.Status, totalCount, importedCount, erroredCount, perc)

		} else {
			if i == 0 {
				addHeader(uiTable, "TABLE", "STATUS", "TOTAL ROWS", "IMPORTED ROWS", "ERRORED ROWS", "PERCENTAGE")
			}
			// case of importData where row counts is available
			uiTable.AddRow(row.TableName, row.Status, row.TotalCount, row.ImportedCount, row.ErroredCount, perc)
		}
	}

	if len(rows) > 0 {
		fmt.Print("\n")
		fmt.Println(uiTable)
		fmt.Print("\n")
	}

	return nil
}

func prepareDummyDescriptor(state *ImportDataState) (*datafile.Descriptor, error) {
	var dataFileDescriptor datafile.Descriptor
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return nil, fmt.Errorf("getting migration status record: %v", err)
	}
	dataStore = datastore.NewDataStore(msr.ImportDataFileFlagDataDir)
	importFileTasksForFTM := getImportFileTasks(msr.ImportDataFileFlagFileTableMapping)
	for _, task := range importFileTasksForFTM {
		dataFileDescriptor.DataFileList = append(dataFileDescriptor.DataFileList, &datafile.FileEntry{
			FilePath:  task.FilePath,
			TableName: task.TableNameTup.ForKey(),
			FileSize:  task.FileSize,
			RowCount:  -1,
		})
	}

	return &dataFileDescriptor, nil
}

func prepareImportDataStatusTable() ([]*tableMigStatusOutputRow, error) {
	var table []*tableMigStatusOutputRow
	var err error
	var dataFileDescriptor *datafile.Descriptor

	state := NewImportDataState(exportDir)
	dataFileDescriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	if utils.FileOrFolderExists(dataFileDescriptorPath) {
		// Case of `import data` command where row counts are available.
		dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	} else {
		// Case of `import data file` command where row counts are not available.
		// Use file sizes for progress reporting.
		importerRole = IMPORT_FILE_ROLE
		state = NewImportDataState(exportDir)
		dataFileDescriptor, err = prepareDummyDescriptor(state)
		if err != nil {
			return nil, fmt.Errorf("prepare dummy descriptor: %w", err)
		}
	}

	outputRows := make(map[string]*tableMigStatusOutputRow)

	for _, dataFile := range dataFileDescriptor.DataFileList {
		row, err := prepareRowWithDatafile(dataFile, state)
		if err != nil {
			return nil, fmt.Errorf("prepare row with datafile: %w", err)
		}
		if importerRole == IMPORT_FILE_ROLE {
			outputRows[row.TableName] = row
		} else {
			// In import-data, for partitioned tables, we may have multiple data files for the same table.
			// We aggregate the counts for such tables.
			var existingRow *tableMigStatusOutputRow
			var found bool
			existingRow, found = outputRows[row.TableName]
			if !found {
				existingRow = &tableMigStatusOutputRow{}
				outputRows[row.TableName] = existingRow
			}
			existingRow.TableName = row.TableName
			existingRow.TotalCount += row.TotalCount
			existingRow.ImportedCount += row.ImportedCount
			existingRow.ErroredCount += row.ErroredCount
		}
	}

	for _, row := range outputRows {
		row.PercentageComplete = (float64(row.ImportedCount) + float64(row.ErroredCount)) * 100.0 / float64(row.TotalCount)
		if row.PercentageComplete == 100 {
			if row.ErroredCount > 0 {
				row.Status = "DONE_WITH_ERRORS"
			} else {
				row.Status = "DONE"
			}
		} else if row.PercentageComplete == 0 {
			row.Status = "NOT_STARTED"
		} else {
			row.Status = "MIGRATING"
		}
		table = append(table, row)
	}

	// First sort by status and then by table-name.
	sort.Slice(table, func(i, j int) bool {
		ordStates := map[string]int{"MIGRATING": 1, "DONE": 2, "DONE_WITH_ERRORS": 3, "NOT STARTED": 4, "STREAMING": 5}
		row1 := table[i]
		row2 := table[j]
		if row1.Status == row2.Status {
			if row1.TableName == row2.TableName {
				return strings.Compare(row1.FileName, row2.FileName) < 0
			} else {
				return strings.Compare(row1.TableName, row2.TableName) < 0
			}
		} else {
			return ordStates[row1.Status] < ordStates[row2.Status]
		}
	})

	return table, nil
}

func prepareRowWithDatafile(dataFile *datafile.FileEntry, state *ImportDataState) (*tableMigStatusOutputRow, error) {
	var totalCount, importedCount, erroredCount int64
	var err error
	var perc float64
	var status string
	reportProgressInBytes = reportProgressInBytes || dataFile.RowCount == -1
	dataFileNt, err := namereg.NameReg.LookupTableName(dataFile.TableName)
	if err != nil {
		return nil, fmt.Errorf("lookup %s from name registry: %v", dataFile.TableName, err)
	}

	if reportProgressInBytes {
		totalCount = dataFile.FileSize
		importedCount, err = state.GetImportedByteCount(dataFile.FilePath, dataFileNt)
		if err != nil {
			return nil, fmt.Errorf("compute imported data size: %w", err)
		}
		erroredCount, err = state.GetErroredByteCount(dataFile.FilePath, dataFileNt)
		if err != nil {
			return nil, fmt.Errorf("compute errored data size: %w", err)
		}
	} else {
		totalCount = dataFile.RowCount
		importedCount, err = state.GetImportedRowCount(dataFile.FilePath, dataFileNt)
		if err != nil {
			return nil, fmt.Errorf("compute imported data size: %w", err)
		}
		erroredCount, err = state.GetErroredRowCount(dataFile.FilePath, dataFileNt)
		if err != nil {
			return nil, fmt.Errorf("compute errored data size: %w", err)
		}
	}

	if totalCount != 0 {
		perc = float64(importedCount) * 100.0 / float64(totalCount)
	}

	row := &tableMigStatusOutputRow{
		TableName:          dataFileNt.ForKey(),
		FileName:           path.Base(dataFile.FilePath),
		Status:             status,
		TotalCount:         totalCount,
		ImportedCount:      importedCount,
		ErroredCount:       erroredCount,
		PercentageComplete: perc,
	}
	return row, nil
}
