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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// ---------- linkParamsToColumns ----------

func TestLinkParamsToColumns_Equality(t *testing.T) {
	links, err := linkParamsToColumns("SELECT id FROM public.users WHERE id = $1")
	require.NoError(t, err)
	require.Contains(t, links, 1)
	assert.Equal(t, "users", links[1].table)
	assert.Equal(t, "id", links[1].column)
}

func TestLinkParamsToColumns_ParamLeftSide(t *testing.T) {
	links, err := linkParamsToColumns("SELECT id FROM users WHERE $1 = id")
	require.NoError(t, err)
	require.Contains(t, links, 1)
	assert.Equal(t, "id", links[1].column)
}

func TestLinkParamsToColumns_InList(t *testing.T) {
	links, err := linkParamsToColumns("SELECT id FROM orders WHERE status IN ($1, $2, $3)")
	require.NoError(t, err)
	require.Len(t, links, 3)
	assert.Equal(t, "status", links[1].column)
	assert.Equal(t, "status", links[2].column)
	assert.Equal(t, "status", links[3].column)
}

func TestLinkParamsToColumns_Between(t *testing.T) {
	links, err := linkParamsToColumns("SELECT id FROM orders WHERE created_at BETWEEN $1 AND $2")
	require.NoError(t, err)
	require.Len(t, links, 2)
	assert.Equal(t, "created_at", links[1].column)
	assert.Equal(t, "created_at", links[2].column)
}

func TestLinkParamsToColumns_QualifiedAlias(t *testing.T) {
	q := "SELECT u.id FROM public.users u WHERE u.email = $1 AND u.status = $2"
	links, err := linkParamsToColumns(q)
	require.NoError(t, err)
	assert.Equal(t, "users", links[1].table)
	assert.Equal(t, "email", links[1].column)
	assert.Equal(t, "status", links[2].column)
}

func TestLinkParamsToColumns_NoParams(t *testing.T) {
	_, err := linkParamsToColumns("SELECT * FROM users WHERE id = 1")
	assert.Error(t, err)
}

func TestLinkParamsToColumns_UnqualifiedColumnDisambiguatedByCatalog(t *testing.T) {
	oldFn := relationHasColumnFn
	t.Cleanup(func() { relationHasColumnFn = oldFn })
	relationHasColumnFn = func(tgt paramTarget, col string) bool {
		return tgt.table == "frequent_flyer" && col == "ff_c_id_str"
	}

	links, err := linkParamsToColumns(
		"SELECT C_ID, FF_AL_ID FROM customer, frequent_flyer WHERE FF_C_ID_STR = $1 AND FF_C_ID = C_ID",
	)
	require.NoError(t, err)
	require.Contains(t, links, 1)
	assert.Equal(t, "frequent_flyer", links[1].table)
	assert.Equal(t, "ff_c_id_str", links[1].column)
}

func TestLinkParamsToColumns_UnqualifiedColumnStillFailsWhenAmbiguous(t *testing.T) {
	oldFn := relationHasColumnFn
	t.Cleanup(func() { relationHasColumnFn = oldFn })
	relationHasColumnFn = func(tgt paramTarget, col string) bool {
		return col == "id"
	}

	_, err := linkParamsToColumns("SELECT * FROM t1, t2 WHERE id = $1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no param/column links found")
}

func TestCountQueryParams_IncludesUnlinkedLimitParam(t *testing.T) {
	count, err := countQueryParams("SELECT old_text FROM text WHERE old_id = $1 LIMIT $2")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

// ---------- buildValueTuples ----------

func TestBuildValueTuples_CrossProduct(t *testing.T) {
	in := [][]string{
		{"a", "b"},
		{"x", "y"},
	}
	out := buildValueTuples(in)
	// 2x2 = 4 tuples, well under the cap (10).
	assert.Len(t, out, 4)
}

func TestBuildValueTuples_PositionalFallback(t *testing.T) {
	// Force total > cap of 10: 4 * 4 = 16.
	in := [][]string{
		{"a", "b", "c", "d"},
		{"w", "x", "y", "z"},
	}
	out := buildValueTuples(in)
	// Positional fallback produces exactly COVERING_INDEX_MAX_EXPLAIN_PER_QUERY tuples.
	assert.Len(t, out, queryissue.COVERING_INDEX_MAX_EXPLAIN_PER_QUERY)
	for _, tup := range out {
		assert.Len(t, tup, 2)
	}
}

func TestBuildValueTuples_Empty(t *testing.T) {
	assert.Nil(t, buildValueTuples(nil))
	assert.Nil(t, buildValueTuples([][]string{{}}))
}

func TestDefaultParamValueForType(t *testing.T) {
	assert.Equal(t, "false", defaultParamValueForType("boolean"))
	assert.Equal(t, "2024-01-01", defaultParamValueForType("date"))
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", defaultParamValueForType("uuid"))
	assert.Equal(t, "1", defaultParamValueForType("bigint"))
}

// ---------- normalizeTypedLiteralParams ----------
//
// pg_stat_statements rewrites typed-prefix literals like `timestamptz '...'`
// into the invalid form `timestamptz $N`. These tests pin down the cast
// rewrite that allows such queries to PREPARE/EXECUTE downstream.

func TestNormalizeTypedLiteralParams_SingleWordTimestamptz(t *testing.T) {
	in := "SELECT count(*) FROM t WHERE created_at >= timestamptz $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT count(*) FROM t WHERE created_at >= $1::timestamptz", out)
}

