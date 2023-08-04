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
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type TargetYugabyteDB struct {
	sync.Mutex
	tconf    *TargetConf
	conn_    *pgx.Conn
	connPool *ConnectionPool
}

var ybValueConverterSuite = map[string]ConverterFn{
	"io.debezium.time.Date": func(columnValue string, formatIfRequired bool) (string, error) {
		epochDays, err := strconv.ParseUint(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch seconds: %v", err)
		}
		epochSecs := epochDays * 24 * 60 * 60
		date := time.Unix(int64(epochSecs), 0).Local().Format(time.DateOnly)
		if formatIfRequired {
			date = fmt.Sprintf("'%s'", date)
		}
		return date, nil
	},
	"io.debezium.time.Timestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMilliSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch milliseconds: %v", err)
		}
		epochSecs := epochMilliSecs / 1000
		timestamp := time.Unix(epochSecs, 0).Local().Format(time.DateTime)
		if formatIfRequired {
			timestamp = fmt.Sprintf("'%s'", timestamp)
		}
		return timestamp, nil
	},
	"io.debezium.time.MicroTimestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMicroSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch microseconds: %v", err)
		}
		epochSeconds := epochMicroSecs / 1000000
		epochNanos := (epochMicroSecs % 1000000) * 1000
		microTimeStamp, err := time.Parse(time.RFC3339Nano, time.Unix(epochSeconds, epochNanos).Local().Format(time.RFC3339Nano)) //TODO: check if proper format for Micro can work
		if err != nil {
			return columnValue, err
		}
		timestamp := strings.TrimSuffix(microTimeStamp.String(), " +0000 UTC")
		if formatIfRequired {
			timestamp = fmt.Sprintf("'%s'", timestamp)
		}
		return timestamp, nil
	},
	"io.debezium.time.NanoTimestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		epochNanoSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch nanoseconds: %v", err)
		}
		epochSeconds := epochNanoSecs / 1000000000
		epochNanos := epochNanoSecs % 1000000000
		nanoTimeStamp, err := time.Parse(time.RFC3339Nano, time.Unix(epochSeconds, epochNanos).Local().Format(time.RFC3339Nano))
		if err != nil {
			return columnValue, err
		}
		timestamp := strings.TrimSuffix(nanoTimeStamp.String(), " +0000 UTC")
		if formatIfRequired {
			timestamp = fmt.Sprintf("'%s'", timestamp)
		}
		return timestamp, nil
	},
	"io.debezium.time.ZonedTimestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		// no transformation as columnValue is formatted string from debezium by default
		if formatIfRequired {
			columnValue = fmt.Sprintf("'%s'", columnValue)
		}
		return columnValue, nil
	},
	"io.debezium.time.Time": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMilliSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch milliseconds: %v", err)
		}
		epochSecs := epochMilliSecs / 1000
		timeValue := time.Unix(epochSecs, 0).Local().Format(time.TimeOnly)
		if formatIfRequired {
			timeValue = fmt.Sprintf("'%s'", timeValue)
		}
		return timeValue, nil
	},
	"io.debezium.time.MicroTime": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMicroSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch microseconds: %v", err)
		}
		epochSeconds := epochMicroSecs / 1000000
		epochNanos := (epochMicroSecs % 1000000) * 1000
		MICRO_TIME_FORMAT := "15:04:05.000000"
		timeValue := time.Unix(epochSeconds, epochNanos).Local().Format(MICRO_TIME_FORMAT)
		if formatIfRequired {
			timeValue = fmt.Sprintf("'%s'", timeValue)
		}
		return timeValue, nil
	},
	"io.debezium.data.Bits": func(columnValue string, formatIfRequired bool) (string, error) {
		bytes, err := base64.StdEncoding.DecodeString(columnValue)
		if err != nil {
			return columnValue, fmt.Errorf("decoding variable scale decimal in base64: %v", err)
		}
		var data uint64
		if len(bytes) >= 8 {
			data = binary.LittleEndian.Uint64(bytes[:8])
		} else {
			for i, b := range bytes {
				data |= uint64(b) << (8 * i)
			}
		}
		if formatIfRequired {
			return fmt.Sprintf("'%b'", data), nil
		} else {
			return fmt.Sprintf("%b", data), nil
		}
	},
	"io.debezium.data.geometry.Point": func(columnValue string, formatIfRequired bool) (string, error) {
		// TODO: figure out if we want to represent it as a postgres native point or postgis point.
		return columnValue, nil
	},
	"io.debezium.data.geometry.Geometry": func(columnValue string, formatIfRequired bool) (string, error) {
		// TODO: figure out if we want to represent it as a postgres native point or postgis geometry point.
		return columnValue, nil
	},
	"io.debezium.data.geometry.Geography": func(columnValue string, formatIfRequired bool) (string, error) {
		//TODO: figure out if we want to represent it as a postgres native geography or postgis geometry geography.
		return columnValue, nil
	},
	"org.apache.kafka.connect.data.Decimal": func(columnValue string, formatIfRequired bool) (string, error) {
		return columnValue, nil //handled in exporter plugin
	},
	"io.debezium.data.VariableScaleDecimal": func(columnValue string, formatIfRequired bool) (string, error) {
		return columnValue, nil //handled in exporter plugin
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
			hexValue = fmt.Sprintf("'\\x%s'", hexString) // in insert statement no need of escaping the backslash and add quotes
		} else {
			hexValue = fmt.Sprintf(`\\x%s`, hexString) // in data file need to escape the backslash
		}
		return string(hexValue), nil
	},
	"MAP": func(columnValue string, formatIfRequired bool) (string, error) {
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
	"io.debezium.time.Interval": func(columnValue string, formatIfRequired bool) (string, error) {
		if formatIfRequired {
			columnValue = fmt.Sprintf("'%s'", columnValue)
		}
		return columnValue, nil
	},
}

