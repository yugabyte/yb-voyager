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
Container-free tests for the EXPORT-side failure detector.

The import side has had one since the crash-loop work; the export side had none, and an
exporter that dies is the most severe outcome in the whole audit - nothing migrates at all,
and `initiate cutover` then hangs forever. Read from the import log alone (which is all the
harness used to look at) it is INDISTINGUISHABLE from a quiet no-op: zero events, no
repeating importer error, no value difference to compare. So it came out INCONCLUSIVE, and
the audit's worst finding was reported as its most boring one.

These tests pin the fix and, just as importantly, its limits: a routine exporter log must
not produce the verdict, a healthy exporter with zero events must still be INCONCLUSIVE,
and an import-side crash-loop must still be an import verdict.

None of these need Docker, a database or real time.
*/

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The line an exporter actually dies on, in shape from a DOM-005 (domain over enum) run:
// the Debezium connector throws while priming its type registry, at startup, before a
// single row is read. Note where the cause sits - at the very END, behind the connector's
// whole config - which is why the evidence is narrowed rather than truncated.
const exporterNPELine = `2026-09-01 09:41:02,910 ERROR [io.deb.ser.ConnectorLifecycle] (pool-9-thread-1) ` +
	`Connector completed: success = 'false', message = 'Unable to initialize and start connector's task class ` +
	`'io.debezium.connector.postgresql.PostgresConnectorTask' with config: ` +
	`{connector.class=io.debezium.connector.postgresql.PostgresConnector, database.dbname=dtsweep_live_domains, ` +
	`database.hostname=localhost, database.port=5432, plugin.name=pgoutput, slot.name=voyager_dtsweep, ` +
	`table.include.list=sweep_schema.p_dom_005,sweep_schema.p_ctrl_001, snapshot.mode=never}', ` +
	`error = 'java.lang.NullPointerException: Cannot invoke "java.sql.Array.getArray()" because the return ` +
	`value of "java.sql.ResultSet.getArray(String)" is null'`

// The whole export side of such a run: the connector's stack trace, and the only thing
// `export data` itself ever says about it.
const exporterDeathLog = `2026-09-01 09:41:01,204 INFO  [io.deb.con.pos.PostgresConnectorTask] (pool-9-thread-1) user 'ybvoyager' connected to database 'dtsweep_live_domains'
2026-09-01 09:41:02,331 INFO  [io.deb.con.pos.con.PostgresReplicationConnection] (pool-9-thread-1) Creating replication slot with command CREATE_REPLICATION_SLOT voyager_dtsweep LOGICAL pgoutput
` + exporterNPELine + `
	at io.debezium.connector.postgresql.TypeRegistry.createTypeBuilderFromResultSet(TypeRegistry.java:371)
	at io.debezium.connector.postgresql.TypeRegistry.prime(TypeRegistry.java:334)
	at io.debezium.connector.postgresql.PostgresConnectorTask.start(PostgresConnectorTask.java:136)
Export of data failed! Check /tmp/exportdir-123/logs for more details.
`

// A perfectly ordinary exporter log. The second line deliberately NAMES a Java exception
// class in a config value: a throwable on its own must never establish a death, because
// Debezium logs exceptions it then recovers from, and quarantining a datatype on one of
// those would record a guess as a finding.
const exporterHealthyLog = `2026-09-01 09:40:58,110 INFO  [io.deb.con.pos.PostgresConnectorTask] (pool-9-thread-1) user 'ybvoyager' connected to database 'dtsweep_live_core'
2026-09-01 09:40:59,001 INFO  [io.deb.emb.EmbeddedEngine] (main) config: errors.retriable.exception = io.debezium.DebeziumException
2026-09-01 09:41:00,540 INFO  [io.deb.ser.ConnectorLifecycle] (main) Connector started: success = 'true'
2026-09-01 09:41:03,918 INFO  [io.deb.con.pos.PostgresStreamingChangeEventSource] (main) Streaming requested from LSN 0/1A2B3C4
2026-09-01 09:41:31,002 INFO  [io.deb.con.com.BaseSourceTask] (main) 6 records sent, last recorded offset: {lsn=0/1A2B444}
`

