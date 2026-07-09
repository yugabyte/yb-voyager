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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// Note: ref(schema, name) is already declared in diff_test.go (same package).

// objOf renders a table-level finding's Object from its ObjectRef, using the
// postgresql dialect (all filter_test.go fixtures model PG snapshots).
func objOf(o schemasnapshot.ObjectRef) QualifiedObject {
	return QualifiedObject{Key: o.ForKey(constants.POSTGRESQL), Display: o.ForDisplay(constants.POSTGRESQL)}
}

// colObj renders a column-level finding's Object as table.column, using the
// postgresql dialect.
func colObj(table schemasnapshot.ObjectRef, column string) QualifiedObject {
	return QualifiedObject{Key: objOf(table).Key + "." + column}
}

// tableDiff builds a Difference anchored to a table (AnchorTable == Object).
func tableDiff(dt DiffType, schema, name string) Difference {
	o := ref(schema, name)
	return Difference{Type: dt, Object: objOf(o), AnchorTable: &o}
}

// nilAnchorDiff builds a Difference with a nil AnchorTable, using a kept
// DiffType. This is used to test filter behaviour for nil-AnchorTable findings
// without relying on removed object types.
func nilAnchorDiff(dt DiffType, schema, name string) Difference {
	return Difference{Type: dt, Object: objOf(ref(schema, name)), AnchorTable: nil}
}

// nameChangedDiff builds a *_NAME_CHANGED finding with the given old and new names.
// AnchorTable is set to the old ObjectRef (same as Object).
func nameChangedDiff(dt DiffType, schema, oldName, newName string) Difference {
	o := ref(schema, oldName)
	return Difference{
		Type:        dt,
		Object:      objOf(o),
		AnchorTable: &o,
		OldValue:    oldName,
		NewValue:    newName,
	}
}

// collectTypes returns the set of DiffType values from a []Difference.
func collectTypes(diffs []Difference) map[DiffType]bool {
	m := make(map[DiffType]bool, len(diffs))
	for _, d := range diffs {
		m[d.Type] = true
	}
	return m
}

// ─── DiffType coverage ───────────────────────────────────────────────────────

// TestDiffTypeDefsRegistryIsExhaustive asserts that every DiffType constant
// declared in difftypes.go appears in the diffTypeDefs registry. If a new
// constant is added to difftypes.go without a corresponding registry entry,
// this test fails.
//
// The set of all DiffType constants is enumerated manually here because Go has
// no reflection-level "list all constants of type T" API; the list below must
// stay in sync with difftypes.go.
func TestDiffTypeDefsRegistryIsExhaustive(t *testing.T) {
	// allDiffTypes must list every DiffType constant from difftypes.go.
	// Only the 15 V1-emitted constants (tables + columns) are declared.
	// Keeping this in sync is enforced by the test itself: a constant absent
	// from this slice AND absent from diffTypeDefs passes silently — the
	// failure only fires when a constant is here but not in the registry.
	// Developers should add to this slice whenever they add to difftypes.go.
	allDiffTypes := []DiffType{
		// TABLE (9 constants)
		TableAdded, TableDropped, TableNameChanged, TableSchemaChanged,
		TableKindChanged, TablePartitionParentChanged, TablePartitionChildrenChanged,
		TableInheritsChanged, TableInheritedByChanged,
		// COLUMN (6 constants)
		ColumnAdded, ColumnDropped, ColumnNameChanged, ColumnTypeChanged,
		ColumnNullabilityChanged, ColumnDefaultChanged,
	}

	// Guard: the two lists must have the same length. If they diverge someone
	// added a DiffType to one place but not the other.
	assert.Equal(t, len(allDiffTypes), len(diffTypeDefs),
		"allDiffTypes and diffTypeDefs have different lengths — when adding a DiffType constant, add it to BOTH the allDiffTypes slice in this test AND the diffTypeDefs registry in difftypes.go")

	for _, dt := range allDiffTypes {
		_, ok := diffTypeDefs[dt]
		assert.True(t, ok, "DiffType %q is missing from diffTypeDefs — add an entry in difftypes.go", dt)
	}
}

// ─── Empty scope ─────────────────────────────────────────────────────────────

// TestFilterByScopeEmptyScopeKeepsEverything verifies that an empty Scope
// (all lists nil/empty) passes every finding through unchanged.
func TestFilterByScopeEmptyScopeKeepsEverything(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		tableDiff(TableDropped, "public", "legacy"),
		tableDiff(ColumnAdded, "public", "orders"),
		tableDiff(ColumnDropped, "public", "orders"),
	}

	got := FilterByScope(diffs, Scope{})
	assert.Equal(t, diffs, got)
}

