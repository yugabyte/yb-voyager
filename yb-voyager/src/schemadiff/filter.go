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

// Scope is the caller-resolved filter applied by FilterByScope. Both lists are
// positive allow-lists holding the EXACT set to keep. Refs must already be resolved
// (globs and the default schema expanded) — matching is exact struct equality.
//
// Empty means empty, not "all": an unfiltered run passes the whole universe
// explicitly. Reading empty as "keep everything" would make it carry two meanings —
// "the caller did not filter" and "the caller excluded everything" — and invert the
// second into its opposite.
//
// There is deliberately no exclude counterpart: only the command knows the universe
// to subtract from, so it resolves --exclude-* into a keep-set before calling here.
//
// Never errors; an entry matching nothing is a no-op. Flag-level policy (e.g.
// --table-list vs --exclude-table-list) is the command's to enforce.
type Scope struct {
	Schemas     []string                   // the exact set to keep; matched against EITHER side's schema
	Tables      []schemasnapshot.ObjectRef // the exact set to keep; matched against the finding's derived anchor table
	ObjectTypes []ObjectType               // the exact set to keep; matched against the finding's ObjectType
}

// FilterByScope returns the subset of diffs within scope. It is pure: inputs are
// never mutated and the result is a fresh slice; a name matching nothing is a
// silent no-op (validation is the caller's job).
//
// Filtering applies, in order:
//  1. ObjectTypes
//  2. Schemas (either side's — see passesSchemaFilter)
//  3. Tables (a finding with no anchor table passes — see passesTableFilter)
//
// The schema dimension is applied HERE, after diffing, and must not be replaced by
// narrowing each snapshot's content first: projecting both sides down to the
// requested schemas turns a table moving public → sales into a TABLE_DROPPED,
// because side B no longer holds it at all. The engine has to see both schemas to
// recognise the move; only then can the finding be judged in or out of scope.
//
// NOTE: table rename/move alias handling is temporarily disabled (see the body).
// With it off, a finding anchored to a renamed table matches only its as-emitted
// anchor, not its old/new counterpart. Pending the cross-window alias decision.
//
// KNOWN GAP: with the alias off, --table-list cannot return a renamed table's full
// history. anchorTableOf prefers the side-A identity, so a finding about the table
// under its old name anchors to the old ref while later findings anchor to the new
// one, and neither name matches both. Unfiltered runs report both.
//
// Re-enabling the alias below only fixes this within one window, since schemadrift
// filters each snapshot pair separately. A real fix needs a cross-window alias map,
// or anchors keyed by stable table OID.
func FilterByScope(diffs []Difference, scope Scope) []Difference {
	// Rename/move alias handling is temporarily disabled pending the cross-window
	// alias decision (PR #3648 discussion). Preserved for re-enable: the builder
	// (buildTableRenameAliases, below) and the alias branch in passesTableFilter.
	// To re-enable: uncomment the builder line below, restore the `aliases`
	// parameter + the commented alias block in passesTableFilter, and pass
	// tableRenameAliases to that call.
	// tableRenameAliases := buildTableRenameAliases(diffs)

	// Pre-build lookup sets for every list to avoid O(n²) inner scans.
	includeTypes := toSet(scope.ObjectTypes)
	includeSchemas := toSet(scope.Schemas)
	includeTables := toSet(scope.Tables)

	out := make([]Difference, 0, len(diffs))
	for _, d := range diffs {
		if !passesObjectTypeFilter(d, includeTypes) {
			continue
		}
		if !passesSchemaFilter(d, includeSchemas) {
			continue
		}
		if !passesTableFilter(d, includeTables) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// passesSchemaFilter keeps a finding when EITHER side's schema is listed.
//
// Either side, because a table that moves between schemas has a different schema on
// each: public.orders → sales.orders must still be reported to someone who asked
// only about public, since their table left their scope. Matching side B alone would
// hide it; matching side A alone would hide the reverse move into scope.
func passesSchemaFilter(d Difference, includeSchemas map[string]struct{}) bool {
	for _, s := range schemasOf(d) {
		if _, ok := includeSchemas[s]; ok {
			return true
		}
	}
	return false
}

// schemasOf returns the schemas a finding touches, one per populated side. A
// table-scoped object (a column) reports its parent table's schema. Unlike
// anchorTableOf there is no "none" case: every identity carries a schema, including
// a top-level object's.
func schemasOf(d Difference) []string {
	out := make([]string, 0, 2)
	for _, id := range []ObjectIdent{d.ObjectA, d.ObjectB} {
		switch v := id.(type) {
		case schemasnapshot.ObjectRef:
			out = append(out, v.Schema)
		case schemasnapshot.TableScopedObjectRef:
			out = append(out, v.Table.Schema)
		}
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
	case schemasnapshot.TableScopedObjectRef:
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
//
// TEMPORARILY DISABLED (see FilterByScope): the whole function is commented out
// while rename/move alias handling is off, pending the cross-window alias
// decision. Preserved verbatim for re-enable — do not delete.
/*
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
*/

// passesObjectTypeFilter returns true if the finding's object-type bucket is
// allowed by the include list. An empty includeTypes means "all".
func passesObjectTypeFilter(d Difference, includeTypes map[ObjectType]struct{}) bool {
	_, ok := includeTypes[d.ObjectType]
	return ok
}

// passesTableFilter keeps a finding under the Tables filter:
//   - no anchor table keeps it, whatever the list holds
//   - otherwise keep it if the anchor is listed
//
// Rename-alias either-side matching is temporarily disabled (see FilterByScope).
// Re-enable by restoring the `aliases map[...]` parameter and the commented block.
func passesTableFilter(d Difference, includeTables map[schemasnapshot.ObjectRef]struct{}) bool {
	anchor, ok := anchorTableOf(d)
	if !ok {
		// A view, a function, a sequence: no host table, so a table list has nothing
		// to say about it. Selecting object KINDS is --object-type-list's dimension,
		// and conflating the two would make drift vanish from the report silently.
		return true
	}

	// Check the anchor itself.
	if _, ok := includeTables[anchor]; ok {
		return true
	}

	// Rename-alias matching disabled — see FilterByScope. Re-enable with an
	// `aliases map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef` parameter:
	// for _, alias := range aliases[anchor] {
	// 	if _, ok := includeTables[alias]; ok {
	// 		return true
	// 	}
	// }

	return false
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