// runWaitWithLogs drives the wait loop against a fixed import log AND a fixed export log,
// with the counts frozen - the shape of a pipeline that is not moving.
func runWaitWithLogs(budget time.Duration, importLog, exportLog string) waitResult {
	clock := newFakeClock()
	return waitForSignalOrCrashLoop(budget, sweepWaitPoll, func() waitSample {
		return waitSample{satisfied: false, progress: "frozen", logText: importLog, exportText: exportLog}
	}, clock.now, clock.sleep)
}

// TestExportFailureEvidenceQuotesTheException is the evidence half: the verdict is only
// worth anything if it carries the exception that caused it, class and message intact.
func TestExportFailureEvidenceQuotesTheException(t *testing.T) {
	cause, dead := exportFailureEvidence(exporterDeathLog)
	if !dead {
		t.Fatalf("a connector that reported success = 'false' was not recognised as dead")
	}
	for _, want := range []string{
		"Connector completed: success = 'false'",
		"java.lang.NullPointerException",
		"java.sql.Array.getArray()",
	} {
		if !strings.Contains(cause, want) {
			t.Errorf("quoted cause is missing %q: %q", want, cause)
		}
	}
	// The cause must survive by NARROWING, not truncation: the connector config sits
	// between the marker and the exception and would push it past any sane limit.
	if strings.Contains(cause, "database.hostname=localhost") {
		t.Errorf("quoted cause still carries the connector config dump: %q", cause)
	}
	// The config dump also lists every captured table. Carrying it into the evidence
	// would hand attribution a list of probe names that has nothing to do with the
	// failure, and attribution would then blame one of them.
	if strings.Contains(cause, "table.include.list") {
		t.Errorf("quoted cause carries the captured-table list: %q", cause)
	}

	// The exporter's own log alone (no `Export of data failed!` line) is enough.
	if _, dead := exportFailureEvidence(exporterNPELine); !dead {
		t.Error("the ConnectorLifecycle line on its own must establish the death")
	}

	// So is voyager's own death notice - but then the evidence says only that, and does
	// not invent a cause it does not have.
	only := "Export of data failed! Check /tmp/exportdir-123/logs for more details."
	quoted, dead := exportFailureEvidence(only)
	if !dead {
		t.Fatal("`Export of data failed!` must establish the death on its own")
	}
	if strings.Contains(quoted, "Exception") {
		t.Errorf("evidence invented an exception it does not have: %q", quoted)
	}
}

// TestExportFailureEvidenceIgnoresAHealthyExporterLog is the other half, and the one that
// keeps the verdict honest: routine INFO lines - including one that names a throwable in a
// config value - must never establish a death.
func TestExportFailureEvidenceIgnoresAHealthyExporterLog(t *testing.T) {
	if cause, dead := exportFailureEvidence(exporterHealthyLog); dead {
		t.Fatalf("a healthy exporter log was read as a death: %q", cause)
	}
	if cause, dead := exportFailureEvidence(""); dead {
		t.Fatalf("an empty export log was read as a death: %q", cause)
	}
	// A throwable with no terminal marker is a Debezium retry, not a death.
	retriable := `2026-09-01 09:41:05,001 WARN [io.deb.con.com.BaseSourceTask] (main) Retriable exception thrown, ` +
		`connector will be restarted: error = 'org.postgresql.util.PSQLException: connection reset'`
	if cause, dead := exportFailureEvidence(retriable); dead {
		t.Fatalf("a retriable exception was read as a death: %q", cause)
	}
	// And a spew struct dump whose VALUE contains the marker text is a value, not a
	// message - the same trap that produced two waves of bogus STUCK verdicts.
	dump := `	ExportStatus: (string) (len=22) "Export of data failed!",`
	if cause, dead := exportFailureEvidence(dump); dead {
		t.Fatalf("a struct dump was read as a death: %q", cause)
	}
}

