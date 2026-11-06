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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// kindTypes is the set of DiffTypes one kind of key or index reports.
type kindTypes struct {
	added, dropped, nameChanged DiffType
	objectType                  ObjectType
}

var (
	primaryKeyTypes = kindTypes{PrimaryKeyAdded, PrimaryKeyDropped, PrimaryKeyNameChanged, ObjectTypeConstraint}
	uniqueTypes     = kindTypes{UniqueAdded, UniqueDropped, UniqueNameChanged, ObjectTypeConstraint}
	checkTypes      = kindTypes{CheckAdded, CheckDropped, CheckNameChanged, ObjectTypeConstraint}
	foreignKeyTypes = kindTypes{ForeignKeyAdded, ForeignKeyDropped, ForeignKeyNameChanged, ObjectTypeConstraint}

	uniqueIndexTypes = kindTypes{UniqueIndexAdded, UniqueIndexDropped, UniqueIndexNameChanged, ObjectTypeIndex}
	indexTypes       = kindTypes{IndexAdded, IndexDropped, IndexNameChanged, ObjectTypeIndex}
)

// objectRef is what the diff reads of a constraint or index; generic code cannot
// read struct fields, so each element type converts itself to this.
type objectRef struct {
	ID         string
	Ref        schemasnapshot.TableScopedObjectRef
	Definition string
	FromParent bool
}

func constraintRef(c schemasnapshot.Constraint) objectRef {
	return objectRef{ID: c.ID, Ref: c.TableScopedObjectRef, Definition: c.Definition, FromParent: c.FromParent}
}

func indexRef(i schemasnapshot.Index) objectRef {
	return objectRef{ID: i.ID, Ref: i.TableScopedObjectRef, Definition: i.Definition, FromParent: i.FromParent}
}

func primaryKeyAsSlice(key *schemasnapshot.Constraint) []schemasnapshot.Constraint {
	if key == nil {
		return nil
	}
	return []schemasnapshot.Constraint{*key}
}

// diffConstraintsOrIndexes matches the constraints or indexes of one kind on two versions of a table
// by OID, in the order of the inputs. A new OID under an existing name reports a drop
// and an add. Copies made for a partition or an inheritance child report nothing.
func diffConstraintsOrIndexes[T schemasnapshot.Constraint | schemasnapshot.Index](earlier, later []T, toRef func(T) objectRef, types kindTypes) []Difference {
	laterByID := make(map[string]objectRef, len(later))
	for _, item := range later {
		r := toRef(item)
		laterByID[r.ID] = r
	}
	earlierIDs := make(map[string]struct{}, len(earlier))

	var diffs []Difference
	for _, item := range earlier {
		rEarlier := toRef(item)
		earlierIDs[rEarlier.ID] = struct{}{}
		if rLater, ok := laterByID[rEarlier.ID]; ok {
			diffs = append(diffs, diffMatchedConstraintOrIndex(types, rEarlier, rLater)...)
			continue
		}
		if !rEarlier.FromParent {
			diffs = append(diffs, droppedDifference(types, rEarlier))
		}
	}
	for _, item := range later {
		rLater := toRef(item)
		if _, ok := earlierIDs[rLater.ID]; ok || rLater.FromParent {
			continue
		}
		diffs = append(diffs, addedDifference(types, rLater))
	}
	return diffs
}

func droppedDifference(types kindTypes, r objectRef) Difference {
	return newDifference(types.dropped, OpDropped, types.objectType, AttrNone, r.Ref, nil, r.Definition, nil)
}

func addedDifference(types kindTypes, r objectRef) Difference {
	return newDifference(types.added, OpAdded, types.objectType, AttrNone, nil, r.Ref, nil, r.Definition)
}

// diffMatchedConstraintOrIndex reports a rename of a constraint or index present on both sides under
// one OID. A copy on either side reports nothing: renaming the parent's object renames
// its copies, and attaching, detaching and inheriting have table-level findings.
func diffMatchedConstraintOrIndex(types kindTypes, a, b objectRef) []Difference {
	if a.FromParent || b.FromParent || a.Ref.Name == b.Ref.Name {
		return nil
	}
	return []Difference{newDifference(types.nameChanged, OpChanged, types.objectType, AttrName, a.Ref, b.Ref, a.Ref.Name, b.Ref.Name)}
}
