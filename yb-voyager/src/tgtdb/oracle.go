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
package tgtdb

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/sqlldr"
	tgtdbsuite "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/suites"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"golang.org/x/exp/slices"
)

type TargetOracleDB struct {
	sync.Mutex
	tconf *TargetConf
	oraDB *sql.DB
	conn  *sql.Conn
}

func newTargetOracleDB(tconf *TargetConf) TargetDB {
	return &TargetOracleDB{tconf: tconf}
}

func (tdb *TargetOracleDB) connect() error {
	conn, err := tdb.oraDB.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("connect to target db: %w", err)
	}
	tdb.setTargetSchema(conn)
	tdb.conn = conn
	return err
}

func (tdb *TargetOracleDB) Init() error {
	db, err := sql.Open("godror", tdb.getConnectionUri(tdb.tconf))
	if err != nil {
		return fmt.Errorf("open connection to target db: %w", err)
	}
	tdb.oraDB = db

	err = tdb.connect()
	if err != nil {
		return err
	}
	tdb.tconf.Schema = strings.ToUpper(tdb.tconf.Schema)
	checkSchemaExistsQuery := fmt.Sprintf(
		"SELECT 1 FROM ALL_USERS WHERE USERNAME = '%s'",
		tdb.tconf.Schema)
	var cntSchemaName int
	if err = tdb.conn.QueryRowContext(context.Background(), checkSchemaExistsQuery).Scan(&cntSchemaName); err != nil {
		err = fmt.Errorf("run query %q on target %q to check schema exists: %s", checkSchemaExistsQuery, tdb.tconf.Host, err)
	} else if cntSchemaName == 0 {
		err = fmt.Errorf("schema '%s' does not exist in target", tdb.tconf.Schema)
	}
	return err
}

func (tdb *TargetOracleDB) Query(query string) (Rows, error) {
	rows, err := tdb.conn.QueryContext(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("run query %q on oracle %s: %s", query, tdb.tconf.Host, err)
	}
	return &sqlRowsAdapter{rows: rows}, nil
}

func (tdb *TargetOracleDB) QueryRow(query string) Row {
	row := tdb.conn.QueryRowContext(context.Background(), query)
	return row
}

func (tdb *TargetOracleDB) Exec(query string) (int64, error) {
	res, err := tdb.conn.ExecContext(context.Background(), query)
	if err != nil {
		return 0, fmt.Errorf("run query %q on oracle %s: %s", query, tdb.tconf.Host, err)
	}
	rowsAffected, _ := res.RowsAffected()
	return rowsAffected, nil
}

func (tdb *TargetOracleDB) disconnect() {
	if tdb.conn != nil {
		log.Infof("No connection to the target database to close")
	}

	err := tdb.conn.Close()
	if err != nil {
		log.Errorf("Failed to close connection to the target database: %v", err)
	}
	tdb.conn = nil
}

func (tdb *TargetOracleDB) GetConnection() *sql.Conn {
	if tdb.conn == nil {
		utils.ErrExit("Called target db GetConnection() before Init()")
	}
	return tdb.conn
}

func (tdb *TargetOracleDB) Finalize() {
	tdb.disconnect()
}

func (tdb *TargetOracleDB) getTargetSchemaName(tableName string) string {
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		return parts[0]
	}
	return tdb.tconf.Schema
}

func (tdb *TargetOracleDB) CleanFileImportState(filePath, tableName string) error {
	// Delete all entries from ${BATCH_METADATA_TABLE_NAME} for the given file.
	schemaName := tdb.getTargetSchemaName(tableName)
	cmd := fmt.Sprintf(
		`DELETE FROM %s WHERE data_file_name = '%s' AND schema_name = '%s' AND table_name = '%s'`,
		BATCH_METADATA_TABLE_NAME, filePath, schemaName, tableName)
	res, err := tdb.conn.ExecContext(context.Background(), cmd)
	if err != nil {
		return fmt.Errorf("remove %q related entries from %s: %w", tableName, BATCH_METADATA_TABLE_NAME, err)
	}
	rowsAffected, _ := res.RowsAffected()
	log.Infof("query: [%s] => rows affected %v", cmd, rowsAffected)
	return nil
}