// ─── Purity ───────────────────────────────────────────────────────────────────

// TestFilterByScopeIsPure verifies that FilterByScope does not mutate the
// input slice or the Scope value, and that the returned slice is independent.
func TestFilterByScopeIsPure(t *testing.T) {
	orders := ref("public", "orders")
	orig := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		// A finding with nil AnchorTable to verify it is excluded by a non-empty
		// Tables filter without panicking. We use a kept DiffType here.
		{Type: ColumnAdded, Object: colObj(orders, "x"), AnchorTable: nil},
	}
	// Make a copy of the originals' JSON to compare after the call.
	origJSON, err := json.Marshal(orig)
	require.NoError(t, err)

	scope := Scope{IncludeObjectTypes: []ObjectType{ObjectTypeTable}}
	got := FilterByScope(orig, scope)

	// Input slice must be unchanged.
	afterJSON, err := json.Marshal(orig)
	require.NoError(t, err)
	assert.JSONEq(t, string(origJSON), string(afterJSON), "FilterByScope must not mutate the input slice")

	// Returned slice must be a new allocation (both findings are ObjectTypeTable,
	// so both survive the include filter).
	require.Len(t, got, 2)
	assert.Equal(t, TableAdded, got[0].Type)

	// Mutating the returned slice must not affect the input.
	got[0].Type = TableDropped
	assert.Equal(t, TableAdded, orig[0].Type, "mutation of returned slice must not affect input")
}

// ─── ObjectTypes include filter ───────────────────────────────────────────────

// TestFilterByScopeObjectTypeInclude verifies that only findings whose bucket
// is listed in ObjectTypes are kept. In V1 all emitted DiffTypes map to
// ObjectTypeTable, so a TABLE filter keeps everything.
func TestFilterByScopeObjectTypeInclude(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		tableDiff(TableDropped, "public", "legacy"),
		tableDiff(ColumnAdded, "public", "orders"),
		tableDiff(ColumnTypeChanged, "public", "orders"),
	}

	// TABLE filter keeps all V1 findings (all map to ObjectTypeTable).
	gotTable := FilterByScope(diffs, Scope{IncludeObjectTypes: []ObjectType{ObjectTypeTable}})
	assert.Len(t, gotTable, len(diffs), "TABLE filter must keep all V1 findings")
}

// ─── ObjectTypes exclude filter ───────────────────────────────────────────────

// TestFilterByScopeObjectTypeExclude verifies that findings in excluded buckets
// are dropped. In V1 all findings map to ObjectTypeTable, so excluding TABLE
// drops everything.
func TestFilterByScopeObjectTypeExclude(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		tableDiff(ColumnAdded, "public", "orders"),
		tableDiff(TableDropped, "public", "legacy"),
	}

	// Excluding ObjectTypeTable drops all V1 findings.
	gotExcludeTable := FilterByScope(diffs, Scope{ExcludeObjectTypes: []ObjectType{ObjectTypeTable}})
	assert.Empty(t, gotExcludeTable, "excluding TABLE must drop all V1 findings")
}

// ─── Tables include filter ────────────────────────────────────────────────────

// TestFilterByScopeTableInclude verifies that only findings whose AnchorTable
// is in the Tables list are kept. A nil-AnchorTable finding is dropped by a
// non-empty Tables filter.
func TestFilterByScopeTableInclude(t *testing.T) {
	orders := ref("public", "orders")
	customers := ref("public", "customers")

	diffs := []Difference{
		{Type: TableAdded, Object: objOf(orders), AnchorTable: &orders},
		{Type: ColumnAdded, Object: colObj(orders, "id"), AnchorTable: &orders},
		{Type: TableAdded, Object: objOf(customers), AnchorTable: &customers},
		{Type: ColumnAdded, Object: colObj(customers, "name"), AnchorTable: &customers},
		// A synthetic nil-AnchorTable finding using a kept DiffType, to verify
		// that nil-anchored entries are dropped by a Tables include filter.
		{Type: TableNameChanged, Object: objOf(orders), AnchorTable: nil},
	}

	got := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "orders")}})

	// Only orders-anchored findings should survive.
	for _, d := range got {
		if d.AnchorTable != nil {
			assert.Equal(t, "public.orders", d.AnchorTable.ForDisplay(constants.POSTGRESQL),
				"only public.orders-anchored findings should pass")
		} else {
			t.Errorf("nil-AnchorTable finding should have been dropped by Tables filter: %v", d)
		}
	}
	assert.Len(t, got, 2, "expect TableAdded and ColumnAdded for orders only")
}

