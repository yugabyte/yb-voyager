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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var oraValueConverterSuite = map[string]ConverterFn{
	"STRING": func(columnValue string, formatIfRequired bool) (string, error) {
		if formatIfRequired {
			formattedColumnValue := strings.Replace(columnValue, "'", "''", -1)
			return fmt.Sprintf("'%s'", formattedColumnValue), nil
		} else {
			return columnValue, nil
		}
	},
}

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

	checkSchemaExistsQuery := fmt.Sprintf(
		"SELECT 1 FROM ALL_USERS WHERE USERNAME = '%s'",
		strings.ToUpper(tdb.tconf.Schema))
	var cntSchemaName int
	if err = tdb.conn.QueryRowContext(context.Background(), checkSchemaExistsQuery).Scan(&cntSchemaName); err != nil {
		err = fmt.Errorf("run query %q on target %q to check schema exists: %s", checkSchemaExistsQuery, tdb.tconf.Host, err)
	} else if cntSchemaName == 0 {
		err = fmt.Errorf("schema '%s' does not exist in target", tdb.tconf.Schema)
	}
	return err
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

func (tdb *TargetOracleDB) reconnect() error {
	tdb.Mutex.Lock()
	defer tdb.Mutex.Unlock()

	var err error
	tdb.disconnect()
	for attempt := 1; attempt < 5; attempt++ {
		err = tdb.connect()
		if err == nil {
			return nil
		}
		log.Warnf("Attempt %d: Failed to reconnect to the target database: %s", attempt, err)
		time.Sleep(time.Duration(attempt*2) * time.Second)
		// Retry.
	}
	return fmt.Errorf("reconnect to target db: %w", err)
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
	createBatchMetadataTableQuery := fmt.Sprintf(`BEGIN
		EXECUTE IMMEDIATE 'CREATE TABLE %s (
			data_file_name VARCHAR2(250),
			batch_number NUMBER(10),
			schema_name VARCHAR2(250),
			table_name VARCHAR2(250),
			rows_imported NUMBER(19),
			PRIMARY KEY (data_file_name, batch_number, schema_name, table_name)
		)';
	EXCEPTION
		WHEN OTHERS THEN
			IF SQLCODE != -955 THEN
				RAISE;
			END IF;
	END;`, BATCH_METADATA_TABLE_NAME)
	// The exception block is to ignore the error if the table already exists and continue without error.
	createEventChannelsMetadataTableQuery := fmt.Sprintf(`BEGIN
		EXECUTE IMMEDIATE 'CREATE TABLE %s (
			migration_uuid VARCHAR2(36),
			channel_no INT,
			last_applied_vsn NUMBER(19),
			PRIMARY KEY (migration_uuid, channel_no)
		)';
	EXCEPTION
		WHEN OTHERS THEN
			IF SQLCODE != -955 THEN
				RAISE;
			END IF;
	END;`, EVENT_CHANNELS_METADATA_TABLE_NAME)

	cmds := []string{
		createBatchMetadataTableQuery,
		createEventChannelsMetadataTableQuery,
	}

	maxAttempts := 12
	var err error

outer:
	for _, cmd := range cmds {
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			log.Infof("Executing on target: [%s]", cmd)
			conn := tdb.GetConnection()
			_, err = conn.ExecContext(context.Background(), cmd)
			if err == nil {
				// No error. Move on to the next command.
				continue outer
			}
			log.Warnf("Error while running [%s] attempt %d: %s", cmd, attempt, err)
			time.Sleep(5 * time.Second)
			err2 := tdb.reconnect()
			if err2 != nil {
				return fmt.Errorf("reconnect to target db: %w", err2)
			}
		}
		if err != nil {
			return fmt.Errorf("create ybvoyager schema on target: %w", err)
		}
	}
	return nil
}

