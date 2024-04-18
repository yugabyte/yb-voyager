package tgtdb

import (
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"golang.org/x/exp/slices"
)

type AttributeNameRegistry struct {
	dbType                  string
	tdb                     TargetDB
	tconf                   *TargetConf
	attrNames               sync.Map
	bestMatchingColumnCache sync.Map
}

func NewAttributeNameRegistry(tdb TargetDB, tconf *TargetConf) *AttributeNameRegistry {
	return &AttributeNameRegistry{
		dbType:                  tconf.TargetDBType,
		tdb:                     tdb,
		tconf:                   tconf,
		attrNames:               sync.Map{}, //sqlname.NameTuple.ForKey() -> []string
		bestMatchingColumnCache: sync.Map{}, //sqlname.NameTuple.ForKey() -> sync.Map{}
	}
}

func (reg *AttributeNameRegistry) QuoteAttributeName(tableNameTup sqlname.NameTuple, columnName string) (string, error) {
	var err error
	anyArr, ok := reg.attrNames.Load(tableNameTup.ForKey())
	var targetColumns []string
	if anyArr != nil {
		targetColumns = anyArr.([]string)
	}
	if !ok {
		targetColumns, err = reg.tdb.GetListOfTableAttributes(tableNameTup)
		log.Infof("columns of table %s in target db: %v", tableNameTup, targetColumns)
		if err != nil {
			return "", fmt.Errorf("get list of table attributes: %w", err)
		}
		reg.attrNames.Store(tableNameTup.ForKey(), targetColumns)
	}
	return reg.withCacheFindBestMatchingColumnName(columnName, targetColumns, tableNameTup)
}

func (reg *AttributeNameRegistry) withCacheFindBestMatchingColumnName(columnName string, targetColumns []string, tableNameTup sqlname.NameTuple) (string, error) {
	anyMap, ok := reg.bestMatchingColumnCache.Load(tableNameTup.ForKey())
	bestMatchingColumnMapForTuple := &sync.Map{}
	if anyMap != nil {
		bestMatchingColumnMapForTuple = anyMap.(*sync.Map)
	}
	if !ok {
		reg.bestMatchingColumnCache.Store(tableNameTup.ForKey(), bestMatchingColumnMapForTuple)
	}
	bestMatchColumn, foundMatch := bestMatchingColumnMapForTuple.Load(columnName)
	if foundMatch {
		return bestMatchColumn.(string), nil
	}
	c, err := reg.findBestMatchingColumnName(columnName, targetColumns)
	if err != nil {
		return "", fmt.Errorf("find best matching column name for %q in table %s: %w", columnName, tableNameTup, err)
	}
	bestMatchingColumnMapForTuple.Store(columnName, fmt.Sprintf("%q", c))
	reg.bestMatchingColumnCache.Store(tableNameTup.ForKey(), bestMatchingColumnMapForTuple)
	return fmt.Sprintf("%q", c), nil

}

func (reg *AttributeNameRegistry) QuoteAttributeNames(tableNameTup sqlname.NameTuple, columns []string) ([]string, error) {
	result := make([]string, len(columns))

	for i, colName := range columns {
		quotedColName, err := reg.QuoteAttributeName(tableNameTup, colName)
		if err != nil {
			return nil, fmt.Errorf("quote attribute name for %q in table %s: %w", colName, tableNameTup, err)
		}
		result[i] = quotedColName
	}
	log.Infof("columns of table %s after quoting: %v", tableNameTup, result)
	return result, nil
}

func (reg *AttributeNameRegistry) findBestMatchingColumnName(colName string, targetColumns []string) (string, error) {
	if colName[0] == '"' && colName[len(colName)-1] == '"' {
		colName = colName[1 : len(colName)-1]
	}
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
