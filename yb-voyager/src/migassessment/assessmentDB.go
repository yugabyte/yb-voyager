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
package migassessment

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/pgss"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/types"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	TABLE_INDEX_IOPS         = "table_index_iops"
	TABLE_INDEX_SIZES        = "table_index_sizes"
	TABLE_ROW_COUNTS         = "table_row_counts"
	TABLE_COLUMNS_COUNT      = "table_columns_count"
	INDEX_TO_TABLE_MAPPING   = "index_to_table_mapping"
	OBJECT_TYPE_MAPPING      = "object_type_mapping"
	TABLE_COLUMNS_DATA_TYPES = "table_columns_data_types"
	TABLE_INDEX_STATS        = "table_index_stats"
	DB_QUERIES_SUMMARY       = "db_queries_summary"
	REDUNDANT_INDEXES        = "redundant_indexes"
	COLUMN_STATISTICS        = "column_statistics"
	TABLE_INDEX_USAGE_STATS  = "table_index_usage_stats"

	PARTITIONED_TABLE_OBJECT_TYPE = "partitioned table"
	PARTITIONED_INDEX_OBJECT_TYPE = "partitioned index"
)

type TableIndexStats struct {
	SchemaName      string  `json:"SchemaName"`
	ObjectName      string  `json:"ObjectName"`
	RowCount        *int64  `json:"RowCount"` // Pointer to allows null values
	ColumnCount     *int64  `json:"ColumnCount"`
	ReadsPerSecond  *int64  `json:"ReadsPerSecond"`
	WritesPerSecond *int64  `json:"WritesPerSecond"`
	IsIndex         bool    `json:"IsIndex"`
	ObjectType      string  `json:"ObjectType"`
	ParentTableName *string `json:"ParentTableName"`
	SizeInBytes     *int64  `json:"SizeInBytes"`
}

var GetSourceMetadataDBFilePath = func() string {
	if AssessmentDir == "" {
		panic("AssessmentDir must be set before calling GetSourceMetadataDBFilePath()")
	}
	return filepath.Join(AssessmentDir, "dbs", "assessment.db")
}

func GetTableIndexStatName() string {
	return TABLE_INDEX_STATS
}

func GetTableRedundantIndexesName() string {
	return REDUNDANT_INDEXES
}

