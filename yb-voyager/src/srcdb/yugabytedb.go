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
package srcdb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// Apart from these we also skip UDT columns. Array of enums, hstore, and tsvector are supported with logical connector (default).
var YugabyteUnsupportedDataTypesForDbzmLogical = []string{"BOX", "CIRCLE", "LINE", "LSEG", "PATH", "PG_LSN", "POINT", "POLYGON", "TSQUERY", "TXID_SNAPSHOT", "GEOMETRY", "GEOGRAPHY", "RASTER"}


//For the gRPC connector - datatypes like HSTORE/CITEXT/LTREE that are available by extensions, are not supported and the table of these needs to be skipped for the migration with grpc connector
//but right now we are only skipping columns of that table and if there are DML on those tables the gRPC connector will error out. 
// TODO to handle that
var YugabyteUnsupportedDataTypesForDbzmGrpc = []string{"BOX", "CIRCLE", "LINE", "LSEG", "PATH", "PG_LSN", "POINT", "POLYGON", "TSQUERY", "TSVECTOR", "TXID_SNAPSHOT", "GEOMETRY", "GEOGRAPHY", "RASTER", "HSTORE", "CITEXT", "LTREE"}

func GetYugabyteUnsupportedDatatypesDbzm(isGRPCConnector bool) []string {
	if isGRPCConnector {
		return YugabyteUnsupportedDataTypesForDbzmGrpc
	}
	return YugabyteUnsupportedDataTypesForDbzmLogical
}

type YugabyteDB struct {
	source *Source

	db *sql.DB
}

func newYugabyteDB(s *Source) *YugabyteDB {
	return &YugabyteDB{source: s}
}

func (yb *YugabyteDB) Connect() error {
	if yb.db != nil {
		err := yb.db.Ping()
		if err == nil {
			log.Infof("Already connected to the source database")
			return nil
		} else {
			log.Infof("Failed to ping the source database: %s", err)
			yb.Disconnect()
		}
		log.Info("Reconnecting to the source database")
	}
	db, err := sql.Open("pgx", yb.getConnectionUri())
	db.SetMaxOpenConns(yb.source.NumConnections)
	db.SetConnMaxIdleTime(5 * time.Minute)
	yb.db = db

	err = yb.db.Ping()
	if err != nil {
		return err
	}
	return err
}

func (yb *YugabyteDB) Disconnect() {
	if yb.db == nil {
		log.Infof("No connection to the source database to close")
		return
	}

	err := yb.db.Close()
	if err != nil {
		log.Errorf("Failed to close connection to the source database: %s", err)
	}
}

func (yb *YugabyteDB) Query(query string) (*sql.Rows, error) {
	return yb.db.Query(query)
}

func (yb *YugabyteDB) QueryRow(query string) *sql.Row {
	return yb.db.QueryRow(query)
}

func (yb *YugabyteDB) GetTableRowCount(tableName sqlname.NameTuple) (int64, error) {
	var rowCount int64
	query := fmt.Sprintf("select count(*) from %s", tableName.ForUserQuery())
	log.Infof("Querying row count of table %q", tableName)
	err := yb.db.QueryRow(query).Scan(&rowCount)
	if err != nil {
		return 0, fmt.Errorf("query %q for row count of %q: %w", query, tableName, err)
	}
	log.Infof("Table %q has %v rows.", tableName, rowCount)
	return rowCount, nil
}

func (yb *YugabyteDB) GetTableApproxRowCount(tableName sqlname.NameTuple) int64 {
	var approxRowCount sql.NullInt64 // handles case: value of the row is null, default for int64 is 0
	query := fmt.Sprintf("SELECT reltuples::bigint FROM pg_class "+
		"where oid = '%s'::regclass", tableName.ForOutput())

	log.Infof("Querying '%s' approx row count of table %q", query, tableName.String())
	err := yb.db.QueryRow(query).Scan(&approxRowCount)
	if err != nil {
		utils.ErrExit("Failed to query: %q for approx row count of %q: %w", query, tableName.String(), err)
	}

	log.Infof("Table %q has approx %v rows.", tableName.String(), approxRowCount)
	return approxRowCount.Int64
}

func (yb *YugabyteDB) GetVersion() string {
	if yb.source.DBVersion != "" {
		return yb.source.DBVersion
	}

	var version string
	query := "SELECT setting from pg_settings where name = 'server_version'"
	err := yb.db.QueryRow(query).Scan(&version)
	if err != nil {
		utils.ErrExit("run query: %q on source: %w", query, err)
	}
	yb.source.DBVersion = version
	return version
}

func (yb *YugabyteDB) CheckSchemaExists() bool {
	schemaList := yb.checkSchemasExists()
	return schemaList != nil
}

func (yb *YugabyteDB) checkSchemasExists() []string {
	list := strings.Split(yb.source.Schema, "|")
	var trimmedList []string
	for _, schema := range list {
		if utils.IsQuotedString(schema) {
			schema = strings.Trim(schema, `"`)
		}
		trimmedList = append(trimmedList, schema)
	}
	querySchemaList := "'" + strings.Join(trimmedList, "','") + "'"
	chkSchemaExistsQuery := fmt.Sprintf(`SELECT schema_name
	FROM information_schema.schemata where schema_name IN (%s);`, querySchemaList)
	rows, err := yb.db.Query(chkSchemaExistsQuery)
	if err != nil {
		utils.ErrExit("error in querying source database for checking mentioned schema(s) present or not: %q: %w\n", chkSchemaExistsQuery, err)
	}
	var listOfSchemaPresent []string
	var tableSchemaName string

	for rows.Next() {
		err = rows.Scan(&tableSchemaName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for schema names: %w\n", err)
		}
		listOfSchemaPresent = append(listOfSchemaPresent, tableSchemaName)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", chkSchemaExistsQuery, closeErr)
		}
	}()

	schemaNotPresent := utils.SetDifference(trimmedList, listOfSchemaPresent)
	if len(schemaNotPresent) > 0 {
		utils.ErrExit("Following schemas are not present in source database: %v, please provide a valid schema list.\n", schemaNotPresent)
	}
	return trimmedList
}

