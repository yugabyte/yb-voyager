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
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	pgconn5 "github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jinzhu/copier"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	_ "github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/errs"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const (
	CPU_USAGE_USER_METRIC                  = "cpu_usage_user"
	CPU_USAGE_SYSTEM_METRIC                = "cpu_usage_system"
	TSERVER_ROOT_MEMORY_CONSUMPTION_METRIC = "tserver_root_memory_consumption"
	TSERVER_ROOT_MEMORY_SOFT_LIMIT_METRIC  = "tserver_root_memory_soft_limit"
	MEMORY_FREE_METRIC                     = "memory_free"
	MEMORY_TOTAL_METRIC                    = "memory_total"
	MEMORY_AVAILABLE_METRIC                = "memory_available"
)

type TargetYugabyteDB struct {
	sync.Mutex
	*AttributeNameRegistry
	tconf    *TargetConf
	db       *sql.DB
	conn_    *pgx.Conn
	connPool *ConnectionPool

	attrNames map[string][]string
}

func newTargetYugabyteDB(tconf *TargetConf) *TargetYugabyteDB {
	tdb := &TargetYugabyteDB{
		tconf:     tconf,
		attrNames: make(map[string][]string),
	}
	tdb.AttributeNameRegistry = NewAttributeNameRegistry(tdb, tconf)
	return tdb
}

func (yb *TargetYugabyteDB) Query(query string) (*sql.Rows, error) {
	return yb.db.Query(query)
}

func (yb *TargetYugabyteDB) QueryRow(query string) *sql.Row {
	return yb.db.QueryRow(query)
}

func (yb *TargetYugabyteDB) Exec(query string) (int64, error) {

	var rowsAffected int64

	res, err := yb.db.Exec(query)
	if err != nil {
		var pgErr *pgconn5.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Hint != "" || pgErr.Detail != "" {
				return rowsAffected, fmt.Errorf("run query %q on target %q: %w \nHINT: %s\nDETAIL: %s", query, yb.tconf.Host, err, pgErr.Hint, pgErr.Detail)
			}
		}
		return rowsAffected, fmt.Errorf("run query %q on target %q: %w", query, yb.tconf.Host, err)
	}
	rowsAffected, err = res.RowsAffected()
	if err != nil {
		return rowsAffected, fmt.Errorf("rowsAffected on query %q on target %q: %w", query, yb.tconf.Host, err)
	}
	return rowsAffected, err
}

func (yb *TargetYugabyteDB) WithTx(fn func(tx *sql.Tx) error) error {
	tx, err := yb.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction on target %q: %w", yb.tconf.Host, err)
	}
	defer tx.Rollback()
	err = fn(tx)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit transaction on target %q: %w", yb.tconf.Host, err)
	}
	return nil
}

func (yb *TargetYugabyteDB) Init() error {
	err := yb.connect()
	if err != nil {
		return err
	}

	if len(yb.tconf.SessionVars) == 0 {
		yb.tconf.SessionVars = getYBSessionInitScript(yb.tconf)
	}

	checkSchemaExistsQuery := fmt.Sprintf(
		"SELECT count(nspname) FROM pg_catalog.pg_namespace WHERE nspname = '%s';",
		yb.tconf.Schema)
	var cntSchemaName int
	if err = yb.QueryRow(checkSchemaExistsQuery).Scan(&cntSchemaName); err != nil {
		err = fmt.Errorf("run query %q on target %q to check schema exists: %s", checkSchemaExistsQuery, yb.tconf.Host, err)
	} else if cntSchemaName == 0 {
		err = fmt.Errorf("schema '%s' does not exist in target", yb.tconf.Schema)
	}
	return err
}

func (yb *TargetYugabyteDB) Finalize() {
	yb.disconnect()
}

func (yb *TargetYugabyteDB) reconnect() error {
	yb.Lock()
	defer yb.Unlock()

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
	var err error
	yb.db, err = sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("open connection to target db: %w", err)
	}
	// setting this to only 1, because this is used for adhoc queries.
	// We have a separate pool for importing data.
	yb.db.SetMaxOpenConns(1)
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		return fmt.Errorf("connect to target db: %w", err)
	}
	err = yb.setTargetSchema(conn)
	if err != nil {
		return fmt.Errorf("error setting target schema: %w", err)
	}
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
	yb.Lock()
	defer yb.Unlock()
	query := "SELECT setting FROM pg_settings WHERE name = 'server_version'"
	err := yb.QueryRow(query).Scan(&yb.tconf.DBVersion)
	if err != nil {
		utils.ErrExit("get target db version: %s", err)
	}
	return yb.tconf.DBVersion
}

func (yb *TargetYugabyteDB) PrepareForStreaming() {
	log.Infof("Preparing target DB for streaming - disable throttling")
	yb.connPool.DisableThrottling()
}

func (yb *TargetYugabyteDB) InitConnPool() error {
	loadBalancerUsed, confs, err := yb.GetYBServers()
	if err != nil {
		return fmt.Errorf("error fetching the yb servers: %v", err)
	}
	if loadBalancerUsed {
		utils.PrintAndLog(LB_WARN_MSG)
	}
	tconfs := yb.getTargetConfsAsPerLoadBalancerUsed(loadBalancerUsed, confs)
	var targetUriList []string
	for _, tconf := range tconfs {
		targetUriList = append(targetUriList, tconf.Uri)
	}
	log.Infof("targetUriList: %s", utils.GetRedactedURLs(targetUriList))

	if yb.tconf.Parallelism <= 0 {
		yb.tconf.Parallelism = fetchDefaultParallelJobs(tconfs, YB_DEFAULT_PARALLELISM_FACTOR)
		log.Infof("Using %d parallel jobs by default. Use --parallel-jobs to specify a custom value", yb.tconf.Parallelism)
	}

	if yb.tconf.EnableYBAdaptiveParallelism {
		if yb.tconf.MaxParallelism <= 0 {
			yb.tconf.MaxParallelism = yb.tconf.Parallelism * 2
		}
	} else {
		yb.tconf.MaxParallelism = yb.tconf.Parallelism
	}
	params := &ConnectionParams{
		NumConnections:    yb.tconf.Parallelism,
		NumMaxConnections: yb.tconf.MaxParallelism,
		ConnUriList:       targetUriList,
		SessionInitScript: yb.tconf.SessionVars,
	}
	yb.connPool = NewConnectionPool(params)
	redactedParams := &ConnectionParams{}
	//Whenever adding new fields to CONNECTION PARAMS check if that needs to be redacted while logging
	err = copier.Copy(redactedParams, params)
	if err != nil {
		log.Errorf("couldn't get the copy of connection params for logging: %v", err)
		return nil
	}
	redactedParams.ConnUriList = utils.GetRedactedURLs(redactedParams.ConnUriList)
	log.Info("Initialized connection pool with settings: ", spew.Sdump(redactedParams))
	return nil
}

/*
Example:
ERROR:  duplicate key value violates unique constraint "orders_pkey"
DETAIL:  Key (col1, col2)=(1, 2) already exists.
*/
const VIOLATES_UNIQUE_CONSTRAINT_ERROR_RETRYABLE_FAST_PATH = `violates unique constraint "%s"`
const VIOLATES_UNIQUE_CONSTRAINT_ERROR = "violates unique constraint"
const SYNTAX_ERROR = "syntax error at"
const RPC_MSG_LIMIT_ERROR = "Sending too long RPC message"
const INVALID_INPUT_SYNTAX_ERROR = "invalid input syntax"

var NonRetryCopyErrors = []string{
	INVALID_INPUT_SYNTAX_ERROR,
	VIOLATES_UNIQUE_CONSTRAINT_ERROR,
	SYNTAX_ERROR,
}

func (yb *TargetYugabyteDB) IsNonRetryableCopyError(err error) bool {
	NonRetryCopyErrorsYB := NonRetryCopyErrors
	NonRetryCopyErrorsYB = append(NonRetryCopyErrorsYB, RPC_MSG_LIMIT_ERROR)
	return err != nil && utils.ContainsAnySubstringFromSlice(NonRetryCopyErrorsYB, err.Error())
}

func (yb *TargetYugabyteDB) checkIfPrimaryKeyViolationError(err error, pkConstraintNames []string) bool {
	if err == nil {
		return false
	}
	// Check if the error is a primary key violation error.
	for _, pkConstraintName := range pkConstraintNames {
		pkViolationErr := fmt.Sprintf(VIOLATES_UNIQUE_CONSTRAINT_ERROR_RETRYABLE_FAST_PATH, pkConstraintName)
		if strings.Contains(err.Error(), pkViolationErr) {
			log.Infof("matched primary key violation error for constraint %q\nexpectedErr=%s, actualErr=%s\n",
				pkConstraintName, pkViolationErr, err.Error())
			return true
		}
	}

	log.Infof("not a primary key violation error, expected one of the following: %v, actualErr=%s", pkConstraintNames, err.Error())
	return false
}

