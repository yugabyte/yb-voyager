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

package driftreport

import "github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"

// Guidance is a finding's "Impact & action" note, split exactly as the design
// mockup renders it: one paragraph saying what the migration will actually do if
// the change is not reconciled on the target, then one paragraph giving the
// corrective step.
//
// Wording rules taken from the mockup, because they are what make the note
// useful rather than a restatement of the header:
//
//   - Impact names the affected command concretely — "`import data` can fail
//     if …" — or states plainly that the migration is unaffected AND why the
//     change still matters ("…but inserts relying on the default will diverge
//     between source and target after cutover").
//   - Action is an instruction, not "review before cutover": what to change on
//     the target, and how to get the pipeline moving again if it already failed.
//   - Voyager never applies any of this automatically; say so where a reader
//     might assume otherwise.
//
// Text in `backticks` renders as inline code in the HTML report (see
// codeSpans in render.go) and stays literal in the JSON report.
type Guidance struct {
	Impact string `json:"impact"`
	Action string `json:"action"`
}

// guidanceByDiffType holds the Impact & action note per DiffType.
//
// The entries for the cases the design mockup covers use its wording verbatim.
// The remaining v1 DiffTypes follow the same pattern, derived from the
// DDL-scenario matrix (severity P0/P1/P2 and the "Complete Flow" column):
// a rename or schema move of a captured table is P0 because export data cannot
// be restarted afterwards, while nullability/default changes are parity concerns.
//
// A DiffType with no entry yields the zero Guidance, and the report omits the
// note entirely.
var guidanceByDiffType = map[schemadiff.DiffType]Guidance{
	schemadiff.TableAdded: {
		Impact: "If a table is added while `export data` is running, the migration does not pick up the newly added table — Voyager does not change the scope of a data migration mid-migration, so this table's data is not migrated.",
		Action: "Create the table on the target and start a separate supplemental migration to migrate its data: a fresh `export data` in a new export directory listing only the new table in `--table-list`.",
	},
	schemadiff.TableDropped: {
		Impact: "If a table is dropped while `export data` is running, it won't crash `export data`, but any `export data` restart will fail because the migration still expects the dropped table — the migration cannot be resumed.",
		Action: "The migration must be restarted from scratch. Avoid dropping migrated tables mid-migration.",
	},
	schemadiff.TableNameChanged: {
		Impact: "`export data` keeps running, but any restart fails: the stored table list still holds the old name while the source catalog now has the new one, so the table lookup errors out and the migration cannot be resumed. Rows written under the new name are never exported.",
		Action: "Rename the table on the target to match, then restart the migration from scratch. Avoid renaming migrated tables mid-migration.",
	},
	schemadiff.TableSchemaChanged: {
		Impact: "Moving a captured table to another schema has the same effect as renaming it: `export data` keeps running, but a restart fails because the stored table list still points at the old schema, so the migration cannot be resumed.",
		Action: "Move the table on the target to match, then restart the migration from scratch.",
	},
	schemadiff.ColumnAdded: {
		Impact: "`import data` can fail if a column added on the source is not applied on the target.",
		Action: "Add the column on the target. If `import data` has failed, re-run `import data` once the column is added.",
	},
	schemadiff.ColumnDropped: {
		Impact: "The migration is not affected by this, but the target keeps the dropped column, so the schemas diverge. Voyager does not drop it automatically.",
		Action: "Drop the column on the target before cutover.",
	},
	schemadiff.ColumnNameChanged: {
		Impact: "`import data` can fail if a column renamed on the source is still under its old name on the target, since incoming events carry the new name.",
		Action: "Rename the column on the target. If `import data` has failed, re-run `import data` once the names match.",
	},
	schemadiff.ColumnTypeChanged: {
		Impact: "`import data` can fail if this column's type change on the source is not applied on the target.",
		Action: "Apply a compatible type change on the target. If `import data` has failed, re-run `import data` once the types are aligned.",
	},
	schemadiff.ColumnNullabilityChanged: {
		Impact: "`import data` can fail if the source now permits values the target column still rejects — for example rows with NULLs arriving after a `DROP NOT NULL` on the source.",
		Action: "Apply the same nullability on the target. If `import data` has failed, re-run `import data` once the constraints match.",
	},
	schemadiff.ColumnDefaultChanged: {
		Impact: "The migration is not affected by this, but inserts relying on the default will diverge between source and target after cutover.",
		Action: "Apply the same default on the target before cutover.",
	},
	schemadiff.TableKindChanged: {
		Impact: "The migration is not affected mid-flight, but the target still has the table in its original form (ordinary vs partitioned vs foreign), so the schemas diverge and a restart may not reproduce the source layout.",
		Action: "Recreate the table on the target with the matching kind before cutover.",
	},
	schemadiff.TablePartitionParentChanged: {
		Impact: "The migration is not affected by this, but the target's partitioning layout no longer matches the source, so rows may land in a different partition after cutover.",
		Action: "Apply the same partition attachment on the target before cutover.",
	},
	schemadiff.TablePartitionChildrenChanged: {
		Impact: "A partition added on the source is outside the migration's scope, exactly like a newly added table, so its data is not migrated; a partition dropped on the source leaves the target with data the source no longer has.",
		Action: "Mirror the partition change on the target. For an added partition, migrate its data with a separate supplemental migration.",
	},
	schemadiff.TableInheritsChanged: {
		Impact: "The migration is not affected by this, but the target's inheritance relationships no longer match the source, which changes what queries against the parent return after cutover.",
		Action: "Apply the same `INHERITS` change on the target before cutover.",
	},
	schemadiff.TableInheritedByChanged: {
		Impact: "The migration is not affected by this, but the set of tables inheriting from this one differs on the target, which changes what queries against it return after cutover.",
		Action: "Apply the same inheritance change on the target before cutover.",
	},
}

// GuidanceFor returns the Impact & action note for t, or the zero Guidance when
// none is defined (the report then omits the note).
func GuidanceFor(t schemadiff.DiffType) Guidance {
	return guidanceByDiffType[t]
}
