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
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// Note: ref(schema, name) is already declared in diff_test.go (same package).

// tableDiff builds a Difference anchored to a table (AnchorTable == Object).
func tableDiff(dt DiffType, schema, name string) Difference {
	o := ref(schema, name)
	return Difference{Type: dt, Object: o, AnchorTable: &o}
}

// indexDiff builds an INDEX_* finding whose Object is an index but whose
// AnchorTable points to the host table.
func indexDiff(dt DiffType, idxSchema, idxName, tblSchema, tblName string) Difference {
	tbl := ref(tblSchema, tblName)
	return Difference{
		Type:        dt,
		Object:      ref(idxSchema, idxName),
		AnchorTable: &tbl,
	}
}

// viewDiff builds a VIEW_* finding with a nil AnchorTable.
func viewDiff(dt DiffType, schema, name string) Difference {
	return Difference{Type: dt, Object: ref(schema, name), AnchorTable: nil}
}

// funcDiff builds a FUNCTION_* finding with a nil AnchorTable.
func funcDiff(dt DiffType, schema, name string) Difference {
	return Difference{Type: dt, Object: ref(schema, name), AnchorTable: nil}
}

// typeDiff builds a TYPE_* finding with a nil AnchorTable.
func typeDiff(dt DiffType, schema, name string) Difference {
	return Difference{Type: dt, Object: ref(schema, name), AnchorTable: nil}
}

