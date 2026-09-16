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

// TestClassify_MappedTypes pins the severity policy against the design mockup.
// Severity answers "what does the migration do", not "how alarming is the DDL":
// an ADDED column can fail import data (recoverable), a DROPPED column cannot
// (it only leaves the target with an extra column), and dropping or renaming a
// captured table makes export data unrestartable.
func TestClassify_MappedTypes(t *testing.T) {
	cases := map[schemadiff.DiffType]Status{
		// export data cannot be restarted afterwards -> restart from scratch.
		schemadiff.TableDropped:       StatusBreaksUnrecoverable,
		schemadiff.TableNameChanged:   StatusBreaksUnrecoverable,
		schemadiff.TableSchemaChanged: StatusBreaksUnrecoverable,

		// import data can fail until the DDL is applied on the target.
		schemadiff.ColumnAdded:              StatusBreaksRecoverable,
		schemadiff.ColumnNameChanged:        StatusBreaksRecoverable,
		schemadiff.ColumnTypeChanged:        StatusBreaksRecoverable,
		schemadiff.ColumnNullabilityChanged: StatusBreaksRecoverable,

		// Migration unaffected, but source and target diverge.
		schemadiff.TableAdded:                    StatusPotentialImpact,
		schemadiff.ColumnDropped:                 StatusPotentialImpact,
		schemadiff.ColumnDefaultChanged:          StatusPotentialImpact,
		schemadiff.TableKindChanged:              StatusPotentialImpact,
		schemadiff.TablePartitionParentChanged:   StatusPotentialImpact,
		schemadiff.TablePartitionChildrenChanged: StatusPotentialImpact,

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
