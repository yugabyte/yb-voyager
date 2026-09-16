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
package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	goerrors "github.com/go-errors/errors"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

/*
Restore sequence in offline migration via debezium or pg_dump/ora2pg
getting sequences to last value mapping from different sources - debezium / ora2pg / pg_dump

For source type is PG -
	we are sure that all the sequences being migrated are attached to the tables in all cases either user provided table-list or not from export data side
	so we can skip the sequence if it is not attached to a
	table in the table list and in scenario if table is not present in the target database, we will skip the sequences attached to tables
	not part of table list

For any other sources
	we are not sure if all the sequences are attached to the tables so we will restore all the sequences
	and in scenario if table is not present in the target database, we will error out if any of them not present in the target database
	which is the same behaviour as before as for example for oracle, its complicated to figure out if a sequence is attached to a table or
	not, hence not doing that for now as its not done on export side

if import data has table list filteration and source type is POSTGRESQL
	if a sequence is  attached to a table - tableColumn -> sequence name
		if table is part of table list being imported
			yes then continue exectuing
		else
			skip the sequence
	else //independent sequence  // not possible right now
		skip the sequence
else //
	execute the sequence

while executing the sequence, we are using the setval('sequence_name', last_value) function consitently irrespective of export tool to
restore sequence last value there is a change in behaviour for ora2pg now as earlier we were directly executing the DDL dumped in
postdata.sql file now we are fetching the sequence name and last value from the ALTER SEQUENCE <seq> RESTART WITH <last_value>
and using the setval('sequence_name', last_value) function to restore sequence last value

The behaviour of ALTER SEQUENCE RESTART WITH val is different than setval('sequence_name', last_value)
as the setval updates the currval to the last_Value so the next value used further is the last_value + 1
but in case of ALTER SEQUENCE RESTART WITH val, the currval is previous value and next value is the last_value

but its okay because sequence value should always be unique even if there are some gaps in between.

*/

func restoreSequencesInOfflineMigration(msr *metadb.MigrationStatusRecord, importTableList []sqlname.NameTuple) error {
	// offline migration; either using dbzm or pg_dump/ora2pg
	var err error
	sequenceTupleToLastValue, err := fetchSequenceLastValueMap(msr)
	if err != nil {
		return fmt.Errorf("failed to fetch sequence last value map: %w", err)
	}
	sequenceNameTupleToLastValueMap := utils.NewStructMap[sqlname.NameTuple, int64]()

	if shouldFilterSequences(msr.SourceDBConf.DBType) {
		//if import data has table list filteration
		//get the sequence name to table name map from MSR
		sequenceNameToTableMap, err := fetchSequenceToTableListMap(msr)
		if err != nil {
			return fmt.Errorf("failed to fetch sequence to table list map: %w", err)
		}
		sequenceNameTupleToLastValueMap, err = filterSequencesAsPerImportTableList(sequenceTupleToLastValue, importTableList, sequenceNameToTableMap)
		if err != nil {
			return fmt.Errorf("failed to filter sequences as per import table list: %w", err)
		}

	} else {
		//restore all sequences
		//if tables are not being filtered then ideally we should restore all sequences
		err = sequenceTupleToLastValue.IterKV(func(sequenceTuple sqlname.NameTuple, lastValue int64) (bool, error) {
			if !sequenceTuple.TargetTableAvailable() {
				return false, goerrors.Errorf("sequence %q is not present in the target database", sequenceTuple.ForKey())
			}
			sequenceNameTupleToLastValueMap.Put(sequenceTuple, lastValue)
			return true, nil
		})
		if err != nil {
			return fmt.Errorf("failed to check sequences available on target: %w", err)
		}
	}

	err = correctSequenceLastValuesAgainstTableMax(sequenceNameTupleToLastValueMap)
	if err != nil {
		return fmt.Errorf("failed to correct sequence last values: %w", err)
	}

	//restore the sequence last value
	err = tdb.RestoreSequences(sequenceNameTupleToLastValueMap)
	if err != nil {
		return fmt.Errorf("failed to restore sequences: %w", err)
	}
	return nil
}