func (yb *TargetYugabyteDB) GetAllSchemaNamesRaw() ([]string, error) {
	query := "SELECT schema_name FROM information_schema.schemata"
	rows, err := yb.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error in querying YB database for schema names: %w", err)
	}
	defer rows.Close()

	var schemaNames []string
	var schemaName string
	for rows.Next() {
		err = rows.Scan(&schemaName)
		if err != nil {
			return nil, fmt.Errorf("error in scanning query rows for schema names: %w", err)
		}
		schemaNames = append(schemaNames, schemaName)
	}
	log.Infof("Query found %d schemas in the YB db: %v", len(schemaNames), schemaNames)
	return schemaNames, nil
}

func (yb *TargetYugabyteDB) GetAllTableNamesRaw(schemaName string) ([]string, error) {
	query := fmt.Sprintf(`SELECT table_name
			  FROM information_schema.tables
			  WHERE table_type = 'BASE TABLE' AND
			        table_schema = '%s';`, schemaName)

	rows, err := yb.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error in querying(%q) YB database for table names: %w", query, err)
	}
	defer rows.Close()

	var tableNames []string
	var tableName string

	for rows.Next() {
		err = rows.Scan(&tableName)
		if err != nil {
			return nil, fmt.Errorf("error in scanning query rows for table names: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}
	log.Infof("Query found %d tables in the YB db: %v", len(tableNames), tableNames)
	return tableNames, nil
}

// GetAllSequencesRaw returns all the sequence names in the database for the schema
func (yb *TargetYugabyteDB) GetAllSequencesRaw(schemaName string) ([]string, error) {
	var sequenceNames []string
	query := fmt.Sprintf(`SELECT sequencename FROM pg_sequences where schemaname = '%s';`, schemaName)
	rows, err := yb.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error in querying(%q) source database for sequence names: %v", query, err)
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
	if rows.Err() != nil {
		return nil, fmt.Errorf("error in scanning query rows for sequence names: %v", rows.Err())
	}
	return sequenceNames, nil
}

// The _v2 is appended in the table name so that the import code doesn't
// try to use the similar table created by the voyager 1.3 and earlier.
// Voyager 1.4 uses import data state format that is incompatible from
// the earlier versions.
const BATCH_METADATA_TABLE_SCHEMA = "ybvoyager_metadata"
const BATCH_METADATA_TABLE_NAME = BATCH_METADATA_TABLE_SCHEMA + "." + "ybvoyager_import_data_batches_metainfo_v3"
const EVENT_CHANNELS_METADATA_TABLE_NAME = BATCH_METADATA_TABLE_SCHEMA + "." + "ybvoyager_import_data_event_channels_metainfo"
const EVENTS_PER_TABLE_METADATA_TABLE_NAME = BATCH_METADATA_TABLE_SCHEMA + "." + "ybvoyager_imported_event_count_by_table"
const YB_DEFAULT_PARALLELISM_FACTOR = 2 // factor for default parallelism in case fetchDefaultParallelJobs() is not able to get the no of cores
const ALTER_QUERY_RETRY_COUNT = 5

func (yb *TargetYugabyteDB) CreateVoyagerSchema() error {
	cmds := []string{
		fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s;`, BATCH_METADATA_TABLE_SCHEMA),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			migration_uuid uuid,
			data_file_name TEXT,
			batch_number INT,
			schema_name TEXT,
			table_name TEXT,
			rows_imported BIGINT,
			PRIMARY KEY (migration_uuid, data_file_name, batch_number, schema_name, table_name)
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
			table_name TEXT, 
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
			_, err = yb.Exec(cmd)
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

// GetPrimaryKeyColumns returns the subset of `columns` that belong to the
// primary‑key definition of the given table.
func (yb *TargetYugabyteDB) GetPrimaryKeyColumns(table sqlname.NameTuple) ([]string, error) {
	var primaryKeyColumns []string
	schemaName, tableName := table.ForCatalogQuery()
	query := fmt.Sprintf(`
		SELECT a.attname
		FROM pg_index i
		JOIN pg_class      c ON c.oid = i.indrelid
		JOIN pg_namespace  n ON n.oid = c.relnamespace
		JOIN pg_attribute  a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
		WHERE n.nspname = '%s'
			AND c.relname  = '%s'
			AND i.indisprimary;`, schemaName, tableName)

	rows, err := yb.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query PK columns for %s.%s: %w", schemaName, tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, fmt.Errorf("scan PK column: %w", err)
		}
		primaryKeyColumns = append(primaryKeyColumns, col)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return primaryKeyColumns, nil
}

// GetPrimaryKeyConstraintName returns the name of the primary key constraint for the given table.
// If the table does not have a primary key, it returns an empty string and no error
// If the table is partitioned, it returns the list of primary key constraint names for all partitions
//
//	If multi level partitioning, need to return all the primary key constraint names for all levels(recursively)
func (yb *TargetYugabyteDB) GetPrimaryKeyConstraintNames(table sqlname.NameTuple) ([]string, error) {
	recursiveCTEQuery := fmt.Sprintf(`
WITH RECURSIVE all_parts AS (
  -- 1) Seed: include the parent table's OID
  SELECT oid AS tbl_oid
    FROM pg_class
   WHERE oid = '%s'::regclass

  UNION ALL

  -- 2) Recursion: find each table's immediate partitions
  SELECT inh.inhrelid
    FROM pg_inherits inh
    JOIN all_parts ap ON inh.inhparent = ap.tbl_oid
)
-- 3) Pull every primary-key constraint on the parent or any partition
SELECT
  c.conname AS constraint_name
FROM pg_constraint c
JOIN all_parts ap
  ON ap.tbl_oid = c.conrelid   -- only constraints on our collected tables
WHERE c.contype = 'p';          -- filter for PRIMARY KEY only`, table.ForOutput()) // use the fully qualified table name

	log.Infof("Querying for primary key constraint names for table %s: %s", table.ForMinOutput(), recursiveCTEQuery)
	var constraintNames []string
	rows, err := yb.Query(recursiveCTEQuery)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No primary key constraint found
		}
		return nil, fmt.Errorf("query PK constraint name for table %s: %w", table.ForMinOutput(), err)
	}
	defer rows.Close()

	for rows.Next() {
		var cn string
		if err := rows.Scan(&cn); err != nil {
			return nil, fmt.Errorf("scan PK constraint name for table %s: %w", table.ForMinOutput(), err)
		}
		constraintNames = append(constraintNames, cn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over PK constraint names for table %s: %w", table.ForMinOutput(), err)
	}

	log.Infof("found %d primary key constraint(s) for table %s: %v", len(constraintNames), table.ForMinOutput(), constraintNames)
	return constraintNames, nil
}

func (yb *TargetYugabyteDB) GetNonEmptyTables(tables []sqlname.NameTuple) []sqlname.NameTuple {
	result := []sqlname.NameTuple{}

	for _, table := range tables {
		log.Infof("checking if table %q is empty.", table)
		tmp := false
		stmt := fmt.Sprintf("SELECT TRUE FROM %s LIMIT 1;", table.ForUserQuery())
		err := yb.QueryRow(stmt).Scan(&tmp)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			utils.ErrExit("failed to check whether table is empty: %q: %s", table, err)
		}
		result = append(result, table)
	}
	log.Infof("non empty tables: %v", result)
	return result
}

func (yb *TargetYugabyteDB) TruncateTables(tables []sqlname.NameTuple) error {
	tableNames := lo.Map(tables, func(nt sqlname.NameTuple, _ int) string {
		return nt.ForUserQuery()
	})
	commaSeparatedTableNames := strings.Join(tableNames, ", ")
	query := fmt.Sprintf("TRUNCATE TABLE %s", commaSeparatedTableNames)
	_, err := yb.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

/*
ImportBatch function handles variety of cases for importing a batch into YugabyteDB
1. Normal Mode - Importing a batch using COPY command with transaction.
2. Fast Path Mode - Importing a batch using COPY command without transaction.
3. Fast Path Recovery Mode - Importing a batch using COPY command without transaction but conflict handling.
*/
func (yb *TargetYugabyteDB) ImportBatch(batch Batch, args *ImportBatchArgs,
	exportDir string, tableSchema map[string]map[string]string, isRecoveryCandidate bool) (int64, error, bool) {

	var rowsAffected int64
	var err error
	var isPartialBatchIngestionPossibleOnError bool
	copyFn := func(conn *pgx.Conn) (bool, error) {
		if args.ShouldUseFastPath() {
			if !isRecoveryCandidate {
				rowsAffected, err = yb.importBatchFast(conn, batch, args)
			} else {
				rowsAffected, err = yb.importBatchFastRecover(conn, batch, args)
			}
			// if we get an error in the fast path (either COPY w/o txn or recovery path where we run one COPY per row),
			// it is likely that there was partial ingestion of the batch.
			// This is not necessarily always the case. For instance, if there is an error while reading the file,
			// (i.e. before even executing COPY), the entire batch was not ingested, so it's not really a case of partial ingeetion.
			// However, if it's a resumption case (i.e. batch is retried after a stop-start), then, even if it fails while reading the file,
			// it is likely that the batch was partially ingested in the previous attempt, so it is indeed a case of partial ingestion.
			// Since this is hard to determine, we always assume that if there is an error in the fast path,
			// it is likely that the batch was partially ingested.
			isPartialBatchIngestionPossibleOnError = lo.Ternary(err != nil, true, false)
		} else {
			// Normal mode, don't require handling recovery separately as it is transactional hence no partial ingestion
			rowsAffected, err = yb.importBatch(conn, batch, args)
		}
		return false, err // Retries are now implemented in the caller.
	}
	err = yb.connPool.WithConn(copyFn)
	return rowsAffected, err, isPartialBatchIngestionPossibleOnError
}

func (yb *TargetYugabyteDB) importBatch(conn *pgx.Conn, batch Batch, args *ImportBatchArgs) (rowsAffected int64, err error) {
	log.Infof("importing %q using COPY command normal path(transactional)", batch.GetFilePath())
	// NOTE: DO NOT DEFINE A NEW err VARIABLE IN THIS FUNCTION. ELSE, IT WILL MASK THE err FROM RETURN LIST.
	ctx := context.Background()
	var tx pgx.Tx
	tx, err = conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, newImportBatchErrorPgYb(err, batch,
			errs.IMPORT_BATCH_ERROR_FLOW_COPY_NORMAL,
			errs.IMPORT_BATCH_ERROR_STEP_BEGIN_TXN)
	}
	defer func() {
		var err2 error
		if err != nil {
			err2 = tx.Rollback(ctx)
			if err2 != nil {
				rowsAffected = 0
				err = newImportBatchErrorPgYb(fmt.Errorf("%w (while processing %s)", err2, err), batch,
					errs.IMPORT_BATCH_ERROR_FLOW_COPY_NORMAL,
					errs.IMPORT_BATCH_ERROR_STEP_ROLLBACK_TXN)
			}
		} else {
			err2 = tx.Commit(ctx)
			if err2 != nil {
				rowsAffected = 0
				err = newImportBatchErrorPgYb(err2, batch,
					errs.IMPORT_BATCH_ERROR_FLOW_COPY_NORMAL,
					errs.IMPORT_BATCH_ERROR_STEP_COMMIT_TXN)
			}
		}
	}()

	// using the conn on which the transaction is executing which should ensure -
	// all DB operations inside copyBatchCore will be a part of the transaction
	rowsAffected, err = yb.copyBatchCore(tx.Conn(), batch, args)
	return rowsAffected, err
}

func (yb *TargetYugabyteDB) importBatchFast(conn *pgx.Conn, batch Batch, args *ImportBatchArgs) (int64, error) {
	log.Infof("importing %q using COPY command fast path(non transactional)", batch.GetFilePath())
	// running copy without transaction
	rowsAffected, err := yb.copyBatchCore(conn, batch, args)

	/*
		Check if the error is violates unique constraint then use the fast path recovery to ingest this batch
		Case: If the table already has the data(before import started) which is being imported

		Lets say the importBatchFastRecover fails with some trasient DB error. In that case,
		caller(fileTaskImporter.importBatch() function) takes care of retrying with importBatchFastRecover
	*/
	if yb.checkIfPrimaryKeyViolationError(err, args.PKConstraintNames) {
		log.Infof("falling back to importBatchFastRecover for batch %q: %s", batch.GetFilePath(), err.Error())
		return yb.importBatchFastRecover(conn, batch, args)
	}

	return rowsAffected, err
}

func (yb *TargetYugabyteDB) copyBatchCore(conn *pgx.Conn, batch Batch, args *ImportBatchArgs) (int64, error) {
	// 1. Open the batch file
	file, err := batch.Open()
	if err != nil {
		err = newImportBatchErrorPgYb(err, batch,
			lo.Ternary(args.ShouldUseFastPath(), errs.IMPORT_BATCH_ERROR_FLOW_COPY_FAST, errs.IMPORT_BATCH_ERROR_FLOW_COPY_NORMAL),
			errs.IMPORT_BATCH_ERROR_STEP_OPEN_BATCH)
		return 0, err
	}
	defer file.Close()

	// 2. setting the schema so that COPY command can acesss the table
	// Q: If we set the schema for this batch on this conn, will it impact others using the same conn from pool later?
	yb.setTargetSchema(conn)

	// 3. Check if the split is already imported.
	alreadyImported, rowsAffected, err := yb.isBatchAlreadyImported(conn, batch)
	if err != nil {
		err = newImportBatchErrorPgYb(err, batch,
			lo.Ternary(args.ShouldUseFastPath(), errs.IMPORT_BATCH_ERROR_FLOW_COPY_FAST, errs.IMPORT_BATCH_ERROR_FLOW_COPY_NORMAL),
			errs.IMPORT_BATCH_ERROR_STEP_CHECK_BATCH_ALREADY_IMPORTED)
		return 0, err
	}
	if alreadyImported {
		log.Infof("batch %q already imported, skipping", batch.GetFilePath())
		return rowsAffected, nil
	}

	// 4. Import the batch using COPY command.
	var res pgconn.CommandTag
	var copyCommand string

	// TODO: maybe move this check into the args struct methods and just call GetYBCopyStatement()
	if args.ShouldUseFastPath() {
		copyCommand = args.GetYBNonTxnCopyStatement()
	} else {
		copyCommand = args.GetYBTxnCopyStatement()
	}

	log.Infof("Importing %q using COPY command: [%s]", batch.GetFilePath(), copyCommand)
	res, err = conn.PgConn().CopyFrom(context.Background(), file, copyCommand)
	if err != nil {
		err = newImportBatchErrorPgYb(err, batch,
			lo.Ternary(args.ShouldUseFastPath(), errs.IMPORT_BATCH_ERROR_FLOW_COPY_FAST, errs.IMPORT_BATCH_ERROR_FLOW_COPY_NORMAL),
			errs.IMPORT_BATCH_ERROR_STEP_COPY)

		return res.RowsAffected(), err
	}

	// 5. Record the import in the DB.
	err = yb.recordEntryInDB(conn, batch, res.RowsAffected())
	if err != nil {
		err = newImportBatchErrorPgYb(err, batch,
			lo.Ternary(args.ShouldUseFastPath(), errs.IMPORT_BATCH_ERROR_FLOW_COPY_FAST, errs.IMPORT_BATCH_ERROR_FLOW_COPY_NORMAL),
			errs.IMPORT_BATCH_ERROR_STEP_METADATA_ENTRY)
		return res.RowsAffected(), err
	}
	return res.RowsAffected(), nil
}

// importBatchFastRecover is used to import a batch which was previously tried via fast path but failed
func (yb *TargetYugabyteDB) importBatchFastRecover(conn *pgx.Conn, batch Batch, args *ImportBatchArgs) (int64, error) {
	log.Infof("importing %q using COPY command fast path with recovery", batch.GetFilePath())
	// 1. Check if the split is already imported.
	alreadyImported, rowsAffected, err := yb.isBatchAlreadyImported(conn, batch)
	if err != nil {
		return 0, newImportBatchErrorPgYb(err, batch,
			errs.IMPORT_BATCH_ERROR_FLOW_COPY_RECOVER,
			errs.IMPORT_BATCH_ERROR_STEP_CHECK_BATCH_ALREADY_IMPORTED)
	}
	if alreadyImported {
		log.Infof("batch %q already imported, skipping fast recover", batch.GetFilePath())
		return rowsAffected, nil
	}

	// 2. Open the batch file as datafile
	df, err := batch.OpenAsDataFile()
	if err != nil {
		return 0, newImportBatchErrorPgYb(err, batch,
			errs.IMPORT_BATCH_ERROR_FLOW_COPY_RECOVER,
			errs.IMPORT_BATCH_ERROR_STEP_OPEN_BATCH)
	}
	defer df.Close()

	// 3. Read the batch file line by line(row) and build INSERT statment for it based on PK conflict action
	var rowsIgnored int64 = 0
	rowsAffected = 0 // reset to 0
	copyCommand := args.GetYBTxnCopyStatement()
	copyHeader := ""
	if args.HasHeader {
		copyHeader = df.GetHeader()
	}
	for {
		line, _, readLinErr := df.NextLine()
		if readLinErr != nil && readLinErr != io.EOF {
			return 0, newImportBatchErrorPgYb(err, batch,
				errs.IMPORT_BATCH_ERROR_FLOW_COPY_RECOVER,
				errs.IMPORT_BATCH_ERROR_STEP_READ_LINE_BATCH)
		}

		/*
			Both the cases are handled:
			1. line="" + EOF error
			2. line!=""(last line) + EOF error
		*/
		if line == "" { // handles case 1
			if readLinErr == io.EOF {
				break
			} else {
				// skipping if any empty line (not expected from batch file)
				continue
			}
		}

		var singleLineReader io.Reader
		if copyHeader != "" {
			singleLineReader = strings.NewReader(copyHeader + "\n" + line)
		} else {
			singleLineReader = strings.NewReader(line)
		}

		// 5. Execute the COPY statement with the line read from the file
		res, err := conn.PgConn().CopyFrom(context.Background(), singleLineReader, copyCommand)
		if err != nil {
			// Ignore err if its VIOLATES_UNIQUE_CONSTRAINT_ERROR_RETRYABLE_FAST_PATH only
			if yb.checkIfPrimaryKeyViolationError(err, args.PKConstraintNames) {
				// logging lineNum might not be useful as batches are truncated later on
				log.Debugf("ignoring error %s for line=%q in batch %q", err.Error(), line, batch.GetFilePath())
				rowsIgnored++ // increment before continuing to next line
				continue
			}

			err = newImportBatchErrorPgYb(err, batch,
				errs.IMPORT_BATCH_ERROR_FLOW_COPY_RECOVER,
				errs.IMPORT_BATCH_ERROR_STEP_COPY)
			return rowsAffected + rowsIgnored, err
		}

		// At this point, rowsAffected should be 1 always
		if res.RowsAffected() > 0 {
			rowsAffected++
		} else {
			// since at this point it is not expected to have 0 rows affected, adding warning to logs
			rowsIgnored++
			log.Warnf("Unexpected: COPY command for line=%q in batch %s returned 0 rows affected which is not expected", line, batch.GetFilePath())
		}

		if readLinErr == io.EOF { // handles case 2
			log.Infof("reached end of file %s", batch.GetFilePath())
			break
		}
	}

	totalRowsInBatch := rowsAffected + rowsIgnored

	// 6. Update the Metadata about the batch imported
	// account for rowsIgnored and rowsAffected both, as partial ingestion in last run didn't update the metadata
	err = yb.recordEntryInDB(conn, batch, totalRowsInBatch)
	if err != nil {
		return 0, newImportBatchErrorPgYb(err, batch,
			errs.IMPORT_BATCH_ERROR_FLOW_COPY_RECOVER,
			errs.IMPORT_BATCH_ERROR_STEP_METADATA_ENTRY)
	}

	// 7. log the summary: how many conflicts, how many inserted, how many update/upserted
	log.Infof("(recovery) [conflict_action=%s] %q => %d rows inserted, %d rows ignored",
		args.PKConflictAction, batch.GetFilePath(), rowsAffected, rowsIgnored)
	log.Debugf("(recovery) [conflict_action=%s] %q => insert stmt: %s, rows inserted: %d, rows ignored: %d",
		args.PKConflictAction, batch.GetFilePath(), copyCommand, rowsAffected, rowsIgnored)

	// 8. returns total number of rows so that stats reporting(import data status) is consistent
	return totalRowsInBatch, nil
}

func newImportBatchErrorPgYb(underlyingErr error, batch Batch, flow string, step string) errs.ImportBatchError {
	dbContext := map[string]string{}
	var pgerr *pgconn.PgError
	if errors.As(underlyingErr, &pgerr) {
		if pgerr.Where != "" {
			dbContext["where"] = pgerr.Where
		}
	}

	return errs.NewImportBatchError(
		batch.GetTableName(),
		batch.GetFilePath(),
		underlyingErr,
		flow,
		step,
		dbContext)
}

func (yb *TargetYugabyteDB) GetListOfTableAttributes(nt sqlname.NameTuple) ([]string, error) {
	schemaName, tableName := nt.ForCatalogQuery()
	var result []string
	// The hint /*+set(enable_nestloop off)*/ is used to disable the nested loop join in the query. It makes the query faster.
	// Without this import data was taking atleast 5 mins to start picking up events in the CDC phase.
	query := fmt.Sprintf(
		"/*+set(enable_nestloop off)*/ SELECT column_name FROM information_schema.columns WHERE table_schema = '%s' AND table_name ILIKE '%s'",
		schemaName, tableName)
	rows, err := yb.Query(query)
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
		seqName, err := namereg.NameReg.LookupTableName(sequenceName)
		if err != nil {
			return fmt.Errorf("error looking up sequence name %q: %w", sequenceName, err)
		}
		sequenceName := seqName.ForUserQuery()
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
	log.Infof("executing batch(%s) of %d events", batch.ID(), len(batch.Events))
	ybBatch := pgx.Batch{}
	stmtToPrepare := make(map[string]string)
	// processing batch events to convert into prepared or unprepared statements based on Op type
	for i := 0; i < len(batch.Events); i++ {
		event := batch.Events[i]
		if event.Op == "u" {
			stmt, err := event.GetSQLStmt(yb)
			if err != nil {
				return fmt.Errorf("get sql stmt: %w", err)
			}
			ybBatch.Queue(stmt)
			log.Debugf("SQL statement: Batch(%s): Event(%d): [%s]", batch.ID(), event.Vsn, stmt)
		} else {
			stmt, err := event.GetPreparedSQLStmt(yb, yb.tconf.TargetDBType)
			if err != nil {
				return fmt.Errorf("get prepared sql stmt: %w", err)
			}
			psName := event.GetPreparedStmtName()
			params := event.GetParams()
			if _, ok := stmtToPrepare[psName]; !ok {
				stmtToPrepare[psName] = stmt
			}
			ybBatch.Queue(psName, params...)
			log.Debugf("SQL statement: Batch(%s): Event(%d): PREPARED STMT:[%s] PARAMS:[%s]", batch.ID(), event.Vsn, stmt, event.GetParamsString())
		}
	}

	err := yb.connPool.WithConn(func(conn *pgx.Conn) (retry bool, err error) {
		ctx := context.Background()
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return false, fmt.Errorf("error creating tx: %w", err)
		}
		defer func() {
			errRollBack := tx.Rollback(ctx)
			if errRollBack != nil && errRollBack != pgx.ErrTxClosed {
				log.Errorf("error rolling back tx for batch id (%s): %v", batch.ID(), err)
			}
		}()

		for name, stmt := range stmtToPrepare {
			err := yb.connPool.PrepareStatement(conn, name, stmt)
			if err != nil {
				log.Errorf("error preparing stmt(%q): %v", stmt, err)
				return false, fmt.Errorf("error preparing stmt: %w", err)
			}
		}

		// This is an additional safety net to workaround
		// issue in YB where in batched execution, transactions can be retried partially, breaking atomicity.
		// SELECT 1 causes the ysql layer to record that data was sent back to the user, thereby, preventing retries
		// https://yugabyte.slack.com/archives/CAR5BCH29/p1708320808330589
		res, err := tx.Exec(ctx, "SELECT 1")
		if err != nil || res.RowsAffected() == 0 {
			log.Errorf("error executing stmt: %v, rowsAffected: %v", err, res.RowsAffected())
			return false, fmt.Errorf("failed to run SELECT 1 query: %w, rowsAffected: %v",
				err, res.RowsAffected())
		}
		var rowsAffectedInserts, rowsAffectedDeletes, rowsAffectedUpdates int64
		br := tx.SendBatch(ctx, &ybBatch)
		closeBatch := func() error {
			if closeErr := br.Close(); closeErr != nil {
				log.Errorf("error closing batch(%s): %v", batch.ID(), closeErr)
				return closeErr
			}
			return nil
		}
		for i := 0; i < len(batch.Events); i++ {
			res, err := br.Exec()
			if err != nil {
				// When using pgx SendBatch, there can be two types of errors thrown:
				// 1. Error while preparing the statement - this is preprocessing (parsing, preparinng statements, etc)
				//	that pgx will do before sending the batch. Examples - syntax error.
				//  No matter which statement in the batch has an issue, the error will be thrown on calling br.Exec() for the first time.
				// 2. Error while executing the statement - this is the actual execution of the statement, and the error comes from the DB.
				// 	Examples - constraint violation, etc.
				//  In this case, we get the error on the appropriate br.Exec() call associated with the statement that failed.
				// Therefore, if error is thrown on the first br.Exec() call, it could be either of the above cases.
				// Reference - https://github.com/jackc/pgx/issues/872
				// This ideally needs to be fixed in pgx library.
				errorMsg := ""
				if i == 0 {
					errorMsg = fmt.Sprintf("error preparing statements for events in batch (%s) or when executing event with vsn(%d)", batch.ID(), batch.Events[i].Vsn)
					log.Errorf("Event VSNs in batch(%s): %v", batch.ID(), batch.GetAllVsns())
				} else {
					errorMsg = fmt.Sprintf("error executing stmt for event with vsn(%d) in batch(%s)", batch.Events[i].Vsn, batch.ID())
				}
				log.Errorf("%s : %v", errorMsg, err)
				closeBatch()
				return false, fmt.Errorf("%s: %w", errorMsg, err)
			}
			switch true {
			case res.Insert():
				rowsAffectedInserts += res.RowsAffected()
			case res.Delete():
				rowsAffectedDeletes += res.RowsAffected()
			case res.Update():
				rowsAffectedUpdates += res.RowsAffected()
			}
			if res.RowsAffected() != 1 {
				log.Warnf("unexpected rows affected for event with vsn(%d) in batch(%s): %d", batch.Events[i].Vsn, batch.ID(), res.RowsAffected())
			}
		}
		err = closeBatch()
		if err != nil {
			return false, err
		}

		updateVsnQuery := batch.GetChannelMetadataUpdateQuery(migrationUUID)
		res, err = tx.Exec(context.Background(), updateVsnQuery)
		if err != nil || res.RowsAffected() == 0 {
			log.Errorf("error executing stmt for batch(%s): %v, rowsAffected: %v", batch.ID(), err, res.RowsAffected())
			return false, fmt.Errorf("failed to update vsn on target db via query-%s: %w, rowsAffected: %v",
				updateVsnQuery, err, res.RowsAffected())
		}
		log.Debugf("Updated event channel meta info with query = %s; rows Affected = %d", updateVsnQuery, res.RowsAffected())

		tableNames := batch.GetTableNames()
		for _, tableName := range tableNames {
			updateTableStatsQuery := batch.GetQueriesToUpdateEventStatsByTable(migrationUUID, tableName)
			res, err = tx.Exec(context.Background(), updateTableStatsQuery)
			if err != nil {
				log.Errorf("error executing stmt: %v, rowsAffected: %v", err, res.RowsAffected())
				return false, fmt.Errorf("failed to update table stats on target db via query-%s: %w, rowsAffected: %v",
					updateTableStatsQuery, err, res.RowsAffected())
			}
			if res.RowsAffected() == 0 {
				insertTableStatsQuery := batch.GetQueriesToInsertEventStatsByTable(migrationUUID, tableName)
				res, err = tx.Exec(context.Background(), insertTableStatsQuery)
				if err != nil {
					log.Errorf("error executing stmt: %v, rowsAffected: %v", err, res.RowsAffected())
					return false, fmt.Errorf("failed to insert table stats on target db via query-%s: %w, rowsAffected: %v",
						updateTableStatsQuery, err, res.RowsAffected())
				}
			}
			log.Debugf("Updated table stats meta info with query = %s; rows Affected = %d", updateTableStatsQuery, res.RowsAffected())
		}
		if err = tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("failed to commit transaction : %w", err)
		}
		logDiscrepancyInEventBatchIfAny(batch, rowsAffectedInserts, rowsAffectedDeletes, rowsAffectedUpdates)
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

func logDiscrepancyInEventBatchIfAny(batch *EventBatch, rowsAffectedInserts, rowsAffectedDeletes, rowsAffectedUpdates int64) {
	if !(rowsAffectedInserts == batch.EventCounts.NumInserts &&
		rowsAffectedDeletes == batch.EventCounts.NumDeletes &&
		rowsAffectedUpdates == batch.EventCounts.NumUpdates) {
		var vsns []int64
		for _, e := range batch.Events {
			vsns = append(vsns, e.Vsn)
		}
		log.Warnf("Discrepancy in committed batch(%s) with inserts=%d, deletes=%d and updates=%d: got rowsAffectedInserts=%d, rowsAffectedDeletes=%d rowsAffectedUpdates=%d. Vsns in batch %v",
			batch.ID(), batch.EventCounts.NumInserts, batch.EventCounts.NumDeletes, batch.EventCounts.NumUpdates, rowsAffectedInserts, rowsAffectedDeletes, rowsAffectedUpdates, vsns)
	}
}

//==============================================================================

const (
	LB_WARN_MSG = "--target-db-host was detected as a load balancer IP which will be used to create connections for data import.\n" +
		"\t To explicitly specify the servers to be used, refer to the `target-endpoints` flag.\n"

	GET_YB_SERVERS_QUERY = "SELECT host, port, num_connections, node_type, cloud, region, zone, public_ip FROM yb_servers()"
)

func (yb *TargetYugabyteDB) GetYBServers() (bool, []*TargetConf, error) {
	var tconfs []*TargetConf
	var loadBalancerUsed bool

	tconf := yb.tconf

	if tconf.TargetEndpoints != "" {
		msg := fmt.Sprintf("given yb-servers for import data: %q\n", tconf.TargetEndpoints)
		log.Infof(msg)

		ybServers := utils.CsvStringToSlice(tconf.TargetEndpoints)
		for _, ybServer := range ybServers {
			clone := tconf.Clone()

			if strings.Contains(ybServer, ":") {
				clone.Host = strings.Split(ybServer, ":")[0]
				var err error
				clone.Port, err = strconv.Atoi(strings.Split(ybServer, ":")[1])

				if err != nil {
					return false, nil, fmt.Errorf("error in parsing useYbServers flag: %v", err)
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
			return false, nil, fmt.Errorf("Unable to connect to database: %v", err)
		}
		defer conn.Close(context.Background())

		rows, err := conn.Query(context.Background(), GET_YB_SERVERS_QUERY)
		if err != nil {
			return false, nil, fmt.Errorf("error in query rows from yb_servers(): %v", err)
		}
		defer rows.Close()

		var hostPorts []string
		for rows.Next() {
			clone := tconf.Clone()
			var host, nodeType, cloud, region, zone, public_ip string
			var port, num_conns int
			if err := rows.Scan(&host, &port, &num_conns,
				&nodeType, &cloud, &region, &zone, &public_ip); err != nil {
				return false, nil, fmt.Errorf("error in scanning rows of yb_servers(): %v", err)
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
	return loadBalancerUsed, tconfs, nil
}

func (yb *TargetYugabyteDB) getTargetConfsAsPerLoadBalancerUsed(loadBalancerUsed bool, confs []*TargetConf) []*TargetConf {
	if loadBalancerUsed { // if load balancer is used no need to check direct connectivity
		return []*TargetConf{yb.tconf}
	} else {
		return testAndFilterYbServers(confs)
	}
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

func (yb *TargetYugabyteDB) GetCallhomeTargetDBInfo() *callhome.TargetDBDetails {
	loadBalancerUsed, actualTconfs, err := yb.GetYBServers()
	if err != nil {
		log.Errorf("callhome error fetching yb servers: %v", err)
	}
	confs := yb.getTargetConfsAsPerLoadBalancerUsed(loadBalancerUsed, actualTconfs)
	totalCores, _ := fetchCores(confs) // no need to handle error in case we couldn't fine cores
	return &callhome.TargetDBDetails{
		NodeCount: len(actualTconfs),
		Cores:     totalCores,
		DBVersion: yb.GetVersion(),
	}
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

func fetchCores(tconfs []*TargetConf) (int, error) {
	targetCores := 0
	totalCores := 0
	for _, tconf := range tconfs {
		log.Infof("Determining CPU core count on: %s", utils.GetRedactedURLs([]string{tconf.Uri})[0])
		conn, err := pgx.Connect(context.Background(), tconf.Uri)
		if err != nil {
			log.Warnf("Unable to reach target while querying cores: %v", err)
			return 0, err
		}
		defer conn.Close(context.Background())

		cmd := "CREATE TEMP TABLE yb_voyager_cores(num_cores int);"
		_, err = conn.Exec(context.Background(), cmd)
		if err != nil {
			log.Warnf("Unable to create tables on target DB: %v", err)
			return 0, err
		}

		cmd = "COPY yb_voyager_cores(num_cores) FROM PROGRAM 'grep processor /proc/cpuinfo|wc -l';"
		_, err = conn.Exec(context.Background(), cmd)
		if err != nil {
			log.Warnf("Error while running query %s on host %s: %v", cmd, utils.GetRedactedURLs([]string{tconf.Uri}), err)
			return 0, err
		}

		cmd = "SELECT num_cores FROM yb_voyager_cores;"
		if err = conn.QueryRow(context.Background(), cmd).Scan(&targetCores); err != nil {
			log.Warnf("Error while running query %s: %v", cmd, err)
			return 0, err
		}
		totalCores += targetCores
	}
	return totalCores, nil
}

func fetchDefaultParallelJobs(tconfs []*TargetConf, defaultParallelismFactor int) int {
	totalCores, err := fetchCores(tconfs)
	if err != nil {
		defaultParallelJobs := len(tconfs) * defaultParallelismFactor
		log.Errorf("error while fetching the cores information and using default parallelism: %v : %v ", defaultParallelJobs, err)
		return defaultParallelJobs
	}
	if totalCores == 0 { //if target is running on MacOS, we are unable to determine totalCores
		return 3
	}
	if tconfs[0].TargetDBType == YUGABYTEDB {
		return totalCores / 4
	}
	return totalCores / 2
}

// import session parameters
const (
	SET_CLIENT_ENCODING_TO_UTF8           = "SET client_encoding TO 'UTF8'"
	SET_SESSION_REPLICATE_ROLE_TO_REPLICA = "SET session_replication_role TO replica" //Disable triggers or fkeys constraint checks.
	SET_YB_ENABLE_UPSERT_MODE             = "SET yb_enable_upsert_mode to true"
	SET_YB_DISABLE_TRANSACTIONAL_WRITES   = "SET yb_disable_transactional_writes to true" // Disable transactions to improve ingestion throughput.
	SET_YB_FAST_PATH_FOR_COLOCATED_COPY   = "SET yb_fast_path_for_colocated_copy=true"
	// The "SELECT 1" workaround introduced in ExecuteBatch does not work if isolation level is read_committed. Therefore, for now, we are forcing REPEATABLE READ.
	SET_DEFAULT_ISOLATION_LEVEL_REPEATABLE_READ = "SET default_transaction_isolation = 'repeatable read'"
	ERROR_MSG_PERMISSION_DENIED                 = "permission denied"
)

func getPGSessionInitScript(tconf *TargetConf) []string {
	var sessionVars []string
	if checkSessionVariableSupport(tconf, SET_CLIENT_ENCODING_TO_UTF8) {
		sessionVars = append(sessionVars, SET_CLIENT_ENCODING_TO_UTF8)
	}
	if checkSessionVariableSupport(tconf, SET_SESSION_REPLICATE_ROLE_TO_REPLICA) {
		sessionVars = append(sessionVars, SET_SESSION_REPLICATE_ROLE_TO_REPLICA)
	}
	return sessionVars
}

func getYBSessionInitScript(tconf *TargetConf) []string {
	var sessionVars []string
	if checkSessionVariableSupport(tconf, SET_CLIENT_ENCODING_TO_UTF8) {
		sessionVars = append(sessionVars, SET_CLIENT_ENCODING_TO_UTF8)
	}
	if checkSessionVariableSupport(tconf, SET_SESSION_REPLICATE_ROLE_TO_REPLICA) {
		sessionVars = append(sessionVars, SET_SESSION_REPLICATE_ROLE_TO_REPLICA)
	}
	if checkSessionVariableSupport(tconf, SET_DEFAULT_ISOLATION_LEVEL_REPEATABLE_READ) {
		sessionVars = append(sessionVars, SET_DEFAULT_ISOLATION_LEVEL_REPEATABLE_READ)
	}

	// enable `set yb_fast_path_for_colocated_copy` only if opted for IGNORE or UPDATE as PK conflict action
	if (tconf.OnPrimaryKeyConflictAction == constants.PRIMARY_KEY_CONFLICT_ACTION_IGNORE) &&
		checkSessionVariableSupport(tconf, SET_YB_FAST_PATH_FOR_COLOCATED_COPY) {
		sessionVars = append(sessionVars, SET_YB_FAST_PATH_FOR_COLOCATED_COPY)
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
		utils.ErrExit("error while creating connection for checking session parameter support: %q: %v", sqlStmt, err)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), sqlStmt)
	if err != nil {
		if !strings.Contains(err.Error(), "unrecognized configuration parameter") {
			if strings.Contains(err.Error(), ERROR_MSG_PERMISSION_DENIED) {
				utils.PrintAndLog("Superuser privileges are required on the target database user.\nAttempted operation: %q. Error message: %s", sqlStmt, err.Error())
				if !utils.AskPrompt("Are you sure you want to proceed?") {
					utils.ErrExit("Aborting import.")
				}
				return false // support is not there even if the target user doesn't have privileges to set this parameter.
			}
			utils.ErrExit("error while executing sqlStatement: %q: %v", sqlStmt, err)
		} else {
			log.Warnf("Warning: %q is not supported: %v", sqlStmt, err)
		}
	}

	return err == nil
}

func (yb *TargetYugabyteDB) setTargetSchema(conn *pgx.Conn) error {
	setSchemaQuery := fmt.Sprintf("SET SCHEMA '%s'", yb.tconf.Schema)
	_, err := conn.Exec(context.Background(), setSchemaQuery)
	if err != nil {
		return fmt.Errorf("run query: %q on target %q: %s", setSchemaQuery, conn.Config().Host, err)
	}

	// append oracle schema in the search_path for orafce
	// It is okay even if the schema does not exist in the target.
	updateSearchPath := `SELECT set_config('search_path', current_setting('search_path') || ', oracle', false)`
	_, err = conn.Exec(context.Background(), updateSearchPath)
	if err != nil {
		return fmt.Errorf("unable to update search_path for orafce extension with query: %q on target %q: %v", updateSearchPath, conn.Config().Host, err)
	}
	return nil
}

func (yb *TargetYugabyteDB) isBatchAlreadyImported(conn *pgx.Conn, batch Batch) (bool, int64, error) {
	var rowsImported int64
	query := batch.GetQueryIsBatchAlreadyImported()
	err := conn.QueryRow(context.Background(), query).Scan(&rowsImported)
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

func (yb *TargetYugabyteDB) recordEntryInDB(conn *pgx.Conn, batch Batch, rowsAffected int64) error {
	cmd := batch.GetQueryToRecordEntryInDB(rowsAffected)
	_, err := conn.Exec(context.Background(), cmd)
	if err != nil {
		return fmt.Errorf("insert into %s: %w", BATCH_METADATA_TABLE_NAME, err)
	}
	return nil
}

func (yb *TargetYugabyteDB) MaxBatchSizeInBytes() int64 {
	// if MAX_BATCH_SIZE is set in env then return that value
	return utils.GetEnvAsInt64("MAX_BATCH_SIZE_BYTES", 200*1024*1024) //default: 200 * 1024 * 1024 MB
}

func (yb *TargetYugabyteDB) GetIdentityColumnNamesForTable(tableNameTup sqlname.NameTuple, identityType string) ([]string, error) {
	sname, tname := tableNameTup.ForCatalogQuery()
	query := fmt.Sprintf(`SELECT column_name FROM information_schema.columns where table_schema='%s' AND
		table_name='%s' AND is_identity='YES' AND identity_generation='%s'`, sname, tname, identityType)
	log.Infof("query of identity(%s) columns for table(%s): %s", identityType, tableNameTup, query)
	var identityColumns []string
	err := yb.connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
		rows, err := conn.Query(context.Background(), query)
		if err != nil {
			log.Errorf("querying identity(%s) columns: %v", identityType, err)
			return false, fmt.Errorf("querying identity(%s) columns: %w", identityType, err)
		}
		defer rows.Close()
		for rows.Next() {
			var colName string
			err = rows.Scan(&colName)
			if err != nil {
				log.Errorf("scanning row for identity(%s) column name: %v", identityType, err)
				return false, fmt.Errorf("scanning row for identity(%s) column name: %w", identityType, err)
			}
			identityColumns = append(identityColumns, colName)
		}
		return false, nil
	})
	return identityColumns, err
}

func (yb *TargetYugabyteDB) DisableGeneratedAlwaysAsIdentityColumns(tableColumnsMap *utils.StructMap[sqlname.NameTuple, []string]) error {
	log.Infof("disabling generated always as identity columns")
	return yb.alterColumns(tableColumnsMap, "SET GENERATED BY DEFAULT")
}

func (yb *TargetYugabyteDB) EnableGeneratedAlwaysAsIdentityColumns(tableColumnsMap *utils.StructMap[sqlname.NameTuple, []string]) error {
	log.Infof("enabling generated always as identity columns")
	// YB automatically resumes the value for further inserts due to sequence attached
	return yb.alterColumns(tableColumnsMap, "SET GENERATED ALWAYS")
}

func (yb *TargetYugabyteDB) EnableGeneratedByDefaultAsIdentityColumns(tableColumnsMap *utils.StructMap[sqlname.NameTuple, []string]) error {
	log.Infof("enabling generated by default as identity columns")
	return yb.alterColumns(tableColumnsMap, "SET GENERATED BY DEFAULT")
}

func (yb *TargetYugabyteDB) alterColumns(tableColumnsMap *utils.StructMap[sqlname.NameTuple, []string], alterAction string) error {
	log.Infof("altering columns for action %s", alterAction)
	return tableColumnsMap.IterKV(func(table sqlname.NameTuple, columns []string) (bool, error) {
		for _, column := range columns {
			query := fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s %s`, table.ForUserQuery(), column, alterAction)
			sleepIntervalSec := 10
			for i := 0; i < ALTER_QUERY_RETRY_COUNT; i++ {
				err := yb.connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
					// Execute the query to alter the column
					_, err := conn.Exec(context.Background(), query)
					if err != nil {
						log.Errorf("executing query to alter columns for table(%s): %v", table.ForUserQuery(), err)
						return false, fmt.Errorf("executing query to alter columns for table(%s): %v", table.ForUserQuery(), err)
					}
					return false, nil
				})

				if err != nil {
					log.Errorf("error in altering columns for table(%s): %v", table.ForUserQuery(), err)
					if !strings.Contains(err.Error(), "while reaching out to the tablet servers") {
						return false, err
					}
					log.Infof("retrying after %d seconds for table(%s)", sleepIntervalSec, table.ForUserQuery())
					time.Sleep(time.Duration(sleepIntervalSec) * time.Second)
					continue
				}
				break
			}
		}

		return true, nil
	})
}

