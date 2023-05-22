package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/maps"
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

	tableNames := maps.Keys(totalRowCountMap)
	state := NewImportDataState(exportDir)
	importedRowCountMap, err := state.GetImportProgressAmount(tableNames)
	if err != nil {
		return fmt.Errorf("get imported rows count: %w", err)
	}
	table := uitable.New()
	headerfmt := color.New(color.FgGreen, color.Underline).SprintFunc()

	if dataFileDescriptor.TableRowCount != nil {
		// case of importData where row counts is available
		table.AddRow(headerfmt("TABLE"), headerfmt("STATUS"), headerfmt("TOTAL ROWS"), headerfmt("IMPORTED ROWS"), headerfmt("PERCENTAGE"))
	} else {
		// case of importDataFileCommand where file size is available not row counts
		table.AddRow(headerfmt("TABLE"), headerfmt("STATUS"), headerfmt("TOTAL SIZE(MB)"), headerfmt("IMPORTED SIZE(MB)"), headerfmt("PERCENTAGE"))
	}

	var outputRows []*tableMigStatusOutputRow
	for _, tableName := range tableNames {
		// totalCount and importedCount store row-count for import data command and byte-count for import data file command.
		totalCount := totalRowCountMap[tableName]
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
			table.AddRow(row.tableName, row.status, row.totalCount, row.importedCount, row.percentageComplete)
		} else {
			// case of importDataFileCommand where file size is available not row counts
			table.AddRow(row.tableName, row.status, float64(row.totalCount)/1000000.0, float64(row.importedCount)/1000000.0, row.percentageComplete)
		}
	}

	if len(tableNames) > 0 {
		fmt.Print("\n")
		fmt.Println(table)
		fmt.Print("\n")
	}

	return nil
}