// ─── Tables exclude filter ────────────────────────────────────────────────────

// TestFilterByScopeTableExclude verifies that findings whose AnchorTable is in
// ExcludeTables are dropped, and nil-AnchorTable findings survive.
func TestFilterByScopeTableExclude(t *testing.T) {
	orders := ref("public", "orders")
	customers := ref("public", "customers")

	diffs := []Difference{
		{Type: TableAdded, Object: objOf(orders), AnchorTable: &orders},
		{Type: TableAdded, Object: objOf(customers), AnchorTable: &customers},
		// Synthetic nil-AnchorTable finding using a kept DiffType.
		// The ExcludeTables filter must NOT drop it (nil anchor is never excluded).
		nilAnchorDiff(TableNameChanged, "public", "some_obj"),
	}

	got := FilterByScope(diffs, Scope{ExcludeTables: []schemasnapshot.ObjectRef{ref("public", "orders")}})

	gotTypes := collectTypes(got)
	assert.False(t, func() bool {
		for _, d := range got {
			if d.AnchorTable != nil && d.AnchorTable.ForDisplay(constants.POSTGRESQL) == "public.orders" {
				return true
			}
		}
		return false
	}(), "public.orders-anchored findings should be excluded")
	assert.True(t, gotTypes[TableAdded], "public.customers TableAdded should remain")
	assert.True(t, gotTypes[TableNameChanged], "nil-AnchorTable finding should not be dropped by ExcludeTables")
	assert.Len(t, got, 2)
}

// ─── nil AnchorTable findings ─────────────────────────────────────────────────

// TestFilterByScopeNilAnchorDroppedByTables verifies that a finding with a nil
// AnchorTable is dropped when Tables is non-empty, but passes through an empty
// Scope (no filter applied). All findings use kept DiffTypes; their AnchorTable
// is set to nil synthetically to exercise the nil-anchor code path.
func TestFilterByScopeNilAnchorDroppedByTables(t *testing.T) {
	// Three synthetic nil-AnchorTable findings using kept DiffTypes.
	diffs := []Difference{
		nilAnchorDiff(TableAdded, "public", "t1"),
		nilAnchorDiff(ColumnAdded, "public", "t2"),
		nilAnchorDiff(TableDropped, "public", "t3"),
	}

	// Non-empty Tables list: nil-AnchorTable findings must be dropped.
	got := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "orders")}})
	assert.Empty(t, got, "nil-AnchorTable findings must be dropped when Tables is non-empty")

	// Empty scope: all pass through (nil-AnchorTable is not dropped by ObjectTypes alone).
	got2 := FilterByScope(diffs, Scope{})
	assert.Len(t, got2, 3, "nil-AnchorTable findings pass through an empty Scope")

	// TABLE object-type filter: all three pass (they all map to ObjectTypeTable).
	got3 := FilterByScope(diffs, Scope{IncludeObjectTypes: []ObjectType{ObjectTypeTable}})
	assert.Len(t, got3, 3, "nil-AnchorTable findings with TABLE DiffType pass ObjectTypeTable filter")
}

// ─── Either-side NAME_CHANGED rule ───────────────────────────────────────────

// TestFilterByScopeTableNameChangedOldNameInScope verifies that a TABLE_NAME_CHANGED
// finding is kept when only the OLD name is in Tables.
func TestFilterByScopeTableNameChangedOldNameInScope(t *testing.T) {
	// TABLE_NAME_CHANGED: old name "orders", new name "purchase_orders".
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "orders")}})
	assert.Len(t, got, 1, "TABLE_NAME_CHANGED should be kept when old name is in Tables")
}

// TestFilterByScopeTableNameChangedNewNameInScope verifies that a TABLE_NAME_CHANGED
// finding is kept when only the NEW name is in Tables.
func TestFilterByScopeTableNameChangedNewNameInScope(t *testing.T) {
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "purchase_orders")}})
	assert.Len(t, got, 1, "TABLE_NAME_CHANGED should be kept when new name is in Tables")
}