func restoreSequencesInLiveMigration(sequenceLastValue map[string]int64) error {
	if importerRole == SOURCE_REPLICA_DB_IMPORTER_ROLE && tconf.TargetDBType == ORACLE {
		//for oracle source-replica import data, we don't restore sequences as its not supported
		//so no need to do anything here right now
		return nil
	}
	sequenceNameTupleToLastValueMap := utils.NewStructMap[sqlname.NameTuple, int64]()
	for sequenceName, lastValue := range sequenceLastValue {
		sequenceTuple, err := namereg.NameReg.LookupTableName(sequenceName)
		if err != nil {
			return fmt.Errorf("error looking up sequence name %q: %w", sequenceName, err)
		}
		sequenceNameTupleToLastValueMap.Put(sequenceTuple, lastValue)
	}
	err := correctSequenceLastValuesAgainstTableMax(sequenceNameTupleToLastValueMap)
	if err != nil {
		return fmt.Errorf("failed to correct sequence last values: %w", err)
	}
	err = tdb.RestoreSequences(sequenceNameTupleToLastValueMap)
	if err != nil {
		return fmt.Errorf("failed to restore sequences: %w", err)
	}
	return nil
}

// StrictSequenceRestoreEnvVar turns the correction below into a hard failure instead of
// a warning, so that a lagging exported sequence value can be surfaced loudly rather
// than absorbed. Off by default; enable it when you want a lag to fail the run.
const StrictSequenceRestoreEnvVar = "YB_VOYAGER_STRICT_SEQUENCE_RESTORE"

type sequenceOwnerColumn struct {
	table  sqlname.NameTuple
	column string
}

/*
correctSequenceLastValuesAgainstTableMax raises each sequence's restore value to the
maximum value actually present in the column(s) that sequence feeds, whenever the
exported value lags behind the data.

The exported value is a running maximum of the values observed in the change stream. If
the last events before cutover are not folded into it before it is read, setval() leaves
the sequence at or below a value already present in the table, and the next insert that
relies on the sequence default fails with a duplicate key error. Using
GREATEST(exported, table max) makes restoration correct regardless of why the exported
value lagged behind.

Sequences whose exported value is 0 are left untouched, matching the convention in
RestoreSequences that such values can be legitimate, for example cyclic sequences.
*/
func correctSequenceLastValuesAgainstTableMax(sequenceNameTupleToLastValueMap *utils.StructMap[sqlname.NameTuple, int64]) error {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("get migration status record: %w", err)
	}
	sequenceToColumns, err := fetchSequenceToColumnListMap(msr)
	if err != nil {
		return fmt.Errorf("failed to fetch sequence to column list map: %w", err)
	}

	var laggedSequences []string
	byKey := func(a, b sqlname.NameTuple) bool { return a.ForKey() < b.ForKey() }

	err = sequenceNameTupleToLastValueMap.IterKVSorted(byKey, func(sequenceTuple sqlname.NameTuple, lastValue int64) (bool, error) {
		if lastValue == 0 {
			return true, nil
		}
		ownerColumns, ok := sequenceToColumns.Get(sequenceTuple)
		if !ok {
			return true, nil
		}
		tableMax := maxValueAcrossSequenceColumns(sequenceTuple, ownerColumns)
		if tableMax <= lastValue {
			return true, nil
		}
		laggedSequences = append(laggedSequences,
			fmt.Sprintf("%s (exported=%d, data max=%d)", sequenceTuple.ForKey(), lastValue, tableMax))
		utils.PrintAndLogfWarning("sequence %s: exported last value %d is behind the maximum value %d already present in the data; "+
			"restoring the sequence to that maximum instead.", sequenceTuple.ForKey(), lastValue, tableMax)
		sequenceNameTupleToLastValueMap.Put(sequenceTuple, tableMax)
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to compare sequence values against the data: %w", err)
	}

	if utils.GetEnvAsBool(StrictSequenceRestoreEnvVar, false) && len(laggedSequences) > 0 {
		sort.Strings(laggedSequences)
		return goerrors.Errorf("exported sequence values lagged behind the migrated data for %d sequence(s): %s (%s is enabled)",
			len(laggedSequences), strings.Join(laggedSequences, ", "), StrictSequenceRestoreEnvVar)
	}
	return nil
}