func (tdb *TargetOracleDB) GetVersion() string {
	var version string
	query := "SELECT BANNER FROM V$VERSION"
	// query sample output: Oracle Database 19c Enterprise Edition Release 19.0.0.0.0 - Production
	err := tdb.conn.QueryRowContext(context.Background(), query).Scan(&version)
	if err != nil {
		utils.ErrExit("run query %q on source: %s", query, err)
	}
	return version
}

func (tdb *TargetOracleDB) CreateVoyagerSchema() error {
	return nil
}

func (tdb *TargetOracleDB) clearMigrationStateFromTable(conn *sql.Conn, tableName string, migrationUUID uuid.UUID) error {
	stmt := fmt.Sprintf("DELETE FROM %s where migration_uuid='%s'", tableName, migrationUUID)
	res, err := conn.ExecContext(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error executing stmt - %v: %w", stmt, err)
	}
	rowsAffected, _ := res.RowsAffected()
	log.Infof("Query: %s ==> Rows affected: %d", stmt, rowsAffected)
	return nil
}

func (tdb *TargetOracleDB) getEventChannelsRowCount(conn *sql.Conn, migrationUUID uuid.UUID) (int64, error) {
	rowsStmt := fmt.Sprintf(
		"SELECT count(*) FROM %s where migration_uuid='%s'", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID)
	var rowCount int64
	err := conn.QueryRowContext(context.Background(), rowsStmt).Scan(&rowCount)
	if err != nil {
		return 0, fmt.Errorf("error executing stmt - %v: %w", rowsStmt, err)
	}
	return rowCount, nil
}

func (tdb *TargetOracleDB) getLiveMigrationMetaInfoByTable(conn *sql.Conn, migrationUUID uuid.UUID, tableName string) (int64, error) {
	var rowCount int64
	rowsStmt := fmt.Sprintf(
		"SELECT count(*) FROM %s where migration_uuid='%s' AND table_name='%s'",
		EVENTS_PER_TABLE_METADATA_TABLE_NAME, migrationUUID, tableName)
	err := conn.QueryRowContext(context.Background(), rowsStmt).Scan(&rowCount)
	if err != nil {
		return 0, fmt.Errorf("error executing stmt - %v: %w", rowsStmt, err)
	}
	return rowCount, nil
}

func (tdb *TargetOracleDB) initChannelMetaInfo(conn *sql.Conn, migrationUUID uuid.UUID, numChans int) error {
	// if there are >0 rows, then skip because already been inited.
	rowCount, err := tdb.getEventChannelsRowCount(conn, migrationUUID)
	if err != nil {
		return fmt.Errorf("error getting channels meta info for %s: %w", EVENT_CHANNELS_METADATA_TABLE_NAME, err)
	}
	if rowCount > 0 {
		log.Info("event channels meta info already created. Skipping init.")
		return nil
	}
	ctx := context.Background()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error creating tx: %w", err)
	}
	defer tx.Rollback()
	for c := 0; c < numChans; c++ {
		insertStmt := fmt.Sprintf("INSERT INTO %s VALUES ('%s', %d, -1, %d, %d, %d)", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID, c, 0, 0, 0)
		_, err := tx.Exec(insertStmt)
		if err != nil {
			return fmt.Errorf("error executing stmt - %v: %w", insertStmt, err)
		}
		log.Infof("created channels meta info: %s;", insertStmt)
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing tx: %w", err)
	}
	return nil
}

