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
	"strings"

)

type ColumnSchema struct {
	Type string `json:"type"`
	Name string `json:"name"`
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

func (ts *TableSchema) getColumnType(columnName string) (string, error) {
	for _, colSchema := range ts.Columns {
		if colSchema.Name == columnName {
			if colSchema.Schema.Name != "" { // in case Kafka speicfic type which start with io.debezium...
				return colSchema.Schema.Name, nil
			} else {
				return colSchema.Schema.Type, nil // in case of Primitive types e.g. BYTES/STRING..
			}

		}
	}
	return "", fmt.Errorf("Column %s not found in table schema %v", columnName, ts)
}

//===========================================================

type SchemaRegistry struct {
	exportDir     string
	tableNameToSchema map[string]*TableSchema
}

func NewSchemaRegistry(exportDir string) *SchemaRegistry {
	return &SchemaRegistry{
		exportDir:     exportDir,
		tableNameToSchema: make(map[string]*TableSchema),
	}
}

func (sreg *SchemaRegistry) GetColumnTypes(tableName string, columnNames []string) ([]string, error) {
	tableSchema := sreg.tableNameToSchema[tableName]
	if tableSchema == nil {
		return nil, fmt.Errorf("table %s not found in schema registry", tableName)
	}
	columnTypes := make([]string, len(columnNames))
	for idx, columnName := range columnNames {
		var err error
		columnTypes[idx], err = tableSchema.getColumnType(columnName)
		if err != nil {
			return nil, fmt.Errorf("failed to get column type for table %s, column %s: %w", tableName, columnName, err)
		}
	}
	return columnTypes, nil
}

func (sreg *SchemaRegistry) GetColumnType(tableName, columnName string) (string, error) {
	tableSchema := sreg.tableNameToSchema[tableName]
	if tableSchema == nil {
		return "", fmt.Errorf("table %s not found in schema registry", tableName)
	}
	return tableSchema.getColumnType(columnName)
}

func (sreg *SchemaRegistry) Init() error {
	schemaDir := filepath.Join(sreg.exportDir, "data", "schemas")
	schemaFiles, err := os.ReadDir(schemaDir)
	if err != nil {
		return fmt.Errorf("failed to read schema dir %s: %w", schemaDir, err)
	}
	for _, schemaFile := range schemaFiles {
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
		table := strings.TrimSuffix(filepath.Base(schemaFile.Name()), "_schema.json")
		sreg.tableNameToSchema[table] = &tableSchema
		schemaFile.Close()
	}
	return nil
}