// sequenceColumnMaxQuerier is overridden in tests.
var sequenceColumnMaxQuerier = func(query string) (string, error) {
	var value string
	err := tdb.QueryRow(query).Scan(&value)
	return value, err
}

func maxValueAcrossSequenceColumns(sequenceTuple sqlname.NameTuple, ownerColumns []sequenceOwnerColumn) int64 {
	var maxValue int64
	for _, ownerColumn := range ownerColumns {
		if !ownerColumn.table.TargetTableAvailable() {
			continue
		}
		columnMax, ok := maxValueOfSequenceColumn(sequenceTuple, ownerColumn)
		if ok && columnMax > maxValue {
			maxValue = columnMax
		}
	}
	return maxValue
}

/*
maxValueOfSequenceColumn reads the largest value in one column, reporting ok=false when
it cannot be read.

The mapping records the source-side spelling of the column and nothing maps it to the
target, the way the name registry maps table names: a MySQL or Oracle source column
arrives as ID or AlbumId while the target column is folded to lower case. Quoting keeps a
genuinely case-sensitive name addressable, leaving it unquoted lets the target fold it,
so both are attempted.

A column whose maximum still cannot be read only costs the comparison for that column, so
it is reported and skipped. Failing here would break migrations that work today, since
the target spelling cannot be resolved for every source type.
*/
func maxValueOfSequenceColumn(sequenceTuple sqlname.NameTuple, ownerColumn sequenceOwnerColumn) (int64, bool) {
	columnRefs := []string{quoteIdentifierIfUnquoted(ownerColumn.column)}
	if ownerColumn.column != columnRefs[0] {
		columnRefs = append(columnRefs, ownerColumn.column)
	}

	var lastErr error
	for _, columnRef := range columnRefs {
		query := fmt.Sprintf(`SELECT COALESCE(MAX(%s)::text, '0') FROM %s`, columnRef, ownerColumn.table.ForUserQuery())
		raw, err := sequenceColumnMaxQuerier(query)
		if err != nil {
			lastErr = err
			continue
		}
		columnMax, isInteger := parseSequenceColumnMax(raw)
		if !isInteger {
			log.Warnf("column %s of table %s feeding sequence %s does not hold integer values (max is %q); skipping it",
				ownerColumn.column, ownerColumn.table.ForKey(), sequenceTuple.ForKey(), raw)
			return 0, false
		}
		return columnMax, true
	}

	log.Warnf("could not read the maximum of column %s of table %s feeding sequence %s, so its exported value is used as is: %v",
		ownerColumn.column, ownerColumn.table.ForKey(), sequenceTuple.ForKey(), lastErr)
	return 0, false
}