func newTargetYugabyteDB(tconf *TargetConf) *TargetYugabyteDB {
	return &TargetYugabyteDB{tconf: tconf}
}

func (yb *TargetYugabyteDB) Init() error {
	return yb.connect()
}

func (yb *TargetYugabyteDB) Finalize() {
	yb.disconnect()
}

// TODO We should not export `Conn`. This is temporary--until we refactor all target db access.
func (yb *TargetYugabyteDB) Conn() *pgx.Conn {
	if yb.conn_ == nil {
		utils.ErrExit("Called TargetDB.Conn() before TargetDB.Connect()")
	}
	return yb.conn_
}

func (yb *TargetYugabyteDB) reconnect() error {
	yb.Mutex.Lock()
	defer yb.Mutex.Unlock()

	var err error
	yb.disconnect()
	for attempt := 1; attempt < 5; attempt++ {
		err = yb.connect()
		if err == nil {
			return nil
		}
		log.Infof("Failed to reconnect to the target database: %s", err)
		time.Sleep(time.Duration(attempt*2) * time.Second)
		// Retry.
	}
	return fmt.Errorf("reconnect to target db: %w", err)
}

func (yb *TargetYugabyteDB) connect() error {
	if yb.conn_ != nil {
		// Already connected.
		return nil
	}
	connStr := yb.tconf.GetConnectionUri()
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		return fmt.Errorf("connect to target db: %w", err)
	}
	yb.setTargetSchema(conn)
	yb.conn_ = conn
	return nil
}

func (yb *TargetYugabyteDB) disconnect() {
	if yb.conn_ == nil {
		// Already disconnected.
		return
	}

	err := yb.conn_.Close(context.Background())
	if err != nil {
		log.Infof("Failed to close connection to the target database: %s", err)
	}
	yb.conn_ = nil
}

func (yb *TargetYugabyteDB) EnsureConnected() {
	err := yb.connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the target DB: %s", err)
	}
}

func (yb *TargetYugabyteDB) GetVersion() string {
	if yb.tconf.dbVersion != "" {
		return yb.tconf.dbVersion
	}

	yb.EnsureConnected()
	yb.Mutex.Lock()
	defer yb.Mutex.Unlock()
	query := "SELECT setting FROM pg_settings WHERE name = 'server_version'"
	err := yb.conn_.QueryRow(context.Background(), query).Scan(&yb.tconf.dbVersion)
	if err != nil {
		utils.ErrExit("get target db version: %s", err)
	}
	return yb.tconf.dbVersion
}

func (yb *TargetYugabyteDB) InitConnPool() error {
	tconfs := yb.getYBServers()
	var targetUriList []string
	for _, tconf := range tconfs {
		targetUriList = append(targetUriList, tconf.Uri)
	}
	log.Infof("targetUriList: %s", utils.GetRedactedURLs(targetUriList))

	if yb.tconf.Parallelism == -1 {
		yb.tconf.Parallelism = fetchDefaultParllelJobs(tconfs)
		utils.PrintAndLog("Using %d parallel jobs by default. Use --parallel-jobs to specify a custom value", yb.tconf.Parallelism)
	} else {
		utils.PrintAndLog("Using %d parallel jobs", yb.tconf.Parallelism)
	}

	params := &ConnectionParams{
		NumConnections:    yb.tconf.Parallelism,
		ConnUriList:       targetUriList,
		SessionInitScript: getYBSessionInitScript(yb.tconf),
	}
	yb.connPool = NewConnectionPool(params)
	return nil
}