func InitAssessmentDB() error {
	assessmentDBPath := GetSourceMetadataDBFilePath()
	log.Infof("initializing assessment db at %s", assessmentDBPath)
	conn, err := sql.Open("sqlite3", fmt.Sprintf("%s%s", assessmentDBPath, metadb.SQLITE_OPTIONS))
	if err != nil {
		return fmt.Errorf("error opening assessment db %s: %w", assessmentDBPath, err)
	}

	cmds := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node		TEXT DEFAULT 'primary',
			schema_name		TEXT,
			object_name		TEXT,
			object_type		TEXT,
			seq_reads		INTEGER,
			row_writes		INTEGER,
			measurement_type TEXT,
			PRIMARY KEY (source_node, schema_name, object_name, measurement_type));`, TABLE_INDEX_IOPS),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node		TEXT DEFAULT 'primary',
			schema_name		TEXT,
			object_name		TEXT,
			object_type		TEXT,
			size_in_bytes	INTEGER,
			PRIMARY KEY (source_node, schema_name, object_name));`, TABLE_INDEX_SIZES),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node		TEXT DEFAULT 'primary',
			schema_name		TEXT,
			table_name		TEXT,
			row_count		INTEGER,
			PRIMARY KEY (source_node, schema_name, table_name));`, TABLE_ROW_COUNTS),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node		TEXT DEFAULT 'primary',
			schema_name		TEXT,
			object_name		TEXT,
			object_type		TEXT,
			column_count	INTEGER,
			PRIMARY KEY (source_node, schema_name, object_name));`, TABLE_COLUMNS_COUNT),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node		TEXT DEFAULT 'primary',
			index_schema	TEXT,
			index_name		TEXT,
			table_schema	TEXT,
			table_name		TEXT,
			PRIMARY KEY (source_node, index_schema, index_name));`, INDEX_TO_TABLE_MAPPING),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node		TEXT DEFAULT 'primary',
			schema_name     TEXT,
			object_name     TEXT,
			object_type     TEXT,
			PRIMARY KEY (source_node, schema_name, object_name));`, OBJECT_TYPE_MAPPING),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node		TEXT DEFAULT 'primary',
			schema_name		TEXT,
			table_name		TEXT,
			column_name		TEXT,
			data_type		TEXT,
			PRIMARY KEY (source_node, schema_name, table_name, column_name));`, TABLE_COLUMNS_DATA_TYPES),
		// derived from the above metric tables
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			schema_name         TEXT,
			object_name         TEXT,
			row_count           INTEGER,
			column_count		INTEGER,
			reads_per_second	INTEGER,
			writes_per_second	INTEGER,
			is_index            BOOLEAN,
			object_type			TEXT,
			parent_table_name   TEXT,
			size_in_bytes       INTEGER,
			PRIMARY KEY(schema_name, object_name));`, TABLE_INDEX_STATS),

		// to store pgss output for performance validation and unsupported query constructs detection
		// Schema exactly matches src/pgss.PgStatStatements struct for consistency
		// Future: might have to change/adapt this for Oracle/MySQL stats
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node			TEXT DEFAULT 'primary',
			queryid				BIGINT,
			query				TEXT,
			calls				BIGINT,
			rows				BIGINT,
			total_exec_time		REAL,
			mean_exec_time		REAL,
			min_exec_time		REAL,
			max_exec_time		REAL,
			stddev_exec_time	REAL);`, DB_QUERIES_SUMMARY),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node TEXT DEFAULT 'primary',
			redundant_schema_name TEXT,
			redundant_table_name TEXT,
			redundant_index_name TEXT,
			existing_schema_name TEXT,
			existing_table_name TEXT,
			existing_index_name TEXT,
			redundant_ddl TEXT,
			existing_ddl TEXT,
			PRIMARY KEY(source_node,redundant_schema_name,redundant_table_name,redundant_index_name));`, REDUNDANT_INDEXES),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node TEXT DEFAULT 'primary',
			schema_name TEXT,
			table_name TEXT,
			column_name TEXT,
			null_frac REAL,
			effective_n_distinct INTEGER,
			most_common_freq REAL,
			most_common_val TEXT,
			PRIMARY KEY(source_node, schema_name, table_name, column_name));`, COLUMN_STATISTICS),
		/*
			object info - schema, object name and type
			parent table name - only available for indexes else empty string
			scans, inserts, updates, deletes - usage stats
		*/
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			source_node TEXT DEFAULT 'primary',
			schema_name TEXT,
			object_name TEXT,
			object_type TEXT,
			parent_table_name TEXT,
			scans INTEGER,
			inserts INTEGER,
			updates INTEGER,
			deletes INTEGER, 
			PRIMARY KEY(source_node, schema_name, object_name));`, TABLE_INDEX_USAGE_STATS),
	}

	for _, cmd := range cmds {
		_, err = conn.Exec(cmd)
		if err != nil {
			return fmt.Errorf("error while initializing assessment db with query-%s: %w", cmd, err)
		}
	}

	// Note: Assessment DB is always created fresh (either new run or --start-clean)
	// Backward compatibility for loading old CSVs (without source_node column) is handled by:
	// 1. DEFAULT 'primary' in CREATE TABLE statements above
	// 2. BulkInsert() dynamic SQL which builds INSERT based on CSV headers

	err = conn.Close()
	if err != nil {
		return fmt.Errorf("error closing assessment db %s: %w", assessmentDBPath, err)
	}
	return nil
}

type AssessmentDB struct {
	db *sql.DB
}

func NewAssessmentDB() (*AssessmentDB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s%s", GetSourceMetadataDBFilePath(), metadb.SQLITE_OPTIONS))
	if err != nil {
		return nil, fmt.Errorf("error opening assessment db %s: %w", GetSourceMetadataDBFilePath(), err)
	}

	return &AssessmentDB{db: db}, nil
}

