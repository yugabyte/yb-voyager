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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/sqlldr"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type TargetOracleDB struct {
	sync.Mutex
	tconf *TargetConf
	oraDB *sql.DB
	conn  *sql.Conn
}

var oraValueConverterSuite = map[string]ConverterFn{
	"DATE": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMilliSecs, err := strconv.ParseInt(columnValue, 10, 64) // from oracle for DATE type debezium gives epoch milliseconds or type is io.debezium.time.Timestamp
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch milliseconds: %v", err)
		}
		epochSecs := epochMilliSecs / 1000
		parsedTime := time.Unix(epochSecs, 0).Local()
		oracleDateFormat := "02-01-2006" //format: DD-MON-YYYY
		formattedDate := parsedTime.Format(oracleDateFormat)
		if err != nil {
			return "", fmt.Errorf("parsing date: %v", err)
		}
		if formatIfRequired {
			formattedDate = fmt.Sprintf("'%s'", formattedDate)
		}
		return formattedDate, nil
	}, // TODO: change type to DATE as per typename in db
	"io.debezium.time.Timestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMilliSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch milliseconds: %v", err)
		}
		epochSecs := epochMilliSecs / 1000
		timestamp := time.Unix(epochSecs, 0).Local()
		oracleTimestampFormat := "02-01-2006 03.04.05.000 PM" //format: DD-MM-YY HH.MI.SS.FFF PM
		formattedTimestamp := timestamp.Format(oracleTimestampFormat)
		if formatIfRequired {
			formattedTimestamp = fmt.Sprintf("'%s'", formattedTimestamp)
		}
		return formattedTimestamp, nil
	},
	"io.debezium.time.MicroTimestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMicroSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch microseconds: %v", err)
		}
		epochSeconds := epochMicroSecs / 1000000
		epochNanos := (epochMicroSecs % 1000000) * 1000
		microTimeStamp := time.Unix(epochSeconds, epochNanos).Local()
		oracleTimestampFormat := "02-01-2006 03.04.05.000000 PM" //format: DD-MON-YYYY HH.MI.SS.FFFFFF PM
		formattedTimestamp := microTimeStamp.Format(oracleTimestampFormat)
		if formatIfRequired {
			formattedTimestamp = fmt.Sprintf("'%s'", formattedTimestamp)
		}
		return formattedTimestamp, nil
	},
	"io.debezium.time.NanoTimestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		epochNanoSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return "", fmt.Errorf("parsing epoch nanoseconds: %v", err)
		}
		epochSeconds := epochNanoSecs / 1000000000
		epochNanos := epochNanoSecs % 1000000000
		nanoTimestamp := time.Unix(epochSeconds, epochNanos).Local()
		oracleTimestampFormat := "02-01-2006 03.04.05.000000000 PM" //format: DD-MON-YYYY HH.MI.SS.FFFFFFFFF PM
		formattedTimestamp := nanoTimestamp.Format(oracleTimestampFormat)
		if formatIfRequired {
			formattedTimestamp = fmt.Sprintf("'%s'", formattedTimestamp)
		}
		return formattedTimestamp, nil
	},
	"io.debezium.time.ZonedTimestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		debeziumFormat := "2006-01-02T15:04:05Z07:00"

		parsedTime, err := time.Parse(debeziumFormat, columnValue)
		if err != nil {
			return "", fmt.Errorf("parsing timestamp: %v", err)
		}

		oracleFormat := "06-01-02 3:04:05.000000000 PM -07:00"
		if parsedTime.Location().String() == "UTC" { //LOCAL TIMEZONE Case
			oracleFormat = "06-01-02 3:04:05.000000000 PM" // TODO: for timezone ones sqlldr is inserting as GMT though GMT/UTC is similar
		}
		formattedTimestamp := parsedTime.Format(oracleFormat)
		if formatIfRequired {
			formattedTimestamp = fmt.Sprintf("'%s'", formattedTimestamp)
		}
		return formattedTimestamp, nil
	},
	"BYTES": func(columnValue string, formatIfRequired bool) (string, error) {
		//decode base64 string to bytes
		decodedBytes, err := base64.StdEncoding.DecodeString(columnValue) //e.g.`////wv==` -> `[]byte{0x00, 0x00, 0x00, 0x00}`
		if err != nil {
			return columnValue, fmt.Errorf("decoding base64 string: %v", err)
		}
		//convert bytes to hex string e.g. `[]byte{0x00, 0x00, 0x00, 0x00}` -> `\\x00000000`
		hexString := ""
		for _, b := range decodedBytes {
			hexString += fmt.Sprintf("%02x", b)
		}
		hexValue := ""
		if formatIfRequired {
			hexValue = fmt.Sprintf("'%s'", hexString) // in insert statement no need of escaping the backslash and add quotes
		} else {
			hexValue = fmt.Sprintf(`%s`, hexString) // in data file need to escape the backslash
		}
		return string(hexValue), nil
	},
	"MAP": func(columnValue string, _ bool) (string, error) {
		mapValue := make(map[string]interface{})
		err := json.Unmarshal([]byte(columnValue), &mapValue)
		if err != nil {
			return columnValue, fmt.Errorf("parsing map: %v", err)
		}
		var transformedMapValue string
		for key, value := range mapValue {
			transformedMapValue = transformedMapValue + fmt.Sprintf("\"%s\"=>\"%s\",", key, value)
		}
		return fmt.Sprintf("'%s'", transformedMapValue[:len(transformedMapValue)-1]), nil //remove last comma and add quotes
	},
	"STRING": func(columnValue string, formatIfRequired bool) (string, error) {
		if formatIfRequired {
			formattedColumnValue := strings.Replace(columnValue, "'", "''", -1)
			return fmt.Sprintf("'%s'", formattedColumnValue), nil
		} else {
			return columnValue, nil
		}
	},
	"INTERVAL YEAR TO MONTH": func(columnValue string, formatIfRequired bool) (string, error) {
		//columnValue format: P-1Y-5M0DT0H0M0S
		splits := strings.Split(strings.TrimPrefix(columnValue, "P"), "M") // ["-1Y-5", "0DT0H0M0S"]
		yearsMonths := strings.Split(splits[0], "Y")                       // ["-1", "-5"]
		years, err := strconv.ParseInt(yearsMonths[0], 10, 64)             // -1
		if err != nil {
			return "", fmt.Errorf("parsing years: %v", err)
		}
		months, err := strconv.ParseInt(yearsMonths[1], 10, 64) // -5
		if err != nil {
			return "", fmt.Errorf("parsing months: %v", err)
		}
		if years < 0 || months < 0 {
			years = int64(math.Abs(float64(years)))
			months = int64(math.Abs(float64(months)))
			columnValue = fmt.Sprintf("-%d-%d", years, months) // -1-5
		} else {
			columnValue = fmt.Sprintf("%d-%d", years, months) // 1-5
		}
		if formatIfRequired {
			columnValue = fmt.Sprintf("'%s'", columnValue)
		}
		return columnValue, nil
	},
	"INTERVAL DAY TO SECOND": func(columnValue string, formatIfRequired bool) (string, error) {
		//columnValue format: P0Y0M24DT23H34M5.878667S //TODO check regex will be better or not
		splits := strings.Split(strings.TrimPrefix(columnValue, "P"), "M") // ["0Y0M", "24DT23H34, 5.878667S"]
		daysTime := strings.Split(splits[1], "DT")                         // ["24", "23H34M5.878667S"]
		days, err := strconv.ParseInt(daysTime[0], 10, 64)                 // 24
		if err != nil {
			return "", fmt.Errorf("parsing days: %v", err)
		}
		time := strings.Split(daysTime[1], "H")         // ["23", "34"]
		hours, err := strconv.ParseInt(time[0], 10, 64) // 23
		if err != nil {
			return "", fmt.Errorf("parsing hours: %v", err)
		}
		mins, err := strconv.ParseInt(time[1], 10, 64) // 34
		if err != nil {
			return "", fmt.Errorf("parsing minutes: %v", err)
		}
		seconds, err := strconv.ParseFloat(strings.TrimSuffix(splits[2], "S"), 64) // 5.878667
		if err != nil {
			return "", fmt.Errorf("parsing seconds: %v", err)
		}
		if days < 0 || hours < 0 || mins < 0 || seconds < 0 {
			days = int64(math.Abs(float64(days)))
			hours = int64(math.Abs(float64(hours)))
			mins = int64(math.Abs(float64(mins)))
			seconds = math.Abs(seconds)
			columnValue = fmt.Sprintf("-%d %d:%d:%.9f", days, hours, mins, seconds) // -24 23:34:5.878667
		} else {
			columnValue = fmt.Sprintf("%d %d:%d:%.9f", days, hours, mins, seconds) // 24 23:34:5.878667
		}
		if formatIfRequired {
			columnValue = fmt.Sprintf("'%s'", columnValue)
		}
		return columnValue, nil
	},
}

