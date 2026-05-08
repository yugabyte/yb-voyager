//go:build unit

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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/types"
)

func TestAnalyzeTableColumnOps(t *testing.T) {
	queryStats := []*types.QueryStats{
		{QueryText: "SELECT a,b FROM t WHERE x = 1", ExecutionCount: 10},
		{QueryText: "SELECT * FROM t WHERE x = 1", ExecutionCount: 5},
		{QueryText: "UPDATE t SET a = 100 WHERE x = 1", ExecutionCount: 2},
	}

	metrics := analyzeTableColumnOps(queryStats, "", "t", []string{"a", "b", "c", "x"})

	readCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.readOps > 0 })
	writeCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.updateWriteOps > 0 })
	filteringCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.filterOps > 0 })
	require.ElementsMatch(t, []string{"a", "b", "c", "x"}, readCols)
	require.ElementsMatch(t, []string{"a"}, writeCols)
	require.ElementsMatch(t, []string{"x"}, filteringCols)
}

func TestAnalyzeTableColumnOpsJoinOn(t *testing.T) {
	queryStats := []*types.QueryStats{
		{QueryText: "SELECT t.a FROM t JOIN s ON t.x = s.id WHERE t.x = 1", ExecutionCount: 10},
	}
	metrics := analyzeTableColumnOps(queryStats, "", "t", []string{"a", "x"})
	readCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.readOps > 0 })
	filteringCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.filterOps > 0 })
	assert.Equal(t, []string{"a", "x"}, readCols)
	assert.Equal(t, []string{"x"}, filteringCols)
}

func TestAnalyzeTableColumnOpsGroupByOrderBy(t *testing.T) {
	queryStats := []*types.QueryStats{
		{QueryText: "SELECT a, COUNT(*) FROM t WHERE x = 1 GROUP BY a ORDER BY a", ExecutionCount: 5},
	}
	metrics := analyzeTableColumnOps(queryStats, "", "t", []string{"a", "x"})
	readCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.readOps > 0 })
	assert.Equal(t, []string{"a", "x"}, readCols)
}

func TestAnalyzeTableColumnOpsHaving(t *testing.T) {
	queryStats := []*types.QueryStats{
		{QueryText: "SELECT a, COUNT(*) FROM t GROUP BY a HAVING COUNT(*) > 5", ExecutionCount: 5},
	}
	metrics := analyzeTableColumnOps(queryStats, "", "t", []string{"a", "b"})
	readCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.readOps > 0 })
	assert.Equal(t, []string{"a"}, readCols)
}

func TestAnalyzeTableColumnOpsHavingWithColRef(t *testing.T) {
	queryStats := []*types.QueryStats{
		{QueryText: "SELECT a FROM t GROUP BY a HAVING SUM(b) > 10", ExecutionCount: 5},
	}
	metrics := analyzeTableColumnOps(queryStats, "", "t", []string{"a", "b"})
	readCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.readOps > 0 })
	filterCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.filterOps > 0 })
	assert.Equal(t, []string{"a", "b"}, readCols)
	assert.Equal(t, []string{"b"}, filterCols)
}

func TestAnalyzeTableColumnOpsUnionAll(t *testing.T) {
	queryStats := []*types.QueryStats{
		{QueryText: "SELECT a, b FROM t WHERE x = 1 UNION ALL SELECT a, b FROM t WHERE x = 2", ExecutionCount: 10},
	}
	metrics := analyzeTableColumnOps(queryStats, "", "t", []string{"a", "b", "x"})
	readCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.readOps > 0 })
	filterCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.filterOps > 0 })
	assert.Equal(t, []string{"a", "b", "x"}, readCols)
	assert.Equal(t, []string{"x"}, filterCols)
}

func TestAnalyzeTableColumnOpsUnionThreeWay(t *testing.T) {
	queryStats := []*types.QueryStats{
		{QueryText: "SELECT a FROM t WHERE x = 1 UNION SELECT b FROM t WHERE x = 2 UNION SELECT a FROM t WHERE x = 3", ExecutionCount: 5},
	}
	metrics := analyzeTableColumnOps(queryStats, "", "t", []string{"a", "b", "x"})
	readCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.readOps > 0 })
	filterCols := extractColumnsByPredicate(metrics, func(m columnOpsMetric) bool { return m.filterOps > 0 })
	assert.Equal(t, []string{"a", "b", "x"}, readCols)
	assert.Equal(t, []string{"x"}, filterCols)
}

func TestCollectReferencedColumnsForQueryJoin(t *testing.T) {
	cols, err := collectReferencedColumnsForQuery(`
		SELECT s.id, ss.reblogs_count
		FROM statuses s
		JOIN status_stats ss ON ss.status_id = s.id
		WHERE s.id = $1`)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"id": true}, cols["statuses"])
	assert.Equal(t, map[string]bool{"status_id": true, "reblogs_count": true}, cols["status_stats"])
}

func TestParsePostgresArray(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, parsePostgresArray("{a,b,c}"))
	assert.Equal(t, []string{"x"}, parsePostgresArray("{x}"))
	assert.Equal(t, []string(nil), parsePostgresArray("{}"))
}
