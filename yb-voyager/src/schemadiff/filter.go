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
// the values accepted by --object-type-list. v1 emits and filters only TABLE
// findings, so only TABLE is declared; the remaining selectors are re-enabled as
// the engine starts emitting their findings.
type ObjectType string

const (
	ObjectTypeTable ObjectType = "TABLE"
	// Not yet emitted by the diff engine — uncomment each as its findings land
	// (INDEX classifies here, not under TABLE, but stays anchored to its host
	// table for --table-list):
	// ObjectTypeIndex    ObjectType = "INDEX"
	// ObjectTypeSequence ObjectType = "SEQUENCE"
	// ObjectTypeView     ObjectType = "VIEW"
	// ObjectTypeFunction ObjectType = "FUNCTION"
	// ObjectTypeType     ObjectType = "TYPE"
)

// Scope describes the include/exclude filters applied by FilterByScope.
//
// Tables/ExcludeTables hold caller-RESOLVED ObjectRefs (globs and the default
// schema already expanded); matching is exact, case-sensitive struct equality
// against Difference.AnchorTable. ObjectTypes/ExcludeObjectTypes match a finding's
// object-type bucket. An empty include list means "all".
//
// Scope is permissive and total — it never errors. If an include and its exclude
// are both set, exclude wins (includes apply first, excludes last); an entry that
// matches nothing is a silent no-op. Flag-level policy — e.g. --table-list and
// --exclude-table-list being mutually exclusive — is the command's to enforce, so
// FilterByScope stays pure for any caller.
type Scope struct {
	Tables             []schemasnapshot.ObjectRef // empty = all; matched against AnchorTable
	ExcludeTables      []schemasnapshot.ObjectRef // drop findings whose AnchorTable is in this list
	ObjectTypes        []ObjectType               // empty = all
	ExcludeObjectTypes []ObjectType
}

// FilterByScope returns the subset of diffs within scope. It is pure: inputs are
// never mutated and the result is a fresh slice; a name matching nothing is a
// silent no-op (validation is the caller's job).
//
// Order: (1) build a table rename/move alias map so a finding anchored to a renamed
// table matches on either its old or new identity; (2) include by ObjectTypes;
// (3) include by Tables; (4) exclude by ObjectTypes; (5) exclude by Tables. A nil
// AnchorTable never matches a non-empty Tables list.
func FilterByScope(diffs []Difference, scope Scope) []Difference {
	// Bidirectional old<->new alias map for renamed/moved tables, so a finding
	// anchored to a renamed table matches on either identity (keyed by ObjectRef).
	tableRenameAliases := buildTableRenameAliases(diffs)

	// Pre-build lookup sets for the four lists to avoid O(n²) inner scans.
	includeTypes := makeObjectTypeSet(scope.ObjectTypes)
	excludeTypes := makeObjectTypeSet(scope.ExcludeObjectTypes)
	includeTables := makeObjectRefSet(scope.Tables)
	excludeTables := makeObjectRefSet(scope.ExcludeTables)

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

// buildTableRenameAliases maps each table identity to the identities it aliases,
// from TABLE_NAME_CHANGED (new name) and TABLE_SCHEMA_CHANGED (new schema)
// findings, so either side of a rename/move can stand in during table matching.
//
// A table may change both name and schema in one interval; those arrive as two
// findings sharing the same side-A anchor, so we group by the old ref and rebuild
// the full new ref from whichever parts changed. Aliases accumulate ([]ObjectRef)
// so chains like a→b, b→c don't clobber each other. No-op changes are skipped.
func buildTableRenameAliases(diffs []Difference) map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef {
	// Group the per-attribute identity changes by the side-A (old) ref so a
	// rename-and-move pair is recombined into one old→new mapping.
	type identityChange struct {
		oldRef    schemasnapshot.ObjectRef
		newSchema string // "" => schema unchanged
		newName   string // "" => name unchanged
	}
	changes := make(map[schemasnapshot.ObjectRef]*identityChange)
	at := func(old schemasnapshot.ObjectRef) *identityChange {
		c := changes[old]
		if c == nil {
			c = &identityChange{oldRef: old}
			changes[old] = c
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

	aliases := make(map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef)
	for _, c := range changes {
		newRef := c.oldRef
		if c.newSchema != "" {
			newRef.Schema = c.newSchema
		}
		if c.newName != "" {
			newRef.Name = c.newName
		}
		oldRef := c.oldRef
		if oldRef == newRef {
			continue // no-op
		}
		aliases[oldRef] = append(aliases[oldRef], newRef)
		aliases[newRef] = append(aliases[newRef], oldRef)
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

// passesTableIncludeFilter keeps a finding under the Tables include filter: an
// empty list keeps all; a nil AnchorTable never matches a non-empty list;
// otherwise keep if the anchor or any of its rename aliases is listed (either-side).
func passesTableIncludeFilter(d Difference, includeTables map[schemasnapshot.ObjectRef]struct{}, aliases map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef) bool {
	if len(includeTables) == 0 {
		return true
	}
	if d.AnchorTable == nil {
		return false
	}
	anchor := *d.AnchorTable

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

// passesTableExcludeFilter drops a finding under the ExcludeTables filter: an empty
// list excludes nothing; a nil AnchorTable is never excluded; otherwise drop if the
// anchor or any of its rename aliases is listed (either-side).
func passesTableExcludeFilter(d Difference, excludeTables map[schemasnapshot.ObjectRef]struct{}, aliases map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef) bool {
	if len(excludeTables) == 0 {
		return true
	}
	if d.AnchorTable == nil {
		return true // nil anchor is never excluded by ExcludeTables
	}
	anchor := *d.AnchorTable

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

// makeObjectRefSet converts a []schemasnapshot.ObjectRef into a map for O(1)
// exact, case-sensitive struct-equality lookup.
func makeObjectRefSet(refs []schemasnapshot.ObjectRef) map[schemasnapshot.ObjectRef]struct{} {
	if len(refs) == 0 {
		return nil
	}
	m := make(map[schemasnapshot.ObjectRef]struct{}, len(refs))
	for _, r := range refs {
		m[r] = struct{}{}
	}
	return m
}
