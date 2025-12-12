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
	"sync"

	goerrors "github.com/go-errors/errors"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	tgtdbsuite "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/suites"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/schemareg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/stdlibcsv"
)

type SnapshotPhaseValueConverter interface {
	ConvertRow(tableNameTup sqlname.NameTuple, columnNames []string, columnValues []string) error
	GetTableNameToSchema() (*utils.StructMap[sqlname.NameTuple, map[string]map[string]string], error) //returns table name to schema mapping
}

func NewSnapshotPhaseValueConverter(exportDir string, tdb tgtdb.TargetDB, targetConf tgtdb.TargetConf, importerRole string, sourceDBType string, tableList []sqlname.NameTuple) (SnapshotPhaseValueConverter, error) {
	return NewSnapshotPhaseDebeziumValueConverter(tableList, exportDir, tdb, targetConf, importerRole, sourceDBType)
}

func NewSnapshotPhaseNoOpValueConverter() (SnapshotPhaseValueConverter, error) {
	return &SnapshotPhaseNoOpValueConverter{}, nil
}

//============================================================================

type SnapshotPhaseNoOpValueConverter struct{}

func (nvc *SnapshotPhaseNoOpValueConverter) ConvertRow(tableName sqlname.NameTuple, columnNames []string, columnValues []string) error {
	return nil
}

func (nvc *SnapshotPhaseNoOpValueConverter) GetTableNameToSchema() (*utils.StructMap[sqlname.NameTuple, map[string]map[string]string], error) {
	return utils.NewStructMap[sqlname.NameTuple, map[string]map[string]string](), nil
}

//============================================================================

type SnapshotPhaseDebeziumValueConverter struct {
	exportDir              string
	targetDBType           string
	tdb                    tgtdb.TargetDB
	schemaRegistrySource   *schemareg.SchemaRegistry
	valueConverterSuite    map[string]tgtdbsuite.ConverterFn
	cacheMutex             sync.RWMutex
	converterFnCache       *utils.StructMap[sqlname.NameTuple, []tgtdbsuite.ConverterFn] //stores table name to converter functions for each column
	dbzmColumnSchemasCache *utils.StructMap[sqlname.NameTuple, []*schemareg.ColumnSchema]
}

// Initialize debezium value converter with the given table list
func NewSnapshotPhaseDebeziumValueConverter(tableList []sqlname.NameTuple, exportDir string, tdb tgtdb.TargetDB, targetConf tgtdb.TargetConf, importerRole string, sourceDBType string) (*SnapshotPhaseDebeziumValueConverter, error) {
	schemaRegistrySource := schemareg.NewSchemaRegistry(tableList, exportDir, "source_db_exporter", importerRole)
	err := schemaRegistrySource.Init()
	if err != nil {
		return nil, fmt.Errorf("initializing schema registry: %w", err)
	}

	tdbValueConverterSuite, err := getDebeziumValueConverterSuite(targetConf)
	if err != nil {
		return nil, err
	}

	conv := &SnapshotPhaseDebeziumValueConverter{
		exportDir:              exportDir,
		schemaRegistrySource:   schemaRegistrySource,
		valueConverterSuite:    tdbValueConverterSuite,
		converterFnCache:       utils.NewStructMap[sqlname.NameTuple, []tgtdbsuite.ConverterFn](),
		dbzmColumnSchemasCache: utils.NewStructMap[sqlname.NameTuple, []*schemareg.ColumnSchema](),
		targetDBType:           targetConf.TargetDBType,
		tdb:                    tdb,
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
		return nil, goerrors.Errorf("no converter suite found for %s", tconf.TargetDBType)
	}
}

