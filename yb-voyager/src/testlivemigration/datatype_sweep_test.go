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
Entry points for the datatype sweep - one test per migration mode, plus a single-probe
runner for poison isolation.

Each mode test iterates the batch table and runs each batch as a subtest, so a batch can
be selected on its own:

	go test -tags integration_live_migration ./src/testlivemigration/ \
	    -run 'TestDatatypeSweepLive/ranges' -timeout 3h

One probe on its own (PROBE_SPEC.md's poison-isolation rule):

	PROBE_ID=HSTORE-001 PROBE_MODE=LIVE \
	    go test -tags integration_live_migration ./src/testlivemigration/ \
	    -run 'TestDatatypeSweepSuspect' -timeout 1h

Machine-readable output. Every probe prints exactly one line to stdout:

	PROBE-RESULT: <id> | <type> | <mode> | <verdict> | <detail>

so a whole sweep collapses to `go test ... | grep '^PROBE-RESULT:'`. Because that line
goes to stdout rather than through t.Log, add -v only if you also want the phase logs.

None of these tests call t.Parallel(). The test containers are shared singletons whose
config (including the database name) is rewritten by each NewTestContainer call, so two
sweeps running concurrently would race over which database they are pointed at.
*/

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDatatypeSweepOffline runs export data (snapshot-only) -> import data.
// The unsupported-datatype filter does not run in this mode, so the question here is
// purely snapshot fidelity: does pg_dump/COPY round-trip the value.
func TestDatatypeSweepOffline(t *testing.T) {
	runSweepBatches(t, modeOffline)
}

// TestDatatypeSweepLive runs export data (snapshot-and-changes) -> import data, then the
// full delta op set, including the "update a different column" op that exposes columns
// missing from the event stream.
func TestDatatypeSweepLive(t *testing.T) {
	runSweepBatches(t, modeLive)
}

// TestDatatypeSweepFallback runs the live flow, then cutover to target with
// --prepare-for-fall-back, then the reverse direction (export from target -> import to
// source) driven by target-side deltas.
func TestDatatypeSweepFallback(t *testing.T) {
	runSweepBatches(t, modeFallback)
}

// TestDatatypeSweepFallForward runs the live flow, brings up import data to
// source-replica (which sets FallForwardEnabled), cuts over, then replicates target-side
// deltas onward to the source-replica. Needs the third (SourceReplicaDB) container.
func TestDatatypeSweepFallForward(t *testing.T) {
	runSweepBatches(t, modeFallForward)
}

// TestDatatypeSweepSuspect runs exactly ONE probe per invocation, selected by the
// PROBE_ID environment variable, with PROBE_MODE choosing the mode (default LIVE).
//
// This exists because of PROBE_SPEC.md's poison-isolation rule: one bad event
// crash-loops the whole channel and blocks every later event in that segment, so a type
// suspected of STUCK cannot share a run with anything else. runDatatypeSweep refuses to
// put a probe marked Poison into a batch; this is the way to run it.
//
//	PROBE_ID=HSTORE-001 PROBE_MODE=FALL-BACK go test -tags integration_live_migration \
//	    ./src/testlivemigration/ -run TestDatatypeSweepSuspect -timeout 1h
func TestDatatypeSweepSuspect(t *testing.T) {
	id := os.Getenv("PROBE_ID")
	if id == "" {
		t.Skip("set PROBE_ID=<probe id> to run a single probe in isolation " +
			"(optionally PROBE_MODE=OFFLINE|LIVE|FALL-BACK|FALL-FORWARD, default LIVE)")
	}
	probe, ok := findProbeByID(id)
	if !ok {
		t.Fatalf("unknown PROBE_ID %q; ids come from the case tables in datatype_sweep_cases.go", id)
	}

	mode := sweepMode(os.Getenv("PROBE_MODE"))
	switch mode {
	case "":
		mode = modeLive
	case modeOffline, modeLive, modeFallback, modeFallForward:
	default:
		t.Fatalf("unknown PROBE_MODE %q; expected one of %s, %s, %s, %s",
			mode, modeOffline, modeLive, modeFallback, modeFallForward)
	}

	// Batch name is per-probe so the derived database name cannot collide with a
	// concurrently-scheduled batch run.
	runDatatypeSweep(t, mode, sweepBatch{
		Name:   "solo_" + sanitizeIdent(id),
		Probes: []datatypeProbe{probe},
	})
}

// runSweepBatches turns the batch table into one subtest per batch.
func runSweepBatches(t *testing.T, mode sweepMode) {
	assertUniqueProbeIDs(t)
	for _, batch := range sweepBatches() {
		batch := batch
		t.Run(batch.Name, func(t *testing.T) {
			runDatatypeSweep(t, mode, batch)
		})
	}
}

// assertUniqueProbeIDs guards the audit matrix: a duplicated id would silently overwrite
// a row of the report.
func assertUniqueProbeIDs(t *testing.T) {
	t.Helper()
	seen := map[string]bool{}
	tables := map[string]string{}
	for _, p := range allSweepProbes() {
		if seen[p.ID] {
			t.Fatalf("duplicate probe id %q in the case tables", p.ID)
		}
		seen[p.ID] = true
		if other, clash := tables[p.tableName()]; clash {
			t.Fatalf("probes %q and %q derive the same table name %q", other, p.ID, p.tableName())
		}
		tables[p.tableName()] = p.ID
		if p.InitialValue == "" || p.AltValue == "" {
			t.Fatalf("probe %q must set both InitialValue and AltValue", p.ID)
		}
		// A NullOnly probe is exempt: its type has no storable literal at all, so both
		// values are NULL by necessity and the update op is knowingly a no-op. The
		// probe still proves the type survives DDL, snapshot and CDC.
		if p.InitialValue == p.AltValue && !p.NullOnly {
			t.Fatalf("probe %q has InitialValue == AltValue, so its update op proves nothing", p.ID)
		}
	}
}

// TestSweepClassifierRequiresEvidence pins the invariant that a probe is never reported
// WORKS without positive evidence that it was actually exercised. This is a regression
// test for a real false pass: a framework wait that calls t.Fatalf unwinds the test
// goroutine via runtime.Goexit, so compareInto / applyDelta / recordQueueColumnPresence
// never run, and the deferred emitAll then classified an all-zero observation as
// "snapshot + insert/update/delete all identical; column present in the event stream
// (0 events for this table)" - a pass claim about a run that measured nothing.
func TestSweepClassifierRequiresEvidence(t *testing.T) {
	// A fully-measured LIVE probe: compared in both phases, events actually seen.
	good := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 6, columnSeenInEvents: true,
		deltaOpsApplied: 6, deltaConfirmed: true,
	}

	cases := []struct {
		name string
		mode sweepMode
		obs  probeObservation
		want string
	}{
		{"live fully measured is WORKS", modeLive, good, verdictWorks},
		{
			// The exact shape of the false pass.
			name: "live nothing measured at all",
			mode: modeLive,
			obs:  probeObservation{},
			want: verdictInconclusive,
		},
		{
			name: "live snapshot compared but delta phase never ran",
			mode: modeLive,
			obs:  probeObservation{snapshotCompared: true},
			want: verdictInconclusive,
		},
		{
			name: "live zero events and delta not confirmed on source",
			mode: modeLive,
			obs: probeObservation{
				snapshotCompared: true, streamCompared: true,
				eventsForTable: 0, deltaOpsApplied: 6, deltaConfirmed: false,
			},
			want: verdictInconclusive,
		},
		{
			// Ops demonstrably happened on the source, yet nothing reached the queue.
			name: "live zero events but delta confirmed on source is a loss",
			mode: modeLive,
			obs: probeObservation{
				snapshotCompared: true, streamCompared: true,
				eventsForTable: 0, deltaOpsApplied: 6, deltaConfirmed: true,
			},
			want: verdictSilentLoss,
		},
		{
			name: "offline nothing measured at all",
			mode: modeOffline,
			obs:  probeObservation{},
			want: verdictInconclusive,
		},
		{
			name: "offline snapshot compared clean is WORKS",
			mode: modeOffline,
			obs:  probeObservation{snapshotCompared: true},
			want: verdictWorks,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, detail := decideVerdict(tc.mode, tc.obs)
			if got != tc.want {
				t.Fatalf("decideVerdict(%s) = %s, want %s (detail: %s)",
					tc.mode, got, tc.want, detail)
			}
		})
	}

	// Guard the specific wording that made the false pass so convincing: a zero-event
	// run must never claim the column was present in the event stream.
	if _, detail := decideVerdict(modeLive, probeObservation{
		snapshotCompared: true, streamCompared: true, eventsForTable: 0,
	}); strings.Contains(detail, "column present in the event stream") {
		t.Fatalf("zero-event run still claims the column was present: %s", detail)
	}
}

// TestSweepClassifierFlakeIsNotStuck pins the STUCK/flake boundary. STUCK is a product
// verdict meaning "the importer is retrying a specific event and cannot get past it", so
// it may only be emitted when that error can actually be quoted. A run where no event
// ever flowed - a slow or dead Debezium JVM - looks superficially identical (a wait
// expired, nothing arrived) but says nothing about any datatype.
func TestSweepClassifierFlakeIsNotStuck(t *testing.T) {
	cases := []struct {
		name string
		obs  probeObservation
		want string
	}{
		{
			// The real thing: a quotable repeating error.
			name: "wait expired with a quotable importer error is STUCK",
			obs: probeObservation{
				snapshotCompared: true, streamCompared: true,
				eventsForTable: 3, columnSeenInEvents: true, waitTimedOut: true,
				stuckDetail: `SQLSTATE 42804: ERROR: column "v" is of type foo[] but expression is of type text repeated x14`,
			},
			want: verdictStuck,
		},
		{
			// The flake: wait expired, zero events, nothing to quote.
			name: "wait expired with zero events and no error is not STUCK",
			obs: probeObservation{
				snapshotCompared: true, streamCompared: true,
				eventsForTable: 0, waitTimedOut: true,
			},
			want: verdictInconclusive,
		},
		{
			name: "export never reached streaming is not STUCK",
			obs: probeObservation{
				exportNeverStreamed: true,
				flakeDetail:         "export data never reached streaming mode within 8m0s",
				waitTimedOut:        true,
			},
			want: verdictInconclusive,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, detail := decideVerdict(modeLive, tc.obs)
			if got != tc.want {
				t.Fatalf("decideVerdict = %s, want %s (detail: %s)", got, tc.want, detail)
			}
		})
	}
}

