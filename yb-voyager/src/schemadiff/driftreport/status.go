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

// Status is the severity level assigned to a DiffEntry.
type Status string

const (
	StatusAdvisory            Status = "advisory"
	StatusPotentialImpact     Status = "potential_impact"
	StatusBreaksRecoverable   Status = "breaks_migration_recoverable"
	StatusBreaksUnrecoverable Status = "breaks_migration_unrecoverable"
)

// statusByDiffType is the severity policy, aligned with the design mockup and
// the DDL-scenario matrix. The question each level answers is what the migration
// does, NOT how alarming the DDL sounds:
//
//   - Unrecoverable: the pipeline cannot be resumed; the migration must be
//     restarted from scratch (P0 in the matrix).
//   - Recoverable: `import data` can fail, but applying the DDL on the target and
//     re-running clears it (P1).
//   - Potential impact: the migration runs fine, yet source and target diverge in
//     a way that matters after cutover (P2).
//   - Advisory: informational only.
//
// Read against that definition, four of these deliberately differ from an earlier
// draft: a dropped COLUMN does not break anything (it only leaves the target with
// an extra column) so it is Potential impact, while an ADDED column can fail
// `import data` and so is Recoverable — the opposite of the intuitive reading. A
// dropped or renamed captured table is Unrecoverable because `export data` cannot
// be restarted afterwards. Any DiffType not present here classifies as
// StatusAdvisory (the safe default) via Classify.
var statusByDiffType = map[schemadiff.DiffType]Status{
	// Unrecoverable: export data cannot be restarted; restart from scratch.
	schemadiff.TableDropped:       StatusBreaksUnrecoverable,
	schemadiff.TableNameChanged:   StatusBreaksUnrecoverable,
	schemadiff.TableSchemaChanged: StatusBreaksUnrecoverable,

	// Recoverable: import data can fail until the DDL is applied on the target,
	// then re-running import data clears it.
	schemadiff.ColumnAdded:              StatusBreaksRecoverable,
	schemadiff.ColumnNameChanged:        StatusBreaksRecoverable,
	schemadiff.ColumnTypeChanged:        StatusBreaksRecoverable,
	schemadiff.ColumnNullabilityChanged: StatusBreaksRecoverable,

	// Potential impact: the migration is unaffected, but source and target
	// diverge in a way that matters at or after cutover.
	schemadiff.TableAdded:                    StatusPotentialImpact,
	schemadiff.ColumnDropped:                 StatusPotentialImpact,
	schemadiff.ColumnDefaultChanged:          StatusPotentialImpact,
	schemadiff.TableKindChanged:              StatusPotentialImpact,
	schemadiff.TablePartitionParentChanged:   StatusPotentialImpact,
	schemadiff.TablePartitionChildrenChanged: StatusPotentialImpact,

	// Advisory: informational changes unlikely to affect migration mechanics.
	schemadiff.TableInheritsChanged:    StatusAdvisory,
	schemadiff.TableInheritedByChanged: StatusAdvisory,
}

// Classify maps a schemadiff.DiffType to its Status. Provisional severity
// policy — to be reconciled with the DDL-scenario matrix in a later PR.
// Any DiffType not explicitly mapped (including the zero value) classifies
// as StatusAdvisory, the safe default.
func Classify(t schemadiff.DiffType) Status {
	if s, ok := statusByDiffType[t]; ok {
		return s
	}
	return StatusAdvisory
}
