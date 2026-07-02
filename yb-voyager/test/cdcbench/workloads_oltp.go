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

// OLTP-pattern workloads modeled on common (and corner-case) customer shapes,
// plus the conflict types documented in cmd/conflictDetectionCache.go.
//
// Two workloads are deliberate CANARIES: they assert conflicts that are false
// positives of the current detection semantics. When the semantics are fixed,
// their ExpectConflicts flips to false — that flip is the desired signal.
func init() {
	// CUSTOMER PATTERN (reported in the field): records are INSERTed as drafts
	// with NULL unique columns; the unique value is filled by a later UPDATE.
	// SQL-semantically there are ZERO conflicts (NULLs are distinct in unique
	// indexes), but detection currently treats nil==nil as a conflict.
	// CANARY: flip ExpectConflicts to false when NULL-distinctness is fixed.
	Register(Workload{
		Name:            "canary-null-fill",
		SchemaSQL:       mustRead("insert_null_then_fill/schema.sql"),
		SeedSQL:         mustRead("insert_null_then_fill/seed.sql"),
		DMLSQL:          mustRead("insert_null_then_fill/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: true,
	})

	// The most common OLTP lifecycle: insert once, then mutate status/payload
	// columns repeatedly; the unique column never changes.
	Register(Workload{
		Name:            "oltp-order-lifecycle",
		SchemaSQL:       mustRead("orders_status/schema.sql"),
		SeedSQL:         mustRead("orders_status/seed.sql"),
		DMLSQL:          mustRead("orders_status/dml.sql"),
		TableList:       []string{"orders"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// Auth/session churn: unique tokens inserted and deleted, never reused.
	Register(Workload{
		Name:            "oltp-session-churn",
		SchemaSQL:       mustRead("session_token_churn/schema.sql"),
		SeedSQL:         mustRead("session_token_churn/seed.sql"),
		DMLSQL:          mustRead("session_token_churn/dml.sql"),
		TableList:       []string{"sessions"},
		ExpectedEvents:  19_900,
		ExpectConflicts: false,
	})

	// "Replace" upsert idiom: DELETE + re-INSERT with the same unique value —
	// the documented DELETE-INSERT conflict type, at volume.
	Register(Workload{
		Name:            "conflict-delete-reinsert",
		SchemaSQL:       mustRead("delete_reinsert/schema.sql"),
		SeedSQL:         mustRead("delete_reinsert/seed.sql"),
		DMLSQL:          mustRead("delete_reinsert/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: true,
	})

	// Unique-value swap between row pairs via a temp value (UPDATE-UPDATE chains).
	Register(Workload{
		Name:            "conflict-value-swap",
		SchemaSQL:       mustRead("uk_value_swap/schema.sql"),
		SeedSQL:         mustRead("uk_value_swap/seed.sql"),
		DMLSQL:          mustRead("uk_value_swap/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  19_998,
		ExpectConflicts: true,
	})

	// Two unique indexes on one table (email, username): per-index scan cost.
	Register(Workload{
		Name:            "schema-two-unique-indexes",
		SchemaSQL:       mustRead("two_unique_indexes/schema.sql"),
		SeedSQL:         mustRead("two_unique_indexes/seed.sql"),
		DMLSQL:          mustRead("two_unique_indexes/dml.sql"),
		TableList:       []string{"accounts"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// Multi-tenant composite key UNIQUE(tenant_id, slug); all slugs renamed to
	// globally fresh values. True composite-tuple semantics => zero conflicts,
	// but flattened per-column unique-key metadata (what current exporters
	// write) makes in-flight same-tenant rows "conflict" on tenant_id.
	// CANARY: flip ExpectConflicts to false once composite tuples are honored
	// end-to-end (exporter metadata + detection).
	Register(Workload{
		Name:            "canary-composite-per-column",
		SchemaSQL:       mustRead("composite_uk/schema.sql"),
		SeedSQL:         mustRead("composite_uk/seed.sql"),
		DMLSQL:          mustRead("composite_uk/dml.sql"),
		TableList:       []string{"tenant_items"},
		ExpectedEvents:  20_000,
		ExpectConflicts: true,
	})

	// Wide rows: 50-column before-images (REPLICA IDENTITY FULL) measure the
	// decode/convert share of the pipeline.
	Register(Workload{
		Name:            "schema-wide-rows",
		SchemaSQL:       mustRead("wide_rows/schema.sql"),
		SeedSQL:         mustRead("wide_rows/seed.sql"),
		DMLSQL:          mustRead("wide_rows/dml.sql"),
		TableList:       []string{"wide_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// Skewed access: 80% of updates hit 100 hot rows (same-PK exclusion path),
	// 20% change cold rows' unique values.
	Register(Workload{
		Name:            "oltp-skewed-hot-rows",
		SchemaSQL:       mustRead("skewed_hot_rows/schema.sql"),
		SeedSQL:         mustRead("skewed_hot_rows/seed.sql"),
		DMLSQL:          mustRead("skewed_hot_rows/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// All four conflict types documented in conflictDetectionCache.go.
	Register(Workload{
		Name:            "conflict-documented-types",
		SchemaSQL:       mustRead("documented_conflict_types/schema.sql"),
		SeedSQL:         mustRead("documented_conflict_types/seed.sql"),
		DMLSQL:          mustRead("documented_conflict_types/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: true,
	})

	// Corner case: inserts only — checks run against an always-empty cache.
	Register(Workload{
		Name:            "edge-all-inserts",
		SchemaSQL:       mustRead("all_inserts_uk/schema.sql"),
		SeedSQL:         mustRead("all_inserts_uk/seed.sql"),
		DMLSQL:          mustRead("all_inserts_uk/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// Corner case: deletes only — cache fills but nothing ever scans it.
	Register(Workload{
		Name:            "edge-all-deletes",
		SchemaSQL:       mustRead("all_deletes_uk/schema.sql"),
		SeedSQL:         mustRead("all_deletes_uk/seed.sql"),
		DMLSQL:          mustRead("all_deletes_uk/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// Corner case: archival — updates + deletes, no inserts.
	Register(Workload{
		Name:            "edge-updates-and-deletes",
		SchemaSQL:       mustRead("updates_and_deletes/schema.sql"),
		SeedSQL:         mustRead("updates_and_deletes/seed.sql"),
		DMLSQL:          mustRead("updates_and_deletes/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})
}
