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

// tableDiff builds a TABLE-level Difference anchored to itself (ObjectA == ObjectB,
// both the table's ref). Used for non-rename table findings, where the table's
// identity doesn't change across sides.
func tableDiff(dt DiffType, schema, name string) Difference {
	o := ref(schema, name)
	return Difference{Type: dt, ObjectType: ObjectTypeTable, ObjectA: o, ObjectB: o}
}

// colDiff builds a COLUMN-level Difference anchored to its host table
// (ObjectA == ObjectB, both the column's TableScopedObjectRef). Used for non-rename
// column findings.
func colDiff(dt DiffType, table schemasnapshot.ObjectRef, column string) Difference {
	ts := schemasnapshot.TableScopedObjectRef{Table: table, Name: column}
	return Difference{Type: dt, ObjectType: ObjectTypeColumn, ObjectA: ts, ObjectB: ts}
}

// noAnchorDiff builds a Difference whose derived anchor is absent: its identity
// is a plain ObjectRef, but ObjectType is a placeholder non-table type ("VIEW" —
// a raw cast; no such constant is declared yet), so anchorTableOf returns
// ok=false. This replaces the old nil-AnchorTable synthetic findings used to
// exercise the "no table anchor" filter path (top-level objects like views/
// functions, not yet emitted by the diff engine).
func noAnchorDiff(dt DiffType, schema, name string) Difference {
	return Difference{Type: dt, ObjectType: ObjectType("VIEW"), ObjectA: ref(schema, name)}
}

// nameChangedDiff builds a TABLE_NAME_CHANGED finding with the given old and new
// names: ObjectA is the old ref, ObjectB is the new ref (same schema).
func nameChangedDiff(dt DiffType, schema, oldName, newName string) Difference {
	return Difference{
		Type:       dt,
		ObjectType: ObjectTypeTable,
		ObjectA:    ref(schema, oldName),
		ObjectB:    ref(schema, newName),
		SideAValue: oldName,
		SideBValue: newName,
	}
}

// schemaChangedDiff builds a TABLE_SCHEMA_CHANGED finding: a table moved from
// oldSchema to newSchema, keeping the same name. ObjectA is the old ref, ObjectB
// is the new ref, matching how compareMatchedTables emits it.
func schemaChangedDiff(oldSchema, name, newSchema string) Difference {
	return Difference{
		Type:       TableSchemaChanged,
		ObjectType: ObjectTypeTable,
		ObjectA:    ref(oldSchema, name),
		ObjectB:    ref(newSchema, name),
		SideAValue: oldSchema,
		SideBValue: newSchema,
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

// anchorDisplay renders a finding's derived anchor table for assertions; it
// panics (via require semantics in the caller) is avoided by returning "" when
// there is no anchor.
func anchorDisplay(d Difference) (string, bool) {
	anchor, ok := anchorTableOf(d)
	if !ok {
		return "", false
	}
	return anchor.ForDisplay(constants.POSTGRESQL), true
}

// ─── Empty scope ─────────────────────────────────────────────────────────────

// TestFilterByScopeEmptyScopeKeepsEverything verifies that an empty Scope
// (all lists nil/empty) passes every finding through unchanged.
func TestFilterByScopeEmptyScopeKeepsEverything(t *testing.T) {
	orders := ref("public", "orders")
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		tableDiff(TableDropped, "public", "legacy"),
		colDiff(ColumnAdded, orders, "email"),
		colDiff(ColumnDropped, orders, "phone"),
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
		// A column finding to verify the TABLE include filter drops it (it maps
		// to ObjectTypeColumn) without panicking.
		colDiff(ColumnAdded, orders, "x"),
	}
	// Make a copy of the originals' JSON to compare after the call.
	origJSON, err := json.Marshal(orig)
	require.NoError(t, err)

	scope := Scope{ObjectTypes: []ObjectType{ObjectTypeTable}}
	got := FilterByScope(orig, scope)

	// Input slice must be unchanged.
	afterJSON, err := json.Marshal(orig)
	require.NoError(t, err)
	assert.JSONEq(t, string(origJSON), string(afterJSON), "FilterByScope must not mutate the input slice")

	// Returned slice must be a new allocation. Only the TableAdded finding
	// survives the TABLE include filter — the ColumnAdded finding maps to
	// ObjectTypeColumn and is dropped.
	require.Len(t, got, 1)
	assert.Equal(t, TableAdded, got[0].Type)

	// Mutating the returned slice must not affect the input.
	got[0].Type = TableDropped
	assert.Equal(t, TableAdded, orig[0].Type, "mutation of returned slice must not affect input")
}

// ─── ObjectTypes include filter ───────────────────────────────────────────────

// TestFilterByScopeObjectTypeInclude verifies that only findings whose bucket
// is listed in ObjectTypes are kept. COLUMN is its own bucket (ObjectTypeColumn),
// distinct from ObjectTypeTable, so a TABLE filter keeps only table findings and
// a COLUMN filter keeps only column findings.
func TestFilterByScopeObjectTypeInclude(t *testing.T) {
	orders := ref("public", "orders")
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		tableDiff(TableDropped, "public", "legacy"),
		colDiff(ColumnAdded, orders, "email"),
		colDiff(ColumnTypeChanged, orders, "amount"),
	}

	// TABLE filter keeps only the table-level findings; column findings map to
	// ObjectTypeColumn and are dropped.
	gotTable := FilterByScope(diffs, Scope{ObjectTypes: []ObjectType{ObjectTypeTable}})
	require.Len(t, gotTable, 2, "TABLE filter must keep only table findings")
	for _, d := range gotTable {
		assert.Equal(t, ObjectTypeTable, d.ObjectType)
	}

	// COLUMN filter keeps only the column-level findings.
	gotColumn := FilterByScope(diffs, Scope{ObjectTypes: []ObjectType{ObjectTypeColumn}})
	require.Len(t, gotColumn, 2, "COLUMN filter must keep only column findings")
	for _, d := range gotColumn {
		assert.Equal(t, ObjectTypeColumn, d.ObjectType)
	}
}