func (tdb *TargetOracleDB) initEventStatByTableMetainfo(tableNames []string, migrationUUID uuid.UUID, conn *sql.Conn, numChans int) error {

	ctx := context.Background()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error creating tx: %w", err)
	}
	defer tx.Rollback()
	for _, tableName := range tableNames {
		// for handling case-sensitive tablenames as case-insensitive in oracle
		// TODO: need proper logic and handling for case-sensitive tablenames
		tableName = tdb.getTargetSchemaName(tableName) + "." + strings.ToUpper(tableName)
		rowCount, err := tdb.getLiveMigrationMetaInfoByTable(conn, migrationUUID, tableName)
		if err != nil {
			return fmt.Errorf("failed to get table wise meta info: %w", err)
		}
		if rowCount > 0 {
			log.Info("table wise meta info already created. Skipping init.")
		} else {
			for c := 0; c < numChans; c++ {
				insertStmt := fmt.Sprintf("INSERT INTO %s VALUES ('%s', '%s', %d, %d, %d, %d, %d)", EVENTS_PER_TABLE_METADATA_TABLE_NAME, migrationUUID, tableName, c, 0, 0, 0, 0)
				_, err := tx.Exec(insertStmt)
				if err != nil {
					return fmt.Errorf("error executing stmt - %v: %w", insertStmt, err)
				}
				log.Infof("created table wise meta info: %s;", insertStmt)
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing tx: %w", err)
	}
	return nil
}

func (tdb *TargetOracleDB) InitLiveMigrationState(migrationUUID uuid.UUID, numChans int, startClean bool, tableNames []string) error {
	err := tdb.WithConn(func(conn *sql.Conn) (bool, error) {
		if startClean {
			err := tdb.clearMigrationStateFromTable(conn, EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID)
			if err != nil {
				return false, fmt.Errorf("failed to clear live migration meta info: %w", err)
			}
			err = tdb.clearMigrationStateFromTable(conn, EVENTS_PER_TABLE_METADATA_TABLE_NAME, migrationUUID)
			if err != nil {
				return false, fmt.Errorf("failed to clear live migration meta info: %w", err)
			}
		}

		err := tdb.initChannelMetaInfo(conn, migrationUUID, numChans)
		if err != nil {
			return false, fmt.Errorf("failed to init live migration meta info: %w", err)
		}
		err = tdb.initEventStatByTableMetainfo(tableNames, migrationUUID, conn, numChans)
		if err != nil {
			return false, fmt.Errorf("failed to init table wise meta info: %w", err)
		}
		return false, nil
	})
	return err
}

func (tdb *TargetOracleDB) qualifyTableName(tableName string) string {
	if len(strings.Split(tableName, ".")) != 2 {
		tableName = fmt.Sprintf("%s.%s", tdb.tconf.Schema, tableName)
	}
	return tableName
}

func (tdb *TargetOracleDB) GetEventChannelsMetaInfo(migrationUUID uuid.UUID) (map[int]EventChannelMetaInfo, error) {
	metainfo := map[int]EventChannelMetaInfo{}

	query := fmt.Sprintf("SELECT channel_no, last_applied_vsn FROM %s where migration_uuid='%s'", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID)
	rows, err := tdb.conn.QueryContext(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query meta info for channels: %w", err)
	}

	for rows.Next() {
		var chanMetaInfo EventChannelMetaInfo
		err := rows.Scan(&(chanMetaInfo.ChanNo), &(chanMetaInfo.LastAppliedVsn))
		if err != nil {
			return nil, fmt.Errorf("error while scanning rows returned from DB: %w", err)
		}
		metainfo[chanMetaInfo.ChanNo] = chanMetaInfo
	}
	return metainfo, nil
}

func (tdb *TargetOracleDB) GetNonEmptyTables(tables []string) []string {
	result := []string{}

	for _, table := range tables {
		log.Infof("Checking if table %s.%s is empty", tdb.tconf.Schema, table)
		rowCount := 0
		stmt := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", tdb.tconf.Schema, table)
		err := tdb.conn.QueryRowContext(context.Background(), stmt).Scan(&rowCount)
		if err != nil {
			utils.ErrExit("run query %q on target: %s", stmt, err)
		}
		if rowCount > 0 {
			result = append(result, table)
		}
	}

	return result
}

func (tdb *TargetOracleDB) IsNonRetryableCopyError(err error) bool {
	return false
}

// NOTE: TODO support for identity columns sequences
func (tdb *TargetOracleDB) RestoreSequences(sequencesLastVal map[string]int64) error {
	return nil
}

func (tdb *TargetOracleDB) ImportBatch(batch Batch, args *ImportBatchArgs, exportDir string, tableSchema map[string]map[string]string) (int64, error) {
	tdb.Lock()
	defer tdb.Unlock()

	var rowsAffected int64
	var err error
	copyFn := func(conn *sql.Conn) (bool, error) {
		rowsAffected, err = tdb.importBatch(conn, batch, args, exportDir, tableSchema)
		return false, err
	}
	err = tdb.WithConn(copyFn)
	return rowsAffected, err
}

func (tdb *TargetOracleDB) WithConn(fn func(*sql.Conn) (bool, error)) error {
	var err error
	retry := true

	for retry {
		var maxAttempts = 5
		var conn *sql.Conn
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			conn, err = tdb.oraDB.Conn(context.Background())
			if err == nil {
				break
			}

			if attempt < maxAttempts {
				log.Warnf("Connection pool is busy. Sleeping for 2 seconds: %s", err)
				time.Sleep(2 * time.Second)
				continue
			}
		}

		if conn == nil {
			return fmt.Errorf("failed to get connection from target db: %w", err)
		}

		retry, err = fn(conn)
		conn.Close()

		if retry {
			time.Sleep(2 * time.Second)
		}
	}
	return err
}

