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
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
)

var requiredTimings = []Timing{
	TimingBeforeExportData,
	TimingOfflineAfterExportStart,
	TimingLiveAfterExportStart,
}

func TestInfoCoversEveryDiffTypeAndTiming(t *testing.T) {
	assert.ElementsMatch(t, schemadiff.AllDiffTypes(), lo.Keys(infoByDiffType),
		"infoByDiffType must have exactly one entry per DiffType the engine emits")

	for _, diffType := range schemadiff.AllDiffTypes() {
		t.Run(string(diffType), func(t *testing.T) {
			byTiming := infoByDiffType[diffType]
			for _, timing := range requiredTimings {
				info, ok := byTiming[timing]
				require.True(t, ok, "missing timing %s", timing)
				assertCompleteInfo(t, info)
			}
			for timing, info := range byTiming {
				if timing != TimingChangesOnlyAfterExportStart {
					assert.Contains(t, requiredTimings, timing, "after cutover is the same for every type, so no type lists it")
				}
				assertCompleteInfo(t, info)
			}
		})
	}
	for diffType, risk := range fallBackRiskByDiffType {
		assert.Contains(t, schemadiff.AllDiffTypes(), diffType)
		assert.NotEmpty(t, risk)
	}
}

func assertCompleteInfo(t *testing.T, info DriftInfo) {
	t.Helper()
	assert.Contains(t, []Severity{SeverityAdvisory, SeverityPotentialImpact, SeverityBreaksRecoverable, SeverityBreaksUnrecoverable}, info.Severity)
	assert.NotEmpty(t, info.Impact, "every classified change explains its impact")
	assert.NotEmpty(t, info.Action, "every classified change says what to do")
}

// Severity answers "what does the migration do", not "how alarming is the DDL":
// an ADDED column can fail import data (recoverable), a DROPPED column cannot
// (it only leaves the target with an extra column), and dropping or renaming a
// captured table makes export data unrestartable.
func TestGetDriftInfo_DefaultSeverities(t *testing.T) {
	want := map[schemadiff.DiffType]Severity{
		schemadiff.TableDropped:       SeverityBreaksUnrecoverable,
		schemadiff.TableNameChanged:   SeverityBreaksUnrecoverable,
		schemadiff.TableSchemaChanged: SeverityBreaksUnrecoverable,

		schemadiff.ColumnAdded:          SeverityBreaksRecoverable,
		schemadiff.ColumnNameChanged:    SeverityBreaksRecoverable,
		schemadiff.ColumnTypeChanged:    SeverityBreaksRecoverable,
		schemadiff.ColumnNotNullSet:     SeverityBreaksRecoverable,
		schemadiff.ColumnNotNullDropped: SeverityBreaksRecoverable,

		schemadiff.TableAdded:                    SeverityPotentialImpact,
		schemadiff.ColumnDropped:                 SeverityPotentialImpact,
		schemadiff.ColumnDefaultChanged:          SeverityPotentialImpact,
		schemadiff.TableKindChanged:              SeverityPotentialImpact,
		schemadiff.TablePartitionAttached:        SeverityPotentialImpact,
		schemadiff.TablePartitionDetached:        SeverityPotentialImpact,
		schemadiff.TablePartitionChildrenChanged: SeverityPotentialImpact,

		schemadiff.TableInheritAdded:       SeverityAdvisory,
		schemadiff.TableInheritRemoved:     SeverityAdvisory,
		schemadiff.TableInheritedByAdded:   SeverityAdvisory,
		schemadiff.TableInheritedByRemoved: SeverityAdvisory,
	}
	require.ElementsMatch(t, schemadiff.AllDiffTypes(), lo.Keys(want))

	for diffType, wantSeverity := range want {
		t.Run(string(diffType), func(t *testing.T) {
			got, err := getDriftInfo(diffType, situation{Timing: TimingLiveAfterExportStart, FallBackPossible: true})
			require.NoError(t, err)
			assert.Equal(t, wantSeverity, got.Severity)
		})
	}
}

// A split type takes over its old combined type's text in both directions until
// per-direction text lands.
func TestGetDriftInfo_SplitTypesShareTheCombinedText(t *testing.T) {
	pairs := [][2]schemadiff.DiffType{
		{schemadiff.ColumnNotNullSet, schemadiff.ColumnNotNullDropped},
		{schemadiff.TablePartitionAttached, schemadiff.TablePartitionDetached},
		{schemadiff.TableInheritAdded, schemadiff.TableInheritRemoved},
		{schemadiff.TableInheritedByAdded, schemadiff.TableInheritedByRemoved},
	}
	for _, p := range pairs {
		t.Run(string(p[0]), func(t *testing.T) {
			assert.Equal(t, infoByDiffType[p[0]], infoByDiffType[p[1]])
		})
	}
}

