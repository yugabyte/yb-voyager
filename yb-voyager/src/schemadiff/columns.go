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

// diffColumns computes column-level differences between snapshots a and b.
// Columns are matched by ID only when BOTH conditions hold:
//  1. a.DatabaseType == b.DatabaseType — IDs are only comparable within the same
//     database engine; cross-type ID comparison is illegal.
//  2. Both snapshots declare StableIdentity=true — the capturing provider guarantees
//     that IDs are stable across captures.
//
// If either condition fails, all columns fall back to matching by the composite key
// Column.Table.String() + "." + Column.Name.
func diffColumns(a, b *schemasnapshot.SchemaSnapshot) []Difference {
	var diffs []Difference

	matchByID := a.DatabaseType == b.DatabaseType && a.StableIdentity && b.StableIdentity

	// Build maps: ID → Column (non-empty ID, when matchByID is true) and
	// compositeKey → Column (empty ID or matchByID=false).
	byIDA := make(map[string]schemasnapshot.Column)
	byIDB := make(map[string]schemasnapshot.Column)
	byNameA := make(map[string]schemasnapshot.Column)
	byNameB := make(map[string]schemasnapshot.Column)

	for _, c := range a.Columns {
		if matchByID && c.ID != "" {
			byIDA[c.ID] = c
		} else {
			byNameA[c.Table.String()+"."+c.Name] = c
		}
	}
	for _, c := range b.Columns {
		if matchByID && c.ID != "" {
			byIDB[c.ID] = c
		} else {
			byNameB[c.Table.String()+"."+c.Name] = c
		}
	}

	// Diff ID-matched columns.
	for id, cA := range byIDA {
		cB, ok := byIDB[id]
		if !ok {
			// Only in A → dropped
			objRef := cA.Table
			diffs = append(diffs, Difference{
				Type:        ColumnDropped,
				Object:      objRef,
				AnchorTable: &objRef,
				SubObject:   cA.Name,
				OldValue:    cA.DataType,
			})
			continue
		}
		diffs = append(diffs, compareMatchedColumns(cA, cB)...)
	}
	for id, cB := range byIDB {
		if _, ok := byIDA[id]; !ok {
			// Only in B → added
			objRef := cB.Table
			diffs = append(diffs, Difference{
				Type:        ColumnAdded,
				Object:      objRef,
				AnchorTable: &objRef,
				SubObject:   cB.Name,
				NewValue:    cB.DataType,
			})
		}
	}

	// Diff name-matched columns (ID-empty fallback).
	for key, cA := range byNameA {
		cB, ok := byNameB[key]
		if !ok {
			// Only in A → dropped
			objRef := cA.Table
			diffs = append(diffs, Difference{
				Type:        ColumnDropped,
				Object:      objRef,
				AnchorTable: &objRef,
				SubObject:   cA.Name,
				OldValue:    cA.DataType,
			})
			continue
		}
		diffs = append(diffs, compareMatchedColumns(cA, cB)...)
	}
	for key, cB := range byNameB {
		if _, ok := byNameA[key]; !ok {
			// Only in B → added
			objRef := cB.Table
			diffs = append(diffs, Difference{
				Type:        ColumnAdded,
				Object:      objRef,
				AnchorTable: &objRef,
				SubObject:   cB.Name,
				NewValue:    cB.DataType,
			})
		}
	}

	return diffs
}

// compareMatchedColumns emits all field-level differences for a pair of columns
// matched by ID (or composite key for the ID-empty fallback). Object in each
// Difference is always the side-A (old) Column.Table.
func compareMatchedColumns(cA, cB schemasnapshot.Column) []Difference {
	var diffs []Difference

	oldRef := cA.Table
	anchor := oldRef // copy; we take address of the copy below

	base := Difference{
		Object:      oldRef,
		AnchorTable: &anchor,
		SubObject:   cA.Name, // SubObject is the old (side-A) column name
	}

	if cA.Name != cB.Name {
		d := base
		d.Type = ColumnNameChanged
		d.Property = "name"
		d.OldValue = cA.Name
		d.NewValue = cB.Name
		diffs = append(diffs, d)
	}

	if cA.DataType != cB.DataType {
		d := base
		d.Type = ColumnTypeChanged
		d.Property = "data_type"
		d.OldValue = cA.DataType
		d.NewValue = cB.DataType
		diffs = append(diffs, d)
	}

	if cA.NotNull != cB.NotNull {
		d := base
		d.Type = ColumnNullabilityChanged
		d.Property = "not_null"
		d.OldValue = cA.NotNull
		d.NewValue = cB.NotNull
		diffs = append(diffs, d)
	}

	if cA.Default != cB.Default {
		d := base
		d.Type = ColumnDefaultChanged
		d.Property = "default"
		d.OldValue = cA.Default
		d.NewValue = cB.Default
		diffs = append(diffs, d)
	}

	// DO NOT diff Attrs (empty in v1).

	return diffs
}