// The _v2 is appended in the table name so that the import code doesn't
// try to use the similar table created by the voyager 1.3 and earlier.
// Voyager 1.4 uses import data state format that is incompatible from
// the earlier versions.
const BATCH_METADATA_TABLE_NAME = "ybvoyager_metadata.ybvoyager_import_data_batches_metainfo_v2"
const EVENT_CHANNELS_METADATA_TABLE_NAME = "ybvoyager_metadata.ybvoyager_import_data_event_channels_metainfo"
const EVENT_CHANNELS_UPSERT_FUNC_NAME = "ybvoyager_metadata.upsert_event_channel_metainfo"

func (yb *TargetYugabyteDB) CreateVoyagerSchema() error {
	cmds := []string{
		"CREATE SCHEMA IF NOT EXISTS ybvoyager_metadata",
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			data_file_name VARCHAR(250),
			batch_number INT,
			schema_name VARCHAR(250),
			table_name VARCHAR(250),
			rows_imported BIGINT,
			PRIMARY KEY (data_file_name, batch_number, schema_name, table_name)
		);`, BATCH_METADATA_TABLE_NAME),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			channel_no INT PRIMARY KEY,
			last_applied_vsn BIGINT);`, EVENT_CHANNELS_METADATA_TABLE_NAME),
	}

	maxAttempts := 12
	var err error
outer:
	for _, cmd := range cmds {
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			log.Infof("Executing on target: [%s]", cmd)
			conn := yb.Conn()
			_, err = conn.Exec(context.Background(), cmd)
			if err == nil {
				// No error. Move on to the next command.
				continue outer
			}
			log.Warnf("Error while running [%s] attempt %d: %s", cmd, attempt, err)
			time.Sleep(5 * time.Second)
			err = yb.reconnect()
			if err != nil {
				break
			}
		}
		if err != nil {
			return fmt.Errorf("create ybvoyager schema on target: %w", err)
		}
	}
	return nil
}

func (yb *TargetYugabyteDB) InitEventChannelsMetaInfo(numChans int, truncate bool) error {
	err := yb.connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
		if truncate {
			truncateStmt := fmt.Sprintf("TRUNCATE TABLE %s;", EVENT_CHANNELS_METADATA_TABLE_NAME)
			_, err := conn.Exec(context.Background(), truncateStmt)
			if err != nil {
				return false, fmt.Errorf("error executing stmt - %v: %w", truncateStmt, err)
			}
		}
		// if there are >0 rows, then already been inited.
		rowsStmt := fmt.Sprintf(
			"SELECT count(*) FROM %s", EVENT_CHANNELS_METADATA_TABLE_NAME)
		var rows int
		err := conn.QueryRow(context.Background(), rowsStmt).Scan(&rows)
		if err != nil {
			fmt.Errorf("error executing stmt - %v: %w", rowsStmt, err)
		}
		if rows > 0 {
			return false, nil
		}

		for c := 0; c < numChans; c++ {
			insertStmt := fmt.Sprintf("INSERT INTO %s VALUES (%d, -1);", EVENT_CHANNELS_METADATA_TABLE_NAME, c)
			_, err := conn.Exec(context.Background(), insertStmt)
			if err != nil {
				return false, fmt.Errorf("error executing stmt - %v: %w", insertStmt, err)
			}
		}
		return false, nil
	})
	return err
}

func (yb *TargetYugabyteDB) GetEventChannelsMetaInfo() (map[int]EventChannelMetaInfo, error) {
	metainfo := map[int]EventChannelMetaInfo{}

	query := fmt.Sprintf("SELECT channel_no, last_applied_vsn FROM %s;", EVENT_CHANNELS_METADATA_TABLE_NAME)
	rows, err := yb.Conn().Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("Failed to query meta info for channels: %w", err)
	}

	for rows.Next() {
		var chanMetaInfo EventChannelMetaInfo
		rows.Scan(&(chanMetaInfo.ChanNo), &(chanMetaInfo.LastAppliedVsn))
		metainfo[chanMetaInfo.ChanNo] = chanMetaInfo
	}
	return metainfo, nil
}