func (tdb *TargetOracleDB) importBatch(conn *sql.Conn, batch Batch, args *ImportBatchArgs, exportDir string, tableSchema map[string]map[string]string) (rowsAffected int64, err error) {
	var file *os.File
	file, err = batch.Open()
	if err != nil {
		return 0, fmt.Errorf("open batch file %q: %w", batch.GetFilePath(), err)
	}
	defer file.Close()

	//setting the schema so that the table is created in the correct schema
	tdb.setTargetSchema(conn)

	ctx := context.Background()
	var tx *sql.Tx
	tx, err = conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		var err2 error
		if err != nil {
			err2 = tx.Rollback()
			if err2 != nil {
				rowsAffected = 0
				err = fmt.Errorf("rollback transaction: %w (while processing %s)", err2, err)
			}
		} else {
			err2 = tx.Commit()
			if err2 != nil {
				rowsAffected = 0
				err = fmt.Errorf("commit transaction: %w", err2)
			}
		}
	}()

	var alreadyImported bool
	alreadyImported, rowsAffected, err = tdb.isBatchAlreadyImported(tx, batch)
	if err != nil {
		return 0, err
	}
	if alreadyImported {
		return rowsAffected, nil
	}

	tableName := batch.GetTableName()
	sqlldrConfig := args.GetSqlLdrControlFile(tdb.tconf.Schema, tableSchema)
	fileName := filepath.Base(batch.GetFilePath())

	err = sqlldr.CreateSqlldrDir(exportDir)
	if err != nil {
		return 0, err
	}
	var sqlldrControlFilePath string
	sqlldrControlFilePath, err = sqlldr.CreateSqlldrControlFile(exportDir, tableName, sqlldrConfig, fileName)
	if err != nil {
		return 0, err
	}

	var sqlldrLogFilePath string
	var sqlldrLogFile *os.File
	sqlldrLogFilePath, sqlldrLogFile, err = sqlldr.CreateSqlldrLogFile(exportDir, tableName)
	if err != nil {
		return 0, err
	}
	defer sqlldrLogFile.Close()

	user := tdb.tconf.User
	password := tdb.tconf.Password
	connectString := tdb.getConnectionString(tdb.tconf)
	oracleConnectionString := fmt.Sprintf("%s@\"%s\"", user, connectString)
	/*
			reference for sqlldr cli options https://docs.oracle.com/en/database/oracle/oracle-database/19/sutil/oracle-sql-loader-commands.html#GUID-24205A60-E16F-4DBA-AD82-376C401013DF
		    DIRECT=TRUE for using faster mode (direct path)
			NO_INDEX_ERRORS=TRUE for not ignoring index errors
			SKIP=1 for skipping the first row which is the header
			ERRORS=0 for exiting on first error and 0 errors allowed
	*/
	sqlldrArgs := fmt.Sprintf("userid=%s control=%s log=%s DIRECT=TRUE NO_INDEX_ERRORS=TRUE SKIP=1 ERRORS=0",
		oracleConnectionString, sqlldrControlFilePath, sqlldrLogFilePath)

	var outbuf string
	var errbuf string
	outbuf, errbuf, err = sqlldr.RunSqlldr(sqlldrArgs, password)

	if err != nil {
		// for error related to the stdinPipe of created while running sqlldr
		return 0, fmt.Errorf("run sqlldr error: %w %s\nPlease check the log file for more information - %s", err, errbuf, sqlldrLogFilePath)
	}

	var err2 error
	rowsAffected, err2 = getRowsAffected(outbuf)
	if err2 != nil {
		return 0, fmt.Errorf("get rows affected from sqlldr output: %w", err)
	}

	ignoreError := false
	if err != nil {
		log.Infof("sqlldr out:\n%s", outbuf)
		log.Errorf("sqlldr error:\n%s", errbuf)
		// find ORA-00001: unique constraint * violated in log file
		pattern := regexp.MustCompile(`ORA-00001: unique constraint \(.+?\) violated`)
		scanner := bufio.NewScanner(sqlldrLogFile)
		for scanner.Scan() {
			line := scanner.Text()
			if pattern.MatchString(line) {
				ignoreError = true
				break
			}
		}

		if !ignoreError {
			return rowsAffected, fmt.Errorf("run sqlldr: %w", err)
		}
	}

	err = tdb.recordEntryInDB(tx, batch, rowsAffected)
	if err != nil {
		err = fmt.Errorf("record entry in DB for batch %q: %w", batch.GetFilePath(), err)
	}

	return rowsAffected, err
}