// ─── ObjectTypes exclude filter ───────────────────────────────────────────────

// ─── COLUMN as a first-class object type ─────────────────────────────────────

// TestFilterByScopeColumnObjectTypeIsFirstClass verifies that COLUMN is a
// directly-selectable object-type bucket in its own right — it is not swept in
// under TABLE. Given a diff set containing both a table-level and a
// column-level finding, ObjectTypes=[COLUMN] and ObjectTypes=[TABLE] must each
// isolate the expected finding.
//
// The "exclude COLUMN" direction is the command's to express, by resolving
// --exclude-object-type-list into the complementary keep-set ([TABLE] here)
// before calling FilterByScope; see Scope's doc.
func TestFilterByScopeColumnObjectTypeIsFirstClass(t *testing.T) {
	orders := ref("public", "orders")
	tableFinding := tableDiff(TableAdded, "public", "orders")
	columnFinding := colDiff(ColumnAdded, orders, "email")

	diffs := []Difference{tableFinding, columnFinding}

	// ObjectTypes: [COLUMN] returns only the column finding.
	gotColumn := FilterByScope(diffs, Scope{ObjectTypes: []ObjectType{ObjectTypeColumn}})
	require.Len(t, gotColumn, 1, "COLUMN include must keep only the column finding")
	assert.Equal(t, ColumnAdded, gotColumn[0].Type)
	assert.Equal(t, ObjectTypeColumn, gotColumn[0].ObjectType)

	// ObjectTypes: [TABLE] returns only the table finding — which is also exactly
	// what the command passes for --exclude-object-type-list=COLUMN.
	gotTable := FilterByScope(diffs, Scope{ObjectTypes: []ObjectType{ObjectTypeTable}})
	require.Len(t, gotTable, 1, "TABLE include must keep only the table finding")
	assert.Equal(t, TableAdded, gotTable[0].Type)
	assert.Equal(t, ObjectTypeTable, gotTable[0].ObjectType)
}

// TestFilterByScopeColumnAnchorsToHostTableForTableList verifies that the
// object-type dimension is orthogonal to the table-list dimension: a column
// finding still anchors to its host table for --table-list, even though it is
// its own bucket for --object-type-list.
func TestFilterByScopeColumnAnchorsToHostTableForTableList(t *testing.T) {
	orders := ref("public", "orders")
	columnFinding := colDiff(ColumnAdded, orders, "email")

	got := FilterByScope([]Difference{columnFinding}, Scope{Tables: []schemasnapshot.ObjectRef{orders}})
	require.Len(t, got, 1, "column finding must be kept when its host table is in Tables")
	assert.Equal(t, ObjectTypeColumn, got[0].ObjectType, "ObjectType stays COLUMN even though the derived anchor is the host table")
}

// ─── Tables include filter ────────────────────────────────────────────────────

