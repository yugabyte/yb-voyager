package cmd

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var importDataStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print status of an ongoing/completed data import.",

	Run: func(cmd *cobra.Command, args []string) {
		validateExportDirFlag()
		err := runImportDataStatusCmd()
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
	exportDataDoneFlagFilePath := filepath.Join(exportDir, "metainfo/flags/exportDataDone")
	_, err := os.Stat(exportDataDoneFlagFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot run `import data status` before data export is done")
		}
		return fmt.Errorf("check if data export is done: %w", err)
	}

	table, err := prepareImportDataStatusTable()
	if err != nil {
		return fmt.Errorf("prepare import data status table: %w", err)
	}
	uiTable := uitable.New()
	headerfmt := color.New(color.FgGreen, color.Underline).SprintFunc()
	for i, row := range table {
		perc := fmt.Sprintf("%.2f", row.percentageComplete)
		if reportProgressInBytes {
			if i == 0 {
				uiTable.AddRow(headerfmt("TABLE"), headerfmt("FILE"), headerfmt("STATUS"), headerfmt("TOTAL SIZE(MB)"), headerfmt("IMPORTED SIZE(MB)"), headerfmt("PERCENTAGE"))
			}
			// case of importDataFileCommand where file size is available not row counts
			uiTable.AddRow(row.tableName, row.fileName, row.status, float64(row.totalCount)/1000000.0, float64(row.importedCount)/1000000.0, perc)
		} else {
			if i == 0 {
				uiTable.AddRow(headerfmt("TABLE"), headerfmt("FILE"), headerfmt("STATUS"), headerfmt("TOTAL ROWS"), headerfmt("IMPORTED ROWS"), headerfmt("PERCENTAGE"))
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

func prepareImportDataStatusTable() ([]*tableMigStatusOutputRow, error) {
	var table []*tableMigStatusOutputRow
	state := NewImportDataState(exportDir)
	dataFileDescriptor = datafile.OpenDescriptor(exportDir)

	for _, dataFile := range dataFileDescriptor.DataFileList {
		var totalCount, importedCount int64
		var err error

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
		perc := float64(importedCount) * 100.0 / float64(totalCount)
		var status string
		switch true {
		case importedCount == totalCount:
			status = "DONE"
		case importedCount == 0:
			status = "NOT_STARTED"
		case importedCount < totalCount:
			status = "MIGRATING"
		}
		row := &tableMigStatusOutputRow{
			fileName:           path.Base(dataFile.FilePath),
			tableName:          dataFile.TableName,
			status:             status,
			totalCount:         totalCount,
			importedCount:      importedCount,
			percentageComplete: perc,
		}
		table = append(table, row)
	}
	// First sort by status and then by table-name.
	sort.Slice(table, func(i, j int) bool {
		ordStates := map[string]int{"MIGRATING": 1, "DONE": 2, "NOT_STARTED": 3}
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