func (yb *TargetYugabyteDB) isSchemaExists(schema string) bool {
	query := fmt.Sprintf("SELECT true FROM information_schema.schemata WHERE schema_name = '%s'", schema)
	return yb.isQueryResultNonEmpty(query)
}

func (yb *TargetYugabyteDB) isTableExists(tableNameTup sqlname.NameTuple) bool {
	schema, table := tableNameTup.ForCatalogQuery()
	query := fmt.Sprintf("SELECT true FROM information_schema.tables WHERE table_schema = '%s' AND table_name = '%s'", schema, table)
	return yb.isQueryResultNonEmpty(query)
}

func (yb *TargetYugabyteDB) isQueryResultNonEmpty(query string) bool {
	rows, err := yb.Query(query)
	if err != nil {
		utils.ErrExit("error checking if query is empty: [%s]: %v", query, err)
	}
	defer rows.Close()

	return rows.Next()
}

func (yb *TargetYugabyteDB) IsDBColocated() (bool, error) {
	query := "select yb_is_database_colocated()"
	var isColocated bool
	err := yb.QueryRow(query).Scan(&isColocated)
	return isColocated, err
}

func (yb *TargetYugabyteDB) IsTableColocated(tableName sqlname.NameTuple) (bool, error) {
	query := fmt.Sprintf("select is_colocated from yb_table_properties('%s'::regclass)", tableName.ForUserQuery())
	var isTableColocated bool
	err := yb.QueryRow(query).Scan(&isTableColocated)
	return isTableColocated, err
}

