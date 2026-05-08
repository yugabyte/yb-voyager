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

// Tests for the partial-index SQL fix and for partition-aware index
// resolution at walker time. The walker must roll per-partition child
// Index Scan matches up to the root parent index (sourced from
// pg_inherits) so the resulting candidate is keyed by the parent's
// CREATE INDEX -- which is what the DDL detector looks up.

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/sqltransformer"
)

// Partial indexes pass through the SQL fix correctly.
// AddIncludeColumnsToIndex should append INCLUDE(...) without disturbing the
// existing WHERE predicate that makes the index partial.
func TestPartialIndex_AddInclude_PreservesWhere(t *testing.T) {
	input := `CREATE INDEX idx_users_active_email ON public.users (email) WHERE active = true;`
	parseTree, err := queryparser.Parse(input)
	require.NoError(t, err)

	tx := sqltransformer.NewTransformer()
	modified, err := tx.AddIncludeColumnsToIndex(parseTree, []string{"full_name", "signup_at"})
	require.NoError(t, err)

	out, err := queryparser.Deparse(modified)
	require.NoError(t, err)

	assert.Contains(t, out, "INCLUDE (full_name, signup_at)", "INCLUDE clause must be added")
	assert.Contains(t, out, "WHERE active = true", "partial index WHERE must be preserved")
}

// Defense: works for partial UNIQUE indexes too.
func TestPartialIndex_AddInclude_UniqueWithWhere(t *testing.T) {
	input := `CREATE UNIQUE INDEX uniq_open_tickets ON support.tickets (requester_id) WHERE status = 'open';`
	parseTree, err := queryparser.Parse(input)
	require.NoError(t, err)

	tx := sqltransformer.NewTransformer()
	modified, err := tx.AddIncludeColumnsToIndex(parseTree, []string{"assignee_id"})
	require.NoError(t, err)

	out, err := queryparser.Deparse(modified)
	require.NoError(t, err)

	assert.Contains(t, out, "UNIQUE INDEX")
	assert.Contains(t, out, "INCLUDE (assignee_id)")
	assert.Contains(t, out, "WHERE status = 'open'")
}

// ---------- (m *indexMeta).rootIndex ----------
//
// Pin the resolver semantics in isolation from the walker:
//   - non-partitioned indexes resolve to themselves
//   - one-level / multi-level chains resolve to the topmost parent
//   - any cycle stops the walk and falls back to the input key (with a warn)

func TestRootIndex_NoParentReturnsSelf(t *testing.T) {
	m := &indexMeta{
		parentIndex: map[indexKey]indexKey{},
	}
	in := indexKey{schema: "public", name: "idx_orders_account"}
	assert.Equal(t, in, m.rootIndex(in))
}

func TestRootIndex_NilParentMapReturnsSelf(t *testing.T) {
	// Defensive: an indexMeta produced by callers that forgot to populate
	// parentIndex (older test fixtures, e.g.) must not panic and must
	// behave as if no partition link exists.
	m := &indexMeta{}
	in := indexKey{schema: "public", name: "idx_orders_account"}
	assert.Equal(t, in, m.rootIndex(in))
}

func TestRootIndex_OneLevelPartition(t *testing.T) {
	parent := indexKey{schema: "public", name: "idx_events_account"}
	child := indexKey{schema: "public", name: "events_2024_account_id_idx"}
	m := &indexMeta{
		parentIndex: map[indexKey]indexKey{
			child: parent,
		},
	}
	assert.Equal(t, parent, m.rootIndex(child))
	// Parent already at root.
	assert.Equal(t, parent, m.rootIndex(parent))
}

func TestRootIndex_MultiLevelPartition(t *testing.T) {
	root := indexKey{schema: "public", name: "idx_events_account"}
	mid := indexKey{schema: "public", name: "events_2025_account_id_idx"}
	leaf := indexKey{schema: "public", name: "events_2025_q1_account_id_idx"}
	m := &indexMeta{
		parentIndex: map[indexKey]indexKey{
			leaf: mid,
			mid:  root,
		},
	}
	assert.Equal(t, root, m.rootIndex(leaf))
	assert.Equal(t, root, m.rootIndex(mid))
	assert.Equal(t, root, m.rootIndex(root))
}

func TestRootIndex_CycleProtection(t *testing.T) {
	a := indexKey{schema: "public", name: "a"}
	b := indexKey{schema: "public", name: "b"}
	c := indexKey{schema: "public", name: "c"}
	// a -> b -> c -> a is impossible in real pg_inherits but we still
	// must not loop forever or panic if the catalog is corrupted.
	m := &indexMeta{
		parentIndex: map[indexKey]indexKey{
			a: b,
			b: c,
			c: a,
		},
	}
	got := m.rootIndex(a)
	// Cycle-safe contract: when a cycle is detected the helper returns the
	// input unchanged so the walker still emits a candidate (worst case:
	// keyed at the per-partition child, matching pre-fix behavior).
	assert.Equal(t, a, got)
}

// ---------- walker: partitioned Append plans ----------

// Realistic PG EXPLAIN JSON for a parent-partition scan: Append over per-
// partition Index Scans, each on a child relation with an auto-generated
// child-index name. This is the shape that produced the silent skip the
// resolver fix removes.
const partitionedAppendPlanJSON = `[{
  "Plan": {
    "Node Type": "Append",
    "Plans": [
      {"Node Type":"Index Scan","Schema":"public","Relation Name":"events_2024","Index Name":"events_2024_account_id_idx","Output":["events_2024.event_id","events_2024.account_id","events_2024.kind","events_2024.payload"]},
      {"Node Type":"Index Scan","Schema":"public","Relation Name":"events_2025","Index Name":"events_2025_account_id_idx","Output":["events_2025.event_id","events_2025.account_id","events_2025.kind","events_2025.payload"]}
    ]
  }
}]`