func TestNormalizeTypedLiteralParams_Interval(t *testing.T) {
	in := "SELECT * FROM t WHERE created_at < now() - interval $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT * FROM t WHERE created_at < now() - $1::interval", out)
}

func TestNormalizeTypedLiteralParams_Date(t *testing.T) {
	in := "SELECT id FROM t WHERE due >= date $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT id FROM t WHERE due >= $1::date", out)
}

func TestNormalizeTypedLiteralParams_TimestampWithTimeZone(t *testing.T) {
	in := "SELECT id FROM t WHERE created_at = TIMESTAMP WITH TIME ZONE $1 LIMIT $2"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT id FROM t WHERE created_at = $1::TIMESTAMP WITH TIME ZONE LIMIT $2", out)
}

func TestNormalizeTypedLiteralParams_TimestampWithoutTimeZone(t *testing.T) {
	in := "SELECT id FROM t WHERE created_at = TIMESTAMP WITHOUT TIME ZONE $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT id FROM t WHERE created_at = $1::TIMESTAMP WITHOUT TIME ZONE", out)
}

func TestNormalizeTypedLiteralParams_TimestampWithPrecisionAndTZ(t *testing.T) {
	in := "SELECT 1 WHERE x = TIMESTAMP(3) WITH TIME ZONE $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT 1 WHERE x = $1::TIMESTAMP(3) WITH TIME ZONE", out)
}

func TestNormalizeTypedLiteralParams_NumericWithModifier(t *testing.T) {
	in := "SELECT 1 WHERE numeric(10,2) $1 > 1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT 1 WHERE $1::numeric(10,2) > 1", out)
}

func TestNormalizeTypedLiteralParams_VarcharWithLength(t *testing.T) {
	in := "SELECT 1 WHERE varchar(10) $1 = 'bar'"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT 1 WHERE $1::varchar(10) = 'bar'", out)
}

func TestNormalizeTypedLiteralParams_CharacterVarying(t *testing.T) {
	in := "SELECT 1 WHERE x = character varying $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT 1 WHERE x = $1::character varying", out)
}

func TestNormalizeTypedLiteralParams_CharacterVaryingWithLength(t *testing.T) {
	in := "SELECT 1 WHERE x = character varying(20) $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT 1 WHERE x = $1::character varying(20)", out)
}

func TestNormalizeTypedLiteralParams_BitVarying(t *testing.T) {
	in := "SELECT 1 WHERE bits = bit varying $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT 1 WHERE bits = $1::bit varying", out)
}

func TestNormalizeTypedLiteralParams_DoublePrecision(t *testing.T) {
	in := "SELECT 1 WHERE x > double precision $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT 1 WHERE x > $1::double precision", out)
}

func TestNormalizeTypedLiteralParams_PgCatalogQualified(t *testing.T) {
	in := "SELECT 1 WHERE x = pg_catalog.timestamptz $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT 1 WHERE x = $1::pg_catalog.timestamptz", out)
}

func TestNormalizeTypedLiteralParams_MultipleOccurrences(t *testing.T) {
	in := "SELECT * FROM t WHERE a = timestamptz $1 AND b = interval $2 AND c = date $3"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t,
		"SELECT * FROM t WHERE a = $1::timestamptz AND b = $2::interval AND c = $3::date",
		out)
}

func TestNormalizeTypedLiteralParams_PreservesOriginalCasing(t *testing.T) {
	in := "SELECT 1 WHERE x = TIMESTAMPTZ $1"
	out := normalizeTypedLiteralParams(in)
	assert.Equal(t, "SELECT 1 WHERE x = $1::TIMESTAMPTZ", out)
}

func TestNormalizeTypedLiteralParams_Idempotent(t *testing.T) {
	in := "SELECT 1 WHERE x = $1::timestamptz"
	assert.Equal(t, in, normalizeTypedLiteralParams(in))
}

func TestNormalizeTypedLiteralParams_NoMatchOnCastForm(t *testing.T) {
	// $N::timestamptz is already valid; never rewrite to garbage.
	in := "SELECT 1 WHERE created_at >= $1::timestamptz AND status = $2"
	assert.Equal(t, in, normalizeTypedLiteralParams(in))
}

