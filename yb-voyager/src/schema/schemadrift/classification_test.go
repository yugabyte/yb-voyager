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
	"strings"
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
// (it only leaves the target with an extra column), and a change after export
// data started that needs a restart is unrecoverable. Columns: before export
// data, offline, live, changes-only.
func TestGetDriftInfo_SeverityPerTiming(t *testing.T) {
	const (
		adv = SeverityAdvisory
		pot = SeverityPotentialImpact
		rec = SeverityBreaksRecoverable
		unr = SeverityBreaksUnrecoverable
	)
	want := map[schemadiff.DiffType][4]Severity{
		schemadiff.TableDropped:                  {adv, adv, unr, unr},
		schemadiff.TableNameChanged:              {rec, pot, unr, unr},
		schemadiff.TableSchemaChanged:            {rec, pot, unr, unr},
		schemadiff.TableAdded:                    {rec, adv, adv, adv},
		schemadiff.ColumnAdded:                   {rec, pot, rec, rec},
		schemadiff.ColumnDropped:                 {pot, pot, pot, pot},
		schemadiff.ColumnNameChanged:             {rec, pot, unr, unr},
		schemadiff.ColumnTypeChanged:             {pot, pot, pot, pot},
		schemadiff.ColumnNotNullDropped:          {rec, pot, rec, rec},
		schemadiff.ColumnNotNullSet:              {adv, adv, adv, adv},
		schemadiff.ColumnDefaultChanged:          {pot, pot, pot, pot},
		schemadiff.TableKindChanged:              {pot, pot, pot, pot},
		schemadiff.TablePartitionAttached:        {rec, pot, pot, pot},
		schemadiff.TablePartitionDetached:        {pot, pot, unr, unr},
		schemadiff.TablePartitionChildrenChanged: {adv, adv, adv, adv},
		schemadiff.TableInheritAdded:             {rec, pot, unr, unr},
		schemadiff.TableInheritRemoved:           {adv, adv, adv, adv},
		schemadiff.TableInheritedByAdded:         {rec, pot, unr, unr},
		schemadiff.TableInheritedByRemoved:       {adv, adv, adv, adv},
		schemadiff.PrimaryKeyAdded:               {rec, adv, adv, adv},
		schemadiff.PrimaryKeyDropped:             {rec, pot, unr, unr},
		schemadiff.PrimaryKeyNameChanged:         {adv, adv, adv, adv},
		schemadiff.UniqueAdded:                   {adv, adv, adv, adv},
		schemadiff.UniqueDropped:                 {rec, pot, rec, rec},
		schemadiff.UniqueNameChanged:             {adv, adv, adv, adv},
		schemadiff.CheckAdded:                    {adv, adv, adv, adv},
		schemadiff.CheckDropped:                  {rec, pot, rec, rec},
		schemadiff.CheckNameChanged:              {adv, adv, adv, adv},
		schemadiff.ForeignKeyAdded:               {adv, adv, adv, adv},
		schemadiff.ForeignKeyDropped:             {pot, pot, pot, pot},
		schemadiff.ForeignKeyNameChanged:         {adv, adv, adv, adv},
	}
	require.ElementsMatch(t, schemadiff.AllDiffTypes(), lo.Keys(want))

	timings := [4]Timing{TimingBeforeExportData, TimingOfflineAfterExportStart, TimingLiveAfterExportStart, TimingChangesOnlyAfterExportStart}
	for diffType, severities := range want {
		for i, timing := range timings {
			t.Run(fmt.Sprintf("%s/%s", diffType, timing), func(t *testing.T) {
				got, err := getDriftInfo(diffType, situation{Timing: timing})
				require.NoError(t, err)
				assert.Equal(t, severities[i], got.Severity)
			})
		}
	}
}

// Each direction of a split type has its own text: dropping NOT NULL breaks
// import data, setting it does not.
func TestGetDriftInfo_SplitTypesHaveTheirOwnText(t *testing.T) {
	pairs := [][2]schemadiff.DiffType{
		{schemadiff.ColumnNotNullSet, schemadiff.ColumnNotNullDropped},
		{schemadiff.TablePartitionAttached, schemadiff.TablePartitionDetached},
		{schemadiff.TableInheritAdded, schemadiff.TableInheritRemoved},
		{schemadiff.TableInheritedByAdded, schemadiff.TableInheritedByRemoved},
	}
	for _, p := range pairs {
		t.Run(string(p[0]), func(t *testing.T) {
			assert.NotEqual(t, infoByDiffType[p[0]][TimingBeforeExportData], infoByDiffType[p[1]][TimingBeforeExportData])
		})
	}
}

