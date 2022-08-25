package importdata

import "fmt"

type TableID struct {
	DatabaseName, SchemaName, TableName string
}

func NewTableID(dbName, schemaName, tableName string) *TableID {
	if schemaName == "" {
		schemaName = "public"
	}
	return &TableID{DatabaseName: dbName, SchemaName: schemaName, TableName: tableName}
}

func (tableID *TableID) String() string {
	return fmt.Sprintf("%s:%s:%s", tableID.DatabaseName, tableID.SchemaName, tableID.TableName)
}

func (tableID *TableID) QualifiedName() string {
	return fmt.Sprintf("%s.%s", tableID.SchemaName, tableID.TableName)
}