func (tdb *TargetOracleDB) recordEntryInDB(tx *sql.Tx, batch Batch, rowsAffected int64) error {
	cmd := batch.GetQueryToRecordEntryInDB(rowsAffected)
	_, err := tx.ExecContext(context.Background(), cmd)
	if err != nil {
		return fmt.Errorf("insert into %s: %w", BATCH_METADATA_TABLE_NAME, err)
	}
	return nil
}

func getRowsAffected(outbuf string) (int64, error) {
	regex := regexp.MustCompile(`Load completed - logical record count (\d+).`)
	matches := regex.FindStringSubmatch(outbuf)
	if len(matches) < 2 {
		return 0, fmt.Errorf("RowsAffected not found in the sqlldr output")
	}
	return strconv.ParseInt(matches[1], 10, 64)
}

func (tdb *TargetOracleDB) isBatchAlreadyImported(tx *sql.Tx, batch Batch) (bool, int64, error) {
	var rowsImported int64
	query := batch.GetQueryIsBatchAlreadyImported()
	err := tx.QueryRowContext(context.Background(), query).Scan(&rowsImported)
	if err == nil {
		log.Infof("%v rows from %q are already imported", rowsImported, batch.GetFilePath())
		return true, rowsImported, nil
	}
	if err == sql.ErrNoRows {
		log.Infof("%q is not imported yet", batch.GetFilePath())
		return false, 0, nil
	}
	return false, 0, fmt.Errorf("check if %s is already imported: %w", batch.GetFilePath(), err)
}

func (tdb *TargetOracleDB) setTargetSchema(conn *sql.Conn) {
	setSchemaQuery := fmt.Sprintf("ALTER SESSION SET CURRENT_SCHEMA = %s", tdb.tconf.Schema)
	_, err := conn.ExecContext(context.Background(), setSchemaQuery)
	if err != nil {
		utils.ErrExit("run query %q on target %q to set schema: %s", setSchemaQuery, tdb.tconf.Host, err)
	}
}