func (yb *TargetYugabyteDB) IsAdaptiveParallelismSupported() bool {
	query := "SELECT * FROM pg_proc WHERE proname='yb_servers_metrics'"
	return yb.isQueryResultNonEmpty(query)
}

/*
Sample output of yb_servers_metrics:
yugabyte=# select uuid, jsonb_pretty(metrics), status, error from yb_servers_metrics();

	uuid               |                    jsonb_pretty                     | status | error

----------------------------------+-----------------------------------------------------+--------+-------

	bf98c74dd7044b34943c5bff7bd3d0d1 | {                                                  +| OK     |
	                                 |     "memory_free": "0",                            +|        |
	                                 |     "memory_total": "17179869184",                 +|        |
	                                 |     "cpu_usage_user": "0.135827",                  +|        |
	                                 |     "cpu_usage_system": "0.118110",                +|        |
	                                 |     "memory_available": "0",                       +|        |
	                                 |     "tserver_root_memory_limit": "11166914969",    +|        |
	                                 |     "tserver_root_memory_soft_limit": "9491877723",+|        |
	                                 |     "tserver_root_memory_consumption": "52346880"  +|        |
	                                 | }                                                   |        |
	d105c3a6128640f5a25cc74435e48ae3 | {                                                  +| OK     |
	                                 |     "memory_free": "0",                            +|        |
	                                 |     "memory_total": "17179869184",                 +|        |
	                                 |     "cpu_usage_user": "0.135189",                  +|        |
	                                 |     "cpu_usage_system": "0.119284",                +|        |
	                                 |     "memory_available": "0",                       +|        |
	                                 |     "tserver_root_memory_limit": "11166914969",    +|        |
	                                 |     "tserver_root_memory_soft_limit": "9491877723",+|        |
	                                 |     "tserver_root_memory_consumption": "55074816"  +|        |
	                                 | }                                                   |        |
	a321e13e5bf24060a764b35894cd4070 | {                                                  +| OK     |
	                                 |     "memory_free": "0",                            +|        |
	                                 |     "memory_total": "17179869184",                 +|        |
	                                 |     "cpu_usage_user": "0.135827",                  +|        |
	                                 |     "cpu_usage_system": "0.118110",                +|        |
	                                 |     "memory_available": "0",                       +|        |
	                                 |     "tserver_root_memory_limit": "11166914969",    +|        |
	                                 |     "tserver_root_memory_soft_limit": "9491877723",+|        |
	                                 |     "tserver_root_memory_consumption": "62062592"  +|        |
	                                 | }                                                   |        |
*/
func (yb *TargetYugabyteDB) GetClusterMetrics() (map[string]NodeMetrics, error) {
	result := make(map[string]NodeMetrics)

	query := "select uuid, metrics, status, error from yb_servers_metrics();"

	// since the query is run on a single common connection shared across all queries to be executed on target.
	// GetClusterMetrics() function itself in being used at multiple places: Monitoring, Adaptive Parallelism, and callhome
	yb.Lock()
	defer yb.Unlock()
	rows, err := yb.Query(query)
	if err != nil {
		return result, fmt.Errorf("querying yb_servers_metrics(): %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close the result set for query [%v]", query)
		}
	}()

	for rows.Next() {
		var uuid, metrics, status, errorStr sql.NullString
		if err := rows.Scan(&uuid, &metrics, &status, &errorStr); err != nil {
			return result, fmt.Errorf("scanning row for yb_servers_metrics(): %w", err)
		}
		if !uuid.Valid || !status.Valid || !errorStr.Valid || !metrics.Valid {
			return result, fmt.Errorf("got invalid NULL values from yb_servers_metrics() : %v, %v, %v, %v",
				uuid, metrics, status, errorStr)
		}
		nodeMetrics := NodeMetrics{
			UUID:    uuid.String,
			Metrics: make(map[string]string),
			Status:  status.String,
			Error:   errorStr.String,
		}
		if err := json.Unmarshal([]byte(metrics.String), &(nodeMetrics.Metrics)); err != nil {
			return result, fmt.Errorf("unmarshalling metrics json string: %w", err)
		}
		result[uuid.String] = nodeMetrics
	}
	return result, nil
}