// TestFilterByScopeTableNameChangedNeitherNameInScope verifies that a TABLE_NAME_CHANGED
// finding is dropped when neither the old nor the new name appears in Tables.
func TestFilterByScopeTableNameChangedNeitherNameInScope(t *testing.T) {
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "customers")}})
	assert.Empty(t, got, "TABLE_NAME_CHANGED should be dropped when neither name is in Tables")
}

// ─── Anchor-rename extension ─────────────────────────────────────────────────

// TestFilterByScopeAnchorRenameExtension verifies that when a table is renamed
// (TABLE_NAME_CHANGED), findings anchored to either the old or the new name are
// included when either name appears in the Tables list.
func TestFilterByScopeAnchorRenameExtension(t *testing.T) {
	// TABLE_NAME_CHANGED: old "orders", new "purchase_orders".
	rename := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")
	// A column change anchored to the OLD table name (as the diff engine would emit it).
	oldAnchor := ref("public", "orders")
	colChange := Difference{
		Type:        ColumnTypeChanged,
		Object:      colObj(oldAnchor, "amount"),
		AnchorTable: &oldAnchor,
		OldValue:    "integer",
		NewValue:    "bigint",
	}

	diffs := []Difference{rename, colChange}

	// Filtering by the NEW name should include both: the rename itself and
	// the column change whose anchor is the old name.
	got := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "purchase_orders")}})
	assert.Len(t, got, 2, "rename + column change should both be kept when new name is in Tables")
}

// TestFilterByScopeAnchorRenameExtensionExclude verifies that when a table is
// renamed and a finding is anchored to the old name, ExcludeTables with the
// new name still drops the finding (either-side rule applies to excludes too).
func TestFilterByScopeAnchorRenameExtensionExclude(t *testing.T) {
	rename := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")
	oldAnchor := ref("public", "orders")
	colChange := Difference{
		Type:        ColumnTypeChanged,
		Object:      colObj(oldAnchor, "amount"),
		AnchorTable: &oldAnchor,
	}

	diffs := []Difference{rename, colChange}

	// Excluding by the NEW name should drop findings whose anchor is the OLD name
	// (because they are aliases via the rename map).
	got := FilterByScope(diffs, Scope{ExcludeTables: []schemasnapshot.ObjectRef{ref("public", "purchase_orders")}})
	assert.Empty(t, got, "findings anchored to old name should be excluded when new name is in ExcludeTables")
}

// TestFilterByScopeAliasMapCollision verifies that two TABLE_NAME_CHANGED
// entries sharing a name (e.g. "users→customers" and "customers→clients")
// accumulate multiple aliases per name instead of the second rename
// overwriting the first.
//
// Scenario:
//
//	rename1: users        → customers
//	rename2: customers    → clients
//
// A column finding anchored to "users" must stay included when Tables contains
// "customers" (via rename1's alias), without being dropped by the map-key
// collision between the two renames.
func TestFilterByScopeAliasMapCollision(t *testing.T) {
	// Two renames where rename2's old name equals rename1's new name.
	rename1 := nameChangedDiff(TableNameChanged, "public", "users", "customers")
	rename2 := nameChangedDiff(TableNameChanged, "public", "customers", "clients")

	// A column change anchored to the original "users" name.
	usersRef := ref("public", "users")
	colChange := Difference{
		Type:        ColumnAdded,
		Object:      colObj(usersRef, "email"),
		AnchorTable: &usersRef,
	}

	diffs := []Difference{rename1, rename2, colChange}

	// Filtering by "customers" (rename1's new name) should keep all three:
	//   - rename1: anchor "users" aliases "customers" ✓
	//   - rename2: anchor "customers" direct match ✓
	//   - colChange: anchor "users" aliases "customers" ✓
	// The alias-map collision (both renames touching "customers" as a key) must
	// not drop rename1 or colChange.
	got := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "customers")}})
	gotTypes := collectTypes(got)
	assert.True(t, gotTypes[TableNameChanged], "rename findings should be kept — 'customers' is an anchor or alias")
	assert.True(t, gotTypes[ColumnAdded], "column change anchored to 'users' must NOT be dropped — 'users' aliases 'customers'")
	assert.Len(t, got, 3, "all three findings should survive filtering by 'customers'")

	// Filtering by "users" should include all three: rename1 and colChange
	// (direct anchor), and rename2 (anchor "customers" aliases "users" since
	// rename1 recorded the alias in both directions).
	got2 := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "users")}})
	got2Types := collectTypes(got2)
	assert.True(t, got2Types[TableNameChanged], "rename findings should be kept — 'users' is an anchor or alias")
	assert.True(t, got2Types[ColumnAdded], "colChange anchored to 'users' should be kept")
	assert.Len(t, got2, 3, "all three findings are reachable from 'users'")

	// Filtering by "clients" (rename2's new name) should keep rename2 only.
	// rename1 and colChange (anchor "users") must NOT be included:
	// aliases["public.users"] = ["public.customers"] only, not "public.clients".
	got3 := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "clients")}})
	got3Types := collectTypes(got3)
	assert.True(t, got3Types[TableNameChanged], "rename2 (customers→clients) should be kept when Tables=['public.clients']")
	assert.False(t, got3Types[ColumnAdded], "colChange anchored to 'users' must NOT be incorrectly included for 'clients'")
	assert.Len(t, got3, 1, "only rename2 should survive filtering by 'clients'")
}

