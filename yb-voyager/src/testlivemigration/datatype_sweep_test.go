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
	"os"
	"strings"
	"testing"
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
