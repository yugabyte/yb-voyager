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
// positive allow-lists and an empty (or nil) list means "all".
//
// There is deliberately no exclude counterpart. Exclude-style user input — e.g.
// --exclude-table-list — is resolved by the command layer into the concrete set of
// objects to KEEP before it reaches the engine, because only the command knows the
// universe to subtract from (the tables present across the compared snapshots plus
// the live read) and only the command expands globs. Keeping one filtering
// direction here means there is no include-vs-exclude precedence rule to define,
// and no second code path that production never exercises.
//
// Tables holds caller-RESOLVED ObjectRefs (globs and the default schema already
// expanded); matching is exact, case-sensitive struct equality against the
// finding's anchor table, derived from its identity by anchorTableOf. ObjectTypes
// matches a finding's object-type bucket.
//
// Scope is permissive and total — it never errors, and an entry that matches
// nothing is a silent no-op. Flag-level policy — e.g. --table-list and
// --exclude-table-list being mutually exclusive — is the command's to enforce, so
// FilterByScope stays pure for any caller.
type Scope struct {
	Tables      []schemasnapshot.ObjectRef // empty = all; matched against the finding's derived anchor table
	ObjectTypes []ObjectType               // empty = all; matched against the finding's ObjectType
}

// FilterByScope returns the subset of diffs within scope. It is pure: inputs are
// never mutated and the result is a fresh slice; a name matching nothing is a
// silent no-op (validation is the caller's job).
//
// Filtering applies, in order:
//  1. ObjectTypes
//  2. Tables (a finding with no derived anchor never matches a non-empty list)
//
// NOTE: table rename/move alias handling is temporarily disabled (see the body).
// With it off, a finding anchored to a renamed table matches only its as-emitted
// anchor, not its old/new counterpart. Pending the cross-window alias decision.
//
// KNOWN GAP (confirmed end-to-end against PostgreSQL, 2026-07-30): this makes
// `schema detect-drift --table-list` unable to return a renamed table's full drift
// history, whichever name is given. anchorTableOf derives the anchor from ObjectA
// (falling back to ObjectB), so a TABLE_NAME_CHANGED anchors to the OLD ref while
// every later finding on that table anchors to the NEW ref — and with the alias off,
// nothing bridges the two. Renaming sales."Mixed Case Tbl" to sales."MixedCase" and
// then adding a column to it yields:
//
//	--table-list sales.MixedCase          -> COLUMN_ADDED only (rename dropped)
//	--table-list 'sales.Mixed Case Tbl'   -> TABLE_NAME_CHANGED only (column add dropped)
//
// Unfiltered runs are unaffected: both findings are always reported.
//
// Re-enabling the alias below fixes this only WITHIN one window. driftreport
// diffs each consecutive snapshot pair and calls FilterByScope per pair, so the
// rename lives in exactly one window and cannot alias findings in the others. A
// real fix needs either a rename alias map built across all windows before
// filtering, or a canonical anchor keyed by stable table OID with --table-list
// names resolved to OIDs.
func FilterByScope(diffs []Difference, scope Scope) []Difference {
	// Rename/move alias handling is temporarily disabled pending the cross-window
	// alias decision (PR #3648 discussion). Preserved for re-enable: the builder
	// (buildTableRenameAliases, below) and the alias branch in passesTableFilter.
	// To re-enable: uncomment the builder line below, restore the `aliases`
	// parameter + the commented alias block in passesTableFilter, and pass
	// tableRenameAliases to that call.
	// tableRenameAliases := buildTableRenameAliases(diffs)

	// Pre-build lookup sets for both lists to avoid O(n²) inner scans.
	includeTypes := toSet(scope.ObjectTypes)
	includeTables := toSet(scope.Tables)

	out := make([]Difference, 0, len(diffs))
	for _, d := range diffs {
		if !passesObjectTypeFilter(d, includeTypes) {
			continue
		}
		if !passesTableFilter(d, includeTables) {
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
	if len(includeTypes) == 0 {
		return true
	}
	_, ok := includeTypes[d.ObjectType]
	return ok
}

// passesTableFilter keeps a finding under the Tables filter:
//   - empty list keeps all
//   - no derived anchor never matches a non-empty list
//   - otherwise keep if the anchor is listed
//
// Rename-alias either-side matching is temporarily disabled (see FilterByScope).
// Re-enable by restoring the `aliases map[...]` parameter and the commented block.
func passesTableFilter(d Difference, includeTables map[schemasnapshot.ObjectRef]struct{}) bool {
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