func (tdb *TargetOracleDB) IfRequiredQuoteColumnNames(tableName string, columns []string) ([]string, error) {
	result := make([]string, len(columns))
	// FAST PATH.
	fastPathSuccessful := true
	for i, colName := range columns {
		if strings.ToUpper(colName) == colName {
			if sqlname.IsReservedKeywordOracle(colName) && colName[0:1] != `"` {
				result[i] = fmt.Sprintf(`"%s"`, colName)
			} else {
				result[i] = colName
			}
		} else {
			// Go to slow path.
			log.Infof("column name (%s) is not all upper-case. Going to slow path.", colName)
			result = make([]string, len(columns))
			fastPathSuccessful = false
			break
		}
	}
	if fastPathSuccessful {
		log.Infof("FAST PATH: columns of table %s after quoting: %v", tableName, result)
		return result, nil
	}
	// SLOW PATH.
	schemaName := tdb.tconf.Schema
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		schemaName = parts[0]
		tableName = parts[1]
	}
	targetColumns, err := tdb.getListOfTableAttributes(schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("get list of table attributes: %w", err)
	}
	log.Infof("columns of table %s.%s in target db: %v", schemaName, tableName, targetColumns)
	for i, colName := range columns {
		if colName[0] == '"' && colName[len(colName)-1] == '"' {
			colName = colName[1 : len(colName)-1]
		}
		switch true {
		// TODO: Move sqlname.IsReservedKeywordOracle() in this file.
		case sqlname.IsReservedKeywordOracle(colName):
			result[i] = fmt.Sprintf(`"%s"`, colName)
		case colName == strings.ToUpper(colName): // Name is all Upper case.
			result[i] = colName
		case slices.Contains(targetColumns, colName): // Name is not keyword and is not all uppercase.
			result[i] = fmt.Sprintf(`"%s"`, colName)
		case slices.Contains(targetColumns, strings.ToUpper(colName)): // Case insensitive name given with mixed case.
			result[i] = strings.ToUpper(colName)
		default:
			return nil, fmt.Errorf("column %q not found in table %s", colName, tableName)
		}
	}
	log.Infof("columns of table %s.%s after quoting: %v", schemaName, tableName, result)
	return result, nil
}

func (tdb *TargetOracleDB) getListOfTableAttributes(schemaName string, tableName string) ([]string, error) {
	// TODO: handle case-sensitivity properly
	query := fmt.Sprintf("SELECT column_name FROM all_tab_columns WHERE UPPER(table_name) = UPPER('%s') AND owner = '%s'", tableName, schemaName)
	rows, err := tdb.conn.QueryContext(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query meta info for channels: %w", err)
	}
	var columns []string
	for rows.Next() {
		var column string
		err := rows.Scan(&column)
		if err != nil {
			return nil, fmt.Errorf("error while scanning rows returned from DB: %w", err)
		}
		columns = append(columns, column)
	}
	return columns, nil
}

