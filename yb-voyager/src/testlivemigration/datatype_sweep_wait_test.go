//go:build integration_live_migration

/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package testlivemigration

/*
Container-free tests for the sweep's bounded wait.

The wait is the suite's dominant cost: before crash-loop detection, a FAILING probe paid
the entire budget (240 s solo, 900 s batched) and a batch poisoned by one wedged value paid
~48 minutes and then produced nothing. The point of these tests is that the saving is real
and that it was not bought by weakening the evidence rules - so each one drives
waitForSignalOrCrashLoop end to end with an injected clock and a synthetic import log, and
asserts the POLL COUNT at which it concluded rather than just the outcome.

None of these need Docker, a database or real time.
*/

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
)

// A genuine wedged-importer line, verbatim from a fall-back run: the importer retries the
// same batch and logs this every few seconds, forever.
const crashLoopLine = `[import data] error executing batch on channel 3: error preparing statements for ` +
	`events in batch (12:24) or when executing event with vsn(12): ERROR: invalid input syntax for ` +
	`type tid: "\x2831372c3529" (SQLSTATE 22P02) table=sweep_schema.p_tid_001`

// The line that produced two waves of bogus STUCK verdicts: a spew dump of a CONFIG FIELD
// whose VALUE merely contains the word ERROR. It must never terminate a wait.
const spewConfigLine = `	PKConflictAction: (string) (len=12) "ERROR-POLICY",`

// fakeClock replaces time.Now/time.Sleep so a 900 s budget is exercised in microseconds.
// Sleeping advances the clock, which is exactly the relationship the loop assumes.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time        { return c.t }
func (c *fakeClock) sleep(d time.Duration) { c.t = c.t.Add(d) }

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
}

// runWaitWithLog drives the loop against a fixed log and a fixed (frozen) count
// fingerprint - the shape of a wedged pipeline.
func runWaitWithLog(budget time.Duration, logText string) waitResult {
	clock := newFakeClock()
	return waitForSignalOrCrashLoop(budget, sweepWaitPoll, func() waitSample {
		return waitSample{satisfied: false, progress: "frozen", logText: logText}
	}, clock.now, clock.sleep)
}

// TestSweepWaitTerminatesOnRepeatingImporterError is the headline claim: a crash-loop ends
// the wait in a couple of polls instead of burning the full 900 s budget.
func TestSweepWaitTerminatesOnRepeatingImporterError(t *testing.T) {
	budget := seconds(sweepStreamingTimeout) // the batched budget: 900 s
	res := runWaitWithLog(budget, strings.Repeat(crashLoopLine+"\n", 5))

	if res.outcome != waitRepeatingError {
		t.Fatalf("outcome = %s, want %s (summary: %s)", res.outcome, waitRepeatingError, res.summary())
	}
	// sweepCrashLoopPolls = 2 consecutive polls with the same signature and frozen counts,
	// and only one sleep happens between them.
	if res.polls != sweepCrashLoopPolls {
		t.Errorf("concluded after %d polls, want %d", res.polls, sweepCrashLoopPolls)
	}
	if want := time.Duration(sweepCrashLoopPolls-1) * sweepWaitPoll; res.elapsed != want {
		t.Errorf("elapsed = %s, want %s", res.elapsed, want)
	}
	if res.saved() != budget-res.elapsed {
		t.Errorf("saved = %s, want %s", res.saved(), budget-res.elapsed)
	}
	// The evidence must be quotable, with the real SQLSTATE hoisted out of it.
	if !strings.Contains(res.quotedError, "22P02") {
		t.Errorf("quoted error lost its SQLSTATE: %q", res.quotedError)
	}
	if !strings.Contains(res.quotedError, "invalid input syntax for type tid") {
		t.Errorf("quoted error does not carry the message: %q", res.quotedError)
	}
	if res.repeats < sweepCrashLoopRepeats {
		t.Errorf("repeats = %d, want >= %d", res.repeats, sweepCrashLoopRepeats)
	}
	// The saving has to be legible in the run output, not just true.
	if s := res.summary(); !strings.Contains(s, "repeated x5") || !strings.Contains(s, "898s") {
		t.Errorf("summary does not state the repeat count and the saving: %q", s)
	}
}