// TestSweepWaitTerminatesOnDeadExporter: there is no point waiting out a streaming budget
// for events from a process that is dead. Same reasoning as the crash-loop detector, and
// an even cheaper signal - a connector that reported itself completed-with-failure does
// not un-complete, so no repeat count is needed.
func TestSweepWaitTerminatesOnDeadExporter(t *testing.T) {
	budget := seconds(sweepStreamingTimeout) // the batched budget: 900 s
	res := runWaitWithLogs(budget, "", exporterDeathLog)

	if res.outcome != waitExportDied {
		t.Fatalf("outcome = %s, want %s (summary: %s)", res.outcome, waitExportDied, res.summary())
	}
	if res.polls != 1 {
		t.Errorf("concluded after %d polls, want 1: the evidence is there on the first look", res.polls)
	}
	if res.elapsed != 0 {
		t.Errorf("elapsed = %s, want 0", res.elapsed)
	}
	if res.saved() != budget {
		t.Errorf("saved = %s, want the whole %s budget", res.saved(), budget)
	}
	if !strings.Contains(res.quotedError, "NullPointerException") {
		t.Errorf("the wait concluded without quoting the exception: %q", res.quotedError)
	}
	if s := res.summary(); !strings.Contains(s, "export side is dead") || !strings.Contains(s, "900s") {
		t.Errorf("summary does not say the exporter is dead and what that saved: %q", s)
	}
}

// TestSweepWaitIgnoresAHealthyExporter: a healthy exporter that has simply produced
// nothing must still cost the full budget and end as a timeout. That is the case the long
// timeout is KEPT for, and export detection must not quietly take it over.
func TestSweepWaitIgnoresAHealthyExporter(t *testing.T) {
	budget := 20 * time.Second
	res := runWaitWithLogs(budget, "", exporterHealthyLog)
	if res.outcome != waitTimeout {
		t.Fatalf("outcome = %s, want %s (summary: %s)", res.outcome, waitTimeout, res.summary())
	}
	// And the resulting observation is INCONCLUSIVE, not a product verdict of any kind.
	verdict, detail := decideVerdict(modeLive, probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 0, waitTimedOut: true,
	})
	if verdict != verdictInconclusive {
		t.Fatalf("zero events with a healthy exporter classified %s, want %s (%s)",
			verdict, verdictInconclusive, detail)
	}
}

// TestImportCrashLoopSurvivesExportDetection is the no-regression test, in both
// directions: the export detector must not shadow a wedged importer, and the import
// detector must not fire on an exporter's Java stack trace.
func TestImportCrashLoopSurvivesExportDetection(t *testing.T) {
	// A crash-looping importer, with an ordinary exporter alongside it.
	res := runWaitWithLogs(20*time.Second, strings.Repeat(crashLoopLine+"\n", 6), exporterHealthyLog)
	if res.outcome != waitRepeatingError {
		t.Fatalf("outcome = %s, want %s: export detection shadowed the crash-loop",
			res.outcome, waitRepeatingError)
	}
	if !strings.Contains(res.quotedError, "22P02") {
		t.Errorf("quoted error is not the importer's: %q", res.quotedError)
	}

	// A dead exporter's log must not look like an import failure. The import log text
	// includes the Debezium logs, so a Java stack trace passing isImportFailureSignature
	// would manufacture a STUCK verdict out of an export-side death.
	for _, line := range strings.Split(exporterDeathLog, "\n") {
		if isImportFailureSignature(strings.TrimSpace(line)) {
			t.Errorf("an export-side line was read as an import failure: %q", line)
		}
	}
	// And the importer's own crash-loop line must not look like an export death.
	if cause, dead := exportFailureEvidence(strings.Repeat(crashLoopLine+"\n", 6)); dead {
		t.Errorf("an importer crash-loop was read as an export death: %q", cause)
	}

	// Classification: when both signals are present, the IMPORT verdict wins. A quotable
	// importer error was produced BY this value reaching the target, so the import side
	// has already measured this type; the export process dying afterwards does not erase
	// that measurement.
	both := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 4, columnSeenInEvents: true, waitTimedOut: true,
		stuckDetail:    "SQLSTATE 22P02: " + crashLoopLine + " repeated x9",
		exporterDied:   true,
		exporterDetail: "the export side died: " + exporterNPELine,
	}
	if verdict, detail := decideVerdict(modeLive, both); verdict != verdictStuck {
		t.Fatalf("a wedged importer classified %s, want %s (%s)", verdict, verdictStuck, detail)
	}
}

