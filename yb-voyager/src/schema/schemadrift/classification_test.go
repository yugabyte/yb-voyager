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

package schemadrift

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
)

// TestClassify_MappedTypes pins the severity assigned to every mapped change type.
// Severity answers "what does the migration do", not "how alarming is the DDL":
// an ADDED column can fail import data (recoverable), a DROPPED column cannot
// (it only leaves the target with an extra column), and dropping or renaming a
// captured table makes export data unrestartable.
func TestClassify_MappedTypes(t *testing.T) {
	cases := map[schemadiff.DiffType]Severity{
		// export data cannot be restarted afterwards -> restart from scratch.
		schemadiff.TableDropped:       SeverityBreaksUnrecoverable,
		schemadiff.TableNameChanged:   SeverityBreaksUnrecoverable,
		schemadiff.TableSchemaChanged: SeverityBreaksUnrecoverable,

		// import data can fail until the DDL is applied on the target.
		schemadiff.ColumnAdded:              SeverityBreaksRecoverable,
		schemadiff.ColumnNameChanged:        SeverityBreaksRecoverable,
		schemadiff.ColumnTypeChanged:        SeverityBreaksRecoverable,
		schemadiff.ColumnNullabilityChanged: SeverityBreaksRecoverable,

		// Migration unaffected, but source and target diverge.
		schemadiff.TableAdded:                    SeverityPotentialImpact,
		schemadiff.ColumnDropped:                 SeverityPotentialImpact,
		schemadiff.ColumnDefaultChanged:          SeverityPotentialImpact,
		schemadiff.TableKindChanged:              SeverityPotentialImpact,
		schemadiff.TablePartitionParentChanged:   SeverityPotentialImpact,
		schemadiff.TablePartitionChildrenChanged: SeverityPotentialImpact,

		schemadiff.TableInheritsChanged:    SeverityAdvisory,
		schemadiff.TableInheritedByChanged: SeverityAdvisory,
	}

	for diffType, want := range cases {
		t.Run(string(diffType), func(t *testing.T) {
			assert.Equal(t, want, classify(diffType).Severity)
		})
	}
}

func TestClassify_UnknownDefaultsToAdvisory(t *testing.T) {
	for _, unknown := range []schemadiff.DiffType{"", "SOME_FUTURE_DIFF_TYPE"} {
		got := classify(unknown)
		assert.Equal(t, SeverityAdvisory, got.Severity)
		assert.Empty(t, got.Impact, "an unmapped type has no note, so the report omits it")
		assert.Empty(t, got.Action)
	}
}

// TestClassify_MappedTypesCarryGuidance is why severity and wording share one map:
// an entry declared with a severity but no note would render a finding the report
// cannot explain, and nothing else would catch it.
func TestClassify_MappedTypesCarryGuidance(t *testing.T) {
	for diffType := range infoByDiffType {
		t.Run(string(diffType), func(t *testing.T) {
			c := classify(diffType)
			assert.NotEmpty(t, c.Severity)
			assert.NotEmpty(t, c.Impact, "every classified change explains its impact")
			assert.NotEmpty(t, c.Action, "every classified change says what to do")
		})
	}
}