func TestNormalizeTypedLiteralParams_NoMatchOnRegularColumns(t *testing.T) {
	// A column that happens to have a type-keyword name must not be rewritten;
	// here `time` is a real column, not a typed-literal prefix.
	in := "SELECT id, time FROM events WHERE id = $1"
	assert.Equal(t, in, normalizeTypedLiteralParams(in))
}

func TestNormalizeTypedLiteralParams_LeavesUntypedParamsAlone(t *testing.T) {
	in := "SELECT * FROM t WHERE id = $1 AND name = $2 LIMIT $3"
	assert.Equal(t, in, normalizeTypedLiteralParams(in))
}

// Round-trip: a normalized query must parse cleanly via the same code path
// the rest of the pipeline uses (linkParamsToColumns / countQueryParams).
func TestNormalizeTypedLiteralParams_ParseableAfterRewrite(t *testing.T) {
	in := "SELECT title FROM catalog_items WHERE created_at >= timestamptz $1 AND deleted_at < interval $2 LIMIT $3"
	out := normalizeTypedLiteralParams(in)

	count, err := countQueryParams(out)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// linkParamsToColumns must successfully link the rewritten WHERE columns.
	links, err := linkParamsToColumns(out)
	require.NoError(t, err)
	assert.Equal(t, "created_at", links[1].column)
	assert.Equal(t, "deleted_at", links[2].column)
}

// ---------- walkPlanForIndexScans ----------

func TestWalkPlanForIndexScans_SingleIndexScan(t *testing.T) {
	meta := &indexMeta{
		keyColumns: map[indexKey][]string{
			{"public", "idx_users_email"}: {"email"},
		},
		indexTable: map[indexKey]struct{ schema, table string }{
			{"public", "idx_users_email"}: {"public", "users"},
		},
		columnAvgWidth: map[string]int{
			"public.users.id":    4,
			"public.users.name":  16,
			"public.users.email": 32,
		},
	}
	planJSON := []byte(`[{"Plan":{"Node Type":"Index Scan","Schema":"public","Relation Name":"users","Index Name":"idx_users_email","Output":["users.id","users.name","users.email"]}}]`)
	perIdx := make(map[indexKey]*indexCandidate)
	referencedCols := map[string]map[string]bool{
		"public.users": {"id": true, "name": true, "email": true},
	}
	walkPlanForIndexScans(planJSON, meta, perIdx, highImpactQuery{text: "q", calls: 10, totalExecMs: 1000}, referencedCols)
	require.Len(t, perIdx, 1)
	cand := perIdx[indexKey{"public", "idx_users_email"}]
	require.NotNil(t, cand)
	assert.ElementsMatch(t, []string{"id", "name"}, keys(cand.missingCols))
}

func TestLinkParamsToColumns_PreservesQuotedIdentifierCase(t *testing.T) {
	q := `SELECT "Email" FROM public."Users" WHERE "Id" = $1`
	links, err := linkParamsToColumns(q)
	require.NoError(t, err)
	require.Contains(t, links, 1)
	assert.Equal(t, "public", links[1].schema)
	assert.Equal(t, "Users", links[1].table)
	assert.Equal(t, "Id", links[1].column)
}

func TestCollectReferencedColumnsForQuery_PreservesQuotedIdentifierCase(t *testing.T) {
	q := `SELECT "Email", "Name" FROM public."Users" WHERE "Id" = $1`
	cols, err := collectReferencedColumnsForQuery(q)
	require.NoError(t, err)
	rel := cols["public.Users"]
	require.NotNil(t, rel, "expected relation key public.Users, got: %v", cols)
	assert.True(t, rel["Email"])
	assert.True(t, rel["Name"])
	assert.True(t, rel["Id"])
	// The lowered "users"/"email" must NOT be present; that would resurrect the bug.
	assert.Empty(t, cols["public.users"])
	assert.False(t, rel["email"])
}

func TestWalkPlanForIndexScans_PreservesQuotedIdentifierCase(t *testing.T) {
	meta := &indexMeta{
		keyColumns: map[indexKey][]string{
			{"public", "idx_users_Id"}: {"Id"},
		},
		indexTable: map[indexKey]struct{ schema, table string }{
			{"public", "idx_users_Id"}: {"public", "Users"},
		},
		columnAvgWidth: map[string]int{
			"public.Users.Email": 32,
			"public.Users.Id":    4,
		},
	}
	planJSON := []byte(`[{"Plan":{"Node Type":"Index Scan","Schema":"public","Relation Name":"Users","Index Name":"idx_users_Id"}}]`)
	referencedCols := map[string]map[string]bool{
		"public.Users": {"Id": true, "Email": true},
	}
	perIdx := make(map[indexKey]*indexCandidate)
	walkPlanForIndexScans(planJSON, meta, perIdx, highImpactQuery{text: "q", calls: 10, totalExecMs: 1000}, referencedCols)
	cand := perIdx[indexKey{"public", "idx_users_Id"}]
	require.NotNil(t, cand)
	assert.ElementsMatch(t, []string{"Email"}, keys(cand.missingCols))
}