// TestFilterByScopeTableInclude verifies that only findings whose derived
// anchor table is in the Tables list are kept. A no-anchor finding is dropped
// by a non-empty Tables filter.
func TestFilterByScopeTableInclude(t *testing.T) {
	orders := ref("public", "orders")
	customers := ref("public", "customers")

	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		colDiff(ColumnAdded, orders, "id"),
		tableDiff(TableAdded, "public", "customers"),
		colDiff(ColumnAdded, customers, "name"),
		// A synthetic no-anchor finding, to verify that anchor-less entries are
		// dropped by a Tables include filter.
		noAnchorDiff(TableNameChanged, "public", "orders"),
	}

	got := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "orders")}})

	// Only orders-anchored findings should survive.
	for _, d := range got {
		if disp, ok := anchorDisplay(d); ok {
			assert.Equal(t, "public.orders", disp, "only public.orders-anchored findings should pass")
		} else {
			t.Errorf("no-anchor finding should have been dropped by Tables filter: %v", d)
		}
	}
	assert.Len(t, got, 2, "expect TableAdded and ColumnAdded for orders only")
}

// ─── Tables exclude filter ────────────────────────────────────────────────────

// ─── no-anchor findings ───────────────────────────────────────────────────────

// TestFilterByScopeNoAnchorDroppedByTables verifies that a finding with no
// derived anchor is dropped when Tables is non-empty, but passes through an
// empty Scope (no filter applied). All findings are built via noAnchorDiff,
// which forces an ObjectRef identity with a non-table ObjectType so
// anchorTableOf returns ok=false.
func TestFilterByScopeNoAnchorDroppedByTables(t *testing.T) {
	diffs := []Difference{
		noAnchorDiff(TableAdded, "public", "t1"),
		noAnchorDiff(ColumnAdded, "public", "t2"),
		noAnchorDiff(TableDropped, "public", "t3"),
	}

	// Non-empty Tables list: no-anchor findings must be dropped.
	got := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "orders")}})
	assert.Empty(t, got, "no-anchor findings must be dropped when Tables is non-empty")

	// Empty scope: all pass through (no-anchor is not dropped by ObjectTypes alone).
	got2 := FilterByScope(diffs, Scope{})
	assert.Len(t, got2, 3, "no-anchor findings pass through an empty Scope")

	// TABLE object-type filter: none of these findings are ObjectTypeTable — they
	// use the placeholder "VIEW" ObjectType specifically to be anchor-less, since
	// anchorTableOf's ObjectRef case treats "is a TABLE" and "has a self-anchor"
	// as the same fact (a real TABLE finding always anchors to itself). So all
	// three are dropped by an ObjectTypeTable-only include, unlike the old model
	// where AnchorTable was an independently-settable nil field.
	got3 := FilterByScope(diffs, Scope{ObjectTypes: []ObjectType{ObjectTypeTable}})
	assert.Empty(t, got3, "no-anchor findings are never ObjectTypeTable, so the TABLE filter drops all of them")
}

// ─── Either-side NAME_CHANGED rule ───────────────────────────────────────────

// TestFilterByScopeTableNameChangedOldNameInScope verifies that a TABLE_NAME_CHANGED
// finding is kept when only the OLD name is in Tables.
func TestFilterByScopeTableNameChangedOldNameInScope(t *testing.T) {
	// TABLE_NAME_CHANGED: old name "orders", new name "purchase_orders".
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "orders")}})
	assert.Len(t, got, 1, "TABLE_NAME_CHANGED should be kept when old name is in Tables")
}

// TestFilterByScopeTableNameChangedNewNameInScope verifies that a TABLE_NAME_CHANGED
// finding is kept when only the NEW name is in Tables.
func TestFilterByScopeTableNameChangedNewNameInScope(t *testing.T) {
	t.Skip("rename or move alias handling temporarily disabled in FilterByScope; re-enable with the alias logic")
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "purchase_orders")}})
	assert.Len(t, got, 1, "TABLE_NAME_CHANGED should be kept when new name is in Tables")
}

// TestFilterByScopeTableNameChangedNeitherNameInScope verifies that a TABLE_NAME_CHANGED
// finding is dropped when neither the old nor the new name appears in Tables.
func TestFilterByScopeTableNameChangedNeitherNameInScope(t *testing.T) {
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "customers")}})
	assert.Empty(t, got, "TABLE_NAME_CHANGED should be dropped when neither name is in Tables")
}

