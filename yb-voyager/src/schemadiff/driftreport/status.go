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

// statusByDiffType is a provisional severity policy — to be reconciled with
// the DDL-scenario matrix in a later PR. Any schemadiff.DiffType not present
// here classifies as StatusAdvisory (the safe default) via Classify.
var statusByDiffType = map[schemadiff.DiffType]Status{
	// Breaking, but recoverable: the source object identity/shape changed in a
	// way the target migration cannot silently reconcile, though the data is
	// not gone.
	schemadiff.TableDropped:       StatusBreaksRecoverable,
	schemadiff.ColumnDropped:      StatusBreaksRecoverable,
	schemadiff.ColumnNameChanged:  StatusBreaksRecoverable,
	schemadiff.TableNameChanged:   StatusBreaksRecoverable,
	schemadiff.TableSchemaChanged: StatusBreaksRecoverable,
	schemadiff.ColumnTypeChanged:  StatusBreaksRecoverable,

	// Potential impact: new objects/relaxed constraints that may need review
	// but do not themselves indicate lost or renamed data.
	schemadiff.TableAdded:               StatusPotentialImpact,
	schemadiff.ColumnAdded:              StatusPotentialImpact,
	schemadiff.ColumnNullabilityChanged: StatusPotentialImpact,
	schemadiff.TableKindChanged:         StatusPotentialImpact,
	schemadiff.PartitionParentChanged:   StatusPotentialImpact,
	schemadiff.PartitionChildrenChanged: StatusPotentialImpact,

	// Advisory: informational changes unlikely to affect migration mechanics.
	schemadiff.ColumnDefaultChanged:    StatusAdvisory,
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