func TestWalkPlanForIndexScans_UsesASTColumnsNotVerboseOutput(t *testing.T) {
	meta := &indexMeta{
		keyColumns: map[indexKey][]string{
			{"public", "index_status_stats_on_status_id"}: {"status_id"},
		},
		columnAvgWidth: map[string]int{
			"public.status_stats.reblogs_count":    8,
			"public.status_stats.favourites_count": 8,
			"public.status_stats.created_at":       8,
			"public.status_stats.updated_at":       8,
		},
	}
	planJSON := []byte(`[{"Plan":{"Node Type":"Index Scan","Schema":"public","Relation Name":"status_stats","Index Name":"index_status_stats_on_status_id","Output":["ss.id","ss.status_id","ss.replies_count","ss.reblogs_count","ss.favourites_count","ss.created_at","ss.updated_at"]}}]`)
	referencedCols := map[string]map[string]bool{
		"public.status_stats": {"status_id": true, "reblogs_count": true},
	}
	perIdx := make(map[indexKey]*indexCandidate)
	walkPlanForIndexScans(planJSON, meta, perIdx, highImpactQuery{text: "q", calls: 10, totalExecMs: 1000}, referencedCols)
	cand := perIdx[indexKey{"public", "index_status_stats_on_status_id"}]
	require.NotNil(t, cand)
	assert.ElementsMatch(t, []string{"reblogs_count"}, keys(cand.missingCols))
}

func TestWalkPlanForIndexScans_IgnoresIndexOnlyScan(t *testing.T) {
	meta := &indexMeta{
		keyColumns: map[indexKey][]string{{"public", "idx"}: {"a"}},
	}
	planJSON := []byte(`[{"Plan":{"Node Type":"Index Only Scan","Schema":"public","Relation Name":"t","Index Name":"idx","Output":["t.a","t.b"]}}]`)
	perIdx := make(map[indexKey]*indexCandidate)
	walkPlanForIndexScans(planJSON, meta, perIdx, highImpactQuery{}, map[string]map[string]bool{"public.t": {"b": true}})
	assert.Empty(t, perIdx)
}

func TestWalkPlanForIndexScans_NestedPlansAndMerging(t *testing.T) {
	meta := &indexMeta{
		keyColumns: map[indexKey][]string{{"public", "idx_a"}: {"x"}},
	}
	planJSON := []byte(`[{"Plan":{"Node Type":"Nested Loop","Plans":[{"Node Type":"Index Scan","Schema":"public","Relation Name":"t","Index Name":"idx_a","Output":["t.a","t.b","t.x"]}]}}]`)
	perIdx := make(map[indexKey]*indexCandidate)
	referencedCols := map[string]map[string]bool{"public.t": {"a": true, "b": true, "x": true}}
	// Call twice with different queries - contributing should merge, total should add.
	walkPlanForIndexScans(planJSON, meta, perIdx, highImpactQuery{text: "q1", calls: 5, totalExecMs: 500}, referencedCols)
	walkPlanForIndexScans(planJSON, meta, perIdx, highImpactQuery{text: "q2", calls: 7, totalExecMs: 700}, referencedCols)
	cand := perIdx[indexKey{"public", "idx_a"}]
	require.NotNil(t, cand)
	usage := cand.missingCols["a"]
	require.NotNil(t, usage)
	assert.Equal(t, int64(12), usage.totalCalls)
	assert.InDelta(t, 1200.0, usage.totalExecMs, 0.001)
	assert.Len(t, usage.contributing, 2)
}

func TestWalkPlanForIndexScans_DedupesSameQueryAcrossSampledPlans(t *testing.T) {
	meta := &indexMeta{
		keyColumns: map[indexKey][]string{{"public", "idx_a"}: {"x"}},
		indexTable: map[indexKey]struct{ schema, table string }{
			{"public", "idx_a"}: {"public", "t"},
		},
		columnAvgWidth: map[string]int{"public.t.a": 4},
	}
	planJSON := []byte(`[{"Plan":{"Node Type":"Index Scan","Relation Name":"t","Index Name":"idx_a"}}]`)
	perIdx := make(map[indexKey]*indexCandidate)
	referencedCols := map[string]map[string]bool{"public.t": {"a": true, "x": true}}
	q := highImpactQuery{text: "q", calls: 5, totalExecMs: 500}

	walkPlanForIndexScans(planJSON, meta, perIdx, q, referencedCols)
	walkPlanForIndexScans(planJSON, meta, perIdx, q, referencedCols)

	cand := perIdx[indexKey{"public", "idx_a"}]
	require.NotNil(t, cand)
	usage := cand.missingCols["a"]
	require.NotNil(t, usage)
	assert.Equal(t, int64(5), usage.totalCalls)
	assert.InDelta(t, 500.0, usage.totalExecMs, 0.001)
	assert.Len(t, usage.contributing, 1)
}