// TestSweepWaitBelowRepeatThresholdDoesNotTerminate: one or two occurrences is a transient
// error the importer may well get past. Only a REPEATING one is a crash-loop.
func TestSweepWaitBelowRepeatThresholdDoesNotTerminate(t *testing.T) {
	budget := 20 * time.Second
	res := runWaitWithLog(budget, strings.Repeat(crashLoopLine+"\n", sweepCrashLoopRepeats-1))
	if res.outcome != waitTimeout {
		t.Fatalf("outcome = %s, want %s: %d occurrences must not end the wait",
			res.outcome, waitTimeout, sweepCrashLoopRepeats-1)
	}
}

// TestSweepWaitSilentStallRunsToTimeout pins the case the long timeout is KEPT for: a
// stall that logs nothing at all. It must still cost the whole budget - and must classify
// INCONCLUSIVE, never STUCK, because there is nothing to quote.
func TestSweepWaitSilentStallRunsToTimeout(t *testing.T) {
	budget := 20 * time.Second
	quiet := strings.Repeat("Waiting for streaming mode\nexport data: 0 events\n", 40)

	res := runWaitWithLog(budget, quiet)
	if res.outcome != waitTimeout {
		t.Fatalf("outcome = %s, want %s (summary: %s)", res.outcome, waitTimeout, res.summary())
	}
	if wantPolls := int(budget/sweepWaitPoll) + 1; res.polls != wantPolls {
		t.Errorf("polls = %d, want %d (the whole budget)", res.polls, wantPolls)
	}
	if res.elapsed != budget {
		t.Errorf("elapsed = %s, want the full budget %s", res.elapsed, budget)
	}
	if res.saved() != 0 {
		t.Errorf("a timeout saved %s, want 0", res.saved())
	}

	// And the verdict that follows from it. This is the observation applyWaitResult builds
	// for a timeout with nothing quotable: no stuckDetail, no events.
	verdict, detail := decideVerdict(modeLive, probeObservation{
		snapshotCompared: true, streamCompared: true,
		waitTimedOut: true, eventsForTable: 0,
		waitNote: "forward streaming wait expired",
	})
	if verdict != verdictInconclusive {
		t.Fatalf("silent stall classified %s, want %s (%s)", verdict, verdictInconclusive, detail)
	}
}

// TestSweepWaitIgnoresSpewAndConfigLines is rule (2): early termination reuses
// isImportFailureSignature, so no amount of repetition of a struct dump can end a wait.
// Both lines here are real offenders that previously produced whole batches of bogus
// STUCK verdicts.
func TestSweepWaitIgnoresSpewAndConfigLines(t *testing.T) {
	noise := []string{
		spewConfigLine,
		`Error: (string) ""`,
		`config: mode set to "error-policy"`,
	}
	for _, line := range noise {
		t.Run(strings.Fields(line)[0], func(t *testing.T) {
			res := runWaitWithLog(20*time.Second, strings.Repeat(line+"\n", 200))
			if res.outcome != waitTimeout {
				t.Fatalf("%q ended the wait as %s (quoted %q); a spew/config line must never "+
					"terminate a wait", line, res.outcome, res.quotedError)
			}
		})
	}
}

// TestSweepWaitAdvancingCountsPreventEarlyTermination: a moving pipeline is not a wedged
// one. Even with a real, repeating error in the log, advancing counts must keep the wait
// alive - the importer is plainly getting past whatever it complained about.
func TestSweepWaitAdvancingCountsPreventEarlyTermination(t *testing.T) {
	clock := newFakeClock()
	imported := 0
	res := waitForSignalOrCrashLoop(20*time.Second, sweepWaitPoll, func() waitSample {
		imported += 7
		return waitSample{
			progress: fmt.Sprintf("imported=%d", imported),
			logText:  strings.Repeat(crashLoopLine+"\n", 9),
		}
	}, clock.now, clock.sleep)

	if res.outcome != waitTimeout {
		t.Fatalf("outcome = %s, want %s: advancing counts must veto early termination "+
			"(quoted %q)", res.outcome, waitTimeout, res.quotedError)
	}
}

