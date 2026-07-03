//go:build unit

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

package driftreport

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
)

func TestClassify_MappedTypes(t *testing.T) {
	cases := map[schemadiff.DiffType]Status{
		schemadiff.TableDropped:       StatusBreaksRecoverable,
		schemadiff.ColumnDropped:      StatusBreaksRecoverable,
		schemadiff.ColumnNameChanged:  StatusBreaksRecoverable,
		schemadiff.TableNameChanged:   StatusBreaksRecoverable,
		schemadiff.TableSchemaChanged: StatusBreaksRecoverable,
		schemadiff.ColumnTypeChanged:  StatusBreaksRecoverable,

		schemadiff.TableAdded:               StatusPotentialImpact,
		schemadiff.ColumnAdded:              StatusPotentialImpact,
		schemadiff.ColumnNullabilityChanged: StatusPotentialImpact,
		schemadiff.TableKindChanged:         StatusPotentialImpact,
		schemadiff.PartitionParentChanged:   StatusPotentialImpact,
		schemadiff.PartitionChildrenChanged: StatusPotentialImpact,

		schemadiff.ColumnDefaultChanged:    StatusAdvisory,
		schemadiff.TableInheritsChanged:    StatusAdvisory,
		schemadiff.TableInheritedByChanged: StatusAdvisory,
	}

	for diffType, want := range cases {
		t.Run(string(diffType), func(t *testing.T) {
			assert.Equal(t, want, Classify(diffType))
		})
	}
}

func TestClassify_UnknownDefaultsToAdvisory(t *testing.T) {
	assert.Equal(t, StatusAdvisory, Classify(schemadiff.DiffType("")))
	assert.Equal(t, StatusAdvisory, Classify(schemadiff.DiffType("SOME_FUTURE_DIFF_TYPE")))
}