// ---------- buildRecommendations ----------

// Caps were removed from the pipeline: every surviving missing column is now
// admitted as an INCLUDE candidate, and IncludeColumns is sorted alphabetically
// for deterministic output. The tests below pin those guarantees.

func TestBuildRecommendations_AdmitsAllColumnsRegardlessOfWidth(t *testing.T) {
	// A previously "oversized" column (single column wider than the old
	// MAX_INCLUDE_WIDTH_BYTES/2) must now flow through unfiltered.
	cand := &indexCandidate{
		key:    indexKey{"public", "idx"},
		schema: "public",
		table:  "t",
		missingCols: map[string]*colUsage{
			"huge": {column: "huge", avgWidth: 4096, totalExecMs: 100,
				contributing: []contributingQuery{{text: "q", calls: 1, totalExecMs: 100}}},
			"tiny": {column: "tiny", avgWidth: 4, totalExecMs: 50,
				contributing: []contributingQuery{{text: "q", calls: 1, totalExecMs: 50}}},
		},
	}
	out := buildRecommendations(map[indexKey]*indexCandidate{cand.key: cand})
	rec := out[queryissue.CoveringIndexRecommendationKey("public", "idx")]
	require.NotNil(t, rec)
	assert.Equal(t, []string{"huge", "tiny"}, rec.IncludeColumns)
	assert.Empty(t, rec.DroppedColumns, "no cap-based drops should be produced")
}

func TestBuildRecommendations_AdmitsManyColumnsRegardlessOfCount(t *testing.T) {
	// Beyond the old count cap (8): all admitted.
	missing := map[string]*colUsage{}
	for i := 0; i < 12; i++ {
		name := "c" + string(rune('a'+i))
		missing[name] = &colUsage{
			column: name, avgWidth: 4, totalExecMs: float64(10 - i),
			contributing: []contributingQuery{{text: "q", calls: 1, totalExecMs: float64(10 - i)}},
		}
	}
	cand := &indexCandidate{key: indexKey{"public", "idx"}, schema: "public", table: "t", missingCols: missing}
	out := buildRecommendations(map[indexKey]*indexCandidate{cand.key: cand})
	rec := out[queryissue.CoveringIndexRecommendationKey("public", "idx")]
	require.NotNil(t, rec)
	assert.Len(t, rec.IncludeColumns, 12)
	for _, d := range rec.DroppedColumns {
		assert.NotEqual(t, "count_cap", d.Reason)
		assert.NotEqual(t, "width_cap", d.Reason)
		assert.NotEqual(t, "oversized", d.Reason)
	}
}

func TestBuildRecommendations_SortsIncludeColumnsAlphabetically(t *testing.T) {
	cand := &indexCandidate{
		key: indexKey{"public", "idx"}, schema: "public", table: "t",
		missingCols: map[string]*colUsage{
			"wide":   {column: "wide", avgWidth: 200, totalExecMs: 100, contributing: []contributingQuery{{text: "q", calls: 1, totalExecMs: 100}}},
			"narrow": {column: "narrow", avgWidth: 4, totalExecMs: 100, contributing: []contributingQuery{{text: "q", calls: 1, totalExecMs: 100}}},
		},
	}
	out := buildRecommendations(map[indexKey]*indexCandidate{cand.key: cand})
	rec := out[queryissue.CoveringIndexRecommendationKey("public", "idx")]
	require.NotNil(t, rec)
	assert.Equal(t, []string{"narrow", "wide"}, rec.IncludeColumns)
}

func TestBuildRecommendations_TotalsDedupedAcrossColumns(t *testing.T) {
	cand := &indexCandidate{
		key: indexKey{"public", "idx"}, schema: "public", table: "t",
		missingCols: map[string]*colUsage{
			"a": {column: "a", avgWidth: 4, totalExecMs: 100, totalCalls: 10,
				contributing: []contributingQuery{{text: "q", calls: 10, totalExecMs: 100}}},
			"b": {column: "b", avgWidth: 4, totalExecMs: 100, totalCalls: 10,
				contributing: []contributingQuery{{text: "q", calls: 10, totalExecMs: 100}}},
		},
	}

	out := buildRecommendations(map[indexKey]*indexCandidate{cand.key: cand})
	rec := out[queryissue.CoveringIndexRecommendationKey("public", "idx")]
	require.NotNil(t, rec)
	assert.ElementsMatch(t, []string{"a", "b"}, rec.IncludeColumns)
	assert.Equal(t, int64(10), rec.TotalCalls)
	assert.InDelta(t, 100.0, rec.TotalExecTimeMs, 0.001)
	require.Len(t, rec.RelevantQueries, 1)
	assert.Equal(t, int64(10), rec.RelevantQueries[0].Calls)
}

