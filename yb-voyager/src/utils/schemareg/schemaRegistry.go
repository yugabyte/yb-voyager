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
package schemareg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goerrors "github.com/go-errors/errors"

	"github.com/samber/lo"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type ColumnSchema struct {
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Parameters map[string]string `json:"parameters"`
	// Not decoding the rest of the fields for now.
}

type Column struct {
	Name   string       `json:"name"`
	Index  int64        `json:"index"`
	Schema ColumnSchema `json:"schema"`
}

type TableSchema struct {
	Columns []Column `json:"columns"`
}

func (ts *TableSchema) getColumnType(columnName string, getSourceDatatypeIfRequired bool) (string, *ColumnSchema, error) {
	for _, colSchema := range ts.Columns {
		if colSchema.Name == columnName { //TODO: store the column names in map for faster lookup
			if dbColType, ok := colSchema.Schema.Parameters["__debezium.source.column.type"]; ok && getSourceDatatypeIfRequired &&
				(strings.Contains(dbColType, "DATE") || strings.Contains(dbColType, "INTERVAL")) {
				//in case oracle import for DATE and INTERVAL types, need to use the original type from source
				return dbColType, &colSchema.Schema, nil
			} else if colSchema.Schema.Name != "" { // in case Kafka speicfic type which start with io.debezium...
				return colSchema.Schema.Name, &colSchema.Schema, nil
			} else {
				return colSchema.Schema.Type, &colSchema.Schema, nil // in case of Primitive types e.g. BYTES/STRING..
			}

		}
	}
	return "", nil, goerrors.Errorf("Column %s not found in table schema %v", columnName, ts)
}

//===========================================================

type SchemaRegistry struct {
	exportDir         string
	exporterRole      string
	importerRole      string
	tableList         []sqlname.NameTuple
	TableNameToSchema *utils.StructMap[sqlname.NameTuple, *TableSchema]
}

// Initialize schema registry with the given table list
func NewSchemaRegistry(tableList []sqlname.NameTuple, exportDir string, exporterRole string, importerRole string) *SchemaRegistry {
	return &SchemaRegistry{
		exportDir:         exportDir,
		exporterRole:      exporterRole,
		importerRole:      importerRole,
		tableList:         tableList,
		TableNameToSchema: utils.NewStructMap[sqlname.NameTuple, *TableSchema](),
	}
}

func (sreg *SchemaRegistry) GetColumnTypes(tableNameTup sqlname.NameTuple, columnNames []string, getSourceDatatypes bool) ([]string, []*ColumnSchema, error) {
	tableSchema, _ := sreg.TableNameToSchema.Get(tableNameTup)
	if tableSchema == nil {
		return nil, nil, goerrors.Errorf("table %s not found in schema registry", tableNameTup)
	}
	columnTypes := make([]string, len(columnNames))
	columnSchemas := make([]*ColumnSchema, len(columnNames))
	for idx, columnName := range columnNames {
		var err error
		columnTypes[idx], columnSchemas[idx], err = tableSchema.getColumnType(columnName, getSourceDatatypes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get column type for table %s, column %s: %w", tableNameTup, columnName, err)
		}
	}
	return columnTypes, columnSchemas, nil
}

func (sreg *SchemaRegistry) GetColumnType(tableNameTup sqlname.NameTuple, columnName string, getSourceDatatype bool) (string, *ColumnSchema, error) {
	var tableSchema *TableSchema
	var found bool
	var err error
	tableSchema, _ = sreg.TableNameToSchema.Get(tableNameTup)
	if tableSchema == nil {
		// reinit
		sreg.TableNameToSchema.Clear()

		err = sreg.Init()
		if err != nil {
			return "", nil, goerrors.Errorf("re-init of registry : %v", err)
		}
		tableSchema, found = sreg.TableNameToSchema.Get(tableNameTup)
		if !found || tableSchema == nil {
			return "", nil, fmt.Errorf("table %s not found in schema registry:%w", tableNameTup, err)
		}
	}
	return tableSchema.getColumnType(columnName, getSourceDatatype)
}

func (sreg *SchemaRegistry) Init() error {
	schemaDir := filepath.Join(sreg.exportDir, "data", "schemas", sreg.exporterRole)
	schemaFiles, err := os.ReadDir(schemaDir)
	if err != nil {
		return fmt.Errorf("failed to read schema dir %s: %w", schemaDir, err)
	}
	for _, schemaFile := range schemaFiles {
		if strings.HasSuffix(schemaFile.Name(), ".tmp") {
			continue
		}
		schemaFilePath := filepath.Join(schemaDir, schemaFile.Name())
		schemaFile, err := os.Open(schemaFilePath)
		if err != nil {
			return fmt.Errorf("failed to open table schema file %s: %w", schemaFilePath, err)
		}
		var tableSchema TableSchema
		err = json.NewDecoder(schemaFile).Decode(&tableSchema)
		if err != nil {
			return fmt.Errorf("failed to decode table schema file %s: %w", schemaFilePath, err)
		}
		tableName := strings.TrimSuffix(filepath.Base(schemaFile.Name()), "_schema.json")
		//Using this function here as schema registry is used in import data and to handle the case where the table is not present in the target
		//using this function and checking if this is actually excluded in list
		table, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(tableName)
		if err != nil {
			return goerrors.Errorf("lookup %s from name registry: %v", tableName, err)
		}
		if !lo.ContainsBy(sreg.tableList, func(t sqlname.NameTuple) bool {
			return t.ForKey() == table.ForKey()
		}) {
			//skip the table if it is not present in the table list
			continue
		} else if !table.TargetTableAvailable() && sreg.importerRole == namereg.TARGET_DB_IMPORTER_ROLE {
			//Table is in table list but target not found during lookup
			// only possible in offline migration where table is exported, but not present in in import-data table list.
			return goerrors.Errorf("table %s is not present in the target database", table)
		}
		sreg.TableNameToSchema.Put(table, &tableSchema)
		schemaFile.Close()
	}
	return nil
}