// ─── Either-side rule across schema moves (SET SCHEMA) ───────────────────────

// schemaChangedDiff builds a TABLE_SCHEMA_CHANGED finding: a table moved from
// oldSchema to newSchema, keeping the same name. AnchorTable is the old ref,
// matching how compareMatchedTables emits it (Object/AnchorTable = side-A ref,
// OldValue/NewValue = the schema strings).
func schemaChangedDiff(oldSchema, name, newSchema string) Difference {
	o := ref(oldSchema, name)
	return Difference{
		Type:        TableSchemaChanged,
		Object:      objOf(o),
		AnchorTable: &o,
		OldValue:    oldSchema,
		NewValue:    newSchema,
	}
}

// TestFilterByScopeSchemaMoveNewIdentityInScope verifies the either-side rule
// for a table moved to a new schema (TABLE_SCHEMA_CHANGED). A finding anchored
// to the old schema-qualified identifier must be kept when the NEW one is in
// Tables, just as renames are kept by either name.
func TestFilterByScopeSchemaMoveNewIdentityInScope(t *testing.T) {
	// "old_s.orders" moved to "new_s.orders".
	move := schemaChangedDiff("old_s", "orders", "new_s")
	// A column change anchored to the OLD (schema, name).
	oldAnchor := ref("old_s", "orders")
	colChange := Difference{
		Type:        ColumnTypeChanged,
		Object:      colObj(oldAnchor, "amount"),
		AnchorTable: &oldAnchor,
		OldValue:    "integer",
		NewValue:    "bigint",
	}

	diffs := []Difference{move, colChange}

	// Filtering by the NEW schema-qualified name must keep BOTH the move finding
	// and the column change anchored to the old identifier.
	got := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("new_s", "orders")}})
	assert.Len(t, got, 2, "schema-move + column change should both be kept when the new identifier is in Tables")

	// Symmetric: filtering by the OLD identifier also keeps both.
	gotOld := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("old_s", "orders")}})
	assert.Len(t, gotOld, 2, "both findings should be kept when the old identifier is in Tables")
}

// TestFilterByScopeSchemaMoveExclude verifies the either-side rule applies to
// ExcludeTables for schema moves: excluding by the NEW identifier drops a
// finding anchored to the OLD identifier.
func TestFilterByScopeSchemaMoveExclude(t *testing.T) {
	move := schemaChangedDiff("old_s", "orders", "new_s")
	oldAnchor := ref("old_s", "orders")
	colChange := Difference{
		Type:        ColumnTypeChanged,
		Object:      colObj(oldAnchor, "amount"),
		AnchorTable: &oldAnchor,
	}

	diffs := []Difference{move, colChange}

	got := FilterByScope(diffs, Scope{ExcludeTables: []schemasnapshot.ObjectRef{ref("new_s", "orders")}})
	assert.Empty(t, got, "findings anchored to the old identifier should be excluded when the new identifier is in ExcludeTables")
}