// TestSweepClassifierExporterDeath pins the verdict itself, and the collateral rule that
// goes with it. An exporter that dies at startup takes the WHOLE run down, so its
// batch-mates were never measured: they are INCONCLUSIVE, never WORKS and never a product
// failure verdict of their own.
func TestSweepClassifierExporterDeath(t *testing.T) {
	cause, _ := exportFailureEvidence(exporterDeathLog)

	culprit := probeObservation{
		exporterDied:   true,
		exporterDetail: "the export side died (export data failed to start: exit status 1): " + cause,
	}
	verdict, detail := decideVerdict(modeLive, culprit)
	if verdict != verdictExporterCrashes {
		t.Fatalf("the culprit classified %s, want %s (%s)", verdict, verdictExporterCrashes, detail)
	}
	if !strings.Contains(detail, "NullPointerException") {
		t.Errorf("the exporter-died detail does not quote the exception: %s", detail)
	}

	// A batch-mate. It carries everything a healthy probe carries, INCLUDING the
	// all-identical comparison that a run which never moved a single row inevitably
	// produces - which is exactly how this used to read as a clean WORKS.
	mate := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 0, deltaOpsApplied: 6, deltaConfirmed: true,
		waitTimedOut:      true,
		exporterDiedInRun: "the export side died: " + cause,
	}
	verdict, detail = decideVerdict(modeLive, mate)
	if verdict != verdictInconclusive {
		t.Fatalf("a batch-mate of an export death classified %s, want %s (%s)",
			verdict, verdictInconclusive, detail)
	}
	if !strings.Contains(detail, "died before this probe was measured") {
		t.Errorf("the collateral detail does not say why it is inconclusive: %s", detail)
	}
	// The specific wrong answers this replaces, spelled out so a future reordering of
	// decideVerdictCore cannot reintroduce one quietly. Note that the observation above
	// would classify SILENT_LOSS on its own (ops confirmed, zero events).
	for _, bad := range []string{verdictWorks, verdictSilentLoss, verdictStuck, verdictBlocks, verdictExporterCrashes} {
		if verdict == bad {
			t.Fatalf("a batch-mate of an export death must never come out %s", bad)
		}
	}

	// An export death also outranks a run-level abort: BLOCKS means "voyager stopped up
	// front with a clear error", and a connector dying on an NPE is not that.
	withAbort := probeObservation{
		runAbort:       "export data failed to start: exit status 1",
		exporterDied:   true,
		exporterDetail: cause,
	}
	if verdict, detail := decideVerdict(modeLive, withAbort); verdict != verdictExporterCrashes {
		t.Fatalf("an export death behind a run abort classified %s, want %s (%s)",
			verdict, verdictExporterCrashes, detail)
	}

	// ...and it outranks the "never reached streaming mode" flake, which is the label it
	// used to hide behind: a JVM that never came up says nothing about a type, but one
	// that came up and then threw says a great deal.
	withFlake := probeObservation{
		exportNeverStreamed: true,
		flakeDetail:         "export data never reached streaming mode within 8m0s",
		exporterDied:        true,
		exporterDetail:      cause,
	}
	if verdict, detail := decideVerdict(modeLive, withFlake); verdict != verdictExporterCrashes {
		t.Fatalf("an export death behind the streaming-mode flake classified %s, want %s (%s)",
			verdict, verdictExporterCrashes, detail)
	}
}