// TestImportFailureSignature pins which log lines may be quoted as evidence of a wedged
// importer. Keyword matching on /error/ was tried twice and produced two waves of bogus
// STUCK verdicts, so the rule is positive signature matching plus outright rejection of
// struct-dump shapes. Both real offenders are must-not-match cases here.
func TestImportFailureSignature(t *testing.T) {
	mustNotMatch := []string{
		// Wave 1: a spew dump of an empty error value.
		`Error: (string) ""`,
		// Wave 2: a spew dump of a CONFIG FIELD whose VALUE contains the word ERROR.
		// This one passed a 15-char payload threshold and produced 19 bogus STUCKs.
		`PKConflictAction: (string) (len=12) "ERROR-POLICY"`,
		`	PKConflictAction: (string) (len=12) "ERROR-POLICY",`,
		`OnPrimaryKeyConflictAction: (string) (len=5) "ERROR"`,
		// An error word that exists only as a quoted value is not a message.
		`config: mode set to "error-policy"`,
		`TableName: (string) (len=7) "failed_"`,
		// Prose with no failure signature at all.
		`Waiting for streaming mode`,
	}
	for _, line := range mustNotMatch {
		if isImportFailureSignature(line) {
			t.Errorf("isImportFailureSignature(%q) = true, want false", line)
		}
		if got, n := mostRepeatedError(strings.Repeat(line+"\n", 30), ""); got != "" {
			t.Errorf("mostRepeatedError(%q) = %q x%d, want no match", line, got, n)
		}
	}

	mustMatch := []string{
		// The shape a genuine importer failure actually has in these logs.
		`import batch: "p_val_001/batch::1": flow=copy_normal: step=copy: ERROR: DECIMAL does not support NaN yet (SQLSTATE 0A000): dbcontext=[host=localhost]`,
		`[import data] error executing batch on channel 5: error preparing statements for events in batch (48:60) or when executing event with vsn(48): ERROR: syntax error at end of jsonpath input (SQLSTATE 42601)`,
		`ERROR: type "sweep_schema.p_mrange_003_mr" does not exist (SQLSTATE 42704)`,
		`flow=copy_normal: step=copy: ERROR: cannot cast type bytea to xid`,
	}
	for _, line := range mustMatch {
		if !isImportFailureSignature(line) {
			t.Errorf("isImportFailureSignature(%q) = false, want true", line)
		}
	}

	// A real error must still be quoted with its SQLSTATE preserved.
	real := `import batch: "x": flow=copy_normal: step=copy: ERROR: DECIMAL does not support NaN yet (SQLSTATE 0A000)`
	got, n := mostRepeatedError(strings.Repeat(real+"\n", 30), "")
	if got == "" || n != 30 {
		t.Fatalf("mostRepeatedError on a real error = %q x%d, want it quoted x30", got, n)
	}
	if !strings.Contains(got, "0A000") {
		t.Errorf("quoted error lost its SQLSTATE: %q", got)
	}

	// A run whose log holds ONLY the config dump must not yield a stuck detail at all,
	// which is what turns the bogus STUCK into a correctly-labelled flake.
	dump := strings.Repeat("\tPKConflictAction: (string) (len=12) \"ERROR-POLICY\",\n", 40)
	if got, n := mostRepeatedError(dump, ""); got != "" {
		t.Errorf("config dump still produced a stuck detail: %q x%d", got, n)
	}
}

// TestSweepClassifierImportDeathBeatsValueMismatch pins the ordering that the audit found
// inverted: a run in which the importer DIED on a classified SQL error was coming out
// SILENT_WRONG / SILENT_LOSS, because the value comparison ran first and found the target
// still holding its snapshot value.
//
// That is the worst kind of wrong answer the harness can give. SILENT_WRONG claims voyager
// changed a value without telling anyone; the run it is claimed about is one where voyager
// refused the value loudly, printed the SQLSTATE, and exited. The stale target row is the
// CONSEQUENCE of the death, not an independent finding.
//
// Shape taken from the real solo LIVE run of VAL-006 (float8 NaN), whose import command
// exited on `ERROR: column "nan" does not exist (SQLSTATE 42703)`.
func TestSweepClassifierImportDeathBeatsValueMismatch(t *testing.T) {
	const importErr = `[import data] error executing batch on channel 92: error executing ` +
		`batch: error preparing statements for events in batch (17:17) or when executing ` +
		`event with vsn(17): ERROR: column "nan" does not exist (SQLSTATE 42703)`

	// Everything the run really observed: the values differ (the target is frozen at its
	// pre-delta state) AND the importer is gone with a quotable error.
	obs := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 4, columnSeenInEvents: true, columnSeenOps: "insert/update",
		deltaOpsApplied: 6, deltaConfirmed: true,
		streamVerdict: verdictSilentWrong,
		streamDetail:  `streaming source->target: [update-this-column] id=1 source="1.5" destination="NaN"`,
		waitTimedOut:  true, commandExited: true,
		commandExitDetail: "import data exited during the forward streaming wait",
		stuckDetail:       importFailureDetail("SQLSTATE 42703: " + importErr),
	}

	got, detail := decideVerdict(modeLive, obs)
	if got != verdictStuck {
		t.Fatalf("decideVerdict = %s, want %s (detail: %s)", got, verdictStuck, detail)
	}
	if !strings.Contains(detail, `column "nan" does not exist`) {
		t.Errorf("detail does not quote the importer error: %s", detail)
	}
	if !strings.Contains(detail, "(SQLSTATE 42703)") {
		t.Errorf("detail does not carry a machine-readable SQLSTATE: %s", detail)
	}
	if strings.Contains(detail, "destination=") {
		t.Errorf("detail still leads with the value mismatch: %s", detail)
	}

	// The same evidence without the death is still a value verdict: this ordering must
	// not swallow real SILENT_WRONG findings.
	noDeath := obs
	noDeath.stuckDetail, noDeath.commandExited, noDeath.waitTimedOut = "", false, false
	if got, _ := decideVerdict(modeLive, noDeath); got != verdictSilentWrong {
		t.Errorf("without an importer death the value verdict = %s, want %s", got, verdictSilentWrong)
	}
}

// TestImportFailureDetailCarriesSQLState pins the one format the report collector can read
// a SQLSTATE out of. sweepreport/results.go matches either "SQLSTATE 42703" or a
// parenthesised "(42703)", so the token is written the way the importer writes it.
func TestImportFailureDetailCarriesSQLState(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`ERROR: DECIMAL does not support NaN yet (SQLSTATE 0A000)`, "(SQLSTATE 0A000)"},
		{`SQLSTATE 42703: ERROR: column "nan" does not exist`, "(SQLSTATE 42703)"},
		{`ERROR: time zone displacement out of range (SQLSTATE 22009)`, "(SQLSTATE 22009)"},
	}
	for _, tc := range cases {
		got := importFailureDetail(tc.in)
		if !strings.Contains(got, tc.want) {
			t.Errorf("importFailureDetail(%q) = %q, want it to carry %s", tc.in, got, tc.want)
		}
		if !strings.Contains(got, tc.in) {
			t.Errorf("importFailureDetail(%q) dropped the original text: %q", tc.in, got)
		}
		// Never appended twice when the text already carries the token.
		if n := strings.Count(got, tc.want); n != 1 {
			t.Errorf("importFailureDetail(%q) repeated the SQLSTATE %d times: %q", tc.in, n, got)
		}
	}
	// Nothing to add when there is no SQLSTATE at all.
	plain := "connection reset by peer"
	if got := importFailureDetail(plain); got != plain {
		t.Errorf("importFailureDetail(%q) = %q, want it unchanged", plain, got)
	}
}

// TestSweepClassifierTimedOutWaitIsNotAPass pins the second half of the same rule: a wait
// that ran out of clock may only pass on the strength of an event stream that confirms the
// column. "The values match" is equally true of two sides that were both left untouched,
// so on its own it is not evidence of a round trip.
func TestSweepClassifierTimedOutWaitIsNotAPass(t *testing.T) {
	measured := probeObservation{
		snapshotCompared: true, streamCompared: true,
		deltaOpsApplied: 6, deltaConfirmed: true, waitTimedOut: true,
	}

	// Timed out and NOTHING for this table ever appeared in the queue.
	got, detail := decideVerdict(modeLive, measured)
	if got != verdictInconclusive {
		t.Fatalf("timed-out wait with zero events = %s, want %s (detail: %s)",
			got, verdictInconclusive, detail)
	}
	if strings.Contains(detail, "values identical") {
		t.Errorf("inconclusive run still claims identical values as a pass: %s", detail)
	}

	// Timed out, but the stream confirms the column was carried: still a pass, with the
	// timeout stated.
	confirmed := measured
	confirmed.eventsForTable, confirmed.columnSeenInEvents = 6, true
	confirmed.columnSeenOps = "insert/update/delete"
	got, detail = decideVerdict(modeLive, confirmed)
	if got != verdictWorks {
		t.Fatalf("timed-out wait with a confirmed column = %s, want %s (detail: %s)",
			got, verdictWorks, detail)
	}
	if !strings.Contains(detail, "insert/update/delete") {
		t.Errorf("pass detail does not say which ops carried the column: %s", detail)
	}
	if !strings.Contains(detail, "timeout") {
		t.Errorf("pass detail hides the timeout: %s", detail)
	}
}

// TestSweepClassifierFailedDeltaIsNotAPass pins item 5 of the audit: the change statements
// are what this harness actually tests, so a delta the database refused, or one that never
// became visible on the side it was written to, leaves both sides holding their PRE-delta
// state. They then compare identical - indistinguishable from a clean round trip, and
// until now reported as one.
func TestSweepClassifierFailedDeltaIsNotAPass(t *testing.T) {
	clean := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 6, columnSeenInEvents: true,
		deltaOpsApplied: 6, deltaConfirmed: true,
	}
	if got, _ := decideVerdict(modeLive, clean); got != verdictWorks {
		t.Fatalf("the clean baseline is not WORKS, so the rest of this test proves nothing")
	}

	// The statements were refused outright.
	refused := clean
	refused.deltaError = `delta on target failed: ERROR: cannot cast type bytea to xid`
	got, detail := decideVerdict(modeLive, refused)
	if got != verdictInconclusive {
		t.Fatalf("refused delta = %s, want %s (detail: %s)", got, verdictInconclusive, detail)
	}
	if !strings.Contains(detail, "cannot cast type bytea to xid") {
		t.Errorf("detail does not carry the delta error: %s", detail)
	}

	// The statements ran, but the side they were written to never showed them.
	unconfirmed := clean
	unconfirmed.deltaConfirmed = false
	if got, detail := decideVerdict(modeLive, unconfirmed); got != verdictInconclusive {
		t.Fatalf("unconfirmed delta with events = %s, want %s (detail: %s)",
			got, verdictInconclusive, detail)
	}
}

