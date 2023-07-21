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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type ValueConverter interface {
	ConvertRow(tableName string, columnNames []string, row string) (string, error)
	ConvertEvent(ev *tgtdb.Event, table string) error
}

func NewValueConverter(exportDir string, tdb tgtdb.TargetDB, snapshotMode bool) ValueConverter {
	if utils.IsDebeziumForDataExport(exportDir) {
		return NewDebeziumValueConverter(exportDir, tdb, snapshotMode)
	} else {
		return &NoOpValueConverter{}
	}
}

//============================================================================

type NoOpValueConverter struct{}

func (_ *NoOpValueConverter) ConvertRow(tableName string, columnNames []string, row string) (string, error) {
	return row, nil
}

func (_ *NoOpValueConverter) ConvertEvent(ev *tgtdb.Event, table string) error {
	return nil
}

//============================================================================

type DebeziumValueConverter struct {
	schemaRegistry      *SchemaRegistry
	valueConverterSuite map[string]tgtdb.ConverterFn
	converterFnCache    map[string][]tgtdb.ConverterFn
	snapshotMode        bool
}

func NewDebeziumValueConverter(exportDir string, tdb tgtdb.TargetDB, snapshotMode bool) *DebeziumValueConverter {
	schemaRegistry := NewSchemaRegistry(exportDir)
	tdbValueConverterSuite := tdb.GetDebeziumValueConverterSuite(snapshotMode)

	return &DebeziumValueConverter{
		schemaRegistry:      schemaRegistry,
		valueConverterSuite: tdbValueConverterSuite,
		converterFnCache:    map[string][]tgtdb.ConverterFn{},
		snapshotMode:        snapshotMode,
	}
}

func (conv *DebeziumValueConverter) ConvertRow(tableName string, columnNames []string, row string) (string, error) {
	converterFns, err := conv.getConverterFnCache(tableName, columnNames)
	if err != nil {
		return row, err
	}
	columnValues := strings.Split(row, "\t")
	var transformedValues []string
	for i, columnValue := range columnValues {
		if columnValue != "\\N" {
			transformedValue, err := converterFns[i](columnValue)
			if err != nil {
				return row, err
			}
			transformedValues = append(transformedValues, transformedValue)
		} else {
			transformedValues = append(transformedValues, columnValue)
		}
	}
	transformedRow := strings.Join(transformedValues, "\t")
	return transformedRow, nil
}

func (conv *DebeziumValueConverter) getConverterFnCache(tableName string, columnNames []string) ([]tgtdb.ConverterFn, error) {
	result := conv.converterFnCache[tableName]
	if result == nil {
		colSchemas, err := conv.schemaRegistry.GetColumnSchemas(tableName, columnNames)
		if err != nil {
			return nil, err
		}
		result = make([]tgtdb.ConverterFn, len(columnNames))
		for i, _ := range columnNames {
			schema := colSchemas[i]
			result[i] = schema.ColDbzSchema.GetConverterFn(conv.valueConverterSuite)
		}
		conv.converterFnCache[tableName] = result
	}
	return result, nil
}

func (conv *DebeziumValueConverter) ConvertEvent(ev *tgtdb.Event, table string) error {
	err := conv.convertMap(table, ev.Key)
	if err != nil {
		return fmt.Errorf("convert event fields: %v", err)
	}
	err = conv.convertMap(table, ev.Fields)
	if err != nil {
		return fmt.Errorf("convert event keys: %v", err)
	}
	return nil
}

func (conv *DebeziumValueConverter) convertMap(tableName string, m map[string]interface{}) error {
	for column, value := range m {
		if value == nil {
			m[column] = "NULL"
			continue
		}
		columnValue := fmt.Sprintf("%v", value)
		colSchema, err := conv.schemaRegistry.GetColumnSchema(tableName, column)
		converterFn := colSchema.ColDbzSchema.GetConverterFn(conv.valueConverterSuite)
		columnValue, err = converterFn(columnValue)
		if err != nil {
			return err
		}
		m[column] = columnValue
	}
	return nil
}
