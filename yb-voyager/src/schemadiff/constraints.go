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

// ownConstraint returns the table's own constraint: nil for none, and for a copy
// made for a partition or an inheritance child.
func ownConstraint(c *schemasnapshot.Constraint) *schemasnapshot.Constraint {
	if c == nil || c.FromParent {
		return nil
	}
	return c
}

// diffPrimaryKey matches the single primary key slot of two versions of a table by
// OID. A new OID on both sides reports a drop and an add, which overstates a
// re-create rather than hiding it.
func diffPrimaryKey(tA, tB schemasnapshot.Table) []Difference {
	keyA, keyB := tA.PrimaryKey, tB.PrimaryKey
	if keyA != nil && keyB != nil && keyA.ID == keyB.ID {
		return diffMatchedConstraint(primaryKeyTypes, constraintRef(*keyA), constraintRef(*keyB))
	}

	var diffs []Difference
	if own := ownConstraint(keyA); own != nil {
		diffs = append(diffs, droppedDifference(primaryKeyTypes, constraintRef(*own)))
	}
	if own := ownConstraint(keyB); own != nil {
		diffs = append(diffs, addedDifference(primaryKeyTypes, constraintRef(*own)))
	}
	return diffs
}

// diffByOID matches the constraints or indexes of one kind on two versions of a table
// by OID, in the order of the inputs. A new OID under an existing name reports a drop
// and an add. Copies made for a partition or an inheritance child report nothing.
func diffByOID[T schemasnapshot.Constraint | schemasnapshot.Index](earlier, later []T, toRef func(T) objectRef, types kindTypes) []Difference {
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
			diffs = append(diffs, diffMatchedConstraint(types, rEarlier, rLater)...)
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

// diffMatchedConstraint reports a rename of a constraint present on both sides under
// one OID. A copy on either side reports nothing: renaming the parent's object renames
// its copies, and attaching, detaching and inheriting have table-level findings.
func diffMatchedConstraint(types kindTypes, a, b objectRef) []Difference {
	if a.FromParent || b.FromParent || a.Ref.Name == b.Ref.Name {
		return nil
	}
	return []Difference{newDifference(types.nameChanged, OpChanged, types.objectType, AttrName, a.Ref, b.Ref, a.Ref.Name, b.Ref.Name)}
}
