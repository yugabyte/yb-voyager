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
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type ValueConverter interface {
	ConvertRow(tableName string, columnNames []string, row string) (string, error)
	ConvertEvent(ev *tgtdb.Event, table string, formatIfRequired bool) error
}

func NewValueConverter(exportDir string, tdb tgtdb.TargetDB) (ValueConverter, error) {
	if IsDebeziumForDataExport(exportDir) {
		return NewDebeziumValueConverter(exportDir, tdb)
	} else {
		return &NoOpValueConverter{}, nil
	}
}

//============================================================================

type NoOpValueConverter struct{}

func (nvc *NoOpValueConverter) ConvertRow(tableName string, columnNames []string, row string) (string, error) {
	return row, nil
}

func (nvc *NoOpValueConverter) ConvertEvent(ev *tgtdb.Event, table string, formatIfRequired bool) error {
	return nil
}

//============================================================================

type DebeziumValueConverter struct {
	schemaRegistry      *SchemaRegistry
	valueConverterSuite map[string]tgtdb.ConverterFn
	converterFnCache    map[string][]tgtdb.ConverterFn //stores table name to converter functions for each column
}

func NewDebeziumValueConverter(exportDir string, tdb tgtdb.TargetDB) (*DebeziumValueConverter, error) {
	schemaRegistry := NewSchemaRegistry(exportDir)
	err := schemaRegistry.Init()
	if err != nil {
		return nil, fmt.Errorf("initializing schema registry: %w", err)
	}
	tdbValueConverterSuite := tdb.GetDebeziumValueConverterSuite()

	return &DebeziumValueConverter{
		schemaRegistry:      schemaRegistry,
		valueConverterSuite: tdbValueConverterSuite,
		converterFnCache:    map[string][]tgtdb.ConverterFn{},
	}, nil
}

func (conv *DebeziumValueConverter) ConvertRow(tableName string, columnNames []string, row string) (string, error) {
	converterFns, err := conv.getConverterFns(tableName, columnNames)
	if err != nil {
		return "", fmt.Errorf("fetching converter functions: %w", err)
	}
	columnValues, err := csv.NewReader(strings.NewReader(row)).Read()
	if err != nil {
		return "", fmt.Errorf("reading row: %w", err)
	}
	for i, columnValue := range columnValues {
		if columnValue == utils.YB_VOYAGER_NULL_STRING || converterFns[i] == nil { // TODO: make nullstring condition Target specific tdb.NullString()
			continue
		}
		transformedValue, err := converterFns[i](columnValue, false)
		if err != nil {
			return "", fmt.Errorf("converting value for %s, column %d and value %s : %w", tableName, i, columnValue, err)
		}
		columnValues[i] = transformedValue
	}
	var buffer bytes.Buffer
	csvWriter := csv.NewWriter(&buffer)
	csvWriter.Write(columnValues)
	csvWriter.Flush()
	transformedRow := buffer.String()
	return transformedRow, nil
}

func (conv *DebeziumValueConverter) getConverterFns(tableName string, columnNames []string) ([]tgtdb.ConverterFn, error) {
	result := conv.converterFnCache[tableName]
	if result == nil {
		colTypes, err := conv.schemaRegistry.GetColumnTypes(tableName, columnNames)
		if err != nil {
			return nil, fmt.Errorf("get types of columns of table %s: %w", tableName, err)
		}
		result = make([]tgtdb.ConverterFn, len(columnNames))
		for i, colType := range colTypes {
			result[i] = conv.valueConverterSuite[colType]
		}
		conv.converterFnCache[tableName] = result
	}
	return result, nil
}

func (conv *DebeziumValueConverter) ConvertEvent(ev *tgtdb.Event, table string, formatIfRequired bool) error {
	err := conv.convertMap(table, ev.Key, formatIfRequired)
	if err != nil {
		return fmt.Errorf("convert event key: %w", err)
	}
	err = conv.convertMap(table, ev.Fields, formatIfRequired)
	if err != nil {
		return fmt.Errorf("convert event fields: %w", err)
	}
	return nil
}

func (conv *DebeziumValueConverter) convertMap(tableName string, m map[string]*string, formatIfRequired bool) error {
	for column, value := range m {
		if value == nil {
			continue
		}
		columnValue := *value
		colType, err := conv.schemaRegistry.GetColumnType(tableName, column)
		if err != nil {
			return fmt.Errorf("fetch column schema: %w", err)
		}
		converterFn := conv.valueConverterSuite[colType]
		if converterFn != nil {
			columnValue, err = converterFn(columnValue, formatIfRequired)
			if err != nil {
				return fmt.Errorf("error while converting %s.%s of type %s in event: %w", tableName, column, colType, err) // TODO - add event id in log msg
			}
		}
		m[column] = &columnValue
	}
	return nil
}