func TestBuildRecommendations_PreservesWriteHeavyDrops(t *testing.T) {
	// Pre-cap drops recorded by the ratio filter (write_heavy) must still
	// surface in the final DroppedColumns list.
	cand := &indexCandidate{
		key: indexKey{"public", "idx"}, schema: "public", table: "t",
		missingCols: map[string]*colUsage{
			"a": {column: "a", avgWidth: 4, totalExecMs: 100,
				contributing: []contributingQuery{{text: "q", calls: 1, totalExecMs: 100}}},
		},
		droppedList: []droppedColumn{{name: "hot", reason: "write_heavy"}},
	}
	out := buildRecommendations(map[indexKey]*indexCandidate{cand.key: cand})
	rec := out[queryissue.CoveringIndexRecommendationKey("public", "idx")]
	require.NotNil(t, rec)
	require.Len(t, rec.DroppedColumns, 1)
	assert.Equal(t, queryissue.DroppedColumn{Name: "hot", Reason: "write_heavy"}, rec.DroppedColumns[0])
}

func TestBuildRecommendations_EmptyResult(t *testing.T) {
	assert.Empty(t, buildRecommendations(nil))
	assert.Empty(t, buildRecommendations(map[indexKey]*indexCandidate{}))
}

// ---------- applyRatioFilterToCandidate ----------

func TestFilterPerIndexCandidatesByUpdateReadRatio_DropsWriteHeavy(t *testing.T) {
	cand := &indexCandidate{
		key: indexKey{"public", "idx"}, schema: "public", table: "t",
		missingCols: map[string]*colUsage{
			"hot":  {column: "hot", totalExecMs: 100},
			"cold": {column: "cold", totalExecMs: 50},
		},
	}
	metrics := map[string]columnOpsMetric{
		"hot":  {columnName: "hot", updateWriteOps: 80, readOps: 10, filterOps: 10}, // 80% writes
		"cold": {columnName: "cold", updateWriteOps: 1, readOps: 100, filterOps: 0},
	}
	out := applyRatioFilterToCandidate(cand, metrics)
	require.NotNil(t, out)
	assert.Len(t, out.missingCols, 1)
	assert.Contains(t, out.missingCols, "cold")
	require.Len(t, out.droppedList, 1)
	assert.Equal(t, "hot", out.droppedList[0].name)
	assert.Equal(t, "write_heavy", out.droppedList[0].reason)
}

func TestFilterPerIndexCandidatesByUpdateReadRatio_AllDroppedReturnsNil(t *testing.T) {
	cand := &indexCandidate{
		key: indexKey{"public", "idx"}, schema: "public", table: "t",
		missingCols: map[string]*colUsage{
			"x": {column: "x"},
		},
	}
	metrics := map[string]columnOpsMetric{
		"x": {columnName: "x", updateWriteOps: 100, readOps: 0, filterOps: 0},
	}
	assert.Nil(t, applyRatioFilterToCandidate(cand, metrics))
}

func TestFilterPerIndexCandidatesByUpdateReadRatio_KeepsUnknownCols(t *testing.T) {
	// Columns with no metrics (no workload data) should be kept by default.
	cand := &indexCandidate{
		key: indexKey{"public", "idx"}, schema: "public", table: "t",
		missingCols: map[string]*colUsage{
			"mystery": {column: "mystery"},
		},
	}
	out := applyRatioFilterToCandidate(cand, map[string]columnOpsMetric{})
	require.NotNil(t, out)
	assert.Contains(t, out.missingCols, "mystery")
}

// ---------- extractPartialPredicateConstraints ----------
//
// Partial-index predicates are deparsed from pg_get_expr(indpred, indrelid).
// pg_get_expr typically wraps things in parens and emits explicit type casts
// (e.g. `'A'::text`). The extractor walks the AST and harvests `=`, `IN`,
// `= ANY (ARRAY[...])`, and `IS NULL` shapes, AND-conjoined.

func TestExtractPartialPredicateConstraints_Equality(t *testing.T) {
	eq, isNull := extractPartialPredicateConstraints("(status = 'ACTIVE'::text)", "public", "foo")
	assert.Equal(t, []string{"ACTIVE"}, eq["public.foo.status"])
	assert.Empty(t, isNull)
}

func TestExtractPartialPredicateConstraints_EqualityIntLiteral(t *testing.T) {
	eq, _ := extractPartialPredicateConstraints("(priority = 1)", "public", "tasks")
	assert.Equal(t, []string{"1"}, eq["public.tasks.priority"])
}

func TestExtractPartialPredicateConstraints_EqualityBoolLiteral(t *testing.T) {
	eq, _ := extractPartialPredicateConstraints("(active = true)", "public", "users")
	assert.Equal(t, []string{"true"}, eq["public.users.active"])
}

func TestExtractPartialPredicateConstraints_LiteralOnLeftSide(t *testing.T) {
	eq, _ := extractPartialPredicateConstraints("('ACTIVE'::text = status)", "public", "foo")
	assert.Equal(t, []string{"ACTIVE"}, eq["public.foo.status"])
}