/*
Used by every batch producer to convert csv row string to transformed csv row string
*/
func (conv *SnapshotPhaseDebeziumValueConverter) ConvertRow(tableNameTup sqlname.NameTuple, columnNames []string, columnValues []string) error {
	converterFns, dbzmColumnSchemas, err := conv.getConverterFns(tableNameTup, columnNames)
	if err != nil {
		return fmt.Errorf("fetching converter functions: %w", err)
	}

	for i, columnValue := range columnValues {
		if columnValue == utils.YB_VOYAGER_NULL_STRING || converterFns[i] == nil { // TODO: make nullstring condition Target specific tdb.NullString()
			continue
		}
		transformedValue, err := converterFns[i](columnValue, false, dbzmColumnSchemas[i])
		if err != nil {
			return fmt.Errorf("converting value for %s, column %d and value %s : %w", tableNameTup, i, columnValue, err)
		}
		columnValues[i] = transformedValue
	}

	return nil
}

func (conv *SnapshotPhaseDebeziumValueConverter) getConverterFns(tableNameTup sqlname.NameTuple, columnNames []string) ([]tgtdbsuite.ConverterFn, []*schemareg.ColumnSchema, error) {
	// First try: read from cache with read lock
	conv.cacheMutex.RLock()
	result, ok1 := conv.converterFnCache.Get(tableNameTup)
	colSchemas, ok2 := conv.dbzmColumnSchemasCache.Get(tableNameTup)
	conv.cacheMutex.RUnlock()

	if ok1 && ok2 && result != nil {
		return result, colSchemas, nil
	}

	// Cache miss: acquire write lock to populate cache
	conv.cacheMutex.Lock()
	defer conv.cacheMutex.Unlock()

	// Double-check: another goroutine might have populated while we waited for the lock
	result, ok1 = conv.converterFnCache.Get(tableNameTup)
	colSchemas, ok2 = conv.dbzmColumnSchemasCache.Get(tableNameTup)
	if ok1 && ok2 && result != nil {
		return result, colSchemas, nil
	}

	// Actually populate the cache
	colTypes, colSchemas, err := conv.schemaRegistrySource.GetColumnTypes(tableNameTup, columnNames, conv.shouldFormatAsPerSourceDatatypes())
	if err != nil {
		return nil, nil, fmt.Errorf("get types of columns of table %s: %w", tableNameTup, err)
	}

	result = make([]tgtdbsuite.ConverterFn, len(columnNames))
	for i, colType := range colTypes {
		result[i] = conv.valueConverterSuite[colType]
	}

	conv.converterFnCache.Put(tableNameTup, result)
	conv.dbzmColumnSchemasCache.Put(tableNameTup, colSchemas)

	return result, colSchemas, nil
}

func (conv *SnapshotPhaseDebeziumValueConverter) shouldFormatAsPerSourceDatatypes() bool {
	return conv.targetDBType == tgtdb.ORACLE
}

func (conv *SnapshotPhaseDebeziumValueConverter) GetTableNameToSchema() (*utils.StructMap[sqlname.NameTuple, map[string]map[string]string], error) {

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

type CsvRowProcessor struct {
	tableNameTup sqlname.NameTuple
	// readers to help read from a csv string to slice of strings (columns)
	csvReader *stdlibcsv.Reader
	bufReader *bufio.Reader

	// writer to help write a csv string from a slice of strings (columns)
	bufWriter *bufio.Writer
	wbuf      *bytes.Buffer

	// Note: One concern with these buffers (csv reader's rawBuffer and writer's wbuf)
	// is that they will grow to accomodate large rows, but will never shrunk in size later on.
	// We support up to ~250MB of row size, so if multiple row processors are initialized anr running
	// in parallel, and if we encounter occasional large rows, they could easily eat up memory.
	// However, on testing, it was observed that due to operations like buf = buf[:0], the RSS memory (physical RAM)
	// eventually gets freed up  after having processed large rows.
	// This is because golang tells the OS that it can free up those pages. However those still
	// remain allocated in virtual memory.
	//
	// Therefore, it is not really a concern if we have to accomodate
	// occasional large rows for multiple batch producers in parallel.
	// If this becomes a problem, we can easily solve it by periodically (every so 100k records for example),
	// resetting the buffers by re-initializing them.
}

func NewCsvRowProcessor(tableNameTup sqlname.NameTuple) *CsvRowProcessor {
	bufReader := &bufio.Reader{}
	csvReader := stdlibcsv.NewReader(bufReader)
	csvReader.ReuseRecord = true

	bufWriter := &bufio.Writer{}
	wbuf := &bytes.Buffer{}

	return &CsvRowProcessor{
		tableNameTup: tableNameTup,
		csvReader:    csvReader,
		bufReader:    bufReader,
		bufWriter:    bufWriter,
		wbuf:         wbuf,
	}
}
func (crp *CsvRowProcessor) ReadRow(row string) ([]string, error) {
	crp.bufReader.Reset(strings.NewReader(row))
	columnValues, err := crp.csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading row: %w", err)
	}
	return columnValues, nil
}