func (yb *TargetYugabyteDB) GetNonEmptyTables(tables []string) []string {
	result := []string{}

	for _, table := range tables {
		log.Infof("Checking if table %q is empty.", table)
		tmp := false
		stmt := fmt.Sprintf("SELECT TRUE FROM %s LIMIT 1;", table)
		err := yb.Conn().QueryRow(context.Background(), stmt).Scan(&tmp)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			utils.ErrExit("failed to check whether table %q empty: %s", table, err)
		}
		result = append(result, table)
	}
	log.Infof("non empty tables: %v", result)
	return result
}

func (yb *TargetYugabyteDB) CleanFileImportState(filePath, tableName string) error {
	// Delete all entries from ${BATCH_METADATA_TABLE_NAME} for this table.
	schemaName := yb.getTargetSchemaName(tableName)
	cmd := fmt.Sprintf(
		`DELETE FROM %s WHERE data_file_name = '%s' AND schema_name = '%s' AND table_name = '%s'`,
		BATCH_METADATA_TABLE_NAME, filePath, schemaName, tableName)
	res, err := yb.Conn().Exec(context.Background(), cmd)
	if err != nil {
		return fmt.Errorf("remove %q related entries from %s: %w", tableName, BATCH_METADATA_TABLE_NAME, err)
	}
	log.Infof("query: [%s] => rows affected %v", cmd, res.RowsAffected())
	return nil
}

func (yb *TargetYugabyteDB) ImportBatch(batch Batch, args *ImportBatchArgs) (int64, error) {
	var rowsAffected int64
	var err error
	copyFn := func(conn *pgx.Conn) (bool, error) {
		rowsAffected, err = yb.importBatch(conn, batch, args)
		return false, err // Retries are now implemented in the caller.
	}
	err = yb.connPool.WithConn(copyFn)
	return rowsAffected, err
}

func (yb *TargetYugabyteDB) importBatch(conn *pgx.Conn, batch Batch, args *ImportBatchArgs) (rowsAffected int64, err error) {
	var file *os.File
	file, err = batch.Open()
	if err != nil {
		return 0, fmt.Errorf("open file %s: %w", batch.GetFilePath(), err)
	}
	defer file.Close()

	//setting the schema so that COPY command can acesss the table
	yb.setTargetSchema(conn)

	// NOTE: DO NOT DEFINE A NEW err VARIABLE IN THIS FUNCTION. ELSE, IT WILL MASK THE err FROM RETURN LIST.
	ctx := context.Background()
	var tx pgx.Tx
	tx, err = conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		var err2 error
		if err != nil {
			err2 = tx.Rollback(ctx)
			if err2 != nil {
				rowsAffected = 0
				err = fmt.Errorf("rollback txn: %w (while processing %s)", err2, err)
			}
		} else {
			err2 = tx.Commit(ctx)
			if err2 != nil {
				rowsAffected = 0
				err = fmt.Errorf("commit txn: %w", err2)
			}
		}
	}()

	// Check if the split is already imported.
	var alreadyImported bool
	alreadyImported, rowsAffected, err = yb.isBatchAlreadyImported(tx, batch)
	if err != nil {
		return 0, err
	}
	if alreadyImported {
		return rowsAffected, nil
	}

	// Import the split using COPY command.
	var res pgconn.CommandTag
	copyCommand := args.GetYBCopyStatement()
	log.Infof("Importing %q using COPY command: [%s]", batch.GetFilePath(), copyCommand)
	res, err = tx.Conn().PgConn().CopyFrom(context.Background(), file, copyCommand)
	if err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) {
			err = fmt.Errorf("%s, %s in %s", err.Error(), pgerr.Where, batch.GetFilePath())
		}
		return res.RowsAffected(), err
	}

	err = yb.recordEntryInDB(tx, batch, res.RowsAffected())
	if err != nil {
		err = fmt.Errorf("record entry in DB for batch %q: %w", batch.GetFilePath(), err)
	}
	return res.RowsAffected(), err
}

