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
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type TargetDB interface {
	Init() error
	Finalize()
	InitConnPool() error
	PrepareForStreaming()
	GetVersion() string
	CreateVoyagerSchema() error
	GetNonEmptyTables(tableNames []sqlname.NameTuple) []sqlname.NameTuple
	TruncateTables(tableNames []sqlname.NameTuple) error
	IsNonRetryableCopyError(err error) bool
	ImportBatch(batch Batch, args *ImportBatchArgs, exportDir string, tableSchema map[string]map[string]string, isRecoveryCandidate bool) (int64, error)
	QuoteAttributeNames(tableNameTup sqlname.NameTuple, columns []string) ([]string, error)
	GetPrimaryKeyColumns(table sqlname.NameTuple) ([]string, error)
	ExecuteBatch(migrationUUID uuid.UUID, batch *EventBatch) error
	GetListOfTableAttributes(tableNameTup sqlname.NameTuple) ([]string, error)
	QuoteAttributeName(tableNameTup sqlname.NameTuple, columnName string) (string, error)
	MaxBatchSizeInBytes() int64
	RestoreSequences(sequencesLastValue map[string]int64) error
	GetIdentityColumnNamesForTable(tableNameTup sqlname.NameTuple, identityType string) ([]string, error)
	DisableGeneratedAlwaysAsIdentityColumns(tableColumnsMap *utils.StructMap[sqlname.NameTuple, []string]) error
	EnableGeneratedAlwaysAsIdentityColumns(tableColumnsMap *utils.StructMap[sqlname.NameTuple, []string]) error
	EnableGeneratedByDefaultAsIdentityColumns(tableColumnsMap *utils.StructMap[sqlname.NameTuple, []string]) error
	ClearMigrationState(migrationUUID uuid.UUID, exportDir string) error
	GetCallhomeTargetDBInfo() *callhome.TargetDBDetails
	// NOTE: The following four methods should not be used for arbitrary query
	// execution on TargetDB. The should be only used from higher level
	// abstractions like ImportDataState.
	Query(query string) (*sql.Rows, error)
	QueryRow(query string) *sql.Row
	Exec(query string) (int64, error)
	WithTx(fn func(tx *sql.Tx) error) error
	GetMissingImportDataPermissions(isFallForwardEnabled bool) ([]string, error)
	GetEnabledTriggersAndFks() (enabledTriggers []string, enabledFks []string, err error)
}

//=============================================================

const (
	ORACLE     = "oracle"
	MYSQL      = "mysql"
	POSTGRESQL = "postgresql"
	YUGABYTEDB = "yugabytedb"
)

type Batch interface {
	Open() (*os.File, error)
	OpenDataFile() (datafile.DataFile, error)
	GetFilePath() string
	GetTableName() sqlname.NameTuple
	GetQueryIsBatchAlreadyImported() string
	GetQueryToRecordEntryInDB(rowsAffected int64) string
}

func NewTargetDB(tconf *TargetConf) TargetDB {
	switch tconf.TargetDBType {
	case ORACLE:
		return newTargetOracleDB(tconf)
	case POSTGRESQL:
		return newTargetPostgreSQL(tconf)
	case YUGABYTEDB:
		return newTargetYugabyteDB(tconf)
	}
	return nil
}

// ======================= ImportBatchArgs ====================

type ImportBatchArgs struct {
	FilePath          string
	TableNameTup      sqlname.NameTuple
	Columns           []string
	PrimaryKeyColumns []string // TODO: Implement
	PKConflictAction  string

	FileFormat string
	HasHeader  bool
	Delimiter  string
	QuoteChar  byte
	EscapeChar byte
	NullString string

	RowsPerTransaction int64
}

// Fast Path can be used to import batch is when: Primary Key is present and user has selected IGNORE as PK conflict action
func (args *ImportBatchArgs) IsFastPath() bool {
	return len(args.PrimaryKeyColumns) > 0 &&
		(args.PKConflictAction == constants.PRIMARY_KEY_CONFLICT_ACTION_IGNORE ||
			args.PKConflictAction == constants.PRIMARY_KEY_CONFLICT_ACTION_UPDATE)
}

func (args *ImportBatchArgs) GetYBTxnCopyStatement() string {
	options := args.copyOptions()
	options = append(options, fmt.Sprintf("ROWS_PER_TRANSACTION %v", args.RowsPerTransaction))
	columns := ""
	if len(args.Columns) > 0 {
		columns = fmt.Sprintf("(%s)", strings.Join(args.Columns, ", "))
	}

	return fmt.Sprintf(`COPY %s %s FROM STDIN WITH (%s)`, args.TableNameTup.ForUserQuery(), columns, strings.Join(options, ", "))
}