func (yb *TargetYugabyteDB) GetNumConnectionsInPool() int {
	return yb.connPool.GetNumConnections()
}

func (yb *TargetYugabyteDB) GetNumMaxConnectionsInPool() int {
	return yb.connPool.params.NumMaxConnections
}

func (yb *TargetYugabyteDB) UpdateNumConnectionsInPool(delta int) error {
	return yb.connPool.UpdateNumConnections(delta)
}

func (yb *TargetYugabyteDB) RemoveConnectionsForHosts(servers []string) error {
	return yb.connPool.RemoveConnectionsForHosts(servers)
}

func (yb *TargetYugabyteDB) ClearMigrationState(migrationUUID uuid.UUID, exportDir string) error {
	log.Infof("clearing migration state for migrationUUID: %s", migrationUUID)
	schema := BATCH_METADATA_TABLE_SCHEMA
	if !yb.isSchemaExists(schema) {
		log.Infof("schema %s does not exist, nothing to clear migration state", schema)
		return nil
	}

	// clean up all the tables in BATCH_METADATA_TABLE_SCHEMA for given migrationUUID
	tableNames := []string{BATCH_METADATA_TABLE_NAME, EVENT_CHANNELS_METADATA_TABLE_NAME, EVENTS_PER_TABLE_METADATA_TABLE_NAME} // replace with actual table names
	tables := []sqlname.NameTuple{}
	for _, tableName := range tableNames {
		parts := strings.Split(tableName, ".")
		objName := sqlname.NewObjectName(constants.YUGABYTEDB, "", parts[0], parts[1])
		nt := sqlname.NameTuple{
			CurrentName: objName,
			SourceName:  objName,
			TargetName:  objName,
		}
		tables = append(tables, nt)
	}
	for _, table := range tables {
		if !yb.isTableExists(table) {
			log.Infof("table %s does not exist, nothing to clear migration state", table)
			continue
		}
		log.Infof("cleaning up table %s for migrationUUID=%s", table, migrationUUID)
		query := fmt.Sprintf("DELETE FROM %s WHERE migration_uuid = '%s'", table.ForUserQuery(), migrationUUID)
		_, err := yb.Exec(query)
		if err != nil {
			log.Errorf("error cleaning up table %s for migrationUUID=%s: %v", table, migrationUUID, err)
			return fmt.Errorf("error cleaning up table %s for migrationUUID=%s: %w", table, migrationUUID, err)
		}
	}

	nonEmptyTables := yb.GetNonEmptyTables(tables)
	if len(nonEmptyTables) != 0 {
		log.Infof("tables %v are not empty in schema %s", nonEmptyTables, schema)
		utils.PrintAndLog("removed the current migration state from the target DB. "+
			"But could not remove the schema '%s' as it still contains state of other migrations in '%s' database", schema, yb.tconf.DBName)
		return nil
	}
	utils.PrintAndLog("dropping schema %s", schema)
	query := fmt.Sprintf("DROP SCHEMA %s CASCADE", schema)
	_, err := yb.Exec(query)
	if err != nil {
		log.Errorf("error dropping schema %s: %v", schema, err)
		return fmt.Errorf("error dropping schema %s: %w", schema, err)
	}

	return nil
}

