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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
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
	withKey.UniqueKeys = []schemasnapshot.Constraint{*makePrimaryKey("public", "Orders", "11", "Orders_Email_key", `UNIQUE ("Email")`)}
	withKey.Checks = []schemasnapshot.Constraint{*makePrimaryKey("public", "Orders", "12", "Orders_Qty_check", `CHECK ("Qty" > 0)`)}
	withKey.ForeignKeys = []schemasnapshot.Constraint{*makePrimaryKey("public", "Orders", "13", "Orders_Cust_fkey", `FOREIGN KEY ("Cust") REFERENCES "Sales"."Customers"(id)`)}
	withoutKey := tableWithPrimaryKey(nil)

	snapshot := func(version int, table schemasnapshot.Table) *schemasnapshot.SnapshotContent {
		s := snapWithTables(table)
		s.Version = version
		return s
	}

	type finding struct {
		Type   DiffType
		Object string
	}
	tests := []struct {
		name               string
		versionA, versionB int
		want               []finding
	}{
		{"v1 to v2 skips keys", 1, schemasnapshot.VersionWithKeysAndIndexes, nil},
		{"v2 to v1 skips keys", schemasnapshot.VersionWithKeysAndIndexes, 1, nil},
		{"v2 to v2 compares keys", schemasnapshot.VersionWithKeysAndIndexes, schemasnapshot.VersionWithKeysAndIndexes, []finding{
			{PrimaryKeyAdded, "public.Orders.Orders_pkey"},
			{UniqueAdded, "public.Orders.Orders_Email_key"},
			{CheckAdded, "public.Orders.Orders_Qty_check"},
			{ForeignKeyAdded, "public.Orders.Orders_Cust_fkey"},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []finding
			for _, d := range Diff(snapshot(tc.versionA, withoutKey), snapshot(tc.versionB, withKey)) {
				got = append(got, finding{d.Type, identKey(d, constants.POSTGRESQL)})
			}
			assert.ElementsMatch(t, tc.want, got)
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

func TestDiffConstraintsOrIndexes(t *testing.T) {
	newKey := func(id, name, definition string) schemasnapshot.Constraint {
		return *makePrimaryKey("public", "Orders", id, name, definition)
	}
	parentCopy := func(c schemasnapshot.Constraint) schemasnapshot.Constraint {
		c.FromParent = true
		return c
	}
	emailKey := newKey("10", "Orders_Email_key", `UNIQUE ("Email")`)
	renamedEmailKey := newKey("10", "orders_email_uq", `UNIQUE ("Email")`)
	skuKey := newKey("11", "Orders_Sku_key", `UNIQUE ("Sku")`)
	recreatedEmailKey := newKey("20", "Orders_Email_key", `UNIQUE ("Email")`)

	dropped := func(c schemasnapshot.Constraint) Difference {
		return Difference{Type: UniqueDropped, Operation: OpDropped, ObjectType: ObjectTypeConstraint, ObjectA: c.TableScopedObjectRef, SideAValue: c.Definition}
	}
	added := func(c schemasnapshot.Constraint) Difference {
		return Difference{Type: UniqueAdded, Operation: OpAdded, ObjectType: ObjectTypeConstraint, ObjectB: c.TableScopedObjectRef, SideBValue: c.Definition}
	}
	renamed := Difference{Type: UniqueNameChanged, Operation: OpChanged, ObjectType: ObjectTypeConstraint, Attribute: AttrName,
		ObjectA: emailKey.TableScopedObjectRef, ObjectB: renamedEmailKey.TableScopedObjectRef, SideAValue: "Orders_Email_key", SideBValue: "orders_email_uq"}

	tests := []struct {
		name           string
		earlier, later []schemasnapshot.Constraint
		want           []Difference
	}{
		{"both empty", nil, nil, nil},
		{"unchanged", []schemasnapshot.Constraint{emailKey}, []schemasnapshot.Constraint{emailKey}, nil},
		{"renamed", []schemasnapshot.Constraint{emailKey}, []schemasnapshot.Constraint{renamedEmailKey}, []Difference{renamed}},
		{"dropped", []schemasnapshot.Constraint{emailKey}, nil, []Difference{dropped(emailKey)}},
		{"added", nil, []schemasnapshot.Constraint{emailKey}, []Difference{added(emailKey)}},
		{"new OID under the same name reports a drop and an add", []schemasnapshot.Constraint{emailKey}, []schemasnapshot.Constraint{recreatedEmailKey}, []Difference{dropped(emailKey), added(recreatedEmailKey)}},
		{"same OID with a copy on either side reports nothing", []schemasnapshot.Constraint{emailKey}, []schemasnapshot.Constraint{parentCopy(renamedEmailKey)}, nil},
		{"copy on side A only is ignored", []schemasnapshot.Constraint{parentCopy(emailKey)}, nil, nil},
		{"copy on side B only is ignored", nil, []schemasnapshot.Constraint{parentCopy(emailKey)}, nil},
		{"several constraints: rename, drop and add in input order",
			[]schemasnapshot.Constraint{emailKey, skuKey},
			[]schemasnapshot.Constraint{renamedEmailKey, newKey("12", "Orders_Zip_key", `UNIQUE ("Zip")`)},
			[]Difference{renamed, dropped(skuKey), added(newKey("12", "Orders_Zip_key", `UNIQUE ("Zip")`))}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, diffConstraintsOrIndexes(tc.earlier, tc.later, constraintRef, uniqueTypes))
		})
	}

	t.Run("check types carry their own DiffTypes", func(t *testing.T) {
		got := diffConstraintsOrIndexes(nil, []schemasnapshot.Constraint{emailKey}, constraintRef, checkTypes)
		assert.Equal(t, CheckAdded, got[0].Type)
	})

	t.Run("foreign key types carry their own DiffTypes", func(t *testing.T) {
		got := diffConstraintsOrIndexes(nil, []schemasnapshot.Constraint{emailKey}, constraintRef, foreignKeyTypes)
		assert.Equal(t, ForeignKeyAdded, got[0].Type)
	})
}

func TestDiffConstraintsOrIndexes_ForeignKeys(t *testing.T) {
	newFK := func(id, name, definition string) schemasnapshot.Constraint {
		return *makePrimaryKey("public", "Orders", id, name, definition)
	}
	parentCopy := func(c schemasnapshot.Constraint) schemasnapshot.Constraint {
		c.FromParent = true
		return c
	}
	ownFK := newFK("10", "Orders_Customer_fkey", `FOREIGN KEY ("Customer_Id") REFERENCES "Sales"."Customers"(id) ON DELETE CASCADE`)
	copyA := parentCopy(newFK("11", "Orders_Customer_fkey", `FOREIGN KEY ("Customer_Id") REFERENCES "Sales"."Customers_p1"(id)`))
	copyB := parentCopy(newFK("12", "Orders_Customer_fkey", `FOREIGN KEY ("Customer_Id") REFERENCES "Sales"."Customers_p2"(id)`))
	copyC := parentCopy(newFK("13", "Orders_Customer_fkey", `FOREIGN KEY ("Customer_Id") REFERENCES "Sales"."Customers_p3"(id)`))

	t.Run("copies added and dropped next to an own foreign key report nothing", func(t *testing.T) {
		earlier := []schemasnapshot.Constraint{ownFK, copyA, copyB}
		later := []schemasnapshot.Constraint{ownFK, copyB, copyC}
		assert.Empty(t, diffConstraintsOrIndexes(earlier, later, constraintRef, foreignKeyTypes))
	})

	t.Run("an own foreign key is still reported amid copies", func(t *testing.T) {
		got := diffConstraintsOrIndexes([]schemasnapshot.Constraint{copyA}, []schemasnapshot.Constraint{ownFK, copyB}, constraintRef, foreignKeyTypes)
		want := []Difference{{Type: ForeignKeyAdded, Operation: OpAdded, ObjectType: ObjectTypeConstraint, ObjectB: ownFK.TableScopedObjectRef, SideBValue: ownFK.Definition}}
		assert.Equal(t, want, got)
	})
}
