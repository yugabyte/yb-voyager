package srcdb

import (
	"path/filepath"
	"strings"

	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

func getExportedRowCount(tablesMetadata *map[string]*utils.TableProgressMetadata) map[string]int64 {
	exportedRowCount := make(map[string]int64)

	sortedKeys := utils.GetSortedKeys(tablesMetadata)
	for _, key := range sortedKeys {
		tableMetadata := (*tablesMetadata)[key]
		targetTableName := strings.TrimSuffix(filepath.Base(tableMetadata.FinalFilePath), "_data.sql")
		exportedRowCount[targetTableName] = tableMetadata.CountLiveRows

	}
	return exportedRowCount
}