func TestGetDriftInfo_FallBackSentenceOnlyOnItsTypes(t *testing.T) {
	sentences := map[schemadiff.DiffType]string{
		schemadiff.ColumnDropped:   "If you cut over with fall-back, `import data to source` stops at the first insert on this table while the target still has this column, and later changes don't reach the source.",
		schemadiff.UniqueAdded:     uniqueAddedFallBackRisk,
		schemadiff.CheckAdded:      "If you cut over with fall-back while the target doesn't have this check, `import data to source` stops at the first row from the target that breaks it.",
		schemadiff.ForeignKeyAdded: "If you cut over with fall-back while the target doesn't have this key, rows from the target that break it still load into the source: `import data to source` runs with foreign-key checks off, or refuses to start until they are turned off. The source's key then no longer holds for those rows.",
	}
	// A type without a sentence must read the same with and without fall-back, so new
	// fall-back text on it fails here whatever its wording.
	for _, diffType := range schemadiff.AllDiffTypes() {
		withFallBack, err := getDriftInfo(diffType, situation{Timing: TimingLiveAfterExportStart, FallBackPossible: true})
		require.NoError(t, err)
		withoutFallBack, err := getDriftInfo(diffType, situation{Timing: TimingLiveAfterExportStart})
		require.NoError(t, err)
		want := withoutFallBack.Impact
		if sentence, ok := sentences[diffType]; ok {
			want += " " + sentence
		}
		assert.Equal(t, want, withFallBack.Impact, string(diffType))
	}
}

// Offline import data replays no deletes or updates, so the key's ON DELETE or ON UPDATE
// action is mentioned only for live, and before export data only scoped to live.
func TestGetDriftInfo_ForeignKeyDroppedActionOnlyWhenChangesReplay(t *testing.T) {
	type want struct{ impact, action string }
	liveOnly := want{
		impact: "In a live migration, a delete or update of a parent row",
		action: "In a live migration, restore from the source",
	}
	live := want{
		impact: "A delete or update of a parent row",
		action: "Restore from the source",
	}
	for timing, w := range map[Timing]want{
		TimingBeforeExportData:            liveOnly,
		TimingOfflineAfterExportStart:     {},
		TimingLiveAfterExportStart:        live,
		TimingChangesOnlyAfterExportStart: live,
	} {
		info, err := getDriftInfo(schemadiff.ForeignKeyDropped, situation{Timing: timing})
		require.NoError(t, err)
		if w == (want{}) {
			assert.NotContains(t, info.Impact, "`CASCADE`", "%s Impact", timing)
			assert.NotContains(t, strings.ToLower(info.Action), "restore from the source", "%s Action", timing)
			continue
		}
		assert.Contains(t, info.Impact, w.impact, "%s Impact", timing)
		assert.Contains(t, info.Action, w.action, "%s Action", timing)
	}
}

// Only a streaming import data reads the target's unique keys, so only the timings that
// can be live tell the user to restart it, and they carve out stored generated columns.
func TestGetDriftInfo_UniqueAddedRestartOnlyWhenLive(t *testing.T) {
	for timing, wantRestart := range map[Timing]bool{
		TimingBeforeExportData:            false,
		TimingOfflineAfterExportStart:     false,
		TimingLiveAfterExportStart:        true,
		TimingChangesOnlyAfterExportStart: true,
	} {
		info, err := getDriftInfo(schemadiff.UniqueAdded, situation{Timing: timing})
		require.NoError(t, err)
		assert.Equal(t, SeverityAdvisory, info.Severity, string(timing))
		assert.Equal(t, wantRestart, strings.Contains(info.Action, "restart it afterwards"), string(timing))
		assert.Equal(t, wantRestart, strings.Contains(info.Action, "If it covers a stored generated column, add it on the target only after cutover"), string(timing))
	}
}

func TestGetDriftInfo_ChangesOnlyStartsANewMigration(t *testing.T) {
	for diffType, byTiming := range infoByDiffType {
		live := byTiming[TimingLiveAfterExportStart]
		changesOnly, ok := byTiming[TimingChangesOnlyAfterExportStart]
		assert.Equal(t, strings.Contains(live.Action, "restart the data migration:"), ok, string(diffType))
		if ok {
			assert.Contains(t, changesOnly.Action, "start a new migration in a new export directory", string(diffType))
			assert.NotContains(t, changesOnly.Action, "--start-clean", string(diffType))
			assert.Equal(t, live.Impact, changesOnly.Impact, string(diffType))
		}
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