// TestSweepClassifierUnattributedBreakageIsNotStuck pins item 8: when the importer breaks
// on an error that names no probe's table, in a batch with more than one probe under test,
// the batch-wide error used to be pinned on EVERY active probe as STUCK. One poison value
// then failed all of its batch-mates, and the report showed a dozen types as broken on the
// strength of one error that mentioned none of them.
func TestSweepClassifierUnattributedBreakageIsNotStuck(t *testing.T) {
	obs := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 3, columnSeenInEvents: true, waitTimedOut: true,
		importBrokeUnattributed: importFailureDetail(
			`SQLSTATE 0A000: ERROR: DECIMAL does not support NaN yet (SQLSTATE 0A000) repeated x14`),
	}
	got, detail := decideVerdict(modeLive, obs)
	if got != verdictInconclusive {
		t.Fatalf("unattributed importer breakage = %s, want %s (detail: %s)",
			got, verdictInconclusive, detail)
	}
	if !strings.Contains(detail, "another probe in this batch broke the importer") {
		t.Errorf("detail does not say the breakage was someone else's: %s", detail)
	}
	if !strings.Contains(detail, "this type was not measured") {
		t.Errorf("detail does not say this type went unmeasured: %s", detail)
	}
	// The error is still carried, so the row is not a dead end for whoever reads it.
	if !strings.Contains(detail, "DECIMAL does not support NaN yet") {
		t.Errorf("detail dropped the error that broke the importer: %s", detail)
	}

	// A probe the error DOES name keeps its STUCK verdict.
	named := probeObservation{
		snapshotCompared: true, streamCompared: true, waitTimedOut: true,
		stuckDetail: `SQLSTATE 0A000: ERROR: DECIMAL does not support NaN yet (SQLSTATE 0A000)`,
	}
	if got, _ := decideVerdict(modeLive, named); got != verdictStuck {
		t.Errorf("the named culprit = %s, want %s", got, verdictStuck)
	}
}

// TestSoleNonControlAttribution pins the carve-out that lets a solo run blame its one
// probe: an importer error naming no table, in a batch with exactly one probe under test,
// has exactly one possible cause. With two or more it has none.
func TestSoleNonControlAttribution(t *testing.T) {
	ctrl := []datatypeProbe{
		{ID: "CTRL-001", TypeName: "int", ExpectVerdict: verdictWorks},
		{ID: "CTRL-002", TypeName: "text", ExpectVerdict: verdictWorks},
	}
	solo := append(append([]datatypeProbe{}, ctrl...), datatypeProbe{ID: "VAL-006", TypeName: "float8"})
	if id, ok := soleNonControl(solo); !ok || id != "VAL-006" {
		t.Errorf("soleNonControl(solo batch) = %q,%v, want VAL-006,true", id, ok)
	}

	two := append(append([]datatypeProbe{}, solo...), datatypeProbe{ID: "VAL-007", TypeName: "numeric"})
	if id, ok := soleNonControl(two); ok {
		t.Errorf("soleNonControl(two probes) = %q,true, want no attribution", id)
	}

	if id, ok := soleNonControl(ctrl); ok {
		t.Errorf("soleNonControl(controls only) = %q,true, want no attribution", id)
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it printed. The
// harness's machine-readable lines (RUN-META, PROBE-RESULT, PROBE-WAIT) go to stdout
// rather than through t.Log, so this is the only way to assert on them.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = pw
	done := make(chan string, 1)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, pr)
		done <- b.String()
	}()
	func() {
		defer func() {
			os.Stdout = saved
			pw.Close()
		}()
		fn()
	}()
	out := <-done
	pr.Close()
	return out
}

// TestRunMetaSurvivesAFailedSetup pins the rule that the run header can never take the
// test binary down with it.
//
// emitRunMeta is printed for EVERY run, including one whose containers never came up,
// because the report collector needs a header for every log it reads. Asking such a run
// for its server versions used to dereference a container that was never created (a nil
// interface, so a panic) or, for a target that started without a published port, walk
// into YugabyteDBContainer.GetConnectionWithDB's utils.ErrExit - an os.Exit(1) that takes
// the whole `go test` invocation with it, losing the PROBE-RESULT lines of this batch AND
// of every batch queued behind it.
func TestRunMetaSurvivesAFailedSetup(t *testing.T) {
	started := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	fixtures := []struct {
		name string
		run  *sweepRun
	}{
		{"no fixture at all", &sweepRun{t: t}},
		{"a fixture whose containers never started", &sweepRun{t: t, lm: &LiveMigrationTest{t: t}}},
	}
	for _, f := range fixtures {
		// dbsUp=false is what runDatatypeSweep passes when SetupContainers failed.
		out := captureStdout(t, func() { f.run.emitRunMeta(started, false) })
		if !strings.Contains(out, "pg=unknown yb=unknown") {
			t.Errorf("%s: header does not degrade to unknown versions: %q", f.name, out)
		}
		if !strings.Contains(out, "started=2026-09-11T12:00:00Z") {
			t.Errorf("%s: header lost its start time: %q", f.name, out)
		}

		// And serverVersion itself must be safe even when a caller forgets the gate.
		out = captureStdout(t, func() { f.run.emitRunMeta(started, true) })
		if !strings.Contains(out, "pg=unknown yb=unknown") {
			t.Errorf("%s: serverVersion is not nil-safe: %q", f.name, out)
		}
	}
}

// TestSoloAttributionRequiresATypeRelatedError pins the content gate on the solo
// carve-out.
//
// "Exactly one probe was under test" answers who COULD have caused an importer failure,
// not who DID. Without a gate on the error's content, a tserver restart, a dropped
// connection, a lock timeout or one of voyager's own metadata errors during a solo run
// was published as STUCK - "Import stops" - against a type that may be perfectly fine.
func TestSoloAttributionRequiresATypeRelatedError(t *testing.T) {
	probes := []datatypeProbe{
		{ID: "CTRL-001", TypeName: "int", ExpectVerdict: verdictWorks},
		{ID: "CTRL-002", TypeName: "text", ExpectVerdict: verdictWorks},
		{ID: "VAL-006", TypeName: "float8 (NaN)"},
	}

	cases := []struct {
		name        string
		errText     string
		wantVerdict string
		wantIn      string
	}{
		{
			name:        "a tserver restart is the environment talking",
			errText:     `ERROR: terminating connection due to administrator command (SQLSTATE 57P01)`,
			wantVerdict: verdictInconclusive,
			wantIn:      "does not look type-related",
		},
		{
			name:        "a refused connection carries no SQLSTATE at all",
			errText:     `ERROR: dial tcp 127.0.0.1:5433: connect: connection refused`,
			wantVerdict: verdictInconclusive,
			wantIn:      "does not look type-related",
		},
		{
			name:        "a lock timeout is not a statement about a value",
			errText:     `ERROR: canceling statement due to lock timeout (SQLSTATE 55P03)`,
			wantVerdict: verdictInconclusive,
			wantIn:      "does not look type-related",
		},
		{
			name:        "a serialization failure is not a statement about a value",
			errText:     `ERROR: could not serialize access due to concurrent update (SQLSTATE 40001)`,
			wantVerdict: verdictInconclusive,
			wantIn:      "does not look type-related",
		},
		{
			name:        "the harness's own metadata table is never a type verdict",
			errText:     `ERROR: relation "ybvoyager_metadata.ybvoyager_import_data_batches_metainfo_v3" does not exist (SQLSTATE 42P01)`,
			wantVerdict: verdictInconclusive,
			wantIn:      "does not look type-related",
		},
		{
			name:        "a 42 class error naming no table is still SQL-level",
			errText:     `ERROR: column "nan" does not exist (SQLSTATE 42703)`,
			wantVerdict: verdictStuck,
			wantIn:      "(SQLSTATE 42703)",
		},
		{
			name:        "a data exception naming the probe's table",
			errText:     `ERROR: invalid input syntax for type numeric: "NaN" (SQLSTATE 22P02): table=sweep_schema.p_val_006`,
			wantVerdict: verdictStuck,
			wantIn:      "p_val_006",
		},
		{
			name:        "an unsupported feature is the classic type finding",
			errText:     `ERROR: DECIMAL does not support NaN yet (SQLSTATE 0A000)`,
			wantVerdict: verdictStuck,
			wantIn:      "(SQLSTATE 0A000)",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			culprit, unrelated := soleCulpritFor(probes, c.errText)

			// Build the observation exactly as recordCommandExit would for the one
			// probe under test, then let the real classifier speak.
			obs := probeObservation{
				snapshotCompared: true, streamCompared: true,
				eventsForTable: 3, columnSeenInEvents: true,
				waitTimedOut: true, commandExited: true,
				commandExitDetail: "import data exited during the streaming wait",
			}
			switch "VAL-006" {
			case culprit:
				obs.stuckDetail = importFailureDetail(c.errText)
			case unrelated:
				obs.importBrokeUnrelated = importFailureDetail(c.errText)
			default:
				t.Fatalf("the solo probe was neither blamed nor excused: culprit=%q unrelated=%q",
					culprit, unrelated)
			}

			got, detail := decideVerdict(modeLive, obs)
			if got != c.wantVerdict {
				t.Fatalf("verdict = %s, want %s (detail: %s)", got, c.wantVerdict, detail)
			}
			if !strings.Contains(detail, c.wantIn) {
				t.Errorf("detail does not carry %q: %s", c.wantIn, detail)
			}
			// Whatever the verdict, the error text itself is never dropped: an
			// INCONCLUSIVE row still has to tell the reader what actually happened.
			if !strings.Contains(detail, strings.SplitN(c.errText, " (SQLSTATE", 2)[0]) {
				t.Errorf("detail dropped the importer error: %s", detail)
			}
			if c.wantVerdict == verdictInconclusive &&
				!strings.Contains(detail, "this type was not measured") {
				t.Errorf("detail does not say this type went unmeasured: %s", detail)
			}
		})
	}

	// The gate applies only to the SOLO carve-out. With two probes under test there is
	// no carve-out to gate in the first place.
	two := append(append([]datatypeProbe{}, probes...), datatypeProbe{ID: "VAL-007", TypeName: "numeric(130,60)"})
	culprit, unrelated := soleCulpritFor(two, `ERROR: column "nan" does not exist (SQLSTATE 42703)`)
	if culprit != "" || unrelated != "" {
		t.Errorf("soleCulpritFor(two probes) = %q,%q, want no attribution either way", culprit, unrelated)
	}
}

// TestTypeNameUsableForAttribution pins the structural rule that replaced a nine-word
// denylist.
//
// The denylist let "interval" through, which is both a probe type name and a substring of
// Debezium's own `poll.interval.ms`; since `.` counts as an identifier boundary, a routine
// config line "named" the interval probe and an export death was published against it.
func TestTypeNameUsableForAttribution(t *testing.T) {
	// Bare English words: attributable by table name only, never by type name.
	for _, n := range []string{"interval", "enum", "name", "date", "time", "line", "text",
		"path", "point", "money", "char", "timestamptz", "hstore", "xml"} {
		if usableTypeNameForAttribution(n) {
			t.Errorf("%q is a bare word and must not be usable for attribution", n)
		}
	}
	// Anything carrying a digit, a space, a bracket or punctuation is distinctive.
	for _, n := range []string{"numeric(130,60)", "float8 (NaN)", "int4range[]", "float8",
		"timestamp(6) with time zone", "domain(enum)", "bit(3)"} {
		if !usableTypeNameForAttribution(n) {
			t.Errorf("%q contains a non-letter and must be usable for attribution", n)
		}
	}
	if usableTypeNameForAttribution("") || usableTypeNameForAttribution("   ") {
		t.Error("an empty type name must never be usable for attribution")
	}

	// The regression itself: a Debezium config line must not attribute an export death
	// to the interval probe.
	probes := []datatypeProbe{
		{ID: "CTRL-001", TypeName: "int", ExpectVerdict: verdictWorks},
		{ID: "RNG-004", TypeName: "interval"},
	}
	if id, ok := attributeExportFailure(probes, `poll.interval.ms = 500`); ok {
		t.Errorf("a Debezium config line attributed the export death to %s", id)
	}
	// The same probe is still attributable by its unambiguous table name.
	if id, ok := attributeExportFailure(probes, `Error processing sweep_schema.p_rng_004.v`); !ok || id != "RNG-004" {
		t.Errorf("table-name attribution = %q,%v, want RNG-004,true", id, ok)
	}
}

