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
	tgtdbsuite "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/suites"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type ValueConverter interface {
	ConvertRow(tableName string, columnNames []string, row string) (string, error)
	ConvertEvent(ev *tgtdb.Event, table string, formatIfRequired bool) error
	GetTableNameToSchema() map[string]map[string]map[string]string //returns table name to schema mapping
}

func NewValueConverter(exportDir string, tdb tgtdb.TargetDB, targetConf tgtdb.TargetConf, importerRole string) (ValueConverter, error) {
	return NewDebeziumValueConverter(exportDir, tdb, targetConf, importerRole)
}

func NewNoOpValueConverter() (ValueConverter, error) {
	return &NoOpValueConverter{}, nil
}

//============================================================================

type NoOpValueConverter struct{}

func (nvc *NoOpValueConverter) ConvertRow(tableName string, columnNames []string, row string) (string, error) {
	return row, nil
}

func (nvc *NoOpValueConverter) ConvertEvent(ev *tgtdb.Event, table string, formatIfRequired bool) error {
	return nil
}

func (nvc *NoOpValueConverter) GetTableNameToSchema() map[string]map[string]map[string]string {
	return nil
}

//============================================================================

type DebeziumValueConverter struct {
	exportDir            string
	schemaRegistrySource *SchemaRegistry
	schemaRegistryTarget *SchemaRegistry
	targetSchema         string
	valueConverterSuite  map[string]tgtdbsuite.ConverterFn
	converterFnCache     map[string][]tgtdbsuite.ConverterFn //stores table name to converter functions for each column
	targetDBType         string
	buf                  bytes.Buffer
}

func NewDebeziumValueConverter(exportDir string, tdb tgtdb.TargetDB, targetConf tgtdb.TargetConf, importerRole string) (*DebeziumValueConverter, error) {
	schemaRegistrySource := NewSchemaRegistry(exportDir, "source_db_exporter")
	err := schemaRegistrySource.Init()
	if err != nil {
		return nil, fmt.Errorf("initializing schema registry: %w", err)
	}
	var schemaRegistryTarget *SchemaRegistry
	switch importerRole {
	case "source_replica_db_importer":
		schemaRegistryTarget = NewSchemaRegistry(exportDir, "target_db_exporter_ff")
	case "source_db_importer":
		schemaRegistryTarget = NewSchemaRegistry(exportDir, "target_db_exporter_fb")
	}

	tdbValueConverterSuite := tdb.GetDebeziumValueConverterSuite()

	return &DebeziumValueConverter{
		exportDir:            exportDir,
		schemaRegistrySource: schemaRegistrySource,
		schemaRegistryTarget: schemaRegistryTarget,
		valueConverterSuite:  tdbValueConverterSuite,
		converterFnCache:     map[string][]tgtdbsuite.ConverterFn{},
		targetDBType:         targetConf.TargetDBType,
		targetSchema:         targetConf.Schema,
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
	csvWriter := csv.NewWriter(&conv.buf)
	csvWriter.Write(columnValues)
	csvWriter.Flush()
	transformedRow := strings.TrimSuffix(conv.buf.String(), "\n")
	conv.buf.Reset()
	return transformedRow, nil
}

func (conv *DebeziumValueConverter) getConverterFns(tableName string, columnNames []string) ([]tgtdbsuite.ConverterFn, error) {
	result := conv.converterFnCache[tableName]
	if result == nil {
		colTypes, err := conv.schemaRegistrySource.GetColumnTypes(tableName, columnNames, conv.shouldFormatAsPerSourceDatatypes())
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

func (conv *DebeziumValueConverter) shouldFormatAsPerSourceDatatypes() bool {
	return conv.targetDBType == tgtdb.ORACLE
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

func checkSourceExporter(exporterRole string) bool {
	return exporterRole == "source_db_exporter"
}

func (conv *DebeziumValueConverter) convertMap(tableName string, m map[string]*string, exportSourceType string, formatIfRequired bool) error {
	var schemaRegistry *SchemaRegistry
	if checkSourceExporter(exportSourceType) {
		schemaRegistry = conv.schemaRegistrySource
	} else {
		schemaRegistry = conv.schemaRegistryTarget
	}
	for column, value := range m {
		if value == nil {
			continue
		}
		columnValue := *value
		colType, err := schemaRegistry.GetColumnType(tableName, column, conv.shouldFormatAsPerSourceDatatypes())
		if err != nil {
			return fmt.Errorf("fetch column schema: %w", err)
		}
		if !checkSourceExporter(exportSourceType) && strings.EqualFold(colType, "io.debezium.time.Interval") {
			colType, err = conv.schemaRegistrySource.GetColumnType(strings.ToUpper(tableName), strings.ToUpper(column), conv.shouldFormatAsPerSourceDatatypes())
			//assuming table name/column name is case insensitive TODO: handle this case sensitivity properly
			if err != nil {
				return fmt.Errorf("fetch column schema: %w", err)
			}
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

func (conv *DebeziumValueConverter) GetTableNameToSchema() map[string]map[string]map[string]string {

	//need to create explicit map with required details only as can't use TableSchema directly in import area because of cyclic dependency
	//TODO: fix this cyclic dependency maybe using DataFileDescriptor
	var tableToSchema = make(map[string]map[string]map[string]string)
	// tableToSchema {<table>: {<column>:<parameters>}}
	for table, col := range conv.schemaRegistrySource.tableNameToSchema {
		tableToSchema[table] = make(map[string]map[string]string)
		for _, col := range col.Columns {
			tableToSchema[table][col.Name] = col.Schema.Parameters
		}
	}
	return tableToSchema
}
