//go:build unit

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

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

func makePrimaryKey(schema, table, id, name, definition string) *schemasnapshot.Constraint {
	return &schemasnapshot.Constraint{
		TableScopedObjectRef: schemasnapshot.TableScopedObjectRef{Table: ref(schema, table), Name: name},
		ID:                   id,
		Definition:           definition,
	}
}

func fromParent(c *schemasnapshot.Constraint) *schemasnapshot.Constraint {
	if c == nil {
		return nil
	}
	copied := *c
	copied.FromParent = true
	return &copied
}

func tableWithPrimaryKey(key *schemasnapshot.Constraint) schemasnapshot.Table {
	t := makeTable("1", "public", "Orders", schemasnapshot.TableKindOrdinary)
	t.PrimaryKey = key
	return t
}

func TestDiffPrimaryKey(t *testing.T) {
	pk := makePrimaryKey("public", "Orders", "10", "Orders_pkey", `PRIMARY KEY ("Id")`)
	renamed := makePrimaryKey("public", "Orders", "10", "Orders_pk2", `PRIMARY KEY ("Id")`)
	recreated := makePrimaryKey("public", "Orders", "20", "Orders_pkey", `PRIMARY KEY ("Id")`)

	dropped := func(key *schemasnapshot.Constraint) Difference {
		return Difference{Type: PrimaryKeyDropped, Operation: OpDropped, ObjectType: ObjectTypeConstraint, ObjectA: key.TableScopedObjectRef, SideAValue: key.Definition}
	}
	added := func(key *schemasnapshot.Constraint) Difference {
		return Difference{Type: PrimaryKeyAdded, Operation: OpAdded, ObjectType: ObjectTypeConstraint, ObjectB: key.TableScopedObjectRef, SideBValue: key.Definition}
	}
	nameChanged := Difference{Type: PrimaryKeyNameChanged, Operation: OpChanged, ObjectType: ObjectTypeConstraint, Attribute: AttrName,
		ObjectA: pk.TableScopedObjectRef, ObjectB: renamed.TableScopedObjectRef, SideAValue: "Orders_pkey", SideBValue: "Orders_pk2"}

	tests := []struct {
		name string
		a, b *schemasnapshot.Constraint
		want []Difference
	}{
		{"both none", nil, nil, nil},
		{"unchanged", pk, pk, nil},
		{"added", nil, pk, []Difference{added(pk)}},
		{"dropped", pk, nil, []Difference{dropped(pk)}},
		{"renamed", pk, renamed, []Difference{nameChanged}},
		{"new OID on both sides reports both", pk, recreated, []Difference{dropped(pk), added(recreated)}},
		{"copy on side B only is ignored", nil, fromParent(pk), nil},
		{"copy on side A only is ignored", fromParent(pk), nil, nil},
		{"attach: copy to own with same OID", fromParent(pk), pk, nil},
		{"detach: own to copy with same OID", pk, fromParent(pk), nil},
		{"copies on both sides with a rename", fromParent(pk), fromParent(renamed), nil},
		{"own renamed against a copy", pk, fromParent(renamed), nil},
		{"own dropped when a copy with a new OID replaces it", pk, fromParent(recreated), []Difference{dropped(pk)}},
		{"own added when it replaces a copy with a new OID", fromParent(pk), recreated, []Difference{added(recreated)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := diffConstraintsOrIndexes(primaryKeyAsSlice(tc.a), primaryKeyAsSlice(tc.b), constraintRef, primaryKeyTypes)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDiff_KeysCompareOnlyWhenBothSnapshotsHaveKeys(t *testing.T) {
	pk := makePrimaryKey("public", "Orders", "10", "Orders_pkey", `PRIMARY KEY ("Id")`)
	withKey := tableWithPrimaryKey(pk)
	withoutKey := tableWithPrimaryKey(nil)

	snapshot := func(version int, table schemasnapshot.Table) *schemasnapshot.SnapshotContent {
		s := snapWithTables(table)
		s.Version = version
		return s
	}

	tests := []struct {
		name               string
		versionA, versionB int
		want               []DiffType
	}{
		{"v1 to v2 skips keys", 1, schemasnapshot.VersionWithKeysAndIndexes, nil},
		{"v2 to v1 skips keys", schemasnapshot.VersionWithKeysAndIndexes, 1, nil},
		{"v2 to v2 compares keys", schemasnapshot.VersionWithKeysAndIndexes, schemasnapshot.VersionWithKeysAndIndexes, []DiffType{PrimaryKeyAdded}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []DiffType
			for _, d := range Diff(snapshot(tc.versionA, withoutKey), snapshot(tc.versionB, withKey)) {
				got = append(got, d.Type)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFilterByScope_PrimaryKeyFindingAnchorsToItsTable(t *testing.T) {
	pk := makePrimaryKey("public", "Orders", "10", "Orders_pkey", `PRIMARY KEY ("Id")`)
	a := snapWithTables(tableWithPrimaryKey(nil))
	b := snapWithTables(tableWithPrimaryKey(pk))
	a.Version = schemasnapshot.VersionWithKeysAndIndexes
	b.Version = schemasnapshot.VersionWithKeysAndIndexes
	diffs := Diff(a, b)

	scope := func(table schemasnapshot.ObjectRef, types ...ObjectType) Scope {
		return Scope{Schemas: []string{"public"}, Tables: []schemasnapshot.ObjectRef{table}, ObjectTypes: types}
	}
	assert.Len(t, FilterByScope(diffs, scope(ref("public", "Orders"), ObjectTypeConstraint)), 1)
	assert.Empty(t, FilterByScope(diffs, scope(ref("public", "Other"), ObjectTypeConstraint)))
	assert.Empty(t, FilterByScope(diffs, scope(ref("public", "Orders"), ObjectTypeTable)))
}
