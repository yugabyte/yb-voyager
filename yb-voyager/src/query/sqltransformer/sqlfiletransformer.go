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
package sqltransformer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const (
	REMOVED_REDUNDANT_INDEXES_FILE_NAME            = "redundant_indexes.sql"
	SUGGESTION_TO_USE_SKIP_PERF_OPTIMIZATIONS_FLAG = "Use --skip-performance-recommendations true flag to skip applying performance recommendations to the index file"
	HASH_SPLITTING_SESSION_VARIABLE_ON             = "set yb_use_hash_splitting_by_default=on;"
	HASH_SPLITTING_SESSION_VARIABLE_OFF            = "set yb_use_hash_splitting_by_default=off;"
)

// ensurePlainBackup creates the canonical backup_<base> sibling alongside `file`
// and returns its path. The backup must always hold the PLAIN, pre-transform
// DDL so a PostgreSQL-compatible target (yb-amp) can import the originals.
//
// existingPlainBackup, when non-empty, is an already-created plain backup of the
// pre-transform DDL written by an earlier step (e.g. the colocation step renames
// the plain file to <file>.orig before writing colocation clauses into `file`).
// In that case we copy that plain file into backup_<base> rather than `file`,
// because `file` has already been mutated by the earlier step and is no longer
// plain. This is the fix for the clobber bug where copying the post-colocation
// `file` into backup_<base> destroyed the plain backup.
//
// If backup_<base> already exists it is left untouched (skip-if-exists) so the
// earliest/plain backup is preserved across multiple transform passes / re-runs.
func ensurePlainBackup(file string, existingPlainBackup string) (string, error) {
	backUpFile := filepath.Join(filepath.Dir(file), fmt.Sprintf("backup_%s", filepath.Base(file)))
	if utils.FileOrFolderExists(backUpFile) {
		// Preserve the earliest (plain) backup; do not clobber it.
		return backUpFile, nil
	}
	src := file
	if existingPlainBackup != "" && utils.FileOrFolderExists(existingPlainBackup) {
		// `file` was already mutated by an earlier step; copy the plain original.
		src = existingPlainBackup
	}
	if err := utils.CopyFile(src, backUpFile); err != nil {
		return "", fmt.Errorf("failed to copy %s to %s: %w", src, backUpFile, err)
	}
	return backUpFile, nil
}

// =========================INDEX FILE TRANSFORMER=====================================
type IndexFileTransformer struct {
	//skipping the performance optimizations with this parameter
	skipPerformanceOptimizations bool
	sourceDBType                 string

	//map of redundant index name to the existing index name to remove
	RedundantIndexesToExistingIndexToRemove *utils.StructMap[*sqlname.ObjectNameQualifiedWithTableName, string]
	//list of redundant indexes removed
	RemovedRedundantIndexes []*sqlname.ObjectNameQualifiedWithTableName
	//file name of the backup of the redundant indexes DDL
	RedundantIndexesFileName string

	//list of modified secondary indexes to range-sharded indexes
	ModifiedIndexesToRange []*sqlname.ObjectNameQualifiedWithTableName

	//backup file path of the index file
	backupFilePath string
}

func NewIndexFileTransformer(redundantIndexesToRemove *utils.StructMap[*sqlname.ObjectNameQualifiedWithTableName, string], skipPerformanceOptimizations bool, sourceDBType string) *IndexFileTransformer {
	return &IndexFileTransformer{
		RedundantIndexesToExistingIndexToRemove: redundantIndexesToRemove,
		RemovedRedundantIndexes:                 make([]*sqlname.ObjectNameQualifiedWithTableName, 0),
		ModifiedIndexesToRange:                  make([]*sqlname.ObjectNameQualifiedWithTableName, 0),
		skipPerformanceOptimizations:            skipPerformanceOptimizations,
		sourceDBType:                            sourceDBType,
	}
}