// ================================ NodeMetrics =================================

type NodeMetrics struct {
	UUID    string
	Metrics map[string]string
	Status  string
	Error   string
}

// CPUPercent returns (user + system) CPU usage as a percent (0–100).
func (n *NodeMetrics) GetCPUPercent() (float64, error) {
	userStr, ok1 := n.Metrics[CPU_USAGE_USER_METRIC]
	sysStr, ok2 := n.Metrics[CPU_USAGE_SYSTEM_METRIC]
	if !ok1 || !ok2 {
		return -1, fmt.Errorf("node %s: missing cpu_usage_user or cpu_usage_system", n.UUID)
	}

	user, err := strconv.ParseFloat(userStr, 64)
	if err != nil {
		return -1, fmt.Errorf("node %s: parse cpu_usage_user: %w", n.UUID, err)
	}
	sys, err := strconv.ParseFloat(sysStr, 64)
	if err != nil {
		return -1, fmt.Errorf("node %s: parse cpu_usage_system: %w", n.UUID, err)
	}

	return (user + sys) * 100, nil
}

// MemPercent returns memory consumption as a percent of the soft limit (0–100).
func (n *NodeMetrics) GetMemPercent() (float64, error) {
	usedStr, ok1 := n.Metrics[TSERVER_ROOT_MEMORY_CONSUMPTION_METRIC]
	softStr, ok2 := n.Metrics[TSERVER_ROOT_MEMORY_SOFT_LIMIT_METRIC]
	if !ok1 || !ok2 {
		return -1, fmt.Errorf("node %s: missing tserver_root_memory_consumption or tserver_root_memory_soft_limit", n.UUID)
	}

	used, err := strconv.ParseFloat(usedStr, 64)
	if err != nil {
		return -1, fmt.Errorf("node %s: parse memory_consumption: %w", n.UUID, err)
	}
	soft, err := strconv.ParseFloat(softStr, 64)
	if err != nil {
		return -1, fmt.Errorf("node %s: parse memory_soft_limit: %w", n.UUID, err)
	}
	if soft == 0 {
		return -1, fmt.Errorf("node %s: soft memory limit is zero", n.UUID)
	}

	return (used / soft) * 100, nil
}

