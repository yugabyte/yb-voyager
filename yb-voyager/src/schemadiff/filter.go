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

// ObjectType is the user-facing object-type selector for Scope filtering, matching
// the values accepted by --object-type-list. v1 emits and filters TABLE and COLUMN
// findings, so both are declared; the remaining selectors are re-enabled as the
// engine starts emitting their findings.
//
// COLUMN is its own selector: a column change is filtered directly by
// --object-type-list=COLUMN (it is NOT swept in under TABLE). Table-scoping is
// orthogonal — a column finding still anchors to its host table for --table-list.
type ObjectType string

const (
	ObjectTypeTable  ObjectType = "TABLE"
	ObjectTypeColumn ObjectType = "COLUMN"
	// Not yet emitted by the diff engine — uncomment each as its findings land
	// (INDEX is its own selector but stays anchored to its host table for
	// --table-list, like COLUMN):
	// ObjectTypeIndex    ObjectType = "INDEX"
	// ObjectTypeSequence ObjectType = "SEQUENCE"
	// ObjectTypeView     ObjectType = "VIEW"
	// ObjectTypeFunction ObjectType = "FUNCTION"
	// ObjectTypeType     ObjectType = "TYPE"
)

// Scope describes the include/exclude filters applied by FilterByScope.
//
// IncludeTables/ExcludeTables hold caller-RESOLVED ObjectRefs (globs and the default
// schema already expanded); matching is exact, case-sensitive struct equality
// against the finding's anchor table, derived from its identity by anchorTableOf.
// IncludeObjectTypes/ExcludeObjectTypes match a finding's object-type bucket. An
// empty include list means "all".
//
// Scope is permissive and total — it never errors:
//   - If an include and its exclude are both set, exclude wins (includes apply
//     first, excludes last).
//   - An entry that matches nothing is a silent no-op.
//
// Flag-level policy — e.g. --table-list and --exclude-table-list being mutually
// exclusive — is the command's to enforce, so FilterByScope stays pure for any caller.
type Scope struct {
	IncludeTables      []schemasnapshot.ObjectRef // empty = all; matched against the finding's derived anchor table
	ExcludeTables      []schemasnapshot.ObjectRef // drop findings whose derived anchor table is in this list
	IncludeObjectTypes []ObjectType               // empty = all
	ExcludeObjectTypes []ObjectType
}

