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

// Catalog v2: multi-table OLTP shapes, second-index conflicts, composite-key
// baseline, and partial-unique-index patterns (the case the conflict cache's
// before-before logic and its documented false positives are about).
func init() {
	// TPC-C-ish checkout across four co-streamed tables: orders + line items +
	// hot inventory + payments. Multi-table event routing with two UK tables
	// in the mix.
	Register(Workload{
		Name:            "oltp-checkout-multi-table",
		SchemaSQL:       mustRead("checkout_multi_table/schema.sql"),
		SeedSQL:         mustRead("checkout_multi_table/seed.sql"),
		DMLSQL:          mustRead("checkout_multi_table/dml.sql"),
		TableList:       []string{"co_orders", "co_items", "co_inventory", "co_payments"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// The most common real migration shape: one UK table receiving 20% of
	// traffic co-streamed with a plain table receiving 80%. Measures the
	// collateral damage of UK conflict checks on unrelated tables that share
	// the single ingest thread.
	Register(Workload{
		Name:            "oltp-mixed-uk-and-plain-tables",
		SchemaSQL:       mustRead("mixed_uk_and_plain/schema.sql"),
		SeedSQL:         mustRead("mixed_uk_and_plain/seed.sql"),
		DMLSQL:          mustRead("mixed_uk_and_plain/dml.sql"),
		TableList:       []string{"accounts", "audit_log"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// Double-entry ledger: append-only journal (unique entry numbers) + hot
	// balance rows without unique keys.
	Register(Workload{
		Name:            "oltp-ledger",
		SchemaSQL:       mustRead("ledger/schema.sql"),
		SeedSQL:         mustRead("ledger/seed.sql"),
		DMLSQL:          mustRead("ledger/dml.sql"),
		TableList:       []string{"journal", "balances"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// Job queue / outbox churn: insert -> claim -> complete -> delete with
	// short row lifetimes and never-reused unique keys.
	Register(Workload{
		Name:            "oltp-job-queue",
		SchemaSQL:       mustRead("job_queue/schema.sql"),
		SeedSQL:         mustRead("job_queue/seed.sql"),
		DMLSQL:          mustRead("job_queue/dml.sql"),
		TableList:       []string{"jobs"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// Composite unique key without conflicts (tenant_id == id, so the
	// expectation is stable under both per-column and tuple semantics;
	// the per-column false-positive case is canary-composite-per-column).
	Register(Workload{
		Name:            "schema-composite-uk-no-conflict",
		SchemaSQL:       mustRead("composite_uk_no_conflict/schema.sql"),
		SeedSQL:         mustRead("composite_uk_no_conflict/seed.sql"),
		DMLSQL:          mustRead("composite_uk_no_conflict/dml.sql"),
		TableList:       []string{"comp_items"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})

	// Real conflicts confined to the SECOND unique index of a two-index table.
	Register(Workload{
		Name:            "conflict-second-index",
		SchemaSQL:       mustRead("second_index_conflict/schema.sql"),
		SeedSQL:         mustRead("second_index_conflict/seed.sql"),
		DMLSQL:          mustRead("second_index_conflict/dml.sql"),
		TableList:       []string{"accounts2"},
		ExpectedEvents:  20_000,
		ExpectConflicts: true,
	})

	// Versioned rows behind a partial unique index (UNIQUE(entity_id) WHERE
	// most_recent): demote current version + insert successor with the same
	// entity_id. REAL conflicts — the insert must not apply before the
	// demotion. The exporter's unique-index metadata includes partial indexes
	// (columns only, predicate dropped), which is what makes detection here
	// possible at all.
	Register(Workload{
		Name:            "conflict-versioned-rows-partial-index",
		SchemaSQL:       mustRead("versioned_rows_partial/schema.sql"),
		SeedSQL:         mustRead("versioned_rows_partial/seed.sql"),
		DMLSQL:          mustRead("versioned_rows_partial/dml.sql"),
		TableList:       []string{"versions"},
		ExpectedEvents:  20_000,
		ExpectConflicts: true,
	})

	// Payload-only updates to soft-DELETED rows that legitimately share email
	// values (the partial index UNIQUE(email) WHERE NOT deleted excludes
	// them). Detection drops the predicate, so in-flight pair updates
	// "conflict" on email: the exact partial-predicate false positive
	// documented in conflictDetectionCache.go.
	// CANARY: zero semantic conflicts; flips to false if detection ever
	// becomes predicate-aware.
	Register(Workload{
		Name:            "canary-partial-index-inactive-rows",
		SchemaSQL:       mustRead("soft_delete_partial/schema.sql"),
		SeedSQL:         mustRead("soft_delete_partial/seed.sql"),
		DMLSQL:          mustRead("soft_delete_partial/dml.sql"),
		TableList:       []string{"soft_users"},
		ExpectedEvents:  20_000,
		ExpectConflicts: true,
	})
}
