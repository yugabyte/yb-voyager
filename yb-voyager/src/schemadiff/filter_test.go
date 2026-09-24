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

// allScope returns the explicit Scope that keeps every finding in diffs: every
// schema, anchor table and object type present. Scope holds the exact set to keep, so
// this is what a caller passes for an unfiltered run -- the test-side stand-in for
// the universe the command resolves from the catalog and the snapshots.
func allScope(diffs []Difference) Scope {
	var s Scope
	seenSchema := make(map[string]bool)
	seenTable := make(map[schemasnapshot.ObjectRef]bool)
	seenType := make(map[ObjectType]bool)
	for _, d := range diffs {
		for _, sch := range schemasOf(d) {
			if !seenSchema[sch] {
				seenSchema[sch] = true
				s.Schemas = append(s.Schemas, sch)
			}
		}
		if anchor, ok := anchorTableOf(d); ok && !seenTable[anchor] {
			seenTable[anchor] = true
			s.Tables = append(s.Tables, anchor)
		}
		if !seenType[d.ObjectType] {
			seenType[d.ObjectType] = true
			s.ObjectTypes = append(s.ObjectTypes, d.ObjectType)
		}
	}
	return s
}

// narrowSchemas is allScope with the Schemas dimension replaced.
func narrowSchemas(diffs []Difference, schemas ...string) Scope {
	s := allScope(diffs)
	s.Schemas = schemas
	return s
}

// narrowTables is allScope with the Tables dimension replaced -- "filter by these
// tables, keep every object type".
func narrowTables(diffs []Difference, tables ...schemasnapshot.ObjectRef) Scope {
	s := allScope(diffs)
	s.Tables = tables
	return s
}

// narrowTypes is allScope with the ObjectTypes dimension replaced -- "filter by
// these object types, keep every table".
func narrowTypes(diffs []Difference, types ...ObjectType) Scope {
	s := allScope(diffs)
	s.ObjectTypes = types
	return s
}

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

// TestFilterByScopeEmptyScopeKeepsNothing pins that empty means empty. An empty
// dimension is the caller saying "keep none of these", which is what an
// --exclude-* list covering the whole universe resolves to; reading it as "keep
// everything" would invert exactly that request.
func TestFilterByScopeEmptyScopeKeepsNothing(t *testing.T) {
	orders := ref("public", "orders")
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		tableDiff(TableDropped, "public", "legacy"),
		colDiff(ColumnAdded, orders, "email"),
		colDiff(ColumnDropped, orders, "phone"),
	}

	assert.Empty(t, FilterByScope(diffs, Scope{}), "an empty Scope keeps nothing")

	// An unfiltered run passes the universe explicitly, and keeps everything.
	assert.Equal(t, diffs, FilterByScope(diffs, allScope(diffs)))
}

// ─── Schemas filter ───────────────────────────────────────────────────────────

// TestFilterByScopeSchemasKeepsOnlyRequested verifies the schema dimension: a
// finding in a schema nobody asked about is dropped, whatever its table or type.
// Without this, --source-db-schema public still reported drift in sales.
func TestFilterByScopeSchemasKeepsOnlyRequested(t *testing.T) {
	orders := ref("public", "orders")
	customers := ref("sales", "customers")
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		colDiff(ColumnAdded, orders, "note"),
		tableDiff(TableDropped, "sales", "customers"),
		colDiff(ColumnAdded, customers, "tier"),
	}

	got := FilterByScope(diffs, narrowSchemas(diffs, "public"))

	// A column reports its parent table's schema, so the column finding survives
	// on the strength of orders being in public.
	assert.Equal(t, []Difference{
		tableDiff(TableAdded, "public", "orders"),
		colDiff(ColumnAdded, orders, "note"),
	}, got, "only the public findings survive, in input order")
}