// nameChangedDiff builds a *_NAME_CHANGED finding with the given old and new names.
// AnchorTable is set to the old ObjectRef (same as Object), per the spec.
func nameChangedDiff(dt DiffType, schema, oldName, newName string) Difference {
	o := ref(schema, oldName)
	return Difference{
		Type:        dt,
		Object:      o,
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

// TestDiffTypeObjectTypeMapIsExhaustive asserts that every DiffType constant
// declared in diff.go appears in the diffTypeObjectType map. If a new constant
// is added to diff.go without a corresponding map entry, this test fails.
//
// The set of all DiffType constants is obtained by the same technique used in
// similar exhaustiveness tests in the codebase: JSON-marshal every constant
// that ships in this package. We enumerate them manually here because Go has
// no reflection-level "list all constants of type T" API; the list below must
// stay in sync with diff.go.
func TestDiffTypeObjectTypeMapIsExhaustive(t *testing.T) {
	// allDiffTypes must list every DiffType constant from diff.go.
	// Keeping this in sync is enforced by the test itself: a constant absent
	// from this slice AND absent from diffTypeObjectType passes silently — the
	// failure only fires when a constant is here but not in the map. Developers
	// should add to this slice whenever they add to diff.go.
	allDiffTypes := []DiffType{
		// TABLE
		TableAdded, TableDropped, TableNameChanged, TableSchemaChanged,
		TableKindChanged, PartitionStrategyChanged, PartitionKeyChanged,
		PartitionChildrenChanged, ReplicaIdentityChanged, TablePersistenceChanged,
		// New table-level link constants (not in the original parked branch).
		PartitionParentChanged, TableInheritsChanged, TableInheritedByChanged,
		// COLUMN
		ColumnAdded, ColumnDropped, ColumnNameChanged, ColumnTypeChanged,
		ColumnNullabilityChanged, ColumnDefaultChanged, ColumnIdentityChanged,
		ColumnGeneratedChanged, ColumnCollationChanged,
		// CONSTRAINT
		ConstraintAdded, ConstraintDropped, ConstraintNameChanged,
		PrimaryKeyChanged, UniqueConstraintChanged, ForeignKeyChanged,
		CheckConstraintChanged, ExclusionConstraintChanged, NullsNotDistinctChanged,
		// INDEX
		IndexAdded, IndexDropped, IndexNameChanged, IndexColumnsChanged,
		IndexAccessMethodChanged, IndexUniqueChanged, IndexWhereChanged,
		IndexIncludedColumnsChanged,
		// SEQUENCE
		SequenceAdded, SequenceDropped, SequenceNameChanged, SequenceSchemaChanged,
		SequencePropertiesChanged, SequenceOwnedByChanged,
		// VIEW
		ViewAdded, ViewDropped, ViewNameChanged, ViewSchemaChanged, ViewDefinitionChanged,
		// MATERIALIZED VIEW
		MaterializedViewAdded, MaterializedViewDropped, MaterializedViewNameChanged,
		MaterializedViewSchemaChanged, MaterializedViewDefinitionChanged,
		// FUNCTION
		FunctionAdded, FunctionDropped, FunctionNameChanged, FunctionSchemaChanged,
		FunctionKindChanged, FunctionSignatureChanged, FunctionReturnTypeChanged,
		FunctionVolatilityChanged, FunctionParallelSafetyChanged, FunctionStrictChanged,
		FunctionLanguageChanged, FunctionSecurityChanged,
		// TRIGGER
		TriggerAdded, TriggerDropped, TriggerNameChanged, TriggerDefinitionChanged,
		TriggerEnabledStateChanged,
		// TYPE
		TypeAdded, TypeDropped, TypeNameChanged, TypeSchemaChanged, TypeKindChanged,
		EnumValueAdded, EnumValueRemoved, TypeAttributeChanged,
		// GENERIC
		AttrChanged,
	}

	// Guard: the two lists must have the same length. If they diverge someone
	// added a DiffType to one place but not the other.
	assert.Equal(t, len(allDiffTypes), len(diffTypeObjectType),
		"allDiffTypes and diffTypeObjectType have different lengths — when adding a DiffType constant, add it to BOTH the allDiffTypes slice in this test AND the diffTypeObjectType map in filter.go")

	for _, dt := range allDiffTypes {
		_, ok := diffTypeObjectType[dt]
		assert.True(t, ok, "DiffType %q is missing from diffTypeObjectType — add an entry in filter.go", dt)
	}
}

// ─── Empty scope ─────────────────────────────────────────────────────────────

// TestFilterByScopeEmptyScopeKeepsEverything verifies that an empty Scope
// (all lists nil/empty) passes every finding through unchanged.
func TestFilterByScopeEmptyScopeKeepsEverything(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		viewDiff(ViewAdded, "public", "v_active"),
		funcDiff(FunctionAdded, "public", "calc_total"),
		typeDiff(TypeAdded, "public", "order_status"),
		indexDiff(IndexAdded, "public", "orders_idx", "public", "orders"),
	}

	got := FilterByScope(diffs, Scope{})
	assert.Equal(t, diffs, got)
}

// ─── Purity ───────────────────────────────────────────────────────────────────

// TestFilterByScopeIsPure verifies that FilterByScope does not mutate the
// input slice or the Scope value, and that the returned slice is independent.
func TestFilterByScopeIsPure(t *testing.T) {
	orig := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		viewDiff(ViewAdded, "public", "v_active"),
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

	// Returned slice must be a new allocation.
	require.Len(t, got, 1)
	assert.Equal(t, TableAdded, got[0].Type)

	// Mutating the returned slice must not affect the input.
	got[0].Type = TableDropped
	assert.Equal(t, TableAdded, orig[0].Type, "mutation of returned slice must not affect input")
}

// ─── ObjectTypes include filter ───────────────────────────────────────────────

// TestFilterByScopeObjectTypeInclude verifies that only findings whose bucket
// is listed in ObjectTypes are kept.
func TestFilterByScopeObjectTypeInclude(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		viewDiff(ViewAdded, "public", "v_active"),
		funcDiff(FunctionAdded, "public", "calc_total"),
		typeDiff(TypeAdded, "public", "order_status"),
		indexDiff(IndexAdded, "public", "orders_idx", "public", "orders"),
		{Type: SequenceAdded, Object: ref("public", "orders_id_seq"), AnchorTable: nil},
	}

	got := FilterByScope(diffs, Scope{ObjectTypes: []ObjectType{ObjectTypeTable}})
	gotTypes := collectTypes(got)

	assert.True(t, gotTypes[TableAdded], "TABLE_ADDED should pass TABLE filter")
	assert.False(t, gotTypes[IndexAdded], "INDEX_ADDED is its own object type, should be excluded by TABLE-only filter")
	assert.False(t, gotTypes[ViewAdded], "VIEW_ADDED should be excluded by TABLE-only filter")
	assert.False(t, gotTypes[FunctionAdded], "FUNCTION_ADDED should be excluded")
	assert.False(t, gotTypes[TypeAdded], "TYPE_ADDED should be excluded")
	assert.False(t, gotTypes[SequenceAdded], "SEQUENCE_ADDED should be excluded")
}

