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

// guidanceByDiffType holds short, one-line action hints per DiffType.
// Provisional wording — to be reconciled with the DDL-scenario matrix in a
// later PR. A DiffType with no entry yields "" from Guidance.
var guidanceByDiffType = map[schemadiff.DiffType]string{
	schemadiff.TableAdded:                    "A new table was added on the source after this capture; it will not be migrated unless re-exported.",
	schemadiff.TableDropped:                  "A table was dropped on the source after this capture; the target may retain data that no longer exists on the source.",
	schemadiff.TableNameChanged:              "A table was renamed on the source after this capture; the target still has it under the old name.",
	schemadiff.TableSchemaChanged:            "A table moved to a different schema on the source after this capture; the target still has it under the old schema.",
	schemadiff.TableKindChanged:              "A table's kind (ordinary/partitioned/foreign) changed on the source after this capture. Review before cutover.",
	schemadiff.TablePartitionParentChanged:   "A partition's parent table changed on the source after this capture. Review the partitioning layout before cutover.",
	schemadiff.TablePartitionChildrenChanged: "A partitioned table's set of child partitions changed on the source after this capture. Review before cutover.",
	schemadiff.TableInheritsChanged:          "A table's INHERITS parent(s) changed on the source after this capture.",
	schemadiff.TableInheritedByChanged:       "The set of tables inheriting from this table changed on the source after this capture.",

	schemadiff.ColumnAdded:              "A new column was added on the source after this capture; it will not be migrated unless re-exported.",
	schemadiff.ColumnDropped:            "A column was dropped on the source after this capture; the target may retain it. Review before cutover.",
	schemadiff.ColumnNameChanged:        "A column was renamed on the source after this capture; the target still has it under the old name.",
	schemadiff.ColumnTypeChanged:        "A column's data type changed on the source after this capture; existing target data may not match. Review before cutover.",
	schemadiff.ColumnNullabilityChanged: "A column's NOT NULL constraint changed on the source after this capture. Review before cutover.",
	schemadiff.ColumnDefaultChanged:     "A column's default expression changed on the source after this capture.",
}

// Guidance returns a short, one-line action hint for t, or "" if none is
// defined. Provisional wording — to be reconciled with the DDL-scenario
// matrix in a later PR.
func Guidance(t schemadiff.DiffType) string {
	return guidanceByDiffType[t]
}