// TestDeltaSideNamedPerMode pins which database a zero-events detail sends the reader to.
// Fall-back and fall-forward judge the REVERSE direction, whose change ops are applied on
// the TARGET; saying "confirmed on the source" for those is a wrong-database wild goose
// chase for whoever reads the row.
func TestDeltaSideNamedPerMode(t *testing.T) {
	silent := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 0, deltaOpsApplied: 3, deltaConfirmed: true,
	}
	for _, tc := range []struct {
		mode sweepMode
		want string
	}{
		{modeLive, "confirmed on the source"},
		{modeFallback, "confirmed on the target"},
		{modeFallForward, "confirmed on the target"},
	} {
		got, detail := decideVerdict(tc.mode, silent)
		if got != verdictSilentLoss {
			t.Fatalf("%s: verdict = %s, want %s", tc.mode, got, verdictSilentLoss)
		}
		if !strings.Contains(detail, tc.want) {
			t.Errorf("%s: detail does not say %q: %s", tc.mode, tc.want, detail)
		}
	}
}

// TestImportLogFileTextIsImporterOnly pins that the exporter's log stays out of the text
// a STUCK verdict is quoted from. A Debezium line carrying a SQLState token would
// otherwise be quotable as "the importer will not take this value", collapsing STUCK and
// EXPORTER_CRASHES into one verdict - and the wrong one.
func TestImportLogFileTextIsImporterOnly(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(logs, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("yb-voyager-import-data.log", "ERROR: DECIMAL does not support NaN yet (SQLSTATE 0A000)\n")
	write("debezium.log", "ERROR: SQLState: 42883 no function matches (SQLSTATE 42883)\n")
	write("yb-voyager-export-data.log", "ERROR: export-side noise (SQLSTATE 58P01)\n")

	r := &sweepRun{t: t, lm: &LiveMigrationTest{t: t, exportDir: dir}}
	got := r.importLogFileText()
	if !strings.Contains(got, "DECIMAL does not support NaN") {
		t.Errorf("the importer's own log was dropped: %q", got)
	}
	for _, unwanted := range []string{"42883", "58P01"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("a non-importer log leaked into the import evidence (%s): %q", unwanted, got)
		}
	}
}

// TestQueueScanEarlyReturnZeroesEvidence pins that a direction with NO queue at all is
// judged on its own (absent) evidence.
//
// recordQueueColumnPresence runs once per direction, and in fall-back / fall-forward the
// second call is the one that judges the REVERSE direction. Returning early on a missing
// queue used to leave the FORWARD run's event counts in place, so a reverse direction that
// produced nothing whatsoever could still be passed on forward evidence.
func TestQueueScanEarlyReturnZeroesEvidence(t *testing.T) {
	p := datatypeProbe{ID: "VAL-006", TypeName: "float8 (NaN)"}
	r := &sweepRun{
		t:      t,
		mode:   modeFallback,
		lm:     &LiveMigrationTest{t: t, exportDir: t.TempDir()}, // no data/queue under it
		active: []datatypeProbe{p},
		obs: map[string]*probeObservation{
			// What the FORWARD run left behind.
			p.ID: {eventsForTable: 7, columnSeenInEvents: true, columnSeenOps: "insert/update/delete"},
		},
	}
	r.recordQueueColumnPresence()

	o := r.obs[p.ID]
	if o.eventsForTable != 0 || o.columnSeenInEvents || o.columnSeenOps != "" {
		t.Fatalf("forward-run evidence survived a queue-less reverse direction: "+
			"events=%d seen=%v ops=%q", o.eventsForTable, o.columnSeenInEvents, o.columnSeenOps)
	}
	if !strings.Contains(o.queueScanNote, "no queue segments") {
		t.Errorf("queueScanNote does not say the queue was missing: %q", o.queueScanNote)
	}
	// And the classifier must now refuse to pass it.
	o.snapshotCompared, o.streamCompared = true, true
	o.deltaOpsApplied, o.deltaConfirmed = 3, true
	if got, detail := decideVerdict(modeFallback, *o); got == verdictWorks {
		t.Errorf("a reverse direction with no queue classified as %s: %s", got, detail)
	}
}

// TestSweepClassifierImportDeathBeatsColumnAbsent pins the second half of the
// death-outranks-everything rule, found by running the sweep.
//
// Solo LIVE VAL-002 (numeric +Infinity) killed `import data` during the snapshot with
// `ERROR: DECIMAL does not support Infinity yet (SQLSTATE 0A000)`, and both controls were
// correctly INCONCLUSIVE naming VAL-002 as the killer - while VAL-002 itself came out
// SILENT_LOSS, because the column-absent branch ran before the dead-importer one. Calling
// that run silent is a false description of it: voyager printed a SQLSTATE and exited.
// VAL-001, whose column DID appear in the events, got STUCK from the same evidence.
//
// The missing column is still a real second signal (Debezium omitted it), so it is kept
// as a note rather than dropped.
func TestSweepClassifierImportDeathBeatsColumnAbsent(t *testing.T) {
	obs := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 3, columnSeenInEvents: false,
		waitTimedOut: true, commandExited: true,
		commandExitDetail: "import data exited during the snapshot wait after 449s",
		stuckDetail: importFailureDetail(
			`SQLSTATE 0A000: ERROR: DECIMAL does not support Infinity yet (SQLSTATE 0A000)`),
	}

	got, detail := decideVerdict(modeLive, obs)
	if got != verdictStuck {
		t.Fatalf("decideVerdict = %s, want %s (detail: %s)", got, verdictStuck, detail)
	}
	if !strings.Contains(detail, "(SQLSTATE 0A000)") {
		t.Errorf("the STUCK detail does not carry the SQLSTATE: %s", detail)
	}
	if !strings.Contains(detail, "DECIMAL does not support Infinity yet") {
		t.Errorf("the STUCK detail does not quote the importer error: %s", detail)
	}
	// The column-absent fact survives as a secondary note.
	if !strings.Contains(detail, "absent from all 3") {
		t.Errorf("the STUCK detail dropped the column-absent fact: %s", detail)
	}
	if !strings.Contains(detail, "no exclusion warning") {
		t.Errorf("the note does not say whether export warned: %s", detail)
	}

	// Without the death the column-absent verdict is unchanged: this ordering must not
	// swallow a real SILENT_LOSS.
	alive := obs
	alive.stuckDetail, alive.commandExited, alive.waitTimedOut = "", false, false
	if got, d := decideVerdict(modeLive, alive); got != verdictSilentLoss {
		t.Errorf("without an importer death the column-absent verdict = %s, want %s (%s)",
			got, verdictSilentLoss, d)
	}
	// And a printed-and-confirmed exclusion is still EXCLUDED_TOLD, not STUCK.
	told := alive
	told.warned, told.promptShown = true, true
	if got, d := decideVerdict(modeLive, told); got != verdictExcludedTold {
		t.Errorf("a confirmed exclusion = %s, want %s (%s)", got, verdictExcludedTold, d)
	}
}

// TestQuotedImportErrorStartsAtTheError pins what a STUCK detail quotes.
//
// The line below is the real one from the solo LIVE run of VAL-001 (numeric NaN). Quoting
// it from the left spent the whole budget on the timestamp, the caller and a temp-dir
// path, and cut the message off mid-path:
//
//	SQLSTATE 0A000: 2026-09-11 16:15:20.714093 ERROR logging.go:57 import batch:
//	"/var/folders/78/.../table::\"sweep_s... (x1) - import data exited ...
//
// so the one fact the row exists to carry - which value the importer refused, and why -
// never reached the report.
func TestQuotedImportErrorStartsAtTheError(t *testing.T) {
	const realLine = `2026-09-11 16:15:20.714093 ERROR logging.go:57 import batch: ` +
		`"/var/folders/78/wmy1tdts1hd4j_wnhqnk2ct40000gn/T/yb-voyager-export384200852/metainfo/` +
		`import_data_state/target_db_importer/table::\"sweep_schema\".\"p_val_001\"/file::` +
		`p_val_001_data.sql::b05fc82c/batch::0.10.10.169.169.P" into sweep_schema.p_val_001: ` +
		`flow=copy_normal: step=copy: ERROR: DECIMAL does not support NaN yet (SQLSTATE 0A000): ` +
		`dbcontext=[where=COPY p_val_001, line 1: "1	filler-1	NaN"]`

	quoted, n := mostRepeatedError(realLine+"\n", "")
	if quoted == "" || n != 1 {
		t.Fatalf("the real importer line is not quotable: %q x%d", quoted, n)
	}
	if !strings.Contains(quoted, "DECIMAL does not support NaN yet (SQLSTATE 0A000)") {
		t.Errorf("the quote lost the error and its SQLSTATE: %q", quoted)
	}
	// The noise that used to fill the budget.
	if strings.Contains(quoted, "/var/folders") {
		t.Errorf("the quote still carries the temp-dir path: %q", quoted)
	}
	if strings.Contains(quoted, "2026-09-11 16:15:20") || strings.Contains(quoted, "logging.go:57") {
		t.Errorf("the quote still leads with the log-line prefix: %q", quoted)
	}
	// mostRepeatedError hoists the SQLSTATE itself, so the quote proper starts at the
	// last ERROR: - the server's, not voyager's wrapper.
	body := strings.TrimPrefix(quoted, "SQLSTATE 0A000: ")
	if !strings.HasPrefix(body, "ERROR: DECIMAL") {
		t.Errorf("the quote does not start at the error: %q", body)
	}
	// The context that makes the row actionable is kept, since it fits.
	if !strings.Contains(quoted, "sweep_schema.p_val_001") {
		t.Errorf("the quote dropped the table it happened on: %q", quoted)
	}

	// The form the report collector reads a SQLSTATE out of survives end to end.
	detail := importFailureDetail(quoted + " (x1) - import data exited during the snapshot wait")
	if !strings.Contains(detail, "(SQLSTATE 0A000)") {
		t.Errorf("the detail lost the collector's SQLSTATE form: %q", detail)
	}
}