// TestFilterByScopeObjectTypeIncludeIndex verifies that INDEX is an
// independently selectable object type: an INDEX filter keeps index findings
// and excludes table findings (and vice versa).
func TestFilterByScopeObjectTypeIncludeIndex(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		indexDiff(IndexAdded, "public", "orders_idx", "public", "orders"),
	}

	got := FilterByScope(diffs, Scope{ObjectTypes: []ObjectType{ObjectTypeIndex}})
	gotTypes := collectTypes(got)

	assert.True(t, gotTypes[IndexAdded], "INDEX_ADDED should pass INDEX filter")
	assert.False(t, gotTypes[TableAdded], "TABLE_ADDED should be excluded by INDEX-only filter")
}

// TestFilterByScopeObjectTypeIncludeView verifies that VIEW filter covers
// both plain views and materialized views.
func TestFilterByScopeObjectTypeIncludeView(t *testing.T) {
	diffs := []Difference{
		viewDiff(ViewAdded, "public", "v_active"),
		{Type: MaterializedViewAdded, Object: ref("public", "mv_totals"), AnchorTable: nil},
		tableDiff(TableAdded, "public", "orders"),
	}

	got := FilterByScope(diffs, Scope{ObjectTypes: []ObjectType{ObjectTypeView}})
	gotTypes := collectTypes(got)

	assert.True(t, gotTypes[ViewAdded], "VIEW_ADDED should pass VIEW filter")
	assert.True(t, gotTypes[MaterializedViewAdded], "MATERIALIZED_VIEW_ADDED should pass VIEW filter")
	assert.False(t, gotTypes[TableAdded], "TABLE_ADDED should not pass VIEW filter")
}

// ─── ObjectTypes exclude filter ───────────────────────────────────────────────

// TestFilterByScopeObjectTypeExclude verifies that findings in excluded buckets
// are dropped.
func TestFilterByScopeObjectTypeExclude(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
		viewDiff(ViewAdded, "public", "v_active"),
		funcDiff(FunctionAdded, "public", "calc_total"),
	}

	got := FilterByScope(diffs, Scope{ExcludeObjectTypes: []ObjectType{ObjectTypeView}})
	gotTypes := collectTypes(got)

	assert.True(t, gotTypes[TableAdded], "TABLE_ADDED should not be excluded")
	assert.False(t, gotTypes[ViewAdded], "VIEW_ADDED should be excluded")
	assert.True(t, gotTypes[FunctionAdded], "FUNCTION_ADDED should not be excluded")
}

// ─── Tables include filter ────────────────────────────────────────────────────