// ─── Anchor-rename extension ─────────────────────────────────────────────────

// TestFilterByScopeAnchorRenameExtension verifies that when a table is renamed
// (TABLE_NAME_CHANGED), findings anchored to either the old or the new name are
// included when either name appears in the Tables list.
func TestFilterByScopeAnchorRenameExtension(t *testing.T) {
	t.Skip("rename or move alias handling temporarily disabled in FilterByScope; re-enable with the alias logic")
	// TABLE_NAME_CHANGED: old "orders", new "purchase_orders".
	rename := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")
	// A column change anchored to the OLD table name (as the diff engine would emit it).
	oldAnchor := ref("public", "orders")
	colChange := Difference{
		Type:       ColumnTypeChanged,
		ObjectType: ObjectTypeColumn,
		ObjectA:    schemasnapshot.TableScopedObjectRef{Table: oldAnchor, Name: "amount"},
		ObjectB:    schemasnapshot.TableScopedObjectRef{Table: oldAnchor, Name: "amount"},
		SideAValue: "integer",
		SideBValue: "bigint",
	}

	diffs := []Difference{rename, colChange}

	// Filtering by the NEW name should include both: the rename itself and
	// the column change whose anchor is the old name.
	got := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "purchase_orders")}})
	assert.Len(t, got, 2, "rename + column change should both be kept when new name is in Tables")
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
	t.Skip("rename or move alias handling temporarily disabled in FilterByScope; re-enable with the alias logic")
	// Two renames where rename2's old name equals rename1's new name.
	rename1 := nameChangedDiff(TableNameChanged, "public", "users", "customers")
	rename2 := nameChangedDiff(TableNameChanged, "public", "customers", "clients")

	// A column change anchored to the original "users" name.
	usersRef := ref("public", "users")
	colChange := colDiff(ColumnAdded, usersRef, "email")

	diffs := []Difference{rename1, rename2, colChange}

	// Filtering by "customers" (rename1's new name) should keep all three:
	//   - rename1: anchor "users" aliases "customers" ✓
	//   - rename2: anchor "customers" direct match ✓
	//   - colChange: anchor "users" aliases "customers" ✓
	// The alias-map collision (both renames touching "customers" as a key) must
	// not drop rename1 or colChange.
	got := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "customers")}})
	gotTypes := collectTypes(got)
	assert.True(t, gotTypes[TableNameChanged], "rename findings should be kept — 'customers' is an anchor or alias")
	assert.True(t, gotTypes[ColumnAdded], "column change anchored to 'users' must NOT be dropped — 'users' aliases 'customers'")
	assert.Len(t, got, 3, "all three findings should survive filtering by 'customers'")

	// Filtering by "users" should include all three: rename1 and colChange
	// (direct anchor), and rename2 (anchor "customers" aliases "users" since
	// rename1 recorded the alias in both directions).
	got2 := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "users")}})
	got2Types := collectTypes(got2)
	assert.True(t, got2Types[TableNameChanged], "rename findings should be kept — 'users' is an anchor or alias")
	assert.True(t, got2Types[ColumnAdded], "colChange anchored to 'users' should be kept")
	assert.Len(t, got2, 3, "all three findings are reachable from 'users'")

	// Filtering by "clients" (rename2's new name) should keep rename2 only.
	// rename1 and colChange (anchor "users") must NOT be included:
	// aliases["public.users"] = ["public.customers"] only, not "public.clients".
	got3 := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "clients")}})
	got3Types := collectTypes(got3)
	assert.True(t, got3Types[TableNameChanged], "rename2 (customers→clients) should be kept when Tables=['public.clients']")
	assert.False(t, got3Types[ColumnAdded], "colChange anchored to 'users' must NOT be incorrectly included for 'clients'")
	assert.Len(t, got3, 1, "only rename2 should survive filtering by 'clients'")
}

// ─── Either-side rule across schema moves (SET SCHEMA) ───────────────────────