// TestAttributedKillIsNotAFlake pins the run-level line a solo kill prints.
//
// In the solo runs the one probe under test killed `import data`, both controls came out
// INCONCLUSIVE naming it, and the run then announced
//
//	PROBE-RUN-FLAKE: solo_val_001 | LIVE | 2 inconclusive | probes came out INCONCLUSIVE
//
// which reads as an environment wobble and is the opposite of what happened. An
// attributed kill prints no FLAKE line at all; a genuinely environmental inconclusive
// still does.
//
// The controls carry no ExpectVerdict here on purpose: emitAll fails the test on a control
// that misses its expectation, and this fixture is about the FLAKE line rather than the
// control gate.
func TestAttributedKillIsNotAFlake(t *testing.T) {
	probes := []datatypeProbe{
		{ID: "CTRL-001", TypeName: "int"},
		{ID: "CTRL-002", TypeName: "text"},
		{ID: "VAL-001", TypeName: "numeric (NaN)"},
	}
	newRun := func(obs map[string]*probeObservation) *sweepRun {
		return &sweepRun{t: t, mode: modeLive, batch: "solo_val_001", probes: probes, obs: obs}
	}

	// What the solo VAL-001 run really observed.
	killed := newRun(map[string]*probeObservation{
		"CTRL-001": {channelWedgedBy: "VAL-001", channelWedgedHow: "killed import data",
			waitTimedOut: true, commandExited: true},
		"CTRL-002": {channelWedgedBy: "VAL-001", channelWedgedHow: "killed import data",
			waitTimedOut: true, commandExited: true},
		"VAL-001": {snapshotCompared: true, streamCompared: true,
			eventsForTable: 2, columnSeenInEvents: true, waitTimedOut: true, commandExited: true,
			stuckDetail: importFailureDetail(
				`SQLSTATE 0A000: ERROR: DECIMAL does not support NaN yet (SQLSTATE 0A000)`)},
	})
	out := captureStdout(t, killed.emitAll)
	if strings.Contains(out, "PROBE-RUN-FLAKE") {
		t.Errorf("an attributed kill still printed a FLAKE line:\n%s", out)
	}
	// The cause is still on the record, twice over.
	if !strings.Contains(out, "PROBE-RESULT: VAL-001 | numeric (NaN) | LIVE | STUCK") {
		t.Errorf("the killer's STUCK line is missing:\n%s", out)
	}
	if !strings.Contains(out, "probe VAL-001 killed import data") {
		t.Errorf("the controls no longer name the killer:\n%s", out)
	}

	// An environmental inconclusive - no events ever flowed, nobody to blame - still
	// prints the FLAKE line.
	envFlake := newRun(map[string]*probeObservation{
		"CTRL-001": {snapshotCompared: true, streamCompared: true, waitTimedOut: true},
		"CTRL-002": {snapshotCompared: true, streamCompared: true, waitTimedOut: true},
		"VAL-001":  {snapshotCompared: true, streamCompared: true, waitTimedOut: true},
	})
	out = captureStdout(t, envFlake.emitAll)
	if !strings.Contains(out, "PROBE-RUN-FLAKE") {
		t.Errorf("an environmental inconclusive lost its FLAKE line:\n%s", out)
	}

	// So does a run whose exporter died: every INCONCLUSIVE behind it is collateral of
	// something the batch cannot be re-run past.
	exportDied := newRun(map[string]*probeObservation{
		"CTRL-001": {exporterDiedInRun: "NullPointerException", waitTimedOut: true},
		"CTRL-002": {exporterDiedInRun: "NullPointerException", waitTimedOut: true},
		"VAL-001":  {exporterDiedInRun: "NullPointerException", waitTimedOut: true},
	})
	exportDied.exportDeath = "NullPointerException in TypeRegistry"
	out = captureStdout(t, exportDied.emitAll)
	if !strings.Contains(out, "PROBE-RUN-FLAKE") {
		t.Errorf("an export death lost its FLAKE line:\n%s", out)
	}
}

// ============================================================
// DEFECT 1: an offline import failure names its culprit
// ============================================================

// offValuesRealError is verbatim from off_values.log of the 2026-09-11 re-run: the reason
// string every single probe in the `values` batch was given, controls included.
const offValuesRealError = `import data failed: failed to start import data: command failed: ` +
	`exit status 1; importer error: SQLSTATE 22007: ERROR: invalid input syntax for type ` +
	`interval: "infinity" (SQLSTATE 22007) [import batch ... into sweep_schema.p_val_033]`

// offCatalogStatsRealError is the same shape from off_catalogstats.log.
const offCatalogStatsRealError = `import data failed: failed to start import data: command failed: ` +
	`exit status 1; importer error: SQLSTATE 0A000: ERROR: cannot accept a value of type ` +
	`pg_node_tree (SQLSTATE 0A000) [import batch ... into sweep_schema.p_catstat_002]`

/*
TestOfflineImportAbortNamesTheCulprit is the offline half of the attribution rule.

The OFFLINE flow used to hand the run-level abort reason to EVERY probe, so the whole batch
came out BLOCKS - including the known-good int and text controls, which made the run
INVALID and threw away 46 measured cells. Nothing named the value that did it, so the
re-run had nothing to exclude.

The importer names the table in as many words: `into sweep_schema.p_val_033`.
*/
func TestOfflineImportAbortNamesTheCulprit(t *testing.T) {
	probes := []datatypeProbe{
		{ID: "CTRL-001", TypeName: "int", ExpectVerdict: verdictWorks},
		{ID: "CTRL-002", TypeName: "text", ExpectVerdict: verdictWorks},
		{ID: "VAL-008", TypeName: "float8 (-0.0)"},
		{ID: "VAL-033", TypeName: "interval (+/-infinity)"},
	}
	r := &sweepRun{t: t, mode: modeOffline, batch: "values",
		probes: probes, active: probes, obs: map[string]*probeObservation{}}

	out := captureStdout(t, func() { r.applyImportAbort(offValuesRealError) })

	// The culprit, and only the culprit, BLOCKS - with the error and its SQLSTATE quoted.
	got, detail := verdictOf(r, "VAL-033")
	if got != verdictBlocks {
		t.Errorf("VAL-033 = %s, want %s (%s)", got, verdictBlocks, detail)
	}
	if !strings.Contains(detail, `invalid input syntax for type interval: "infinity"`) {
		t.Errorf("the BLOCKS detail does not quote the importer error: %s", detail)
	}
	if !strings.Contains(detail, "(SQLSTATE 22007)") {
		t.Errorf("the BLOCKS detail does not carry the SQLSTATE: %s", detail)
	}

	// Everyone else is collateral, not a finding. The controls in particular must no
	// longer be BLOCKS: a control that BLOCKS is what made the whole run invalid with
	// nothing to exclude on the re-run.
	for _, id := range []string{"CTRL-001", "CTRL-002", "VAL-008"} {
		got, detail := verdictOf(r, id)
		if got != verdictInconclusive {
			t.Errorf("%s = %s, want %s (%s)", id, got, verdictInconclusive, detail)
		}
		if !strings.Contains(detail, "probe VAL-033 killed import data during this run") {
			t.Errorf("%s does not name the culprit: %s", id, detail)
		}
		// Offline has no event stream, so the wording must not claim one.
		if strings.Contains(detail, "every event for this table was stuck behind it") {
			t.Errorf("%s uses CDC wording in an OFFLINE run: %s", id, detail)
		}
	}

	// And the runner gets its greppable quarantine line, which is the thing the batch
	// abort never produced: without it the re-run has nothing to exclude.
	if !strings.Contains(out, "PROBE-RUN-QUARANTINE: values | OFFLINE | VAL-033 (interval (+/-infinity)) killed import data") {
		t.Errorf("no PROBE-RUN-QUARANTINE line for the culprit:\n%s", out)
	}
	if !strings.Contains(out, "PROBE_ID=VAL-033 PROBE_MODE=OFFLINE") {
		t.Errorf("the quarantine line does not say how to re-measure the culprit:\n%s", out)
	}
	if len(r.quarantined) != 1 || r.quarantined[0] != "VAL-033" {
		t.Errorf("quarantined = %v, want [VAL-033]", r.quarantined)
	}
}

// TestOfflineImportAbortAttributesCatalogStats: the same offline path on the other real
// offline failure, whose error names a type rather than quoting a value.
func TestOfflineImportAbortAttributesCatalogStats(t *testing.T) {
	probes := []datatypeProbe{
		{ID: "CTRL-001", TypeName: "int", ExpectVerdict: verdictWorks},
		{ID: "CATSTAT-002", TypeName: "pg_node_tree", ColumnDDL: "pg_node_tree"},
		{ID: "CATSTAT-006", TypeName: "pg_brin_bloom_summary", NullOnly: true},
	}
	r := &sweepRun{t: t, mode: modeOffline, batch: "catalogstats",
		probes: probes, active: probes, obs: map[string]*probeObservation{}}

	out := captureStdout(t, func() { r.applyImportAbort(offCatalogStatsRealError) })

	if got, d := verdictOf(r, "CATSTAT-002"); got != verdictBlocks {
		t.Errorf("CATSTAT-002 = %s, want %s (%s)", got, verdictBlocks, d)
	}
	for _, id := range []string{"CTRL-001", "CATSTAT-006"} {
		if got, d := verdictOf(r, id); got != verdictInconclusive {
			t.Errorf("%s = %s, want %s (%s)", id, got, verdictInconclusive, d)
		}
	}
	if !strings.Contains(out, "PROBE-RUN-QUARANTINE: catalogstats | OFFLINE | CATSTAT-002") {
		t.Errorf("no quarantine line:\n%s", out)
	}
}

// TestOfflineImportAbortBlamesNobodyWhenNothingIsNamed: with no table, no matching value
// and more than one probe under test, nothing is guessed.
func TestOfflineImportAbortBlamesNobodyWhenNothingIsNamed(t *testing.T) {
	probes := []datatypeProbe{
		{ID: "CTRL-001", TypeName: "int", ExpectVerdict: verdictWorks},
		{ID: "MISC-001", TypeName: "uuid", ColumnDDL: "uuid"},
		{ID: "MISC-002", TypeName: "macaddr", ColumnDDL: "macaddr"},
	}
	r := &sweepRun{t: t, mode: modeOffline, batch: "misc",
		probes: probes, active: probes, obs: map[string]*probeObservation{}}

	const anonymous = "import data failed: failed to start import data: command failed: " +
		"exit status 1; importer error: SQLSTATE 08006: ERROR: connection to server was lost " +
		"(SQLSTATE 08006)"
	out := captureStdout(t, func() { r.applyImportAbort(anonymous) })

	for _, id := range []string{"CTRL-001", "MISC-001", "MISC-002"} {
		got, detail := verdictOf(r, id)
		if got != verdictInconclusive {
			t.Errorf("%s = %s, want %s (%s)", id, got, verdictInconclusive, detail)
		}
		if !strings.Contains(detail, "connection to server was lost") {
			t.Errorf("%s did not keep the error for context: %s", id, detail)
		}
	}
	if strings.Contains(out, "PROBE-RUN-QUARANTINE") {
		t.Errorf("an unattributable failure still quarantined somebody:\n%s", out)
	}
	if len(r.quarantined) != 0 {
		t.Errorf("quarantined = %v, want none", r.quarantined)
	}
}

// verdictOf classifies one probe of a run, by id.
func verdictOf(r *sweepRun, probeID string) (string, string) {
	for _, p := range r.probes {
		if p.ID == probeID {
			return decideVerdict(r.mode, *r.observe(p))
		}
	}
	return "", "no such probe"
}

// ============================================================
// DEFECT 2: a dead run's truncated stream is not evidence of a drop
// ============================================================

