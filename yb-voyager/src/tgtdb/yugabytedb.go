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
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	tgtdbsuite "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/suites"
)

type TargetYugabyteDB struct {
	sync.Mutex
	tconf    *TargetConf
	conn_    *pgx.Conn
	connPool *ConnectionPool
}

func newTargetYugabyteDB(tconf *TargetConf) *TargetYugabyteDB {
	return &TargetYugabyteDB{tconf: tconf}
}

func (yb *TargetYugabyteDB) Init() error {
	err := yb.connect()
	if err != nil {
		return err
	}

	checkSchemaExistsQuery := fmt.Sprintf(
		"SELECT count(schema_name) FROM information_schema.schemata WHERE schema_name = '%s'",
		yb.tconf.Schema)
	var cntSchemaName int
	if err = yb.conn_.QueryRow(context.Background(), checkSchemaExistsQuery).Scan(&cntSchemaName); err != nil {
		err = fmt.Errorf("run query %q on target %q to check schema exists: %s", checkSchemaExistsQuery, yb.tconf.Host, err)
	} else if cntSchemaName == 0 {
		err = fmt.Errorf("schema '%s' does not exist in target", yb.tconf.Schema)
	}
	return err
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
	if yb.tconf.DBVersion != "" {
		return yb.tconf.DBVersion
	}

	yb.EnsureConnected()
	yb.Mutex.Lock()
	defer yb.Mutex.Unlock()
	query := "SELECT setting FROM pg_settings WHERE name = 'server_version'"
	err := yb.conn_.QueryRow(context.Background(), query).Scan(&yb.tconf.DBVersion)
	if err != nil {
		utils.ErrExit("get target db version: %s", err)
	}
	return yb.tconf.DBVersion
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
		log.Infof("Using %d parallel jobs by default. Use --parallel-jobs to specify a custom value", yb.tconf.Parallelism)
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
const BATCH_METADATA_TABLE_SCHEMA = "ybvoyager_metadata"
const BATCH_METADATA_TABLE_NAME = BATCH_METADATA_TABLE_SCHEMA + "." + "ybvoyager_import_data_batches_metainfo_v2"
const EVENT_CHANNELS_METADATA_TABLE_NAME = BATCH_METADATA_TABLE_SCHEMA + "." + "ybvoyager_import_data_event_channels_metainfo"
const EVENTS_PER_TABLE_METADATA_TABLE_NAME = BATCH_METADATA_TABLE_SCHEMA + "." + "ybvoyager_imported_event_count_by_table"

func (yb *TargetYugabyteDB) CreateVoyagerSchema() error {
	cmds := []string{
		fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s;`, BATCH_METADATA_TABLE_SCHEMA),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			data_file_name VARCHAR(250),
			batch_number INT,
			schema_name VARCHAR(250),
			table_name VARCHAR(250),
			rows_imported BIGINT,
			PRIMARY KEY (data_file_name, batch_number, schema_name, table_name)
		);`, BATCH_METADATA_TABLE_NAME),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			migration_uuid uuid,
			channel_no INT,
			last_applied_vsn BIGINT,
			num_inserts BIGINT,
			num_deletes BIGINT,
			num_updates BIGINT,
			PRIMARY KEY (migration_uuid, channel_no));`, EVENT_CHANNELS_METADATA_TABLE_NAME),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			migration_uuid uuid,
			table_name VARCHAR(250), 
			channel_no INT,
			total_events BIGINT,
			num_inserts BIGINT,
			num_deletes BIGINT,
			num_updates BIGINT,
			PRIMARY KEY (migration_uuid, table_name, channel_no));`, EVENTS_PER_TABLE_METADATA_TABLE_NAME),
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
			err2 := yb.reconnect()
			if err2 != nil {
				log.Warnf("Failed to reconnect to the target database: %s", err2)
				break
			}
		}
		if err != nil {
			return fmt.Errorf("create ybvoyager schema on target: %w", err)
		}
	}
	return nil
}

func (yb *TargetYugabyteDB) clearMigrationStateFromTable(conn *pgx.Conn, tableName string, migrationUUID uuid.UUID) error {
	stmt := fmt.Sprintf("DELETE FROM %s where migration_uuid='%s'", tableName, migrationUUID)
	res, err := conn.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error executing stmt - %v: %w", stmt, err)
	}
	log.Infof("Query: %s ==> Rows affected: %d", stmt, res.RowsAffected())
	return nil
}