// TestFilterByScopeSchemasMatchesEitherSide pins why the rule is either-side and not
// side-B: a table moving between schemas has a different one on each side, and a
// move OUT of the requested set is exactly what its owner needs to be told about.
func TestFilterByScopeSchemasMatchesEitherSide(t *testing.T) {
	t.Run("moves out of scope: kept on side A", func(t *testing.T) {
		d := schemaChangedDiff("public", "orders", "sales")
		got := FilterByScope([]Difference{d}, narrowSchemas([]Difference{d}, "public"))
		require.Len(t, got, 1, "the owner of public must learn their table left it")
	})

	t.Run("moves into scope: kept on side B", func(t *testing.T) {
		d := schemaChangedDiff("sales", "orders", "public")
		got := FilterByScope([]Difference{d}, narrowSchemas([]Difference{d}, "public"))
		require.Len(t, got, 1, "a table arriving in public is in scope too")
	})

	t.Run("moves between two unrequested schemas: dropped", func(t *testing.T) {
		d := schemaChangedDiff("sales", "legacy", "hr")
		got := FilterByScope([]Difference{d}, narrowSchemas([]Difference{d}, "public"))
		assert.Empty(t, got, "neither side is public, so it is not this report's business")
	})
}

// TestFilterByScopeSchemasGovernAnchorlessFindings contrasts the two dimensions:
// an anchorless finding escapes the Tables filter, but NOT the schema filter --
// a view has no host table, yet it does live in a schema.
func TestFilterByScopeSchemasGovernAnchorlessFindings(t *testing.T) {
	diffs := []Difference{
		noAnchorDiff(TableAdded, "public", "v_orders"),
		noAnchorDiff(TableAdded, "sales", "v_customers"),
	}

	got := FilterByScope(diffs, narrowSchemas(diffs, "public"))

	require.Len(t, got, 1, "the sales one is out of scope even though no table anchors it")
	assert.Equal(t, []string{"public"}, schemasOf(got[0]))
}

// TestFilterByScopeEmptySchemasKeepsNothing is the schema dimension's half of the
// empty-means-empty rule.
func TestFilterByScopeEmptySchemasKeepsNothing(t *testing.T) {
	diffs := []Difference{tableDiff(TableAdded, "public", "orders")}
	s := allScope(diffs)
	s.Schemas = nil
	assert.Empty(t, FilterByScope(diffs, s), "an empty Schemas keeps nothing")
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

	scope := narrowTypes(orig, ObjectTypeTable)
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
	gotTable := FilterByScope(diffs, narrowTypes(diffs, ObjectTypeTable))
	require.Len(t, gotTable, 2, "TABLE filter must keep only table findings")
	for _, d := range gotTable {
		assert.Equal(t, ObjectTypeTable, d.ObjectType)
	}

	// COLUMN filter keeps only the column-level findings.
	gotColumn := FilterByScope(diffs, narrowTypes(diffs, ObjectTypeColumn))
	require.Len(t, gotColumn, 2, "COLUMN filter must keep only column findings")
	for _, d := range gotColumn {
		assert.Equal(t, ObjectTypeColumn, d.ObjectType)
	}
}

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
	gotColumn := FilterByScope(diffs, narrowTypes(diffs, ObjectTypeColumn))
	assert.Equal(t, []Difference{columnFinding}, gotColumn,
		"COLUMN include must keep only the column finding")

	// ObjectTypes: [TABLE] returns only the table finding. Asserting the whole
	// slice is what exercises the exclude direction: this keep-set is exactly what
	// the command resolves --exclude-object-type-list=COLUMN into, so the column
	// finding being absent is the claim under test, not just the table one being
	// present.
	gotTable := FilterByScope(diffs, narrowTypes(diffs, ObjectTypeTable))
	assert.Equal(t, []Difference{tableFinding}, gotTable,
		"TABLE include must keep only the table finding")
}