func (adb *AssessmentDB) BulkInsert(table string, records [][]string) error {
	if len(records) == 0 {
		return nil
	}
	ctx := context.Background()
	tx, err := adb.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("error starting transaction for bulk insert into %s: %w", table, err)
	}

	defer func() {
		err = tx.Rollback()
		if err != nil && errors.Is(err, sql.ErrTxDone) {
			log.Warnf("error while rollback the BulkInsert txn: %v", err)
		}
	}()

	columnNames := records[0]
	stmtStr := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, table,
		strings.Join(columnNames, ", "), strings.Repeat("?, ", len(columnNames)-1)+"?")

	stmt, err := tx.PrepareContext(ctx, stmtStr)
	if err != nil {
		return fmt.Errorf("error preparing statement for bulk insert into %s: %w", table, err)
	}

	for rowNum := 1; rowNum < len(records); rowNum++ {
		row := utils.ConvertStringSliceToInterface(records[rowNum])
		_, err = stmt.ExecContext(ctx, row...)
		if err != nil {
			return fmt.Errorf("error inserting record for bulk insert into %s: %w", table, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing transaction for bulk insert into %s: %w", table, err)
	}

	return nil
}

const (
	InsertTableStats = `INSERT INTO %s (schema_name, object_name, row_count, column_count, reads_per_second, writes_per_second, is_index, object_type, parent_table_name, size_in_bytes)
	SELECT 
		trc.schema_name,
		trc.table_name AS object_name,
		trc.row_count,
		tcc.column_count,
		NULL AS reads_per_second,
		NULL as writes_per_second,
		0 AS is_index,
		otm.object_type,
		NULL AS parent_table_name,
		tis.size_in_bytes
	FROM %s trc
	LEFT JOIN %s tii ON trc.schema_name = tii.schema_name AND trc.table_name = tii.object_name and tii.measurement_type='initial' AND tii.source_node = 'primary'
	LEFT JOIN %s tis ON trc.schema_name = tis.schema_name AND trc.table_name = tis.object_name AND tis.source_node = 'primary'
	LEFT JOIN %s tcc ON trc.schema_name = tcc.schema_name AND trc.table_name = tcc.object_name AND tcc.source_node = 'primary'
	LEFT JOIN %s otm ON trc.schema_name = otm.schema_name AND trc.table_name = otm.object_name AND otm.source_node = 'primary'
	WHERE trc.source_node = 'primary' AND otm.object_type NOT IN ('%s', '%s');`

	// No insertion into 'column_count' for indexes
	InsertIndexStats = `INSERT INTO %s (schema_name, object_name, row_count, column_count, reads_per_second, writes_per_second, is_index, object_type, parent_table_name, size_in_bytes)
	SELECT 
		itm.index_schema AS schema_name,
		itm.index_name AS object_name,
		NULL AS row_count,
		tcc.column_count,
		NULL AS reads_per_second,
		NULL as writes_per_second,
		1 AS is_index,
		otm.object_type,
		itm.table_schema || '.' || itm.table_name AS parent_table_name,
		tis.size_in_bytes
	FROM %s itm
	LEFT JOIN %s tii ON itm.index_schema = tii.schema_name AND itm.index_name = tii.object_name and tii.measurement_type='initial' AND tii.source_node = 'primary'
	LEFT JOIN %s tis ON itm.index_schema = tis.schema_name AND itm.index_name = tis.object_name AND tis.source_node = 'primary'
	LEFT JOIN %s tcc ON itm.index_schema = tcc.schema_name AND itm.index_name = tcc.object_name AND tcc.source_node = 'primary'
	LEFT JOIN %s otm ON itm.index_schema = otm.schema_name AND itm.index_name = otm.object_name AND otm.source_node = 'primary'
	WHERE itm.source_node = 'primary' AND otm.object_type NOT IN ('%s', '%s');`

	CreateTempTable = `CREATE TEMP TABLE read_write_rates AS
	SELECT
		initial.schema_name,
		initial.object_name,
		initial.object_type,
		-- Sum reads across ALL nodes (primary + replicas)
		(SUM(final.seq_reads) - SUM(initial.seq_reads)) / %d AS seq_reads_per_second, -- iops capture interval(default: 120)
		-- Take writes from PRIMARY only
		(SUM(CASE WHEN final.source_node = 'primary' THEN final.row_writes ELSE 0 END) - 
		 SUM(CASE WHEN initial.source_node = 'primary' THEN initial.row_writes ELSE 0 END)) / %d AS row_writes_per_second
	FROM
		%s AS initial
	JOIN
		%s AS final ON initial.schema_name = final.schema_name
									AND initial.object_name = final.object_name
									AND initial.source_node = final.source_node -- Match snapshots from same node (primary↔primary, replica1↔replica1, etc.)
									AND final.measurement_type = 'final'
	WHERE
		initial.measurement_type = 'initial'
	GROUP BY initial.schema_name, initial.object_name, initial.object_type; -- Collapse multi-node data into single row per object`

	UpdateStatsWithRates = `UPDATE table_index_stats
	SET
		reads_per_second = (SELECT seq_reads_per_second
							FROM read_write_rates
							WHERE read_write_rates.schema_name = table_index_stats.schema_name
								AND read_write_rates.object_name = table_index_stats.object_name),
		writes_per_second = (SELECT row_writes_per_second
								FROM read_write_rates
									WHERE read_write_rates.schema_name = table_index_stats.schema_name
									AND read_write_rates.object_name = table_index_stats.object_name)
	WHERE EXISTS (
		SELECT 1
		FROM read_write_rates
		WHERE read_write_rates.schema_name = table_index_stats.schema_name
			AND read_write_rates.object_name = table_index_stats.object_name
	);`
)

// populate table_index_stats table using the data from other tables
func (adb *AssessmentDB) PopulateMigrationAssessmentStats() error {
	statements := []string{
		fmt.Sprintf(InsertTableStats, TABLE_INDEX_STATS, TABLE_ROW_COUNTS, TABLE_INDEX_IOPS, TABLE_INDEX_SIZES,
			TABLE_COLUMNS_COUNT, OBJECT_TYPE_MAPPING, PARTITIONED_TABLE_OBJECT_TYPE, PARTITIONED_INDEX_OBJECT_TYPE),
		fmt.Sprintf(InsertIndexStats, TABLE_INDEX_STATS, INDEX_TO_TABLE_MAPPING, TABLE_INDEX_IOPS, TABLE_INDEX_SIZES,
			TABLE_COLUMNS_COUNT, OBJECT_TYPE_MAPPING, PARTITIONED_TABLE_OBJECT_TYPE, PARTITIONED_INDEX_OBJECT_TYPE),
	}

	switch SourceDBType {
	case constants.POSTGRESQL:
		var createTempTableForIops string
		if IntervalForCapturingIops == 0 { // considering value as 0 to avoid division by zero
			createTempTableForIops = fmt.Sprintf(CreateTempTable, 1, 1, TABLE_INDEX_IOPS, TABLE_INDEX_IOPS)
		} else {
			createTempTableForIops = fmt.Sprintf(CreateTempTable, IntervalForCapturingIops, IntervalForCapturingIops, TABLE_INDEX_IOPS, TABLE_INDEX_IOPS)
		}

		statements = append(statements,
			createTempTableForIops,
			UpdateStatsWithRates)
	case constants.ORACLE:
		// already accounted
	default:
		panic("invalid source db type")
	}

	for _, stmt := range statements {
		log.Infof("executing query for populating migration assessment stats- %s", stmt)
		if _, err := adb.db.Exec(stmt); err != nil {
			return fmt.Errorf("error executing statement-%s: %w", stmt, err)
		}
	}

	return nil
}

func (adb *AssessmentDB) FetchAllStats() (*[]TableIndexStats, error) {
	log.Infof("fetching all stats info from %q table", TABLE_INDEX_STATS)
	query := fmt.Sprintf(`SELECT schema_name, object_name, row_count, column_count, reads_per_second, writes_per_second, 
	is_index, object_type, parent_table_name, size_in_bytes FROM %s ORDER BY schema_name, object_name;`, TABLE_INDEX_STATS)
	rows, err := adb.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying all stats-%s: %w", query, err)
	}
	defer rows.Close()

	var stats []TableIndexStats
	for rows.Next() {
		var stat TableIndexStats
		var rowCount, columnCount, readsPerSecond, writesPerSecond, sizeInBytes sql.NullInt64
		var parentTableName sql.NullString
		if err := rows.Scan(&stat.SchemaName, &stat.ObjectName, &rowCount, &columnCount, &readsPerSecond, &writesPerSecond,
			&stat.IsIndex, &stat.ObjectType, &parentTableName, &sizeInBytes); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		stat.RowCount = lo.Ternary(rowCount.Valid, &rowCount.Int64, nil)
		stat.ColumnCount = lo.Ternary(columnCount.Valid, &columnCount.Int64, nil)
		stat.ReadsPerSecond = lo.Ternary(readsPerSecond.Valid, &readsPerSecond.Int64, nil)
		stat.WritesPerSecond = lo.Ternary(writesPerSecond.Valid, &writesPerSecond.Int64, nil)
		stat.ParentTableName = lo.Ternary(parentTableName.Valid, &parentTableName.String, nil)
		stat.SizeInBytes = lo.Ternary(sizeInBytes.Valid, &sizeInBytes.Int64, nil)
		stats = append(stats, stat)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading rows: %w", err)
	}

	return &stats, nil
}

