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

	log "github.com/sirupsen/logrus"
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

func (ts *TableSchema) getColumnType(columnName string) *string {
	for _, colSchema := range ts.Columns {
		if colSchema.Name == columnName {
			if colSchema.Schema.Name != "" {
				return &colSchema.Schema.Name
			} else {
				return &colSchema.Schema.Type
			}

		}
	}
	log.Warnf("Column %s not found in table schema %v", columnName, ts)
	return nil
}

//===========================================================

type SchemaRegistry struct {
	exportDir     string
	tableToSchema map[string]*TableSchema
}

func NewSchemaRegistry(exportDir string) *SchemaRegistry {
	return &SchemaRegistry{
		exportDir:     exportDir,
		tableToSchema: make(map[string]*TableSchema),
	}
}

func (sreg *SchemaRegistry) GetColumnTypes(tableName string, columnNames []string) ([]string, error) {
	tableSchema := sreg.tableToSchema[tableName]
	columnToTypes := make([]string, len(columnNames))
	for idx, columnName := range columnNames {
		columnToTypes[idx] = *tableSchema.getColumnType(columnName)
	}
	return columnToTypes, nil
}

func (sreg *SchemaRegistry) GetColumnType(tableName, columnName string) (*string, error) {
	tableSchema := sreg.tableToSchema[tableName]
	return tableSchema.getColumnType(columnName), nil
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
			return fmt.Errorf("failed to open ColumnSchema file %s: %w", schemaFilePath, err)
		}
		defer schemaFile.Close()
		var tableSchema TableSchema
		err = json.NewDecoder(schemaFile).Decode(&tableSchema)
		if err != nil {
			return fmt.Errorf("failed to decode ColumnSchema file %s: %w", schemaFilePath, err)
		}
		table := strings.TrimSuffix(filepath.Base(schemaFile.Name()), "_schema.json")
		sreg.tableToSchema[table] = &tableSchema
	}
	return nil
}