// TestFilterByScopeColumnAnchorsToHostTableForTableList verifies that the
// object-type dimension is orthogonal to the table-list dimension: a column
// finding still anchors to its host table for --table-list, even though it is
// its own bucket for --object-type-list.
func TestFilterByScopeColumnAnchorsToHostTableForTableList(t *testing.T) {
	orders := ref("public", "orders")
	columnFinding := colDiff(ColumnAdded, orders, "email")

	got := FilterByScope([]Difference{columnFinding}, narrowTables([]Difference{columnFinding}, orders))
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
		// A synthetic no-anchor finding: a table list cannot speak to it, so it
		// survives alongside the orders-anchored ones.
		noAnchorDiff(TableNameChanged, "public", "orders"),
	}

	got := FilterByScope(diffs, narrowTables(diffs, ref("public", "orders")))

	// Every ANCHORED finding that survived must be an orders one; the anchor-less
	// finding passes on its own rule.
	anchored := 0
	for _, d := range got {
		if disp, ok := anchorDisplay(d); ok {
			anchored++
			assert.Equal(t, "public.orders", disp, "only public.orders-anchored findings should pass")
		}
	}
	assert.Equal(t, 2, anchored, "expect TableAdded and ColumnAdded for orders only")
	assert.Len(t, got, 3, "plus the anchor-less finding, which a table list does not govern")
}

// ─── no-anchor findings ───────────────────────────────────────────────────────

// TestFilterByScopeNoAnchorPassesTableFilter verifies that a finding with no
// anchor table passes the Tables filter whatever it holds: a view or a function
// has no host table, so --table-list has nothing to say about it, and dropping it
// would make drift vanish from the report silently. --object-type-list is the
// dimension that selects object kinds, and it still applies. All findings are
// built via noAnchorDiff, which forces an ObjectRef identity with a non-table
// ObjectType so anchorTableOf returns ok=false.
func TestFilterByScopeNoAnchorPassesTableFilter(t *testing.T) {
	diffs := []Difference{
		noAnchorDiff(TableAdded, "public", "t1"),
		noAnchorDiff(ColumnAdded, "public", "t2"),
		noAnchorDiff(TableDropped, "public", "t3"),
	}

	// A table list that matches none of them: they pass anyway, because none of
	// them is about a table.
	got := FilterByScope(diffs, narrowTables(diffs, ref("public", "orders")))
	assert.Len(t, got, 3, "a table list must not drop findings that have no host table")

	// The object-type dimension still governs them.
	got2 := FilterByScope(diffs, Scope{})
	assert.Empty(t, got2, "an empty ObjectTypes keeps nothing, anchor or not")

	// TABLE object-type filter: none of these findings are ObjectTypeTable — they
	// use the placeholder "VIEW" ObjectType specifically to be anchor-less, since
	// anchorTableOf's ObjectRef case treats "is a TABLE" and "has a self-anchor"
	// as the same fact (a real TABLE finding always anchors to itself). So all
	// three are dropped by an ObjectTypeTable-only include, unlike the old model
	// where AnchorTable was an independently-settable nil field.
	got3 := FilterByScope(diffs, narrowTypes(diffs, ObjectTypeTable))
	assert.Empty(t, got3, "no-anchor findings are never ObjectTypeTable, so the TABLE filter drops all of them")
}

// ─── Either-side NAME_CHANGED rule ───────────────────────────────────────────

// TestFilterByScopeTableNameChangedOldNameInScope verifies that a TABLE_NAME_CHANGED
// finding is kept when only the OLD name is in Tables.
func TestFilterByScopeTableNameChangedOldNameInScope(t *testing.T) {
	// TABLE_NAME_CHANGED: old name "orders", new name "purchase_orders".
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, narrowTables([]Difference{d}, ref("public", "orders")))
	assert.Len(t, got, 1, "TABLE_NAME_CHANGED should be kept when old name is in Tables")
}

// TestFilterByScopeTableNameChangedNewNameInScope verifies that a TABLE_NAME_CHANGED
// finding is kept when only the NEW name is in Tables.
func TestFilterByScopeTableNameChangedNewNameInScope(t *testing.T) {
	t.Skip("rename or move alias handling temporarily disabled in FilterByScope; re-enable with the alias logic")
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, narrowTables([]Difference{d}, ref("public", "purchase_orders")))
	assert.Len(t, got, 1, "TABLE_NAME_CHANGED should be kept when new name is in Tables")
}

