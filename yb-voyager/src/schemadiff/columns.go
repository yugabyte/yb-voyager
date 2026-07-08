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

package schemadiff

import "github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"

// diffColumns computes column-level differences between snapshots a and b. Columns
// are matched by chooseMatchKeys: by stable "{tableOID}:{attnum}" ID when IDs are
// usable (same engine, all present) so a rename surfaces as COLUMN_NAME_CHANGED,
// else by the unquoted dot-joined composite key schema.table.col (see Column.ForKey
// and its dot-collision limitation). Unmatched side-A columns become COLUMN_DROPPED
// and unmatched side-B columns COLUMN_ADDED.
func diffColumns(a, b *schemasnapshot.SnapshotContent) []Difference {
	keyA, keyB := chooseMatchKeys(a.DatabaseType, b.DatabaseType, a.Columns, b.Columns,
		func(c schemasnapshot.Column) string { return c.ID },
		func(c schemasnapshot.Column, dbType string) string { return c.ForKey(dbType) })

	return matchByKey(a.Columns, b.Columns, keyA, keyB,
		compareMatchedColumns,
		func(c schemasnapshot.Column) []Difference {
			return []Difference{newDifference(ColumnDropped, c.Table, &c.Table, c.Name, c, nil)}
		},
		func(c schemasnapshot.Column) []Difference {
			return []Difference{newDifference(ColumnAdded, c.Table, &c.Table, c.Name, nil, c)}
		},
	)
}

// compareMatchedColumns emits all field-level differences for a pair of columns
// matched by chooseMatchKeys (ID or composite name key). Object in each
// Difference is always the side-A (old) Column.Table.
func compareMatchedColumns(cA, cB schemasnapshot.Column) []Difference {
	var diffs []Difference

	if cA.Name != cB.Name {
		diffs = append(diffs, newDifference(ColumnNameChanged, cA.Table, &cA.Table, cA.Name, cA.Name, cB.Name))
	}

	if cA.DataType != cB.DataType {
		diffs = append(diffs, newDifference(ColumnTypeChanged, cA.Table, &cA.Table, cA.Name, cA.DataType, cB.DataType))
	}

	if cA.NotNull != cB.NotNull {
		diffs = append(diffs, newDifference(ColumnNullabilityChanged, cA.Table, &cA.Table, cA.Name, cA.NotNull, cB.NotNull))
	}

	if cA.Default != cB.Default {
		diffs = append(diffs, newDifference(ColumnDefaultChanged, cA.Table, &cA.Table, cA.Name, cA.Default, cB.Default))
	}

	return diffs
}
