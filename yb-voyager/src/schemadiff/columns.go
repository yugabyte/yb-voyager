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
// by stable "{tableOID}:{attnum}" ID for a same-engine Postgres/YugabyteDB diff
// so a rename surfaces as COLUMN_NAME_CHANGED, else by the unquoted dot-joined
// composite key schema.table.col (see Column.ForKey and its dot-collision
// limitation). Unmatched side-A columns become COLUMN_DROPPED and unmatched
// side-B columns COLUMN_ADDED.
func diffColumnsIn(colsA, colsB []schemasnapshot.Column, dbTypeA, dbTypeB string) []Difference {
	keyA, keyB := chooseMatchKeys(dbTypeA, dbTypeB,
		idOfColumn, schemasnapshot.Column.ForKey)

	return matchByKey(colsA, colsB, keyA, keyB,
		compareMatchedColumns, emitColumnDropped, emitColumnAdded)
}

// idOfColumn extracts a column's stable ID ("{tableOID}:{attnum}") for ID-based matching.
func idOfColumn(c schemasnapshot.Column) string { return c.ID }

// emitColumnDropped is the matchByKey onDropped callback for columns: it emits a
// COLUMN_DROPPED finding carrying the dropped column, identified by its
// TableScopedRef.
func emitColumnDropped(c schemasnapshot.Column) []Difference {
	return []Difference{newDifference(ColumnDropped, OpDropped, ObjectTypeColumn, AttrNone, c.TableScopedRef, nil, c, nil)}
}

// emitColumnAdded is the matchByKey onAdded callback for columns: it emits a
// COLUMN_ADDED finding carrying the added column, identified by its TableScopedRef.
func emitColumnAdded(c schemasnapshot.Column) []Difference {
	return []Difference{newDifference(ColumnAdded, OpAdded, ObjectTypeColumn, AttrNone, nil, c.TableScopedRef, nil, c)}
}

// compareMatchedColumns is the matchByKey onMatch callback for columns: it emits
// all field-level differences for a pair of columns matched by chooseMatchKeys
// (ID or composite name key). ObjectA/ObjectB are the side-A/side-B columns'
// TableScopedRef identities.
func compareMatchedColumns(cA, cB schemasnapshot.Column) []Difference {
	var diffs []Difference

	if cA.Name != cB.Name {
		diffs = append(diffs, newDifference(ColumnNameChanged, OpChanged, ObjectTypeColumn, AttrName, cA.TableScopedRef, cB.TableScopedRef, cA.Name, cB.Name))
	}

	if cA.DataType != cB.DataType {
		diffs = append(diffs, newDifference(ColumnTypeChanged, OpChanged, ObjectTypeColumn, AttrType, cA.TableScopedRef, cB.TableScopedRef, cA.DataType, cB.DataType))
	}

	if cA.NotNull != cB.NotNull {
		diffs = append(diffs, newDifference(ColumnNullabilityChanged, OpChanged, ObjectTypeColumn, AttrNullability, cA.TableScopedRef, cB.TableScopedRef, cA.NotNull, cB.NotNull))
	}

	if cA.Default != cB.Default {
		diffs = append(diffs, newDifference(ColumnDefaultChanged, OpChanged, ObjectTypeColumn, AttrDefault, cA.TableScopedRef, cB.TableScopedRef, cA.Default, cB.Default))
	}

	return diffs
}
