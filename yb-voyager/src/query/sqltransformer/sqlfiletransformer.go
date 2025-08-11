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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const (
	REMOVED_REDUNDANT_INDEXES_FILE_NAME = "redundant_indexes.sql"
	SUGGESTION_TO_USE_SKIP_PERF_OPTIMIZATIONS_FLAG = "Use --skip-performance-optimizations true flag to skip applying performance optimizations to the index file"
)

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
	ModifiedIndexesToRange []string
}

func NewIndexFileTransformer(redundantIndexesToRemove *utils.StructMap[*sqlname.ObjectNameQualifiedWithTableName, string], skipPerformanceOptimizations bool, sourceDBType string) *IndexFileTransformer {
	return &IndexFileTransformer{
		RedundantIndexesToExistingIndexToRemove: redundantIndexesToRemove,
		RemovedRedundantIndexes:                 make([]*sqlname.ObjectNameQualifiedWithTableName, 0),
		ModifiedIndexesToRange:                  make([]string, 0),
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
	var err error
	var parseTree *pg_query.ParseResult
	backUpFile := filepath.Join(filepath.Dir(file), fmt.Sprintf("backup_%s", filepath.Base(file)))
	//copy files
	err = utils.CopyFile(file, backUpFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy %s to %s: %w", file, backUpFile, err)
	}

	parseTree, err = queryparser.ParseSqlFile(file)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", file, err)
	}

	transformer := NewTransformer()
	if !t.skipPerformanceOptimizations {

		//remove redundant indexes
		var removedIndexToStmtMap *utils.StructMap[*sqlname.ObjectNameQualifiedWithTableName, *pg_query.RawStmt]
		parseTree.Stmts, removedIndexToStmtMap, err = transformer.RemoveRedundantIndexes(parseTree.Stmts, t.RedundantIndexesToExistingIndexToRemove)
		if err != nil {
			return "", fmt.Errorf("failed to remove redundant indexes: %w\n%s", err, SUGGESTION_TO_USE_SKIP_PERF_OPTIMIZATIONS_FLAG)
		}
		t.RedundantIndexesFileName = filepath.Join(filepath.Dir(file), REMOVED_REDUNDANT_INDEXES_FILE_NAME)
		err = t.writeRemovedRedundantIndexesToFile(removedIndexToStmtMap)
		if err != nil {
			return "", fmt.Errorf("failed to write removed redundant indexes to file: %w\n%s", err, SUGGESTION_TO_USE_SKIP_PERF_OPTIMIZATIONS_FLAG)
		}

	} else {
		log.Infof("skipping performance optimizations for %s", file)
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

	return backUpFile, nil
}

func (t *IndexFileTransformer) writeRemovedRedundantIndexesToFile(removedIndexToStmtMap *utils.StructMap[*sqlname.ObjectNameQualifiedWithTableName, *pg_query.RawStmt]) error {
	var removedSqlStmts []string
	var err error
	var removedIndexes []*sqlname.ObjectNameQualifiedWithTableName
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
		removedIndexes = append(removedIndexes, key)
		return true, nil
	})
	t.RemovedRedundantIndexes = removedIndexes

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
	skipMergeConstraints bool
	sourceDBType         string
}

func NewTableFileTransformer(skipMergeConstraints bool, sourceDBType string) *TableFileTransformer {
	return &TableFileTransformer{
		skipMergeConstraints: skipMergeConstraints,
		sourceDBType:         sourceDBType,
	}
}

func (t *TableFileTransformer) Transform(file string) (string, error) {
	var err error
	var parseTree *pg_query.ParseResult
	backUpFile := filepath.Join(filepath.Dir(file), fmt.Sprintf("backup_%s", filepath.Base(file)))
	//copy files
	err = utils.CopyFile(file, backUpFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy %s to %s: %w", file, backUpFile, err)
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

	return backUpFile, nil
}
