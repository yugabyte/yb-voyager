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
// "all".
//
// Scope is intentionally permissive and policy-free — it never errors:
//   - Both an include and its exclude list may be non-empty at once. Overlap is
//     resolved deterministically rather than rejected: exclude wins, because
//     FilterByScope applies includes first and excludes last (a finding kept by
//     an include is still dropped if it matches the corresponding exclude).
//   - An entry that matches nothing in the diff is a silent no-op.
//
// Flag-level policy is deliberately left to the caller (the command), mirroring
// where voyager validates such things. In particular, the convention that
// --table-list and --exclude-table-list are mutually exclusive is the command's
// to enforce at flag-parse time; the library does not, so that FilterByScope can
// stay a pure, total function for any programmatic caller (a future Scope.Validate
// could host such rules without making FilterByScope failable).
type Scope struct {
	Tables             []string     // empty = all; matched against AnchorTable
	ExcludeTables      []string     // drop findings whose AnchorTable is in this list
	ObjectTypes        []ObjectType // empty = all
	ExcludeObjectTypes []ObjectType
}

// FilterByScope returns the subset of diffs that fall within the given scope.
//
// Filtering is pure: diffs and scope are never mutated and the returned slice
// is always a new allocation. A name matching nothing in the diff set is a
// silent no-op — validation is the caller's responsibility.
//
// The algorithm (§8):
//  1. Build an identity-alias map from TABLE_NAME_CHANGED and TABLE_SCHEMA_CHANGED
//     findings so that a finding whose AnchorTable was itself renamed and/or
//     moved is kept when either the old or new catalog identifier appears in
//     Tables / ExcludeTables.
//  2. Apply ObjectTypes (include) — keep if list is empty or bucket is listed.
//  3. Apply Tables (include) — keep if list is empty or AnchorTable matches.
//     A nil AnchorTable never matches a non-empty Tables list.
//  4. Apply ExcludeObjectTypes — drop if bucket is listed.
//  5. Apply ExcludeTables — drop if AnchorTable matches; nil is never dropped.
func FilterByScope(diffs []Difference, scope Scope) []Difference {
	// Pre-pass: build a bidirectional alias map between each table's old and new
	// catalog identifier from TABLE_NAME_CHANGED / TABLE_SCHEMA_CHANGED findings.
	// This implements the anchor either-side rule: a finding whose AnchorTable was
	// itself renamed and/or moved is kept when either the old or the new
	// identifier is in scope.
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

// buildTableRenameAliases scans diffs for the findings that change a table's
// catalog identity — TABLE_NAME_CHANGED (new name) and TABLE_SCHEMA_CHANGED (new
// schema, i.e. SET SCHEMA) — and returns a map from each identifier to all the
// identifiers it is aliased to, so either side of an identity change can stand
// in for the other during table-scope matching (the either-side rule, §8).
//
// A single table may change BOTH its name and schema in one interval. The diff
// engine emits these as two separate findings that share the same side-A anchor
// (compareMatchedTables builds both from the same base.Object). We therefore
// group identity changes by the old ObjectRef and reconstruct the FULL new ref
// by applying whichever of {schema, name} changed — never "old schema + new
// name", which is not a real identity of the table on either side.
//
// The map accumulates multiple aliases per identifier (a []string) so rename
// chains across distinct tables in one diff set (e.g. a→b and b→c) do not clobber
// one another. Keys and values are ObjectRef.String() catalog identifiers.
//
// No-op changes (old == new) are skipped to keep the alias set clean.
func buildTableRenameAliases(diffs []Difference) map[string][]string {
	// Group the per-attribute identity changes by the side-A (old) ref so a
	// rename-and-move pair is recombined into one old→new mapping.
	type identityChange struct {
		oldRef    schemasnapshot.ObjectRef
		newSchema string // "" => schema unchanged
		newName   string // "" => name unchanged
	}
	changes := make(map[string]*identityChange)
	at := func(old schemasnapshot.ObjectRef) *identityChange {
		key := old.String()
		c := changes[key]
		if c == nil {
			c = &identityChange{oldRef: old}
			changes[key] = c
		}
		return c
	}
	for _, d := range diffs {
		switch d.Type {
		case TableNameChanged:
			if newName, ok := d.NewValue.(string); ok && newName != "" {
				at(d.Object).newName = newName
			}
		case TableSchemaChanged:
			if newSchema, ok := d.NewValue.(string); ok && newSchema != "" {
				at(d.Object).newSchema = newSchema
			}
		}
	}

	aliases := make(map[string][]string)
	for _, c := range changes {
		newRef := c.oldRef
		if c.newSchema != "" {
			newRef.Schema = c.newSchema
		}
		if c.newName != "" {
			newRef.Name = c.newName
		}
		oldID := c.oldRef.String()
		newID := newRef.String()
		if oldID == newID {
			continue // no-op
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
	bucket := diffTypeDefs[d.Type].ObjectType
	_, ok := includeTypes[bucket]
	return ok
}

// passesObjectTypeExcludeFilter returns true if the finding is NOT in the
// exclude list. An empty excludeTypes means "nothing excluded".
func passesObjectTypeExcludeFilter(d Difference, excludeTypes map[ObjectType]struct{}) bool {
	if len(excludeTypes) == 0 {
		return true
	}
	bucket := diffTypeDefs[d.Type].ObjectType
	_, excluded := excludeTypes[bucket]
	return !excluded
}

// passesTableIncludeFilter returns true if the finding should be kept after
// applying the Tables include filter.
//
// Rules:
//   - Empty includeTables → keep everything.
//   - nil AnchorTable → never matches; drop when list is non-empty.
//   - Non-nil AnchorTable → keep if the anchor or any of its identity aliases
//     appears in the list (either-side rule, §8). The alias map built from
//     TABLE_NAME_CHANGED / TABLE_SCHEMA_CHANGED findings covers all either-side
//     cases including the change finding itself (old and new identifiers are
//     both recorded as aliases).
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
//   - Non-nil AnchorTable → drop if the anchor or any identity alias is in the list.
//     The alias map covers TABLE_NAME_CHANGED / TABLE_SCHEMA_CHANGED either-side
//     matching automatically.
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