// TestSweepWaitResumesDetectionAfterProgressStops is the other half of the same rule: once
// the counts DO freeze, the same repeating error is again a crash-loop.
func TestSweepWaitResumesDetectionAfterProgressStops(t *testing.T) {
	clock := newFakeClock()
	poll := 0
	res := waitForSignalOrCrashLoop(900*time.Second, sweepWaitPoll, func() waitSample {
		poll++
		progress := "wedged"
		if poll <= 3 {
			progress = fmt.Sprintf("imported=%d", poll)
		}
		return waitSample{progress: progress, logText: strings.Repeat(crashLoopLine+"\n", 4)}
	}, clock.now, clock.sleep)

	if res.outcome != waitRepeatingError {
		t.Fatalf("outcome = %s, want %s", res.outcome, waitRepeatingError)
	}
	// Polls 2-4 see the counts move (poll 4 is where "imported=3" becomes "wedged", which
	// is itself a change), poll 5 is the first genuinely frozen one and starts the
	// signature run, poll 6 confirms it.
	if res.polls != 6 {
		t.Errorf("concluded after %d polls, want 6", res.polls)
	}
}

// TestSweepWaitStopsWhenCountsSatisfied: the positive signal still wins. A pipeline that
// is moving and then completes must report counts-satisfied even though its log holds a
// repeating error - the importer plainly got past whatever it complained about.
func TestSweepWaitStopsWhenCountsSatisfied(t *testing.T) {
	clock := newFakeClock()
	poll := 0
	res := waitForSignalOrCrashLoop(900*time.Second, sweepWaitPoll, func() waitSample {
		poll++
		return waitSample{
			satisfied: poll >= 3,
			progress:  fmt.Sprintf("imported=%d", poll),
			logText:   strings.Repeat(crashLoopLine+"\n", 9),
		}
	}, clock.now, clock.sleep)

	if res.outcome != waitSatisfied {
		t.Fatalf("outcome = %s, want %s", res.outcome, waitSatisfied)
	}
	if res.polls != 3 {
		t.Errorf("polls = %d, want 3", res.polls)
	}
}

// ============================================================
// LIVENESS: the wait must not outlive the process it is waiting on
// ============================================================

// The one-shot death this whole path exists for: PostgreSQL refuses to accept a value of
// the type at all, `import data` says so ONCE, and the process exits. There is no
// crash-loop to count, so the repeat threshold never trips.
const oneShotDeathLine = `[import data] error executing batch on channel 1: error preparing statements ` +
	`for events in batch (1:12): ERROR: cannot accept a value of type pg_node_tree ` +
	`(SQLSTATE 0A000) table=sweep_schema.p_catalog_004`

// runWaitWithDeadCommand drives the loop against a pipeline whose command has exited at a
// chosen poll. exitErr nil means it exited cleanly.
func runWaitWithDeadCommand(budget time.Duration, logText, command string, exitErr error, deadFrom int) waitResult {
	clock := newFakeClock()
	poll := 0
	return waitForSignalOrCrashLoop(budget, sweepWaitPoll, func() waitSample {
		poll++
		s := waitSample{progress: "frozen", logText: logText}
		if poll >= deadFrom {
			s.goneCommand, s.goneErr = command, exitErr
		}
		return s
	}, clock.now, clock.sleep)
}

// TestSweepWaitTerminatesOnAnExitedImporter: the process is gone, so the counts are never
// coming. No repeat count, no signature, no threshold - just liveness.
func TestSweepWaitTerminatesOnAnExitedImporter(t *testing.T) {
	budget := seconds(sweepStreamingTimeout) // 900 s
	res := runWaitWithDeadCommand(budget, oneShotDeathLine+"\n", "import data",
		fmt.Errorf("command failed: exit status 1"), 1)

	if res.outcome != waitProcessGone {
		t.Fatalf("outcome = %s, want %s (summary: %s)", res.outcome, waitProcessGone, res.summary())
	}
	// One detecting poll plus one confirming poll, and a single sleep between them.
	if res.polls != 2 {
		t.Errorf("concluded after %d polls, want 2", res.polls)
	}
	if res.elapsed != sweepWaitPoll {
		t.Errorf("elapsed = %s, want %s", res.elapsed, sweepWaitPoll)
	}
	if res.saved() != budget-res.elapsed {
		t.Errorf("saved = %s, want %s", res.saved(), budget-res.elapsed)
	}
	if s := res.summary(); !strings.Contains(s, "import data") ||
		!strings.Contains(s, "exit status 1") || !strings.Contains(s, "898s") {
		t.Errorf("summary does not name the command, its exit and the saving: %q", s)
	}
	// The single occurrence is what makes this case different from a crash-loop: the
	// repeat-count path could not have concluded here.
	if crash := runWaitWithLog(20*time.Second, oneShotDeathLine+"\n"); crash.outcome != waitTimeout {
		t.Errorf("a single occurrence with a LIVE importer ended the wait as %s; only "+
			"liveness may conclude on one occurrence", crash.outcome)
	}
}