// execute all events sequentially one by one in a single transaction
func (tdb *TargetOracleDB) ExecuteBatch(migrationUUID uuid.UUID, batch *EventBatch) error {
	// TODO: figure out how to avoid round trips to Oracle DB
	log.Infof("executing batch of %d events", len(batch.Events))
	err := tdb.WithConn(func(conn *sql.Conn) (bool, error) {
		tx, err := conn.BeginTx(context.Background(), nil)
		if err != nil {
			return false, fmt.Errorf("begin transaction: %w", err)
		}
		defer tx.Rollback()

		for i := 0; i < len(batch.Events); i++ {
			event := batch.Events[i]
			stmt := event.GetSQLStmt(tdb.tconf.Schema)
			if event.Op == "c" && tdb.tconf.EnableUpsert {
				// converting to an UPSERT
				event.Op = "u"
				updateStmt := event.GetSQLStmt(tdb.tconf.Schema)
				stmt = fmt.Sprintf("BEGIN %s; EXCEPTION WHEN dup_val_on_index THEN %s; END;", stmt, updateStmt)
				event.Op = "c" // reverting state
			}
			_, err = tx.Exec(stmt)
			if err != nil {
				log.Errorf("error executing stmt for event with vsn(%d) via query-%s: %v", event.Vsn, stmt, err)
				return false, fmt.Errorf("failed to execute stmt for event with vsn(%d) via query-%s: %w", event.Vsn, stmt, err)
			}
		}

		updateVsnQuery := batch.GetChannelMetadataUpdateQuery(migrationUUID)
		res, err := tx.Exec(updateVsnQuery)
		if err != nil {
			log.Errorf("error executing stmt: %v", err)
			return false, fmt.Errorf("failed to update vsn on target db via query-%s: %w", updateVsnQuery, err)
		} else if rowsAffected, err := res.RowsAffected(); rowsAffected == 0 || err != nil {
			log.Errorf("error executing stmt: %v, rowsAffected: %v", err, rowsAffected)
			return false, fmt.Errorf("failed to update vsn on target db via query-%s: %w, rowsAffected: %v",
				updateVsnQuery, err, rowsAffected)
		}

		tableNames := batch.GetTableNames()
		for _, tableName := range tableNames {
			tableName := tdb.qualifyTableName(tableName)
			updatePerTableEvents := batch.GetQueriesToUpdateEventStatsByTable(migrationUUID, tableName)
			res, err = tx.Exec(updatePerTableEvents)
			if err != nil {
				log.Errorf("error executing stmt: %v", err)
				return false, fmt.Errorf("failed to update per table events on target db via query-%s: %w", updatePerTableEvents, err)
			}
			rowsAffected, err := res.RowsAffected()
			if err != nil {
				return false, fmt.Errorf("failed to get number of rows affected in update per table events on target db via query-%s: %w",
					updatePerTableEvents, err)
			}
			if rowsAffected == 0 {
				insertTableStatsQuery := batch.GetQueriesToInsertEventStatsByTable(migrationUUID, tableName)
				_, err = tx.Exec(insertTableStatsQuery)
				if err != nil {
					log.Errorf("error executing stmt: %v ", err)
					return false, fmt.Errorf("failed to insert table stats on target db via query-%s: %w",
						insertTableStatsQuery, err)
				}
			}
		}

		if err = tx.Commit(); err != nil {
			return false, fmt.Errorf("failed to commit transaction : %w", err)
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("error executing batch: %w", err)
	}

	return nil
}

func (tdb *TargetOracleDB) InitConnPool() error {
	if tdb.tconf.Parallelism == -1 {
		tdb.tconf.Parallelism = 1
		log.Infof("Using %d parallel jobs by default. Use --parallel-jobs to specify a custom value", tdb.tconf.Parallelism)
	}
	tdb.oraDB.SetMaxIdleConns(tdb.tconf.Parallelism + 1)
	tdb.oraDB.SetMaxOpenConns(tdb.tconf.Parallelism + 1)
	return nil
}

func (tdb *TargetOracleDB) PrepareForStreaming() {}

func (tdb *TargetOracleDB) GetDebeziumValueConverterSuite() map[string]tgtdbsuite.ConverterFn {
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
	return oraValueConverterSuite
}

func (tdb *TargetOracleDB) getConnectionUri(tconf *TargetConf) string {
	if tconf.Uri != "" {
		return tconf.Uri
	}

	connectString := tdb.getConnectionString(tconf)
	tconf.Uri = fmt.Sprintf(`user="%s" password="%s" connectString="%s"`, tconf.User, tconf.Password, connectString)

	return tconf.Uri
}

func (tdb *TargetOracleDB) getConnectionString(tconf *TargetConf) string {
	var connectString string
	switch true {
	case tconf.DBSid != "":
		connectString = fmt.Sprintf("(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SID=%s)))",
			tconf.Host, tconf.Port, tconf.DBSid)
	case tconf.TNSAlias != "":
		connectString = tconf.TNSAlias
	case tconf.DBName != "":
		connectString = fmt.Sprintf("(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SERVICE_NAME=%s)))",
			tconf.Host, tconf.Port, tconf.DBName)
	}

	return connectString
}

func (tdb *TargetOracleDB) MaxBatchSizeInBytes() int64 {
	return 2 * 1024 * 1024 * 1024 // 2GB
}

func (tdb *TargetOracleDB) GetImportedEventsStatsForTable(tableName string, migrationUUID uuid.UUID) (*EventCounter, error) {
	var eventCounter EventCounter
	// TODO: handle case-sensitive properly for tablenames
	tableName = tdb.getTargetSchemaName(tableName) + "." + strings.ToUpper(tableName)
	var query string
	err := tdb.WithConn(func(conn *sql.Conn) (bool, error) {
		query = fmt.Sprintf(`SELECT SUM(total_events), SUM(num_inserts), SUM(num_updates), SUM(num_deletes) FROM %s 
		WHERE table_name='%s' AND migration_uuid='%s'`, EVENTS_PER_TABLE_METADATA_TABLE_NAME, tableName, migrationUUID)
		err := conn.QueryRowContext(context.Background(), query).Scan(&eventCounter.TotalEvents,
			&eventCounter.NumInserts, &eventCounter.NumUpdates, &eventCounter.NumDeletes)
		return false, err
	})
	if err != nil {
		log.Errorf("error in getting import stats from target db: using query-%s %v", query, err)
		return nil, fmt.Errorf("error in getting import stats from target db: using query-%s %w", query, err)
	}
	log.Infof("import stats for table %s: %v", tableName, eventCounter)
	return &eventCounter, nil
}

func (tdb *TargetOracleDB) GetImportedSnapshotRowCountForTable(tableName string) (int64, error) {
	var snapshotRowCount int64
	schema := tdb.getTargetSchemaName(tableName)
	err := tdb.WithConn(func(conn *sql.Conn) (bool, error) {
		query := fmt.Sprintf(`SELECT SUM(rows_imported) FROM %s where schema_name='%s' AND table_name='%s'`,
			BATCH_METADATA_TABLE_NAME, schema, tableName)
		err := conn.QueryRowContext(context.Background(), query).Scan(&snapshotRowCount)
		if err != nil {
			log.Errorf("error in querying row_imported for snapshot import of table %s: %v", tableName, err)
			return false, fmt.Errorf("error in querying row_imported for snapshot import of table %s: %w", tableName, err)
		}
		return false, nil
	})
	if err != nil {
		log.Errorf("error in getting total row count for snapshot import of table %s: %v", tableName, err)
		return -1, fmt.Errorf("error in getting total row count for snapshot import of table %s: %w", tableName, err)
	}
	log.Infof("total row count for snapshot import of table %s: %d", tableName, snapshotRowCount)
	return snapshotRowCount, nil
}

func (tdb *TargetOracleDB) GetGeneratedAlwaysAsIdentityColumnNamesForTable(table string) ([]string, error) {
	schema := tdb.getTargetSchemaName(table)
	query := fmt.Sprintf(`Select COLUMN_NAME from ALL_TAB_IDENTITY_COLS where OWNER = '%s'
	AND TABLE_NAME = '%s' AND GENERATION_TYPE='ALWAYS'`, schema, table)
	log.Infof("query of identity columns for table(%s): %s", table, query)
	var identityColumns []string
	err := tdb.WithConn(func(conn *sql.Conn) (bool, error) {
		rows, err := conn.QueryContext(context.Background(), query)
		if err != nil {
			if err == sql.ErrNoRows {
				return false, nil
			}
			log.Errorf("querying identity columns: %v", err)
			return false, fmt.Errorf("querying identity columns: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var colName string
			err := rows.Scan(&colName)
			if err != nil {
				log.Errorf("scanning row for identity column name: %v", err)
				return false, fmt.Errorf("scanning row for identity column name: %w", err)
			}
			identityColumns = append(identityColumns, colName)
		}
		return false, nil
	})
	return identityColumns, err
}

func (tdb *TargetOracleDB) DisableGeneratedAlwaysAsIdentityColumns(tableColumnsMap map[string][]string) error {
	log.Infof("disabling generated always as identity columns")
	return tdb.alterColumns(tableColumnsMap, "GENERATED BY DEFAULT AS IDENTITY(START WITH LIMIT VALUE)")
}

func (tdb *TargetOracleDB) EnableGeneratedAlwaysAsIdentityColumns(tableColumnsMap map[string][]string) error {
	log.Infof("enabling generated always as identity columns")
	// Oracle needs start value to resumes the value for further inserts correctly
	return tdb.alterColumns(tableColumnsMap, "GENERATED ALWAYS AS IDENTITY(START WITH LIMIT VALUE)")
}

func (tdb *TargetOracleDB) alterColumns(tableColumnsMap map[string][]string, alterAction string) error {
	for table, columns := range tableColumnsMap {
		qualifiedTblName := tdb.qualifyTableName(table)
		for _, column := range columns {
			// LIMIT VALUE - ensures that start it is set to the current value of the sequence
			query := fmt.Sprintf(`ALTER TABLE %s MODIFY %s %s`, qualifiedTblName, column, alterAction)
			err := tdb.WithConn(func(conn *sql.Conn) (bool, error) {
				_, err := conn.ExecContext(context.Background(), query)
				if err != nil {
					log.Errorf("executing query-%s to alter column(%s) for table(%s): %v", query, column, qualifiedTblName, err)
					return false, fmt.Errorf("executing query to alter column for table(%s): %w", qualifiedTblName, err)
				}
				return false, nil
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
