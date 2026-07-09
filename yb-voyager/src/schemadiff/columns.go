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

// diffColumnsIn computes column-level differences between two column slices
// (the nested columns of a matched table pair). Matched by chooseMatchKeys:
// by stable "{tableOID}:{attnum}" ID when IDs are usable (same engine, all
// present) so a rename surfaces as COLUMN_NAME_CHANGED, else by the unquoted
// dot-joined composite key schema.table.col (see Column.ForKey and its
// dot-collision limitation). Unmatched side-A columns become COLUMN_DROPPED
// and unmatched side-B columns COLUMN_ADDED.
func diffColumnsIn(colsA, colsB []schemasnapshot.Column, dbTypeA, dbTypeB string) []Difference {
	keyA, keyB := chooseMatchKeys(dbTypeA, dbTypeB, colsA, colsB,
		idOfColumn, schemasnapshot.Column.ForKey)

	return matchByKey(colsA, colsB, keyA, keyB,
		compareMatchedColumns(dbTypeA), emitColumnDropped(dbTypeA), emitColumnAdded(dbTypeB))
}

// idOfColumn extracts a column's stable ID ("{tableOID}:{attnum}") for ID-based matching.
func idOfColumn(c schemasnapshot.Column) string { return c.ID }

// columnObject renders a column's identity (schema.table.column) for a finding.
func columnObject(c schemasnapshot.Column, dbType string) QualifiedObject {
	return QualifiedObject{Key: c.ForKey(dbType), Display: c.ForDisplay(dbType)}
}

// emitColumnDropped returns the matchByKey onDropped callback for columns: it
// emits a COLUMN_DROPPED finding carrying the dropped column, anchored to its
// parent table.
func emitColumnDropped(dbType string) func(schemasnapshot.Column) []Difference {
	return func(c schemasnapshot.Column) []Difference {
		return []Difference{newDifference(ColumnDropped, columnObject(c, dbType), &c.Table, c, nil)}
	}
}

// emitColumnAdded returns the matchByKey onAdded callback for columns: it emits a
// COLUMN_ADDED finding carrying the added column, anchored to its parent table.
func emitColumnAdded(dbType string) func(schemasnapshot.Column) []Difference {
	return func(c schemasnapshot.Column) []Difference {
		return []Difference{newDifference(ColumnAdded, columnObject(c, dbType), &c.Table, nil, c)}
	}
}

// compareMatchedColumns returns the matchByKey onMatch callback for columns: it
// emits all field-level differences for a pair of columns matched by
// chooseMatchKeys (ID or composite name key). Object in each Difference is
// always the side-A (old) column, rendered in dbType.
func compareMatchedColumns(dbType string) func(cA, cB schemasnapshot.Column) []Difference {
	return func(cA, cB schemasnapshot.Column) []Difference {
		var diffs []Difference
		obj := columnObject(cA, dbType)

		if cA.Name != cB.Name {
			diffs = append(diffs, newDifference(ColumnNameChanged, obj, &cA.Table, cA.Name, cB.Name))
		}

		if cA.DataType != cB.DataType {
			diffs = append(diffs, newDifference(ColumnTypeChanged, obj, &cA.Table, cA.DataType, cB.DataType))
		}

		if cA.NotNull != cB.NotNull {
			diffs = append(diffs, newDifference(ColumnNullabilityChanged, obj, &cA.Table, cA.NotNull, cB.NotNull))
		}

		if cA.Default != cB.Default {
			diffs = append(diffs, newDifference(ColumnDefaultChanged, obj, &cA.Table, cA.Default, cB.Default))
		}

		return diffs
	}
}