// TestSweepWaitExitedImporterQuotesTheError: the verdict a one-shot death produces. The
// error is quotable, so it is STUCK with the SQLSTATE - the same evidence bar as the
// crash-loop path, met by a single occurrence because the process only got to say it once.
func TestSweepWaitExitedImporterQuotesTheError(t *testing.T) {
	quoted, _, n := mostRepeatedErrorDetail(oneShotDeathLine, "")
	if quoted == "" || n != 1 {
		t.Fatalf("the one-shot death line is not quotable: %q x%d", quoted, n)
	}
	if !strings.Contains(quoted, "0A000") {
		t.Errorf("quoted error lost its SQLSTATE: %q", quoted)
	}

	verdict, detail := decideVerdict(modeLive, probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 2, columnSeenInEvents: true,
		waitTimedOut: true, commandExited: true,
		commandExitDetail: "import data exited during the forward streaming wait after 2s",
		stuckDetail:       quoted + " (x1) - import data exited during the forward streaming wait after 2s",
	})
	if verdict != verdictStuck {
		t.Fatalf("an importer that exited on a quotable error classified %s, want %s (%s)",
			verdict, verdictStuck, detail)
	}
	if !strings.Contains(detail, "0A000") {
		t.Errorf("the STUCK detail does not quote the SQLSTATE: %s", detail)
	}
}

// TestSweepWaitCleanExitIsInconclusive: a process that finished and left is a reason to
// stop waiting, never evidence against a datatype.
func TestSweepWaitCleanExitIsInconclusive(t *testing.T) {
	res := runWaitWithDeadCommand(20*time.Second, "nothing interesting\n", "import data", nil, 1)
	if res.outcome != waitProcessGone {
		t.Fatalf("outcome = %s, want %s", res.outcome, waitProcessGone)
	}
	if !strings.Contains(res.summary(), "exited cleanly") {
		t.Errorf("a clean exit is not described as one: %q", res.summary())
	}

	// What recordCommandExit records for it: commandExited, and nothing quotable.
	verdict, detail := decideVerdict(modeLive, probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 2, columnSeenInEvents: true,
		waitTimedOut: true, commandExited: true,
		commandExitDetail: "import data exited during the forward streaming wait after 2s, " +
			"before the expected counts arrived; a clean exit is not a failure, so nothing " +
			"is claimed about this type",
	})
	if verdict != verdictInconclusive {
		t.Fatalf("a clean exit classified %s, want %s (%s)", verdict, verdictInconclusive, detail)
	}
	if verdict == verdictStuck || verdict == verdictSilentLoss {
		t.Fatal("a clean exit must never produce a failure verdict")
	}
}

// TestSweepWaitUnquotableExitIsInconclusive: the process died for a reason nothing in the
// log can be quoted for. Real, but not attributable to a datatype - so it is INCONCLUSIVE,
// exactly as the "wait expired with nothing quotable" case is.
func TestSweepWaitUnquotableExitIsInconclusive(t *testing.T) {
	verdict, detail := decideVerdict(modeLive, probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 2, columnSeenInEvents: true,
		waitTimedOut: true, commandExited: true,
		commandExitDetail: "import data exited during the snapshot wait after 4s: exit status 1; " +
			"NO error line matching an import-failure signature was found",
	})
	if verdict != verdictInconclusive {
		t.Fatalf("an unexplained exit classified %s, want %s (%s)", verdict, verdictInconclusive, detail)
	}
}

