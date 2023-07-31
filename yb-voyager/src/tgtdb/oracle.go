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
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
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
		return fmt.Errorf("get connection from target db: %w", err)
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

	return tdb.connect()
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
		log.Infof("Failed to reconnect to the target database: %s", err)
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
	createUserQuery := fmt.Sprintf(`BEGIN
    DECLARE
        user_exists NUMBER;
    BEGIN
        SELECT COUNT(*) INTO user_exists FROM all_users WHERE username = UPPER('%s');
        IF user_exists = 0 THEN
            EXECUTE IMMEDIATE 'CREATE USER %s IDENTIFIED BY "password"';
        END IF;
    END;
END;`, BATCH_METADATA_TABLE_SCHEMA, BATCH_METADATA_TABLE_SCHEMA)
	grantQuery := fmt.Sprintf(`GRANT CONNECT, RESOURCE TO %s`, BATCH_METADATA_TABLE_SCHEMA)
	alterQuery := fmt.Sprintf(`ALTER USER %s QUOTA UNLIMITED ON USERS`, BATCH_METADATA_TABLE_SCHEMA)
	createTableQuery := fmt.Sprintf(`BEGIN
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

	cmds := []string{
		createUserQuery,
		grantQuery,
		alterQuery,
		createTableQuery,
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

func (tdb *TargetOracleDB) GetNonEmptyTables(tables []string) []string {
	result := []string{}

	for _, table := range tables {
		log.Infof("Checking if table %s.%s is empty", tdb.tconf.Schema, table)
		tmp := false
		stmt := fmt.Sprintf("SELECT 1 FROM %s.%s WHERE ROWNUM <= 1", tdb.tconf.Schema, table)
		err := tdb.conn.QueryRowContext(context.Background(), stmt).Scan(&tmp)
		if err != nil {
			utils.ErrExit("run query %q on target: %s", stmt, err)
		}
		if tmp {
			result = append(result, table)
		}
	}

	return result
}

func (tdb *TargetOracleDB) IsNonRetryableCopyError(err error) bool {
	return false
}

func (tdb *TargetOracleDB) ImportBatch(batch Batch, args *ImportBatchArgs, exportDir string) (int64, error) {
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
	if tdb.conn == nil {
		err = tdb.reconnect()
		if err != nil {
			return err
		}
	}
	retry := true
	for retry {
		tdb.reconnect()
		retry, err = fn(tdb.conn)
		if err != nil {
			// On err, drop the connection.
			err = tdb.conn.Close()
			if err != nil {
				log.Errorf("Failed to close connection to the target database: %v", err)
			}
		}
		time.Sleep(5 * time.Second)
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

	// fmt.Printf("Importing table path: %s \n", batch.GetFilePath())
	tableName := batch.GetTableName()

	sqlldrConfig := args.GetSqlLdrControlFile(tdb.tconf.Schema)

	if _, err := os.Stat(fmt.Sprintf("%s/sqlldr", exportDir)); os.IsNotExist(err) {
		err = os.Mkdir(fmt.Sprintf("%s/sqlldr", exportDir), 0755)
		if err != nil {
			return 0, fmt.Errorf("create sqlldr directory %q: %w", fmt.Sprintf("%s/sqlldr", exportDir), err)
		}
	}
	sqlldrControlFileName := fmt.Sprintf("%s.ctl", tableName)
	sqlldrControlFilePath := fmt.Sprintf("%s/sqlldr/%s", exportDir, sqlldrControlFileName)
	sqlldrControlFile, err := os.Create(sqlldrControlFilePath)
	if err != nil {
		return 0, fmt.Errorf("create sqlldr control file %q: %w", sqlldrControlFilePath, err)
	}
	defer sqlldrControlFile.Close()
	_, err = sqlldrControlFile.WriteString(sqlldrConfig)
	if err != nil {
		return 0, fmt.Errorf("write sqlldr control file %q: %w", sqlldrControlFilePath, err)
	}

	sqlldrLogFileName := fmt.Sprintf("%s.log", tableName)
	sqlldrLogFilePath := fmt.Sprintf("%s/sqlldr/%s", exportDir, sqlldrLogFileName)
	sqlldrLogFile, err := os.Create(sqlldrLogFilePath)
	if err != nil {
		return 0, fmt.Errorf("create sqlldr log file %q: %w", sqlldrLogFilePath, err)
	}
	defer sqlldrLogFile.Close()

	// fmt.Println("Running sqlldr for file: ", sqlldrControlFilePath)
	user := tdb.tconf.User
	password := tdb.tconf.Password
	connectString := tdb.getConnectionString(tdb.tconf)

	oracleConnectionString := fmt.Sprintf("%s/%s@\"%s\"", user, password, connectString)

	sqlldrArgs := fmt.Sprintf("userid=%s control=%s log=%s DIRECT=TRUE NO_INDEX_ERRORS=TRUE", oracleConnectionString, sqlldrControlFilePath, sqlldrLogFilePath)
	// fmt.Println("Args: ", sqlldrArgs)

	cmd := exec.Command("sqlldr", sqlldrArgs)
	var outbuf bytes.Buffer
	var errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	// fmt.Printf("cmd: %v\n", cmd)
	err = cmd.Run()
	if outbuf.String() != "" {
		log.Infof("sqlldr output: %s", outbuf.String())
	}
	if errbuf.String() != "" {
		log.Errorf("sqlldr error: %s", errbuf.String())
	}
	var err2 error
	rowsAffected, err2 = getRowsAffected(outbuf.String())
	if err2 != nil {
		return 0, fmt.Errorf("get rows affected from sqlldr output: %w", err)
	}

	if err != nil {
		// find ORA-00001: unique constraint * violated in log file
		pattern := regexp.MustCompile(`ORA-00001: unique constraint \(.+?\) violated`)
		scanner := bufio.NewScanner(sqlldrLogFile)
		for scanner.Scan() {
			line := scanner.Text()
			if pattern.MatchString(line) {
				err = tdb.recordEntryInDB(tx, batch, rowsAffected)
				if err != nil {
					err = fmt.Errorf("record entry in DB for batch %q: %w", batch.GetFilePath(), err)
				}
				return 0, err
			}
		}

		return rowsAffected, fmt.Errorf("run sqlldr: %w", err)
	}

	err = tdb.recordEntryInDB(tx, batch, rowsAffected)
	if err != nil {
		err = fmt.Errorf("record entry in DB for batch %q: %w", batch.GetFilePath(), err)
	}

	return rowsAffected, err
}

func (tdb *TargetOracleDB) recordEntryInDB(tx *sql.Tx, batch Batch, rowsAffected int64) error {
	cmd := batch.GetQueryToRecordEntryInDB(tdb.tconf.TargetDBType, rowsAffected)
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
		return 0, fmt.Errorf("no rows affected found in the sqlldr output")
	}
	return strconv.ParseInt(matches[1], 10, 64)
}

func getValue(connectionString, key string) string {
	startIndex := strings.Index(connectionString, key)
	if startIndex == -1 {
		return ""
	}

	startIndex += len(key) + 2
	endIndex := strings.Index(connectionString[startIndex:], "\"")

	return connectionString[startIndex : startIndex+endIndex]
}

func (tdb *TargetOracleDB) isBatchAlreadyImported(tx *sql.Tx, batch Batch) (bool, int64, error) {
	var rowsImported int64
	query := batch.GetQueryIsBatchAlreadyImported(tdb.tconf.TargetDBType)
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
	checkSchemaExistsQuery := fmt.Sprintf(
		"SELECT 1 FROM ALL_USERS WHERE USERNAME = '%s'",
		strings.ToUpper(tdb.tconf.Schema))
	var cntSchemaName int

	if err := conn.QueryRowContext(context.Background(), checkSchemaExistsQuery).Scan(&cntSchemaName); err != nil {
		utils.ErrExit("run query %q on target %q to check schema exists: %s", checkSchemaExistsQuery, tdb.tconf.Host, err)
	} else if cntSchemaName == 0 {
		utils.ErrExit("schema '%s' does not exist in target", tdb.tconf.Schema)
	}

	setSchemaQuery := fmt.Sprintf("ALTER SESSION SET CURRENT_SCHEMA = %s", tdb.tconf.Schema)
	_, err := conn.ExecContext(context.Background(), setSchemaQuery)
	if err != nil {
		utils.ErrExit("run query %q on target %q to set schema: %s", setSchemaQuery, tdb.tconf.Host, err)
	}
}

func (tdb *TargetOracleDB) IfRequiredQuoteColumnNames(tableName string, columns []string) ([]string, error) {
	return columns, nil
}

func (tdb *TargetOracleDB) ExecuteBatch(batch []*Event) error {
	return nil
}

func (tdb *TargetOracleDB) InitConnPool() error {
	return nil
}

func (tdb *TargetOracleDB) GetDebeziumValueConverterSuite() map[string]ConverterFn {
	return nil
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
