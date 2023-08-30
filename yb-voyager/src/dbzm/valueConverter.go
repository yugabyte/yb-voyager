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
	ConvertEvent(ev *tgtdb.Event, table string, formatIfRequired bool) error
}

func NewValueConverter(exportDir string, tdb tgtdb.TargetDB, targetConf tgtdb.TargetConf) (ValueConverter, error) {
	if IsDebeziumForDataExport(exportDir) {
		return NewDebeziumValueConverter(exportDir, tdb, targetConf)
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
	exportDir            string
	schemaRegistrySource *SchemaRegistry
	schemaRegistryTarget *SchemaRegistry
	targetDBType         string
	targetSchema         string
	valueConverterSuite  map[string]tgtdb.ConverterFn
	converterFnCache     map[string][]tgtdb.ConverterFn //stores table name to converter functions for each column
}

func NewDebeziumValueConverter(exportDir string, tdb tgtdb.TargetDB, targetConf tgtdb.TargetConf) (*DebeziumValueConverter, error) {
	schemaRegistrySource := NewSchemaRegistry(exportDir, "source-db-exporter")
	err := schemaRegistrySource.Init()
	if err != nil {
		return nil, fmt.Errorf("initializing schema registry: %w", err)
	}
	schemaRegistryTarget := NewSchemaRegistry(exportDir, "target-db-exporter")
	tdbValueConverterSuite := tdb.GetDebeziumValueConverterSuite()

	return &DebeziumValueConverter{
		exportDir:            exportDir,
		schemaRegistrySource: schemaRegistrySource,
		schemaRegistryTarget: schemaRegistryTarget,
		valueConverterSuite:  tdbValueConverterSuite,
		converterFnCache:     map[string][]tgtdb.ConverterFn{},
		targetDBType:         targetConf.TargetDBType,
		targetSchema:         targetConf.Schema,
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

func (conv *DebeziumValueConverter) getConverterFns(tableName string, columnNames []string) ([]tgtdb.ConverterFn, error) {
	result := conv.converterFnCache[tableName]
	if result == nil {
		colTypes, err := conv.schemaRegistrySource.GetColumnTypes(tableName, columnNames)
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
	err := conv.convertMap(table, ev.Key, ev.ExporterRole, formatIfRequired)
	if err != nil {
		return fmt.Errorf("convert event key: %w", err)
	}
	err = conv.convertMap(table, ev.Fields, ev.ExporterRole, formatIfRequired)
	if err != nil {
		return fmt.Errorf("convert event fields: %w", err)
	}
	// setting tableName and schemaName as per target
	// TODO: handle properly. (maybe as part of targetDBinterface?)
	// TODO: handle case sensitivity/quoted table names..
	if conv.targetDBType == tgtdb.ORACLE {
		ev.TableName = strings.ToUpper(ev.TableName)
	}
	ev.SchemaName = conv.targetSchema
	return nil
}

func (conv *DebeziumValueConverter) convertMap(tableName string, m map[string]*string, exportSourceType string, formatIfRequired bool) error {
	var schemaRegistry *SchemaRegistry
	if exportSourceType == "source-db-exporter" {
		schemaRegistry = conv.schemaRegistrySource
	} else {
		schemaRegistry = conv.schemaRegistryTarget
	}
	for column, value := range m {
		if value == nil {
			continue
		}
		columnValue := *value
		colType, err := schemaRegistry.GetColumnType(tableName, column)
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