/*
TestSweepClassifierDeadRunCannotReadSilentLoss.

live_indexkeys.log and live_catalogstats.log both did this: the importer died
(`syntax error (42601)`, `cannot accept a value of type pg_node_tree (0A000)`), the
controls correctly came out INCONCLUSIVE with "another probe in this batch broke the
importer" - and the NULL-only probes in the same batch printed SILENT_LOSS anyway, on the
strength of a column being absent from an event stream that had been cut off mid-run.

The batch-mates of a dead run cannot be the only probes in it that produced evidence.
*/
func TestSweepClassifierDeadRunCannotReadSilentLoss(t *testing.T) {
	// IDXKEY-002 as live_indexkeys.log actually observed it.
	unattributed := probeObservation{
		nullOnly:       true,
		eventsForTable: 3, columnSeenInEvents: false,
		waitTimedOut: true, commandExited: true,
		importBrokeUnattributed: importFailureDetail(
			"SQLSTATE 42601: ERROR: syntax error (SQLSTATE 42601) (x1) - " +
				"import data exited during the forward streaming wait after 6s"),
	}
	got, detail := decideVerdict(modeLive, unattributed)
	if got != verdictInconclusive {
		t.Errorf("an unattributed importer death read as %s, want %s (%s)",
			got, verdictInconclusive, detail)
	}
	if !strings.Contains(detail, "another probe in this batch broke the importer") {
		t.Errorf("the detail does not say the run was broken by someone else: %s", detail)
	}
	if strings.Contains(detail, "no exclusion warning in export stdout/stderr") {
		t.Errorf("the detail still claims a silent drop: %s", detail)
	}

	// CATSTAT-006 as live_catalogstats.log observed it: a named culprit rather than an
	// anonymous one. Same rule.
	wedged := probeObservation{
		nullOnly:       true,
		eventsForTable: 3, columnSeenInEvents: false,
		waitTimedOut: true, commandExited: true,
		channelWedgedBy: "CATSTAT-002", channelWedgedHow: "killed import data",
	}
	got, detail = decideVerdict(modeLive, wedged)
	if got != verdictInconclusive {
		t.Errorf("a batch-mate of a wedging probe read as %s, want %s (%s)",
			got, verdictInconclusive, detail)
	}
	if !strings.Contains(detail, "probe CATSTAT-002 killed import data") {
		t.Errorf("the detail does not name the culprit: %s", detail)
	}

	// The guard must not swallow a real finding: with a healthy run the same shape is
	// still reported.
	healthy := unattributed
	healthy.importBrokeUnattributed, healthy.commandExited, healthy.waitTimedOut = "", false, false
	healthy.nullOnly = false
	if got, d := decideVerdict(modeLive, healthy); got != verdictSilentLoss {
		t.Errorf("a healthy run's column-absent verdict = %s, want %s (%s)",
			got, verdictSilentLoss, d)
	}
}

// ============================================================
// DEFECT 3: attribution from the value the importer quoted
// ============================================================

/*
TestValueAttributionNamesTheProbeFromTheQuotedValue.

Every line below is verbatim from the 2026-09-11 live logs. None of them names a table, so
attributeCrashLoop declined and the whole batch went INCONCLUSIVE - one poison value cost
every batch-mate its measurement, repeatedly.

The literals arrive hex-encoded because the value travelled as bytes:

	\x302f30                             = "0/0"
	\x5b302e312c302e322c302e335d         = "[0.1,0.2,0.3]"
	\x3937372d313433362d3435322d30302d38 = "977-1436-452-00-8"
*/
func TestValueAttributionNamesTheProbeFromTheQuotedValue(t *testing.T) {
	domains := []datatypeProbe{
		{ID: "CTRL-001", TypeName: "int", ExpectVerdict: verdictWorks},
		{ID: "DOM-012", TypeName: "domain(timetz)",
			PreDDL:       []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS timetz"},
			ColumnDDL:    "{{schema}}.{{p}}_d",
			InitialValue: "'12:34:56+05:30'::timetz", AltValue: "'00:00:00+00'::timetz"},
		{ID: "DOM-013", TypeName: "domain(pg_lsn)",
			PreDDL:       []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS pg_lsn"},
			ColumnDDL:    "{{schema}}.{{p}}_d",
			InitialValue: "'16/B374D848'::pg_lsn", AltValue: "'0/0'::pg_lsn"},
	}
	pgvector := []datatypeProbe{
		{ID: "VEC-001", TypeName: "vector(3)", ColumnDDL: "vector(3)",
			InitialValue: "'[1,2,3]'::vector(3)", AltValue: "'[4,5,6]'::vector(3)"},
		{ID: "VEC-002", TypeName: "vector[]", ColumnDDL: "vector[]",
			InitialValue: "ARRAY['[1,2,3]'::vector, '[4,5,6]'::vector]",
			AltValue:     "ARRAY['[7,8,9]'::vector]"},
		{ID: "VEC-003", TypeName: "domain(vector(3))",
			PreDDL:       []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS vector(3)"},
			ColumnDDL:    "{{schema}}.{{p}}_d",
			InitialValue: "'[0.1,0.2,0.3]'::vector(3)", AltValue: "'[0.4,0.5,0.6]'::vector(3)"},
	}
	exttypes := []datatypeProbe{
		{ID: "EXT-008", TypeName: "issn", ColumnDDL: "issn",
			InitialValue: "'1436-4522'::issn", AltValue: "'0264-2875'::issn"},
		{ID: "EXT-009", TypeName: "issn13", ColumnDDL: "issn13",
			InitialValue: "'1436-4522'::issn13", AltValue: "'0264-2875'::issn13"},
	}
	catalogtypes := []datatypeProbe{
		{ID: "CAT-001", TypeName: "cid", ColumnDDL: "cid"},
		{ID: "CAT-002", TypeName: "oidvector", ColumnDDL: "oidvector"},
		{ID: "CAT-003", TypeName: "refcursor", ColumnDDL: "refcursor"},
	}
	system := []datatypeProbe{
		{ID: "SYS-006", TypeName: "int2vector", ColumnDDL: "int2vector"},
		{ID: "SYS-010", TypeName: "oid", ColumnDDL: "oid",
			InitialValue: "'4294967295'::oid", AltValue: "'1'::oid"},
	}

	tests := []struct {
		name    string
		probes  []datatypeProbe
		errText string
		want    string // "" means no attribution
		why     string
	}{
		{
			name: "pg_lsn literal names DOM-013", probes: domains,
			errText: `SQLSTATE 22P02: ERROR: invalid input syntax for type pg_lsn: "\x302f30" (SQLSTATE 22P02)`,
			want:    "DOM-013",
			why:     `\x302f30 decodes to 0/0, which is DOM-013's AltValue`,
		},
		{
			name: "vector literal names VEC-003", probes: pgvector,
			errText: `SQLSTATE 22P02: ERROR: invalid input syntax for type vector: "\x5b302e312c302e322c302e335d" (SQLSTATE 22P02)`,
			want:    "VEC-003",
			why: `[0.1,0.2,0.3] is VEC-003's InitialValue; the type name "vector" alone ` +
				`matches VEC-001, VEC-002 and VEC-003, so only the literal resolves it`,
		},
		{
			name: "ISSN type name names EXT-008", probes: exttypes,
			errText: `SQLSTATE 22P02: ERROR: invalid input syntax for ISSN number: "\x3937372d313433362d3435322d30302d38" (SQLSTATE 22P02)`,
			want:    "EXT-008",
			why: `the stored form 977-1436-452-00-8 is nobody's declared literal, but "issn" ` +
				`is exactly one probe's column type (issn13 is a different type)`,
		},
		{
			name: "oid in a batch that declares none", probes: catalogtypes,
			errText: `SQLSTATE 22P02: ERROR: invalid input syntax for type oid: "[]" (SQLSTATE 22P02)`,
			want:    "",
			why: `no probe in the catalogtypes batch declares oid - oidvector is a different ` +
				`type - and "[]" is nobody's value, so nothing is guessed`,
		},
		{
			name: "smallint in a batch that declares none", probes: system,
			errText: `SQLSTATE 22P02: ERROR: invalid input syntax for type smallint: "[]" (SQLSTATE 22P02)`,
			want:    "",
			why:     `int2vector is not int2, and "[]" is nobody's value`,
		},
		{
			name: "oid names SYS-010 when the batch does declare it", probes: system,
			errText: `SQLSTATE 22P02: ERROR: invalid input syntax for type oid: "99" (SQLSTATE 22P02)`,
			want:    "SYS-010",
			why:     `SYS-010 is the only oid column in the batch`,
		},
		{
			name: "cannot-accept names the type with no literal at all",
			probes: []datatypeProbe{
				{ID: "CTRL-001", TypeName: "int", ExpectVerdict: verdictWorks},
				{ID: "CATSTAT-002", TypeName: "pg_node_tree", ColumnDDL: "pg_node_tree"},
				{ID: "CATSTAT-006", TypeName: "pg_brin_bloom_summary", ColumnDDL: "pg_brin_bloom_summary"},
			},
			errText: `SQLSTATE 0A000: ERROR: cannot accept a value of type pg_node_tree (SQLSTATE 0A000)`,
			want:    "CATSTAT-002",
			why:     `the error names a type and quotes no value; exactly one probe declares it`,
		},
		{
			name: "a control is never blamed",
			probes: []datatypeProbe{
				{ID: "CTRL-001", TypeName: "int", ColumnDDL: "int4",
					InitialValue: "1", AltValue: "2", ExpectVerdict: verdictWorks},
			},
			errText: `SQLSTATE 22P02: ERROR: invalid input syntax for type integer: "2" (SQLSTATE 22P02)`,
			want:    "",
			why:     `blaming a known-good control would say the harness found a product bug in int`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := attributeByValue(tc.probes, tc.errText)
			if tc.want == "" {
				if ok {
					t.Fatalf("blamed %s for an error nobody can be blamed for (%s)\nerror: %s",
						got, tc.why, tc.errText)
				}
				return
			}
			if !ok {
				t.Fatalf("nobody was blamed; want %s (%s)\nerror: %s", tc.want, tc.why, tc.errText)
			}
			if got != tc.want {
				t.Fatalf("blamed %s, want %s (%s)\nerror: %s", got, tc.want, tc.why, tc.errText)
			}
		})
	}
}

// TestValueAttributionKeepsTableNamesFirst: a table name in the error still wins, because
// it is the stronger fact. Only when there is none does the value pass run.
func TestValueAttributionKeepsTableNamesFirst(t *testing.T) {
	probes := []datatypeProbe{
		{ID: "DOM-013", TypeName: "domain(pg_lsn)",
			PreDDL:       []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS pg_lsn"},
			ColumnDDL:    "{{schema}}.{{p}}_d",
			InitialValue: "'16/B374D848'::pg_lsn", AltValue: "'0/0'::pg_lsn"},
		{ID: "SYS-008", TypeName: "pg_lsn", ColumnDDL: "pg_lsn",
			InitialValue: "'16/B374D848'::pg_lsn", AltValue: "'0/0'::pg_lsn"},
	}
	// Both probes carry the same value AND the same type, so the value pass must decline.
	if id, ok := attributeByValue(probes, `invalid input syntax for type pg_lsn: "\x302f30"`); ok {
		t.Errorf("an ambiguous value blamed %s; it must blame nobody", id)
	}
	// The table name resolves it.
	withTable := `import batch ... into sweep_schema.p_sys_008: ERROR: invalid input syntax ` +
		`for type pg_lsn: "\x302f30"`
	id, ok := attributeImportFailure(probes, withTable)
	if !ok || id != "SYS-008" {
		t.Errorf("attributeImportFailure = (%q, %v), want SYS-008", id, ok)
	}
}