/*
Transform the index file to remove redundant indexes and modify secondary indexes to range-sharded indexes.

Input:
  - index file

Output:
  - backup index file
*/
func (t *IndexFileTransformer) Transform(file string) (string, error) {
	if t.skipPerformanceOptimizations {
		//TODO: change this logic to handle other type transformations that are not performance optimizations
		log.Infof("skipping performance optimizations for %s", file)
		return file, nil
	}

	var err error
	var parseTree *pg_query.ParseResult
	// Index files have no earlier-step backup, so the plain backup is just a copy
	// of `file` (still plain at this point), created skip-if-exists.
	backUpFile, err := ensurePlainBackup(file, "")
	if err != nil {
		return "", err
	}
	parseTree, err = queryparser.ParseSqlFile(file)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", file, err)
	}

	transformer := NewTransformer()

	//remove redundant indexes
	var removedIndexToStmtMap *utils.StructMap[*sqlname.ObjectNameQualifiedWithTableName, *pg_query.RawStmt]
	parseTree.Stmts, removedIndexToStmtMap, err = transformer.RemoveRedundantIndexes(parseTree.Stmts, t.RedundantIndexesToExistingIndexToRemove)
	if err != nil {
		return "", fmt.Errorf("failed to remove redundant indexes: %w", err)
	}
	t.RedundantIndexesFileName = filepath.Join(filepath.Dir(file), REMOVED_REDUNDANT_INDEXES_FILE_NAME)
	t.RemovedRedundantIndexes = removedIndexToStmtMap.ActualKeys()
	err = t.writeRemovedRedundantIndexesToFile(removedIndexToStmtMap)
	if err != nil {
		return "", fmt.Errorf("failed to write removed redundant indexes to file: %w", err)
	}

	parseTree.Stmts, t.ModifiedIndexesToRange, err = transformer.ModifySecondaryIndexesToRange(parseTree.Stmts)
	if err != nil {
		return "", fmt.Errorf("failed to modify secondary indexes to range: %w", err)
	}

	sqlStmts, err := queryparser.DeparseRawStmts(parseTree.Stmts)
	if err != nil {
		return "", fmt.Errorf("failed to deparse sql stmts: %w", err)
	}

	fileContent := strings.Join(sqlStmts, "\n\n")
	err = os.WriteFile(file, []byte(fileContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write transformed stmts to %s file: %w", file, err)
	}

	t.backupFilePath = backUpFile

	return backUpFile, nil
}

func (t *IndexFileTransformer) GetBackupFilePath() string {
	return t.backupFilePath
}

func (t *IndexFileTransformer) writeRemovedRedundantIndexesToFile(removedIndexToStmtMap *utils.StructMap[*sqlname.ObjectNameQualifiedWithTableName, *pg_query.RawStmt]) error {
	var removedSqlStmts []string
	var err error
	removedIndexToStmtMap.IterKV(func(key *sqlname.ObjectNameQualifiedWithTableName, value *pg_query.RawStmt) (bool, error) {
		//Add the existing index ddl in the comments for the individual redundant index
		stmtStr, err := queryparser.DeparseRawStmt(value)
		if err != nil {
			return false, fmt.Errorf("failed to deparse removed index stmt: %w", err)
		}
		if existingIndex, ok := t.RedundantIndexesToExistingIndexToRemove.Get(key); ok {
			stmtStr = fmt.Sprintf("/*\n Existing index: %s\n*/\n\n%s",
				existingIndex, stmtStr)
		}
		removedSqlStmts = append(removedSqlStmts, stmtStr)
		return true, nil
	})

	if len(removedSqlStmts) > 0 {
		// Write the removed indexes to a file
		redundantIndexContent := strings.Join(removedSqlStmts, "\n\n")
		err = os.WriteFile(t.RedundantIndexesFileName, []byte(redundantIndexContent), 0644)
		if err != nil {
			return fmt.Errorf("failed to write redundant indexes file: %w", err)
		}
	}
	return nil
}

//=========================TABLE FILE TRANSFORMER=====================================

// TODO: merge the sharding/colocated recommendation changes with this Table file transformation
type TableFileTransformer struct {
	//skipping the merge constraints with this parameter
	skipMergeConstraints                            bool
	MergedConstraints                               bool
	sourceDBType                                    string
	skipPerformanceOptimizations                    bool
	PKTablesOnTimestampWithRangeSharded             []string
	PKTablesWithHashSharded                         []string
	AppliedHashOrRangeShardingStrategyToConstraints bool

	//Colocation recommendation related information - currently this change is done outside of this transformer but whenever we merge it will be easier for consumers of transformer t o rely on same informaiton
	ShardedTables                    []string
	ColocatedTables                  []string
	ColocationRecommendationsApplied bool
	BackupFilePath                   string
}