// TestExportFailureAttribution: an export death is pinned on a probe only when the failure
// names EXACTLY ONE of them. Otherwise it is reported against the RUN and every probe in
// it is inconclusive - a guess here would pin the harshest verdict in the vocabulary on an
// innocent type.
func TestExportFailureAttribution(t *testing.T) {
	probes := []datatypeProbe{
		{ID: "CTRL-001", TypeName: "int", ExpectVerdict: verdictWorks},
		{ID: "DOM-005", TypeName: "domain(enum)"},
		{ID: "HSTORE-001", TypeName: "hstore"},
	}
	// What the runner actually hands to attribution: the NARROWED evidence, never the
	// raw log. It matters here - the raw ConnectorLifecycle line carries the connector's
	// table.include.list, which names every probe in the batch, so attributing over it
	// would blame whichever probe happened to be listed alone.
	realEvidence, _ := exportFailureEvidence(exporterNPELine)

	cases := []struct {
		name string
		text string
		want string // "" means no attribution
	}{
		{
			name: "names one probe table",
			text: `Connector completed: success = 'false' - io.debezium.DebeziumException: no converter for column sweep_schema.p_dom_005.v`,
			want: "DOM-005",
		},
		{
			name: "names one probe type",
			text: `Connector completed: success = 'false' - io.debezium.DebeziumException: unsupported type hstore`,
			want: "HSTORE-001",
		},
		{
			name: "names two probes - ambiguous, no culprit",
			text: `Connector completed: success = 'false' - failed on p_dom_005 and p_hstore_001`,
		},
		{
			// The real DOM-005 shape: the NullPointerException names no table and no
			// type at all, so the finding belongs to the RUN and every probe in it is
			// inconclusive.
			name: "the real NullPointerException names nothing",
			text: realEvidence,
		},
		{
			name: "names only a control - never blame the control",
			text: `Connector completed: success = 'false' - failed reading sweep_schema.p_ctrl_001`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := attributeExportFailure(probes, tc.text)
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

// TestExportAndImportFailuresStayDistinguishable is the vocabulary invariant. Import
// failure means a value could not be APPLIED; export failure means nothing was ever
// PRODUCED. They are different findings about different halves of the pipeline, and a
// release diff showing a type move between them is showing something real.
func TestExportAndImportFailuresStayDistinguishable(t *testing.T) {
	wedged := probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 4, columnSeenInEvents: true, waitTimedOut: true,
		stuckDetail: "SQLSTATE 22P02: " + crashLoopLine + " repeated x9",
	}
	dead := probeObservation{exporterDied: true, exporterDetail: exporterNPELine}

	vw, _ := decideVerdict(modeLive, wedged)
	vd, _ := decideVerdict(modeLive, dead)
	if vw == vd {
		t.Fatalf("a wedged importer and a dead exporter both reported %s", vw)
	}
	if vw != verdictStuck || vd != verdictExporterCrashes {
		t.Fatalf("got %s / %s, want %s / %s", vw, vd, verdictStuck, verdictExporterCrashes)
	}

	// The same distinction in the wait's own vocabulary: three endings, three labels.
	crash := runWaitWithLogs(20*time.Second, strings.Repeat(crashLoopLine+"\n", 6), "")
	died := runWaitWithLogs(20*time.Second, "", exporterDeathLog)
	quiet := runWaitWithLogs(20*time.Second, "nothing to see here\n", exporterHealthyLog)
	outcomes := map[waitOutcome]bool{crash.outcome: true, died.outcome: true, quiet.outcome: true}
	if len(outcomes) != 3 {
		t.Fatalf("the three wait endings collapsed into %d labels: %s / %s / %s",
			len(outcomes), crash.outcome, died.outcome, quiet.outcome)
	}
}

// ============================================================
// PUBLISHING AN ATTRIBUTED EXPORT DEATH
// ============================================================

// TestExportDeathVerdictIsPublishable pins the one carve-out from the control gate.
//
// The gate exists to catch a BROKEN MEASUREMENT. An attributed export death is not one: it
// IS the finding, and the controls going inconclusive is a consequence of it (the exporter
// they needed was dead) rather than evidence against it. So that single row is publishable
// while every other row from the same run is not.
//
// Every clause of the carve-out is tested, because each one is what stops it from becoming
// a way to launder an unproven verdict past the gate.
func TestExportDeathVerdictIsPublishable(t *testing.T) {
	cause, _ := exportFailureEvidence(exporterDeathLog)

	cases := []struct {
		name             string
		probeID, verdict string
		culprit, cause   string
		wantPublishable  bool
	}{
		{
			name: "the attributed culprit is publishable", probeID: "DOM-005",
			verdict: verdictExporterCrashes, culprit: "DOM-005", cause: cause,
			wantPublishable: true,
		},
		{
			// It was never measured: publishing its INCONCLUSIVE would say nothing, and
			// publishing anything else would be a claim about an unexercised type.
			name: "a batch-mate is not", probeID: "CORE-001",
			verdict: verdictInconclusive, culprit: "DOM-005", cause: cause,
		},
		{
			// The marker must never promote a verdict the classifier did not produce.
			name: "the culprit with some other verdict is not", probeID: "DOM-005",
			verdict: verdictSilentLoss, culprit: "DOM-005", cause: cause,
		},
		{
			// The real DOM-005 NullPointerException names neither table nor type. The
			// run-level marker is the record, and a human writes that row from it.
			name: "an unattributed death promotes nothing", probeID: "DOM-005",
			verdict: verdictExporterCrashes, culprit: "", cause: cause,
		},
		{
			// The same rule that governs the verdict itself: no evidence, no claim.
			name: "no quotable cause, no publication", probeID: "DOM-005",
			verdict: verdictExporterCrashes, culprit: "DOM-005", cause: "   ",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			why := publishableReason(tc.probeID, tc.verdict, tc.culprit, tc.cause)
			if tc.wantPublishable {
				if why == "" {
					t.Fatal("verdict is not publishable, want publishable")
				}
				if !strings.Contains(why, "NullPointerException") {
					t.Errorf("the publishable reason does not carry the cause: %q", why)
				}
				return
			}
			if why != "" {
				t.Fatalf("verdict is publishable (%q), want not publishable", why)
			}
		})
	}
}

// ============================================================
// THE POLL ITSELF MUST BE BOUNDED
// ============================================================

// TestBoundedFetcherSurvivesAWedgedCommand is the fix for a hang the wait loop could not
// see: `get data-migration-report` blocked in pipe I/O takes the whole poll with it, so
// there is no polling, no crash-loop detection, no export-death detection and - because
// PROBE-WAIT is only printed when a wait ENDS - no output at all. A catalogstats live batch
// spent 13 minutes exactly that way.
func TestBoundedFetcherSurvivesAWedgedCommand(t *testing.T) {
	// A fetch that never returns, the way Cmd.Wait() never returns while a grandchild
	// still holds the command's stdout pipe.
	block := make(chan struct{})
	defer close(block)
	// Atomic because the counter is written by the (deliberately stuck) fetch goroutine
	// and read by the test while it is still stuck - that is the whole point of it.
	var calls atomic.Int64
	b := &boundedFetcher{fetch: func() (*DataMigrationReport, error) {
		calls.Add(1)
		<-block
		return nil, nil
	}}

	// The first poll pays the budget once, then gives up.
	start := time.Now()
	report, stalled, err := b.get(30 * time.Millisecond)
	if !stalled || report != nil || err != nil {
		t.Fatalf("first poll = (%v, stalled=%v, %v), want (nil, true, nil)", report, stalled, err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("first poll blocked for %s, want about the 30ms budget", elapsed)
	}

	// Every later poll checks that SAME outstanding fetch without blocking, so the loop
	// keeps its cadence instead of paying the budget again on every poll...
	for i := 0; i < 5; i++ {
		start = time.Now()
		if _, stalled, _ := b.get(30 * time.Second); !stalled {
			t.Fatalf("poll %d reported the fetch as healthy while it is still wedged", i+2)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Fatalf("poll %d blocked for %s despite an already-stalled fetch", i+2, elapsed)
		}
	}

	// ...and exactly one goroutine is leaked, however many polls happen.
	if n := calls.Load(); n != 1 {
		t.Errorf("started %d fetches, want exactly 1 while the first is outstanding", n)
	}
	if b.stalls != 1 {
		t.Errorf("recorded %d stalls, want 1", b.stalls)
	}
	if b.drain(20 * time.Millisecond) {
		t.Error("drain reported success while the fetch is still wedged")
	}
}

// TestBoundedFetcherRecoversWhenTheCommandReturns: the bound must not be one-way. A slow
// fetch that eventually completes has to put the fetcher straight back to normal, or a
// single slow poll would blind the loop to the counts for the rest of the run.
func TestBoundedFetcherRecoversWhenTheCommandReturns(t *testing.T) {
	release := make(chan struct{})
	want := &DataMigrationReport{}
	b := &boundedFetcher{fetch: func() (*DataMigrationReport, error) {
		<-release
		return want, nil
	}}

	if _, stalled, _ := b.get(20 * time.Millisecond); !stalled {
		t.Fatal("a fetch that had not returned was reported healthy")
	}
	close(release)

	// Poll until the outstanding fetch is picked up - non-blocking, as the loop does.
	var got *DataMigrationReport
	for i := 0; i < 200; i++ {
		rep, stalled, err := b.get(0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !stalled {
			got = rep
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got != want {
		t.Fatalf("the completed fetch was never picked up (got %v)", got)
	}
	if b.stalled || b.pending != nil {
		t.Errorf("fetcher did not reset after the fetch completed (stalled=%v)", b.stalled)
	}
	// And a healthy fetch afterwards is served normally.
	if rep, stalled, err := b.get(time.Second); stalled || err != nil || rep != want {
		t.Errorf("next fetch = (%v, stalled=%v, %v), want the report", rep, stalled, err)
	}
}

// TestSweepWaitConcludesOnTotalSilence: a pipeline that writes NOTHING - no counts, no
// import log, no export log - for the grace period is concluded rather than sat out. This
// is the "no output for N minutes" case, and it stays a separate label from a budget that
// simply ran out so the run log can say whether the budget mattered.
func TestSweepWaitConcludesOnTotalSilence(t *testing.T) {
	budget := seconds(sweepStreamingTimeout) // 900 s, comfortably longer than the grace
	res := runWaitWithLogs(budget, "same log, forever\n", exporterHealthyLog)

	if res.outcome != waitNoOutput {
		t.Fatalf("outcome = %s, want %s (summary: %s)", res.outcome, waitNoOutput, res.summary())
	}
	if res.silence < sweepSilenceGrace {
		t.Errorf("concluded after %s of silence, want at least the %s grace", res.silence, sweepSilenceGrace)
	}
	if res.elapsed >= budget {
		t.Errorf("elapsed %s, want well short of the %s budget", res.elapsed, budget)
	}
	if res.saved() <= 0 {
		t.Error("a silent-pipeline conclusion saved nothing")
	}
	if s := res.summary(); !strings.Contains(s, "logged nothing") {
		t.Errorf("summary does not say the pipeline logged nothing: %q", s)
	}
	// It is an environment fact, never a datatype verdict.
	verdict, detail := decideVerdict(modeLive, probeObservation{
		snapshotCompared: true, streamCompared: true,
		eventsForTable: 0, waitTimedOut: true,
		waitNote: "forward streaming wait concluded early after 300s of total silence",
	})
	if verdict != verdictInconclusive {
		t.Fatalf("a silent pipeline classified %s, want %s (%s)", verdict, verdictInconclusive, detail)
	}
}

// TestSweepWaitSilenceDoesNotFireOnABusyPipeline is the other half. A crash-looping
// importer is NOISY - it rewrites the same error every few seconds - but the log is only
// ever read as a 512 KiB tail, so once it passes that size it stops changing LENGTH while
// its content still changes. Measure activity by length alone and the busiest possible
// pipeline reads as silent.
func TestSweepWaitSilenceDoesNotFireOnABusyPipeline(t *testing.T) {
	clock := newFakeClock()
	poll := 0
	res := waitForSignalOrCrashLoop(seconds(sweepStreamingTimeout), sweepWaitPoll, func() waitSample {
		poll++
		// Same length every time, different bytes every time: a rolling tail.
		return waitSample{
			progress: "frozen",
			logText:  fmt.Sprintf("%08d writing away\n", poll) + strings.Repeat("x", 100),
		}
	}, clock.now, clock.sleep)

	if res.outcome == waitNoOutput {
		t.Fatalf("a log that changes content at constant length was read as silence after %s",
			res.silence)
	}
	if res.outcome != waitTimeout {
		t.Fatalf("outcome = %s, want %s", res.outcome, waitTimeout)
	}

	// And the signature itself: same length, different bytes, must differ.
	a := activitySignature(waitSample{progress: "p", logText: "aaaa"})
	b := activitySignature(waitSample{progress: "p", logText: "bbbb"})
	if a == b {
		t.Error("activitySignature cannot tell two same-length logs apart")
	}
	// A changing export log is activity too, even with the import side frozen.
	c := activitySignature(waitSample{progress: "p", logText: "aaaa", exportText: "x"})
	if a == c {
		t.Error("activitySignature ignores the export log")
	}
}
