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

import "github.com/yugabyte/yb-voyager/yb-voyager/src/constants"

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
//   - stable-ID keys when both sides are the same engine AND that engine exposes
//     stable catalog IDs (Postgres/YugabyteDB: pg_class.oid, attnum). Matching by
//     ID lets a rename be detected (the ID is stable across it) instead of
//     surfacing as drop+add. An engine's objects uniformly carry IDs or uniformly
//     don't, so this is a property of the engine, not something to check per object.
//   - name keys otherwise — a cross-engine diff, or an engine without stable
//     per-object IDs (e.g. MySQL). Renames then read as drop+add.
//
// TODO(schemadiff): also require the two snapshots to be from the SAME side/
// instance before ID-matching — two different servers of the same engine have
// independent OID spaces, so matching by ID across them is wrong. That needs
// Side/instance identity plumbed from SnapshotHeader into Diff. Until then we
// assume same-instance, which holds for detect-drift v1 (one source over time).
func chooseMatchKeys[T any](
	dbTypeA, dbTypeB string,
	idOf func(T) string,
	nameKey func(obj T, dbType string) string,
) (keyA, keyB func(T) string) {
	if dbTypeA == dbTypeB && (dbTypeA == constants.POSTGRESQL || dbTypeA == constants.YUGABYTEDB) {
		return idOf, idOf
	}
	return func(t T) string { return nameKey(t, dbTypeA) },
		func(t T) string { return nameKey(t, dbTypeB) }
}
