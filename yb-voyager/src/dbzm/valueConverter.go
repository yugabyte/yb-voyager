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
	"fmt"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	tgtdbsuite "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/suites"
)

var NULL_STRING string = "NULL"

type ValueConverter interface {
	ConvertRow(tableName string, columnNames []string, row string) (string, error)
	ConvertEvent(ev *tgtdb.Event, table string) error
	GetTableNameToSchema() map[string]map[string]map[string]string //returns table name to schema mapping
}

func NewValueConverter(targetDBType string, exportDir string, tdb tgtdb.TargetDB) (ValueConverter, error) {
	if IsDebeziumForDataExport(exportDir) {
		return NewDebeziumValueConverter(targetDBType, exportDir, tdb)
	} else {
		return &NoOpValueConverter{}, nil
	}
}

//============================================================================

type NoOpValueConverter struct{}

func (nvc *NoOpValueConverter) ConvertRow(tableName string, columnNames []string, row string) (string, error) {
	return row, nil
}

func (nvc *NoOpValueConverter) ConvertEvent(ev *tgtdb.Event, table string) error {
	return nil
}

func (nvc *NoOpValueConverter) GetTableNameToSchema() map[string]map[string]map[string]string {
	return nil
}

//============================================================================

type DebeziumValueConverter struct {
	schemaRegistry      *SchemaRegistry
	valueConverterSuite map[string]tgtdbsuite.ConverterFn
	converterFnCache    map[string][]tgtdbsuite.ConverterFn //stores table name to converter functions for each column
	targetDBType        string
}

func NewDebeziumValueConverter(targetDBType string, exportDir string, tdb tgtdb.TargetDB) (*DebeziumValueConverter, error) {
	schemaRegistry := NewSchemaRegistry(exportDir)
	err := schemaRegistry.Init()
	if err != nil {
		return nil, fmt.Errorf("initializing schema registry: %w", err)
	}
	tdbValueConverterSuite := tdb.GetDebeziumValueConverterSuite()
	return &DebeziumValueConverter{
		schemaRegistry:      schemaRegistry,
		valueConverterSuite: tdbValueConverterSuite,
		converterFnCache:    map[string][]tgtdbsuite.ConverterFn{},
		targetDBType:        targetDBType,
	}, nil
}

func (conv *DebeziumValueConverter) ConvertRow(tableName string, columnNames []string, row string) (string, error) {
	converterFns, err := conv.getConverterFns(tableName, columnNames)
	if err != nil {
		return "", fmt.Errorf("fetching converter functions: %w", err)
	}
	columnValues := strings.Split(row, "\t")
	for i, columnValue := range columnValues {
		if columnValue == "\\N" || converterFns[i] == nil { // TODO: make "\\N" condition Target specific tdb.NullString()
			continue
		}
		transformedValue, err := converterFns[i](columnValue, false)
		if err != nil {
			return "", fmt.Errorf("converting value for %s, column %d and value %s : %w", tableName, i, columnValue, err)
		}
		columnValues[i] = transformedValue
	}
	return strings.Join(columnValues, "\t"), nil
}

func (conv *DebeziumValueConverter) getConverterFns(tableName string, columnNames []string) ([]tgtdbsuite.ConverterFn, error) {
	result := conv.converterFnCache[tableName]
	if result == nil {
		colTypes, err := conv.schemaRegistry.GetColumnTypes(conv.targetDBType, tableName, columnNames)
		if err != nil {
			return nil, fmt.Errorf("get types of columns of table %s: %w", tableName, err)
		}
		result = make([]tgtdbsuite.ConverterFn, len(columnNames))
		for i, colType := range colTypes {
			result[i] = conv.valueConverterSuite[colType]
		}
		conv.converterFnCache[tableName] = result
	}
	return result, nil
}

func (conv *DebeziumValueConverter) ConvertEvent(ev *tgtdb.Event, table string) error {
	err := conv.convertMap(table, ev.Key)
	if err != nil {
		return fmt.Errorf("convert event key: %w", err)
	}
	err = conv.convertMap(table, ev.Fields)
	if err != nil {
		return fmt.Errorf("convert event fields: %w", err)
	}
	return nil
}

func (conv *DebeziumValueConverter) convertMap(tableName string, m map[string]*string) error {
	for column, value := range m {
		if value == nil {
			m[column] = &NULL_STRING
			continue
		}
		columnValue := *value
		colType, err := conv.schemaRegistry.GetColumnType(conv.targetDBType, tableName, column)
		if err != nil {
			return fmt.Errorf("fetch column schema: %w", err)
		}
		converterFn := conv.valueConverterSuite[colType]
		if converterFn != nil {
			columnValue, err = converterFn(columnValue, true)
			if err != nil {
				return fmt.Errorf("error while converting %s.%s of type %s in event: %w", tableName, column, colType, err) // TODO - add event id in log msg
			}
		}
		m[column] = &columnValue
	}
	return nil
}

func (conv *DebeziumValueConverter) GetTableNameToSchema() map[string]map[string]map[string]string {
	//need to create explicit map with required details only as can't use TableSchema directly in import area because of cyclic dependency
	var tableToSchema = make(map[string]map[string]map[string]string)
	// tableToSchema {<table>: {<column>:<parameters>}}
	for table, col := range conv.schemaRegistry.tableNameToSchema {
		tableToSchema[table] = make(map[string]map[string]string)
		for _, col := range col.Columns {
			tableToSchema[table][col.Name] = col.Schema.Parameters
		}
	}
	return tableToSchema
}