func NewTableFileTransformer(skipMergeConstraints bool, sourceDBType string, skipPerformanceOptimizations bool) *TableFileTransformer {
	return &TableFileTransformer{
		skipMergeConstraints:         skipMergeConstraints,
		sourceDBType:                 sourceDBType,
		skipPerformanceOptimizations: skipPerformanceOptimizations,
	}
}

func (t *TableFileTransformer) Transform(file string) (string, error) {
	var err error
	var parseTree *pg_query.ParseResult
	// The colocation step (applyShardedTablesRecommendation) runs before this and,
	// when it fires, renames the PLAIN table file to <file>.orig and sets
	// t.BackupFilePath to it before writing colocation clauses into `file`. By the
	// time we get here `file` is post-colocation, so we must back up the plain
	// .orig instead of `file` to keep backup_<base> plain (the clobber-bug fix).
	backUpFile, err := ensurePlainBackup(file, t.BackupFilePath)
	if err != nil {
		return "", err
	}

	parseTree, err = queryparser.ParseSqlFile(file)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", file, err)
	}

	transformer := NewTransformer()

	if !t.skipMergeConstraints {
		parseTree.Stmts, err = transformer.MergeConstraints(parseTree.Stmts)
		if err != nil {
			return "", fmt.Errorf("error while merging constraints: %w", err)
		}
		t.MergedConstraints = true
	}

	if t.shouldConfigureShardingStrategyForConstraints() {
		parseTree.Stmts, t.PKTablesOnTimestampWithRangeSharded, t.PKTablesWithHashSharded, err = transformer.AddShardingStrategyForConstraints(parseTree.Stmts)
		if err != nil {
			return "", fmt.Errorf("failed to add hash splitting on for pk constraints: %w", err)
		}
		t.AppliedHashOrRangeShardingStrategyToConstraints = true
	}

	sqlStmts, err := queryparser.DeparseRawStmts(parseTree.Stmts)
	if err != nil {
		return "", fmt.Errorf("failed to deparse transformed raw stmts: %w", err)
	}

	fileContent := strings.Join(sqlStmts, "\n\n")

	err = os.WriteFile(file, []byte(fileContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write transformed table.sql file: %w", err)
	}

	t.BackupFilePath = backUpFile

	return backUpFile, nil
}

func (t *TableFileTransformer) GetBackupFilePath() string {
	//todo this should be the  backupFile in Transform method but the original Tables are dumpes here before colocation recommendation so keeping it like that right now
	//we can change this once we merge the colocation recommendation changes with the table file transformation
	return t.BackupFilePath
}

func (t *TableFileTransformer) shouldConfigureShardingStrategyForConstraints() bool {
	if t.sourceDBType != constants.POSTGRESQL {
		return false
	}
	return !t.skipPerformanceOptimizations
}

//=========================MVIEW FILE TRANSFORMER=====================================

type MviewFileTransformer struct {
	ShardedMviews                    []string
	ColocatedMviews                  []string
	BackupFilePath                   string
	ColocationRecommendationsApplied bool
}

func NewMviewFileTransformer() *MviewFileTransformer {
	return &MviewFileTransformer{}
}

// Transform does not modify the mview file content (mview transformations are,
// for now, applied outside this transformer by the colocation step). Its only
// job is to materialize the canonical plain backup_<base> sibling so a
// PostgreSQL-compatible target (yb-amp) can import the pre-colocation original.
// When the colocation step ran it set t.BackupFilePath to the plain <file>.orig;
// ensurePlainBackup copies that plain file (not the post-colocation `file`) into
// backup_<base>, skip-if-exists. When colocation did not run, no backup is made.
func (t *MviewFileTransformer) Transform(file string) (string, error) {
	if t.BackupFilePath == "" {
		// Nothing transformed the mview file (no colocation), so it is already
		// plain; no backup needed.
		return file, nil
	}
	backUpFile, err := ensurePlainBackup(file, t.BackupFilePath)
	if err != nil {
		return "", err
	}
	t.BackupFilePath = backUpFile
	return backUpFile, nil
}

func (t *MviewFileTransformer) GetBackupFilePath() string {
	return t.BackupFilePath
}