func (yb *TargetYugabyteDB) getEventChannelsRowCount(conn *pgx.Conn, migrationUUID uuid.UUID) (int64, error) {
	rowsStmt := fmt.Sprintf(
		"SELECT count(*) FROM %s where migration_uuid='%s'", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID)
	var rowCount int64
	err := conn.QueryRow(context.Background(), rowsStmt).Scan(&rowCount)
	if err != nil {
		return 0, fmt.Errorf("error executing stmt - %v: %w", rowsStmt, err)
	}
	return rowCount, nil
}

func (yb *TargetYugabyteDB) getLiveMigrationMetaInfoByTable(conn *pgx.Conn, migrationUUID uuid.UUID, tableName string) (int64, error) {
	rowsStmt := fmt.Sprintf(
		"SELECT count(*) FROM %s where migration_uuid='%s' AND table_name='%s'",
		EVENTS_PER_TABLE_METADATA_TABLE_NAME, migrationUUID, tableName)
	var rowCount int64
	err := conn.QueryRow(context.Background(), rowsStmt).Scan(&rowCount)
	if err != nil {
		return 0, fmt.Errorf("error executing stmt - %v: %w", rowsStmt, err)
	}
	return rowCount, nil
}

func (yb *TargetYugabyteDB) initChannelMetaInfo(conn *pgx.Conn, migrationUUID uuid.UUID, numChans int) error {
	// if there are >0 rows, then skip because already been inited.
	rowCount, err := yb.getEventChannelsRowCount(conn, migrationUUID)
	if err != nil {
		return fmt.Errorf("error getting channels meta info for %s: %w", EVENT_CHANNELS_METADATA_TABLE_NAME, err)
	}
	if rowCount > 0 {
		log.Info("event channels meta info already created. Skipping init.")
		return nil
	}
	ctx := context.Background()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("error creating tx: %w", err)
	}
	defer tx.Rollback(ctx)
	for c := 0; c < numChans; c++ {
		insertStmt := fmt.Sprintf("INSERT INTO %s VALUES ('%s', %d, -1, %d, %d, %d)", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID, c, 0, 0, 0)
		_, err := tx.Exec(context.Background(), insertStmt)
		if err != nil {
			return fmt.Errorf("error executing stmt - %v: %w", insertStmt, err)
		}
		log.Infof("created channels meta info: %s;", insertStmt)

		if err != nil {
			return fmt.Errorf("error initializing channels meta info for %s: %w", EVENT_CHANNELS_METADATA_TABLE_NAME, err)
		}
	}
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("error committing tx: %w", err)
	}
	return nil
}

func (yb *TargetYugabyteDB) qualifyTableName(tableName string) string {
	if len(strings.Split(tableName, ".")) != 2 {
		tableName = fmt.Sprintf("%s.%s", yb.tconf.Schema, tableName)
	}
	return tableName
}

func (yb *TargetYugabyteDB) initEventStatsByTableMetainfo(conn *pgx.Conn, migrationUUID uuid.UUID, tableNames []string, numChans int) error {
	ctx := context.Background()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("error creating tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, tableName := range tableNames {
		tableName := yb.qualifyTableName(tableName)
		rowCount, err := yb.getLiveMigrationMetaInfoByTable(conn, migrationUUID, tableName)
		if err != nil {
			return fmt.Errorf("error getting channels meta info for %s: %w", EVENT_CHANNELS_METADATA_TABLE_NAME, err)
		}
		if rowCount > 0 {
			log.Info(fmt.Sprintf("event stats for %s already created. Skipping init.", tableName))
		} else {
			for c := 0; c < numChans; c++ {
				insertStmt := fmt.Sprintf("INSERT INTO %s VALUES ('%s', '%s', %d, %d, %d, %d, %d)", EVENTS_PER_TABLE_METADATA_TABLE_NAME, migrationUUID, tableName, c, 0, 0, 0, 0)
				_, err := tx.Exec(ctx, insertStmt)
				if err != nil {
					return fmt.Errorf("error executing stmt - %v: %w", insertStmt, err)
				}
				log.Infof("created table wise event meta info: %s;", insertStmt)
			}
		}
	}
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("error committing tx: %w", err)
	}
	return nil
}