// returns YB COPY statement for fast path
// To trigger COPY fast path, no transaction and ROWS_PER_TRANSACTION should be used
func (args *ImportBatchArgs) GetYBNonTxnCopyStatement() string {
	options := args.copyOptions()
	columns := ""
	if len(args.Columns) > 0 {
		columns = fmt.Sprintf("(%s)", strings.Join(args.Columns, ", "))
	}

	return fmt.Sprintf(`COPY %s %s FROM STDIN WITH (%s)`, args.TableNameTup.ForUserQuery(), columns, strings.Join(options, ", "))
}

func (args *ImportBatchArgs) GetPGCopyStatement() string {
	options := args.copyOptions()
	columns := ""
	if len(args.Columns) > 0 {
		columns = fmt.Sprintf("(%s)", strings.Join(args.Columns, ", "))
	}
	return fmt.Sprintf(`COPY %s %s FROM STDIN WITH (%s)`, args.TableNameTup.ForUserQuery(), columns, strings.Join(options, ", "))
}

/*
TODOs:
	1. Handle quoting of identifiers(column/table names) in the insert statement
	2. Optimise the prepare statment to be cached and reused in connPool (same as for streaming events)
*/

/*
GetInsertPreparedStmtForBatchImport returns the INSERT prepared statement based on the requested conflict action.
1. ERROR   -> raise error (no ON CONFLICT clause)
2. IGNORE  -> ON CONFLICT DO NOTHING
3. UPDATE  -> ON CONFLICT (pk…) DO UPDATE SET non‑pk = EXCLUDED.non‑pk, …
*/
func (args *ImportBatchArgs) GetInsertPreparedStmtForBatchImport() string {
	// TODO: Need to ensure the order of columns in the insert statement is same as the rows in batch file

	// column list for insert statement (col1, col2, col3, ...)
	columns := strings.Join(args.Columns, ", ")

	// value placeholders for insert statement ($1, $2, $3, ...)
	valuePlaceHolders := make([]string, len(args.Columns))
	for i := range args.Columns {
		valuePlaceHolders[i] = fmt.Sprintf("$%d", i+1)
	}
	values := strings.Join(valuePlaceHolders, ", ")

	// base insert statement
	baseStmt := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`,
		args.TableNameTup.ForUserQuery(), columns, values)

	switch args.PKConflictAction {
	case constants.PRIMARY_KEY_CONFLICT_ACTION_ERROR:
		return baseStmt // no additional clause - although this is not going to be called ever.

	case constants.PRIMARY_KEY_CONFLICT_ACTION_IGNORE:
		// we are dealing with PK conflict only for the target table (not Unique index etc...)
		conflictTarget := strings.Join(args.PrimaryKeyColumns, ", ")

		return fmt.Sprintf("%s ON CONFLICT(%s) DO NOTHING", baseStmt, conflictTarget)

	/*
		Sample Statement:
			INSERT INTO users (id, name, email) VALUES ($1, $2, $3)
				ON CONFLICT (id) DO UPDATE
				SET
					name = EXCLUDED.name,
					email = EXCLUDED.email
	*/
	case constants.PRIMARY_KEY_CONFLICT_ACTION_UPDATE:
		// build conflict target (pk1, pk2, pk3, ...)
		conflictTarget := strings.Join(args.PrimaryKeyColumns, ", ")

		// build update set clause (col1 = EXCLUDED.col1, col2 = EXCLUDED.col2, ...)
		nonPKColumns := utils.SetDifference(args.Columns, args.PrimaryKeyColumns)
		if len(nonPKColumns) == 0 {
			return fmt.Sprintf("%s ON CONFLICT(%s) DO NOTHING", baseStmt, conflictTarget)
		}

		updateSet := make([]string, len(nonPKColumns))
		for i, col := range nonPKColumns {
			updateSet[i] = fmt.Sprintf("%s = EXCLUDED.%s", col, col)
		}
		updateSetClause := strings.Join(updateSet, ", ")

		// build the final insert statement
		baseStmt = fmt.Sprintf("%s ON CONFLICT (%s) DO UPDATE SET %s", baseStmt, conflictTarget, updateSetClause)
	default:
		panic(fmt.Sprintf("Invalid conflict action: %s", args.PKConflictAction))
	}

	return baseStmt
}

func (args *ImportBatchArgs) copyOptions() []string {
	options := []string{
		fmt.Sprintf("FORMAT '%s'", args.FileFormat),
	}
	if args.HasHeader {
		options = append(options, "HEADER")
	}
	if args.Delimiter != "" {
		options = append(options, fmt.Sprintf("DELIMITER E'%c'", []rune(args.Delimiter)[0]))
	}
	if args.QuoteChar != 0 {
		quoteChar := string(args.QuoteChar)
		if quoteChar == `'` || quoteChar == `\` {
			quoteChar = `\` + quoteChar
		}
		options = append(options, fmt.Sprintf("QUOTE E'%s'", quoteChar))
	}
	if args.EscapeChar != 0 {
		escapeChar := string(args.EscapeChar)
		if escapeChar == `'` || escapeChar == `\` {
			escapeChar = `\` + escapeChar
		}
		options = append(options, fmt.Sprintf("ESCAPE E'%s'", escapeChar))
	}
	if args.NullString != "" {
		options = append(options, fmt.Sprintf("NULL '%s'", args.NullString))
	}
	return options
}

func (args *ImportBatchArgs) GetSqlLdrControlFile(tableSchema map[string]map[string]string) string {
	var columns string
	if len(args.Columns) > 0 {
		var columnsList []string
		for _, column := range args.Columns {
			//setting the null string for each column
			dataType, ok := tableSchema[column]["__debezium.source.column.type"] //TODO: rename this to some thing like source-db-datatype
			charLength, okLen := tableSchema[column]["__debezium.source.column.length"]
			switch true {
			case ok && strings.Contains(dataType, "INTERVAL"):
				columnsList = append(columnsList, fmt.Sprintf(`%s %s NULLIF %s='%s'`, column, dataType, column, args.NullString))
			case ok && strings.HasPrefix(dataType, "DATE"):
				columnsList = append(columnsList, fmt.Sprintf(`%s DATE "DD-MM-YYYY" NULLIF %s='%s'`, column, column, args.NullString))
			case ok && strings.HasPrefix(dataType, "TIMESTAMP"):
				switch true {
				case strings.Contains(dataType, "TIME ZONE"):
					columnsList = append(columnsList, fmt.Sprintf(`%s TIMESTAMP WITH TIME ZONE "YYYY-MM-DD HH:MI:SS.FF9 AM TZR" NULLIF %s='%s'`, column, column, args.NullString))
				default:
					columnsList = append(columnsList, fmt.Sprintf(`%s TIMESTAMP "DD-MM-YYYY HH:MI:SS.FF9 AM" NULLIF %s='%s'`, column, column, args.NullString))
				}
			case ok && okLen && strings.Contains(dataType, "CHAR"):
				columnsList = append(columnsList, fmt.Sprintf(`%s CHAR(%s) NULLIF %s='%s'`, column, charLength, column, args.NullString))
			case ok && dataType == "LONG":
				columnsList = append(columnsList, fmt.Sprintf(`%s CHAR(2000000000) NULLIF %s='%s'`, column, column, args.NullString)) // for now mentioning max 2GB length, TODO: figure out if there is any other way to handle LONG data type
			default:
				columnsList = append(columnsList, fmt.Sprintf("%s NULLIF %s='%s'", column, column, args.NullString))
			}
		}
		columns = fmt.Sprintf("(%s)", strings.Join(columnsList, ",\n"))
	}

	configTemplate := `LOAD DATA
INFILE '%s'
APPEND
INTO TABLE %s
REENABLE DISABLED_CONSTRAINTS
FIELDS CSV WITH EMBEDDED 
TRAILING NULLCOLS
%s`
	return fmt.Sprintf(configTemplate, args.FilePath, args.TableNameTup.ForUserQuery(), columns)
	/*
	   reference for sqlldr control file
	   https://docs.oracle.com/en/database/oracle/oracle-database/19/sutil/oracle-sql-loader-control-file-contents.html#GUID-D1762699-8154-40F6-90DE-EFB8EB6A9AB0
	   REENABLE DISABLED_CONSTRAINTS - reenables all disabled constraints on the table
	   FIELDS CSV WITH EMBEDDED - specifies that the data file contains comma-separated values (CSV) with embedded newlines
	   TRAILING NULLCOLS - allows SQL*Loader to load a table when the record contains trailing null fields
	*/
}