// TestValueAttributionHelpersAreExact pins the two normalisers the matching rests on.
func TestValueAttributionHelpersAreExact(t *testing.T) {
	literals := map[string]string{
		"'0/0'::pg_lsn":                    "0/0",
		"'[0.1,0.2,0.3]'::vector(3)":       "[0.1,0.2,0.3]",
		"32767::int2":                      "32767",
		"'it''s'::text":                    "it's",
		"'1436-4522'::issn":                "1436-4522",
		"ARRAY['[1,2,3]'::vector]":         "", // no single stored text
		"ROW(1, '16/B374D848'::pg_lsn)::t": "", // ditto
		"":                                 "",
	}
	for in, want := range literals {
		if got := literalTextOf(in); got != want {
			t.Errorf("literalTextOf(%q) = %q, want %q", in, got, want)
		}
	}

	types := map[string]string{
		"vector(3)":            "vector",
		"vector[]":             "vector",
		"numeric(10,2)":        "numeric",
		`"sweep_schema"."p_d"`: "p_d",
		"smallint":             "int2",
		"double precision":     "float8",
		"pg_lsn":               "pg_lsn",
		"ISSN":                 "issn",
	}
	for in, want := range types {
		if got := normalizeTypeName(in); got != want {
			t.Errorf("normalizeTypeName(%q) = %q, want %q", in, got, want)
		}
	}

	// A hex literal decodes; a plain one is left alone.
	if got := decodeHexLiteral(`\x302f30`); got != "0/0" {
		t.Errorf("decodeHexLiteral = %q, want 0/0", got)
	}
	if got := decodeHexLiteral("infinity"); got != "infinity" {
		t.Errorf("decodeHexLiteral mangled a plain literal: %q", got)
	}
	if got := decodeHexLiteral(`\xZZ`); got != `\xZZ` {
		t.Errorf("decodeHexLiteral on non-hex = %q, want it unchanged", got)
	}
}

// ============================================================
// DEFECT 5: a NULL-only type has no value to lose
// ============================================================

/*
TestNullOnlyColumnAbsentIsNotASilentLoss.

lsolo_IDXKEY-002.log, with both controls WORKS - so the run is sound and the column really
is missing from the stream:

	IDXKEY-002 | ghstore | LIVE | SILENT_LOSS | column "v" absent from all 3 exported
	events for this table; no exclusion warning ... [NULL-only: ...]

SILENT_LOSS is a claim that voyager lost a value. A NULL-only type has no value to lose:
PG refuses every literal for it, so the column holds NULL in every row. What IS true is
that the column never reaches the change stream and nothing warned about it - which is
what QUIET_DROP already means.
*/
func TestNullOnlyColumnAbsentIsQuietDrop(t *testing.T) {
	o := probeObservation{
		nullOnly:       true,
		eventsForTable: 3, columnSeenInEvents: false,
		snapshotCompared: true, streamCompared: true,
	}
	got, detail := decideVerdict(modeLive, o)
	if got != verdictQuietDrop {
		t.Fatalf("a NULL-only probe with a clean run read as %s, want %s (%s)",
			got, verdictQuietDrop, detail)
	}
	if !strings.Contains(detail, `column "v" absent from all 3 exported events`) {
		t.Errorf("the detail dropped the observed fact: %s", detail)
	}
	if !strings.Contains(detail, "the type stores only NULL so no value can be lost") {
		t.Errorf("the detail does not say why nothing was lost: %s", detail)
	}
	if !strings.Contains(detail, "a non-NULL value could not be tested") {
		t.Errorf("the detail does not say what was left untested: %s", detail)
	}

	// The same observation on a type that CAN hold a value is still SILENT_LOSS - this
	// carve-out must not launder a real loss.
	valued := o
	valued.nullOnly = false
	if got, d := decideVerdict(modeLive, valued); got != verdictSilentLoss {
		t.Errorf("a value-carrying type read as %s, want %s (%s)", got, verdictSilentLoss, d)
	}

	// A warned exclusion outranks it: the export side said what it did, and that is the
	// more informative answer.
	warned := o
	warned.warned, warned.promptShown = true, true
	if got, d := decideVerdict(modeLive, warned); got != verdictExcludedTold {
		t.Errorf("a confirmed exclusion read as %s, want %s (%s)", got, verdictExcludedTold, d)
	}
}

/*
FALL-BACK / FALL-FORWARD: the published cell is the REVERSE leg.

Both modes run the whole forward leg first, and both used to funnel every compare into the
same streamVerdict field, where the first verdict per phase wins. The forward compare always
got there first, so a fall-back run whose forward leg found a mismatch published that
mismatch again under FALL-BACK and discarded whatever the return path had done. In the
rerunB audit six of the seven silent FALL-BACK cells printed `source->target` with a forward
row id (1 or 2) for exactly that reason - see EVIDENCE.md, "Note on the FALL-BACK reverse
direction check".

The tests below pin the three shapes the split has to get right, plus the one thing it must
not change: LIVE is forward-only and must behave exactly as it did.
*/

// fallbackCleanObservation is a fall-back run in which everything worked: both legs
// compared, the column was seen, the reverse delta landed. Each test below breaks exactly
// one thing, so the verdict it gets is attributable to that one thing.
func fallbackCleanObservation() probeObservation {
	return probeObservation{
		snapshotCompared: true, streamCompared: true, reverseCompared: true,
		eventsForTable: 6, columnSeenInEvents: true, columnSeenOps: "insert/update/delete",
		deltaOpsApplied: 6, deltaConfirmed: true,
		srcValue: "other_cursor", dstValue: `\x6f746865725f637572736f72`,
	}
}

// TestFallbackPublishesTheReverseLeg is the whole of defect 1 in one table.
func TestFallbackPublishesTheReverseLeg(t *testing.T) {
	// (a) The forward leg mangled the value; the return path carried its own rows back
	//     intact. The fall-back cell is the reverse result, and it has to say where the
	//     forward finding went or the two cells read as the harness contradicting itself.
	t.Run("forward mismatch with a clean reverse leg is WORKS", func(t *testing.T) {
		o := fallbackCleanObservation()
		o.streamVerdict = verdictSilentWrong
		o.streamDetail = `streaming source->target: [update-this-column] id=1 ` +
			`source="other_cursor" destination="\x6f746865725f637572736f72"`

		got, detail := decideVerdict(modeFallback, o)
		if got != verdictWorks {
			t.Fatalf("decideVerdict = %s, want %s (detail: %s)", got, verdictWorks, detail)
		}
		if !strings.Contains(detail, "forward-direction mismatch is reported in the LIVE cell") {
			t.Errorf("the detail does not point at the LIVE cell: %s", detail)
		}
		if strings.Contains(detail, "source->target") {
			t.Errorf("the fall-back detail still repeats the forward compare: %s", detail)
		}
	})

	// (b) The return path is what broke. This is the finding the old code could never
	//     print, because the forward verdict was already sitting in the field.
	t.Run("reverse mismatch is the fall-back verdict", func(t *testing.T) {
		o := fallbackCleanObservation()
		o.streamVerdict = verdictSilentWrong
		o.streamDetail = `streaming source->target: [update-this-column] id=1 source="a" destination="b"`
		o.reverseVerdict = verdictSilentWrong
		o.reverseDetail = `streaming target->source: [update-this-column] id=101 ` +
			`source="24:00:00" destination="00:00:00"`

		got, detail := decideVerdict(modeFallback, o)
		if got != verdictSilentWrong {
			t.Fatalf("decideVerdict = %s, want %s (detail: %s)", got, verdictSilentWrong, detail)
		}
		if !strings.Contains(detail, "target->source") {
			t.Errorf("the detail is not a reverse-direction compare: %s", detail)
		}
		if !strings.Contains(detail, "id=101") {
			t.Errorf("the detail does not name a reverse row (101-106): %s", detail)
		}
		if strings.Contains(detail, "source->target") {
			t.Errorf("the forward compare leaked into the fall-back detail: %s", detail)
		}
		if strings.Contains(detail, "forward-direction mismatch is reported in the LIVE cell") {
			t.Errorf("a reverse FAILURE must not carry the clean-leg note: %s", detail)
		}
	})

	// (c) The forward leg failed so badly that cutover never happened, so no reverse
	//     compare ran. The forward failure IS what blocks fall-back and stays the verdict -
	//     but the detail must not imply the return path was measured.
	t.Run("a reverse leg that never ran says so", func(t *testing.T) {
		o := fallbackCleanObservation()
		o.reverseCompared = false
		o.streamVerdict = verdictSilentLoss
		o.streamDetail = `streaming source->target: [update-this-column] row id=1 present on source, missing on destination`

		got, detail := decideVerdict(modeFallback, o)
		if got != verdictSilentLoss {
			t.Fatalf("decideVerdict = %s, want %s (detail: %s)", got, verdictSilentLoss, detail)
		}
		if !strings.Contains(detail, "fall-back not reached: forward leg failed") {
			t.Errorf("the detail does not say the return path was never reached: %s", detail)
		}
		if !strings.Contains(detail, "missing on destination") {
			t.Errorf("the forward failure itself was dropped from the detail: %s", detail)
		}

		// Fall-forward says fall-forward. The two modes must not borrow each other's words.
		if _, ffDetail := decideVerdict(modeFallForward, o); !strings.Contains(
			ffDetail, "fall-forward not reached: forward leg failed") {
			t.Errorf("fall-forward detail = %s, want it to name its own leg", ffDetail)
		}
	})

	// The snapshot half of the forward leg falls under the same rule: fall-back's snapshot
	// IS the forward direction, so it belongs to the LIVE cell too.
	t.Run("a forward snapshot mismatch does not become the fall-back verdict", func(t *testing.T) {
		o := fallbackCleanObservation()
		o.snapshotVerdict = verdictSilentWrong
		o.snapshotDetail = `snapshot source->target: [update-this-column] id=1 source="1.500" destination="1.5"`

		got, detail := decideVerdict(modeFallback, o)
		if got != verdictWorks {
			t.Fatalf("decideVerdict = %s, want %s (detail: %s)", got, verdictWorks, detail)
		}
		if strings.Contains(detail, "1.500") {
			t.Errorf("the forward snapshot compare leaked into the fall-back detail: %s", detail)
		}
	})
}