// TestFilterByScopeSchemaMoveNewIdentityInScope verifies the either-side rule
// for a table moved to a new schema (TABLE_SCHEMA_CHANGED). A finding anchored
// to the old schema-qualified identifier must be kept when the NEW one is in
// Tables, just as renames are kept by either name.
func TestFilterByScopeSchemaMoveNewIdentityInScope(t *testing.T) {
	t.Skip("rename or move alias handling temporarily disabled in FilterByScope; re-enable with the alias logic")
	// "old_s.orders" moved to "new_s.orders".
	move := schemaChangedDiff("old_s", "orders", "new_s")
	// A column change anchored to the OLD (schema, name).
	oldAnchor := ref("old_s", "orders")
	colChange := Difference{
		Type:       ColumnTypeChanged,
		ObjectType: ObjectTypeColumn,
		ObjectA:    schemasnapshot.TableScopedObjectRef{Table: oldAnchor, Name: "amount"},
		ObjectB:    schemasnapshot.TableScopedObjectRef{Table: oldAnchor, Name: "amount"},
		SideAValue: "integer",
		SideBValue: "bigint",
	}

	diffs := []Difference{move, colChange}

	// Filtering by the NEW schema-qualified name must keep BOTH the move finding
	// and the column change anchored to the old identifier.
	got := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("new_s", "orders")}})
	assert.Len(t, got, 2, "schema-move + column change should both be kept when the new identifier is in Tables")

	// Symmetric: filtering by the OLD identifier also keeps both.
	gotOld := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("old_s", "orders")}})
	assert.Len(t, gotOld, 2, "both findings should be kept when the old identifier is in Tables")
}

// TestFilterByScopeRenameAndMove verifies the either-side rule when a table is
// BOTH renamed and moved in the same interval. compareMatchedTables emits two
// findings (TABLE_NAME_CHANGED and TABLE_SCHEMA_CHANGED) that share the same
// side-A AND side-B identity — ObjectA is the complete old (schema, name) and
// ObjectB is the complete new (schema, name). The alias must be built from
// those complete refs directly — not reconstructed piecemeal from OldValue/
// NewValue strings.
func TestFilterByScopeRenameAndMove(t *testing.T) {
	t.Skip("rename or move alias handling temporarily disabled in FilterByScope; re-enable with the alias logic")
	// "old_s.orders" → "new_s.purchase_orders" (rename + move).
	oldRef := ref("old_s", "orders")
	newRef := ref("new_s", "purchase_orders")
	rename := Difference{
		Type:       TableNameChanged,
		ObjectType: ObjectTypeTable,
		ObjectA:    oldRef,
		ObjectB:    newRef,
		SideAValue: "orders",
		SideBValue: "purchase_orders",
	}
	move := Difference{
		Type:       TableSchemaChanged,
		ObjectType: ObjectTypeTable,
		ObjectA:    oldRef,
		ObjectB:    newRef,
		SideAValue: "old_s",
		SideBValue: "new_s",
	}
	// A column change anchored to the side-A (old) table ref, as the diff engine
	// emits it (compareMatchedColumns anchors to cA.TableScopedObjectRef).
	colChange := colDiff(ColumnAdded, oldRef, "email")

	diffs := []Difference{rename, move, colChange}

	// The true new identity is "new_s.purchase_orders": filtering by it keeps all three.
	got := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("new_s", "purchase_orders")}})
	assert.Len(t, got, 3, "rename+move + column change should all be kept when the true new identifier is in Tables")

	// The OLD identity "old_s.orders" keeps all three too (either-side).
	gotOld := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("old_s", "orders")}})
	assert.Len(t, gotOld, 3, "all three findings should be kept when the old identifier is in Tables")

	// The bogus "old schema + new name" identifier must NOT match anything —
	// it is not a real identity of this table on either side.
	gotBogus := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("old_s", "purchase_orders")}})
	assert.Empty(t, gotBogus, "the spurious old-schema+new-name identifier must not match — the table never had that identity")
}

// ─── Include-then-exclude interaction ────────────────────────────────────────

// ─── Edge cases ───────────────────────────────────────────────────────────────

// TestFilterByScopeUnknownTableNameIsNoOp verifies that a Tables value that
// matches no finding is a silent no-op (no panic, empty result).
func TestFilterByScopeUnknownTableNameIsNoOp(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
	}
	got := FilterByScope(diffs, Scope{Tables: []schemasnapshot.ObjectRef{ref("public", "nonexistent")}})
	assert.Empty(t, got)
}

// TestFilterByScopeEmptyInputReturnsEmpty verifies that an empty input slice
// always returns an empty (not nil) result.
func TestFilterByScopeEmptyInputReturnsEmpty(t *testing.T) {
	got := FilterByScope(nil, Scope{ObjectTypes: []ObjectType{ObjectTypeTable}})
	assert.NotNil(t, got)
	assert.Empty(t, got)
}
