package tgtdb

import (
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"golang.org/x/exp/slices"
)

type AttributeNameRegistry struct {
	dbType    string
	tdb       TargetDB
	tconf     *TargetConf
	attrNames *utils.StructMap[sqlname.NameTuple, []string]
	mu        sync.Mutex
}

func NewAttributeNameRegistry(tdb TargetDB, tconf *TargetConf) *AttributeNameRegistry {
	return &AttributeNameRegistry{
		dbType:    tconf.TargetDBType,
		tdb:       tdb,
		tconf:     tconf,
		attrNames: utils.NewStructMap[sqlname.NameTuple, []string](),
	}
}

func (reg *AttributeNameRegistry) QuoteIdentifier(tableNameTup sqlname.NameTuple, columnName string) string {
	var err error
	// qualifiedTableName := fmt.Sprintf("%s.%s", schemaName, tableName)
	targetColumns, ok := reg.attrNames.Get(tableNameTup)
	if !ok {
		reg.mu.Lock()
		// try again in case it's now available
		targetColumns, ok = reg.attrNames.Get(tableNameTup)
		if !ok {
			targetColumns, err = reg.tdb.GetListOfTableAttributes(tableNameTup)
			log.Infof("columns of table %s in target db: %v", tableNameTup, targetColumns)
			if err != nil {
				utils.ErrExit("get list of table attributes: %w", err)
			}
			reg.attrNames.Put(tableNameTup, targetColumns)
		}

		reg.mu.Unlock()
	}
	c, err := reg.findBestMatchingColumnName(columnName, targetColumns)
	if err != nil {
		utils.ErrExit("find best matching column name for %q in table %s: %w", columnName, tableNameTup, err)
	}
	return fmt.Sprintf("%q", c)
}

func (reg *AttributeNameRegistry) IfRequiredQuoteColumnNames(tableNameTup sqlname.NameTuple, columns []string) ([]string, error) {
	result := make([]string, len(columns))
	// var schemaName string
	// schemaName, tableName = reg.splitMaybeQualifiedTableName(tableName)
	// targetColumns, err := reg.tdb.GetListOfTableAttributes(schemaName, tableName)
	// if err != nil {
	// 	return nil, fmt.Errorf("get list of table attributes: %w", err)
	// }
	// log.Infof("columns of table %s.%s in target db: %v", schemaName, tableName, targetColumns)
	// qualifiedTableName := fmt.Sprintf("%s.%s", schemaName, tableName)
	// reg.attrNames[qualifiedTableName] = targetColumns

	for i, colName := range columns {
		// if colName[0] == '"' && colName[len(colName)-1] == '"' {
		// 	colName = colName[1 : len(colName)-1]
		// }
		// matchCol, err := reg.findBestMatchingColumnName(colName, targetColumns)
		// if err != nil {
		// 	return nil, fmt.Errorf("find best matching column name for %q in table %s.%s: %w",
		// 		colName, schemaName, tableName, err)
		// }
		quotedColName := reg.QuoteIdentifier(tableNameTup, colName)
		result[i] = quotedColName
	}
	log.Infof("columns of table %s after quoting: %v", tableNameTup, result)
	return result, nil
}

func (reg *AttributeNameRegistry) splitMaybeQualifiedTableName(tableName string) (string, string) {
	if strings.Contains(tableName, ".") {
		parts := strings.Split(tableName, ".")
		return parts[0], parts[1]
	}
	return reg.tconf.Schema, tableName
}

func (reg *AttributeNameRegistry) findBestMatchingColumnName(colName string, targetColumns []string) (string, error) {
	if slices.Contains(targetColumns, colName) { // Exact match.
		return colName, nil
	}
	// Case insensitive match.
	candidates := []string{}
	for _, targetCol := range targetColumns {
		if strings.EqualFold(targetCol, colName) {
			candidates = append(candidates, targetCol)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		if reg.dbType == POSTGRESQL || reg.dbType == YUGABYTEDB {
			if slices.Contains(candidates, strings.ToLower(colName)) {
				return strings.ToLower(colName), nil
			}
		} else if reg.dbType == ORACLE {
			if slices.Contains(candidates, strings.ToUpper(colName)) {
				return strings.ToUpper(colName), nil
			}
		}
		return "", fmt.Errorf("ambiguous column name %q in target table: found column names: %s",
			colName, strings.Join(candidates, ", "))
	}
	return "", fmt.Errorf("column %q not found amongst table columns %v", colName, targetColumns)
}