// TestFallbackSplitLeavesLiveUntouched is the regression guard on the other side of the
// split: LIVE and OFFLINE have no reverse leg, and every forward observation must classify
// exactly as it did before the reverse fields existed - same verdict, same detail text.
func TestFallbackSplitLeavesLiveUntouched(t *testing.T) {
	forward := fallbackCleanObservation()
	forward.reverseCompared = false
	forward.streamVerdict = verdictSilentWrong
	forward.streamDetail = `streaming source->target: [NULL->value] id=2 source="-0" destination="0"`

	got, detail := decideVerdict(modeLive, forward)
	if got != verdictSilentWrong {
		t.Fatalf("LIVE decideVerdict = %s, want %s (%s)", got, verdictSilentWrong, detail)
	}
	if detail != forward.streamDetail {
		t.Errorf("LIVE detail = %q, want the forward detail verbatim %q", detail, forward.streamDetail)
	}

	// A LIVE observation that somehow carries reverse fields still reports the forward
	// leg: the MODE, not the presence of the fields, is what selects the leg.
	polluted := forward
	polluted.reverseCompared = true
	polluted.reverseVerdict = verdictSilentLoss
	polluted.reverseDetail = "streaming target->source: [delete] stale row id=103"
	got, detail = decideVerdict(modeLive, polluted)
	if got != verdictSilentWrong || detail != forward.streamDetail {
		t.Errorf("LIVE read the reverse fields: got %s / %q", got, detail)
	}

	// Offline has no reverse leg at all, and its snapshot verdict is unchanged.
	off := probeObservation{
		snapshotCompared: true,
		snapshotVerdict:  verdictSilentWrong,
		snapshotDetail:   `snapshot source->target: [update-this-column] id=1 source="1.500" destination="1.5"`,
	}
	if got, d := decideVerdict(modeOffline, off); got != verdictSilentWrong || d != off.snapshotDetail {
		t.Errorf("OFFLINE decideVerdict = %s / %q, want %s / the snapshot detail",
			got, d, verdictSilentWrong)
	}
}

// TestReverseCompareReadsOnlyTheReverseRowBlock: rows 1..6 are the forward delta's and are
// never replayed on the way back, so a value the forward leg mangled still sits mangled on
// the target. Comparing it against the source's untouched copy re-reports the forward
// finding with the direction label flipped - a fall-back failure the fall-back never
// caused, and the shape that would otherwise have survived defect 1's fix untouched.
func TestReverseCompareReadsOnlyTheReverseRowBlock(t *testing.T) {
	str := func(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }
	filler := str("f")

	// Target side: row 1 holds what the forward leg left there (hex), row 101 holds what
	// the reverse delta wrote. Source side: row 1 is the original, row 101 arrived clean.
	target := map[int]probeRow{
		rowBaseline: {filler: filler, value: str(`\x6f746865725f637572736f72`)},
		revBaseline: {filler: filler, value: str("reverse_value")},
	}
	source := map[int]probeRow{
		rowBaseline: {filler: filler, value: str("other_cursor")},
		revBaseline: {filler: filler, value: str("reverse_value")},
	}

	// Unscoped, this is the bug: the forward damage reads as a reverse-direction mismatch.
	if v, d := compareProbeRows(target, source); v == "" {
		t.Fatalf("the unscoped compare found nothing, so this test proves nothing")
	} else if !strings.Contains(d, "id=1 ") {
		t.Fatalf("expected the unscoped compare to trip on the forward row, got %s: %s", v, d)
	}

	// Scoped to the reverse block, the return path comes out clean.
	if v, d := compareProbeRows(reverseRowBlock(target), reverseRowBlock(source)); v != "" {
		t.Errorf("the reverse block compared as %s, want clean: %s", v, d)
	}

	// And a real reverse-direction loss is still caught.
	broken := map[int]probeRow{
		rowBaseline: {filler: filler, value: str("other_cursor")},
		revBaseline: {filler: filler, value: sql.NullString{}},
	}
	v, d := compareProbeRows(reverseRowBlock(target), reverseRowBlock(broken))
	if v != verdictSilentLoss {
		t.Errorf("a dropped reverse value compared as %s, want %s: %s", v, verdictSilentLoss, d)
	}
	if !strings.Contains(d, "id=101") {
		t.Errorf("the reverse detail does not name the reverse row: %s", d)
	}
}

// TestFallbackValuesLineCarriesTheReverseCompare: PROBE-VALUES on a FALL-BACK row has to be
// the return path's two values. The forward pair is what the LIVE row already prints, and
// printing it twice is how the fall-back cell repeated the live one even in the fields the
// audit tooling was supposed to be able to trust.
func TestFallbackValuesLineCarriesTheReverseCompare(t *testing.T) {
	o := fallbackCleanObservation()
	o.revSrcValue, o.revDstValue = "24:00:00", "00:00:00"

	if src, dst := reportedValues(modeFallback, o); src != "24:00:00" || dst != "00:00:00" {
		t.Errorf("FALL-BACK values = %q / %q, want the reverse compare's pair", src, dst)
	}
	if src, dst := reportedValues(modeFallForward, o); src != "24:00:00" || dst != "00:00:00" {
		t.Errorf("FALL-FORWARD values = %q / %q, want the reverse compare's pair", src, dst)
	}
	// LIVE and OFFLINE keep the forward pair.
	if src, dst := reportedValues(modeLive, o); src != o.srcValue || dst != o.dstValue {
		t.Errorf("LIVE values = %q / %q, want the forward pair", src, dst)
	}
	// A reverse leg that never ran leaves the forward pair as the only reading there is.
	none := fallbackCleanObservation()
	if src, dst := reportedValues(modeFallback, none); src != none.srcValue || dst != none.dstValue {
		t.Errorf("with no reverse values the line printed %q / %q, want the forward pair", src, dst)
	}
}

/*
Defect 2: the RETURN path's exclusion notice is printed on the import-data stream.

With --prepare-for-fall-back the running `import data` process exec's into
`export data from target` at cutover, so the unsupported-columns block that exporter prints
lands on the import-data stdout/stderr. recordExportWarnings reads the EXPORT command's
buffer, which after cutover belongs to `import data to source`, so it could never see it:
MISC-001 (tsquery) published FALL-BACK SILENT_LOSS with "no exclusion warning in export
stdout/stderr" over a run whose import-data stream named that exact table.
*/

// fbMiscImportStream is copied verbatim from fb_misc.log:355-359 - what voyager really
// printed on the [import data] stream of that fall-back run, log prefix included, so the
// parser is exercised on the shape it actually meets.
const fbMiscImportStream = `  [import data] The following columns data export is unsupported:
  [import data] sweep_schema.p_misc_001: [v]
  [import data] sweep_schema.p_misc_012: [v]
  [import data]
  [import data] Do you want to continue with the export by ignoring just these columns' data? [Y/N]: Continuing with the export by ignoring just these columns' data.`

// TestReverseExclusionNoticeIsReadFromTheImportStream pins the parse.
func TestReverseExclusionNoticeIsReadFromTheImportStream(t *testing.T) {
	if !exportWarnedAboutColumn(fbMiscImportStream, "sweep_schema.p_misc_001", sweepColumnUnderTest) {
		t.Errorf("the notice naming p_misc_001 was not read off the import-data stream")
	}
	if !exportWarnedAboutColumn(fbMiscImportStream, "sweep_schema.p_misc_012", sweepColumnUnderTest) {
		t.Errorf("the notice naming p_misc_012 was not read off the import-data stream")
	}
	// A table the notice did not name must not be swept up with the ones it did.
	if exportWarnedAboutColumn(fbMiscImportStream, "sweep_schema.p_ctrl_001", sweepColumnUnderTest) {
		t.Errorf("a table absent from the notice was reported as excluded")
	}
	// The forward stream of that same run named only p_misc_012 (fb_misc.log:138-140),
	// which is exactly why the two directions need flags of their own.
	const fbMiscExportStream = `  [export data] The following columns data export is unsupported:
  [export data] sweep_schema.p_misc_012: [v]
  [export data] Continuing with the export by ignoring just these columns' data.`
	if exportWarnedAboutColumn(fbMiscExportStream, "sweep_schema.p_misc_001", sweepColumnUnderTest) {
		t.Errorf("the forward notice was read as naming p_misc_001")
	}
}

// TestReverseExclusionNoticeIsNotSilent is the verdict half: the column really is absent
// from all 6 reverse-direction events, but the user WAS told. That is not SILENT_LOSS.
func TestReverseExclusionNoticeIsNotSilent(t *testing.T) {
	// MISC-001 as fb_misc.log recorded it: nothing on the forward export stream, the
	// notice on the import-data stream, 6 reverse events without the column.
	o := probeObservation{
		snapshotCompared: true, streamCompared: true, reverseCompared: true,
		eventsForTable: 6, columnSeenInEvents: false,
		deltaOpsApplied: 6, deltaConfirmed: true,
		queueScanNote: "queue scanned from the cutover mark only, so these are reverse-direction events",
	}

	// Before the reverse stream is scanned, this is the cell that was published.
	if got, d := decideVerdict(modeFallback, o); got != verdictSilentLoss {
		t.Fatalf("unscanned decideVerdict = %s, want %s (%s)", got, verdictSilentLoss, d)
	}

	// Scanned, with the notice auto-accepted by --yes and no question on the stream:
	// QUIET_DROP, the shape the fall-back cell should have carried all along.
	quiet := o
	quiet.reverseWarnScanned, quiet.reverseWarned = true, true
	got, detail := decideVerdict(modeFallback, quiet)
	if got != verdictQuietDrop {
		t.Fatalf("decideVerdict = %s, want %s (%s)", got, verdictQuietDrop, detail)
	}

	// The same run with the question actually printed is EXCLUDED_TOLD. fb_misc.log's
	// import stream DID carry it, so this is not a hypothetical branch.
	told := quiet
	told.reversePromptShown = strings.Contains(fbMiscImportStream, unsupportedColsPrompt)
	if !told.reversePromptShown {
		t.Fatalf("the captured fb_misc lines no longer contain the prompt text")
	}
	if g, d := decideVerdict(modeFallback, told); g != verdictExcludedTold {
		t.Errorf("a printed question read as %s, want %s (%s)", g, verdictExcludedTold, d)
	}

	// A reverse stream that WAS read and named nothing stays SILENT_LOSS, and names the
	// stream it looked at so the reader does not grep the one that could not carry it.
	scannedClean := o
	scannedClean.reverseWarnScanned = true
	g, d := decideVerdict(modeFallback, scannedClean)
	if g != verdictSilentLoss {
		t.Errorf("a genuinely silent reverse drop read as %s, want %s (%s)", g, verdictSilentLoss, d)
	}
	if !strings.Contains(d, "import-data stdout/stderr") {
		t.Errorf("the detail does not name the stream that was scanned: %s", d)
	}

	// The forward direction is untouched: LIVE still classifies on the forward flags.
	live := o
	live.warned = true
	if g, d := decideVerdict(modeLive, live); g != verdictQuietDrop {
		t.Errorf("LIVE with a forward notice read as %s, want %s (%s)", g, verdictQuietDrop, d)
	}
	// ... and a fall-back run that never reached cutover has no reverse stream to read,
	// so it keeps classifying on the forward flags exactly as it always did.
	noCutover := o
	noCutover.warned, noCutover.promptShown = true, true
	if g, d := decideVerdict(modeFallback, noCutover); g != verdictExcludedTold {
		t.Errorf("an unscanned fall-back read as %s, want %s (%s)", g, verdictExcludedTold, d)
	}
}
