package tgtdb

import (
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type AttributeNameRegistry struct {
	dbType    string
	tdb       TargetDB
	attrNames map[string][]string
}

func NewAttributeNameRegistry(dbType string, tdb TargetDB) *AttributeNameRegistry {
	return &AttributeNameRegistry{
		dbType:    dbType,
		tdb:       tdb,
		attrNames: make(map[string][]string),
	}
}

func (reg *AttributeNameRegistry) QuoteIdentifier(schemaName, tableName, columnName string) string {
	var err error
	qualifiedTableName := fmt.Sprintf("%s.%s", schemaName, tableName)
	targetColumns, ok := reg.attrNames[qualifiedTableName]
	if !ok {
		targetColumns, err = reg.tdb.GetListOfTableAttributes(schemaName, tableName)
		if err != nil {
			utils.ErrExit("get list of table attributes: %w", err)
		}
		reg.attrNames[qualifiedTableName] = targetColumns
	}
	c, err := findBestMatchingColumnName(reg.dbType, columnName, targetColumns)
	if err != nil {
		utils.ErrExit("find best matching column name for %q in table %s.%s: %w", columnName, schemaName, tableName, err)
	}
	return fmt.Sprintf("%q", c)
}