func (yb *TargetYugabyteDB) IfRequiredQuoteColumnNames(tableName string, columns []string) ([]string, error) {
	result := make([]string, len(columns))
	// FAST PATH.
	fastPathSuccessful := true
	for i, colName := range columns {
		if strings.ToLower(colName) == colName {
			if sqlname.IsReservedKeyword(colName) && colName[0:1] != `"` {
				result[i] = fmt.Sprintf(`"%s"`, colName)
			} else {
				result[i] = colName
			}
		} else {
			// Go to slow path.
			log.Infof("column name (%s) is not all lower-case. Going to slow path.", colName)
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
	schemaName := yb.tconf.Schema
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		schemaName = parts[0]
		tableName = parts[1]
	}
	targetColumns, err := yb.getListOfTableAttributes(schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("get list of table attributes: %w", err)
	}
	log.Infof("columns of table %s.%s in target db: %v", schemaName, tableName, targetColumns)

	for i, colName := range columns {
		if colName[0] == '"' && colName[len(colName)-1] == '"' {
			colName = colName[1 : len(colName)-1]
		}
		switch true {
		// TODO: Move sqlname.IsReservedKeyword() in this file.
		case sqlname.IsReservedKeyword(colName):
			result[i] = fmt.Sprintf(`"%s"`, colName)
		case colName == strings.ToLower(colName): // Name is all lowercase.
			result[i] = colName
		case slices.Contains(targetColumns, colName): // Name is not keyword and is not all lowercase.
			result[i] = fmt.Sprintf(`"%s"`, colName)
		case slices.Contains(targetColumns, strings.ToLower(colName)): // Case insensitive name given with mixed case.
			result[i] = strings.ToLower(colName)
		default:
			return nil, fmt.Errorf("column %q not found in table %s", colName, tableName)
		}
	}
	log.Infof("columns of table %s.%s after quoting: %v", schemaName, tableName, result)
	return result, nil
}

func (yb *TargetYugabyteDB) getListOfTableAttributes(schemaName, tableName string) ([]string, error) {
	var result []string
	if tableName[0] == '"' {
		// Remove the double quotes around the table name.
		tableName = tableName[1 : len(tableName)-1]
	}
	query := fmt.Sprintf(
		`SELECT column_name FROM information_schema.columns WHERE table_schema = '%s' AND table_name ILIKE '%s'`,
		schemaName, tableName)
	rows, err := yb.Conn().Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("run [%s] on target: %w", query, err)
	}
	defer rows.Close()
	for rows.Next() {
		var colName string
		err = rows.Scan(&colName)
		if err != nil {
			return nil, fmt.Errorf("scan column name: %w", err)
		}
		result = append(result, colName)
	}
	return result, nil
}

var NonRetryCopyErrors = []string{
	"Sending too long RPC message",
	"invalid input syntax",
	"violates unique constraint",
	"syntax error at",
}

func (yb *TargetYugabyteDB) IsNonRetryableCopyError(err error) bool {
	return err != nil && utils.InsensitiveSliceContains(NonRetryCopyErrors, err.Error())
}

func (yb *TargetYugabyteDB) ExecuteBatch(batch EventBatch) error {

	var err error

	err = yb.connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
		tx, err := conn.BeginTx(context.Background(), pgx.TxOptions{})
		if err != nil {
			return false, fmt.Errorf("error creating tx: %w", err)
		}
		for i := 0; i < len(batch.Events); i++ {
			event := batch.Events[i]
			stmt := event.GetSQLStmt(yb.tconf.Schema)
			log.Debug(stmt)
			_, err := tx.Exec(context.Background(), stmt)
			if err != nil {
				log.Errorf("Error executing stmt: %v", err)
				return false, fmt.Errorf("error executing stmt - %v: %w", stmt, err)
			}
		}
		importStateQuery := fmt.Sprintf(`UPDATE %s SET last_applied_vsn=%d where channel_no=%d;`, EVENT_CHANNELS_METADATA_TABLE_NAME, batch.GetLastVsn(), batch.ChanNo)
		_, err = tx.Exec(context.Background(), importStateQuery)
		if err != nil {
			log.Errorf("Error executing stmt: %v", err)
			return false, fmt.Errorf("failed to update state on target db via query-%s: %w", importStateQuery, err)
		}
		if err == nil {
			tx.Commit(context.Background())
		} else {
			tx.Rollback(context.Background())
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("error executing batch: %w", err)
	}

	// Idempotency considerations:
	// Note: Assuming PK column value is not changed via UPDATEs
	// INSERT: The connPool sets `yb_enable_upsert_mode to true`. Hence the insert will be
	// successful even if the row already exists.
	// DELETE does NOT fail if the row does not exist. Rows affected will be 0.
	// UPDATE statement does not fail if the row does not exist. Rows affected will be 0.

	return err
}

//==============================================================================

const (
	LB_WARN_MSG = "--target-db-host is a load balancer IP which will be used to create connections for data import.\n" +
		"\t To control the parallelism and servers used, refer to help for --parallel-jobs and --target-endpoints flags.\n"

	GET_YB_SERVERS_QUERY = "SELECT host, port, num_connections, node_type, cloud, region, zone, public_ip FROM yb_servers()"
)

