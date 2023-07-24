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
)

type ValueConverter interface {
	ConvertRow(tableName string, columnNames []string, row string) (string, error)
	ConvertEvent(ev *tgtdb.Event, table string) error
}

func NewValueConverter(exportDir string, tdb tgtdb.TargetDB, snapshotMode bool) (ValueConverter, error) {
	if IsDebeziumForDataExport(exportDir) {
		return NewDebeziumValueConverter(exportDir, tdb, snapshotMode)
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

//============================================================================

type DebeziumValueConverter struct {
	schemaRegistry      *SchemaRegistry
	valueConverterSuite map[string]tgtdb.ConverterFn
	converterFnCache    map[string][]tgtdb.ConverterFn
	snapshotMode        bool
}

func NewDebeziumValueConverter(exportDir string, tdb tgtdb.TargetDB, snapshotMode bool) (*DebeziumValueConverter, error) {
	schemaRegistry := NewSchemaRegistry(exportDir)
	err := schemaRegistry.Init()
	if err != nil {
		return nil, fmt.Errorf("initializing schema registry: %w", err)
	}
	tdbValueConverterSuite := tdb.GetDebeziumValueConverterSuite(snapshotMode)

	return &DebeziumValueConverter{
		schemaRegistry:      schemaRegistry,
		valueConverterSuite: tdbValueConverterSuite,
		converterFnCache:    map[string][]tgtdb.ConverterFn{},
		snapshotMode:        snapshotMode,
	}, nil
}

func (conv *DebeziumValueConverter) ConvertRow(tableName string, columnNames []string, row string) (string, error) {
	converterFns, err := conv.getConverterFns(tableName, columnNames)
	if err != nil {
		return "", fmt.Errorf("fetching converter functions: %w", err)
	}
	columnValues := strings.Split(row, "\t")
	for i, columnValue := range columnValues {
		if columnValue != "\\N" {
			transformedValue, err := converterFns[i](columnValue, false)
			if err != nil {
				return "", fmt.Errorf("converting value: %w", err)
			}
			columnValues[i] = transformedValue
		} else {
			columnValues[i] = columnValue
		}
	}
	return strings.Join(columnValues, "\t"), nil
}

func (conv *DebeziumValueConverter) getConverterFns(tableName string, columnNames []string) ([]tgtdb.ConverterFn, error) {
	result := conv.converterFnCache[tableName]
	if result == nil {
		colTypes, err := conv.schemaRegistry.GetColumnTypes(tableName, columnNames)
		if err != nil {
			return nil, fmt.Errorf("fetching column schemas: %w", err)
		}
		result = make([]tgtdb.ConverterFn, len(columnNames))
		for i := range columnNames {
			colType := colTypes[i]
			if conv.valueConverterSuite[colType] != nil {
				result[i] = conv.valueConverterSuite[colType]
			} else {
				result[i] = func(v string, formatIfRequired bool) (string, error) { return v, nil }
			}
		}
		conv.converterFnCache[tableName] = result
	}
	return result, nil
}

func (conv *DebeziumValueConverter) ConvertEvent(ev *tgtdb.Event, table string) error {
	err := conv.convertMap(table, ev.Key)
	if err != nil {
		return fmt.Errorf("convert event fields: %w", err)
	}
	err = conv.convertMap(table, ev.Fields)
	if err != nil {
		return fmt.Errorf("convert event keys: %w", err)
	}
	return nil
}

func (conv *DebeziumValueConverter) convertMap(tableName string, m map[string]string) error {
	for column, value := range m {
		if value == "" {
			m[column] = "NULL"
			continue
		}
		columnValue := fmt.Sprintf("%v", value)
		colType, err := conv.schemaRegistry.GetColumnType(tableName, column)
		if err != nil {
			return fmt.Errorf("fetch column schema: %w", err)
		}
		converterFn := conv.valueConverterSuite[*colType]
		if converterFn == nil {
			converterFn = func(v string, formatIfRequired bool) (string, error) { return v, nil }
		}
		columnValue, err = converterFn(columnValue, true)
		if err != nil {
			return fmt.Errorf("error while converting %s.%s of type %s in event: %w", tableName, column, *colType, err) // TODO - add event id in log msg
		}
		m[column] = columnValue
	}
	return nil
}
