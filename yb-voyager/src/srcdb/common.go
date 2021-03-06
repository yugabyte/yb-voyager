package srcdb

import (
	"path/filepath"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func getExportedRowCount(tablesMetadata map[string]*utils.TableProgressMetadata) map[string]int64 {
	exportedRowCount := make(map[string]int64)
	for key := range tablesMetadata {
		tableMetadata := tablesMetadata[key]
		targetTableName := strings.TrimSuffix(filepath.Base(tableMetadata.FinalFilePath), "_data.sql")
		exportedRowCount[targetTableName] = tableMetadata.CountLiveRows

	}
	return exportedRowCount
}