// partitionedEventsMeta returns an indexMeta that mirrors what loadIndexMeta
// produces for a RANGE-partitioned `events` table with one declarative
// parent index `idx_events_account` and two per-partition child indexes
// linked via pg_inherits.
func partitionedEventsMeta() *indexMeta {
	parentKey := indexKey{schema: "public", name: "idx_events_account"}
	child2024 := indexKey{schema: "public", name: "events_2024_account_id_idx"}
	child2025 := indexKey{schema: "public", name: "events_2025_account_id_idx"}
	return &indexMeta{
		keyColumns: map[indexKey][]string{
			parentKey: {"account_id"},
			child2024: {"account_id"},
			child2025: {"account_id"},
		},
		indexTable: map[indexKey]struct{ schema, table string }{
			parentKey: {"public", "events"},
			child2024: {"public", "events_2024"},
			child2025: {"public", "events_2025"},
		},
		columnAvgWidth: map[string]int{
			"public.events.kind":    16,
			"public.events.payload": 64,
		},
		parentIndex: map[indexKey]indexKey{
			child2024: parentKey,
			child2025: parentKey,
		},
	}
}

// With pg_inherits links populated, both per-partition Index Scan matches
// fold up to a single candidate keyed by the parent CREATE INDEX. The
// candidate's table is the partitioned parent (`events`), so referenced-
// columns lookup -- which the AST collector keys under the parent table
// only -- finally hits, and the missing columns include the SELECT-list
// columns that aren't in the parent index's key.
func TestWalkPlanForIndexScans_PartitionedAppendBuildsCandidateOnParent(t *testing.T) {
	meta := partitionedEventsMeta()
	parentKey := indexKey{schema: "public", name: "idx_events_account"}

	perIdx := make(map[indexKey]*indexCandidate)
	// AST-collected columns are keyed by the partitioned PARENT relation
	// (queries say `FROM events`, not `FROM events_2024`); referenced-cols
	// for the per-partition children are intentionally absent to prove the
	// resolver -- not a fallback lookup -- is what makes this work.
	referencedCols := map[string]map[string]bool{
		"public.events": {"account_id": true, "kind": true, "payload": true},
	}
	walkPlanForIndexScans([]byte(partitionedAppendPlanJSON), meta, perIdx,
		highImpactQuery{text: "SELECT kind, payload FROM events WHERE account_id = $1", calls: 100, totalExecMs: 5000},
		referencedCols)

	// The candidate is now keyed by the PARENT index, not by either child.
	require.Contains(t, perIdx, parentKey, "candidate must be keyed by the partitioned parent index")
	assert.NotContains(t, perIdx, indexKey{"public", "events_2024_account_id_idx"},
		"per-partition child must not own a separate candidate after rollup")
	assert.NotContains(t, perIdx, indexKey{"public", "events_2025_account_id_idx"},
		"per-partition child must not own a separate candidate after rollup")

	cand := perIdx[parentKey]
	require.NotNil(t, cand)
	assert.Equal(t, "public", cand.schema)
	assert.Equal(t, "events", cand.table, "candidate table must be the partitioned parent, not a per-partition child")
	assert.ElementsMatch(t, []string{"kind", "payload"}, keys(cand.missingCols),
		"missing cols should be SELECT-list cols absent from the parent index keys")

	// Final pipeline produces a recommendation under the parent's key, which
	// is exactly what the DDL detector at detectors_ddl.go looks up.
	finalRecs := buildRecommendations(perIdx)
	parentRecKey := queryissue.CoveringIndexRecommendationKey("public", "idx_events_account")
	require.Contains(t, finalRecs, parentRecKey,
		"recommendation must be emitted under the parent index name; that is what the DDL detector matches")
	rec := finalRecs[parentRecKey]
	require.NotNil(t, rec)
	assert.ElementsMatch(t, []string{"kind", "payload"}, rec.IncludeColumns)
}

// The same query producing two child-scan matches inside one Append must
// not double-count TotalCalls / TotalExecTimeMs at the parent. addMissingCol
// already dedupes contributing entries by query text, so the rollup
// preserves accurate workload-impact stats.
func TestWalkPlanForIndexScans_PartitionedAppendDedupesAcrossPartitions(t *testing.T) {
	meta := partitionedEventsMeta()
	parentKey := indexKey{schema: "public", name: "idx_events_account"}

	perIdx := make(map[indexKey]*indexCandidate)
	referencedCols := map[string]map[string]bool{
		"public.events": {"account_id": true, "kind": true, "payload": true},
	}
	q := highImpactQuery{text: "SELECT kind, payload FROM events WHERE account_id = $1", calls: 100, totalExecMs: 5000}
	walkPlanForIndexScans([]byte(partitionedAppendPlanJSON), meta, perIdx, q, referencedCols)

	cand := perIdx[parentKey]
	require.NotNil(t, cand)

	for _, col := range []string{"kind", "payload"} {
		usage := cand.missingCols[col]
		require.NotNil(t, usage, "missing col %q should be present", col)
		assert.Len(t, usage.contributing, 1,
			"the same query reaching two child partitions in one Append must contribute exactly once at the parent for col %q", col)
		assert.Equal(t, q.calls, usage.totalCalls,
			"totalCalls for col %q must equal the query's call count, not 2x", col)
		assert.InDelta(t, q.totalExecMs, usage.totalExecMs, 0.001,
			"totalExecMs for col %q must equal the query's total exec time, not 2x", col)
	}
}