func (yb *TargetYugabyteDB) getYBServers() []*TargetConf {
	var tconfs []*TargetConf
	var loadBalancerUsed bool

	tconf := yb.tconf

	if tconf.TargetEndpoints != "" {
		msg := fmt.Sprintf("given yb-servers for import data: %q\n", tconf.TargetEndpoints)
		utils.PrintIfTrue(msg, tconf.VerboseMode)
		log.Infof(msg)

		ybServers := utils.CsvStringToSlice(tconf.TargetEndpoints)
		for _, ybServer := range ybServers {
			clone := tconf.Clone()

			if strings.Contains(ybServer, ":") {
				clone.Host = strings.Split(ybServer, ":")[0]
				var err error
				clone.Port, err = strconv.Atoi(strings.Split(ybServer, ":")[1])

				if err != nil {
					utils.ErrExit("error in parsing useYbServers flag: %v", err)
				}
			} else {
				clone.Host = ybServer
			}

			clone.Uri = getCloneConnectionUri(clone)
			log.Infof("using yb server for import data: %+v", GetRedactedTargetConf(clone))
			tconfs = append(tconfs, clone)
		}
	} else {
		loadBalancerUsed = true
		url := tconf.GetConnectionUri()
		conn, err := pgx.Connect(context.Background(), url)
		if err != nil {
			utils.ErrExit("Unable to connect to database: %v", err)
		}
		defer conn.Close(context.Background())

		rows, err := conn.Query(context.Background(), GET_YB_SERVERS_QUERY)
		if err != nil {
			utils.ErrExit("error in query rows from yb_servers(): %v", err)
		}
		defer rows.Close()

		var hostPorts []string
		for rows.Next() {
			clone := tconf.Clone()
			var host, nodeType, cloud, region, zone, public_ip string
			var port, num_conns int
			if err := rows.Scan(&host, &port, &num_conns,
				&nodeType, &cloud, &region, &zone, &public_ip); err != nil {
				utils.ErrExit("error in scanning rows of yb_servers(): %v", err)
			}

			// check if given host is one of the server in cluster
			if loadBalancerUsed {
				if isSeedTargetHost(tconf, host, public_ip) {
					loadBalancerUsed = false
				}
			}

			if tconf.UsePublicIP {
				if public_ip != "" {
					clone.Host = public_ip
				} else {
					var msg string
					if host == "" {
						msg = fmt.Sprintf("public ip is not available for host: %s."+
							"Refer to help for more details for how to enable public ip.", host)
					} else {
						msg = fmt.Sprintf("public ip is not available for host: %s but private ip are available. "+
							"Either refer to help for how to enable public ip or remove --use-public-up flag and restart the import", host)
					}
					utils.ErrExit(msg)
				}
			} else {
				clone.Host = host
			}

			clone.Port = port
			clone.Uri = getCloneConnectionUri(clone)
			tconfs = append(tconfs, clone)

			hostPorts = append(hostPorts, fmt.Sprintf("%s:%v", host, port))
		}
		log.Infof("Target DB nodes: %s", strings.Join(hostPorts, ","))
	}

	if loadBalancerUsed { // if load balancer is used no need to check direct connectivity
		utils.PrintAndLog(LB_WARN_MSG)
		tconfs = []*TargetConf{tconf}
	} else {
		tconfs = testAndFilterYbServers(tconfs)
	}
	return tconfs
}

func getCloneConnectionUri(clone *TargetConf) string {
	var cloneConnectionUri string
	if clone.Uri == "" {
		//fallback to constructing the URI from individual parameters. If URI was not set for target, then its other necessary parameters must be non-empty (or default values)
		cloneConnectionUri = clone.GetConnectionUri()
	} else {
		targetConnectionUri, err := url.Parse(clone.Uri)
		if err == nil {
			targetConnectionUri.Host = fmt.Sprintf("%s:%d", clone.Host, clone.Port)
			cloneConnectionUri = fmt.Sprint(targetConnectionUri)
		} else {
			panic(err)
		}
	}
	return cloneConnectionUri
}

func isSeedTargetHost(tconf *TargetConf, names ...string) bool {
	var allIPs []string
	for _, name := range names {
		if name != "" {
			allIPs = append(allIPs, utils.LookupIP(name)...)
		}
	}

	seedHostIPs := utils.LookupIP(tconf.Host)
	for _, seedHostIP := range seedHostIPs {
		if slices.Contains(allIPs, seedHostIP) {
			log.Infof("Target.Host=%s matched with one of ips in %v\n", seedHostIP, allIPs)
			return true
		}
	}
	return false
}

