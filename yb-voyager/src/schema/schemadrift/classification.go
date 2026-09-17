// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemadrift

import "github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"

// Status is the severity level assigned to a DiffEntry.
type Status string

const (
	StatusAdvisory            Status = "advisory"
	StatusPotentialImpact     Status = "potential_impact"
	StatusBreaksRecoverable   Status = "breaks_migration_recoverable"
	StatusBreaksUnrecoverable Status = "breaks_migration_unrecoverable"
)

// classification is everything the report says about a kind of change: how the
// migration is affected, and the "Impact & action" note explaining it. Severity and
// wording have to agree, so they are declared together rather than in parallel maps.
//
// `backticks` become inline code in HTML (see codeSpans) and stay literal in JSON.
type classification struct {
	Status Status
	Impact string
	Action string
}

// Severity says what the MIGRATION does about a change, not how alarming the DDL
// sounds, so it reads backwards in places: an ADDED column is Recoverable because
// import data can fail on it, while a DROPPED one is only Potential impact.
var classificationByDiffType = map[schemadiff.DiffType]classification{
	// Unrecoverable: export data cannot be restarted; restart from scratch.
	schemadiff.TableDropped: {
		Status: StatusBreaksUnrecoverable,
		Impact: "If a table is dropped while `export data` is running, it won't crash `export data`, but any `export data` restart will fail because the migration still expects the dropped table — the migration cannot be resumed.",
		Action: "The migration must be restarted from scratch. Avoid dropping migrated tables mid-migration.",
	},
	schemadiff.TableNameChanged: {
		Status: StatusBreaksUnrecoverable,
		Impact: "`export data` keeps running, but any restart fails: the stored table list still holds the old name while the source catalog now has the new one, so the table lookup errors out and the migration cannot be resumed. Rows written under the new name are never exported.",
		Action: "Rename the table on the target to match, then restart the migration from scratch. Avoid renaming migrated tables mid-migration.",
	},
	schemadiff.TableSchemaChanged: {
		Status: StatusBreaksUnrecoverable,
		Impact: "Moving a captured table to another schema has the same effect as renaming it: `export data` keeps running, but a restart fails because the stored table list still points at the old schema, so the migration cannot be resumed.",
		Action: "Move the table on the target to match, then restart the migration from scratch.",
	},

	// Recoverable: import data can fail until the DDL is applied on the target.
	schemadiff.ColumnAdded: {
		Status: StatusBreaksRecoverable,
		Impact: "`import data` can fail if a column added on the source is not applied on the target.",
		Action: "Add the column on the target. If `import data` has failed, re-run `import data` once the column is added.",
	},
	schemadiff.ColumnNameChanged: {
		Status: StatusBreaksRecoverable,
		Impact: "`import data` can fail if a column renamed on the source is still under its old name on the target, since incoming events carry the new name.",
		Action: "Rename the column on the target. If `import data` has failed, re-run `import data` once the names match.",
	},
	schemadiff.ColumnTypeChanged: {
		Status: StatusBreaksRecoverable,
		Impact: "`import data` can fail if this column's type change on the source is not applied on the target.",
		Action: "Apply a compatible type change on the target. If `import data` has failed, re-run `import data` once the types are aligned.",
	},
	schemadiff.ColumnNullabilityChanged: {
		Status: StatusBreaksRecoverable,
		Impact: "`import data` can fail if the source now permits values the target column still rejects — for example rows with NULLs arriving after a `DROP NOT NULL` on the source.",
		Action: "Apply the same nullability on the target. If `import data` has failed, re-run `import data` once the constraints match.",
	},

	// Potential impact: migration unaffected, but the schemas diverge.
	schemadiff.TableAdded: {
		Status: StatusPotentialImpact,
		Impact: "If a table is added while `export data` is running, the migration does not pick up the newly added table — Voyager does not change the scope of a data migration mid-migration, so this table's data is not migrated.",
		Action: "Create the table on the target and start a separate supplemental migration to migrate its data: a fresh `export data` in a new export directory listing only the new table in `--table-list`.",
	},
	schemadiff.ColumnDropped: {
		Status: StatusPotentialImpact,
		Impact: "The migration is not affected by this, but the target keeps the dropped column, so the schemas diverge. Voyager does not drop it automatically.",
		Action: "Drop the column on the target before cutover.",
	},
	schemadiff.ColumnDefaultChanged: {
		Status: StatusPotentialImpact,
		Impact: "The migration is not affected by this, but inserts relying on the default will diverge between source and target after cutover.",
		Action: "Apply the same default on the target before cutover.",
	},
	schemadiff.TableKindChanged: {
		Status: StatusPotentialImpact,
		Impact: "The migration is not affected mid-flight, but the target still has the table in its original form (ordinary vs partitioned vs foreign), so the schemas diverge and a restart may not reproduce the source layout.",
		Action: "Recreate the table on the target with the matching kind before cutover.",
	},
	schemadiff.TablePartitionParentChanged: {
		Status: StatusPotentialImpact,
		Impact: "The migration is not affected by this, but the target's partitioning layout no longer matches the source, so rows may land in a different partition after cutover.",
		Action: "Apply the same partition attachment on the target before cutover.",
	},
	schemadiff.TablePartitionChildrenChanged: {
		Status: StatusPotentialImpact,
		Impact: "A partition added on the source is outside the migration's scope, exactly like a newly added table, so its data is not migrated; a partition dropped on the source leaves the target with data the source no longer has.",
		Action: "Mirror the partition change on the target. For an added partition, migrate its data with a separate supplemental migration.",
	},

	// Advisory: informational changes unlikely to affect migration mechanics.
	schemadiff.TableInheritsChanged: {
		Status: StatusAdvisory,
		Impact: "The migration is not affected by this, but the target's inheritance relationships no longer match the source, which changes what queries against the parent return after cutover.",
		Action: "Apply the same `INHERITS` change on the target before cutover.",
	},
	schemadiff.TableInheritedByChanged: {
		Status: StatusAdvisory,
		Impact: "The migration is not affected by this, but the set of tables inheriting from this one differs on the target, which changes what queries against it return after cutover.",
		Action: "Apply the same inheritance change on the target before cutover.",
	},
}

// classify falls back to StatusAdvisory for an unmapped DiffType, the zero value
// included, rather than dropping the change. The zero Impact/Action that comes with
// it is deliberate: the report omits the note when there is nothing useful to say.
func classify(t schemadiff.DiffType) classification {
	if c, ok := classificationByDiffType[t]; ok {
		return c
	}
	return classification{Status: StatusAdvisory}
}
