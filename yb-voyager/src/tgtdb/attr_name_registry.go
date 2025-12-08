package tgtdb

import (
	"fmt"
	"strings"
	"sync"

	goerrors "github.com/go-errors/errors"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"golang.org/x/exp/slices"
)

type AttributeNameRegistry struct {
	dbType      string
	tdb         TargetDB
	tconf       *TargetConf
	attrNames   *utils.StructMap[sqlname.NameTuple, []string]
	resultCache map[string]map[string]string
	mu          sync.Mutex
}

func NewAttributeNameRegistry(tdb TargetDB, tconf *TargetConf) *AttributeNameRegistry {
	return &AttributeNameRegistry{
		dbType:      tconf.TargetDBType,
		tdb:         tdb,
		tconf:       tconf,
		attrNames:   utils.NewStructMap[sqlname.NameTuple, []string](),
		resultCache: make(map[string]map[string]string),
	}
}

func (reg *AttributeNameRegistry) lookupCachedQuotedAttributeName(tableNameTup sqlname.NameTuple, columnName string) (string, bool) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	resultMap, ok := reg.resultCache[tableNameTup.ForKey()]
	if !ok {
		return "", false
	}
	result, ok := resultMap[columnName]
	return result, ok
}

func (reg *AttributeNameRegistry) cacheQuotedAttributeName(tableNameTup sqlname.NameTuple, columnName, result string) {
	var resultMap map[string]string

	reg.mu.Lock()
	defer reg.mu.Unlock()

	resultMap, ok := reg.resultCache[tableNameTup.ForKey()]
	if !ok {
		resultMap = make(map[string]string)
		reg.resultCache[tableNameTup.ForKey()] = resultMap
	}
	resultMap[columnName] = result
}

func (reg *AttributeNameRegistry) QuoteAttributeName(tableNameTup sqlname.NameTuple, columnName string) (string, error) {
	var err error

	result, ok := reg.lookupCachedQuotedAttributeName(tableNameTup, columnName)
	if ok {
		return result, nil
	}
	var targetColumns []string
	reg.mu.Lock()
	targetColumns, ok = reg.attrNames.Get(tableNameTup)
	if !ok {
		targetColumns, err = reg.tdb.GetListOfTableAttributes(tableNameTup)
		log.Infof("columns of table %s in target db: %v", tableNameTup, targetColumns)
		if err != nil {
			reg.mu.Unlock()
			return "", fmt.Errorf("get list of table attributes: %w", err)
		}
		reg.attrNames.Put(tableNameTup, targetColumns)
	}
	reg.mu.Unlock()
	c, err := reg.findBestMatchingColumnName(columnName, targetColumns)
	if err != nil {
		return "", goerrors.Errorf("find best matching column name for %q in table %s: %w", columnName, tableNameTup, err)
	}
	result = fmt.Sprintf("%q", c)
	reg.cacheQuotedAttributeName(tableNameTup, columnName, result)
	return result, nil
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