func parseSequenceColumnMax(raw string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// fetchSequenceToColumnListMap maps each sequence to the columns it feeds. It reads the
// source-side mapping and resolves both names through the name registry, which maps them
// to the database being imported into, the same way fetchSequenceToTableListMap does.
func fetchSequenceToColumnListMap(msr *metadb.MigrationStatusRecord) (*utils.StructMap[sqlname.NameTuple, []sequenceOwnerColumn], error) {
	sequenceToColumns := utils.NewStructMap[sqlname.NameTuple, []sequenceOwnerColumn]()
	for column, sequenceName := range msr.SourceColumnToSequenceMapping {
		parts := strings.Split(column, ".") //column is qualified schema.tablename.colname
		if len(parts) != 3 {
			return nil, goerrors.Errorf("invalid qualified column name %q in column to sequence mapping", column)
		}
		tableName := fmt.Sprintf("%s.%s", parts[0], parts[1])
		tableNameTuple, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(tableName)
		if err != nil {
			return nil, fmt.Errorf("error looking up table name %q: %w", tableName, err)
		}
		sequenceTuple, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(sequenceName)
		if err != nil {
			return nil, fmt.Errorf("error looking up sequence name %q: %w", sequenceName, err)
		}
		ownerColumn := sequenceOwnerColumn{table: tableNameTuple, column: parts[2]}
		ownerColumns, _ := sequenceToColumns.Get(sequenceTuple)
		sequenceToColumns.Put(sequenceTuple, append(ownerColumns, ownerColumn))
	}
	return sequenceToColumns, nil
}

func quoteIdentifierIfUnquoted(identifier string) string {
	if sqlname.IsQuoted(identifier) {
		return identifier
	}
	return fmt.Sprintf(`"%s"`, identifier)
}

func shouldFilterSequences(sourceType string) bool {
	//if source type is not POSTGRESQL, we don't filter sequences
	if sourceType != POSTGRESQL {
		return false
	}
	//if table list filteration is enabled, we filter sequences
	return tconf.TableList != "" || tconf.ExcludeTableList != ""
}

func fetchSequenceLastValueMap(msr *metadb.MigrationStatusRecord) (*utils.StructMap[sqlname.NameTuple, int64], error) {
	isDebeziumExport := msr.IsSnapshotExportedViaDebezium()
	sourceType := msr.SourceDBConf.DBType
	sequenceLastValue := make(map[string]int64)
	var err error
	if isDebeziumExport {
		//read the sequence last value from the export status file if export is via debezium
		status, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
		if err != nil {
			return nil, fmt.Errorf("failed to read export status for restore sequences: %w", err)
		}
		sequenceLastValue = status.Sequences
	} else {
		//read the sequence last value from the postdata.sql file if export is not via debezium
		sequenceFilePath := filepath.Join(exportDir, "data", "postdata.sql")
		if utils.FileOrFolderExists(sequenceFilePath) {
			fmt.Printf("setting resume value for sequences %10s\n", "")
			sequenceLastValue, err = readSequenceLastValueFromPostDataSql(sequenceFilePath, sourceType)
			if err != nil {
				return nil, fmt.Errorf("failed to read sequence last value from postdata.sql: %w", err)
			}
		}
	}
	sequenceTupleToLastValueMap := utils.NewStructMap[sqlname.NameTuple, int64]()
	for sequenceName, lastValue := range sequenceLastValue {
		sequenceTuple, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(sequenceName)
		if err != nil {
			return nil, fmt.Errorf("error looking up sequence name %q: %w", sequenceName, err)
		}
		sequenceTupleToLastValueMap.Put(sequenceTuple, lastValue)
	}
	return sequenceTupleToLastValueMap, nil
}

func fetchSequenceToTableListMap(msr *metadb.MigrationStatusRecord) (*utils.StructMap[sqlname.NameTuple, []sqlname.NameTuple], error) {
	sourceColumnToSequenceMapping := msr.SourceColumnToSequenceMapping

	sequenceNameToTableMap := utils.NewStructMap[sqlname.NameTuple, []sqlname.NameTuple]()
	for column, sequenceName := range sourceColumnToSequenceMapping {
		parts := strings.Split(column, ".") //column is qualified tablename.colname
		if len(parts) < 3 {
			return nil, goerrors.Errorf("invalid column name: %s", column)
		}
		tableName := fmt.Sprintf("%s.%s", parts[0], parts[1])
		tableNameTuple, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(tableName)
		if err != nil {
			return nil, fmt.Errorf("error looking up table name %q: %w", tableName, err)
		}
		//get the sequence name from the source column to sequence mapping
		//lookup the sequence name from source in name reg and ignore if target not found
		sequenceTuple, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(sequenceName)
		if err != nil {
			return nil, fmt.Errorf("error looking up sequence name %q: %w", sequenceName, err)
		}
		tableList, ok := sequenceNameToTableMap.Get(sequenceTuple)
		if ok {
			tableList = append(tableList, tableNameTuple)
			sequenceNameToTableMap.Put(sequenceTuple, tableList)
		} else {
			sequenceNameToTableMap.Put(sequenceTuple, []sqlname.NameTuple{tableNameTuple})
		}
	}
	return sequenceNameToTableMap, nil
}

func checkIfSequenceAttachedToTablesInTableList(importTableList []sqlname.NameTuple, sequenceAttachedTables []sqlname.NameTuple) (bool, error) {
	for _, tableTuple := range sequenceAttachedTables {
		if lo.ContainsBy(importTableList, func(t sqlname.NameTuple) bool {
			return t.ForKey() == tableTuple.ForKey()
		}) {
			return true, nil
		}
	}
	return false, nil
}

func filterSequencesAsPerImportTableList(sequenceTupleToLastValue *utils.StructMap[sqlname.NameTuple, int64], importTableList []sqlname.NameTuple, sequenceNameToTableMap *utils.StructMap[sqlname.NameTuple, []sqlname.NameTuple]) (*utils.StructMap[sqlname.NameTuple, int64], error) {
	sequenceNameTupleToLastValueMap := utils.NewStructMap[sqlname.NameTuple, int64]()

	err := sequenceTupleToLastValue.IterKV(func(sequenceTuple sqlname.NameTuple, lastValue int64) (bool, error) {
		sequenceAttachedToTables, ok := sequenceNameToTableMap.Get(sequenceTuple)
		if !ok {
			//sequence is not attached to any table, right now this is not expected from export as we only migrate sequences
			// attached to tables in all migration workflows
			log.Infof("sequence %q is not attached to any table, skipping", sequenceTuple.ForKey())
			return true, nil
		}
		//check if the sequence is attached to a table and present in the table list
		isTablePresentInTableList, err := checkIfSequenceAttachedToTablesInTableList(importTableList, sequenceAttachedToTables)
		if err != nil {
			return false, fmt.Errorf("failed to check if sequence is attached to tables in table list: %w", err)
		}
		if !isTablePresentInTableList {
			//if the sequence is attached to a table but not in the table list
			//skip the sequence
			log.Infof("sequence %q is not present in the table list, skipping", sequenceTuple.ForKey())
			return true, nil
		}
		if !sequenceTuple.TargetTableAvailable() {
			//if the sequence is attached to a table and table in table list but sequence not present in the target database
			//return error
			return false, goerrors.Errorf("sequence %q is not present in the target database", sequenceTuple.ForKey())
		}
		//restore the sequence last value
		sequenceNameTupleToLastValueMap.Put(sequenceTuple, lastValue)
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to filter sequences as per import table list: %w", err)
	}
	return sequenceNameTupleToLastValueMap, nil
}

func readSequenceLastValueFromPostDataSql(sequenceFilePath string, sourceDBType string) (map[string]int64, error) {
	sequenceLastValue := make(map[string]int64)
	//get all the setval statements from the postdata.sql file
	sqlInfoArr := parseSqlFileForObjectType(sequenceFilePath, "SEQUENCE")
	for _, sqlInfo := range sqlInfoArr {
		parseTree, err := queryparser.Parse(sqlInfo.stmt)
		if err != nil {
			return nil, goerrors.Errorf("error parsing the ddl[%s]: %w", sqlInfo.stmt, err)
		}
		var sequenceName string
		var lastValue int64
		switch sourceDBType {
		case POSTGRESQL:
			//check if the statement is a setval statement for POSTGRESQL SELECT setval('sequence_name', last_value)
			if !queryparser.IsSelectSetValStmt(parseTree) {
				log.Infof("not a setval statement: %s", sqlInfo.stmt)
				continue
			}
			//get the sequence name and last value from the setval statement
			sequenceName, lastValue, err = queryparser.GetSequenceNameAndLastValueFromSetValStmt(parseTree)
			if err != nil {
				return nil, fmt.Errorf("error getting sequence name and last value from setval statement: %w", err)
			}
		case ORACLE, MYSQL:
			//in case of ORACLE and MYSQL, we need to check if its as ALTER SEQUENCE seq RESTART 11 because ora2pg dumps it like that
			if !queryparser.IsAlterSequenceStmt(parseTree) {
				log.Infof("not an alter sequence statement: %s", sqlInfo.stmt)
				continue
			}
			//get the sequence name and last value from the alter sequence statement
			//taking the restart value as the last value since its complicated to get the last value from this restart value as the sequence can't just +1 there can increment with any value asconfigured.
			//Its okay to use this also as the main purpose of sequence to provide unique values and there can be gaps in between.
			sequenceName, lastValue, err = queryparser.GetSequenceNameAndRestartValueFromAlterSequenceStmt(parseTree)
			if err != nil {
				return nil, fmt.Errorf("error getting sequence name and last value from alter sequence statement: %w", err)
			}
		}
		sequenceLastValue[sequenceName] = lastValue
	}
	return sequenceLastValue, nil
}