func TestExtractPartialPredicateConstraints_InList(t *testing.T) {
	eq, _ := extractPartialPredicateConstraints("status IN ('A', 'B', 'C')", "public", "foo")
	assert.Equal(t, []string{"A", "B", "C"}, eq["public.foo.status"])
}

// pg_get_expr typically emits IN as `= ANY (ARRAY[...])` because the catalog
// stores ScalarArrayOpExpr, not the original IN syntax.
func TestExtractPartialPredicateConstraints_AnyArrayForm(t *testing.T) {
	eq, _ := extractPartialPredicateConstraints(
		"(status = ANY (ARRAY['A'::text, 'B'::text]))", "public", "foo")
	assert.Equal(t, []string{"A", "B"}, eq["public.foo.status"])
}

func TestExtractPartialPredicateConstraints_IsNull(t *testing.T) {
	eq, isNull := extractPartialPredicateConstraints("(deleted_at IS NULL)", "public", "foo")
	assert.Empty(t, eq)
	assert.True(t, isNull["public.foo.deleted_at"])
}

func TestExtractPartialPredicateConstraints_IsNotNullSkipped(t *testing.T) {
	// IS NOT NULL: random samples are already NON-NULL so no synthesis needed.
	eq, isNull := extractPartialPredicateConstraints("(deleted_at IS NOT NULL)", "public", "foo")
	assert.Empty(t, eq)
	assert.Empty(t, isNull)
}

func TestExtractPartialPredicateConstraints_AndOfMultiple(t *testing.T) {
	eq, isNull := extractPartialPredicateConstraints(
		"((status = 'ACTIVE'::text) AND (deleted_at IS NULL) AND (priority = 5))",
		"public", "foo")
	assert.Equal(t, []string{"ACTIVE"}, eq["public.foo.status"])
	assert.Equal(t, []string{"5"}, eq["public.foo.priority"])
	assert.True(t, isNull["public.foo.deleted_at"])
}

func TestExtractPartialPredicateConstraints_UnsupportedFunctionDropped(t *testing.T) {
	eq, isNull := extractPartialPredicateConstraints("(lower(name) = 'foo')", "public", "foo")
	assert.Empty(t, eq)
	assert.Empty(t, isNull)
}

func TestExtractPartialPredicateConstraints_UnsupportedRangeDropped(t *testing.T) {
	eq, isNull := extractPartialPredicateConstraints("(amount > 100)", "public", "foo")
	assert.Empty(t, eq)
	assert.Empty(t, isNull)
}

func TestExtractPartialPredicateConstraints_OrDropped(t *testing.T) {
	// OR is conservatively skipped: we cannot synthesize a value that satisfies
	// either branch without knowing which to pick.
	eq, isNull := extractPartialPredicateConstraints(
		"((status = 'A'::text) OR (status = 'B'::text))", "public", "foo")
	assert.Empty(t, eq)
	assert.Empty(t, isNull)
}

func TestExtractPartialPredicateConstraints_PreservesQuotedIdentifierCase(t *testing.T) {
	eq, isNull := extractPartialPredicateConstraints(
		`(("Status" = 'ACTIVE'::text) AND ("DeletedAt" IS NULL))`,
		"public", "Items")
	assert.Equal(t, []string{"ACTIVE"}, eq["public.Items.Status"])
	assert.True(t, isNull["public.Items.DeletedAt"])
	// The lowered keys must NOT be present.
	assert.Empty(t, eq["public.items.status"])
	assert.False(t, isNull["public.items.deletedat"])
}

func TestExtractPartialPredicateConstraints_EmptyText(t *testing.T) {
	eq, isNull := extractPartialPredicateConstraints("", "public", "foo")
	assert.Empty(t, eq)
	assert.Empty(t, isNull)
}

// ---------- augmentSamplesWithPartialConstraints ----------

func TestAugmentSamplesWithPartialConstraints_PrependsEqValues(t *testing.T) {
	meta := &indexMeta{
		partialEqValues: map[string][]string{
			"public.foo.status": {"ACTIVE"},
		},
		partialIsNull: map[string]bool{},
	}
	tgt := paramTarget{schema: "public", table: "foo", column: "status"}
	out := augmentSamplesWithPartialConstraints(meta, tgt, []string{"INACTIVE", "PENDING"})
	assert.Equal(t, []string{"ACTIVE", "INACTIVE", "PENDING"}, out)
}

func TestAugmentSamplesWithPartialConstraints_DedupesAgainstSamples(t *testing.T) {
	meta := &indexMeta{
		partialEqValues: map[string][]string{
			"public.foo.status": {"ACTIVE", "INACTIVE"},
		},
	}
	tgt := paramTarget{schema: "public", table: "foo", column: "status"}
	out := augmentSamplesWithPartialConstraints(meta, tgt, []string{"INACTIVE", "PENDING"})
	assert.Equal(t, []string{"ACTIVE", "INACTIVE", "PENDING"}, out)
}

