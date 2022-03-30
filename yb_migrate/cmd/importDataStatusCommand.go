package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/ybm/yb_migrate/src/migration"
)

var importDataStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print status of an ongoing/completed migration.",

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)
	},

	Run: func(cmd *cobra.Command, args []string) {
		err := runImportDataStatusCmd()
		if err != nil {
			log.Errorf("Get import data status failed: %s", err)
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	importDataCmd.AddCommand(importDataStatusCmd)
}

type tableMigStatusOutputRow struct {
	tableName          string
	status             string
	totalRowCount      int64
	importedRowCount   int64
	percentageComplete float64
}

// Note that the `import data status` is running in a separate process. It won't have access to the in-memory state
// held in the main `import data` process.
func runImportDataStatusCmd() error {
	exportDataDoneFlagFilePath := filepath.Join(exportDir, "metainfo/flags/exportDataDone")
	_, err := os.Stat(exportDataDoneFlagFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot run `import data status` before data export is done.")
		}
		return fmt.Errorf("check if data export is done: %w", err)
	}

	rowCountFilePath := exportDir + "/metainfo/flags/tablesrowcount"
	totalRowCountMap := migration.GetTableRowCount(rowCountFilePath)

	tableNames := []string{}
	for tableName := range totalRowCountMap {
		tableNames = append(tableNames, tableName)
	}

	importedRowCountMap := getImportedRowsCount(exportDir, tableNames)

	if len(tableNames) > 0 {
		fmt.Printf("%-30s %-12s %10s %13s %10s\n", "TABLE", "STATUS", "TOTAL_ROWS", "IMPORTED_ROWS", "PERCENTAGE")
	}

	var outputRows []*tableMigStatusOutputRow
	for _, tableName := range tableNames {
		totalRowCount := totalRowCountMap[tableName]
		importedRowCount := importedRowCountMap[tableName]
		if importedRowCount == -1 {
			importedRowCount = 0
		}
		perc := float64(importedRowCount) * 100.0 / float64(totalRowCount)

		var status string
		switch true {
		case importedRowCount == totalRowCount:
			status = "DONE"
		case importedRowCount == 0:
			status = "NOT_STARTED"
		case importedRowCount < totalRowCount:
			status = "MIGRATING"
		}

		row := &tableMigStatusOutputRow{
			tableName:          tableName,
			status:             status,
			totalRowCount:      totalRowCount,
			importedRowCount:   importedRowCount,
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
		fmt.Printf("%-30s %-12s %10d %13d %10.2f\n",
			row.tableName, row.status, row.totalRowCount, row.importedRowCount, row.percentageComplete)
	}
	return nil
}