// TestFilterByScopeRenameAndMove verifies the either-side rule when a table is
// BOTH renamed and moved in the same interval. compareMatchedTables emits two
// findings (TABLE_NAME_CHANGED and TABLE_SCHEMA_CHANGED) that share the same
// side-A anchor; the new identity is (newSchema, newName). The alias must be
// reconstructed from BOTH findings combined — not the old schema + new name.
func TestFilterByScopeRenameAndMove(t *testing.T) {
	// "old_s.orders" → "new_s.purchase_orders" (rename + move).
	oldAnchor := ref("old_s", "orders")
	rename := Difference{
		Type:        TableNameChanged,
		Object:      objOf(oldAnchor),
		AnchorTable: &oldAnchor,
		OldValue:    "orders",
		NewValue:    "purchase_orders",
	}
	move := Difference{
		Type:        TableSchemaChanged,
		Object:      objOf(oldAnchor),
		AnchorTable: &oldAnchor,
		OldValue:    "old_s",
		NewValue:    "new_s",
	}
	colChange := Difference{
		Type:        ColumnAdded,
		Object:      colObj(oldAnchor, "email"),
		AnchorTable: &oldAnchor,
	}

	diffs := []Difference{rename, move, colChange}

	// The true new identity is "new_s.purchase_orders": filtering by it keeps all three.
	got := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("new_s", "purchase_orders")}})
	assert.Len(t, got, 3, "rename+move + column change should all be kept when the true new identifier is in Tables")

	// The OLD identity "old_s.orders" keeps all three too (either-side).
	gotOld := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("old_s", "orders")}})
	assert.Len(t, gotOld, 3, "all three findings should be kept when the old identifier is in Tables")

	// The bogus "old schema + new name" identifier must NOT match anything —
	// it is not a real identity of this table on either side.
	gotBogus := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("old_s", "purchase_orders")}})
	assert.Empty(t, gotBogus, "the spurious old-schema+new-name identifier must not match — the table never had that identity")
}

// ─── Include-then-exclude interaction ────────────────────────────────────────

// TestFilterByScopeIncludeThenExclude verifies that the include filter runs
// before the exclude filter and that their interaction is correct. In V1 all
// DiffTypes map to ObjectTypeTable, so include TABLE keeps everything and then
// exclude TABLE drops everything.
func TestFilterByScopeIncludeThenExclude(t *testing.T) {
	orders := ref("public", "orders")
	customers := ref("public", "customers")

	diffs := []Difference{
		{Type: TableAdded, Object: objOf(orders), AnchorTable: &orders},
		{Type: TableAdded, Object: objOf(customers), AnchorTable: &customers},
		tableDiff(ColumnAdded, "public", "orders"),
	}

	// Include TABLE, then exclude TABLE — exclude wins, result is empty.
	gotBoth := FilterByScope(diffs, Scope{
		IncludeObjectTypes: []ObjectType{ObjectTypeTable},
		ExcludeObjectTypes: []ObjectType{ObjectTypeTable},
	})
	assert.Empty(t, gotBoth, "TABLE included then excluded — exclude wins, result must be empty")

	// Include TABLE only: all V1 findings pass.
	gotIncludeTable := FilterByScope(diffs, Scope{
		IncludeObjectTypes: []ObjectType{ObjectTypeTable},
	})
	assert.Len(t, gotIncludeTable, len(diffs), "TABLE include keeps all V1 findings")
}

// TestFilterByScopeIncludeTableExcludeTable verifies that when a table appears
// in both Tables and ExcludeTables, the exclude wins (exclude runs after include).
func TestFilterByScopeIncludeTableExcludeTable(t *testing.T) {
	orders := ref("public", "orders")
	customers := ref("public", "customers")

	diffs := []Difference{
		{Type: TableAdded, Object: objOf(orders), AnchorTable: &orders},
		{Type: TableAdded, Object: objOf(customers), AnchorTable: &customers},
	}

	// Include both, then exclude orders.
	got := FilterByScope(diffs, Scope{
		IncludeTables: []schemasnapshot.ObjectRef{ref("public", "orders"), ref("public", "customers")},
		ExcludeTables: []schemasnapshot.ObjectRef{ref("public", "orders")},
	})

	assert.Len(t, got, 1)
	assert.Equal(t, "public.customers", got[0].AnchorTable.ForDisplay(constants.POSTGRESQL))
}

// ─── Edge cases ───────────────────────────────────────────────────────────────

// TestFilterByScopeUnknownTableNameIsNoOp verifies that a Tables value that
// matches no finding is a silent no-op (no panic, empty result).
func TestFilterByScopeUnknownTableNameIsNoOp(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
	}
	got := FilterByScope(diffs, Scope{IncludeTables: []schemasnapshot.ObjectRef{ref("public", "nonexistent")}})
	assert.Empty(t, got)
}

// TestFilterByScopeEmptyInputReturnsEmpty verifies that an empty input slice
// always returns an empty (not nil) result.
func TestFilterByScopeEmptyInputReturnsEmpty(t *testing.T) {
	got := FilterByScope(nil, Scope{IncludeObjectTypes: []ObjectType{ObjectTypeTable}})
	assert.NotNil(t, got)
	assert.Empty(t, got)
}