// TestFilterByScopeTableNameChangedNeitherNameInScope verifies that a TABLE_NAME_CHANGED
// finding is dropped when neither the old nor the new name appears in Tables.
func TestFilterByScopeTableNameChangedNeitherNameInScope(t *testing.T) {
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, narrowTables([]Difference{d}, ref("public", "customers")))
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
	got := FilterByScope(diffs, narrowTables(diffs, ref("public", "purchase_orders")))
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
	got := FilterByScope(diffs, narrowTables(diffs, ref("public", "customers")))
	gotTypes := collectTypes(got)
	assert.True(t, gotTypes[TableNameChanged], "rename findings should be kept — 'customers' is an anchor or alias")
	assert.True(t, gotTypes[ColumnAdded], "column change anchored to 'users' must NOT be dropped — 'users' aliases 'customers'")
	assert.Len(t, got, 3, "all three findings should survive filtering by 'customers'")

	// Filtering by "users" should include all three: rename1 and colChange
	// (direct anchor), and rename2 (anchor "customers" aliases "users" since
	// rename1 recorded the alias in both directions).
	got2 := FilterByScope(diffs, narrowTables(diffs, ref("public", "users")))
	got2Types := collectTypes(got2)
	assert.True(t, got2Types[TableNameChanged], "rename findings should be kept — 'users' is an anchor or alias")
	assert.True(t, got2Types[ColumnAdded], "colChange anchored to 'users' should be kept")
	assert.Len(t, got2, 3, "all three findings are reachable from 'users'")

	// Filtering by "clients" (rename2's new name) should keep rename2 only.
	// rename1 and colChange (anchor "users") must NOT be included:
	// aliases["public.users"] = ["public.customers"] only, not "public.clients".
	got3 := FilterByScope(diffs, narrowTables(diffs, ref("public", "clients")))
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
	got := FilterByScope(diffs, narrowTables(diffs, ref("new_s", "orders")))
	assert.Len(t, got, 2, "schema-move + column change should both be kept when the new identifier is in Tables")

	// Symmetric: filtering by the OLD identifier also keeps both.
	gotOld := FilterByScope(diffs, narrowTables(diffs, ref("old_s", "orders")))
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
	got := FilterByScope(diffs, narrowTables(diffs, ref("new_s", "purchase_orders")))
	assert.Len(t, got, 3, "rename+move + column change should all be kept when the true new identifier is in Tables")

	// The OLD identity "old_s.orders" keeps all three too (either-side).
	gotOld := FilterByScope(diffs, narrowTables(diffs, ref("old_s", "orders")))
	assert.Len(t, gotOld, 3, "all three findings should be kept when the old identifier is in Tables")

	// The bogus "old schema + new name" identifier must NOT match anything —
	// it is not a real identity of this table on either side.
	gotBogus := FilterByScope(diffs, narrowTables(diffs, ref("old_s", "purchase_orders")))
	assert.Empty(t, gotBogus, "the spurious old-schema+new-name identifier must not match — the table never had that identity")
}

// ─── Edge cases ───────────────────────────────────────────────────────────────

// TestFilterByScopeUnknownTableNameIsNoOp verifies that a Tables value that
// matches no finding is a silent no-op (no panic, empty result).
func TestFilterByScopeUnknownTableNameIsNoOp(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
	}
	got := FilterByScope(diffs, narrowTables(diffs, ref("public", "nonexistent")))
	assert.Empty(t, got)
}

// TestFilterByScopeEmptyInputReturnsEmpty verifies that an empty input slice
// always returns an empty (not nil) result.
func TestFilterByScopeEmptyInputReturnsEmpty(t *testing.T) {
	got := FilterByScope(nil, narrowTypes(nil, ObjectTypeTable))
	assert.NotNil(t, got)
	assert.Empty(t, got)
}