func (yb *TargetYugabyteDB) InitLiveMigrationState(migrationUUID uuid.UUID, numChans int, startClean bool, tableNames []string) error {
	err := yb.connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
		if startClean {
			err := yb.clearMigrationStateFromTable(conn, EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID)
			if err != nil {
				return false, fmt.Errorf("error clearing channels meta info for %s: %w", EVENT_CHANNELS_METADATA_TABLE_NAME, err)
			}
			err = yb.clearMigrationStateFromTable(conn, EVENTS_PER_TABLE_METADATA_TABLE_NAME, migrationUUID)
			if err != nil {
				return false, fmt.Errorf("error clearing meta info for %s: %w", EVENTS_PER_TABLE_METADATA_TABLE_NAME, err)
			}
		}
		err := yb.initChannelMetaInfo(conn, migrationUUID, numChans)
		if err != nil {
			return false, fmt.Errorf("error initializing channels meta info for %s: %w", EVENT_CHANNELS_METADATA_TABLE_NAME, err)
		}

		err = yb.initEventStatsByTableMetainfo(conn, migrationUUID, tableNames, numChans)
		if err != nil {
			return false, fmt.Errorf("error initializing event stats by table meta info for %s: %w", EVENTS_PER_TABLE_METADATA_TABLE_NAME, err)
		}
		return false, nil
	})
	return err
}