// FilterByScope returns the subset of diffs within scope. It is pure: inputs are
// never mutated and the result is a fresh slice; a name matching nothing is a
// silent no-op (validation is the caller's job).
//
// A table rename/move alias map is built first so a finding anchored to a renamed
// table matches on either its old or new identity. Then, in order:
//  1. include by IncludeObjectTypes
//  2. include by IncludeTables (a finding with no derived anchor never matches a non-empty list)
//  3. exclude by ExcludeObjectTypes
//  4. exclude by ExcludeTables
func FilterByScope(diffs []Difference, scope Scope) []Difference {
	// Bidirectional old<->new alias map for renamed/moved tables, so a finding
	// anchored to a renamed table matches on either identity (keyed by ObjectRef).
	tableRenameAliases := buildTableRenameAliases(diffs)

	// Pre-build lookup sets for the four lists to avoid O(n²) inner scans.
	includeTypes := toSet(scope.IncludeObjectTypes)
	excludeTypes := toSet(scope.ExcludeObjectTypes)
	includeTables := toSet(scope.IncludeTables)
	excludeTables := toSet(scope.ExcludeTables)

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

// anchorTableOf returns the host table a finding filters under for --table-list,
// derived from its identity: a table-scoped object (column/index) anchors to its
// parent table; a TABLE anchors to itself; a top-level object (view/function) has
// none. ok is false when there is no table anchor. Uses the side-A identity
// (side-B for *_ADDED, where ObjectA is nil).
func anchorTableOf(d Difference) (schemasnapshot.ObjectRef, bool) {
	id := d.ObjectA
	if id == nil {
		id = d.ObjectB
	}
	switch v := id.(type) {
	case schemasnapshot.TableScopedRef:
		return v.Table, true
	case schemasnapshot.ObjectRef:
		if d.ObjectType == ObjectTypeTable {
			return v, true
		}
	}
	return schemasnapshot.ObjectRef{}, false
}

// buildTableRenameAliases maps each table identity to the identities it aliases,
// from TABLE_NAME_CHANGED and TABLE_SCHEMA_CHANGED findings, so either side of a
// rename/move can stand in during table matching. ObjectA/ObjectB now carry the
// complete old/new refs directly (no reconstruction from OldValue/NewValue
// strings needed). Aliases accumulate ([]ObjectRef) so chains like a→b, b→c
// don't clobber each other, and dedup so a rename+move pair (which emits both a
// NAME_CHANGED and a SCHEMA_CHANGED finding sharing the same old→new refs)
// doesn't record the same alias twice.
func buildTableRenameAliases(diffs []Difference) map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef {
	aliases := make(map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef)
	add := func(from, to schemasnapshot.ObjectRef) {
		for _, x := range aliases[from] {
			if x == to {
				return
			}
		}
		aliases[from] = append(aliases[from], to)
	}
	for _, d := range diffs {
		if d.Type != TableNameChanged && d.Type != TableSchemaChanged {
			continue
		}
		oldRef, ok1 := d.ObjectA.(schemasnapshot.ObjectRef)
		newRef, ok2 := d.ObjectB.(schemasnapshot.ObjectRef)
		if !ok1 || !ok2 || oldRef == newRef {
			continue
		}
		add(oldRef, newRef)
		add(newRef, oldRef)
	}
	return aliases
}

// passesObjectTypeFilter returns true if the finding's object-type bucket is
// allowed by the include list. An empty includeTypes means "all".
func passesObjectTypeFilter(d Difference, includeTypes map[ObjectType]struct{}) bool {
	if len(includeTypes) == 0 {
		return true
	}
	_, ok := includeTypes[d.ObjectType]
	return ok
}

// passesObjectTypeExcludeFilter returns true if the finding is NOT in the
// exclude list. An empty excludeTypes means "nothing excluded".
func passesObjectTypeExcludeFilter(d Difference, excludeTypes map[ObjectType]struct{}) bool {
	if len(excludeTypes) == 0 {
		return true
	}
	_, excluded := excludeTypes[d.ObjectType]
	return !excluded
}

// passesTableIncludeFilter keeps a finding under the Tables include filter:
//   - empty list keeps all
//   - no derived anchor never matches a non-empty list
//   - otherwise keep if the anchor or any of its rename aliases is listed (either-side)
func passesTableIncludeFilter(d Difference, includeTables map[schemasnapshot.ObjectRef]struct{}, aliases map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef) bool {
	if len(includeTables) == 0 {
		return true
	}
	anchor, ok := anchorTableOf(d)
	if !ok {
		return false
	}

	// Check the anchor itself.
	if _, ok := includeTables[anchor]; ok {
		return true
	}

	// Check all rename aliases of the anchor.
	for _, alias := range aliases[anchor] {
		if _, ok := includeTables[alias]; ok {
			return true
		}
	}

	return false
}

// passesTableExcludeFilter drops a finding under the ExcludeTables filter:
//   - empty list excludes nothing
//   - no derived anchor is never excluded
//   - otherwise drop if the anchor or any of its rename aliases is listed (either-side)
func passesTableExcludeFilter(d Difference, excludeTables map[schemasnapshot.ObjectRef]struct{}, aliases map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef) bool {
	if len(excludeTables) == 0 {
		return true
	}
	anchor, ok := anchorTableOf(d)
	if !ok {
		return true // no anchor is never excluded by ExcludeTables
	}

	if _, ok := excludeTables[anchor]; ok {
		return false
	}

	// Check all rename aliases of the anchor.
	for _, alias := range aliases[anchor] {
		if _, ok := excludeTables[alias]; ok {
			return false
		}
	}

	return true
}

// toSet builds a lookup set from a slice for O(1) membership tests; returns nil
// for an empty/nil input. For ObjectRef, membership is exact case-sensitive
// struct equality.
func toSet[T comparable](items []T) map[T]struct{} {
	if len(items) == 0 {
		return nil
	}
	m := make(map[T]struct{}, len(items))
	for _, x := range items {
		m[x] = struct{}{}
	}
	return m
}
