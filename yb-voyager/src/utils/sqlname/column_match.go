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
package sqlname

import (
	"fmt"
	"slices"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

// FindBestMatchingColumnName finds the column of tableColumns that colName refers to: an exact match
// first, then a case-insensitive one. When several columns differ only in case, the one in the
// database's default case (lower for PostgreSQL/YugabyteDB, upper for Oracle) wins; otherwise the
// name is ambiguous.
func FindBestMatchingColumnName(dbType, colName string, tableColumns []string) (string, error) {
	if len(colName) >= 2 && colName[0] == '"' && colName[len(colName)-1] == '"' {
		colName = colName[1 : len(colName)-1]
	}
	if slices.Contains(tableColumns, colName) {
		return colName, nil
	}
	var candidates []string
	for _, column := range tableColumns {
		if strings.EqualFold(column, colName) {
			candidates = append(candidates, column)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		switch dbType {
		case constants.POSTGRESQL, constants.YUGABYTEDB:
			if slices.Contains(candidates, strings.ToLower(colName)) {
				return strings.ToLower(colName), nil
			}
		case constants.ORACLE:
			if slices.Contains(candidates, strings.ToUpper(colName)) {
				return strings.ToUpper(colName), nil
			}
		}
		return "", &ErrAmbiguousColumnName{colName: colName, candidates: candidates}
	}
	return "", &ErrColumnNameNotFound{colName: colName, tableColumns: tableColumns}
}

type ErrAmbiguousColumnName struct {
	colName    string
	candidates []string
}

func (e *ErrAmbiguousColumnName) Error() string {
	return fmt.Sprintf("ambiguous column name %q in table: found column names: %s", e.colName, strings.Join(e.candidates, ", "))
}

type ErrColumnNameNotFound struct {
	colName      string
	tableColumns []string
}

func (e *ErrColumnNameNotFound) Error() string {
	return fmt.Sprintf("column name %q not found amongst table columns %v", e.colName, e.tableColumns)
}
