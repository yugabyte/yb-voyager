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
	"fmt"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	TABLE_INDEX_IOPS           = "table_index_iops"
	TABLE_INDEX_SIZES          = "table_index_sizes"
	TABLE_ROW_COUNTS           = "table_row_counts"
	COLUMNS_COUNT              = "columns_count"
	INDEX_TO_TABLE_MAPPING     = "index_to_table_mapping"
	TABLE_COLUMNS_DATA_TYPES   = "table_columns_data_types"
	MIGRATION_ASSESSMENT_STATS = "migration_assessment_stats"
)

func GetDBFilePath() string {
	return filepath.Join(AssessmentDataDir, "assessment.db")
}

func InitAssessmentDB() error {
	assessmentDBPath := GetDBFilePath()
	log.Infof("initializing assessment db at %s", assessmentDBPath)
	conn, err := sql.Open("sqlite3", fmt.Sprintf("%s%s", assessmentDBPath, metadb.SQLITE_OPTIONS))
	if err != nil {
		return fmt.Errorf("error opening assessment db %s: %w", assessmentDBPath, err)
	}

	cmds := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			schema_name TEXT,
			object_name TEXT,
			object_type TEXT,
			seq_reads INTEGER,
			row_writes INTEGER,
			PRIMARY KEY (schema_name, object_name));`, TABLE_INDEX_IOPS),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			schema_name TEXT,
			object_name TEXT,
			object_type TEXT,
			size REAL,
			PRIMARY KEY (schema_name, object_name));`, TABLE_INDEX_SIZES),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			schema_name TEXT,
			table_name TEXT,
			row_count INTEGER,
			PRIMARY KEY (schema_name, table_name));`, TABLE_ROW_COUNTS),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			schema_name TEXT,
			object_name TEXT,
			object_type TEXT,
			column_count INTEGER,
			PRIMARY KEY (schema_name, object_name));`, COLUMNS_COUNT),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			index_schema TEXT,
			index_name TEXT,
			table_schema TEXT,
			table_name TEXT,
			PRIMARY KEY (index_schema, index_name));`, INDEX_TO_TABLE_MAPPING),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			schema_name TEXT,
			table_name TEXT,
			column_name TEXT,
			data_type TEXT,
			PRIMARY KEY (schema_name, table_name, column_name));`, TABLE_COLUMNS_DATA_TYPES),
		// derived from the above metric tables
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			schema_name         TEXT,
			object_name         TEXT,
			row_count           INTEGER,
			reads               INTEGER,
			writes              INTEGER,
			isIndex             BOOLEAN,
			parent_table_name   TEXT,
			size                INTEGER,
			PRIMARY KEY(schema_name, object_name));`, MIGRATION_ASSESSMENT_STATS),
	}

	for _, cmd := range cmds {
		_, err = conn.Exec(cmd)
		if err != nil {
			return fmt.Errorf("error while initializing assessment db with query-%s: %w", cmd, err)
		}
	}

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
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s%s", GetDBFilePath(), metadb.SQLITE_OPTIONS))
	if err != nil {
		return nil, fmt.Errorf("error opening assessment db %s: %w", GetDBFilePath(), err)
	}

	return &AssessmentDB{db: db}, nil
}

func (adb *AssessmentDB) BulkInsert(table string, records [][]string) error {
	ctx := context.Background()
	tx, err := adb.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("error starting transaction for bulk insert into %s: %w", table, err)
	}

	defer func() {
		err = tx.Rollback()
		if err != nil {
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

// populate migration_assessment_stats table using the data from other tables
func (adb *AssessmentDB) PopulateMigrationAssessmentStats() error {
	populateTableStats := fmt.Sprintf(`INSERT INTO %s (schema_name, object_name, row_count, reads, writes, isIndex, parent_table_name, size)
	SELECT
		trc.schema_name,
		trc.table_name AS object_name,
		trc.row_count,
		tii.seq_reads as reads,
		tii.row_writes as writes,
		0 AS isIndex,
		NULL AS parent_table_name, 
		tis.size
	FROM %s trc
	LEFT JOIN %s tii ON trc.schema_name = tii.schema_name AND trc.table_name = tii.object_name
	LEFT JOIN %s tis ON trc.schema_name = tis.schema_name AND trc.table_name = tis.object_name;`,
		TABLE_ROW_COUNTS, TABLE_INDEX_IOPS, TABLE_INDEX_SIZES, MIGRATION_ASSESSMENT_STATS)

	populateIndexStats := fmt.Sprintf(`INSERT INTO %s (schema_name, object_name, row_count, reads, writes, isIndex, parent_table_name, size)
	SELECT
		itm.index_schema AS schema_name,
		itm.index_name AS object_name,
		NULL AS row_count,
		tii.seq_reads as reads,
		tii.row_writes as writes,
		1 AS isIndex,
		itm.table_schema || '.' || itm.table_name AS parent_table_name,
		tis.size
	FROM %s itm
	LEFT JOIN %s tii ON itm.index_schema = tii.schema_name AND itm.index_name = tii.object_name
	LEFT JOIN %s tis ON itm.index_schema = tis.schema_name AND itm.index_name = tis.object_name;`,
		INDEX_TO_TABLE_MAPPING, TABLE_INDEX_IOPS, TABLE_INDEX_SIZES, MIGRATION_ASSESSMENT_STATS)

	stmts := []string{populateTableStats, populateIndexStats}
	for _, stmt := range stmts {
		_, err := adb.db.Exec(stmt)
		if err != nil {
			return fmt.Errorf("error executing query-%s on table %s: %w", stmt, MIGRATION_ASSESSMENT_STATS, err)
		}
	}
	return nil
}
