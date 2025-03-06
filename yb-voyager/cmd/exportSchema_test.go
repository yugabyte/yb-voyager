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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShardingRecommendations(t *testing.T) {
	sqlInfo_mview1 := sqlInfo{
		objName:       "m1",
		stmt:          "CREATE MATERIALIZED VIEW m1 AS SELECT * FROM t1 WHERE a = 3",
		formattedStmt: "CREATE MATERIALIZED VIEW m1 AS SELECT * FROM t1 WHERE a = 3",
		fileName:      "",
	}
	sqlInfo_mview2 := sqlInfo{
		objName:       "m1",
		stmt:          "CREATE MATERIALIZED VIEW m1 AS SELECT * FROM t1 WHERE a = 3 with no data;",
		formattedStmt: "CREATE MATERIALIZED VIEW m1 AS SELECT * FROM t1 WHERE a = 3 with no data;",
		fileName:      "",
	}
	sqlInfo_mview3 := sqlInfo{
		objName:       "m1",
		stmt:          "CREATE MATERIALIZED VIEW m1 WITH (fillfactor=70) AS SELECT * FROM t1 WHERE a = 3 with no data",
		formattedStmt: "CREATE MATERIALIZED VIEW m1 WITH (fillfactor=70) AS SELECT * FROM t1 WHERE a = 3 with no data",
		fileName:      "",
	}
	source.DBType = POSTGRESQL
	modifiedSqlStmt, match, _ := applyShardingRecommendationIfMatching(&sqlInfo_mview1, []string{"m1"}, MVIEW)
	assert.Equal(t, strings.ToLower(modifiedSqlStmt),
		strings.ToLower("create materialized view m1 with (colocation=false) as select * from t1 where a = 3;"))
	assert.Equal(t, match, true)

	modifiedSqlStmt, match, _ = applyShardingRecommendationIfMatching(&sqlInfo_mview2, []string{"m1"}, MVIEW)
	assert.Equal(t, strings.ToLower(modifiedSqlStmt),
		strings.ToLower("create materialized view m1 with (colocation=false) as select * from t1 where a = 3 with no data;"))
	assert.Equal(t, match, true)

	modifiedSqlStmt, match, _ = applyShardingRecommendationIfMatching(&sqlInfo_mview2, []string{"m1_notfound"}, MVIEW)
	assert.Equal(t, modifiedSqlStmt, sqlInfo_mview2.stmt)
	assert.Equal(t, match, false)

	modifiedSqlStmt, match, _ = applyShardingRecommendationIfMatching(&sqlInfo_mview3, []string{"m1"}, MVIEW)
	assert.Equal(t, strings.ToLower(modifiedSqlStmt),
		strings.ToLower("create materialized view m1 with (fillfactor=70, colocation=false) "+
			"as select * from t1 where a = 3 with no data;"))
	assert.Equal(t, match, true)

	sqlInfo_table1 := sqlInfo{
		objName:       "m1",
		stmt:          "create table a (a int, b int)",
		formattedStmt: "create table a (a int, b int)",
		fileName:      "",
	}
	sqlInfo_table2 := sqlInfo{
		objName:       "m1",
		stmt:          "create table a (a int, b int) WITH (fillfactor=70);",
		formattedStmt: "create table a (a int, b int) WITH (fillfactor=70);",
		fileName:      "",
	}
	sqlInfo_table3 := sqlInfo{
		objName:       "m1",
		stmt:          "alter table a add col text;",
		formattedStmt: "alter table a add col text;",
		fileName:      "",
	}
	modifiedTableStmt, matchTable, _ := applyShardingRecommendationIfMatching(&sqlInfo_table1, []string{"a"}, TABLE)
	assert.Equal(t, strings.ToLower(modifiedTableStmt),
		strings.ToLower("create table a (a int, b int) WITH (colocation=false);"))
	assert.Equal(t, matchTable, true)

	modifiedTableStmt, matchTable, _ = applyShardingRecommendationIfMatching(&sqlInfo_table2, []string{"a"}, TABLE)
	assert.Equal(t, strings.ToLower(modifiedTableStmt),
		strings.ToLower("create table a (a int, b int) WITH (fillfactor=70, colocation=false);"))
	assert.Equal(t, matchTable, true)

	modifiedSqlStmt, matchTable, _ = applyShardingRecommendationIfMatching(&sqlInfo_table2, []string{"m1_notfound"}, TABLE)
	assert.Equal(t, modifiedSqlStmt, sqlInfo_table2.stmt)
	assert.Equal(t, matchTable, false)

	modifiedTableStmt, matchTable, _ = applyShardingRecommendationIfMatching(&sqlInfo_table3, []string{"a"}, TABLE)
	assert.Equal(t, strings.ToLower(modifiedTableStmt),
		strings.ToLower(sqlInfo_table3.stmt))
	assert.Equal(t, matchTable, false)
}