func TestGetDriftInfo_ResolvesPerTiming(t *testing.T) {
	const testType schemadiff.DiffType = "TEST_ONLY_TYPE"
	beforeInfo := DriftInfo{Severity: SeverityAdvisory, Impact: "before impact.", Action: "before action."}
	offlineInfo := DriftInfo{Severity: SeverityPotentialImpact, Impact: "offline impact.", Action: "offline action."}
	liveInfo := DriftInfo{Severity: SeverityBreaksUnrecoverable, Impact: "live impact.", Action: "live action."}
	infoByDiffType[testType] = map[Timing]DriftInfo{
		TimingBeforeExportData:        beforeInfo,
		TimingOfflineAfterExportStart: offlineInfo,
		TimingLiveAfterExportStart:    liveInfo,
	}
	fallBackRiskByDiffType[testType] = "fall-back risk."
	t.Cleanup(func() {
		delete(infoByDiffType, testType)
		delete(fallBackRiskByDiffType, testType)
	})

	withRisk := func(info DriftInfo) DriftInfo {
		info.Impact += " fall-back risk."
		return info
	}
	wantAfterCutover := DriftInfo{
		Severity: SeverityAdvisory,
		Impact:   "This change was made after cutover to target. Data already migrated to the target is not affected.",
	}
	tests := []struct {
		name string
		s    situation
		want DriftInfo
	}{
		{"before export data", situation{Timing: TimingBeforeExportData}, beforeInfo},
		{"offline", situation{Timing: TimingOfflineAfterExportStart}, offlineInfo},
		{"live", situation{Timing: TimingLiveAfterExportStart}, liveInfo},
		{"changes-only without its own entry falls back to live", situation{Timing: TimingChangesOnlyAfterExportStart}, liveInfo},
		{"the fall-back sentence is appended while fall-back is possible", situation{Timing: TimingLiveAfterExportStart, FallBackPossible: true}, withRisk(liveInfo)},
		{"the fall-back sentence is appended before export data too", situation{Timing: TimingBeforeExportData, FallBackPossible: true}, withRisk(beforeInfo)},
		{"after cutover is the same advisory for every type, with no action", situation{Timing: TimingAfterCutover}, wantAfterCutover},
		{"after cutover never carries the fall-back sentence", situation{Timing: TimingAfterCutover, FallBackPossible: true}, wantAfterCutover},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDriftInfo(testType, tt.s)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("changes-only with its own entry uses it", func(t *testing.T) {
		changesOnlyInfo := DriftInfo{Severity: SeverityBreaksUnrecoverable, Impact: "changes-only impact.", Action: "changes-only action."}
		infoByDiffType[testType][TimingChangesOnlyAfterExportStart] = changesOnlyInfo

		got, err := getDriftInfo(testType, situation{Timing: TimingChangesOnlyAfterExportStart})
		require.NoError(t, err)
		assert.Equal(t, changesOnlyInfo, got)
	})
}

func TestGetDriftInfo_MissingEntryIsAnError(t *testing.T) {
	const partialType schemadiff.DiffType = "TEST_ONLY_PARTIAL_TYPE"
	infoByDiffType[partialType] = map[Timing]DriftInfo{
		TimingBeforeExportData: {Severity: SeverityAdvisory, Impact: "before impact.", Action: "before action."},
	}
	t.Cleanup(func() { delete(infoByDiffType, partialType) })

	tests := []struct {
		diffType schemadiff.DiffType
		timing   Timing
	}{
		{"", TimingLiveAfterExportStart},
		{"SOME_FUTURE_DIFF_TYPE", TimingLiveAfterExportStart},
		{"SOME_FUTURE_DIFF_TYPE", TimingChangesOnlyAfterExportStart},
		{partialType, TimingOfflineAfterExportStart},
		{partialType, TimingChangesOnlyAfterExportStart},
	}
	for _, tt := range tests {
		_, err := getDriftInfo(tt.diffType, situation{Timing: tt.timing})
		require.Error(t, err)
		assert.Equal(t, fmt.Sprintf("no Impact and Action for change type %q at timing %q", tt.diffType, tt.timing), err.Error())
	}
}
