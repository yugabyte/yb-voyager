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
}

var primaryKeyTypes = kindTypes{PrimaryKeyAdded, PrimaryKeyDropped, PrimaryKeyNameChanged}

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
		return diffMatchedConstraint(primaryKeyTypes, *keyA, *keyB)
	}

	var diffs []Difference
	if own := ownConstraint(keyA); own != nil {
		diffs = append(diffs, newDifference(primaryKeyTypes.dropped, OpDropped, ObjectTypeConstraint, AttrNone, own.TableScopedObjectRef, nil, own.Definition, nil))
	}
	if own := ownConstraint(keyB); own != nil {
		diffs = append(diffs, newDifference(primaryKeyTypes.added, OpAdded, ObjectTypeConstraint, AttrNone, nil, own.TableScopedObjectRef, nil, own.Definition))
	}
	return diffs
}

// diffMatchedConstraint reports a rename of a constraint present on both sides under
// one OID. A copy on either side reports nothing: renaming the parent's object renames
// its copies, and attaching, detaching and inheriting have table-level findings.
func diffMatchedConstraint(types kindTypes, cA, cB schemasnapshot.Constraint) []Difference {
	if cA.FromParent || cB.FromParent || cA.Name == cB.Name {
		return nil
	}
	return []Difference{newDifference(types.nameChanged, OpChanged, ObjectTypeConstraint, AttrName, cA.TableScopedObjectRef, cB.TableScopedObjectRef, cA.Name, cB.Name)}
}
