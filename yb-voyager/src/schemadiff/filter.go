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

// ObjectType is the user-facing object-type selector used in Scope filtering.
// There are six declared selectors, matching the values accepted by
// --object-type-list: TABLE, INDEX, SEQUENCE, VIEW, FUNCTION, TYPE.
// Materialized views ride under VIEW (no separate selector).
type ObjectType string

const (
	ObjectTypeTable ObjectType = "TABLE"
	// ObjectTypeIndex selects INDEX_* findings. Indexes are a first-class,
	// independently selectable object type per the functional spec, so all
	// INDEX_* DiffTypes classify here (not under ObjectTypeTable). An index
	// still carries its host table in Difference.AnchorTable, so --table-list
	// matches an index against its table independently of --object-type-list.
	ObjectTypeIndex    ObjectType = "INDEX"
	ObjectTypeSequence ObjectType = "SEQUENCE"
	ObjectTypeView     ObjectType = "VIEW"
	ObjectTypeFunction ObjectType = "FUNCTION"
	ObjectTypeType     ObjectType = "TYPE"
)

// Scope describes the include/exclude filters applied by FilterByScope.
//
// Tables and ExcludeTables match on Difference.AnchorTable (exact catalog
// identifiers, case-sensitive). ObjectTypes and ExcludeObjectTypes match on
// the object-type bucket that each DiffType belongs to. An empty list means
// "all"; both include and exclude lists may be non-empty simultaneously.
type Scope struct {
	Tables             []string     // empty = all; matched against AnchorTable
	ExcludeTables      []string     // drop findings whose AnchorTable is in this list
	ObjectTypes        []ObjectType // empty = all
	ExcludeObjectTypes []ObjectType
}

