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
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	tgtdbsuite "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/suites"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/schemareg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/stdlibcsv"
)

type ValueConverter interface {
	ConvertRow(tableNameTup sqlname.NameTuple, columnNames []string, row string) (string, error)
	ConvertEvent(ev *tgtdb.Event, tableNameTup sqlname.NameTuple, formatIfRequired bool) error
	GetTableNameToSchema() (*utils.StructMap[sqlname.NameTuple, map[string]map[string]string], error) //returns table name to schema mapping
}

func NewValueConverter(exportDir string, tdb tgtdb.TargetDB, targetConf tgtdb.TargetConf, importerRole string, sourceDBType string, tableList []sqlname.NameTuple) (ValueConverter, error) {
	return NewDebeziumValueConverter(tableList, exportDir, tdb, targetConf, importerRole, sourceDBType)
}

func NewNoOpValueConverter() (ValueConverter, error) {
	return &NoOpValueConverter{}, nil
}

//============================================================================

type NoOpValueConverter struct{}

func (nvc *NoOpValueConverter) ConvertRow(tableName sqlname.NameTuple, columnNames []string, row string) (string, error) {
	return row, nil
}

func (nvc *NoOpValueConverter) ConvertEvent(ev *tgtdb.Event, table sqlname.NameTuple, formatIfRequired bool) error {
	return nil
}

func (nvc *NoOpValueConverter) GetTableNameToSchema() (*utils.StructMap[sqlname.NameTuple, map[string]map[string]string], error) {
	return utils.NewStructMap[sqlname.NameTuple, map[string]map[string]string](), nil
}

//============================================================================

type DebeziumValueConverter struct {
	exportDir              string
	schemaRegistrySource   *schemareg.SchemaRegistry
	schemaRegistryTarget   *schemareg.SchemaRegistry
	targetSchema           string
	tdb                    tgtdb.TargetDB
	valueConverterSuite    map[string]tgtdbsuite.ConverterFn
	converterFnCache       *utils.StructMap[sqlname.NameTuple, []tgtdbsuite.ConverterFn] //stores table name to converter functions for each column
	dbzmColumnSchemasCache *utils.StructMap[sqlname.NameTuple, []*schemareg.ColumnSchema]
	targetDBType           string
	csvReader              *stdlibcsv.Reader
	bufReader              bufio.Reader
	bufWriter              bufio.Writer
	wbuf                   bytes.Buffer
	prevTableName          sqlname.NameTuple
	sourceDBType           string
}

//Initialize debezium value converter with the given table list
func NewDebeziumValueConverter(tableList []sqlname.NameTuple, exportDir string, tdb tgtdb.TargetDB, targetConf tgtdb.TargetConf, importerRole string, sourceDBType string) (*DebeziumValueConverter, error) {
	schemaRegistrySource := schemareg.NewSchemaRegistry(tableList, exportDir, "source_db_exporter")
	err := schemaRegistrySource.Init()
	if err != nil {
		return nil, fmt.Errorf("initializing schema registry: %w", err)
	}
	var schemaRegistryTarget *schemareg.SchemaRegistry
	switch importerRole {
	case "source_replica_db_importer":
		schemaRegistryTarget = schemareg.NewSchemaRegistry(tableList, exportDir, "target_db_exporter_ff")
	case "source_db_importer":
		schemaRegistryTarget = schemareg.NewSchemaRegistry(tableList, exportDir, "target_db_exporter_fb")
	}

	tdbValueConverterSuite, err := getDebeziumValueConverterSuite(targetConf)
	if err != nil {
		return nil, err
	}

	conv := &DebeziumValueConverter{
		exportDir:              exportDir,
		schemaRegistrySource:   schemaRegistrySource,
		schemaRegistryTarget:   schemaRegistryTarget,
		valueConverterSuite:    tdbValueConverterSuite,
		converterFnCache:       utils.NewStructMap[sqlname.NameTuple, []tgtdbsuite.ConverterFn](),
		dbzmColumnSchemasCache: utils.NewStructMap[sqlname.NameTuple, []*schemareg.ColumnSchema](),
		targetDBType:           targetConf.TargetDBType,
		targetSchema:           targetConf.Schema,
		tdb:                    tdb,
		sourceDBType:           sourceDBType,
	}

	return conv, nil
}