// this function will check the reachability to each of the nodes and returns list of ones which are reachable
func testAndFilterYbServers(tconfs []*TargetConf) []*TargetConf {
	var availableTargets []*TargetConf

	for _, tconf := range tconfs {
		log.Infof("testing server: %s\n", spew.Sdump(GetRedactedTargetConf(tconf)))
		conn, err := pgx.Connect(context.Background(), tconf.GetConnectionUri())
		if err != nil {
			utils.PrintAndLog("unable to use yb-server %q: %v", tconf.Host, err)
		} else {
			availableTargets = append(availableTargets, tconf)
			conn.Close(context.Background())
		}
	}

	if len(availableTargets) == 0 {
		utils.ErrExit("no yb servers available for data import")
	}
	return availableTargets
}

func fetchDefaultParllelJobs(tconfs []*TargetConf) int {
	totalCores := 0
	targetCores := 0
	for _, tconf := range tconfs {
		log.Infof("Determining CPU core count on: %s", utils.GetRedactedURLs([]string{tconf.Uri})[0])
		conn, err := pgx.Connect(context.Background(), tconf.Uri)
		if err != nil {
			log.Warnf("Unable to reach target while querying cores: %v", err)
			return len(tconfs) * 2
		}
		defer conn.Close(context.Background())

		cmd := "CREATE TEMP TABLE yb_voyager_cores(num_cores int);"
		_, err = conn.Exec(context.Background(), cmd)
		if err != nil {
			log.Warnf("Unable to create tables on target DB: %v", err)
			return len(tconfs) * 2
		}

		cmd = "COPY yb_voyager_cores(num_cores) FROM PROGRAM 'grep processor /proc/cpuinfo|wc -l';"
		_, err = conn.Exec(context.Background(), cmd)
		if err != nil {
			log.Warnf("Error while running query %s on host %s: %v", cmd, utils.GetRedactedURLs([]string{tconf.Uri}), err)
			return len(tconfs) * 2
		}

		cmd = "SELECT num_cores FROM yb_voyager_cores;"
		if err = conn.QueryRow(context.Background(), cmd).Scan(&targetCores); err != nil {
			log.Warnf("Error while running query %s: %v", cmd, err)
			return len(tconfs) * 2
		}
		totalCores += targetCores
	}
	if totalCores == 0 { //if target is running on MacOS, we are unable to determine totalCores
		return 3
	}
	return totalCores / 2
}

// import session parameters
const (
	SET_CLIENT_ENCODING_TO_UTF8           = "SET client_encoding TO 'UTF8'"
	SET_SESSION_REPLICATE_ROLE_TO_REPLICA = "SET session_replication_role TO replica" //Disable triggers or fkeys constraint checks.
	SET_YB_ENABLE_UPSERT_MODE             = "SET yb_enable_upsert_mode to true"
	SET_YB_DISABLE_TRANSACTIONAL_WRITES   = "SET yb_disable_transactional_writes to true" // Disable transactions to improve ingestion throughput.
)

func getYBSessionInitScript(tconf *TargetConf) []string {
	var sessionVars []string
	if checkSessionVariableSupport(tconf, SET_CLIENT_ENCODING_TO_UTF8) {
		sessionVars = append(sessionVars, SET_CLIENT_ENCODING_TO_UTF8)
	}
	if checkSessionVariableSupport(tconf, SET_SESSION_REPLICATE_ROLE_TO_REPLICA) {
		sessionVars = append(sessionVars, SET_SESSION_REPLICATE_ROLE_TO_REPLICA)
	}

	if tconf.EnableUpsert {
		// upsert_mode parameters was introduced later than yb_disable_transactional writes in yb releases
		// hence if upsert_mode is supported then its safe to assume yb_disable_transactional_writes is already there
		if checkSessionVariableSupport(tconf, SET_YB_ENABLE_UPSERT_MODE) {
			sessionVars = append(sessionVars, SET_YB_ENABLE_UPSERT_MODE)
			// 	SET_YB_DISABLE_TRANSACTIONAL_WRITES is used only with & if upsert_mode is supported
			if tconf.DisableTransactionalWrites {
				if checkSessionVariableSupport(tconf, SET_YB_DISABLE_TRANSACTIONAL_WRITES) {
					sessionVars = append(sessionVars, SET_YB_DISABLE_TRANSACTIONAL_WRITES)
				} else {
					tconf.DisableTransactionalWrites = false
				}
			}
		} else {
			log.Infof("Falling back to transactional inserts of batches during data import")
		}
	}

	sessionVarsPath := "/etc/yb-voyager/ybSessionVariables.sql"
	if !utils.FileOrFolderExists(sessionVarsPath) {
		log.Infof("YBSessionInitScript: %v\n", sessionVars)
		return sessionVars
	}

	varsFile, err := os.Open(sessionVarsPath)
	if err != nil {
		utils.PrintAndLog("Unable to open %s : %v. Using default values.", sessionVarsPath, err)
		log.Infof("YBSessionInitScript: %v\n", sessionVars)
		return sessionVars
	}
	defer varsFile.Close()
	fileScanner := bufio.NewScanner(varsFile)

	var curLine string
	for fileScanner.Scan() {
		curLine = strings.TrimSpace(fileScanner.Text())
		if curLine != "" && checkSessionVariableSupport(tconf, curLine) {
			sessionVars = append(sessionVars, curLine)
		}
	}
	log.Infof("YBSessionInitScript: %v\n", sessionVars)
	return sessionVars
}