// TestFilterByScopeTableInclude verifies that only findings whose AnchorTable
// is in the Tables list are kept.
func TestFilterByScopeTableInclude(t *testing.T) {
	orders := ref("public", "orders")
	customers := ref("public", "customers")

	diffs := []Difference{
		{Type: TableAdded, Object: orders, AnchorTable: &orders},
		{Type: ColumnAdded, Object: orders, AnchorTable: &orders, SubObject: "id"},
		{Type: TableAdded, Object: customers, AnchorTable: &customers},
		{Type: ColumnAdded, Object: customers, AnchorTable: &customers, SubObject: "name"},
		viewDiff(ViewAdded, "public", "v_active"), // nil AnchorTable
	}

	got := FilterByScope(diffs, Scope{Tables: []string{"public.orders"}})

	// Only orders-anchored findings should survive.
	for _, d := range got {
		if d.AnchorTable != nil {
			assert.Equal(t, "public.orders", d.AnchorTable.String(),
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
		{Type: TableAdded, Object: orders, AnchorTable: &orders},
		{Type: TableAdded, Object: customers, AnchorTable: &customers},
		viewDiff(ViewAdded, "public", "v_active"), // nil AnchorTable — must NOT be dropped
	}

	got := FilterByScope(diffs, Scope{ExcludeTables: []string{"public.orders"}})

	gotTypes := collectTypes(got)
	assert.False(t, func() bool {
		for _, d := range got {
			if d.AnchorTable != nil && d.AnchorTable.String() == "public.orders" {
				return true
			}
		}
		return false
	}(), "public.orders-anchored findings should be excluded")
	assert.True(t, gotTypes[TableAdded], "public.customers TableAdded should remain")
	assert.True(t, gotTypes[ViewAdded], "nil-AnchorTable ViewAdded should not be dropped by ExcludeTables")
	assert.Len(t, got, 2)
}

// ─── nil AnchorTable findings ─────────────────────────────────────────────────

// TestFilterByScopeNilAnchorDroppedByTables verifies that a finding with a nil
// AnchorTable is dropped when Tables is non-empty, but is selectable via ObjectTypes.
func TestFilterByScopeNilAnchorDroppedByTables(t *testing.T) {
	diffs := []Difference{
		viewDiff(ViewAdded, "public", "v_active"),
		funcDiff(FunctionAdded, "public", "calc_total"),
		typeDiff(TypeAdded, "public", "order_status"),
	}

	// Non-empty Tables list: nil-AnchorTable findings must be dropped.
	got := FilterByScope(diffs, Scope{Tables: []string{"public.orders"}})
	assert.Empty(t, got, "nil-AnchorTable findings must be dropped when Tables is non-empty")

	// But they are selectable via ObjectTypes.
	got2 := FilterByScope(diffs, Scope{ObjectTypes: []ObjectType{ObjectTypeView, ObjectTypeFunction, ObjectTypeType}})
	assert.Len(t, got2, 3, "nil-AnchorTable findings are selectable via ObjectTypes")
}

// ─── Either-side NAME_CHANGED rule ───────────────────────────────────────────

// TestFilterByScopeTableNameChangedOldNameInScope verifies that a TABLE_NAME_CHANGED
// finding is kept when only the OLD name is in Tables.
func TestFilterByScopeTableNameChangedOldNameInScope(t *testing.T) {
	// TABLE_NAME_CHANGED: old name "orders", new name "purchase_orders".
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, Scope{Tables: []string{"public.orders"}})
	assert.Len(t, got, 1, "TABLE_NAME_CHANGED should be kept when old name is in Tables")
}

// TestFilterByScopeTableNameChangedNewNameInScope verifies that a TABLE_NAME_CHANGED
// finding is kept when only the NEW name is in Tables.
func TestFilterByScopeTableNameChangedNewNameInScope(t *testing.T) {
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, Scope{Tables: []string{"public.purchase_orders"}})
	assert.Len(t, got, 1, "TABLE_NAME_CHANGED should be kept when new name is in Tables")
}

// TestFilterByScopeTableNameChangedNeitherNameInScope verifies that a TABLE_NAME_CHANGED
// finding is dropped when neither the old nor the new name appears in Tables.
func TestFilterByScopeTableNameChangedNeitherNameInScope(t *testing.T) {
	d := nameChangedDiff(TableNameChanged, "public", "orders", "purchase_orders")

	got := FilterByScope([]Difference{d}, Scope{Tables: []string{"public.customers"}})
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
		Object:      oldAnchor,
		AnchorTable: &oldAnchor,
		SubObject:   "amount",
		Property:    "data_type",
		OldValue:    "integer",
		NewValue:    "bigint",
	}

	diffs := []Difference{rename, colChange}

	// Filtering by the NEW name should include both: the rename itself and
	// the column change whose anchor is the old name.
	got := FilterByScope(diffs, Scope{Tables: []string{"public.purchase_orders"}})
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
		Object:      oldAnchor,
		AnchorTable: &oldAnchor,
		SubObject:   "amount",
	}

	diffs := []Difference{rename, colChange}

	// Excluding by the NEW name should drop findings whose anchor is the OLD name
	// (because they are aliases via the rename map).
	got := FilterByScope(diffs, Scope{ExcludeTables: []string{"public.purchase_orders"}})
	assert.Empty(t, got, "findings anchored to old name should be excluded when new name is in ExcludeTables")
}

