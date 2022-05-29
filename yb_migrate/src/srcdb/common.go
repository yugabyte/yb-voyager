package srcdb

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

func getExportedRowCount(tablesMetadata *map[string]*utils.TableProgressMetadata) map[string]int64 {
	exportedRowCount := make(map[string]int64)

	fmt.Println("exported num of rows for each table")
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
	fmt.Printf("| %30s | %30s |\n", "Table", "Row Count")
	sortedKeys := utils.GetSortedKeys(tablesMetadata)
	for _, key := range sortedKeys {
		tableMetadata := (*tablesMetadata)[key]
		fmt.Printf("|%s|\n", strings.Repeat("-", 65))

		targetTableName := strings.TrimSuffix(filepath.Base(tableMetadata.FinalFilePath), "_data.sql")
		actualRowCount := tableMetadata.CountLiveRows
		exportedRowCount[targetTableName] = actualRowCount
		fmt.Printf("| %30s | %30d |\n", key, actualRowCount)
	}
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))

	return exportedRowCount
}
