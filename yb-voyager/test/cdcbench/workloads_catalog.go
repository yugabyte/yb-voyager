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

package cdcbench

import (
	"embed"
	"fmt"
)

//go:embed testdata
var testdataFS embed.FS

// testdataWorkload builds a Workload whose SQL lives in testdata/<name>/
// ({schema,seed,dml}.sql). The workload name, its testdata directory, and the
// benchmark sub-name used to run it (`-bench 'CDCIngest/<name>'`) are all the
// same string by construction.
func testdataWorkload(name string, tableList []string, expectedEvents int, expectConflicts bool) Workload {
	read := func(file string) string {
		raw, err := testdataFS.ReadFile("testdata/" + name + "/" + file)
		if err != nil {
			panic(fmt.Sprintf("cdcbench: workload %q: %v", name, err))
		}
		return string(raw)
	}
	return Workload{
		Name:            name,
		SchemaSQL:       read("schema.sql"),
		SeedSQL:         read("seed.sql"),
		DMLSQL:          read("dml.sql"),
		TableList:       tableList,
		ExpectedEvents:  expectedEvents,
		ExpectConflicts: expectConflicts,
	}
}

// The workload catalog. Names describe the workload's construction, not its
// current detection outcome; where a workload asserts KNOWN false positives
// of current semantics, the comment says so — the assertion flips when the
// semantics are fixed.
func init() {
	// ---- oltp: realistic customer patterns, measured for throughput ----

	// The most common OLTP lifecycle: insert once, then mutate status/payload
	// columns repeatedly; the unique column never changes.
	Register(testdataWorkload("oltp-order-lifecycle", []string{"orders"}, 20_000, false))

	// Auth/session churn: unique tokens inserted and deleted, never reused.
	Register(testdataWorkload("oltp-session-churn", []string{"sessions"}, 19_900, false))

	// Interleaved 60% insert / 30% update / 10% delete on a unique-key table.
	Register(testdataWorkload("oltp-mixed-crud", []string{"uk_table"}, 20_000, false))

	// TPC-C-ish checkout across four co-streamed tables: orders + line items +
	// hot inventory + payments (two of the tables carry unique keys).
	Register(testdataWorkload("oltp-checkout-multi-table",
		[]string{"co_orders", "co_items", "co_inventory", "co_payments"}, 20_000, false))

	// One UK table receiving 20% of traffic co-streamed with a plain table
	// receiving 80%: measures the collateral damage of UK conflict checks on
	// unrelated tables sharing the single ingest thread.
	Register(testdataWorkload("oltp-mixed-uk-and-plain-tables",
		[]string{"accounts", "audit_log"}, 20_000, false))

	// Double-entry ledger: append-only journal (unique entry numbers) + hot
	// balance rows without unique keys.
	Register(testdataWorkload("oltp-ledger", []string{"journal", "balances"}, 20_000, false))

	// Job queue / outbox churn: insert -> claim -> complete -> delete with
	// short row lifetimes and never-reused unique keys.
	Register(testdataWorkload("oltp-job-queue", []string{"jobs"}, 20_000, false))

	// Skewed access: 80% of updates hit 100 hot rows (same-PK exclusion path),
	// 20% change cold rows' unique values.
	Register(testdataWorkload("oltp-skewed-hot-rows", []string{"uk_table"}, 20_000, false))

	// Append-only state-machine transition log (the shape popular state-machine
	// libraries generate): UNIQUE(parent, sort_key) for ordering plus a partial
	// unique index enforcing one most_recent transition per parent. Every step
	// demotes the current transition and appends its successor — REAL
	// conflicts on the partial index.
	Register(testdataWorkload("oltp-state-machine-transitions", []string{"payment_transitions"}, 20_000, true))

	// CUSTOMER PATTERN (reported in the field): records are INSERTed as drafts
	// with NULL unique columns; the unique value is filled by a later UPDATE.
	// SQL-semantically there are ZERO conflicts (NULLs are distinct in unique
	// indexes), but detection currently treats nil==nil as a conflict, so
	// concurrent drafts of DIFFERENT rows false-positive against each other.
	// Flip ExpectConflicts to false when NULL-distinctness is fixed.
	Register(testdataWorkload("oltp-insert-null-then-fill", []string{"uk_table"}, 20_000, true))

	// ---- schema: shape probes (index count, row width, key structure) ----

	// Control table without any unique index: the conflict-detection
	// machinery never engages, measuring the pipeline ceiling.
	Register(testdataWorkload("schema-no-uk-control", []string{"no_uk_table"}, 20_000, false))

	// Two unique indexes on one table (email, username): per-index cost.
	Register(testdataWorkload("schema-two-unique-indexes", []string{"accounts"}, 20_000, false))

	// Composite key UNIQUE(tenant_id, slug), one tenant per row (tenant_id ==
	// id) so no two rows ever share a tenant value: zero conflicts under ANY
	// semantics (per-column or tuple) — the semantics-invariant baseline for
	// composite-key cost. Same DML as schema-composite-uk-shared-tenants; the
	// seed's tenant distribution is the only difference between the pair.
	Register(testdataWorkload("schema-composite-uk-unique-tenants", []string{"comp_items"}, 20_000, false))

	// Composite key UNIQUE(tenant_id, slug) with 100 SHARED tenants; all slugs
	// renamed to globally fresh values => zero conflicts under composite-tuple
	// semantics, but flattened per-column comparison would false-positive on
	// the shared tenant_id — the discriminator proving tuples are honored end
	// to end. Counterpart of schema-composite-uk-unique-tenants (same DML;
	// only the seed's tenant distribution differs).
	Register(testdataWorkload("schema-composite-uk-shared-tenants", []string{"tenant_items"}, 20_000, false))

	// Wide rows: 50-column before-images (REPLICA IDENTITY FULL) measure the
	// decode/convert share of the pipeline.
	Register(testdataWorkload("schema-wide-rows", []string{"wide_table"}, 20_000, false))

	// Composite key UNIQUE(folder_id, name) where every index tuple has a NULL
	// component (unnamed drafts sharing a folder). Semantically ZERO conflicts
	// — SQL treats NULLs as distinct — but detection compares tuples with
	// nil==nil, so in-flight draft pairs false-positive against each other.
	// Flip ExpectConflicts to false when NULL-distinctness is fixed.
	Register(testdataWorkload("schema-composite-uk-null-component", []string{"docs"}, 20_000, true))

	// Payload-only updates to soft-DELETED rows that legitimately share email
	// values (the partial index UNIQUE(email) WHERE NOT deleted excludes
	// them). Detection drops the predicate, so in-flight pair updates
	// "conflict" on email: the partial-predicate false positive documented in
	// conflictDetectionCache.go. Zero semantic conflicts; flip ExpectConflicts
	// to false if detection ever becomes predicate-aware.
	Register(testdataWorkload("schema-partial-index-inactive-rows", []string{"soft_users"}, 20_000, true))

	// ---- edge: degenerate op-mix corner cases ----

	// Updates only: every event scans the cache, none conflict.
	Register(testdataWorkload("edge-all-updates", []string{"uk_table"}, 20_000, false))

	// Inserts only: checks run against an always-empty cache.
	Register(testdataWorkload("edge-all-inserts", []string{"uk_table"}, 20_000, false))

	// Deletes only: the cache fills but nothing ever scans it.
	Register(testdataWorkload("edge-all-deletes", []string{"uk_table"}, 20_000, false))

	// Archival: updates + deletes, no inserts.
	Register(testdataWorkload("edge-updates-and-deletes", []string{"uk_table"}, 20_000, false))

	// ---- conflict: engineered, semantically REAL conflicts ----

	// Update pairs: the second row takes the unique value the first just freed.
	Register(testdataWorkload("conflict-update-pairs", []string{"uk_table"}, 20_000, true))

	// "Replace" upsert idiom: DELETE + re-INSERT with the same unique value —
	// the documented DELETE-INSERT conflict type, at volume.
	Register(testdataWorkload("conflict-delete-reinsert", []string{"uk_table"}, 20_000, true))

	// Unique-value swap between row pairs via a temp value (UPDATE-UPDATE chains).
	Register(testdataWorkload("conflict-value-swap", []string{"uk_table"}, 19_998, true))

	// All four conflict types documented in conflictDetectionCache.go:
	// UPDATE-INSERT, UPDATE-UPDATE, DELETE-INSERT, DELETE-UPDATE.
	Register(testdataWorkload("conflict-documented-types", []string{"uk_table"}, 20_000, true))

	// Real conflicts confined to the SECOND unique index of a two-index table.
	Register(testdataWorkload("conflict-second-index", []string{"accounts2"}, 20_000, true))

	// UNIQUE NULLS NOT DISTINCT: at most one row may hold NULL, and the single
	// NULL "slot" is handed row to row (release, then claim). Each handoff is
	// a REAL conflict — the claim must not apply before the release commits.
	// This is the case that makes nil==nil detection correct; it must KEEP
	// detecting after NULL-distinctness is fixed for ordinary unique indexes.
	Register(testdataWorkload("conflict-nulls-not-distinct", []string{"null_slot"}, 20_000, true))

	// Versioned rows behind a partial unique index (UNIQUE(entity_id) WHERE
	// most_recent): demote current version + insert successor with the same
	// entity_id. REAL conflicts — the insert must not apply before the
	// demotion. Possible because exporter metadata includes partial indexes
	// (columns only, predicate dropped).
	Register(testdataWorkload("conflict-versioned-rows-partial-index", []string{"versions"}, 20_000, true))
}
