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
package dbzm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

type DbzSchema struct {
	Type string `json:"type"`
	Name string `json:"name"`
	// Not decoding the rest of the fields for now.
}

type Schema struct {
	ColName      string    `json:"name"`
	Index        int64     `json:"index"`
	ColDbzSchema DbzSchema `json:"schema"`
}

type TableSchema struct {
	Columns []Schema `json:"columns"`
}

func GetTableSchema(tableName string, exportDir string) (TableSchema, error) {
	schemaFileName := fmt.Sprintf("%s_schema.json", tableName)
	schemaFilePath := filepath.Join(exportDir, "data", "schemas", schemaFileName)
	schemaFile, err := os.Open(schemaFilePath)
	if err != nil {
		return TableSchema{}, fmt.Errorf("failed to open schema file %s: %v", schemaFilePath, err)
	}
	defer schemaFile.Close()
	var tableSchema TableSchema
	err = json.NewDecoder(schemaFile).Decode(&tableSchema)
	if err != nil {
		return TableSchema{}, fmt.Errorf("failed to decode schema file %s: %v", schemaFilePath, err)
	}
	return tableSchema, nil

}

func (ts *TableSchema) GetColumnSchema(columnName string) Schema {
	for _, colSchema := range ts.Columns {
		if colSchema.ColName == columnName {
			return colSchema
		}
	}
	return Schema{}
}

func (dbzs *DbzSchema) GetConverterFn(valueConverterSuite map[string]tgtdb.ConverterFn) tgtdb.ConverterFn {
	logicalType := dbzs.Name
	schemaType := dbzs.Type
	if valueConverterSuite[logicalType] != nil {
		return valueConverterSuite[logicalType]
	} else if valueConverterSuite[schemaType] != nil {
		return valueConverterSuite[schemaType]
	}
	return func(v string) (string, error) { return v, nil }
}

//===========================================================

type SchemaRegistry struct {
	exportDir string
}

func NewSchemaRegistry(exportDir string) *SchemaRegistry {
	return &SchemaRegistry{exportDir: exportDir}
}

func (sreg *SchemaRegistry) GetColumnSchemas(tableName string, columnNames []string) ([]Schema, error) {
	tableSchema, err := GetTableSchema(tableName, sreg.exportDir)
	if err != nil {
		return nil, err
	}
	columnToSchema := make([]Schema, len(columnNames))
	for idx, columnName := range columnNames {
		colSchema := tableSchema.GetColumnSchema(columnName)
		columnToSchema[idx] = colSchema
	}
	return columnToSchema, nil
}

func (sreg *SchemaRegistry) GetColumnSchema(tableName, columnName string) (Schema, error) {
	tableSchema, err := GetTableSchema(tableName, sreg.exportDir)
	if err != nil {
		return Schema{}, err
	}
	for _, colSchema := range tableSchema.Columns {
		if colSchema.ColName == columnName {
			return colSchema, nil
		}
	}
	return Schema{}, nil
}
