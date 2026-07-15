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

// matchByKey is the single object-matching primitive shared by table and column
// diffing. It indexes A and B by a per-side key function, then:
//   - key on both sides → onMatch(a, b)   (field-level comparison)
//   - key only on A      → onDropped(a)
//   - key only on B      → onAdded(b)
//
// keyA/keyB are separate so a cross-engine diff can render each side's key in its
// own dialect. Output order is unspecified; Diff sorts the final slice.
func matchByKey[T any](
	as, bs []T,
	keyA, keyB func(T) string,
	onMatch func(a, b T) []Difference,
	onDropped func(a T) []Difference,
	onAdded func(b T) []Difference,
) []Difference {
	byKeyA := make(map[string]T, len(as))
	for _, a := range as {
		byKeyA[keyA(a)] = a
	}
	byKeyB := make(map[string]T, len(bs))
	for _, b := range bs {
		byKeyB[keyB(b)] = b
	}
	var diffs []Difference
	for k, a := range byKeyA {
		if b, ok := byKeyB[k]; ok {
			diffs = append(diffs, onMatch(a, b)...)
		} else {
			diffs = append(diffs, onDropped(a)...)
		}
	}
	for k, b := range byKeyB {
		if _, ok := byKeyA[k]; !ok {
			diffs = append(diffs, onAdded(b)...)
		}
	}
	return diffs
}

// chooseMatchKeys selects how objects are matched between two snapshots:
//   - stable-ID keys when IDs are USABLE — same engine (IDs compare only within
//     an engine) AND every object on both sides has a non-empty ID. This is what
//     lets a rename be detected (ID is stable across it) instead of drop+add.
//   - name keys otherwise — cross-engine diffs, or same-engine sources with no
//     stable per-object ID (e.g. MySQL). Renames then read as drop+add.
//
// The predicate is "IDs usable", NOT merely "same engine": a same-engine source
// without IDs must still match by name, else every object collides on "".
func chooseMatchKeys[T any](
	dbTypeA, dbTypeB string,
	as, bs []T,
	idOf func(T) string,
	nameKey func(obj T, dbType string) string,
) (keyA, keyB func(T) string) {
	if dbTypeA == dbTypeB && allHaveID(as, idOf) && allHaveID(bs, idOf) {
		return idOf, idOf
	}
	return func(t T) string { return nameKey(t, dbTypeA) },
		func(t T) string { return nameKey(t, dbTypeB) }
}

// allHaveID reports whether every object carries a non-empty stable ID (empty set
// is vacuously true). Falls back to name matching the moment any object lacks one,
// rather than letting empty IDs collide.
func allHaveID[T any](items []T, idOf func(T) string) bool {
	for _, it := range items {
		if idOf(it) == "" {
			return false
		}
	}
	return true
}