func (n *NodeMetrics) GetMemoryFree() (int64, error) {
	memoryFreeStr, ok := n.Metrics[MEMORY_FREE_METRIC]
	if !ok {
		return -1, fmt.Errorf("node %s: missing memory_free", n.UUID)
	}

	memoryFree, err := strconv.ParseInt(memoryFreeStr, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("node %s: parse memory_free: %w", n.UUID, err)
	}
	return memoryFree, nil
}

func (n *NodeMetrics) GetMemoryAvailable() (int64, error) {
	memoryAvailableStr, ok := n.Metrics[MEMORY_AVAILABLE_METRIC]
	if !ok {
		return -1, fmt.Errorf("node %s: missing memory_available", n.UUID)
	}

	memoryAvailable, err := strconv.ParseInt(memoryAvailableStr, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("node %s: parse memory_available: %w", n.UUID, err)
	}
	return memoryAvailable, nil
}

func (n *NodeMetrics) GetMemoryTotal() (int64, error) {
	memoryTotalStr, ok := n.Metrics[MEMORY_TOTAL_METRIC]
	if !ok {
		return -1, fmt.Errorf("node %s: missing memory_total", n.UUID)
	}

	memoryTotal, err := strconv.ParseInt(memoryTotalStr, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("node %s: parse memory_total: %w", n.UUID, err)
	}
	return memoryTotal, nil
}