// diffTypeObjectType is an exhaustive mapping from every DiffType constant to
// its ObjectType bucket for scope filtering.
//
// Bucket assignment rules (§8):
//   - TABLE_*, COLUMN_*, CONSTRAINT_*, TRIGGER_*, PARTITION_*,
//     PRIMARY_KEY_CHANGED, UNIQUE_CONSTRAINT_CHANGED, FOREIGN_KEY_CHANGED,
//     CHECK_CONSTRAINT_CHANGED, EXCLUSION_CONSTRAINT_CHANGED,
//     NULLS_NOT_DISTINCT_CHANGED, REPLICA_IDENTITY_CHANGED,
//     TABLE_PERSISTENCE_CHANGED → ObjectTypeTable
//     (columns, constraints, and triggers "ride with" their table; they are
//     not independently selectable)
//   - INDEX_* → ObjectTypeIndex
//     (indexes are an independently selectable object type per the functional
//     spec; the host table travels in AnchorTable for --table-list matching)
//   - SEQUENCE_* → ObjectTypeSequence
//   - VIEW_*, MATERIALIZED_VIEW_* → ObjectTypeView
//     (ObjectTypeView covers both; no separate matview selector)
//   - FUNCTION_* → ObjectTypeFunction
//   - TYPE_*, ENUM_VALUE_* → ObjectTypeType
//   - ATTR_CHANGED → ObjectTypeTable (V1 simplification: every Attr the current
//     library writes sits on a table or table-owned object — Constraint, Index.
//     Revisit if a future database type places Attrs on non-table objects such
//     as sequences, views, or functions.)
var diffTypeObjectType = map[DiffType]ObjectType{
	// ── TABLE ────────────────────────────────────────────────────────────────
	TableAdded:               ObjectTypeTable,
	TableDropped:             ObjectTypeTable,
	TableNameChanged:         ObjectTypeTable,
	TableSchemaChanged:       ObjectTypeTable,
	TableKindChanged:         ObjectTypeTable,
	PartitionStrategyChanged: ObjectTypeTable,
	PartitionKeyChanged:      ObjectTypeTable,
	PartitionChildrenChanged: ObjectTypeTable,
	ReplicaIdentityChanged:   ObjectTypeTable,
	TablePersistenceChanged:  ObjectTypeTable,
	// New table-level link constants (not in the original parked branch).
	PartitionParentChanged:  ObjectTypeTable,
	TableInheritsChanged:    ObjectTypeTable,
	TableInheritedByChanged: ObjectTypeTable,

	// ── COLUMN ───────────────────────────────────────────────────────────────
	ColumnAdded:              ObjectTypeTable,
	ColumnDropped:            ObjectTypeTable,
	ColumnNameChanged:        ObjectTypeTable,
	ColumnTypeChanged:        ObjectTypeTable,
	ColumnNullabilityChanged: ObjectTypeTable,
	ColumnDefaultChanged:     ObjectTypeTable,
	ColumnIdentityChanged:    ObjectTypeTable,
	ColumnGeneratedChanged:   ObjectTypeTable,
	ColumnCollationChanged:   ObjectTypeTable,

	// ── CONSTRAINT ───────────────────────────────────────────────────────────
	ConstraintAdded:            ObjectTypeTable,
	ConstraintDropped:          ObjectTypeTable,
	ConstraintNameChanged:      ObjectTypeTable,
	PrimaryKeyChanged:          ObjectTypeTable,
	UniqueConstraintChanged:    ObjectTypeTable,
	ForeignKeyChanged:          ObjectTypeTable,
	CheckConstraintChanged:     ObjectTypeTable,
	ExclusionConstraintChanged: ObjectTypeTable,
	NullsNotDistinctChanged:    ObjectTypeTable,

	// ── INDEX ─────────────────────────────────────────────────────────────────
	IndexAdded:                  ObjectTypeIndex,
	IndexDropped:                ObjectTypeIndex,
	IndexNameChanged:            ObjectTypeIndex,
	IndexColumnsChanged:         ObjectTypeIndex,
	IndexAccessMethodChanged:    ObjectTypeIndex,
	IndexUniqueChanged:          ObjectTypeIndex,
	IndexWhereChanged:           ObjectTypeIndex,
	IndexIncludedColumnsChanged: ObjectTypeIndex,

	// ── SEQUENCE ──────────────────────────────────────────────────────────────
	SequenceAdded:             ObjectTypeSequence,
	SequenceDropped:           ObjectTypeSequence,
	SequenceNameChanged:       ObjectTypeSequence,
	SequenceSchemaChanged:     ObjectTypeSequence,
	SequencePropertiesChanged: ObjectTypeSequence,
	SequenceOwnedByChanged:    ObjectTypeSequence,

	// ── VIEW ──────────────────────────────────────────────────────────────────
	ViewAdded:             ObjectTypeView,
	ViewDropped:           ObjectTypeView,
	ViewNameChanged:       ObjectTypeView,
	ViewSchemaChanged:     ObjectTypeView,
	ViewDefinitionChanged: ObjectTypeView,

	// ── MATERIALIZED VIEW ─────────────────────────────────────────────────────
	MaterializedViewAdded:             ObjectTypeView,
	MaterializedViewDropped:           ObjectTypeView,
	MaterializedViewNameChanged:       ObjectTypeView,
	MaterializedViewSchemaChanged:     ObjectTypeView,
	MaterializedViewDefinitionChanged: ObjectTypeView,

	// ── FUNCTION ──────────────────────────────────────────────────────────────
	FunctionAdded:                 ObjectTypeFunction,
	FunctionDropped:               ObjectTypeFunction,
	FunctionNameChanged:           ObjectTypeFunction,
	FunctionSchemaChanged:         ObjectTypeFunction,
	FunctionKindChanged:           ObjectTypeFunction,
	FunctionSignatureChanged:      ObjectTypeFunction,
	FunctionReturnTypeChanged:     ObjectTypeFunction,
	FunctionVolatilityChanged:     ObjectTypeFunction,
	FunctionParallelSafetyChanged: ObjectTypeFunction,
	FunctionStrictChanged:         ObjectTypeFunction,
	FunctionLanguageChanged:       ObjectTypeFunction,
	FunctionSecurityChanged:       ObjectTypeFunction,

	// ── TRIGGER ───────────────────────────────────────────────────────────────
	TriggerAdded:               ObjectTypeTable,
	TriggerDropped:             ObjectTypeTable,
	TriggerNameChanged:         ObjectTypeTable,
	TriggerDefinitionChanged:   ObjectTypeTable,
	TriggerEnabledStateChanged: ObjectTypeTable,

	// ── TYPE (user-defined) ───────────────────────────────────────────────────
	TypeAdded:            ObjectTypeType,
	TypeDropped:          ObjectTypeType,
	TypeNameChanged:      ObjectTypeType,
	TypeSchemaChanged:    ObjectTypeType,
	TypeKindChanged:      ObjectTypeType,
	EnumValueAdded:       ObjectTypeType,
	EnumValueRemoved:     ObjectTypeType,
	TypeAttributeChanged: ObjectTypeType,

	// ── GENERIC (cross-cutting) ───────────────────────────────────────────────
	// V1 simplification: classify as TABLE because all Attrs the current library
	// writes are on table-owned objects. Revisit if non-table Attrs are added.
	AttrChanged: ObjectTypeTable,
}

