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

// statusByDiffType is the severity policy, from the design mockup and the
// DDL-scenario matrix. Each level says what the MIGRATION does, not how alarming
// the DDL sounds:
//
//   - Unrecoverable: pipeline cannot be resumed; restart from scratch (P0).
//   - Recoverable: `import data` can fail until the DDL is applied on the target (P1).
//   - Potential impact: migration runs fine, but source and target diverge (P2).
//   - Advisory: informational only.
//
// So an ADDED column is Recoverable while a DROPPED one is only Potential impact —
// the opposite of the intuitive reading. Unmapped types default to Advisory.
var statusByDiffType = map[schemadiff.DiffType]Status{
	// Unrecoverable: export data cannot be restarted; restart from scratch.
	schemadiff.TableDropped:       StatusBreaksUnrecoverable,
	schemadiff.TableNameChanged:   StatusBreaksUnrecoverable,
	schemadiff.TableSchemaChanged: StatusBreaksUnrecoverable,

	// Recoverable: import data can fail until the DDL is applied on the target.
	schemadiff.ColumnAdded:              StatusBreaksRecoverable,
	schemadiff.ColumnNameChanged:        StatusBreaksRecoverable,
	schemadiff.ColumnTypeChanged:        StatusBreaksRecoverable,
	schemadiff.ColumnNullabilityChanged: StatusBreaksRecoverable,

	// Potential impact: migration unaffected, but the schemas diverge.
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
