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

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type YugabyteDB struct {
	source *Source

	db *pgx.Conn
}

func newYugabyteDB(s *Source) *YugabyteDB {
	return &YugabyteDB{source: s}
}

func (yb *YugabyteDB) Connect() error {
	db, err := pgx.Connect(context.Background(), yb.getConnectionUri())
	yb.db = db
	return err
}

func (yb *YugabyteDB) Disconnect() {
	if yb.db == nil {
		log.Infof("No connection to the source database to close")
		return
	}

	err := yb.db.Close(context.Background())
	if err != nil {
		log.Infof("Failed to close connection to the source database: %s", err)
	}
}

func (yb *YugabyteDB) CheckRequiredToolsAreInstalled() {
	checkTools("strings")
}

func (yb *YugabyteDB) GetTableRowCount(tableName string) int64 {
	// new conn to avoid conn busy err as multiple parallel(and time-taking) queries possible
	conn, err := pgx.Connect(context.Background(), yb.getConnectionUri())
	if err != nil {
		utils.ErrExit("Failed to connect to the source database for table row count: %s", err)
	}
	defer conn.Close(context.Background())

	var rowCount int64
	query := fmt.Sprintf("select count(*) from %s", tableName)
	log.Infof("Querying row count of table %q", tableName)
	err = conn.QueryRow(context.Background(), query).Scan(&rowCount)
	if err != nil {
		utils.ErrExit("Failed to query %q for row count of %q: %s", query, tableName, err)
	}
	log.Infof("Table %q has %v rows.", tableName, rowCount)
	return rowCount
}

func (yb *YugabyteDB) GetTableApproxRowCount(tableName *sqlname.SourceName) int64 {
	var approxRowCount sql.NullInt64 // handles case: value of the row is null, default for int64 is 0
	query := fmt.Sprintf("SELECT reltuples::bigint FROM pg_class "+
		"where oid = '%s'::regclass", tableName.Qualified.MinQuoted)

	log.Infof("Querying '%s' approx row count of table %q", query, tableName.String())
	err := yb.db.QueryRow(context.Background(), query).Scan(&approxRowCount)
	if err != nil {
		utils.ErrExit("Failed to query %q for approx row count of %q: %s", query, tableName.String(), err)
	}

	log.Infof("Table %q has approx %v rows.", tableName.String(), approxRowCount)
	return approxRowCount.Int64
}

func (yb *YugabyteDB) GetVersion() string {
	var version string
	query := "SELECT setting from pg_settings where name = 'server_version'"
	err := yb.db.QueryRow(context.Background(), query).Scan(&version)
	if err != nil {
		utils.ErrExit("run query %q on source: %s", query, err)
	}
	return version
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
	rows, err := yb.db.Query(context.Background(), chkSchemaExistsQuery)
	if err != nil {
		utils.ErrExit("error in querying(%q) source database for checking mentioned schema(s) present or not: %v\n", chkSchemaExistsQuery, err)
	}
	var listOfSchemaPresent []string
	var tableSchemaName string

	for rows.Next() {
		err = rows.Scan(&tableSchemaName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for schema names: %v\n", err)
		}
		listOfSchemaPresent = append(listOfSchemaPresent, tableSchemaName)
	}
	defer rows.Close()

	schemaNotPresent := utils.SetDifference(trimmedList, listOfSchemaPresent)
	if len(schemaNotPresent) > 0 {
		utils.ErrExit("Following schemas are not present in source database %v, please provide a valid schema list.\n", schemaNotPresent)
	}
	return trimmedList
}

