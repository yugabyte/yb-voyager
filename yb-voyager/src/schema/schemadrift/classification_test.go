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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
)

var allTimings = []Timing{
	TimingBeforeExportData,
	TimingOfflineAfterExportStart,
	TimingLiveAfterExportStart,
	TimingChangesOnlyAfterExportStart,
	TimingAfterCutover,
}

func TestGuidanceCoversEveryDiffType(t *testing.T) {
	assert.ElementsMatch(t, schemadiff.AllDiffTypes(), lo.Keys(guidanceByDiffType),
		"guidanceByDiffType must have exactly one entry per DiffType the engine emits")

	for diffType, g := range guidanceByDiffType {
		t.Run(string(diffType), func(t *testing.T) {
			assertCompleteInfo(t, g.Default)
			for timing, info := range g.ByTiming {
				assert.Contains(t, allTimings, timing)
				assert.NotEqual(t, TimingAfterCutover, timing, "after cutover is the same for every type")
				assertCompleteInfo(t, info)
			}
		})
	}
	assertCompleteInfo(t, afterCutoverInfo)
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
func TestGetDriftInfo_SeverityByTiming(t *testing.T) {
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
		schemadiff.TableAdded:                    {rec, unr, unr, unr},
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
			assert.NotEqual(t, guidanceByDiffType[p[0]].Default, guidanceByDiffType[p[1]].Default)
		})
	}
}

func TestGetDriftInfo_FallBackSentenceOnlyOnColumnDropped(t *testing.T) {
	const sentence = "If you cut over with fall-back, `import data to source` stops at the first insert on this table"
	for _, diffType := range schemadiff.AllDiffTypes() {
		info, err := getDriftInfo(diffType, situation{Timing: TimingLiveAfterExportStart, FallBackPossible: true})
		require.NoError(t, err)
		assert.Equal(t, diffType == schemadiff.ColumnDropped, strings.Contains(info.Impact, sentence), string(diffType))
	}
	info, err := getDriftInfo(schemadiff.ColumnDropped, situation{Timing: TimingLiveAfterExportStart})
	require.NoError(t, err)
	assert.NotContains(t, info.Impact, "fall-back")
}

func TestGetDriftInfo_ChangesOnlyStartsANewMigration(t *testing.T) {
	for diffType, g := range guidanceByDiffType {
		live := g.ByTiming[TimingLiveAfterExportStart]
		changesOnly, ok := g.ByTiming[TimingChangesOnlyAfterExportStart]
		assert.Equal(t, strings.Contains(live.Action, "restart the data migration:"), ok, string(diffType))
		if ok {
			assert.Contains(t, changesOnly.Action, "start a new migration in a new export directory", string(diffType))
			assert.NotContains(t, changesOnly.Action, "--start-clean", string(diffType))
			assert.Equal(t, live.Impact, changesOnly.Impact, string(diffType))
		}
	}
}

func TestGetDriftInfo_ResolvesByTiming(t *testing.T) {
	const testType schemadiff.DiffType = "TEST_ONLY_TYPE"
	defaultInfo := DriftInfo{Severity: SeverityAdvisory, Impact: "default impact.", Action: "default action."}
	offlineInfo := DriftInfo{Severity: SeverityPotentialImpact, Impact: "offline impact.", Action: "offline action."}
	liveInfo := DriftInfo{Severity: SeverityBreaksUnrecoverable, Impact: "live impact.", Action: "live action."}
	guidanceByDiffType[testType] = driftGuidance{
		Default: defaultInfo,
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: offlineInfo,
			TimingLiveAfterExportStart:    liveInfo,
		},
		FallBackRisk: "fall-back risk.",
	}
	t.Cleanup(func() { delete(guidanceByDiffType, testType) })

	withRisk := func(info DriftInfo) DriftInfo {
		info.Impact += " fall-back risk."
		return info
	}
	tests := []struct {
		name string
		s    situation
		want DriftInfo
	}{
		{"a timing without its own entry gets the default", situation{Timing: TimingBeforeExportData}, defaultInfo},
		{"a timing with its own entry gets it", situation{Timing: TimingOfflineAfterExportStart}, offlineInfo},
		{"live", situation{Timing: TimingLiveAfterExportStart}, liveInfo},
		{"changes-only without its own entry falls back to live", situation{Timing: TimingChangesOnlyAfterExportStart}, liveInfo},
		{"the fall-back sentence is appended while fall-back is possible", situation{Timing: TimingLiveAfterExportStart, FallBackPossible: true}, withRisk(liveInfo)},
		{"the fall-back sentence is appended to the default too", situation{Timing: TimingBeforeExportData, FallBackPossible: true}, withRisk(defaultInfo)},
		{"after cutover is the same advisory for every type", situation{Timing: TimingAfterCutover}, afterCutoverInfo},
		{"after cutover never carries the fall-back sentence", situation{Timing: TimingAfterCutover, FallBackPossible: true}, afterCutoverInfo},
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
		guidanceByDiffType[testType].ByTiming[TimingChangesOnlyAfterExportStart] = changesOnlyInfo

		got, err := getDriftInfo(testType, situation{Timing: TimingChangesOnlyAfterExportStart})
		require.NoError(t, err)
		assert.Equal(t, changesOnlyInfo, got)
	})
}

func TestGetDriftInfo_UnknownTypeIsAnError(t *testing.T) {
	for _, unknown := range []schemadiff.DiffType{"", "SOME_FUTURE_DIFF_TYPE"} {
		_, err := getDriftInfo(unknown, situation{Timing: TimingLiveAfterExportStart})
		require.Error(t, err)
		assert.Equal(t, fmt.Sprintf("no Impact and Action for change type %q", unknown), err.Error())
	}
}

// The catalogue lists every message the report can print, so a text change is
// reviewed as a diff of the file. Regenerate it with:
//
//	SCHEMADRIFT_REGEN=1 go test -tags unit -run TestGuidanceCatalogue ./src/schema/schemadrift/
func TestGuidanceCatalogue(t *testing.T) {
	path := filepath.Join("testdata", "guidance_catalogue.txt")
	got := guidanceCatalogue(t)
	if os.Getenv("SCHEMADRIFT_REGEN") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "regenerate with SCHEMADRIFT_REGEN=1")
	assert.Equal(t, string(want), got, "guidance changed; regenerate with SCHEMADRIFT_REGEN=1 and review the diff")
}

// The situations situationFor can produce: before export data, the fall-back
// sentence depends on whether the flow is offline or not yet known.
var catalogueSituations = []situation{
	{Timing: TimingBeforeExportData, FallBackPossible: false},
	{Timing: TimingBeforeExportData, FallBackPossible: true},
	{Timing: TimingOfflineAfterExportStart, FallBackPossible: false},
	{Timing: TimingLiveAfterExportStart, FallBackPossible: true},
	{Timing: TimingChangesOnlyAfterExportStart, FallBackPossible: true},
	{Timing: TimingAfterCutover, FallBackPossible: false},
}

func guidanceCatalogue(t *testing.T) string {
	var b strings.Builder
	for i, diffType := range schemadiff.AllDiffTypes() {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "== %s\n", diffType)
		for _, s := range catalogueSituations {
			info, err := getDriftInfo(diffType, s)
			require.NoError(t, err)
			fmt.Fprintf(&b, "\n-- %s, fall-back possible: %t\n", s.Timing, s.FallBackPossible)
			fmt.Fprintf(&b, "Severity: %s\nImpact: %s\nAction: %s\n", info.Severity, info.Impact, info.Action)
		}
	}
	return b.String()
}
