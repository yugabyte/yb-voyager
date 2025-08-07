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

type IndexFileTransformer struct {
	skipPerformanceOptimizations bool
	sourceDBType                 string
	redundantIndexesToRemove     map[string]string
	RemovedRedundantIndexes      []*sqlname.ObjectNameQualifiedWithTableName
	ModifiedIndexesToRange       []string
	RedundantIndexesFileName     string
}

func NewIndexFileTransformer(redundantIndexesToRemove map[string]string, skipPerformanceOptimizations bool, sourceDBType string) *IndexFileTransformer {
	return &IndexFileTransformer{
		redundantIndexesToRemove:     redundantIndexesToRemove,
		RemovedRedundantIndexes:      make([]*sqlname.ObjectNameQualifiedWithTableName, 0),
		ModifiedIndexesToRange:       make([]string, 0),
		skipPerformanceOptimizations: skipPerformanceOptimizations,
		sourceDBType:                 sourceDBType,
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
	backUpFile := fmt.Sprintf("%s.orig", file)
	//copy files
	err = utils.CopyFile(file, backUpFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy %s to %s: %w", file, backUpFile, err)
	}

	parseTree, err = queryparser.ParseSqlFile(file)
	if err != nil {
		//In case of PG we should return error but for other SourceDBTypes we should return nil as the parser can fail
		errMsg := fmt.Errorf("failed to parse %s: %w", file, err)
		if t.sourceDBType != constants.POSTGRESQL {
			log.Infof("skipping error while parsing %s: %v", file, err)
			errMsg = nil
		}
		return "", errMsg
	}

	transformer := NewTransformer()

	var removedSqlStmts []string
	if !t.skipPerformanceOptimizations && t.sourceDBType == constants.POSTGRESQL {
		parseTree.Stmts, removedSqlStmts, t.RemovedRedundantIndexes, err = transformer.RemoveRedundantIndexes(parseTree.Stmts, t.redundantIndexesToRemove)
		if err != nil {
			return "", fmt.Errorf("failed to remove redundant indexes: %w", err)
		}
		if len(removedSqlStmts) > 0 {
			// Write the removed indexes to a file
			t.RedundantIndexesFileName = filepath.Join(filepath.Dir(file), "redundant_indexes.sql")
			redundantIndexContent := strings.Join(removedSqlStmts, "\n\n")
			err = os.WriteFile(t.RedundantIndexesFileName, []byte(redundantIndexContent), 0644)
			if err != nil {
				return "", fmt.Errorf("failed to write redundant indexes file: %w", err)
			}
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

type TableFileTransformer struct {
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
	//TODO: Keep the format of backup file as <file>.orig but for now making it
	// after merging the sharding changes in this transformer
	backUpFile := filepath.Join(filepath.Dir(file), "table_before_merge_constraints.sql")
	//copy files
	err = utils.CopyFile(file, backUpFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy %s to %s: %w", file, backUpFile, err)
	}

	parseTree, err = queryparser.ParseSqlFile(file)
	if err != nil {
		//In case of PG we should return error but for other SourceDBTypes we should return nil as the parser can fail
		errMsg := fmt.Errorf("failed to parse %s: %w", file, err)
		if t.sourceDBType != constants.POSTGRESQL {
			log.Infof("skipping error while parsing %s: %v", file, err)
			errMsg = nil
		}
		return "", errMsg
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
