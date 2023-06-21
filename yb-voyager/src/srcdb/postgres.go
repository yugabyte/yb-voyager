package srcdb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/mcuadros/go-version"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const PG_COMMAND_VERSION string = "14.0"

type PostgreSQL struct {
	source *Source

	db *pgx.Conn
}

func newPostgreSQL(s *Source) *PostgreSQL {
	return &PostgreSQL{source: s}
}

func (pg *PostgreSQL) Connect() error {
	db, err := pgx.Connect(context.Background(), pg.getConnectionUri())
	pg.db = db
	return err
}

func (pg *PostgreSQL) Disconnect() {
	if pg.db == nil {
		log.Infof("No connection to the source database to close")
		return
	}

	err := pg.db.Close(context.Background())
	if err != nil {
		log.Infof("Failed to close connection to the source database: %s", err)
	}
}

func (pg *PostgreSQL) CheckRequiredToolsAreInstalled() {
	checkTools("strings")
}

func (pg *PostgreSQL) GetTableRowCount(tableName string) int64 {
	// new conn to avoid conn busy err as multiple parallel(and time-taking) queries possible
	conn, err := pgx.Connect(context.Background(), pg.getConnectionUri())
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

func (pg *PostgreSQL) GetTableApproxRowCount(tableName *sqlname.SourceName) int64 {
	var approxRowCount sql.NullInt64 // handles case: value of the row is null, default for int64 is 0
	query := fmt.Sprintf("SELECT reltuples::bigint FROM pg_class "+
		"where oid = '%s'::regclass", tableName.Qualified.MinQuoted)

	log.Infof("Querying '%s' approx row count of table %q", query, tableName.String())
	err := pg.db.QueryRow(context.Background(), query).Scan(&approxRowCount)
	if err != nil {
		utils.ErrExit("Failed to query %q for approx row count of %q: %s", query, tableName.String(), err)
	}

	log.Infof("Table %q has approx %v rows.", tableName.String(), approxRowCount)
	return approxRowCount.Int64
}

func (pg *PostgreSQL) GetVersion() string {
	var version string
	query := "SELECT setting from pg_settings where name = 'server_version'"
	err := pg.db.QueryRow(context.Background(), query).Scan(&version)
	if err != nil {
		utils.ErrExit("run query %q on source: %s", query, err)
	}
	return version
}

func (pg *PostgreSQL) checkSchemasExists() []string {
	list := strings.Split(pg.source.Schema, "|")
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
	rows, err := pg.db.Query(context.Background(), chkSchemaExistsQuery)
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

func (pg *PostgreSQL) GetAllTableNames() []*sqlname.SourceName {
	schemaList := pg.checkSchemasExists()
	querySchemaList := "'" + strings.Join(schemaList, "','") + "'"
	query := fmt.Sprintf(`SELECT table_schema, table_name
			  FROM information_schema.tables
			  WHERE table_type = 'BASE TABLE' AND
			        table_schema IN (%s);`, querySchemaList)

	rows, err := pg.db.Query(context.Background(), query)
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

func (pg *PostgreSQL) getConnectionUri() string {
	source := pg.source
	if source.Uri != "" {
		return source.Uri
	}
	hostAndPort := fmt.Sprintf("%s:%d", source.Host, source.Port)
	sourceUrl := &url.URL{
		Scheme:   "postgresql",
		User:     url.UserPassword(source.User, source.Password),
		Host:     hostAndPort,
		Path:     source.DBName,
		RawQuery: generateSSLQueryStringIfNotExists(source),
	}

	source.Uri = sourceUrl.String()
	return source.Uri
}

func (pg *PostgreSQL) getConnectionUriWithoutPassword() string {
	source := pg.source
	hostAndPort := fmt.Sprintf("%s:%d", source.Host, source.Port)
	sourceUrl := &url.URL{
		Scheme:   "postgresql",
		User:     url.User(source.User),
		Host:     hostAndPort,
		Path:     source.DBName,
		RawQuery: generateSSLQueryStringIfNotExists(source),
	}
	return sourceUrl.String()
}

func (pg *PostgreSQL) ExportSchema(exportDir string) {
	pg.checkSchemasExists()
	pgdumpExtractSchema(pg.source, pg.getConnectionUriWithoutPassword(), exportDir)
}

func (pg *PostgreSQL) ExportData(ctx context.Context, exportDir string, tableList []*sqlname.SourceName, quitChan chan bool, exportDataStart, exportSuccessChan chan bool, tablesColumnList map[*sqlname.SourceName][]string) {
	pgdumpExportDataOffline(ctx, pg.source, pg.getConnectionUriWithoutPassword(), exportDir, tableList, quitChan, exportDataStart, exportSuccessChan)
}

func (pg *PostgreSQL) ExportDataPostProcessing(exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	renameDataFiles(tablesProgressMetadata)
	dfd := datafile.Descriptor{
		FileFormat:   datafile.TEXT,
		DataFileList: getExportedDataFileList(tablesProgressMetadata),
		Delimiter:    "\t",
		HasHeader:    false,
		ExportDir:    exportDir,
	}
	dfd.Save()
}

// Given a PG command name ("pg_dump", "pg_restore"), find absolute path of
// the executable file having version >= `PG_COMMAND_VERSION`.
func GetAbsPathOfPGCommand(cmd string) (string, error) {
	paths, err := findAllExecutablesInPath(cmd)
	if err != nil {
		err = fmt.Errorf("error in finding executables: %w", err)
		return "", err
	}
	if len(paths) == 0 {
		err = fmt.Errorf("the command %v is not installed", cmd)
		return "", err
	}

	for _, path := range paths {
		cmd := exec.Command(path, "--version")
		stdout, err := cmd.Output()
		if err != nil {
			err = fmt.Errorf("error in finding version of %v from path %v: %w", cmd, path, err)
			return "", err
		}

		// example output centos: pg_restore (PostgreSQL) 14.5
		// example output Ubuntu: pg_dump (PostgreSQL) 14.5 (Ubuntu 14.5-1.pgdg22.04+1)
		currVersion := strings.Fields(string(stdout))[2]

		if version.CompareSimple(currVersion, PG_COMMAND_VERSION) >= 0 {
			return path, nil
		}
	}

	err = fmt.Errorf("could not find %v with version greater than or equal to %v", cmd, PG_COMMAND_VERSION)
	return "", err
}

// GetAllSequences returns all the sequence names in the database for the given schema list
func (pg *PostgreSQL) GetAllSequences() []string {
	schemaList := pg.checkSchemasExists()
	querySchemaList := "'" + strings.Join(schemaList, "','") + "'"
	var sequenceNames []string
	query := fmt.Sprintf(`SELECT sequence_name FROM information_schema.sequences where sequence_schema IN (%s);`, querySchemaList)
	rows, err := pg.db.Query(context.Background(), query)
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

func (pg *PostgreSQL) GetCharset() (string, error) {
	query := fmt.Sprintf("SELECT pg_encoding_to_char(encoding) FROM pg_database WHERE datname = '%s';", pg.source.DBName)
	encoding := ""
	err := pg.db.QueryRow(context.Background(), query).Scan(&encoding)
	if err != nil {
		return "", fmt.Errorf("error in querying database encoding: %w", err)
	}
	return encoding, nil
}

func (pg *PostgreSQL) FilterUnsupportedTables(tableList []*sqlname.SourceName, useDebezium bool) ([]*sqlname.SourceName, []*sqlname.SourceName) {
	return tableList, nil
}

func (pg *PostgreSQL) FilterEmptyTables(tableList []*sqlname.SourceName) ([]*sqlname.SourceName, []*sqlname.SourceName) {
	var nonEmptyTableList, emptyTableList []*sqlname.SourceName
	for _, tableName := range tableList {
		query := fmt.Sprintf(`SELECT false FROM %s LIMIT 1;`, tableName.Qualified.MinQuoted)
		var empty bool
		err := pg.db.QueryRow(context.Background(), query).Scan(&empty)
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

func (pg *PostgreSQL) GetTableColumns(tableName *sqlname.SourceName) ([]string, []string, []string) {
	return nil, nil, nil
}

func (pg *PostgreSQL) GetColumnsWithSupportedTypes(tableList []*sqlname.SourceName, useDebezium bool) (map[*sqlname.SourceName][]string, []string) {
	return nil, nil
}

func (pg *PostgreSQL) IsTablePartition(table *sqlname.SourceName) bool {
	var parentTable string
	// For this query in case of case sensitive tables, minquoting is required
	query := fmt.Sprintf(`SELECT inhparent::pg_catalog.regclass
	FROM pg_catalog.pg_class c JOIN pg_catalog.pg_inherits ON c.oid = inhrelid
	WHERE c.oid = '%s'::regclass::oid`, table.Qualified.MinQuoted)

	err := pg.db.QueryRow(context.Background(), query).Scan(&parentTable)
	if err != pgx.ErrNoRows && err != nil {
		utils.ErrExit("Error in query=%s for parent tablename of table=%s: %v", query, table, err)
	}

	return parentTable != ""
}

func (pg *PostgreSQL) GetColumnToSequenceMap(tableList []*sqlname.SourceName) map[string]string {
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
		rows, err := pg.db.Query(context.Background(), query)
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