func (adb *AssessmentDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := adb.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error executing query-%s: %w", query, err)
	}
	return rows, nil
}

// LoadCSVFileIntoTable reads a CSV file and loads it into the specified table
func (adb *AssessmentDB) LoadCSVFileIntoTable(filePath, tableName string) error {
	if tableName == DB_QUERIES_SUMMARY {
		// special handling due to pgss postgres version differences
		return adb.LoadPgssCSVIntoTable(filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", filePath, err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	csvReader.ReuseRecord = true

	// Load all CSV rows in-memory; safe due to expected small file size (limited by DB object count or pg_stat_statements.max config)
	rows, err := csvReader.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading csv file %s: %w", filePath, err)
	}

	err = adb.BulkInsert(tableName, rows)
	if err != nil {
		return fmt.Errorf("error bulk inserting data into %s table: %w", tableName, err)
	}
	return nil
}

// LoadPgssCSVIntoTable loads PGSS CSV data into the db_queries_summary table
// special handling because csv can have different columns based on the PG version and we need to load it into the table with specific schema(9 columns)
func (adb *AssessmentDB) LoadPgssCSVIntoTable(filePath string) error {
	log.Infof("starting to parse PGSS CSV file")
	entries, err := pgss.ParseFromCSV(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse PGSS CSV: %w", err)
	}

	if len(entries) == 0 {
		log.Warnf("No valid PGSS entries found in %s", filePath)
		return nil
	}

	log.Infof("inserting PGSS entries into %s table", DB_QUERIES_SUMMARY)
	err = adb.InsertPgssEntries(entries)
	if err != nil {
		return fmt.Errorf("failed to insert PGSS entries: %w", err)
	}

	return nil
}

func (adb *AssessmentDB) InsertPgssEntries(entries []*pgss.PgStatStatements) error {
	if len(entries) == 0 {
		return nil
	}

	// Prepared statement for faster insertion
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (
			queryid,
			query,
			calls,
			rows,
			total_exec_time,
			mean_exec_time,
			min_exec_time,
			max_exec_time,
			stddev_exec_time
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?
		)`, DB_QUERIES_SUMMARY)

	stmt, err := adb.db.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare PGSS insert statement: %w", err)
	}
	defer stmt.Close()

	for _, entry := range entries {
		_, err = stmt.Exec(entry.QueryID, entry.Query, entry.Calls, entry.Rows, entry.TotalExecTime, entry.MeanExecTime,
			entry.MinExecTime, entry.MaxExecTime, entry.StddevExecTime)
		if err != nil {
			return fmt.Errorf("failed to insert PGSS entry for queryid %d: %w", entry.QueryID, err)
		}
	}

	return nil
}

// GetSourceQueryStats retrieves all the source PGSS data from the assessment database
// For multi-node setups, only returns query stats from the primary node to avoid
// mixing workload patterns from read replicas with primary writes.
// Backward compatible: works with older assessment DBs that don't have the source_node column.
func (adb *AssessmentDB) GetSourceQueryStats() ([]*types.QueryStats, error) {
	// Check if source_node column exists (for backward compatibility with older assessment DBs)
	hasSourceNode, err := adb.columnExists(DB_QUERIES_SUMMARY, "source_node")
	if err != nil {
		return nil, fmt.Errorf("failed to check schema version: %w", err)
	}

	// Build query based on schema version
	query := `
		SELECT queryid, query, calls, rows,
		       total_exec_time, mean_exec_time,
		       min_exec_time, max_exec_time
		FROM db_queries_summary`

	if hasSourceNode {
		// New schema (with replicas support): filter to primary only
		query += `
		WHERE source_node = 'primary'`
	}
	// Old schema: no filter needed (only primary data exists)

	query += ";"

	rows, err := adb.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query PGSS data: %w", err)
	}
	defer rows.Close()

	var entries []*types.QueryStats
	for rows.Next() {
		var entry types.QueryStats
		err := rows.Scan(
			&entry.QueryID,
			&entry.QueryText,
			&entry.ExecutionCount,
			&entry.RowsProcessed,
			&entry.TotalExecTime,
			&entry.AverageExecTime,
			&entry.MinExecTime,
			&entry.MaxExecTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan PGSS row: %w", err)
		}
		entries = append(entries, &entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading PGSS rows: %w", err)
	}

	return entries, nil
}

func (adb *AssessmentDB) CheckIfTableExists(tableName string) error {
	query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?;`

	var name string
	err := adb.db.QueryRow(query, tableName).Scan(&name)
	if err != nil {
		return fmt.Errorf("error checking if table %s exists: %w", tableName, err)
	}

	return nil
}

// columnExists checks if a column exists in a SQLite table (for backward compatibility checks)
func (adb *AssessmentDB) columnExists(tableName, columnName string) (bool, error) {
	query := `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`
	var count int
	err := adb.db.QueryRow(query, tableName, columnName).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check if column %s exists in table %s: %w", columnName, tableName, err)
	}
	return count > 0, nil
}

// HasSourceQueryStats checks if query stats data exists in the assessment database (source-db type agnostic)
func (adb *AssessmentDB) HasSourceQueryStats() (bool, error) {
	log.Infof("checking if query stats data exists in the assessment database")
	query := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", DB_QUERIES_SUMMARY)
	var exists int
	err := adb.db.QueryRow(query).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check for PGSS data: %w", err)
	}
	return true, nil
}