func (crp *CsvRowProcessor) WriteRow(columnValues []string) (string, error) {
	crp.bufWriter.Reset(crp.wbuf)
	csvWriter := csv.NewWriter(crp.bufWriter)
	csvWriter.Write(columnValues)
	csvWriter.Flush()
	row := strings.TrimSuffix(crp.wbuf.String(), "\n")
	crp.wbuf.Reset()
	return row, nil
}

//====================================================StreamingPhaseValueConverter====================================================

type StreamingPhaseValueConverter interface {
	ConvertEvent(ev *tgtdb.Event, tableNameTup sqlname.NameTuple, formatIfRequired bool) error
}

type StreamingPhaseDebeziumValueConverter struct {
	exportDir            string
	schemaRegistrySource *schemareg.SchemaRegistry
	schemaRegistryTarget *schemareg.SchemaRegistry
	valueConverterSuite  map[string]tgtdbsuite.ConverterFn
	targetDBType         string
}

func NewStreamingPhaseDebeziumValueConverter(tableList []sqlname.NameTuple, exportDir string, targetConf tgtdb.TargetConf, importerRole string) (*StreamingPhaseDebeziumValueConverter, error) {
	schemaRegistrySource := schemareg.NewSchemaRegistry(tableList, exportDir, "source_db_exporter", importerRole)
	err := schemaRegistrySource.Init()
	if err != nil {
		return nil, fmt.Errorf("initializing schema registry: %w", err)
	}
	var schemaRegistryTarget *schemareg.SchemaRegistry
	switch importerRole {
	case "source_replica_db_importer":
		schemaRegistryTarget = schemareg.NewSchemaRegistry(tableList, exportDir, "target_db_exporter_ff", importerRole)
	case "source_db_importer":
		schemaRegistryTarget = schemareg.NewSchemaRegistry(tableList, exportDir, "target_db_exporter_fb", importerRole)
	}

	tdbValueConverterSuite, err := getDebeziumValueConverterSuite(targetConf)
	if err != nil {
		return nil, err
	}

	conv := &StreamingPhaseDebeziumValueConverter{
		exportDir:            exportDir,
		schemaRegistrySource: schemaRegistrySource,
		schemaRegistryTarget: schemaRegistryTarget,
		valueConverterSuite:  tdbValueConverterSuite,
		targetDBType:         targetConf.TargetDBType,
	}

	return conv, nil
}

func (conv *StreamingPhaseDebeziumValueConverter) ConvertEvent(ev *tgtdb.Event, tableNameTup sqlname.NameTuple, formatIfRequired bool) error {
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

func (conv *StreamingPhaseDebeziumValueConverter) convertMap(tableNameTup sqlname.NameTuple, m map[string]*string, exportSourceType string, formatIfRequired bool) error {
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

func (conv *StreamingPhaseDebeziumValueConverter) shouldFormatAsPerSourceDatatypes() bool {
	return conv.targetDBType == tgtdb.ORACLE
}
