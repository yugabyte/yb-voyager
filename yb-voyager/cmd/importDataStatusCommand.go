package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
)

var importDataStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print status of an ongoing/completed data import.",

	Run: func(cmd *cobra.Command, args []string) {
		validateExportDirFlag()
		err := runImportDataStatusCmd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	importDataCmd.AddCommand(importDataStatusCmd)
}

//totalCount stores the total number of rows in case of import data command and the total size of the file in bytes in case of import data file command.
//importedCount stores the number of rows imported in case of import data command and the size of the file imported in bytes in case of import data file command.
type tableMigStatusOutputRow struct {
	tableName          string
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

	dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	var totalRowCountMap map[string]int64
	if dataFileDescriptor.TableRowCount != nil {
		totalRowCountMap = dataFileDescriptor.TableRowCount
	} else {
		totalRowCountMap = dataFileDescriptor.TableFileSize
	}

	tableNames := []string{}
	for tableName := range totalRowCountMap {
		tableNames = append(tableNames, tableName)
	}

	importedRowCountMap := getImportedRowsCount(exportDir, tableNames)

	if len(tableNames) > 0 {
		if dataFileDescriptor.TableRowCount != nil {
			// case of importData where row counts is available
			fmt.Printf("%-30s %-12s %13s %13s %10s\n", "TABLE", "STATUS", "TOTAL_ROWS", "IMPORTED_ROWS", "PERCENTAGE")
		} else {
			// case of importDataFileCommand where file size is available not row counts
			fmt.Printf("%-30s %-12s %15s %13s %10s\n", "TABLE", "STATUS", "TOTAL_SIZE(MB)", "IMPORTED_SIZE(MB)", "PERCENTAGE")
		}
	}

	var outputRows []*tableMigStatusOutputRow
	for _, tableName := range tableNames {
		//totalCount stores the total number of rows in case of import data command and the total size of the file in bytes in case of import data file command.
		totalCount := totalRowCountMap[tableName]
		//importedCount stores the number of rows imported in case of import data command and the size of the file imported in bytes in case of import data file command.
		importedCount := importedRowCountMap[tableName]
		if importedCount == -1 {
			importedCount = 0
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
			tableName:          tableName,
			status:             status,
			totalCount:         totalCount,
			importedCount:      importedCount,
			percentageComplete: perc,
		}

		outputRows = append(outputRows, row)
	}

	// First sort by status and then by table-name.
	sort.Slice(outputRows, func(i, j int) bool {
		ordStates := map[string]int{"MIGRATING": 1, "DONE": 2, "NOT_STARTED": 3}
		row1 := outputRows[i]
		row2 := outputRows[j]
		if row1.status == row2.status {
			return strings.Compare(row1.tableName, row2.tableName) < 0
		} else {
			return ordStates[row1.status] < ordStates[row2.status]
		}
	})
	for _, row := range outputRows {
		if dataFileDescriptor.TableRowCount != nil {
			// case of importData where row counts is available
			fmt.Printf("%-30s %-12s %13d %13d %10.2f\n",
				row.tableName, row.status, row.totalCount, row.importedCount, row.percentageComplete)
		} else {
			// case of importDataFileCommand where file size is available not row counts
			fmt.Printf("%-30s %-12s %14.2f %17.2f %11.2f\n",
				row.tableName, row.status, float64(row.totalCount)/1000000.0, float64(row.importedCount)/1000000.0, row.percentageComplete)
		}
	}
	return nil
}