// =============================== Guardrails =================================

func (yb *TargetYugabyteDB) GetMissingImportDataPermissions(isFallForwardEnabled bool) ([]string, error) {
	// check if the user is a superuser
	isSuperUser, err := IsCurrentUserSuperUser(yb.tconf)
	if err != nil {
		return nil, fmt.Errorf("checking if user is superuser: %w", err)
	}
	if !isSuperUser {
		errorMsg := fmt.Sprintf("User %s is not a superuser.", yb.tconf.User)
		return []string{errorMsg}, nil
	}

	return nil, nil
}

func IsCurrentUserSuperUser(tconf *TargetConf) (bool, error) {
	conn, err := pgx.Connect(context.Background(), tconf.GetConnectionUri())
	if err != nil {
		return false, fmt.Errorf("unable to connect to target database: %w", err)
	}
	defer conn.Close(context.Background())

	runQueryAndCheckPrivilege := func(query string) (bool, error) {
		rows, err := conn.Query(context.Background(), query)
		if err != nil {
			return false, fmt.Errorf("querying if user is superuser: %w", err)
		}
		defer rows.Close()

		var isProperUser bool
		if rows.Next() {
			err = rows.Scan(&isProperUser)
			if err != nil {
				return false, fmt.Errorf("scanning row for query: %w", err)
			}
		} else {
			return false, fmt.Errorf("no current user found in pg_roles")
		}
		return isProperUser, nil
	}

	//This rolsuper is set to true in the pg_roles if a user is super user
	isSuperUserquery := "SELECT rolsuper FROM pg_roles WHERE rolname=current_user"

	isSuperUser, err := runQueryAndCheckPrivilege(isSuperUserquery)
	if err != nil {
		return false, fmt.Errorf("error checking super user privilege: %w", err)
	}
	if isSuperUser {
		return true, nil
	}
	//In case of YugabyteDB Aeon deployment of target database we need to verify if yb_superuser is granted or not
	isYbSuperUserQuery := `SELECT 
    CASE 
        WHEN EXISTS (
            SELECT 1
            FROM pg_auth_members m
            JOIN pg_roles grantee ON m.member = grantee.oid
            JOIN pg_roles granted ON m.roleid = granted.oid
            WHERE grantee.rolname = CURRENT_USER AND granted.rolname = 'yb_superuser'
        ) 
        THEN TRUE 
        ELSE FALSE 
    END AS is_yb_superuser;`

	isYBSuperUser, err := runQueryAndCheckPrivilege(isYbSuperUserQuery)
	if err != nil {
		return false, fmt.Errorf("error checking yb_superuser privilege: %w", err)
	}

	return isYBSuperUser, nil
}

func (yb *TargetYugabyteDB) GetEnabledTriggersAndFks() (enabledTriggers []string, enabledFks []string, err error) {
	return nil, nil, nil
}

func (yb *TargetYugabyteDB) NumOfLogicalReplicationSlots() (int64, error) {
	query := fmt.Sprintf("SELECT count(slot_name) from pg_replication_slots where database='%s'", yb.tconf.DBName)
	var numOfSlots int64

	err := yb.QueryRow(query).Scan(&numOfSlots)
	if err != nil {
		return 0, fmt.Errorf("error scanning the row returned while querying pg_replication_slots: %v", err)
	}

	return numOfSlots, nil
}