func TestAugmentSamplesWithPartialConstraints_PrependsNullSentinel(t *testing.T) {
	meta := &indexMeta{
		partialEqValues: map[string][]string{},
		partialIsNull: map[string]bool{
			"public.foo.deleted_at": true,
		},
	}
	tgt := paramTarget{schema: "public", table: "foo", column: "deleted_at"}
	out := augmentSamplesWithPartialConstraints(meta, tgt, []string{"2024-01-01"})
	require.Len(t, out, 2)
	assert.Equal(t, nullParamSentinel, out[0])
	assert.Equal(t, "2024-01-01", out[1])
}

// When the query does not schema-qualify the relation, tgt.schema is "" and
// the strict map key won't match. The lookup must fall back to suffix-matching
// on ".<table>.<column>" so the augmentation still kicks in.
func TestAugmentSamplesWithPartialConstraints_FallsBackOnEmptyTargetSchema(t *testing.T) {
	meta := &indexMeta{
		partialEqValues: map[string][]string{
			"public.foo.status": {"ACTIVE"},
		},
		partialIsNull: map[string]bool{},
	}
	tgt := paramTarget{schema: "", table: "foo", column: "status"}
	out := augmentSamplesWithPartialConstraints(meta, tgt, []string{"INACTIVE"})
	assert.Equal(t, []string{"ACTIVE", "INACTIVE"}, out)
}

func TestAugmentSamplesWithPartialConstraints_AmbiguousSchemaSkipsAugmentation(t *testing.T) {
	meta := &indexMeta{
		partialEqValues: map[string][]string{
			"public.foo.status": {"ACTIVE"},
			"other.foo.status":  {"ARCHIVED"},
		},
	}
	tgt := paramTarget{schema: "", table: "foo", column: "status"}
	in := []string{"INACTIVE", "PENDING"}
	out := augmentSamplesWithPartialConstraints(meta, tgt, in)
	assert.Equal(t, in, out)
}

func TestAugmentSamplesWithPartialConstraints_NoOpWhenNoConstraint(t *testing.T) {
	meta := &indexMeta{
		partialEqValues: map[string][]string{},
		partialIsNull:   map[string]bool{},
	}
	tgt := paramTarget{schema: "public", table: "foo", column: "id"}
	in := []string{"1", "2"}
	out := augmentSamplesWithPartialConstraints(meta, tgt, in)
	assert.Equal(t, in, out)
}

// ---------- applyRedundantFilterToCandidates ----------

func TestApplyRedundantFilter_DropsMatchingIndex(t *testing.T) {
	keep := indexKey{schema: "public", name: "idx_keep"}
	drop := indexKey{schema: "public", name: "idx_drop"}
	in := map[indexKey]*indexCandidate{
		keep: {key: keep, schema: "public", table: "t"},
		drop: {key: drop, schema: "public", table: "t"},
	}
	redundant := []utils.RedundantIndexesInfo{
		{RedundantSchemaName: "public", RedundantTableName: "t", RedundantIndexName: "idx_drop"},
	}
	out := applyRedundantFilterToCandidates(in, redundant)
	require.Len(t, out, 1)
	assert.Contains(t, out, keep)
	assert.NotContains(t, out, drop)
}

func TestApplyRedundantFilter_CaseInsensitiveMatch(t *testing.T) {
	drop := indexKey{schema: "public", name: "idx_drop"}
	in := map[indexKey]*indexCandidate{
		drop: {key: drop, schema: "public", table: "t"},
	}
	redundant := []utils.RedundantIndexesInfo{
		{RedundantSchemaName: "Public", RedundantTableName: "T", RedundantIndexName: "Idx_Drop"},
	}
	out := applyRedundantFilterToCandidates(in, redundant)
	assert.Empty(t, out)
}

func TestApplyRedundantFilter_NoRedundantInputReturnsAll(t *testing.T) {
	a := indexKey{schema: "public", name: "idx_a"}
	b := indexKey{schema: "public", name: "idx_b"}
	in := map[indexKey]*indexCandidate{
		a: {key: a, schema: "public", table: "t"},
		b: {key: b, schema: "public", table: "t"},
	}
	out := applyRedundantFilterToCandidates(in, nil)
	assert.Len(t, out, 2)
	assert.Contains(t, out, a)
	assert.Contains(t, out, b)
}

func TestApplyRedundantFilter_EmptyCandidatesReturnsEmpty(t *testing.T) {
	out := applyRedundantFilterToCandidates(nil, []utils.RedundantIndexesInfo{
		{RedundantSchemaName: "public", RedundantIndexName: "x"},
	})
	assert.Empty(t, out)

	out = applyRedundantFilterToCandidates(map[indexKey]*indexCandidate{}, nil)
	assert.Empty(t, out)
}

// ---------- helper ----------

func keys(m map[string]*colUsage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