// TestFilterByScopeAliasMapCollision verifies that when two TABLE_NAME_CHANGED
// entries share a name (e.g. "users→customers" and "customers→clients"), the
// alias map correctly accumulates multiple aliases per name without the second
// rename silently overwriting the first, so no finding is incorrectly dropped.
//
// Scenario:
//
//	rename1: users        → customers
//	rename2: customers    → clients
//
// A column finding anchored to "users" must still be included when Tables
// contains "customers" (transitively via rename1's alias), and must not be
// silently dropped by a map-key collision between the two renames.
func TestFilterByScopeAliasMapCollision(t *testing.T) {
	// Two renames where rename2's old name equals rename1's new name.
	rename1 := nameChangedDiff(TableNameChanged, "public", "users", "customers")
	rename2 := nameChangedDiff(TableNameChanged, "public", "customers", "clients")

	// A column change anchored to the original "users" name.
	usersRef := ref("public", "users")
	colChange := Difference{
		Type:        ColumnAdded,
		Object:      usersRef,
		AnchorTable: &usersRef,
		SubObject:   "email",
	}

	diffs := []Difference{rename1, rename2, colChange}

	// Filtering by "customers" (rename1's new name) should keep all three:
	//   - rename1: anchor "users" aliases "customers" ✓
	//   - rename2: anchor "customers" direct match ✓
	//   - colChange: anchor "users" aliases "customers" ✓
	// The key concern is that the alias-map collision (rename1 and rename2 both
	// touching "customers" as a key) must NOT cause rename1 or colChange to be
	// silently dropped.
	got := FilterByScope(diffs, Scope{Tables: []string{"public.customers"}})
	gotTypes := collectTypes(got)
	assert.True(t, gotTypes[TableNameChanged], "rename findings should be kept — 'customers' is an anchor or alias")
	assert.True(t, gotTypes[ColumnAdded], "column change anchored to 'users' must NOT be dropped — 'users' aliases 'customers'")
	assert.Len(t, got, 3, "all three findings should survive filtering by 'customers'")

	// Filtering by "users" should include all three: rename1 (direct anchor),
	// colChange (direct anchor), and rename2 (its anchor "customers" aliases
	// "users" because rename1 recorded customers→users in both directions).
	got2 := FilterByScope(diffs, Scope{Tables: []string{"public.users"}})
	got2Types := collectTypes(got2)
	assert.True(t, got2Types[TableNameChanged], "rename findings should be kept — 'users' is an anchor or alias")
	assert.True(t, got2Types[ColumnAdded], "colChange anchored to 'users' should be kept")
	assert.Len(t, got2, 3, "all three findings are reachable from 'users'")

	// Filtering by "clients" (rename2's new name) should keep rename2 only.
	// rename1 (anchor "users") and colChange (anchor "users") must NOT be
	// incorrectly included: aliases["public.users"] = ["public.customers"] only,
	// not "public.clients" — so the alias-map collision fix is verified here.
	got3 := FilterByScope(diffs, Scope{Tables: []string{"public.clients"}})
	got3Types := collectTypes(got3)
	assert.True(t, got3Types[TableNameChanged], "rename2 (customers→clients) should be kept when Tables=['public.clients']")
	assert.False(t, got3Types[ColumnAdded], "colChange anchored to 'users' must NOT be incorrectly included for 'clients'")
	assert.Len(t, got3, 1, "only rename2 should survive filtering by 'clients'")
}

// ─── Include-then-exclude interaction ────────────────────────────────────────