func (yb *YugabyteDB) GetAllTableNamesRaw(schemaName string) ([]string, error) {
	// Information schema requires select permission on the tables to query the tables. However, pg_catalog does not require any permission.
	// So, we are using pg_catalog to get the table names.
	query := fmt.Sprintf(`
	SELECT 
		c.relname AS table_name
	FROM 
		pg_catalog.pg_class c
	JOIN 
		pg_catalog.pg_namespace n ON n.oid = c.relnamespace
	WHERE 
		c.relkind IN ('r', 'p')  -- 'r' for regular tables, 'p' for partitioned tables
		AND n.nspname = '%s'; 
	`, schemaName)

	rows, err := yb.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error in querying(%q) YB database for table names: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()

	var tableNames []string
	var tableName string

	for rows.Next() {
		err = rows.Scan(&tableName)
		if err != nil {
			return nil, fmt.Errorf("error in scanning query rows for table names: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}
	log.Infof("Query found %d tables in the source db: %v", len(tableNames), tableNames)
	return tableNames, nil
}

func (yb *YugabyteDB) GetAllTableNames() []*sqlname.SourceName {
	schemaList := yb.checkSchemasExists()
	querySchemaList := "'" + strings.Join(schemaList, "','") + "'"
	// Information schema requires select permission on the tables to query the tables. However, pg_catalog does not require any permission.
	// So, we are using pg_catalog to get the table names.
	query := fmt.Sprintf(`
	SELECT 
		n.nspname AS table_schema,
		c.relname AS table_name
	FROM 
		pg_catalog.pg_class c
	JOIN 
		pg_catalog.pg_namespace n ON n.oid = c.relnamespace
	WHERE 
		c.relkind IN ('r', 'p')  -- 'r' for regular tables, 'p' for partitioned tables
		AND n.nspname IN (%s);  
	`, querySchemaList)

	rows, err := yb.db.Query(query)
	if err != nil {
		utils.ErrExit("error in querying YB database for table names: %q: %w\n", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()

	var tableNames []*sqlname.SourceName
	var tableName, tableSchema string

	for rows.Next() {
		err = rows.Scan(&tableSchema, &tableName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for table names: %w\n", err)
		}
		tableName = fmt.Sprintf("\"%s\"", tableName)
		tableNames = append(tableNames, sqlname.NewSourceName(tableSchema, tableName))
	}
	log.Infof("Query found %d tables in the source db: %v", len(tableNames), tableNames)
	return tableNames
}

func (yb *YugabyteDB) getConnectionUri() string {
	source := yb.source
	if source.Uri == "" {
		hostAndPort := fmt.Sprintf("%s:%d", source.Host, source.Port)
		sourceUrl := &url.URL{
			Scheme:   "postgresql",
			User:     url.UserPassword(source.User, source.Password),
			Host:     hostAndPort,
			Path:     source.DBName,
			RawQuery: generateSSLQueryStringIfNotExists(source),
		}

		source.Uri = sourceUrl.String()
	}
	return source.Uri
}

func (yb *YugabyteDB) GetConnectionUriWithoutPassword() string {
	source := yb.source
	if source.Uri == "" {
		hostAndPort := fmt.Sprintf("%s:%d", source.Host, source.Port)
		sourceUrl := &url.URL{
			Scheme:   "postgresql",
			User:     url.User(source.User),
			Host:     hostAndPort,
			Path:     source.DBName,
			RawQuery: generateSSLQueryStringIfNotExists(source),
		}

		source.Uri = sourceUrl.String()
	}
	return source.Uri
}

func (yb *YugabyteDB) ExportSchema(exportDir string, schemaDir string) {
	panic("not implemented")
}

func (yb *YugabyteDB) GetIndexesInfo() []utils.IndexInfo {
	return nil
}

func (yb *YugabyteDB) ExportData(ctx context.Context, exportDir string, tableList []sqlname.NameTuple, quitChan chan bool, exportDataStart, exportSuccessChan chan bool, tablesColumnList *utils.StructMap[sqlname.NameTuple, []string], snapshotName string) {
	pgdumpExportDataOffline(ctx, yb.source, yb.GetConnectionUriWithoutPassword(), exportDir, tableList, quitChan, exportDataStart, exportSuccessChan, "")
}

func (yb *YugabyteDB) ExportDataPostProcessing(exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	renameDataFiles(tablesProgressMetadata)
	dfd := datafile.Descriptor{
		FileFormat:                 datafile.TEXT,
		DataFileList:               getExportedDataFileList(tablesProgressMetadata),
		Delimiter:                  "\t",
		HasHeader:                  false,
		ExportDir:                  exportDir,
		NullString:                 `\N`,
		TableNameToExportedColumns: yb.getExportedColumnsMap(exportDir, tablesProgressMetadata),
	}

	dfd.Save()
}

func (yb *YugabyteDB) getExportedColumnsMap(
	exportDir string, tablesMetadata map[string]*utils.TableProgressMetadata) map[string][]string {

	result := make(map[string][]string)
	for _, tableMetadata := range tablesMetadata {
		// TODO: Use tableMetadata.TableName instead of parsing the file name.
		// We need a new method in sqlname.SourceName that returns MaybeQuoted and MaybeQualified names.
		tableName := strings.TrimSuffix(filepath.Base(tableMetadata.FinalFilePath), "_data.sql")
		result[tableName] = yb.getExportedColumnsListForTable(exportDir, tableName)
	}
	return result
}

func (yb *YugabyteDB) getExportedColumnsListForTable(exportDir, tableName string) []string {
	var columnsList []string
	var re *regexp.Regexp
	if len(strings.Split(tableName, ".")) == 1 {
		// happens only when table is in public schema, use public schema with table name for regexp
		re = regexp.MustCompile(fmt.Sprintf(`(?i)COPY public.%s[\s]+\((.*)\) FROM STDIN`, tableName))
	} else {
		re = regexp.MustCompile(fmt.Sprintf(`(?i)COPY %s[\s]+\((.*)\) FROM STDIN`, tableName))
	}
	tocFilePath := filepath.Join(exportDir, "data", "toc.dat")
	err := utils.ForEachMatchingLineInFile(tocFilePath, re, func(matches []string) bool {
		columnsList = strings.Split(matches[1], ",")
		for i, column := range columnsList {
			columnsList[i] = strings.TrimSpace(column)
		}
		return false // stop reading file
	})
	if err != nil {
		utils.ErrExit("error in reading toc file: %w\n", err)
	}
	log.Infof("columns list for table %s: %v", tableName, columnsList)
	return columnsList
}

// GetAllSequences returns all the sequence names in the database for the given schema list
func (yb *YugabyteDB) GetAllSequences() []string {
	schemaList := yb.checkSchemasExists()
	querySchemaList := "'" + strings.Join(schemaList, "','") + "'"
	var sequenceNames []string
	query := fmt.Sprintf(`SELECT sequence_name FROM information_schema.sequences where sequence_schema IN (%s);`, querySchemaList)
	rows, err := yb.db.Query(query)
	if err != nil {
		utils.ErrExit("error in querying source database for sequence names: %q: %w\n", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()

	var sequenceName string
	for rows.Next() {
		err = rows.Scan(&sequenceName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for sequence names: %w\n", err)
		}
		sequenceNames = append(sequenceNames, sequenceName)
	}
	return sequenceNames
}

// GetAllSequencesRaw returns all the sequence names in the database for the schema
func (yb *YugabyteDB) GetAllSequencesRaw(schemaName string) ([]string, error) {
	var sequenceNames []string
	query := fmt.Sprintf(`SELECT sequencename FROM pg_sequences where schemaname = '%s';`, schemaName)
	rows, err := yb.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error in querying(%q) source database for sequence names: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()

	var sequenceName string
	for rows.Next() {
		err = rows.Scan(&sequenceName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for sequence names: %w", err)
		}
		sequenceNames = append(sequenceNames, sequenceName)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("error in scanning query rows for sequence names: %w", rows.Err())
	}
	return sequenceNames, nil
}

func (yb *YugabyteDB) GetCharset() (string, error) {
	query := fmt.Sprintf("SELECT pg_encoding_to_char(encoding) FROM pg_database WHERE datname = '%s';", yb.source.DBName)
	encoding := ""
	err := yb.db.QueryRow(query).Scan(&encoding)
	if err != nil {
		return "", fmt.Errorf("error in querying database encoding: %w", err)
	}
	return encoding, nil
}

func (yb *YugabyteDB) GetDatabaseSize() (int64, error) {
	var dbSize sql.NullInt64
	query := fmt.Sprintf("select pg_database_size('%s'); ", yb.source.DBName)
	err := yb.db.QueryRow(query).Scan(&dbSize)
	if err != nil {
		return 0, fmt.Errorf("error in querying database encoding: %w", err)
	}
	log.Infof("Total Database size of YugabyteDB sourceDB: %d", dbSize.Int64)
	return dbSize.Int64, nil
}

func (yb *YugabyteDB) getAllUserDefinedTypesInSchema(schemaName string) []string {
	query := fmt.Sprintf(`SELECT typname
						FROM pg_type t
						JOIN pg_namespace n ON t.typnamespace = n.oid
						WHERE n.nspname = '%s'
						AND t.typcategory <> 'A'
						AND t.typname NOT IN (
							SELECT table_name
							FROM information_schema.tables
							WHERE table_schema = '%s'
							UNION
							SELECT sequence_name
							FROM information_schema.sequences
							WHERE sequence_schema = '%s'
						);`, schemaName, schemaName, schemaName)
	rows, err := yb.db.Query(query)
	if err != nil {
		utils.ErrExit("error in querying source database for enum types: %q: %w\n", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()
	var enumTypes []string
	for rows.Next() {
		var enumType string
		err = rows.Scan(&enumType)
		if err != nil {
			utils.ErrExit("error in scanning query rows for enum types: %w\n", err)
		}
		enumTypes = append(enumTypes, enumType)
	}
	return enumTypes
}

func (yb *YugabyteDB) getAllEnumTypesInSchema(schemaName string) []string {
	query := fmt.Sprintf(`SELECT typname
						FROM pg_type t
						JOIN pg_namespace n ON t.typnamespace = n.oid
						WHERE n.nspname = '%s'
						AND t.typcategory = 'E';`, schemaName)
	rows, err := yb.db.Query(query)
	if err != nil {
		utils.ErrExit("error in querying source database for enum types: %q: %w\n", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()
	var enumTypes []string
	for rows.Next() {
		var enumType string
		err = rows.Scan(&enumType)
		if err != nil {
			utils.ErrExit("error in scanning query rows for enum types: %w\n", err)
		}
		enumTypes = append(enumTypes, enumType)
	}
	return enumTypes
}

func (yb *YugabyteDB) getTypesOfAllArraysInATable(schemaName, tableName string) []string {
	query := fmt.Sprintf(`SELECT udt_name FROM information_schema.columns 
						WHERE table_schema = '%s' 
						AND table_name='%s' 
						AND data_type = 'ARRAY';`, schemaName, tableName)
	rows, err := yb.db.Query(query)
	if err != nil {
		utils.ErrExit("error in querying source database for array types: %q: %w\n", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()
	var tableColumnUdtTypes []string
	for rows.Next() {
		var udtType string
		err = rows.Scan(&udtType)
		if err != nil {
			utils.ErrExit("error in scanning query rows for array types: %w\n", err)
		}
		tableColumnUdtTypes = append(tableColumnUdtTypes, udtType)
	}
	return tableColumnUdtTypes
}

func (yb *YugabyteDB) FilterUnsupportedTables(migrationUUID uuid.UUID, tableList []sqlname.NameTuple, useDebezium bool) ([]sqlname.NameTuple, []sqlname.NameTuple) {
	var unsupportedTables []sqlname.NameTuple
	var filteredTableList []sqlname.NameTuple
	for _, table := range tableList {
		sname, tname := table.ForCatalogQuery()
		userDefinedTypes := yb.getAllUserDefinedTypesInSchema(sname)
		if len(userDefinedTypes) == 0 {
			continue
		}
		enumTypes := yb.getAllEnumTypesInSchema(sname)
		tableColumnArrayTypes := yb.getTypesOfAllArraysInATable(sname, tname)
		if len(tableColumnArrayTypes) == 0 {
			continue
		}

		// Build list of unsupported types based on connector type
		// UDT types without enums are always unsupported
		udtTypes := utils.SetDifference(userDefinedTypes, enumTypes)
		unsupportedTableTypes := udtTypes
		// Array of enums are only unsupported with YBGrpcConnector
		if yb.source.IsYBGrpcConnector {
			unsupportedTableTypes = append(unsupportedTableTypes, enumTypes...)
		}

		// If any of the data types of the arrays are in the unsupported types then add the table to the unsupported tables list
		// udt_type/data_type looks like status_enum[] whereas enum_type looks like status_enum
	outer:
		for _, arrayType := range tableColumnArrayTypes {
			baseType := strings.TrimLeft(arrayType, "_")
			for _, unsupportedType := range unsupportedTableTypes {
				if strings.EqualFold(baseType, unsupportedType) {
					// as the array_type is determined by an underscore at the first place
					//ref - https://www.postgresql.org/docs/current/xtypes.html#:~:text=The%20array%20type%20typically%20has%20the%20same%20name%20as%20the%20base%20type%20with%20the%20underscore%20character%20(_)%20prepended
					unsupportedTables = append(unsupportedTables, table)
					break outer
				}
			}
		}

	}
	if len(unsupportedTables) > 0 {
		unsupportedTablesStringList := make([]string, len(unsupportedTables))
		for i, table := range unsupportedTables {
			unsupportedTablesStringList[i] = table.ForMinOutput()
		}

		var unsupportedReason string
		if yb.source.IsYBGrpcConnector {
			unsupportedReason = "an array of UDTs (enums or composite types)"
		} else {
			unsupportedReason = "an array of composite types"
		}

		if !utils.AskPrompt("\nThe following tables are unsupported since they contain " + unsupportedReason + ":\n" + strings.Join(unsupportedTablesStringList, "\n") +
			"\nDo you want to skip these tables' data and continue with export") {
			utils.ErrExit("Exiting at user's request. Use `--exclude-table-list` flag to continue without these tables")
		}
	}

	for _, table := range tableList {
		if !slices.Contains(unsupportedTables, table) {
			filteredTableList = append(filteredTableList, table)
		}
	}

	return filteredTableList, unsupportedTables
}

func (yb *YugabyteDB) FilterEmptyTables(tableList []sqlname.NameTuple) ([]sqlname.NameTuple, []sqlname.NameTuple) {
	if len(tableList) == 0 {
		return nil, nil
	}

	// Build a single UNION ALL query to check all tables at once.
	// Example query for 3 tables:
	//   SELECT 0 AS table_idx, EXISTS(SELECT 1 FROM public.products) AS has_rows
	//   UNION ALL
	//   SELECT 1 AS table_idx, EXISTS(SELECT 1 FROM public.users) AS has_rows
	//   UNION ALL
	//   SELECT 2 AS table_idx, EXISTS(SELECT 1 FROM public.invoices) AS has_rows
	var unionParts []string
	for idx, tableName := range tableList {
		unionParts = append(unionParts,
			fmt.Sprintf("SELECT %d AS table_idx, EXISTS(SELECT 1 FROM %s) AS has_rows",
				idx, tableName.ForUserQuery()))
	}
	query := strings.Join(unionParts, " UNION ALL ")

	rows, err := yb.db.Query(query)
	if err != nil {
		utils.ErrExit("failed to query for empty tables: %w", err)
	}
	defer rows.Close()

	tableIsEmpty := make([]bool, len(tableList)) // defaults to false for all tables
	for rows.Next() {
		var tableIdx int
		var hasRows bool
		if err := rows.Scan(&tableIdx, &hasRows); err != nil {
			utils.ErrExit("failed to scan row for empty table check: %w", err)
		}
		if !hasRows {
			tableIsEmpty[tableIdx] = true
		}
	}

	if err := rows.Err(); err != nil {
		utils.ErrExit("failed to iterate rows for empty table check: %w", err)
	}

	var nonEmptyTableList, emptyTableList []sqlname.NameTuple
	for i, isEmpty := range tableIsEmpty {
		if isEmpty {
			emptyTableList = append(emptyTableList, tableList[i])
		} else {
			nonEmptyTableList = append(nonEmptyTableList, tableList[i])
		}
	}
	return nonEmptyTableList, emptyTableList
}

/*
Currently all UDTs other than enums and domain are unsupported
so while querying the catalog table, we ignore the enums and domain types and only consider the composite types
i.e. typtype = 'c' (composite) AND NOT typtype = 'e' (enum) AND NOT typtype = 'd' (domain)

This function now accepts a slice of tables and returns a unique list of fully qualified
unsupported user-defined type names (e.g., "hr.contact", "inventory.device_specs") by making a single database query.
Qualified because same typename can be present in multiple schemas in completely different ways.
*/
func (yb *YugabyteDB) filterUnsupportedUserDefinedDatatypes(tableList []sqlname.NameTuple) []string {
	if len(tableList) == 0 {
		return []string{}
	}

	// Build the IN clause with tuples for all tables
	// eg: [('public', 'products'), ('public', 'users'), ('public', 'invoices')]
	var tableTuples []string
	for _, table := range tableList {
		schema, name := table.ForCatalogQuery()
		tableTuples = append(tableTuples, fmt.Sprintf("('%s', '%s')", schema, name))
	}
	inClause := strings.Join(tableTuples, ", ")

	query := fmt.Sprintf(`
		SELECT DISTINCT
			type_n.nspname || '.' || t.typname AS qualified_type_name
		FROM
			pg_attribute AS a
		JOIN
			pg_type AS t ON t.oid = a.atttypid
		JOIN
			pg_namespace AS type_n ON type_n.oid = t.typnamespace
		JOIN
			pg_class AS c ON c.oid = a.attrelid
		JOIN
			pg_namespace AS table_n ON table_n.oid = c.relnamespace
		WHERE
			(table_n.nspname, c.relname) IN (%s)
			AND a.attnum > 0
			AND t.typtype = 'c'
		ORDER BY qualified_type_name;
	`, inClause)

	rows, err := yb.db.Query(query)
	if err != nil {
		utils.ErrExit("error in querying source database for user defined columns: %q: %w\n", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()

	var userDefinedDataTypes []string
	for rows.Next() {
		var qualifiedTypeName string
		err = rows.Scan(&qualifiedTypeName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for user defined columns: %w\n", err)
		}
		userDefinedDataTypes = append(userDefinedDataTypes, qualifiedTypeName)
	}
	return userDefinedDataTypes
}

// tableColumnInfo holds columns and their data types information for a table
type tableColumnInfo struct {
	Columns   []string
	DataTypes []string
}

func (yb *YugabyteDB) GetColumnsWithSupportedTypes(tableList []sqlname.NameTuple, useDebezium bool, isStreamingEnabled bool) (*utils.StructMap[sqlname.NameTuple, []string], *utils.StructMap[sqlname.NameTuple, []string], error) {
	supportedTableColumnsMap := utils.NewStructMap[sqlname.NameTuple, []string]()
	unsupportedTableColumnsMap := utils.NewStructMap[sqlname.NameTuple, []string]()

	// offline migration case, we support all datatypes
	if !(useDebezium || isStreamingEnabled) {
		return supportedTableColumnsMap, unsupportedTableColumnsMap, nil
	}

	// Fetch all user-defined types for all tables in a single query and add them to the unsupported datatypes list
	userDefinedDataTypes := yb.filterUnsupportedUserDefinedDatatypes(tableList)
	unsupportedDatatypesList := GetYugabyteUnsupportedDatatypesDbzm(yb.source.IsYBGrpcConnector)
	unsupportedDatatypesList = append(unsupportedDatatypesList, userDefinedDataTypes...)

	// Fetch all table columns in a single query
	allTablesColumnsInfo, err := yb.getAllTableColumnsInfo(tableList)
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching table columns: %w", err)
	}

	for _, tableName := range tableList {
		columnInfo := allTablesColumnsInfo[tableName]
		columns := columnInfo.Columns
		dataTypes := columnInfo.DataTypes

		var supportedColumnNames []string
		var unsupportedColumnNames []string
		for i, column := range columns {
			//Using this ContainsAnyStringFromSlice as the catalog we use for fetching datatypes uses the data_type only
			// which just contains the base type for example VARCHARs it won't include any length, precision or scale information
			//of these types there are other columns available for these information so we just do string match of types with our list
			if utils.ContainsAnyStringFromSlice(unsupportedDatatypesList, dataTypes[i]) {
				unsupportedColumnNames = append(unsupportedColumnNames, column)
			} else {
				supportedColumnNames = append(supportedColumnNames, column)
			}
		}
		if len(supportedColumnNames) == len(columns) {
			supportedTableColumnsMap.Put(tableName, []string{"*"})
		} else {
			supportedTableColumnsMap.Put(tableName, supportedColumnNames)
			if len(unsupportedColumnNames) > 0 {
				unsupportedTableColumnsMap.Put(tableName, unsupportedColumnNames)
			}
		}
	}

	return supportedTableColumnsMap, unsupportedTableColumnsMap, nil
}

// getAllTableColumnsInfo fetches column information for all tables in a single database query
func (yb *YugabyteDB) getAllTableColumnsInfo(tableList []sqlname.NameTuple) (map[sqlname.NameTuple]tableColumnInfo, error) {
	var result = make(map[sqlname.NameTuple]tableColumnInfo)
	if len(tableList) == 0 {
		return result, nil
	}

	// Build IN clause AND create reverse lookup map for table name to NameTuple
	var tableTuples []string
	// Key: "schema.table" -> Value: original NameTuple
	tableLookup := make(map[string]sqlname.NameTuple)

	for _, table := range tableList {
		schema, name := table.ForCatalogQuery()
		tableTuples = append(tableTuples, fmt.Sprintf("('%s', '%s')", schema, name))

		lookupKey := table.AsQualifiedCatalogName()
		tableLookup[lookupKey] = table
	}
	inClause := strings.Join(tableTuples, ", ")
	query := fmt.Sprintf(`
		SELECT
			n.nspname AS table_schema,
			c.relname AS table_name,
			a.attname AS column_name,
			-- Qualify only composite types (UDTs) to distinguish same-named types across schemas.
			-- Keep other types unqualified (hstore, int4, etc.) to match unsupported types list.
			CASE
				WHEN t.typtype = 'c' THEN type_n.nspname || '.' || t.typname
				ELSE t.typname
			END AS data_type
		FROM pg_attribute AS a
		JOIN pg_type AS t ON t.oid = a.atttypid
		JOIN pg_namespace AS type_n ON type_n.oid = t.typnamespace
		JOIN pg_class AS c ON c.oid = a.attrelid
		JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE (n.nspname, c.relname) IN (%s)
			AND a.attname NOT IN ('tableoid', 'cmax', 'xmax', 'cmin', 'xmin', 'ctid')
			AND a.attnum > 0
			AND NOT a.attisdropped
		ORDER BY n.nspname, c.relname, a.attnum; -- attnum ensures columns appear in table definition order and keeps Columns[] and DataTypes[] arrays aligned deterministically
	`, inClause)
	rows, err := yb.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying table columns: %w", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()

	for rows.Next() {
		var schema, table, column, dataType string
		if err := rows.Scan(&schema, &table, &column, &dataType); err != nil {
			return nil, fmt.Errorf("error scanning column info: %w", err)
		}

		lookupKey := fmt.Sprintf("%s.%s", schema, table)
		matchingTable, exists := tableLookup[lookupKey]
		if !exists {
			// This shouldn't happen, but handle gracefully
			log.Warnf("Received column info for unexpected table: %s.%s", schema, table)
			continue
		}

		info := result[matchingTable]
		info.Columns = append(info.Columns, column)
		info.DataTypes = append(info.DataTypes, dataType)
		result[matchingTable] = info
	}

	return result, rows.Err()
}

func (yb *YugabyteDB) ParentTableOfPartition(table sqlname.NameTuple) string {
	var parentTable string
	// For this query in case of case sensitive tables, minquoting is required
	query := fmt.Sprintf(`SELECT inhparent::pg_catalog.regclass
	FROM pg_catalog.pg_class c JOIN pg_catalog.pg_inherits ON c.oid = inhrelid
	WHERE c.oid = '%s'::regclass::oid`, table.ForOutput())

	err := yb.db.QueryRow(query).Scan(&parentTable)
	if err != sql.ErrNoRows && err != nil {
		utils.ErrExit("Error in query for parent tablename of table: %q: %s: %w", query, table, err)
	}

	return parentTable
}

func (yb *YugabyteDB) GetColumnToSequenceMap(tableList []sqlname.NameTuple) map[string]string {
	columnToSequenceMap := make(map[string]string)
	qualifiedTableList := "'" + strings.Join(lo.Map(tableList, func(t sqlname.NameTuple, _ int) string {
		return t.AsQualifiedCatalogName()
	}), "','") + "'"

	// query to find out column name vs sequence name for a table-list
	// this query also covers the case of identity columns

	runQueryAndUpdateMap := func(template string) {
		query := fmt.Sprintf(template, qualifiedTableList)

		var tableName, columeName, sequenceName, schemaName string
		rows, err := yb.db.Query(query)
		if err != nil {
			log.Infof("Query to find column to sequence mapping: %s", query)
			utils.ErrExit("Error in querying for sequences with  query [%v]: %w", query, err)
		}
		defer func() {
			closeErr := rows.Close()
			if closeErr != nil {
				log.Warnf("close rows query %q: %v", query, closeErr)
			}
		}()
		for rows.Next() {
			err := rows.Scan(&tableName, &columeName, &schemaName, &sequenceName)
			if err != nil {
				utils.ErrExit("Error in scanning for sequences query: %s: %w", query, err)
			}
			qualifiedColumnName := fmt.Sprintf("%s.%s", tableName, columeName)
			// quoting sequence name as it can be case sensitive - required during import data restore sequences
			columnToSequenceMap[qualifiedColumnName] = fmt.Sprintf(`%s."%s"`, schemaName, sequenceName)
		}
		err = rows.Close()
		if err != nil {
			utils.ErrExit("close rows query %q: %w", query, err)
		}
	}

	runQueryAndUpdateMap(FETCH_COLUMN_ALL_SEQUENCES_EXCEPT_DEFAULT_QUERY_TEMPLATE)
	runQueryAndUpdateMap(FETCH_COLUMN_SEQUENCES_DEFAULT_QUERY_TEMPLATE)

	return columnToSequenceMap
}

func (yb *YugabyteDB) GetServers() []string {
	var ybServers []string

	YB_SERVERS_QUERY := "SELECT host FROM yb_servers()"
	rows, err := yb.db.Query(YB_SERVERS_QUERY)
	if err != nil {
		utils.ErrExit("error in querying source database for yb_servers: %q: %w\n", YB_SERVERS_QUERY, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", YB_SERVERS_QUERY, closeErr)
		}
	}()
	for rows.Next() {
		var ybServer string
		err = rows.Scan(&ybServer)
		if err != nil {
			utils.ErrExit("error in scanning query rows for yb_servers: %w\n", err)
		}

		ybServers = append(ybServers, ybServer)
	}
	return ybServers
}

func (yb *YugabyteDB) GetPartitions(tableName sqlname.NameTuple) []string {
	partitions := make([]string, 0)
	sname, tname := tableName.ForCatalogQuery()
	query := fmt.Sprintf(`SELECT
    nmsp_child.nspname  AS child_schema,
    child.relname       AS child
FROM pg_inherits
    JOIN pg_class parent            ON pg_inherits.inhparent = parent.oid
    JOIN pg_class child             ON pg_inherits.inhrelid   = child.oid
    JOIN pg_namespace nmsp_parent   ON nmsp_parent.oid  = parent.relnamespace
    JOIN pg_namespace nmsp_child    ON nmsp_child.oid   = child.relnamespace
WHERE parent.relname='%s' AND nmsp_parent.nspname = '%s' `, tname, sname)

	rows, err := yb.db.Query(query)
	if err != nil {
		log.Errorf("failed to list partitions of table %s: query = [ %s ], error = %s", tableName, query, err)
		utils.ErrExit("failed to find the partitions for table: %s: %w", tableName, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()
	for rows.Next() {
		var childSchema, childTable string
		err := rows.Scan(&childSchema, &childTable)
		if err != nil {
			utils.ErrExit("Error in scanning for child partitions of table: %s: %w", tableName, err)
		}
		partitions = append(partitions, fmt.Sprintf(`%s.%s`, childSchema, childTable))
	}
	if rows.Err() != nil {
		utils.ErrExit("Error in scanning for child partitions of table: %s: %w", tableName, rows.Err())
	}
	return partitions
}

// query retrieves all unique columns in the specified tables and schemas, handling both unique constraints and unique indexes, while excluding primary key columns.
const ybQueryTmplForUniqCols = `
WITH unique_constraints AS (
	-- Retrieve columns with unique constraints
    SELECT
        tc.table_schema,
        tc.table_name,
        kcu.column_name
    FROM
        information_schema.table_constraints tc
    JOIN
        information_schema.key_column_usage kcu
        ON tc.constraint_name = kcu.constraint_name
        AND tc.table_schema = kcu.table_schema
        AND tc.table_name = kcu.table_name
    WHERE
        tc.constraint_type = 'UNIQUE'
        AND tc.table_schema = ANY('{%s}')
        AND tc.table_name = ANY('{%s}')
),
unique_indexes AS (
	-- Retrieve columns with unique indexes (excluding primary keys)
    SELECT
        n.nspname AS table_schema,
        t.relname AS table_name,
        a.attname AS column_name
    FROM
        pg_index ix
	-- Join to get table and schema information from the index
    JOIN
        pg_class t ON t.oid = ix.indrelid
    JOIN
        pg_namespace n ON n.oid = t.relnamespace
	-- Join to get column information
    JOIN
        pg_attribute a ON a.attrelid = t.oid 
		AND a.attnum = ANY(ix.indkey)  -- Match indexed columns
	 -- Left join to ensure we exclude primary keys by checking associated constraints
    LEFT JOIN
        pg_constraint c ON ix.indexrelid = c.conindid AND c.contype = 'p'
    WHERE
        ix.indisunique = TRUE
		AND c.contype IS NULL -- Ensure it's not a primary key
        AND n.nspname = ANY('{%s}')
        AND t.relname = ANY('{%s}')
)
-- UNION will remove duplicate rows between unique constraints and unique indexes
SELECT table_schema, table_name, column_name FROM unique_constraints
UNION
SELECT table_schema, table_name, column_name FROM unique_indexes;
`

func (yb *YugabyteDB) GetTableToUniqueKeyColumnsMap(tableList []sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, []string], error) {
	log.Infof("getting unique key columns for tables: %v", tableList)
	result := utils.NewStructMap[sqlname.NameTuple, []string]()
	var querySchemaList, queryTableList []string
	tableStrToNameTupleMap := make(map[string]sqlname.NameTuple)
	for i := 0; i < len(tableList); i++ {
		sname, tname := tableList[i].ForCatalogQuery()
		querySchemaList = append(querySchemaList, sname)
		queryTableList = append(queryTableList, tname)
		tableStrToNameTupleMap[tableList[i].AsQualifiedCatalogName()] = tableList[i]
	}

	querySchemaList = lo.Uniq(querySchemaList)
	query := fmt.Sprintf(ybQueryTmplForUniqCols, strings.Join(querySchemaList, ","), strings.Join(queryTableList, ","),
		strings.Join(querySchemaList, ","), strings.Join(queryTableList, ","))
	log.Infof("query to get unique key columns: %s", query)
	rows, err := yb.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying unique key columns: %w", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()

	for rows.Next() {
		var schemaName, tableName, colName string
		err := rows.Scan(&schemaName, &tableName, &colName)
		if err != nil {
			return nil, fmt.Errorf("scanning row for unique key column name: %w", err)
		}
		tableName = fmt.Sprintf("%s.%s", schemaName, tableName)
		tableNameTuple, ok := tableStrToNameTupleMap[tableName]
		if !ok {
			return nil, goerrors.Errorf("table %s not found in table list", tableName)
		}
		cols, ok := result.Get(tableNameTuple)
		if !ok {
			cols = []string{}
		}
		cols = append(cols, colName)
		result.Put(tableNameTuple, cols)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating over rows for unique key columns: %w", err)
	}
	log.Infof("unique key columns for tables: %v", result)
	return result, nil
}

func (yb *YugabyteDB) ClearMigrationState(migrationUUID uuid.UUID, exportDir string) error {
	log.Infof("ClearMigrationState not implemented yet for YugabyteDB")
	return nil
}

func (yb *YugabyteDB) GetNonPKTables() ([]string, error) {
	var nonPKTables []string
	schemaList := strings.Split(yb.source.Schema, "|")
	querySchemaList := "'" + strings.Join(schemaList, "','") + "'"
	query := fmt.Sprintf(PG_QUERY_TO_CHECK_IF_TABLE_HAS_PK, querySchemaList)
	rows, err := yb.db.Query(query)
	if err != nil {
		utils.ErrExit("error in querying source database for primary key: %q: %w\n", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()
	for rows.Next() {
		var schemaName, tableName string
		var pkCount int
		err := rows.Scan(&schemaName, &tableName, &pkCount)
		if err != nil {
			utils.ErrExit("error in scanning query rows for primary key: %w\n", err)
		}
		table := sqlname.NewSourceName(schemaName, fmt.Sprintf(`"%s"`, tableName))
		if pkCount == 0 {
			nonPKTables = append(nonPKTables, table.Qualified.Quoted)
		}
	}
	return nonPKTables, nil
}

func (yb *YugabyteDB) GetReplicationConnection() (*pgconn.PgConn, error) {
	return pgconn.Connect(context.Background(), yb.getConnectionUri()+"&replication=database")
}

func (yb *YugabyteDB) CreateOrGetLogicalReplicationSlot(conn *pgconn.PgConn, replicationSlotName string) (string, error) {
	exists, err := yb.CheckIfReplicationSlotExists(replicationSlotName)
	if err != nil {
		return "", err
	}
	if exists {
		log.Infof("replication slot %s already exists, skipping creation", replicationSlotName)
		return replicationSlotName, nil
	}

	log.Infof("creating replication slot %s", replicationSlotName)
	res, err := pglogrepl.CreateReplicationSlot(context.Background(), conn, replicationSlotName, "yboutput",
		pglogrepl.CreateReplicationSlotOptions{Mode: pglogrepl.LogicalReplication})
	if err != nil {
		return "", fmt.Errorf("create replication slot: %w", err)
	}

	return res.SlotName, nil
}

func (yb *YugabyteDB) DropLogicalReplicationSlot(conn *pgconn.PgConn, replicationSlotName string) error {
	var err error
	if conn == nil {
		conn, err = yb.GetReplicationConnection()
		if err != nil {
			return fmt.Errorf("failed to create replication connection for dropping replication slot: %w", err)
		}
		defer conn.Close(context.Background())
	}
	log.Infof("dropping replication slot: %s", replicationSlotName)

	// TODO: Remove this sleep and check the status of the slot before dropping
	// This sleep is added to avoid the error "replication slot is active" while dropping the slot since it takes 60 seconds by default to change the status of the slot
	log.Info("waiting for 60 seconds before dropping replication slot...")
	time.Sleep(60 * time.Second)

	err = pglogrepl.DropReplicationSlot(context.Background(), conn, replicationSlotName, pglogrepl.DropReplicationSlotOptions{})
	if err != nil {
		// ignore "does not exist" error while dropping replication slot
		if !strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("delete existing replication slot(%s): %w", replicationSlotName, err)
		}
	}
	return nil
}

func (yb *YugabyteDB) CreatePublication(conn *pgconn.PgConn, publicationName string, tablelistQualifiedQuoted []string) error {
	exists, err := yb.CheckIfPublicationSlotExists(publicationName)
	if err != nil {
		return err
	}
	if exists {
		log.Infof("publication %s already exists, skipping creation", publicationName)
		return nil
	}

	stmt := fmt.Sprintf("CREATE PUBLICATION %s FOR TABLE %s;", publicationName, strings.Join(tablelistQualifiedQuoted, ","))
	result := conn.Exec(context.Background(), stmt)
	_, err = result.ReadAll()
	if err != nil {
		return fmt.Errorf("create publication with stmt %s: %w", stmt, err)
	}
	log.Infof("created publication with stmt %s", stmt)
	return nil
}

func (yb *YugabyteDB) DropPublication(publicationName string) error {
	log.Infof("dropping publication: %s", publicationName)
	res, err := yb.db.Exec(fmt.Sprintf("DROP PUBLICATION IF EXISTS %s", publicationName))
	log.Infof("drop publication result: %v", res)
	if err != nil {
		return fmt.Errorf("drop publication(%s): %w", publicationName, err)
	}
	return nil
}

func (yb *YugabyteDB) CheckIfPublicationSlotExists(publicationName string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = '%s');", publicationName)

	rows, err := yb.db.Query(query)
	if err != nil {
		return false, fmt.Errorf("querying publication existence: %w", err)
	}
	defer rows.Close()

	var exists bool
	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			return false, fmt.Errorf("scanning publication existence: %w", err)
		}
	}

	// Check for any errors encountered during iteration
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("error during rows iteration: %w", err)
	}

	return exists, nil
}

func (yb *YugabyteDB) CheckIfReplicationSlotExists(replicationSlotName string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = '%s');", replicationSlotName)

	rows, err := yb.db.Query(query)
	if err != nil {
		return false, fmt.Errorf("querying replication slot existence: %w", err)
	}
	defer rows.Close()

	var exists bool
	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			return false, fmt.Errorf("scanning replication slot existence: %w", err)
		}
	}

	// Check for any errors encountered during iteration
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("error during rows iteration: %w", err)
	}

	return exists, nil
}

// ---------------------- Guardrails ----------------------

func (yb *YugabyteDB) CheckSourceDBVersion(exportType string) error {
	return nil
}

func (yb *YugabyteDB) GetMissingExportSchemaPermissions(queryTableList string) ([]string, error) {
	return nil, nil
}

func (yb *YugabyteDB) GetMissingExportDataPermissions(exportType string, finalTableList []sqlname.NameTuple) ([]string, error) {
	return nil, nil
}

func (yb *YugabyteDB) GetMissingAssessMigrationPermissions() ([]string, bool, error) {
	return nil, false, nil
}

func (yb *YugabyteDB) CheckIfReplicationSlotsAreAvailable() (isAvailable bool, usedCount int, maxCount int, err error) {
	return checkReplicationSlotsForPGAndYB(yb.db)
}

func (yb *YugabyteDB) GetSchemasMissingUsagePermissions() ([]string, error) {
	return nil, nil
}