func checkSessionVariableSupport(tconf *TargetConf, sqlStmt string) bool {
	conn, err := pgx.Connect(context.Background(), tconf.GetConnectionUri())
	if err != nil {
		utils.ErrExit("error while creating connection for checking session parameter(%q) support: %v", sqlStmt, err)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), sqlStmt)
	if err != nil {
		if !strings.Contains(err.Error(), "unrecognized configuration parameter") {
			utils.ErrExit("error while executing sqlStatement=%q: %v", sqlStmt, err)
		} else {
			log.Warnf("Warning: %q is not supported: %v", sqlStmt, err)
		}
	}

	return err == nil
}

func (yb *TargetYugabyteDB) setTargetSchema(conn *pgx.Conn) {
	checkSchemaExistsQuery := fmt.Sprintf(
		"SELECT count(schema_name) FROM information_schema.schemata WHERE schema_name = '%s'",
		yb.tconf.Schema)
	var cntSchemaName int

	if err := conn.QueryRow(context.Background(), checkSchemaExistsQuery).Scan(&cntSchemaName); err != nil {
		utils.ErrExit("run query %q on target %q to check schema exists: %s", checkSchemaExistsQuery, yb.tconf.Host, err)
	} else if cntSchemaName == 0 {
		utils.ErrExit("schema '%s' does not exist in target", yb.tconf.Schema)
	}

	setSchemaQuery := fmt.Sprintf("SET SCHEMA '%s'", yb.tconf.Schema)
	_, err := conn.Exec(context.Background(), setSchemaQuery)
	if err != nil {
		utils.ErrExit("run query %q on target %q: %s", setSchemaQuery, yb.tconf.Host, err)
	}

	// append oracle schema in the search_path for orafce
	// It is okay even if the schema does not exist in the target.
	updateSearchPath := `SELECT set_config('search_path', current_setting('search_path') || ', oracle', false)`
	_, err = conn.Exec(context.Background(), updateSearchPath)
	if err != nil {
		utils.ErrExit("unable to update search_path for orafce extension: %v", err)
	}

}

func (yb *TargetYugabyteDB) getTargetSchemaName(tableName string) string {
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		return parts[0]
	}
	return yb.tconf.Schema // default set to "public"
}

func (yb *TargetYugabyteDB) isBatchAlreadyImported(tx pgx.Tx, batch Batch) (bool, int64, error) {
	var rowsImported int64
	query := batch.GetQueryIsBatchAlreadyImported()
	err := tx.QueryRow(context.Background(), query).Scan(&rowsImported)
	if err == nil {
		log.Infof("%v rows from %q are already imported", rowsImported, batch.GetFilePath())
		return true, rowsImported, nil
	}
	if err == pgx.ErrNoRows {
		log.Infof("%q is not imported yet", batch.GetFilePath())
		return false, 0, nil
	}
	return false, 0, fmt.Errorf("check if %s is already imported: %w", batch.GetFilePath(), err)
}

func (yb *TargetYugabyteDB) recordEntryInDB(tx pgx.Tx, batch Batch, rowsAffected int64) error {
	cmd := batch.GetQueryToRecordEntryInDB(rowsAffected)
	_, err := tx.Exec(context.Background(), cmd)
	if err != nil {
		return fmt.Errorf("insert into %s: %w", BATCH_METADATA_TABLE_NAME, err)
	}
	return nil
}

func (yb *TargetYugabyteDB) GetDebeziumValueConverterSuite() map[string]ConverterFn {
	return ybValueConverterSuite
}