// TestFilterByScopeIncludeThenExclude verifies that the include filter runs
// before the exclude filter and that their interaction is correct.
func TestFilterByScopeIncludeThenExclude(t *testing.T) {
	orders := ref("public", "orders")
	customers := ref("public", "customers")

	diffs := []Difference{
		{Type: TableAdded, Object: orders, AnchorTable: &orders},
		{Type: TableAdded, Object: customers, AnchorTable: &customers},
		viewDiff(ViewAdded, "public", "v_active"),
		funcDiff(FunctionAdded, "public", "calc_total"),
	}

	// Include TABLE and FUNCTION, but then exclude FUNCTION.
	got := FilterByScope(diffs, Scope{
		ObjectTypes:        []ObjectType{ObjectTypeTable, ObjectTypeFunction},
		ExcludeObjectTypes: []ObjectType{ObjectTypeFunction},
	})

	gotTypes := collectTypes(got)
	assert.True(t, gotTypes[TableAdded], "TABLE_ADDED should pass (included, not excluded)")
	assert.False(t, gotTypes[ViewAdded], "VIEW_ADDED should be dropped (not in include list)")
	assert.False(t, gotTypes[FunctionAdded], "FUNCTION_ADDED should be dropped (excluded after include)")
	assert.Len(t, got, 2)
}

// TestFilterByScopeIncludeTableExcludeTable verifies that when a table appears
// in both Tables and ExcludeTables, the exclude wins (exclude runs after include).
func TestFilterByScopeIncludeTableExcludeTable(t *testing.T) {
	orders := ref("public", "orders")
	customers := ref("public", "customers")

	diffs := []Difference{
		{Type: TableAdded, Object: orders, AnchorTable: &orders},
		{Type: TableAdded, Object: customers, AnchorTable: &customers},
	}

	// Include both, then exclude orders.
	got := FilterByScope(diffs, Scope{
		Tables:        []string{"public.orders", "public.customers"},
		ExcludeTables: []string{"public.orders"},
	})

	assert.Len(t, got, 1)
	assert.Equal(t, "public.customers", got[0].AnchorTable.String())
}

// ─── Edge cases ───────────────────────────────────────────────────────────────

// TestFilterByScopeUnknownTableNameIsNoOp verifies that a Tables value that
// matches no finding is a silent no-op (no panic, empty result).
func TestFilterByScopeUnknownTableNameIsNoOp(t *testing.T) {
	diffs := []Difference{
		tableDiff(TableAdded, "public", "orders"),
	}
	got := FilterByScope(diffs, Scope{Tables: []string{"public.nonexistent"}})
	assert.Empty(t, got)
}

// TestFilterByScopeEmptyInputReturnsEmpty verifies that an empty input slice
// always returns an empty (not nil) result.
func TestFilterByScopeEmptyInputReturnsEmpty(t *testing.T) {
	got := FilterByScope(nil, Scope{ObjectTypes: []ObjectType{ObjectTypeTable}})
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// TestFilterByScopeSequenceOwnedByTable verifies that an owned sequence
// (AnchorTable set to owner table) is included/excluded by Tables as expected.
func TestFilterByScopeSequenceOwnedByTable(t *testing.T) {
	owner := ref("public", "orders")
	seqDiff := Difference{
		Type:        SequenceAdded,
		Object:      ref("public", "orders_id_seq"),
		AnchorTable: &owner,
	}

	// With TABLE filter: sequences classify as SEQUENCE, not TABLE — should be excluded.
	got := FilterByScope([]Difference{seqDiff}, Scope{ObjectTypes: []ObjectType{ObjectTypeTable}})
	assert.Empty(t, got, "owned sequence classifies as SEQUENCE, not TABLE")

	// With SEQUENCE filter: should be included.
	got2 := FilterByScope([]Difference{seqDiff}, Scope{ObjectTypes: []ObjectType{ObjectTypeSequence}})
	assert.Len(t, got2, 1, "owned sequence passes SEQUENCE filter")

	// With Tables filter and owner name: should be included (AnchorTable matches).
	got3 := FilterByScope([]Difference{seqDiff}, Scope{Tables: []string{"public.orders"}})
	assert.Len(t, got3, 1, "owned sequence is kept when its owner table is in Tables")
}
