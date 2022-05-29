package srcdb

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

func saveExportedRowCount(exportDir string, tablesMetadata *map[string]*utils.TableProgressMetadata) {
	filePath := exportDir + "/metainfo/flags/tablesrowcount"
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	fmt.Println("exported num of rows for each table")
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
	fmt.Printf("| %30s | %30s |\n", "Table", "Row Count")
	sortedKeys := utils.GetSortedKeys(tablesMetadata)
	for _, key := range sortedKeys {
		tableMetadata := (*tablesMetadata)[key]
		fmt.Printf("|%s|\n", strings.Repeat("-", 65))

		targetTableName := strings.TrimSuffix(filepath.Base(tableMetadata.FinalFilePath), "_data.sql")
		actualRowCount := tableMetadata.CountLiveRows
		line := targetTableName + "," + strconv.FormatInt(actualRowCount, 10) + "\n"
		file.WriteString(line)
		fmt.Printf("| %30s | %30d |\n", key, actualRowCount)
	}
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
}