func (yb *YugabyteDB) GetAllTableNames() []*sqlname.SourceName {
	schemaList := yb.checkSchemasExists()
	querySchemaList := "'" + strings.Join(schemaList, "','") + "'"
	query := fmt.Sprintf(`SELECT table_schema, table_name
			  FROM information_schema.tables
			  WHERE table_type = 'BASE TABLE' AND
			        table_schema IN (%s);`, querySchemaList)

	rows, err := yb.db.Query(context.Background(), query)
	if err != nil {
		utils.ErrExit("error in querying(%q) source database for table names: %v\n", query, err)
	}
	defer rows.Close()

	var tableNames []*sqlname.SourceName
	var tableName, tableSchema string

	for rows.Next() {
		err = rows.Scan(&tableSchema, &tableName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for table names: %v\n", err)
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

func (yb *YugabyteDB) getConnectionUriWithoutPassword() string {
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

func (yb *YugabyteDB) ExportSchema(exportDir string) {
	panic("not implemented")
}

func (yb *YugabyteDB) ExportData(ctx context.Context, exportDir string, tableList []*sqlname.SourceName, quitChan chan bool, exportDataStart, exportSuccessChan chan bool, tablesColumnList map[*sqlname.SourceName][]string) {
	pgdumpExportDataOffline(ctx, yb.source, yb.getConnectionUriWithoutPassword(), exportDir, tableList, quitChan, exportDataStart, exportSuccessChan)
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
		utils.ErrExit("error in reading toc file: %v\n", err)
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
	rows, err := yb.db.Query(context.Background(), query)
	if err != nil {
		utils.ErrExit("error in querying(%q) source database for sequence names: %v\n", query, err)
	}
	defer rows.Close()

	var sequenceName string
	for rows.Next() {
		err = rows.Scan(&sequenceName)
		if err != nil {
			utils.ErrExit("error in scanning query rows for sequence names: %v\n", err)
		}
		sequenceNames = append(sequenceNames, sequenceName)
	}
	return sequenceNames
}

func (yb *YugabyteDB) GetCharset() (string, error) {
	query := fmt.Sprintf("SELECT pg_encoding_to_char(encoding) FROM pg_database WHERE datname = '%s';", yb.source.DBName)
	encoding := ""
	err := yb.db.QueryRow(context.Background(), query).Scan(&encoding)
	if err != nil {
		return "", fmt.Errorf("error in querying database encoding: %w", err)
	}
	return encoding, nil
}

func (yb *YugabyteDB) FilterUnsupportedTables(tableList []*sqlname.SourceName, useDebezium bool) ([]*sqlname.SourceName, []*sqlname.SourceName) {
	return tableList, nil
}

func (yb *YugabyteDB) FilterEmptyTables(tableList []*sqlname.SourceName) ([]*sqlname.SourceName, []*sqlname.SourceName) {
	var nonEmptyTableList, emptyTableList []*sqlname.SourceName
	for _, tableName := range tableList {
		query := fmt.Sprintf(`SELECT false FROM %s LIMIT 1;`, tableName.Qualified.MinQuoted)
		var empty bool
		err := yb.db.QueryRow(context.Background(), query).Scan(&empty)
		if err != nil {
			if err == pgx.ErrNoRows {
				empty = true
			} else {
				utils.ErrExit("error in querying table %v: %v", tableName, err)
			}
		}
		if !empty {
			nonEmptyTableList = append(nonEmptyTableList, tableName)
		} else {
			emptyTableList = append(emptyTableList, tableName)
		}
	}
	return nonEmptyTableList, emptyTableList
}

func (yb *YugabyteDB) GetTableColumns(tableName *sqlname.SourceName) ([]string, []string, []string) {
	return nil, nil, nil
}

func (yb *YugabyteDB) GetColumnsWithSupportedTypes(tableList []*sqlname.SourceName, useDebezium bool) (map[*sqlname.SourceName][]string, []string) {
	return nil, nil
}

func (yb *YugabyteDB) IsTablePartition(table *sqlname.SourceName) bool {
	var parentTable string
	// For this query in case of case sensitive tables, minquoting is required
	query := fmt.Sprintf(`SELECT inhparent::pg_catalog.regclass
	FROM pg_catalog.pg_class c JOIN pg_catalog.pg_inherits ON c.oid = inhrelid
	WHERE c.oid = '%s'::regclass::oid`, table.Qualified.MinQuoted)

	err := yb.db.QueryRow(context.Background(), query).Scan(&parentTable)
	if err != pgx.ErrNoRows && err != nil {
		utils.ErrExit("Error in query=%s for parent tablename of table=%s: %v", query, table, err)
	}

	return parentTable != ""
}

func (yb *YugabyteDB) GetColumnToSequenceMap(tableList []*sqlname.SourceName) map[string]string {
	columnToSequenceMap := make(map[string]string)
	for _, table := range tableList {
		// query to find out column name vs sequence name for a table
		// this query also covers the case of identity columns
		query := fmt.Sprintf(`SELECT a.attname AS column_name, s.relname AS sequence_name,
		s.relnamespace::pg_catalog.regnamespace::text AS schema_name
		FROM pg_class AS t
			JOIN pg_attribute AS a
		ON a.attrelid = t.oid
			JOIN pg_depend AS d
		ON d.refobjid = t.oid
			AND d.refobjsubid = a.attnum
			JOIN pg_class AS s
		ON s.oid = d.objid
		WHERE d.classid = 'pg_catalog.pg_class'::regclass
		AND d.refclassid = 'pg_catalog.pg_class'::regclass
		AND d.deptype IN ('i', 'a')
		AND t.relkind IN ('r', 'P')
		AND s.relkind = 'S'
		AND t.oid = '%s'::regclass;`, table.Qualified.MinQuoted)

		var columeName, sequenceName, schemaName string
		rows, err := yb.db.Query(context.Background(), query)
		if err != nil {
			log.Infof("Query to find column to sequence mapping: %s", query)
			utils.ErrExit("Error in querying for sequences in table=%s: %v", table, err)
		}
		for rows.Next() {
			err := rows.Scan(&columeName, &sequenceName, &schemaName)
			if err != nil {
				utils.ErrExit("Error in scanning for sequences in table=%s: %v", table, err)
			}
			qualifiedColumnName := fmt.Sprintf("%s.%s", table.Qualified.Unquoted, columeName)
			columnToSequenceMap[qualifiedColumnName] = schemaName + "." + sequenceName
		}
	}

	return columnToSequenceMap
}

func GetStreamID() string {
	return "25eb4bb5da1541f8be6bb1a4c473971a" 
	//TODO: create stream id for db using yb-admin client 
 	//TODO: see if 4hr retention period can cause any issue then need to recreate stream id
}

func (yb *YugabyteDB) GetServers() string {
	var ybServers []string
	YB_SERVERS_QUERY := "SELECT host FROM yb_servers()"

	rows, err := yb.db.Query(context.Background(), YB_SERVERS_QUERY)
	if err != nil {
		utils.ErrExit("error in querying(%q) source database for yb_servers: %v\n", YB_SERVERS_QUERY, err)
	}
	defer rows.Close()
	for rows.Next() {
		var ybServer string
		err = rows.Scan(&ybServer)
		if err != nil {
			utils.ErrExit("error in scanning query rows for yb_servers: %v\n", err)
		}
		ybServers = append(ybServers, fmt.Sprintf("%s:7100", ybServer)) //TODO: check if master port is configurable?
	}
	return strings.Join(ybServers, ",")
}