func getDebeziumValueConverterSuite(tconf tgtdb.TargetConf) (map[string]tgtdbsuite.ConverterFn, error) {
	switch tconf.TargetDBType {
	case tgtdb.ORACLE:
		oraValueConverterSuite := tgtdbsuite.OraValueConverterSuite
		for _, i := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9} {
			intervalType := fmt.Sprintf("INTERVAL YEAR(%d) TO MONTH", i) //for all interval year to month types with precision
			oraValueConverterSuite[intervalType] = oraValueConverterSuite["INTERVAL YEAR TO MONTH"]
		}
		for _, i := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9} {
			for _, j := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9} {
				intervalType := fmt.Sprintf("INTERVAL DAY(%d) TO SECOND(%d)", i, j) //for all interval day to second types with precision
				oraValueConverterSuite[intervalType] = oraValueConverterSuite["INTERVAL DAY TO SECOND"]
			}
		}
		return oraValueConverterSuite, nil
	case tgtdb.YUGABYTEDB, tgtdb.POSTGRESQL:
		return tgtdbsuite.YBValueConverterSuite, nil
	default:
		return nil, fmt.Errorf("no converter suite found for %s", tconf.TargetDBType)
	}
}

func (conv *DebeziumValueConverter) ConvertRow(tableNameTup sqlname.NameTuple, columnNames []string, row string) (string, error) {
	converterFns, dbzmColumnSchemas, err := conv.getConverterFns(tableNameTup, columnNames)
	if err != nil {
		return "", fmt.Errorf("fetching converter functions: %w", err)
	}
	if conv.prevTableName != tableNameTup {
		conv.csvReader = stdlibcsv.NewReader(&conv.bufReader)
		conv.csvReader.ReuseRecord = true
	}
	conv.bufReader.Reset(strings.NewReader(row))
	columnValues, err := conv.csvReader.Read()
	if err != nil {
		return "", fmt.Errorf("reading row: %w", err)
	}
	for i, columnValue := range columnValues {
		if columnValue == utils.YB_VOYAGER_NULL_STRING || converterFns[i] == nil { // TODO: make nullstring condition Target specific tdb.NullString()
			continue
		}
		transformedValue, err := converterFns[i](columnValue, false, dbzmColumnSchemas[i])
		if err != nil {
			return "", fmt.Errorf("converting value for %s, column %d and value %s : %w", tableNameTup, i, columnValue, err)
		}
		columnValues[i] = transformedValue
	}
	conv.bufWriter.Reset(&conv.wbuf)
	csvWriter := csv.NewWriter(&conv.bufWriter)
	csvWriter.Write(columnValues)
	csvWriter.Flush()
	transformedRow := strings.TrimSuffix(conv.wbuf.String(), "\n")
	conv.wbuf.Reset()
	conv.prevTableName = tableNameTup
	return transformedRow, nil
}

func (conv *DebeziumValueConverter) getConverterFns(tableNameTup sqlname.NameTuple, columnNames []string) ([]tgtdbsuite.ConverterFn, []*schemareg.ColumnSchema, error) {
	result, _ := conv.converterFnCache.Get(tableNameTup)
	colSchemas, _ := conv.dbzmColumnSchemasCache.Get(tableNameTup)
	var colTypes []string
	var err error
	if result == nil {
		colTypes, colSchemas, err = conv.schemaRegistrySource.GetColumnTypes(tableNameTup, columnNames, conv.shouldFormatAsPerSourceDatatypes())
		if err != nil {
			return nil, nil, fmt.Errorf("get types of columns of table %s: %w", tableNameTup, err)
		}
		result = make([]tgtdbsuite.ConverterFn, len(columnNames))
		for i, colType := range colTypes {
			result[i] = conv.valueConverterSuite[colType]
		}
		conv.converterFnCache.Put(tableNameTup, result)
		conv.dbzmColumnSchemasCache.Put(tableNameTup, colSchemas)
	}
	return result, colSchemas, nil
}

