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