func (yb *TargetYugabyteDB) GetEventChannelsMetaInfo(migrationUUID uuid.UUID) (map[int]EventChannelMetaInfo, error) {
	metainfo := map[int]EventChannelMetaInfo{}

	query := fmt.Sprintf("SELECT channel_no, last_applied_vsn FROM %s where migration_uuid='%s'", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID)
	rows, err := yb.Conn().Query(context.Background(), query)
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

func (yb *TargetYugabyteDB) ImportBatch(batch Batch, args *ImportBatchArgs, exportDir string, tableSchema map[string]map[string]string) (int64, error) {
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
			if sqlname.IsReservedKeywordPG(colName) && colName[0:1] != `"` {
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
		case sqlname.IsReservedKeywordPG(colName):
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

func (yb *TargetYugabyteDB) RestoreSequences(sequencesLastVal map[string]int64) error {
	log.Infof("restoring sequences on target")
	batch := pgx.Batch{}
	restoreStmt := "SELECT pg_catalog.setval('%s', %d, true)"
	for sequenceName, lastValue := range sequencesLastVal {
		if lastValue == 0 {
			// TODO: can be valid for cases like cyclic sequences
			continue
		}
		// same function logic will work for sequences as well
		sequenceName = yb.qualifyTableName(sequenceName)
		log.Infof("restore sequence %s to %d", sequenceName, lastValue)
		batch.Queue(fmt.Sprintf(restoreStmt, sequenceName, lastValue))
	}

	err := yb.connPool.WithConn(func(conn *pgx.Conn) (retry bool, err error) {
		br := conn.SendBatch(context.Background(), &batch)
		for i := 0; i < batch.Len(); i++ {
			_, err := br.Exec()
			if err != nil {
				log.Errorf("error executing restore sequence stmt: %v", err)
				return false, fmt.Errorf("error executing restore sequence stmt: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			log.Errorf("error closing batch: %v", err)
			return false, fmt.Errorf("error closing batch: %w", err)
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("error restoring sequences: %w", err)
	}
	return err
}

/*
TODO(future): figure out the sql error codes for prepared statements which have become invalid
and needs to be prepared again
*/
func (yb *TargetYugabyteDB) ExecuteBatch(migrationUUID uuid.UUID, batch *EventBatch) error {
	log.Infof("executing batch of %d events", len(batch.Events))
	ybBatch := pgx.Batch{}
	stmtToPrepare := make(map[string]string)
	// processing batch events to convert into prepared or unprepared statements based on Op type
	for i := 0; i < len(batch.Events); i++ {
		event := batch.Events[i]
		if event.Op == "u" {
			stmt := event.GetSQLStmt(yb.tconf.Schema)
			ybBatch.Queue(stmt)
		} else {
			stmt := event.GetPreparedSQLStmt(yb.tconf.Schema)
			params := event.GetParams()
			if _, ok := stmtToPrepare[stmt]; !ok {
				stmtToPrepare[event.GetPreparedStmtName(yb.tconf.Schema)] = stmt
			}
			ybBatch.Queue(stmt, params...)
		}
	}

	err := yb.connPool.WithConn(func(conn *pgx.Conn) (retry bool, err error) {
		ctx := context.Background()
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return false, fmt.Errorf("error creating tx: %w", err)
		}
		defer tx.Rollback(ctx)

		for name, stmt := range stmtToPrepare {
			err := yb.connPool.PrepareStatement(conn, name, stmt)
			if err != nil {
				log.Errorf("error preparing stmt(%q): %v", stmt, err)
				return false, fmt.Errorf("error preparing stmt: %w", err)
			}
		}

		br := conn.SendBatch(ctx, &ybBatch)
		for i := 0; i < len(batch.Events); i++ {
			_, err := br.Exec()
			if err != nil {
				log.Errorf("error executing stmt for event with vsn(%d): %v", batch.Events[i].Vsn, err)
				return false, fmt.Errorf("error executing stmt for event with vsn(%d): %v", batch.Events[i].Vsn, err)
			}
		}
		if err = br.Close(); err != nil {
			log.Errorf("error closing batch: %v", err)
			return false, fmt.Errorf("error closing batch: %v", err)
		}

		updateVsnQuery := batch.GetChannelMetadataUpdateQuery(migrationUUID)
		res, err := tx.Exec(context.Background(), updateVsnQuery)
		if err != nil || res.RowsAffected() == 0 {
			log.Errorf("error executing stmt: %v, rowsAffected: %v", err, res.RowsAffected())
			return false, fmt.Errorf("failed to update vsn on target db via query-%s: %w, rowsAffected: %v",
				updateVsnQuery, err, res.RowsAffected())
		}
		log.Debugf("Updated event channel meta info with query = %s; rows Affected = %d", updateVsnQuery, res.RowsAffected())

		tableNames := batch.GetTableNames()
		for _, tableName := range tableNames {
			tableName := yb.qualifyTableName(tableName)
			updateTableStatsQuery := batch.GetQueriesToUpdateEventStatsByTable(migrationUUID, tableName)
			res, err = tx.Exec(context.Background(), updateTableStatsQuery)
			if err != nil || res.RowsAffected() == 0 {
				log.Errorf("error executing stmt: %v, rowsAffected: %v", err, res.RowsAffected())
				return false, fmt.Errorf("failed to update table stats on target db via query-%s: %w, rowsAffected: %v",
					updateTableStatsQuery, err, res.RowsAffected())
			}
			log.Debugf("Updated table stats meta info with query = %s; rows Affected = %d", updateTableStatsQuery, res.RowsAffected())
		}
		if err = tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("failed to commit transaction : %w", err)
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

	return nil
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

func (yb *TargetYugabyteDB) GetTotalNumOfEventsImportedByType(migrationUUID uuid.UUID) (int64, int64, int64, error) {
	query := fmt.Sprintf("SELECT SUM(num_inserts), SUM(num_updates), SUM(num_deletes) FROM %s where migration_uuid='%s'",
		EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID)
	var numInserts, numUpdates, numDeletes int64
	err := yb.Conn().QueryRow(context.Background(), query).Scan(&numInserts, &numUpdates, &numDeletes)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("error in getting import stats from target db: %w", err)
	}
	return numInserts, numUpdates, numDeletes, nil
}

func (yb *TargetYugabyteDB) GetDebeziumValueConverterSuite() map[string]tgtdbsuite.ConverterFn {
	return tgtdbsuite.YBValueConverterSuite
}

func (yb *TargetYugabyteDB) MaxBatchSizeInBytes() int64 {
	return 200 * 1024 * 1024 // 200 MB
}

func (yb *TargetYugabyteDB) GetImportedEventsStatsForTable(tableName string, migrationUUID uuid.UUID) (*EventCounter, error) {
	var eventCounter EventCounter
	tableName = yb.qualifyTableName(tableName)
	err := yb.connPool.WithConn(func(conn *pgx.Conn) (retry bool, err error) {
		query := fmt.Sprintf(`SELECT SUM(total_events), SUM(num_inserts), SUM(num_updates), SUM(num_deletes) FROM %s 
		WHERE table_name='%s' AND migration_uuid='%s'`, EVENTS_PER_TABLE_METADATA_TABLE_NAME, tableName, migrationUUID)
		log.Infof("query to get import stats for table %s: %s", tableName, query)
		err = conn.QueryRow(context.Background(), query).Scan(&eventCounter.TotalEvents,
			&eventCounter.NumInserts, &eventCounter.NumUpdates, &eventCounter.NumDeletes)
		return false, err
	})
	if err != nil {
		log.Errorf("error in getting import stats from target db: %v", err)
		return nil, fmt.Errorf("error in getting import stats from target db: %w", err)
	}
	log.Infof("import stats for table %s: %v", tableName, eventCounter)
	return &eventCounter, nil
}

func (yb *TargetYugabyteDB) GetImportedSnapshotRowCountForTable(tableName string) (int64, error) {
	var snapshotRowCount int64
	schema := yb.getTargetSchemaName(tableName)
	err := yb.connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
		query := fmt.Sprintf(`SELECT SUM(rows_imported) FROM %s where schema_name='%s' AND table_name='%s'`,
			BATCH_METADATA_TABLE_NAME, schema, tableName)
		log.Infof("query to get total row count for snapshot import of table %s: %s", tableName, query)
		err := conn.QueryRow(context.Background(), query).Scan(&snapshotRowCount)
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