func (tdb *TargetOracleDB) InitEventChannelsMetaInfo(migrationUUID uuid.UUID, numChans int, startClean bool) error {
	err := tdb.WithConn(func(conn *sql.Conn) (bool, error) {
		if startClean {
			startCleanStmt := fmt.Sprintf("DELETE FROM %s where migration_uuid='%s'", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID)
			res, err := conn.ExecContext(context.Background(), startCleanStmt)
			if err != nil {
				return false, fmt.Errorf("error executing stmt - %v: %w", startCleanStmt, err)
			}
			rowsAffected, _ := res.RowsAffected()
			log.Infof("deleted existing channels meta info using query %s; rows affected = %d", startCleanStmt, rowsAffected)
		}
		// if there are >0 rows, then skip because already been inited.
		rowsStmt := fmt.Sprintf(
			"SELECT count(*) FROM %s where migration_uuid='%s'", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID)
		var rowCount int
		err := conn.QueryRowContext(context.Background(), rowsStmt).Scan(&rowCount)
		if err != nil {
			return false, fmt.Errorf("error executing stmt - %v: %w", rowsStmt, err)
		}
		if rowCount > 0 {
			log.Info("event channels meta info already created. Skipping init.")
			return false, nil
		}

		for c := 0; c < numChans; c++ {
			insertStmt := fmt.Sprintf("INSERT INTO %s VALUES ('%s', %d, -1)", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID, c)
			_, err := conn.ExecContext(context.Background(), insertStmt)
			if err != nil {
				return false, fmt.Errorf("error executing stmt - %v: %w", insertStmt, err)
			}
			log.Infof("created channels meta info: %s;", insertStmt)
		}
		return false, nil
	})
	return err
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

func (tdb *TargetOracleDB) ImportBatch(batch Batch, args *ImportBatchArgs, exportDir string) (int64, error) {
	tdb.Lock()
	defer tdb.Unlock()

	var rowsAffected int64
	var err error
	copyFn := func(conn *sql.Conn) (bool, error) {
		rowsAffected, err = tdb.importBatch(conn, batch, args, exportDir)
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

func (tdb *TargetOracleDB) importBatch(conn *sql.Conn, batch Batch, args *ImportBatchArgs, exportDir string) (rowsAffected int64, err error) {
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
	sqlldrConfig := args.GetSqlLdrControlFile(tdb.tconf.Schema)
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
	sqlldrArgs := fmt.Sprintf("userid=%s control=%s log=%s DIRECT=TRUE NO_INDEX_ERRORS=TRUE", oracleConnectionString, sqlldrControlFilePath, sqlldrLogFilePath)

	var outbuf string
	var errbuf string
	outbuf, errbuf, err = sqlldr.RunSqlldr(sqlldrArgs, password)

	if outbuf == "" && errbuf == "" && err != nil {
		// for error related to the stdinPipe of created while running sqlldr
		return 0, fmt.Errorf("run sqlldr: %w", err)
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
	return columns, nil
}

// execute all events sequentially one by one in a single transaction
func (tdb *TargetOracleDB) ExecuteBatch(migrationUUID uuid.UUID, batch EventBatch) error {
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
			_, err = tx.Exec(stmt)
			if err != nil {
				log.Errorf("error executing stmt for event with vsn(%d): %v", event.Vsn, err)
				return false, fmt.Errorf("error executing stmt for event with vsn(%d): %w", event.Vsn, err)
			}
		}

		updateVsnQuery := batch.GetQueryToUpdateLastAppliedVSN(migrationUUID)
		res, err := tx.Exec(updateVsnQuery)
		if err != nil {
			log.Errorf("error executing stmt: %v", err)
			return false, fmt.Errorf("failed to update vsn on target db via query-%s: %w", updateVsnQuery, err)
		} else if rowsAffected, err := res.RowsAffected(); rowsAffected == 0 || err != nil {
			log.Errorf("error executing stmt: %v, rowsAffected: %v", err, rowsAffected)
			return false, fmt.Errorf("failed to update vsn on target db via query-%s: %w, rowsAffected: %v",
				updateVsnQuery, err, rowsAffected)
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
		utils.PrintAndLog("Using %d parallel jobs by default. Use --parallel-jobs to specify a custom value", tdb.tconf.Parallelism)
	} else {
		utils.PrintAndLog("Using %d parallel jobs", tdb.tconf.Parallelism)
	}
	tdb.oraDB.SetMaxIdleConns(tdb.tconf.Parallelism)
	tdb.oraDB.SetMaxOpenConns(tdb.tconf.Parallelism)
	return nil
}

func (tdb *TargetOracleDB) GetDebeziumValueConverterSuite() map[string]ConverterFn {
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
		connectString = fmt.Sprintf("(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SID=%s)))", tconf.Host, tconf.Port, tconf.DBSid)
	case tconf.TNSAlias != "":
		connectString = tconf.TNSAlias
	case tconf.DBName != "":
		connectString = fmt.Sprintf("(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SERVICE_NAME=%s)))", tconf.Host, tconf.Port, tconf.DBName)
	}

	return connectString
}

func (tdb *TargetOracleDB) MaxBatchSizeInBytes() int64 {
	return 2 * 1024 * 1024 * 1024 // 2GB
}