func (conv *DebeziumValueConverter) shouldFormatAsPerSourceDatatypes() bool {
	return conv.targetDBType == tgtdb.ORACLE
}

func (conv *DebeziumValueConverter) ConvertEvent(ev *tgtdb.Event, tableNameTup sqlname.NameTuple, formatIfRequired bool) error {
	err := conv.convertMap(tableNameTup, ev.Key, ev.ExporterRole, formatIfRequired)
	if err != nil {
		return fmt.Errorf("convert event(vsn=%d) key: %w", ev.Vsn, err)
	}

	err = conv.convertMap(tableNameTup, ev.Fields, ev.ExporterRole, formatIfRequired)
	if err != nil {
		return fmt.Errorf("convert event fields: %w", err)
	}
	return nil
}

func checkSourceExporter(exporterRole string) bool {
	return exporterRole == "source_db_exporter"
}

func (conv *DebeziumValueConverter) convertMap(tableNameTup sqlname.NameTuple, m map[string]*string, exportSourceType string, formatIfRequired bool) error {
	var schemaRegistry *schemareg.SchemaRegistry
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
		colType, colDbzmSchema, err := schemaRegistry.GetColumnType(tableNameTup, column, conv.shouldFormatAsPerSourceDatatypes())
		if err != nil {
			return fmt.Errorf("fetch column schema: %w", err)
		}
		if !checkSourceExporter(exportSourceType) && strings.EqualFold(colType, "io.debezium.time.Interval") {
			colType, colDbzmSchema, err = conv.schemaRegistrySource.GetColumnType(tableNameTup, strings.ToUpper(column), conv.shouldFormatAsPerSourceDatatypes())
			//assuming table name/column name is case insensitive TODO: handle this case sensitivity properly
			if err != nil {
				return fmt.Errorf("fetch column schema: %w", err)
			}
		}
		converterFn := conv.valueConverterSuite[colType]
		if converterFn != nil {
			columnValue, err = converterFn(columnValue, formatIfRequired, colDbzmSchema)
			if err != nil {
				return fmt.Errorf("error while converting %s.%s of type %s in event: %w", tableNameTup, column, colType, err) // TODO - add event id in log msg
			}
		}
		m[column] = &columnValue
	}
	return nil
}

func (conv *DebeziumValueConverter) GetTableNameToSchema() (*utils.StructMap[sqlname.NameTuple, map[string]map[string]string], error) {

	//need to create explicit map with required details only as can't use TableSchema directly in import area because of cyclic dependency
	//TODO: fix this cyclic dependency maybe using DataFileDescriptor
	var tableToSchema = utils.NewStructMap[sqlname.NameTuple, map[string]map[string]string]()

	err := conv.schemaRegistrySource.TableNameToSchema.IterKV(func(tbl sqlname.NameTuple, tblSchema *schemareg.TableSchema) (bool, error) {
		colSchemaMap := make(map[string]map[string]string)

		for _, col := range tblSchema.Columns {
			colNameQuoted, err := conv.tdb.QuoteAttributeName(tbl, col.Name)
			if err != nil {
				return false, fmt.Errorf("error quoting attribute name %s for table %s: %w", col.Name, tbl, err)
			}
			colSchemaMap[colNameQuoted] = col.Schema.Parameters
		}
		tableToSchema.Put(tbl, colSchemaMap)
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return tableToSchema, nil
}