var TableToColumnDataTypesCache = make(map[string]map[string]string)

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
	//TODO: figure out if directly the datatypes from schema files can be used instead of querying the db
	if TableToColumnDataTypesCache[tableName] == nil {
		TableToColumnDataTypesCache[tableName], err = tdb.getColumnDataTypes(tableName)
		if err != nil {
			return 0, fmt.Errorf("get column data types for table %s: %w", tableName, err)
		}
	}
	columnTypes := TableToColumnDataTypesCache[tableName]
	sqlldrConfig := args.GetSqlLdrControlFile(tdb.tconf.Schema, columnTypes)
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
	sqlldrArgs := fmt.Sprintf("userid=%s control=%s log=%s DIRECT=TRUE NO_INDEX_ERRORS=TRUE ERRORS=0",
		oracleConnectionString, sqlldrControlFilePath, sqlldrLogFilePath)

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
	return columns, nil //TODO
}

func (tdb *TargetOracleDB) ExecuteBatch(batch []*Event) error {
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
	return nil
}

func (tdb *TargetOracleDB) GetDebeziumValueConverterSuite() map[string]ConverterFn {
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

func (tdb *TargetOracleDB) getColumnDataTypes(tableName string) (map[string]string, error) {
	var columnTypes = make(map[string]string)
	query := fmt.Sprintf("SELECT COLUMN_NAME, DATA_TYPE, CHAR_LENGTH FROM ALL_TAB_COLUMNS WHERE OWNER = '%s' AND TABLE_NAME = '%s'", tdb.tconf.Schema, tableName)
	rows, err := tdb.conn.QueryContext(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("run query %q on target: %w", query, err)
	}
	defer rows.Close()
	for rows.Next() {
		var columnName string
		var dataType string
		var charLength int
		err = rows.Scan(&columnName, &dataType, &charLength)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		columnTypes[columnName] = fmt.Sprintf("%s:%d", dataType, charLength)
	}
	return columnTypes, nil
}
