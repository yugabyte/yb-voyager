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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)


var importDataStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print status of an ongoing/completed import data/fall-forward setup.",

	Run: func(cmd *cobra.Command, args []string) {
		streamChanges, err := checkWithStreamingMode()
		if err != nil {
			utils.ErrExit("error while checking streaming mode: %w\n", err)
		}
		if streamChanges {
			utils.ErrExit("\nNote: Run the following command to get the report of live migration:\n"+
				color.CyanString("yb-voyager get data-migration-report --export-dir %q\n", exportDir))
		}
		color.Cyan("Import Data Status for TargetDB\n")
		importerRole = TARGET_DB_IMPORTER_ROLE
		err = runImportDataStatusCmd()
		if err != nil {
			utils.ErrExit("error: %s\n", err)
		}
	},
}

func init() {
	importDataCmd.AddCommand(importDataStatusCmd)
}

// totalCount and importedCount store row-count for import data command and byte-count for import data file command.
type tableMigStatusOutputRow struct {
	schemaName         string
	tableName          string
	fileName           string
	status             string
	totalCount         int64
	importedCount      int64
	percentageComplete float64
}

// Note that the `import data status` is running in a separate process. It won't have access to the in-memory state
// held in the main `import data` process.
func runImportDataStatusCmd() error {
	if !dataIsExported() {
		return fmt.Errorf("cannot run `import data status` before data export is done")
	}
	table, err := prepareImportDataStatusTable()
	if err != nil {
		return fmt.Errorf("prepare import data status table: %w", err)
	}
	uiTable := uitable.New()
	for i, row := range table {
		perc := fmt.Sprintf("%.2f", row.percentageComplete)
		if reportProgressInBytes {
			if i == 0 {
				addHeader(uiTable, "TABLE", "FILE", "STATUS", "TOTAL SIZE", "IMPORTED SIZE", "PERCENTAGE")
			}
			// case of importDataFileCommand where file size is available not row counts
			totalCount := utils.HumanReadableByteCount(row.totalCount)
			importedCount := utils.HumanReadableByteCount(row.importedCount)
			uiTable.AddRow(row.tableName, row.fileName, row.status, totalCount, importedCount, perc)
		} else {
			if i == 0 {
				addHeader(uiTable, "TABLE", "FILE", "STATUS", "TOTAL ROWS", "IMPORTED ROWS", "PERCENTAGE")
			}
			// case of importData where row counts is available
			uiTable.AddRow(row.tableName, row.fileName, row.status, row.totalCount, row.importedCount, perc)
		}
	}

	if len(table) > 0 {
		fmt.Print("\n")
		fmt.Println(uiTable)
		fmt.Print("\n")
	}

	return nil
}

func prepareDummyDescriptor(state *ImportDataState) (*datafile.Descriptor, error) {
	var dataFileDescriptor datafile.Descriptor

	tableToFilesMapping, err := state.DiscoverTableToFilesMapping()
	if err != nil {
		return nil, fmt.Errorf("discover table to files mapping: %w", err)
	}
	for tableName, filePaths := range tableToFilesMapping {
		for _, filePath := range filePaths {
			fileSize, err := datastore.NewDataStore(filePath).FileSize(filePath)
			if err != nil {
				return nil, fmt.Errorf("get file size of %q: %w", filePath, err)
			}
			dataFileDescriptor.DataFileList = append(dataFileDescriptor.DataFileList, &datafile.FileEntry{
				FilePath:  filePath,
				TableName: tableName,
				FileSize:  fileSize,
				RowCount:  -1,
			})
		}
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

	for _, dataFile := range dataFileDescriptor.DataFileList {
		row, err := prepareRowWithDatafile(dataFile, state)
		if err != nil {
			return nil, fmt.Errorf("prepare row with datafile: %w", err)
		}
		table = append(table, row)
	}

	// First sort by status and then by table-name.
	sort.Slice(table, func(i, j int) bool {
		ordStates := map[string]int{"MIGRATING": 1, "DONE": 2, "NOT_STARTED": 3, "STREAMING": 4}
		row1 := table[i]
		row2 := table[j]
		if row1.status == row2.status {
			if row1.tableName == row2.tableName {
				return strings.Compare(row1.fileName, row2.fileName) < 0
			} else {
				return strings.Compare(row1.tableName, row2.tableName) < 0
			}
		} else {
			return ordStates[row1.status] < ordStates[row2.status]
		}
	})

	return table, nil
}

func prepareRowWithDatafile(dataFile *datafile.FileEntry, state *ImportDataState) (*tableMigStatusOutputRow, error) {
	var totalCount, importedCount int64
	var err error
	var perc float64
	var status string
	reportProgressInBytes = reportProgressInBytes || dataFile.RowCount == -1

	if reportProgressInBytes {
		totalCount = dataFile.FileSize
		importedCount, err = state.GetImportedByteCount(dataFile.FilePath, dataFile.TableName)
	} else {
		totalCount = dataFile.RowCount
		importedCount, err = state.GetImportedRowCount(dataFile.FilePath, dataFile.TableName)

	}
	if err != nil {
		return nil, fmt.Errorf("compute imported data size: %w", err)
	}

	if totalCount != 0 {
		perc = float64(importedCount) * 100.0 / float64(totalCount)
	}
	switch true {
	case importedCount == totalCount:
		status = "DONE"
	case importedCount == 0:
		status = "NOT_STARTED"
	case importedCount < totalCount:
		status = "MIGRATING"
	}
	row := &tableMigStatusOutputRow{
		tableName:          dataFile.TableName,
		schemaName:         getTargetSchemaName(dataFile.TableName),
		fileName:           path.Base(dataFile.FilePath),
		status:             status,
		totalCount:         totalCount,
		importedCount:      importedCount,
		percentageComplete: perc,
	}
	return row, nil
}