// TestSweepWaitLiveCommandsKeepTheirBudget: the two cases that must NOT be disturbed by
// liveness - a live but wedged importer still takes the crash-loop path, and a live, quiet
// pipeline still spends its budget.
func TestSweepWaitLiveCommandsKeepTheirBudget(t *testing.T) {
	wedged := runWaitWithLog(900*time.Second, strings.Repeat(crashLoopLine+"\n", 6))
	if wedged.outcome != waitRepeatingError {
		t.Errorf("a live, wedged importer ended as %s, want %s", wedged.outcome, waitRepeatingError)
	}
	quiet := runWaitWithLog(20*time.Second, "Waiting for streaming mode\n")
	if quiet.outcome != waitTimeout {
		t.Errorf("a live, quiet pipeline ended as %s, want %s", quiet.outcome, waitTimeout)
	}
}

// TestSweepWaitConfirmsBeforeCallingACommandDead guards the race the confirming poll
// exists for: a command that wrote its last events and then exited has SUCCEEDED, and must
// not be reported as having died without a result.
func TestSweepWaitConfirmsBeforeCallingACommandDead(t *testing.T) {
	clock := newFakeClock()
	poll := 0
	res := waitForSignalOrCrashLoop(900*time.Second, sweepWaitPoll, func() waitSample {
		poll++
		return waitSample{
			// The counts land on the confirming poll, one after the exit is noticed.
			satisfied:   poll >= 2,
			progress:    "frozen",
			goneCommand: "import data",
			goneErr:     fmt.Errorf("command failed: exit status 1"),
		}
	}, clock.now, clock.sleep)

	if res.outcome != waitSatisfied {
		t.Fatalf("outcome = %s, want %s: a command that finished its work and then exited "+
			"must not be reported as dead-without-result", res.outcome, waitSatisfied)
	}
}

// TestSweepWaitExporterLogBeatsBareLiveness: when the export side is dead AND its log says
// why, the log wins - "the connector threw a NullPointerException" is a finding, "the
// process is gone" is only a fact.
func TestSweepWaitExporterLogBeatsBareLiveness(t *testing.T) {
	clock := newFakeClock()
	res := waitForSignalOrCrashLoop(900*time.Second, sweepWaitPoll, func() waitSample {
		return waitSample{
			progress:    "frozen",
			exportText:  exporterDeathLog,
			goneCommand: "export data",
			goneErr:     fmt.Errorf("command failed: exit status 1"),
		}
	}, clock.now, clock.sleep)

	if res.outcome != waitExportDied {
		t.Fatalf("outcome = %s, want %s: an export death with a quotable cause must not be "+
			"downgraded to bare liveness", res.outcome, waitExportDied)
	}
}

// TestReportFingerprintTracksProgress pins what "the counts advanced" means: any exported
// or imported number moving, on any table, in either direction.
func TestReportFingerprintTracksProgress(t *testing.T) {
	report := func(importedInserts int64) *DataMigrationReport {
		return &DataMigrationReport{RowData: []*cmd.RowData{
			{TableName: `"sweep_schema"."p_ctrl_001"`, DBType: "source", ExportedInserts: 3},
			{TableName: `"sweep_schema"."p_ctrl_001"`, DBType: "target", ImportedInserts: importedInserts},
		}}
	}
	if a, b := reportFingerprint(report(1)), reportFingerprint(report(1)); a != b {
		t.Errorf("identical reports fingerprinted differently:\n%s\n%s", a, b)
	}
	if a, b := reportFingerprint(report(1)), reportFingerprint(report(2)); a == b {
		t.Errorf("an advancing import count did not change the fingerprint: %s", a)
	}
	if got := reportFingerprint(nil); got != "<report-unavailable>" {
		t.Errorf("nil report fingerprint = %q", got)
	}
}