// FilterByScope returns the subset of diffs that fall within the given scope.
//
// Filtering is pure: diffs and scope are never mutated and the returned slice
// is always a new allocation. A name matching nothing in the diff set is a
// silent no-op — validation is the caller's responsibility.
//
// The algorithm (§8):
//  1. Build a rename-alias map from TABLE_NAME_CHANGED findings so that a
//     finding whose AnchorTable was itself renamed is kept when either the
//     old or new table name appears in Tables / ExcludeTables.
//  2. Apply ObjectTypes (include) — keep if list is empty or bucket is listed.
//  3. Apply Tables (include) — keep if list is empty or AnchorTable matches.
//     A nil AnchorTable never matches a non-empty Tables list.
//  4. Apply ExcludeObjectTypes — drop if bucket is listed.
//  5. Apply ExcludeTables — drop if AnchorTable matches; nil is never dropped.
func FilterByScope(diffs []Difference, scope Scope) []Difference {
	// Pre-pass: build a map from old table name → new table name and vice versa
	// from TABLE_NAME_CHANGED findings. This implements the anchor-rename
	// either-side rule: a finding whose AnchorTable was itself renamed is kept
	// when either the old or the new name is in scope.
	//
	// The map is keyed by the catalog identifier string (ObjectRef.String()).
	tableRenameAliases := buildTableRenameAliases(diffs)

	// Pre-build lookup sets for the four lists to avoid O(n²) inner scans.
	includeTypes := makeObjectTypeSet(scope.ObjectTypes)
	excludeTypes := makeObjectTypeSet(scope.ExcludeObjectTypes)
	includeTables := makeStringSet(scope.Tables)
	excludeTables := makeStringSet(scope.ExcludeTables)

	out := make([]Difference, 0, len(diffs))
	for _, d := range diffs {
		if !passesObjectTypeFilter(d, includeTypes) {
			continue
		}
		if !passesTableIncludeFilter(d, includeTables, tableRenameAliases) {
			continue
		}
		if !passesObjectTypeExcludeFilter(d, excludeTypes) {
			continue
		}
		if !passesTableExcludeFilter(d, excludeTables, tableRenameAliases) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// buildTableRenameAliases scans diffs for TABLE_NAME_CHANGED entries and
// returns a map from each name to all its rename aliases so that either side
// of a rename can stand in for the other during table-scope matching.
//
// The map accumulates multiple aliases per name to handle rename chains such as
// "users→customers" and "customers→clients" in a single diff set. Each name
// maps to a slice of all names it is aliased to via rename findings.
//
// Keys and values are ObjectRef.String() catalog identifiers.
//
// No-op renames (old == new) are skipped to avoid polluting the alias set.
func buildTableRenameAliases(diffs []Difference) map[string][]string {
	aliases := make(map[string][]string)
	for _, d := range diffs {
		if d.Type != TableNameChanged {
			continue
		}
		// Object is the side-A (old) schemasnapshot.ObjectRef for a NAME_CHANGED finding.
		// NewValue carries the new name string.
		newName, ok := d.NewValue.(string)
		if !ok || newName == "" {
			continue
		}
		oldID := d.Object.String()
		// Construct the new ObjectRef identifier: same schema (schema didn't
		// change here), different name.
		newID := d.Object.Schema + "." + newName
		// Skip no-op renames to keep the alias map clean.
		if oldID == newID {
			continue
		}
		aliases[oldID] = append(aliases[oldID], newID)
		aliases[newID] = append(aliases[newID], oldID)
	}
	return aliases
}

// passesObjectTypeFilter returns true if the finding's object-type bucket is
// allowed by the include list. An empty includeTypes means "all".
func passesObjectTypeFilter(d Difference, includeTypes map[ObjectType]struct{}) bool {
	if len(includeTypes) == 0 {
		return true
	}
	bucket := diffTypeObjectType[d.Type]
	_, ok := includeTypes[bucket]
	return ok
}

// passesObjectTypeExcludeFilter returns true if the finding is NOT in the
// exclude list. An empty excludeTypes means "nothing excluded".
func passesObjectTypeExcludeFilter(d Difference, excludeTypes map[ObjectType]struct{}) bool {
	if len(excludeTypes) == 0 {
		return true
	}
	bucket := diffTypeObjectType[d.Type]
	_, excluded := excludeTypes[bucket]
	return !excluded
}

// passesTableIncludeFilter returns true if the finding should be kept after
// applying the Tables include filter.
//
// Rules:
//   - Empty includeTables → keep everything.
//   - nil AnchorTable → never matches; drop when list is non-empty.
//   - Non-nil AnchorTable → keep if the anchor or any of its rename aliases
//     appears in the list (either-side rule, §8). The alias map built from
//     TABLE_NAME_CHANGED findings covers all either-side cases including the
//     rename finding itself (old and new names are both recorded as aliases).
func passesTableIncludeFilter(d Difference, includeTables map[string]struct{}, aliases map[string][]string) bool {
	if len(includeTables) == 0 {
		return true
	}
	if d.AnchorTable == nil {
		return false
	}
	anchorID := d.AnchorTable.String()

	// Check the anchor itself.
	if _, ok := includeTables[anchorID]; ok {
		return true
	}

	// Check all rename aliases of the anchor.
	for _, alias := range aliases[anchorID] {
		if _, ok := includeTables[alias]; ok {
			return true
		}
	}

	return false
}

// passesTableExcludeFilter returns true if the finding should NOT be excluded
// by the ExcludeTables filter.
//
// Rules:
//   - Empty excludeTables → keep everything.
//   - nil AnchorTable → never excluded by this filter.
//   - Non-nil AnchorTable → drop if the anchor or any rename alias is in the list.
//     The alias map covers TABLE_NAME_CHANGED either-side matching automatically.
func passesTableExcludeFilter(d Difference, excludeTables map[string]struct{}, aliases map[string][]string) bool {
	if len(excludeTables) == 0 {
		return true
	}
	if d.AnchorTable == nil {
		return true // nil anchor is never excluded by ExcludeTables
	}
	anchorID := d.AnchorTable.String()

	if _, ok := excludeTables[anchorID]; ok {
		return false
	}

	// Check all rename aliases of the anchor.
	for _, alias := range aliases[anchorID] {
		if _, ok := excludeTables[alias]; ok {
			return false
		}
	}

	return true
}

// makeObjectTypeSet converts a []ObjectType into a map keyed by ObjectType for O(1) lookup.
func makeObjectTypeSet(types []ObjectType) map[ObjectType]struct{} {
	if len(types) == 0 {
		return nil
	}
	m := make(map[ObjectType]struct{}, len(types))
	for _, t := range types {
		m[t] = struct{}{}
	}
	return m
}

// makeStringSet converts a []string into a map for O(1) lookup.
func makeStringSet(ss []string) map[string]struct{} {
	if len(ss) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

// Ensure schemasnapshot is used (the import is needed for the Difference type
// which uses schemasnapshot.ObjectRef). This blank identifier avoids an
// "imported and not used" error if the compiler cannot infer usage from
// indirect references.
var _ *schemasnapshot.ObjectRef