// TestCrashLoopAttribution pins the quarantine half: the offending probe is named only
// when the repeated error identifies EXACTLY ONE of the batch's probes. A guess would
// quarantine an innocent type and record that guess as a finding.
func TestCrashLoopAttribution(t *testing.T) {
	probes := []datatypeProbe{
		{ID: "CTRL-001", TypeName: "int", ExpectVerdict: verdictWorks},
		{ID: "TID-001", TypeName: "tid"},
		{ID: "HSTORE-001", TypeName: "hstore"},
	}
	cases := []struct {
		name string
		text string
		want string // "" means no attribution
	}{
		{"names one probe table", crashLoopLine, "TID-001"},
		{
			name: "names another probe table",
			text: `ERROR: type "sweep_schema.p_hstore_001_h" does not exist (SQLSTATE 42704)`,
			want: "HSTORE-001",
		},
		{
			name: "names two probe tables - ambiguous, no culprit",
			text: `ERROR: batch p_tid_001 / p_hstore_001 failed (SQLSTATE 42704)`,
		},
		{
			name: "names no probe table",
			text: `ERROR: connection reset by peer (SQLSTATE 08006)`,
		},
		{
			// A control is never the culprit: if the control's table is the only one
			// named, the harness is suspect, not a datatype.
			name: "names only a control table",
			text: `ERROR: p_ctrl_001 batch failed (SQLSTATE 42601)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := attributeCrashLoop(probes, tc.text)
			if tc.want == "" {
				if ok {
					t.Fatalf("attributed %q, want no attribution", got)
				}
				return
			}
			if !ok || got != tc.want {
				t.Fatalf("attributed %q (ok=%v), want %q", got, ok, tc.want)
			}
		})
	}
}

// TestSweepClassifierCollateralIsInconclusive: a probe stuck behind SOMEONE ELSE's poison
// was not measured. Reporting it STUCK blames it for another type's failure; reporting the
// value diff it happens to show manufactures a SILENT_LOSS out of a truncated channel.
func TestSweepClassifierCollateralIsInconclusive(t *testing.T) {
	collateral := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 4, columnSeenInEvents: true,
		waitTimedOut: true, channelWedgedBy: "TID-001",
		// The truncated channel leaves a real-looking value difference behind.
		streamVerdict: verdictSilentLoss, streamDetail: "[insert] row id=6 missing on destination",
	}
	verdict, detail := decideVerdict(modeLive, collateral)
	if verdict != verdictInconclusive {
		t.Fatalf("collateral probe classified %s, want %s (%s)", verdict, verdictInconclusive, detail)
	}
	if !strings.Contains(detail, "TID-001") {
		t.Errorf("collateral detail does not name the culprit: %s", detail)
	}

	// The culprit itself is still STUCK, with the error quoted.
	culprit := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 4, columnSeenInEvents: true, waitTimedOut: true,
		stuckDetail: "SQLSTATE 22P02: " + crashLoopLine + " repeated x9",
	}
	if verdict, detail := decideVerdict(modeLive, culprit); verdict != verdictStuck {
		t.Fatalf("culprit classified %s, want %s (%s)", verdict, verdictStuck, detail)
	}

	// And a control that is collateral still fails the control gate, so the run is still
	// discarded: INCONCLUSIVE is not WORKS.
	if verdict, _ := decideVerdict(modeLive, collateral); verdict == verdictWorks {
		t.Fatal("collateral must never come out WORKS; the control gate depends on it")
	}
}

// TestWaitOutcomesStayDistinguishable guards the one distinction the whole design rests
// on: "wedged, here is the error" and "stalled, logged nothing" are different findings and
// must never collapse into one label.
func TestWaitOutcomesStayDistinguishable(t *testing.T) {
	crash := runWaitWithLog(20*time.Second, strings.Repeat(crashLoopLine+"\n", 6))
	quiet := runWaitWithLog(20*time.Second, "nothing to see here\n")

	if crash.outcome == quiet.outcome {
		t.Fatalf("a crash-loop and a silent stall both reported %s", crash.outcome)
	}
	if strings.Contains(quiet.summary(), "SQLSTATE") {
		t.Errorf("the silent-stall summary quotes an error it does not have: %s", quiet.summary())
	}
	if !strings.Contains(quiet.summary(), "logged nothing") {
		t.Errorf("the silent-stall summary does not say the stall logged nothing: %s", quiet.summary())
	}
}
