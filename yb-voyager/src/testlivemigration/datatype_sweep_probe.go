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
Datatype sweep harness.

One `datatypeProbe` == one row of the final audit matrix. A probe owns exactly one table
with the shape

	(id int PRIMARY KEY, filler text, v <the type under test>)

so that a "touch a *different* column" UPDATE (filler) can be told apart from a "touch the
column under test" UPDATE (v). That distinction is the whole point: a column that never
appears in the exported event stream only shows up when you update its neighbour.

The runner builds one schema holding one table per probe, runs the requested migration
mode, then prints exactly ONE machine-readable line per probe:

	PROBE-RESULT: <id> | <type> | <mode> | <verdict> | <detail>

Verdict vocabulary is fixed by PROBE_SPEC.md, plus two labels that are harness /
environment facts rather than product verdicts: SKIPPED (the server refused the probe's
DDL or the extension is missing) and INCONCLUSIVE (the run never actually exercised the
probe, so no claim can be made either way).

Every wait in here is bounded. A wait that expires never aborts the run: it is recorded
and fed into classification, because "the counts never arrived and the import log keeps
repeating one error" IS the STUCK verdict, not a test timeout.
*/

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	goerrors "github.com/go-errors/errors"

	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// ============================================================
// VERDICTS, MODES, OPS
// ============================================================

// Verdict labels. The first seven are PROBE_SPEC.md's vocabulary, worst to best.
// SKIPPED is not a product verdict: it means the probe could not be set up at all
// (extension missing, or the server rejected the probe's DDL), so no claim is made.
const (
	verdictSilentLoss   = "SILENT_LOSS"
	verdictSilentWrong  = "SILENT_WRONG"
	verdictQuietDrop    = "QUIET_DROP"
	verdictStuck        = "STUCK"
	verdictBlocks       = "BLOCKS"
	verdictExcludedTold = "EXCLUDED_TOLD"
	verdictWorks        = "WORKS"
	verdictSkipped      = "SKIPPED"

	// verdictExporterCrashes is STUCK's export-side twin, and the two must never be
	// collapsed into one label. STUCK means the importer could not APPLY a value: the
	// value was produced, exported and delivered, and the target refused it.
	// EXPORTER_CRASHES means nothing was ever PRODUCED - `export data` (or the Debezium
	// connector inside it) died, usually at startup and before a single row was read, so
	// there is no event stream at all and `initiate cutover` then waits forever.
	//
	// It is at least as severe as STUCK (a wedged channel still delivers everything
	// ahead of the poison value; a dead exporter delivers nothing), and it is a PRODUCT
	// verdict: it says a datatype killed voyager's exporter. That is precisely why it
	// must not be reported without a quotable cause - see exportFailureEvidence.
	verdictExporterCrashes = "EXPORTER_CRASHES"

	// verdictInconclusive is not a product verdict either: it means the run did not
	// actually exercise the probe, so neither a pass nor a failure can be claimed.
	// It exists because an empty observation must never read as a clean WORKS - a
	// framework wait that t.Fatalf's (runtime.Goexit) skips every measurement step
	// while the deferred emitAll still runs.
	verdictInconclusive = "INCONCLUSIVE"
)

type sweepMode string

const (
	modeOffline     sweepMode = "OFFLINE"
	modeLive        sweepMode = "LIVE"
	modeFallback    sweepMode = "FALL-BACK"
	modeFallForward sweepMode = "FALL-FORWARD"
)

// hasCDC reports whether the mode streams change events at all (offline is snapshot-only,
// so there is no queue to inspect and the datatype filter does not run).
func (m sweepMode) hasCDC() bool { return m != modeOffline }

// slug is used to build per-batch database names.
func (m sweepMode) slug() string {
	return strings.ToLower(strings.ReplaceAll(string(m), "-", ""))
}

// deltaOp enumerates the change operations a probe can exercise. These are deliberately
// distinct because each one classifies a different failure mode:
//
//	opInsertRow     - a brand new row carrying a value of the type
//	opUpdateSelf    - UPDATE of the column under test
//	opUpdateOther   - UPDATE of a DIFFERENT column, leaving this one untouched.
//	                  This is what exposes columns missing from the event stream.
//	opDeleteRow     - DELETE of a row
//	opNullToValue   - NULL -> value transition
//	opValueToNull   - value -> NULL transition
type deltaOp string

const (
	opInsertRow   deltaOp = "insert-new-row"
	opUpdateSelf  deltaOp = "update-this-column"
	opUpdateOther deltaOp = "update-other-column"
	opDeleteRow   deltaOp = "delete-row"
	opNullToValue deltaOp = "null-to-value"
	opValueToNull deltaOp = "value-to-null"
)

// allDeltaOps is the default op set: the full assertion list from PROBE_SPEC.md §2.
var allDeltaOps = []deltaOp{opInsertRow, opUpdateSelf, opUpdateOther, opDeleteRow, opNullToValue, opValueToNull}

// Row id layout. Rows 1..5 are the targets of the forward (source-side) delta; rows
// 101..105 are the targets of the reverse (target-side) delta used by FALL-BACK and
// FALL-FORWARD. Keeping the two blocks disjoint means the reverse delta never has to
// guess what the forward delta left behind, so change counts stay deterministic.
const (
	rowBaseline   = 1 // holds InitialValue; opUpdateSelf rewrites it to AltValue
	rowNullSeed   = 2 // starts NULL; opNullToValue fills it
	rowToDelete   = 3 // holds AltValue; opDeleteRow removes it
	rowOtherCol   = 4 // holds InitialValue; opUpdateOther touches only `filler`
	rowToNull     = 5 // holds InitialValue; opValueToNull clears it
	rowNewInsert  = 6 // does not exist initially; opInsertRow creates it
	revBaseline   = 101
	revNullSeed   = 102
	revToDelete   = 103
	revOtherCol   = 104
	revToNull     = 105
	revNewInsert  = 106
	sweepRowCount = 10 // rows created by the initial-data INSERT (5 forward + 5 reverse)
)

// Timeouts, in SECONDS. The framework's wait helpers take a time.Duration but
// internally multiply by time.Second (see utils.RetryWorkWithTimeout), so a bare
// count of seconds is what they actually want.
const (
	sweepSnapshotTimeout  time.Duration = 900
	sweepStreamingTimeout time.Duration = 900
	// A solo run has exactly one probe under test, so a genuine stall is obvious fast:
	// a real crash-loop repeats its error every few seconds, and an importer that dies
	// does so immediately. The long batch wait only exists to absorb slow-but-healthy
	// streaming across many tables, which a solo run does not have.
	sweepSoloStreamingTimeout time.Duration = 240
	sweepCutoverTimeout       time.Duration = 300
	sweepFallForwardWait      time.Duration = 300
	sweepExportStartWait      time.Duration = 300
)

// sweepStreamingModeWait is a REAL duration, unlike the constants above:
// LiveMigrationTest.WaitForStreamingMode sleeps and compares against time.Now directly
// instead of going through utils.RetryWorkWithTimeout.
const (
	sweepStreamingModeWait time.Duration = 8 * time.Minute
	sweepStreamingModePoll time.Duration = 2 * time.Second
)

// Crash-loop detection. The bounded waits above are POSITIVE-signal waits: they watch for
// the expected counts to arrive and can only conclude by running out of clock. That makes
// the suite fast when everything works and slowest exactly when it finds something, which
// is backwards for a tool whose job is finding failures.
//
// A wedged importer does not go quiet, though: it retries the same batch and logs the SAME
// error every few seconds. So the negative signal is a REPEATING error, and it is readable
// in seconds rather than in the 240-900 s the positive signal is given. The wait therefore
// also polls the import log, and concludes early when
//
//	the same import-failure signature has been logged sweepCrashLoopRepeats times, AND
//	that signature was still the most repeated one sweepCrashLoopPolls polls running, AND
//	the observed counts did not advance across those polls.
//
// All three are required. The repeat count alone would fire on a transient error that the
// importer then got past; the frozen counts alone are just a slow pipeline; and the
// signature test (isImportFailureSignature) is what keeps a spew dump or a config field
// out of it. The long timeout is kept for the genuinely different case of a stall that
// logs NOTHING at all - which classifies as INCONCLUSIVE, never STUCK.
const (
	sweepCrashLoopRepeats = 3
	sweepCrashLoopPolls   = 2
	// sweepWaitPoll is a REAL duration: the sweep drives its own wait loop rather than
	// handing a seconds-count to utils.RetryWorkWithTimeout.
	sweepWaitPoll time.Duration = 2 * time.Second

	// sweepSyncCommandTimeout bounds a command the fixture starts SYNCHRONOUSLY - the
	// offline flow's export and import. Seconds, like the other budgets in this block.
	// See SetSyncCommandTimeout: a synchronous start blocks on the output pipes as well
	// as on the process, and offline has no bounded wait of its own to notice.
	sweepSyncCommandTimeout time.Duration = 900

	// sweepSilenceGrace is the third negative signal: NOTHING moved anywhere - not the
	// migration-report counts, not the import log, not the export log - for this long.
	// A healthy pipeline is never silent for five minutes; both voyager processes log
	// continuously, and the importer runs at --log-level debug.
	//
	// This is the "no output for N minutes" case, and it is CONCLUDED rather than sat
	// out. It stays a separate outcome from waitTimeout on purpose: "silent for 5 of
	// the 15 minutes I was willing to give it" and "silent for all 15" are the same
	// finding, but only the first one says so while there is still budget left, and a
	// run log that cannot tell them apart cannot say whether the budget mattered.
	//
	// Like waitTimeout it is an ENVIRONMENT fact and classifies INCONCLUSIVE. Nothing
	// about a datatype can be concluded from a pipeline that wrote nothing down.
	sweepSilenceGrace time.Duration = 5 * time.Minute

	// sweepReportFetchBudget bounds ONE poll's `get data-migration-report`.
	//
	// Every poll shells out to that command, and VoyagerCommandRunner gives the child an
	// io.MultiWriter for stdout/stderr - which makes exec create an OS pipe and makes
	// Cmd.Wait() block until the pipe reaches EOF. EOF needs EVERY descendant that
	// inherited the fd to exit, so one wedged grandchild blocks the reader forever, in
	// `goroutine [IO wait]`, with the voyager process itself already dead. The framework
	// already documents this hazard on WaitForAsyncCompletion.
	//
	// A blocked fetch used to take the whole wait loop down with it: no poll, no
	// crash-loop detection, no export-death detection and - because PROBE-WAIT is only
	// printed when a wait ENDS - not one line of output saying so. Bounding it turns
	// that into a report this poll could not read, which the loop already handles.
	sweepReportFetchBudget time.Duration = 2 * time.Minute
	// sweepFetchDrainBudget is how long the run waits, at test cleanup, for a fetch that
	// was still outstanding. It runs from t.Cleanup - i.e. before the test is marked
	// complete - so a late goroutine can still log without panicking.
	sweepFetchDrainBudget time.Duration = 30 * time.Second
)

// seconds converts one of the seconds-as-Duration constants above into a real duration.
// The framework's wait helpers multiply by time.Second internally; the sweep's own loop
// does not, so the conversion has to happen somewhere and doing it at the call site keeps
// the constants readable next to the helpers that still consume them raw.
func seconds(n time.Duration) time.Duration { return n * time.Second }

// ============================================================
// THE PROBE
// ============================================================

// datatypeProbe describes one audit-matrix row.
//
// Templating: ColumnDDL, PreDDL, InitialValue, AltValue and CompareExpr are all run
// through expandTemplate, which substitutes
//
//	{{schema}} -> the sweep schema name
//	{{p}}      -> this probe's table name (unique per probe)
//
// Use {{p}} to name any type/domain the probe creates, so that two probes needing a
// similar shape never collide and no probe has to CASCADE-drop another's table.
type datatypeProbe struct {
	// ID is the stable audit id, e.g. "RANGE-001". It also derives the table name.
	ID string
	// Name is a short human label.
	Name string
	// TypeName is what lands in the <type> field of the PROBE-RESULT line.
	TypeName string

	// PreDDL runs before the probe table is created (CREATE TYPE / DOMAIN / ...).
	PreDDL []string
	// ColumnDDL is the type expression for the column under test, e.g. "int4range".
	ColumnDDL string
	// Extensions must all be installable on every participating database, or the
	// probe self-reports SKIPPED instead of failing the batch.
	Extensions []string

	// InitialValue and AltValue are SQL literal expressions of the type under test.
	// InitialValue seeds the snapshot; AltValue is what opUpdateSelf / opInsertRow write.
	InitialValue string
	AltValue     string

	// Ops is the delta set. Empty means allDeltaOps.
	Ops []deltaOp

	// CompareExpr is the expression used to compare source and destination values.
	// Empty means "v::text", which is the type's own output function and therefore
	// byte-exact for almost everything. Override when the text form is not stable
	// across servers (e.g. money, which is lc_monetary dependent).
	CompareExpr string

	// Poison marks a probe expected to crash-loop the channel. The runner REFUSES to
	// put a poison probe in a batch; run it alone via TestDatatypeSweepSuspect.
	Poison bool

	// PoisonNote records WHY a probe is known poison - the mode it was established in
	// and the error that established it - so an exclusion line explains itself instead
	// of just asserting.
	PoisonNote string

	// ExpectExcluded records that voyager's unsupported-datatype guardrail is
	// expected to drop this column. Reporting only; it does not gate the verdict.
	ExpectExcluded bool

	// RecordDestValue makes the PROBE-RESULT detail carry the actual destination text
	// of the value under test, even when the probe comes out WORKS. Use it where the
	// exact bytes are the finding rather than just pass/fail - e.g. whether a
	// non-comma array delimiter survived the round-trip.
	RecordDestValue bool

	// NullOnly marks a type whose column can be created and whose rows can be migrated,
	// but for which no literal is storable at all - PostgreSQL's input function for the
	// type raises "cannot accept a value of type <t>". Only NULL can ever be written, so
	// InitialValue and AltValue are both NULL and the update op cannot change the value.
	// That is still a real migration path: the type has to exist on the target, the
	// column has to survive the snapshot, and the rows have to travel through CDC.
	//
	// A probe is only allowed to claim this after the literal has actually been refused
	// at run time, and PoisonNote/Note must quote the error. It exists so the
	// InitialValue != AltValue guard, which is there to stop a silently useless update
	// op, does not force these types out of the audit entirely.
	NullOnly bool

	// ---- Reporting-layer OVERRIDES ------------------------------------------------
	//
	// The published report's per-type columns are DERIVED, not typed in: see
	// datatype_report_meta.go, which computes them from voyager's own variables
	// (srcdb.PostgresUnsupportedDataTypes and friends, plus the runtime typtype='r'
	// filter) so that editing one of those lists changes the report on the next run.
	//
	// These four exist only for the cases where the derivation cannot be right, because
	// it works from a type NAME and some behaviour is not name-driven. EMPTY MEANS
	// DERIVE, which is the correct value for almost every probe. Setting one pins that
	// cell to a hand-written string, so it can go stale exactly like the hand-maintained
	// table this suite replaced - only do it when the derivation is demonstrably wrong,
	// and say why in Note.
	ReportedByAssess  string
	ReportedByAnalyze string
	GuardrailAction   string
	ReportedByDocs    string

	// ExpectVerdict, when set, is enforced after the run as a harness sanity check
	// (PROBE_SPEC.md §"Known-answer checks"). Used by the int/text controls.
	ExpectVerdict string

	// Note is free-form context carried into the report.
	Note string
}

// tableName derives a stable, unique, lower-case identifier from the probe id.
func (p datatypeProbe) tableName() string {
	s := strings.ToLower(p.ID)
	s = strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(s)
	return "p_" + s
}

func (p datatypeProbe) ops() []deltaOp {
	if len(p.Ops) == 0 {
		return allDeltaOps
	}
	return p.Ops
}

func (p datatypeProbe) hasOp(op deltaOp) bool {
	for _, o := range p.ops() {
		if o == op {
			return true
		}
	}
	return false
}

func (p datatypeProbe) compareExpr() string {
	if strings.TrimSpace(p.CompareExpr) == "" {
		return "v::text"
	}
	return p.CompareExpr
}

// expandTemplate substitutes the {{schema}} and {{p}} placeholders.
func (p datatypeProbe) expandTemplate(s, schema string) string {
	return strings.NewReplacer("{{schema}}", schema, "{{p}}", p.tableName()).Replace(s)
}

func (p datatypeProbe) qualifiedTable(schema string) string {
	return fmt.Sprintf("%s.%s", schema, p.tableName())
}

// reportKey is the `"schema"."table"` form used as a key in the data-migration report.
func (p datatypeProbe) reportKey(schema string) string {
	return fmt.Sprintf(`"%s"."%s"`, schema, p.tableName())
}

// ddl returns the full DDL for the probe: its supporting types, its table, nothing else.
func (p datatypeProbe) ddl(schema string) []string {
	stmts := make([]string, 0, len(p.PreDDL)+1)
	for _, d := range p.PreDDL {
		stmts = append(stmts, p.expandTemplate(d, schema))
	}
	stmts = append(stmts, fmt.Sprintf(
		"CREATE TABLE %s (id int PRIMARY KEY, filler text, v %s)",
		p.qualifiedTable(schema), p.expandTemplate(p.ColumnDDL, schema)))
	return stmts
}

// replicaIdentitySQL is applied to the SOURCE only. The YugabyteDB target must keep its
// default REPLICA IDENTITY CHANGE, otherwise voyager's replica-identity guardrail on
// export-from-target rejects the table (see export_data_from_target_failures_test.go).
func (p datatypeProbe) replicaIdentitySQL(schema string) string {
	return fmt.Sprintf("ALTER TABLE %s REPLICA IDENTITY FULL", p.qualifiedTable(schema))
}

// initialDataSQL seeds both the forward-delta rows (1..5) and the reverse-delta rows
// (101..105) in one statement.
func (p datatypeProbe) initialDataSQL(schema string) string {
	init := p.expandTemplate(p.InitialValue, schema)
	alt := p.expandTemplate(p.AltValue, schema)
	tbl := p.qualifiedTable(schema)
	row := func(id int, val string) string {
		return fmt.Sprintf("(%d, 'filler-%d', %s)", id, id, val)
	}
	values := []string{
		row(rowBaseline, init),
		row(rowNullSeed, "NULL"),
		row(rowToDelete, alt),
		row(rowOtherCol, init),
		row(rowToNull, init),
		row(revBaseline, init),
		row(revNullSeed, "NULL"),
		row(revToDelete, alt),
		row(revOtherCol, init),
		row(revToNull, init),
	}
	return fmt.Sprintf("INSERT INTO %s (id, filler, v) VALUES %s", tbl, strings.Join(values, ", "))
}

// deltaSQL builds the delta statements for one direction. `reverse` selects the
// 101..106 row block (used for target-side deltas in FALL-BACK / FALL-FORWARD).
// dropDDL undoes ddl(): the table first, then each PreDDL-created type or domain in
// reverse order. Every PreDDL in the case tables has the uniform shape
// `CREATE <TYPE|DOMAIN> <qualified-name> ...`, so the object to drop is token 2.
func (p datatypeProbe) dropDDL(schema string) []string {
	out := []string{fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", p.qualifiedTable(schema))}
	for i := len(p.PreDDL) - 1; i >= 0; i-- {
		fields := strings.Fields(p.expandTemplate(p.PreDDL[i], schema))
		if len(fields) < 3 || !strings.EqualFold(fields[0], "CREATE") {
			continue
		}
		kind := strings.ToUpper(fields[1])
		if kind != "TYPE" && kind != "DOMAIN" {
			continue
		}
		out = append(out, fmt.Sprintf("DROP %s IF EXISTS %s CASCADE", kind, fields[2]))
	}
	return out
}

func (p datatypeProbe) deltaSQL(schema string, reverse bool) []string {
	init := p.expandTemplate(p.InitialValue, schema)
	alt := p.expandTemplate(p.AltValue, schema)
	tbl := p.qualifiedTable(schema)

	baseline, nullSeed, toDelete, otherCol, toNull, newInsert :=
		rowBaseline, rowNullSeed, rowToDelete, rowOtherCol, rowToNull, rowNewInsert
	if reverse {
		baseline, nullSeed, toDelete, otherCol, toNull, newInsert =
			revBaseline, revNullSeed, revToDelete, revOtherCol, revToNull, revNewInsert
	}

	var stmts []string
	for _, op := range p.ops() {
		switch op {
		case opInsertRow:
			stmts = append(stmts, fmt.Sprintf(
				"INSERT INTO %s (id, filler, v) VALUES (%d, 'filler-%d', %s)", tbl, newInsert, newInsert, alt))
		case opUpdateSelf:
			stmts = append(stmts, fmt.Sprintf("UPDATE %s SET v = %s WHERE id = %d", tbl, alt, baseline))
		case opUpdateOther:
			// Deliberately does NOT touch v. If v is missing from the event stream,
			// the destination row keeps a stale/NULL v after this UPDATE is applied.
			stmts = append(stmts, fmt.Sprintf(
				"UPDATE %s SET filler = 'filler-%d-touched' WHERE id = %d", tbl, otherCol, otherCol))
		case opDeleteRow:
			stmts = append(stmts, fmt.Sprintf("DELETE FROM %s WHERE id = %d", tbl, toDelete))
		case opNullToValue:
			stmts = append(stmts, fmt.Sprintf("UPDATE %s SET v = %s WHERE id = %d", tbl, init, nullSeed))
		case opValueToNull:
			stmts = append(stmts, fmt.Sprintf("UPDATE %s SET v = NULL WHERE id = %d", tbl, toNull))
		}
	}
	return stmts
}

// expectedChanges is the ChangesCount the data-migration report should reach for one
// direction of deltas.
func (p datatypeProbe) expectedChanges() ChangesCount {
	var c ChangesCount
	for _, op := range p.ops() {
		switch op {
		case opInsertRow:
			c.Inserts++
		case opDeleteRow:
			c.Deletes++
		case opUpdateSelf, opUpdateOther, opNullToValue, opValueToNull:
			c.Updates++
		}
	}
	return c
}

// ============================================================
// BATCHES
// ============================================================

// sweepBatch is a selectable group of probes. Each batch becomes a subtest, so
// `-run 'TestDatatypeSweepLive/ranges'` runs just that batch.
type sweepBatch struct {
	Name   string
	Probes []datatypeProbe
}

// ============================================================
// RUN STATE
// ============================================================

type dbSide int

const (
	sideSource dbSide = iota
	sideTarget
	sideReplica
)

func (s dbSide) String() string {
	switch s {
	case sideSource:
		return "source"
	case sideTarget:
		return "target"
	default:
		return "source-replica"
	}
}

// probeObservation collects everything measured about one probe before a verdict is
// chosen. Keeping measurement and classification apart makes the decision table
// readable and keeps the verdict reproducible from the observation alone.
type probeObservation struct {
	settledVerdict string // non-empty short-circuits classification (SKIPPED)
	settledDetail  string

	snapshotVerdict string
	snapshotDetail  string
	streamVerdict   string
	streamDetail    string

	eventsForTable     int
	columnSeenInEvents bool
	queueScanNote      string

	// Evidence that the run actually measured this probe. Without these, an aborted
	// run leaves every field zero and the classifier cannot tell "clean" from
	// "never looked".
	snapshotCompared bool
	streamCompared   bool

	// deltaOpsApplied counts change statements the database accepted; deltaConfirmed
	// records that the delta is actually visible on the side it was applied to.
	// Together they separate "the ops never happened" (INCONCLUSIVE) from "the ops
	// happened but produced no events" (SILENT_LOSS).
	deltaOpsApplied int
	deltaConfirmed  bool

	warned      bool
	promptShown bool

	waitTimedOut bool
	waitNote     string
	stuckDetail  string

	// channelWedgedBy names the OTHER probe whose value crash-looped the import channel
	// during this run. The channel is ordered, so once one value wedges it every later
	// event - including every batch-mate's - is stuck behind it. Such a probe was not
	// measured at all: reporting it STUCK would blame it for someone else's poison, and
	// reporting its (necessarily incomplete) value comparison would manufacture a
	// SILENT_LOSS. It is INCONCLUSIVE, and the culprit is named so the batch can be
	// re-run without it.
	channelWedgedBy string
	// channelWedgedHow says what the culprit actually did, since the two shapes want
	// different words: it either crash-looped the channel (the importer is alive and
	// retrying) or killed the process outright. Empty means the crash-loop.
	channelWedgedHow string

	// commandExited records that a command the wait depended on was no longer running.
	// It is an environment fact on its own - a process that has exited produces no more
	// counts - and becomes a datatype verdict only via stuckDetail, when the log carries
	// a quotable reason for the exit.
	commandExited     bool
	commandExitDetail string

	// exporterDied records that the EXPORT side demonstrably failed and that this probe
	// is the one it can be attributed to. exporterDetail carries the quotable cause: the
	// Java exception, the `Connector completed: success = 'false'` line, or
	// `Export of data failed!`. Never set without one - see exportFailureEvidence.
	exporterDied   bool
	exporterDetail string

	// exporterDiedInRun is the collateral half, exactly parallel to channelWedgedBy: the
	// exporter died during this run but the failure did not name this probe, so this type
	// was never measured. An exporter that dies at startup takes the WHOLE run down, so
	// its batch-mates have no verdict of any kind - INCONCLUSIVE, never a product claim.
	exporterDiedInRun string

	// exportNeverStreamed records that `export data` never reached streaming mode, so
	// the change ops were never observable by anything. That is an environment flake
	// (a slow or dead Debezium JVM), emphatically not a product stall, and it must
	// never be reported as STUCK.
	exportNeverStreamed bool
	flakeDetail         string

	// destSample is the verbatim destination text of the value under test, recorded
	// only for probes with RecordDestValue.
	destSample string

	// srcValue and dstValue are the verbatim baseline-row texts on each side, recorded
	// for EVERY probe rather than only the RecordDestValue ones, and emitted on their own
	// PROBE-VALUES line. They exist so the audit tooling reads the two values it needs as
	// fields instead of regex-scraping them back out of the prose detail - the detail is
	// for a human, and its wording is free to change.
	//
	// Only the FORWARD direction (source -> target) is recorded, and a later phase
	// overwrites an earlier one, so these are "what the target ended up holding".
	srcValue string
	dstValue string

	runAbort string
}

type sweepRun struct {
	t      *testing.T
	lm     *LiveMigrationTest
	mode   sweepMode
	schema string
	dbName string
	batch  string

	// flaked records that this run hit the Debezium-boot flake, so the runner knows to
	// re-run it rather than record its verdicts.
	flaked bool

	// quarantined lists the probes caught crash-looping the import channel during this
	// run. Everything else in the batch is collateral and has to be re-run without them.
	quarantined []string

	// exportDeath is the quoted cause of an export-side death, once one has been seen.
	// Non-empty means every measurement after that point is worthless: the exporter is
	// gone, so nothing further was produced for anybody.
	exportDeath string
	// exportDeathCulprit is the probe that death was attributed to, or "" when the
	// failure named none. Only an attributed death is publishable.
	exportDeathCulprit string

	probes   []datatypeProbe // every probe asked for, in order (all get a result line)
	active   []datatypeProbe // probes whose DDL + data actually landed
	obs      map[string]*probeObservation
	emitted  bool
	extAvail map[string]bool // "<side>/<ext>" -> installable

	// reports bounds the per-poll `get data-migration-report`, so one command stuck in
	// pipe I/O can no longer take the whole wait loop down with it.
	reports *boundedFetcher
}

func newSweepRun(t *testing.T, mode sweepMode, batchName string, probes []datatypeProbe) *sweepRun {
	schema := "sweep_schema"
	dbName := fmt.Sprintf("dtsweep_%s_%s", mode.slug(), sanitizeIdent(batchName))
	if len(dbName) > 60 {
		dbName = dbName[:60]
	}

	cfg := &TestConfig{
		SourceDB: ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: dbName},
		TargetDB: ContainerConfig{Type: "yugabytedb", DatabaseName: dbName},
		// Only the schema shell goes through SetupSchema: container.ExecuteSqls* calls
		// utils.ErrExit on any SQL error, which would take the whole test binary down.
		// Every probe's DDL is applied by the runner over a *sql.DB so a rejected type
		// becomes one SKIPPED line instead of a dead run.
		SchemaNames: []string{schema},
		SchemaSQL: []string{
			fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE;", schema),
			fmt.Sprintf("CREATE SCHEMA %s;", schema),
		},
		CleanupSQL: []string{fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE;", schema)},
	}
	if mode == modeFallForward {
		cfg.SourceReplicaDB = ContainerConfig{Type: "postgresql", DatabaseName: dbName + "_sr"}
	}

	r := &sweepRun{
		t:        t,
		lm:       NewLiveMigrationTest(t, cfg),
		mode:     mode,
		schema:   schema,
		dbName:   dbName,
		batch:    batchName,
		probes:   probes,
		obs:      map[string]*probeObservation{},
		extAvail: map[string]bool{},
	}
	for _, p := range probes {
		r.obs[p.ID] = &probeObservation{}
	}
	// The offline flow starts its export and import SYNCHRONOUSLY, and a synchronous start
	// blocks in exec.Cmd.Wait until the output pipes close as well as until the process
	// exits - so a wedged grandchild holding the inherited pipe parks the harness in an
	// I/O wait with no output at all. Offline runs no bounded wait of its own, so this
	// ceiling is the only thing between it and a hang.
	r.lm.SetSyncCommandTimeout(seconds(sweepSyncCommandTimeout))

	r.reports = &boundedFetcher{fetch: r.lm.GetDataMigrationReport}
	// A fetch that outlived its budget is still running and still logging through t.
	// t.Cleanup runs before the test is marked complete, so this is the last point at
	// which that goroutine can be given a chance to finish safely.
	t.Cleanup(func() {
		if !r.reports.drain(sweepFetchDrainBudget) {
			t.Logf("a `get data-migration-report` was still stuck in pipe I/O after %s; "+
				"its process, or a grandchild holding its stdout pipe, never exited",
				sweepFetchDrainBudget)
		}
	})
	return r
}

func sanitizeIdent(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// ============================================================
// PUBLIC ENTRY POINT
// ============================================================

// runDatatypeSweep sets up one schema containing one table per probe, runs `mode`, and
// emits exactly one PROBE-RESULT line per probe. It never fails the test because a
// probe misbehaved: a bad datatype is the finding, not an error. It DOES fail the test
// when the harness itself is suspect (a known-good control that did not come out WORKS)
// or when a poison probe was handed to it in a batch.
func runDatatypeSweep(t *testing.T, mode sweepMode, batch sweepBatch) {
	probes := excludeBatchedPoison(t, mode, batch.Name, withControls(batch))

	r := newSweepRun(t, mode, batch.Name, probes)
	// Cleanup registered FIRST so it runs LAST: emitAll's known-answer control check
	// calls t.Errorf, and LiveMigrationTest.Cleanup only preserves the export dir when
	// t.Failed() is already true. With the opposite order the export dir - the only
	// place an EXPORT-side root cause exists - was deleted before the run was marked
	// failed.
	defer r.lm.Cleanup()
	defer r.emitAll()

	if err := r.lm.SetupContainers(context.Background()); err != nil {
		r.abortAll(fmt.Sprintf("container setup failed: %v", err))
		t.Fatalf("failed to setup containers: %v", err)
		return
	}
	if err := r.lm.SetupSchema(); err != nil {
		r.abortAll(fmt.Sprintf("schema setup failed: %v", err))
		t.Fatalf("failed to setup schema: %v", err)
		return
	}

	r.installExtensions()
	r.applyProbeDDL()
	r.seedInitialData()

	if len(r.active) == 0 {
		t.Logf("no probe survived setup; every probe reported SKIPPED")
		return
	}

	switch mode {
	case modeOffline:
		r.runOffline()
	case modeLive:
		r.runLive()
	case modeFallback:
		r.runFallback()
	case modeFallForward:
		r.runFallForward()
	default:
		r.abortAll(fmt.Sprintf("unknown mode %q", mode))
	}
}

// withControls prepends the known-good int/text controls to every batch, per
// PROBE_SPEC.md: if a control fails, the harness is wrong and not the product.
func withControls(batch sweepBatch) []datatypeProbe {
	seen := map[string]bool{}
	var out []datatypeProbe
	for _, p := range append(controlProbes(), batch.Probes...) {
		if seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		out = append(out, p)
	}
	return out
}

// excludeBatchedPoison drops known-poison probes from a BATCH, in every mode.
//
// A probe already established as BLOCKS or STUCK anywhere wrecks the replication channel,
// so batching it produces no usable verdict for it AND none for its batch-mates - the
// controls come out broken and the whole run is discarded. Excluding it keeps the batch
// informative; the poison probe itself is covered by a solo run, where the damage is
// attributable to it alone.
//
// A solo run (one non-control probe) keeps its probe: isolating poison is the whole point
// of TestDatatypeSweepSuspect.
func excludeBatchedPoison(t *testing.T, mode sweepMode, batchName string, probes []datatypeProbe) []datatypeProbe {
	t.Helper()
	nonControl := 0
	for _, p := range probes {
		if p.ExpectVerdict == "" {
			nonControl++
		}
	}
	if nonControl <= 1 {
		return probes
	}
	kept := make([]datatypeProbe, 0, len(probes))
	for _, p := range probes {
		if p.ExpectVerdict == "" && p.Poison {
			// Greppable, so the runner and the audit can both see what was left out.
			why := p.PoisonNote
			if why == "" {
				why = "known poison"
			}
			fmt.Printf("PROBE-RUN-EXCLUDED: %s | %s | %s (%s) excluded from this batch: %s; "+
				"run it with PROBE_ID=%s PROBE_MODE=%s -run TestDatatypeSweepSuspect\n",
				batchName, mode, p.ID, p.TypeName, why, p.ID, mode)
			t.Logf("excluding known-poison probe %s (%s) from batch %s in mode %s",
				p.ID, p.TypeName, batchName, mode)
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

// ============================================================
// SETUP (extension probing, DDL, seed data) - all failure-tolerant
// ============================================================

// streamingTimeout is shorter for a solo run: see sweepSoloStreamingTimeout.
func (r *sweepRun) streamingTimeout() time.Duration {
	if strings.HasPrefix(r.batch, "solo_") {
		return sweepSoloStreamingTimeout
	}
	return sweepStreamingTimeout
}

func (r *sweepRun) sides() []dbSide {
	sides := []dbSide{sideSource, sideTarget}
	if r.mode == modeFallForward {
		sides = append(sides, sideReplica)
	}
	return sides
}

func (r *sweepRun) withConn(side dbSide) func(func(*sql.DB) error) error {
	switch side {
	case sideSource:
		return r.lm.WithSourceConn
	case sideTarget:
		return r.lm.WithTargetConn
	default:
		return r.lm.WithSourceReplicaConn
	}
}

// execOn runs statements in order on one side, stopping at the first error.
func (r *sweepRun) execOn(side dbSide, stmts ...string) error {
	return r.withConn(side)(func(db *sql.DB) error {
		for _, s := range stmts {
			if _, err := db.Exec(s); err != nil {
				return goerrors.Errorf("%s: %q: %w", side, truncate(s, 140), err)
			}
		}
		return nil
	})
}

// installExtensions attempts every extension any probe asks for, on every participating
// database, and records what actually worked. Nothing here can fail the run.
func (r *sweepRun) installExtensions() {
	wanted := map[string]bool{}
	for _, p := range r.probes {
		for _, e := range p.Extensions {
			wanted[e] = true
		}
	}
	names := make([]string, 0, len(wanted))
	for e := range wanted {
		names = append(names, e)
	}
	sort.Strings(names)

	for _, side := range r.sides() {
		for _, ext := range names {
			err := r.execOn(side, fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s", ext))
			r.extAvail[extKey(side, ext)] = err == nil
			if err != nil {
				r.t.Logf("extension %q unavailable on %s: %v", ext, side, err)
			}
		}
	}
}

func extKey(side dbSide, ext string) string { return side.String() + "/" + ext }

// missingExtension returns the first extension this probe needs that is not available
// on some participating database.
func (r *sweepRun) missingExtension(p datatypeProbe) (string, dbSide, bool) {
	for _, side := range r.sides() {
		for _, ext := range p.Extensions {
			if !r.extAvail[extKey(side, ext)] {
				return ext, side, true
			}
		}
	}
	return "", sideSource, false
}

// applyProbeDDL creates each probe's supporting types and table on every participating
// database. A probe whose DDL any server rejects is dropped from the run with a SKIPPED
// verdict rather than taking the batch down.
func (r *sweepRun) applyProbeDDL() {
	for _, p := range r.probes {
		if ext, side, missing := r.missingExtension(p); missing {
			r.abandonProbe(p, verdictSkipped, fmt.Sprintf("extension unavailable: %s on %s", ext, side))
			continue
		}

		ddl := p.ddl(r.schema)
		failed := false
		for _, side := range r.sides() {
			if err := r.execOn(side, ddl...); err != nil {
				r.abandonProbe(p, verdictSkipped, fmt.Sprintf("DDL rejected: %v", err))
				failed = true
				break
			}
		}
		if failed {
			continue
		}

		// Source-only. The YB target keeps REPLICA IDENTITY CHANGE on purpose.
		if err := r.execOn(sideSource, p.replicaIdentitySQL(r.schema)); err != nil {
			r.abandonProbe(p, verdictSkipped, fmt.Sprintf("REPLICA IDENTITY FULL rejected: %v", err))
			continue
		}
		r.active = append(r.active, p)
	}
}

// seedInitialData inserts the snapshot rows on the SOURCE only. A probe whose literal
// the server rejects is a case-table bug, not a product finding, so it reports SKIPPED.
func (r *sweepRun) seedInitialData() {
	var kept []datatypeProbe
	for _, p := range r.active {
		if err := r.execOn(sideSource, p.initialDataSQL(r.schema)); err != nil {
			r.abandonProbe(p, verdictSkipped, fmt.Sprintf("initial data rejected: %v", err))
			continue
		}
		kept = append(kept, p)
	}
	r.active = kept
}

// applyDelta runs one direction of deltas against one side, per probe.
func (r *sweepRun) applyDelta(side dbSide, reverse bool) {
	for _, p := range r.active {
		stmts := p.deltaSQL(r.schema, reverse)
		if err := r.execOn(side, stmts...); err == nil {
			r.observe(p).deltaOpsApplied += len(stmts)
		} else {
			// The delta failing on the DESTINATION of a previous phase is itself a
			// finding (the migrated value cannot be updated), so record it as detail
			// rather than skipping.
			r.observe(p).waitNote = appendNote(r.observe(p).waitNote,
				fmt.Sprintf("delta on %s failed: %v", side, err))
			r.t.Logf("probe %s: delta on %s failed: %v", p.ID, side, err)
		}
	}
}

// confirmDeltaApplied re-reads the side the delta was just written to and records
// whether the change is actually visible there. This is what lets the classifier tell
// "the change ops never happened, so the run proved nothing" (INCONCLUSIVE) apart from
// "the change ops really happened on the source but no event ever came out"
// (SILENT_LOSS). Without it, both look like an unchanged table.
func (r *sweepRun) confirmDeltaApplied(side dbSide, reverse bool) {
	newInsert, toDelete, otherCol := rowNewInsert, rowToDelete, rowOtherCol
	if reverse {
		newInsert, toDelete, otherCol = revNewInsert, revToDelete, revOtherCol
	}
	for _, p := range r.active {
		rows, err := r.fetchProbeValues(side, p)
		if err != nil {
			r.t.Logf("probe %s: cannot confirm delta on %s: %v", p.ID, side, err)
			continue
		}
		ops := map[deltaOp]bool{}
		for _, op := range p.ops() {
			ops[op] = true
		}
		confirmed := false
		// The insert and the delete are the two ops whose effect shows up as the mere
		// presence or absence of a row, so confirming them needs no value comparison
		// and cannot be confused with a type-fidelity problem.
		if ops[opInsertRow] {
			if _, ok := rows[newInsert]; ok {
				confirmed = true
			}
		}
		if ops[opDeleteRow] {
			if _, ok := rows[toDelete]; !ok {
				confirmed = true
			}
		}
		if !confirmed && ops[opUpdateOther] {
			if row, ok := rows[otherCol]; ok && row.filler.Valid &&
				strings.HasSuffix(row.filler.String, "-touched") {
				confirmed = true
			}
		}
		r.observe(p).deltaConfirmed = confirmed
		if !confirmed {
			r.t.Logf("probe %s: delta NOT visible on %s after applyDelta", p.ID, side)
		}
	}
}

// ============================================================
// MODE FLOWS
// ============================================================

// snapshotExpectations builds the per-table row counts the report must reach.
func (r *sweepRun) snapshotExpectations() map[string]int64 {
	m := map[string]int64{}
	for _, p := range r.active {
		m[p.reportKey(r.schema)] = sweepRowCount
	}
	return m
}

func (r *sweepRun) changeExpectations() map[string]ChangesCount {
	m := map[string]ChangesCount{}
	for _, p := range r.active {
		m[p.reportKey(r.schema)] = p.expectedChanges()
	}
	return m
}

func (r *sweepRun) activeTableKeys() []string {
	keys := make([]string, 0, len(r.active))
	for _, p := range r.active {
		keys = append(keys, p.reportKey(r.schema))
	}
	return keys
}

// fallForwardRowCountsMatch is the fall-forward wait's positive signal: every probe table
// holds the same number of rows on the target and on the source-replica. It mirrors
// LiveMigrationTest.WaitForFallForwardStreamingComplete's condition, which the sweep can
// no longer call directly because it drives its own crash-loop-aware loop.
func (r *sweepRun) fallForwardRowCountsMatch(tables []string) bool {
	allMatch := true
	err := r.lm.WithTargetConn(func(targetConn *sql.DB) error {
		return r.lm.WithSourceReplicaConn(func(replicaConn *sql.DB) error {
			for _, table := range tables {
				if err := testutils.CompareRowCount(context.Background(), targetConn, replicaConn, table); err != nil {
					allMatch = false
					return nil
				}
			}
			return nil
		})
	})
	return err == nil && allMatch
}

// runOffline: export data (snapshot-only) -> import data. The datatype filter does not
// run in this mode, so the interesting question is purely snapshot fidelity.
//
// StartExportData hardcodes --export-type snapshot-and-changes, but extraArgs are
// appended AFTER the base args and pflag takes the last occurrence of a string flag,
// so passing --export-type here overrides it. cmd.SNAPSHOT_ONLY is used rather than a
// literal so the two stay in sync.
func (r *sweepRun) runOffline() {
	if err := r.lm.StartExportData(false, map[string]string{"--export-type": cmd.SNAPSHOT_ONLY}); err != nil {
		r.abortExport("snapshot-only export data failed", err)
		return
	}
	if err := r.lm.StartImportData(false, map[string]string{"--log-level": "debug"}); err != nil {
		r.abortAll(r.importAbortReason("import data failed", err))
		return
	}
	// No snapshot wait here. `import data` was started with async=false, so the call
	// above only returns once the snapshot import has finished. The usual
	// WaitForSnapshotComplete cannot be used in this mode anyway: it polls
	// `get data-migration-report`, which refuses in snapshot-only mode
	// ("Data migration report is only applicable when export-type is
	// 'snapshot-and-changes'"), and its internal FatalIfError would fail the batch.
	r.recordExportWarnings()
	r.compareInto(sideSource, sideTarget, phaseSnapshot)
}

// runLive: export data (snapshot-and-changes) -> import data, then the forward deltas.
func (r *sweepRun) runLive() {
	if !r.startForwardMigration() {
		return
	}
	r.forwardSnapshotAndStream()
}

// runFallback: the live flow, then cutover with --prepare-for-fall-back, then the
// reverse direction with target-side deltas.
//
// Design note: with prepareForFallback=true, the running `import data` process exec's
// into `export data from target` and the running `export data` process becomes
// `import data to source` at cutover; WaitForCutoverComplete re-points the fixture's
// command handles accordingly. That IS the export-from-target + import-to-source pair -
// the fixture offers no way to start them as fresh commands in this flow, so the flow
// below follows the existing fall-back idiom instead of inventing one.
func (r *sweepRun) runFallback() {
	if !r.startForwardMigration() {
		return
	}
	r.forwardSnapshotAndStream()

	if err := r.lm.InitiateCutoverToTarget(true, nil); err != nil {
		r.abortAll(fmt.Sprintf("cutover to target failed: %v", err))
		return
	}
	if err := r.lm.WaitForCutoverComplete(0, sweepCutoverTimeout); err != nil {
		r.abortAll(fmt.Sprintf("cutover to target did not complete: %v", err))
		return
	}

	if err := r.lm.WaitForExportFromTargetStarted(sweepExportStartWait); err != nil {
		r.markExportNeverStreamed(fmt.Sprintf(
			"export data from target never started (%v), so no target-side change op could be observed", err))
		return
	}
	r.applyDelta(sideTarget, true)
	r.confirmDeltaApplied(sideTarget, true)
	expected := r.changeExpectations()
	r.waitBounded("fall-back streaming", seconds(r.streamingTimeout()), func(report *DataMigrationReport) bool {
		return report != nil && streamingComplete(report, expected, "target", "source")
	})
	r.recordExportWarnings()
	r.recordQueueColumnPresence()
	r.compareInto(sideTarget, sideSource, phaseStreaming)
}

// runFallForward: the live flow, then import data to source-replica, cutover, then
// target-side deltas replicated onward to the source-replica.
func (r *sweepRun) runFallForward() {
	if !r.startForwardMigration() {
		return
	}
	r.forwardSnapshotAndStream()

	if err := r.lm.StartImportDataToSourceReplica(true, nil); err != nil {
		r.abortAll(fmt.Sprintf("import data to source-replica failed: %v", err))
		return
	}
	if err := r.lm.WaitForFallForwardEnabled(0, sweepFallForwardWait); err != nil {
		r.abortAll(fmt.Sprintf("fall-forward was not enabled: %v", err))
		return
	}
	if err := r.lm.InitiateCutoverToTarget(false, nil); err != nil {
		r.abortAll(fmt.Sprintf("cutover to target failed: %v", err))
		return
	}
	if err := r.lm.WaitForCutoverComplete(0, sweepCutoverTimeout); err != nil {
		r.abortAll(fmt.Sprintf("cutover to target did not complete: %v", err))
		return
	}

	if err := r.lm.WaitForExportFromTargetStarted(sweepExportStartWait); err != nil {
		r.markExportNeverStreamed(fmt.Sprintf(
			"export data from target never started (%v), so no target-side change op could be observed", err))
		return
	}
	r.applyDelta(sideTarget, true)
	r.confirmDeltaApplied(sideTarget, true)
	// The fixture's fall-forward wait compares ROW COUNTS target vs source-replica; it
	// has no per-table change-count equivalent. Value fidelity is checked separately by
	// compareInto below, so a passing wait is a precondition and not the assertion.
	tables := r.activeTableKeys()
	r.waitBounded("fall-forward streaming", seconds(r.streamingTimeout()), func(*DataMigrationReport) bool {
		return r.fallForwardRowCountsMatch(tables)
	})
	r.recordExportWarnings()
	r.recordQueueColumnPresence()
	r.compareInto(sideTarget, sideReplica, phaseStreaming)
}

// startForwardMigration launches the async export/import pair used by every non-offline
// mode. Returns false if the run cannot continue.
func (r *sweepRun) startForwardMigration() bool {
	if err := r.lm.StartExportData(true, nil); err != nil {
		r.abortExport("export data failed to start", err)
		return false
	}
	if err := r.lm.StartImportData(true, map[string]string{"--log-level": "debug"}); err != nil {
		r.abortAll(r.importAbortReason("import data failed to start", err))
		return false
	}
	return true
}

// forwardSnapshotAndStream drives snapshot -> compare -> forward deltas -> stream ->
// compare, recording rather than aborting on any expired wait.
func (r *sweepRun) forwardSnapshotAndStream() {
	expectedRows := r.snapshotExpectations()
	r.waitBounded("snapshot", seconds(sweepSnapshotTimeout), func(report *DataMigrationReport) bool {
		return report != nil && snapshotComplete(report, expectedRows)
	})
	r.recordExportWarnings()
	r.compareInto(sideSource, sideTarget, phaseSnapshot)

	// Export must actually BE streaming before the deltas are written. Writing them
	// into a window nobody is watching is a likely cause of zero-event runs, and it
	// makes an environment problem look like a product one.
	if err := r.lm.WaitForStreamingMode(sweepStreamingModeWait, sweepStreamingModePoll); err != nil {
		r.markExportNeverStreamed(fmt.Sprintf(
			"export data never reached streaming mode within %s (%v), so no change op could "+
				"be observed; this is an environment flake (slow or dead Debezium JVM), not a product stall",
			sweepStreamingModeWait, err))
		return
	}

	r.applyDelta(sideSource, false)
	r.confirmDeltaApplied(sideSource, false)
	expected := r.changeExpectations()
	r.waitBounded("forward streaming", seconds(r.streamingTimeout()), func(report *DataMigrationReport) bool {
		return report != nil && streamingComplete(report, expected, "source", "target")
	})
	r.recordExportWarnings()
	r.recordQueueColumnPresence()
	r.compareInto(sideSource, sideTarget, phaseStreaming)
}

// ============================================================
// BOUNDED WAITS WITH CRASH-LOOP DETECTION
// ============================================================

// waitOutcome says WHY a bounded wait ended. It is printed on the PROBE-WAIT line so the
// cost of every wait, and the reason it was paid, is visible in the run output.
type waitOutcome string

const (
	// waitSatisfied - the positive signal arrived: the expected counts were reached.
	waitSatisfied waitOutcome = "counts-satisfied"
	// waitRepeatingError - the negative signal arrived: the importer is crash-looping on
	// one error and the counts are frozen. This is the whole point of the loop.
	waitRepeatingError waitOutcome = "repeating-error"
	// waitExportDied - the OTHER negative signal: the export side is gone. There is no
	// point waiting out a streaming budget for events from a process that is dead, and
	// unlike a crash-loop this needs no repeat count: a connector that reported itself
	// completed-with-failure does not un-complete.
	waitExportDied waitOutcome = "exporter-died"
	// waitProcessGone - the strongest of the negative signals, because it needs no
	// heuristic at all: a command the wait depends on is NO LONGER RUNNING. Event counts
	// cannot arrive from a process that has exited, so waiting out the budget for them is
	// pointless by definition.
	//
	// It is also the commonest failure shape in this suite. A type the importer cannot
	// bind on the target - `cannot accept a value of type pg_node_tree` (SQLSTATE 0A000),
	// and the same for tid and regclass - makes `import data` report the error ONCE and
	// exit. There is no crash-loop to count, so the repeat threshold never trips, and
	// before this outcome existed a batch of such types (catalogstats) sat out its whole
	// budget and emitted nothing.
	waitProcessGone waitOutcome = "process-exited"
	// waitNoOutput - nothing anywhere moved for sweepSilenceGrace: not the counts, not
	// the import log, not the export log. Something upstream is wedged in a way that
	// writes nothing down, which is an environment fact and classifies INCONCLUSIVE.
	// It exists so that "produced no output for five minutes" is CONCLUDED rather than
	// sat out, and so the run log says which of the two silences it was.
	waitNoOutput waitOutcome = "no-output"
	// waitTimeout - neither signal arrived within the budget: a stall that logs nothing.
	// A genuinely different case from waitRepeatingError, and it must stay distinguishable:
	// it classifies INCONCLUSIVE, never STUCK.
	waitTimeout waitOutcome = "timeout"
)

// waitSample is one observation of the pipeline: has the positive signal arrived, what do
// the counts look like right now, and what has the importer been saying.
type waitSample struct {
	satisfied bool
	// progress is a fingerprint of the numbers being waited on. Any change means the
	// pipeline is moving, which rules out a crash-loop no matter what the log holds.
	progress string
	logText  string
	// exportText is the export side's log, watched alongside the import side's. Without
	// it the loop can only ever conclude something about the importer, which is how an
	// exporter that died at startup used to cost a full 900 s budget and then report
	// nothing at all.
	exportText string

	// goneCommand names a migration command that is no longer running, and goneErr is the
	// error its Wait returned (nil for a clean exit). Empty means everything the wait
	// depends on is still alive. This is liveness, not log-reading: it is true or false,
	// with no threshold to tune and no signature to match.
	goneCommand string
	goneErr     error
}

type waitResult struct {
	outcome     waitOutcome
	elapsed     time.Duration
	budget      time.Duration
	polls       int
	quotedError string // set for waitRepeatingError: the error, with its SQLSTATE
	repeats     int
	silence     time.Duration // set for waitNoOutput: how long nothing moved
	goneCommand string        // set for waitProcessGone: the command that is no longer running
	goneErr     error         // its exit error, nil for a clean exit
}

// exitDescription renders a dead command's exit for the PROBE-WAIT line. A clean exit is
// called out explicitly because it is NOT a failure: a command that finished and left is a
// reason to stop waiting, not evidence against a datatype.
func (w waitResult) exitDescription() string {
	if w.goneErr == nil {
		return "it exited cleanly"
	}
	return "it exited with: " + w.goneErr.Error()
}

// activitySignature renders everything the pipeline could possibly have produced since the
// last poll: the counts, and both logs.
//
// The logs are HASHED rather than measured. importLogFileText and exportLogFileText both
// return only a 512 KiB tail, so once a log passes that size its length stops changing
// while its content still does - and a length-only check would call a busy, crash-looping
// pipeline silent.
func activitySignature(s waitSample) string {
	return fmt.Sprintf("%s|%d:%x|%d:%x",
		s.progress, len(s.logText), hashText(s.logText), len(s.exportText), hashText(s.exportText))
}

func hashText(s string) uint64 {
	h := fnv.New64a()
	_, _ = io.WriteString(h, s)
	return h.Sum64()
}

// saved is the wait budget the early conclusion did not have to spend. Both negative
// signals save it: a wedged importer and a dead exporter are equally pointless to sit out.
func (w waitResult) saved() time.Duration {
	switch w.outcome {
	case waitRepeatingError, waitExportDied, waitNoOutput, waitProcessGone:
	default:
		return 0
	}
	if w.elapsed > w.budget {
		return 0
	}
	return w.budget - w.elapsed
}

// summary is the human half of the PROBE-WAIT line: what ended the wait, and what that
// cost or saved. Kept pure so the wording is unit-testable without a container.
func (w waitResult) summary() string {
	switch w.outcome {
	case waitSatisfied:
		return fmt.Sprintf("expected counts reached after %d polls", w.polls)
	case waitRepeatingError:
		return fmt.Sprintf("%s repeated x%d with the observed counts frozen across %d polls; "+
			"concluded without waiting out the remaining %ds of the budget",
			w.quotedError, w.repeats, sweepCrashLoopPolls, int(w.saved().Seconds()))
	case waitExportDied:
		return fmt.Sprintf("the export side is dead - %s; no event can arrive from a dead "+
			"exporter, so the remaining %ds of the budget was not waited out",
			w.quotedError, int(w.saved().Seconds()))
	case waitProcessGone:
		return fmt.Sprintf("%s is no longer running - %s; event counts cannot arrive from a "+
			"process that has exited, so the remaining %ds of the budget was not waited out",
			w.goneCommand, w.exitDescription(), int(w.saved().Seconds()))
	case waitNoOutput:
		return fmt.Sprintf("nothing moved anywhere for %.0fs - not the migration-report counts, "+
			"not the import log, not the export log: a pipeline that logged nothing at all, "+
			"which is an environment fact and not a datatype verdict; concluded without "+
			"waiting out the remaining %ds of the budget",
			w.silence.Seconds(), int(w.saved().Seconds()))
	default:
		return "budget exhausted with no repeating importer error in the log: a stall that " +
			"logged nothing, which is an environment fact and not a datatype verdict"
	}
}

// waitForSignalOrCrashLoop is the wait loop, with the clock and the pipeline both injected
// so it can be exercised end-to-end with no containers and no real time.
//
// It ends on the FIRST of: the positive signal (counts satisfied), the negative signal (a
// repeating import failure with frozen counts), or the budget.
func waitForSignalOrCrashLoop(
	budget, poll time.Duration,
	sample func() waitSample,
	now func() time.Time,
	sleep func(time.Duration),
) waitResult {
	start := now()
	var lastSignature, lastProgress, lastActivity string
	quietSince := start
	signaturePolls := 0
	firstSample := true
	polls := 0

	for {
		s := sample()
		polls++
		elapsed := now().Sub(start)

		// Anything at all that the pipeline could have produced since the last poll.
		// Checked before the early returns so that a poll which DID see movement never
		// leaves a stale silence clock behind it.
		if act := activitySignature(s); firstSample || act != lastActivity {
			lastActivity, quietSince = act, now()
		}

		if s.satisfied {
			return waitResult{outcome: waitSatisfied, elapsed: elapsed, budget: budget, polls: polls}
		}

		// The export side is checked BEFORE the import side because a dead exporter
		// explains an idle importer, and never the other way round. It is checked after
		// the positive signal, though: if the counts already arrived, whatever the
		// exporter did on its way out is not this wait's business.
		if cause, dead := exportFailureEvidence(s.exportText); dead {
			return waitResult{
				outcome: waitExportDied, elapsed: elapsed, budget: budget,
				polls: polls, quotedError: cause,
			}
		}

		// Liveness. Checked after the export log because a log that says WHY the exporter
		// died is a better finding than "the process is gone", and before everything
		// below because no amount of log-reading changes the fact that nothing more is
		// coming from a process that has exited.
		if s.goneCommand != "" {
			// One confirming sample, one poll interval later. The counts are read at the
			// top of a poll, so a command that wrote its last events and then exited
			// would otherwise be called dead-without-result a poll too early; the pause
			// is what gives the report - and a fetch that was stalled on this poll - time
			// to catch up rather than re-reading the same stale numbers immediately.
			sleep(poll)
			confirm := sample()
			polls++
			elapsed = now().Sub(start)
			if confirm.satisfied {
				return waitResult{outcome: waitSatisfied, elapsed: elapsed, budget: budget, polls: polls}
			}
			return waitResult{
				outcome: waitProcessGone, elapsed: elapsed, budget: budget, polls: polls,
				goneCommand: s.goneCommand, goneErr: s.goneErr,
			}
		}

		quoted, signature, repeats := mostRepeatedErrorDetail(s.logText, "")
		advanced := !firstSample && s.progress != lastProgress
		lastProgress, firstSample = s.progress, false

		switch {
		case advanced:
			// The counts moved. Whatever the log says, nothing is wedged: an importer
			// that is still ingesting is not stuck on a batch it cannot get past.
			lastSignature, signaturePolls = "", 0
		case signature != "" && repeats >= sweepCrashLoopRepeats:
			if signature == lastSignature {
				signaturePolls++
			} else {
				lastSignature, signaturePolls = signature, 1
			}
			if signaturePolls >= sweepCrashLoopPolls {
				return waitResult{
					outcome: waitRepeatingError, elapsed: elapsed, budget: budget,
					polls: polls, quotedError: quoted, repeats: repeats,
				}
			}
		default:
			lastSignature, signaturePolls = "", 0
		}

		// Nothing anywhere has moved for the grace period. There is no point sitting out
		// the rest of a budget for a pipeline that is writing nothing down.
		if silence := now().Sub(quietSince); silence >= sweepSilenceGrace {
			return waitResult{
				outcome: waitNoOutput, elapsed: elapsed, budget: budget,
				polls: polls, silence: silence,
			}
		}

		if elapsed >= budget {
			return waitResult{outcome: waitTimeout, elapsed: elapsed, budget: budget, polls: polls}
		}
		sleep(poll)
	}
}

// reportFingerprint renders every count the waits care about into one comparable string.
// Any advance anywhere in it means the pipeline moved. It deliberately covers ALL the
// tables in the report rather than only the expected ones: a batch-mate still making
// progress is just as good a proof that the channel is not wedged.
func reportFingerprint(report *DataMigrationReport) string {
	if report == nil {
		return "<report-unavailable>"
	}
	rows := make([]string, 0, len(report.RowData))
	for _, row := range report.RowData {
		rows = append(rows, fmt.Sprintf("%s/%s:%d,%d,%d,%d,%d,%d,%d,%d",
			row.TableName, row.DBType,
			row.ExportedSnapshotRows, row.ImportedSnapshotRows,
			row.ExportedInserts, row.ExportedUpdates, row.ExportedDeletes,
			row.ImportedInserts, row.ImportedUpdates, row.ImportedDeletes))
	}
	sort.Strings(rows)
	return strings.Join(rows, "|")
}

// ============================================================
// THE POLL ITSELF MUST BE BOUNDED
// ============================================================

/*
A wait loop is only as bounded as the slowest thing inside one poll, and one poll shells
out to `get data-migration-report`.

VoyagerCommandRunner hands the child an io.MultiWriter for stdout and stderr. Because that
is not an *os.File, os/exec creates an OS pipe and a copier goroutine, and Cmd.Wait() then
blocks until the pipe reaches EOF - which requires every descendant that inherited the
write end to exit, not just the process that was started. One wedged grandchild (the
Debezium JVM is the obvious candidate) therefore blocks the reader indefinitely, in
`goroutine [IO wait]`, even with the voyager process itself already dead. The framework
documents the same hazard on WaitForAsyncCompletion.

When that happened inside a poll it took the entire wait loop down with it: no polling, no
crash-loop detection, no export-death detection, and - because PROBE-WAIT is only printed
when a wait ENDS - not one line of output saying anything was wrong. A `catalogstats` live
batch spent 13 minutes that way and emitted nothing.

boundedFetcher makes the fetch unable to do that:

  - at most ONE fetch is outstanding at a time, so a wedged one leaks a single goroutine
    instead of one per poll;
  - the first poll to exceed the budget gives up and reports the report as unreadable,
    which the loop already handles (satisfied(nil) is false, and the fingerprint becomes
    "<report-unavailable>");
  - every later poll checks that same outstanding fetch WITHOUT blocking, so the loop keeps
    its 2 s cadence instead of paying the budget again and again;
  - if the wedged fetch ever completes, the fetcher goes straight back to normal.

The leaked goroutine writes to a buffered channel, so it can always finish. It logs through
t, so the run drains it from t.Cleanup - which runs before the test is marked complete and
therefore before logging from it would panic.
*/

type reportResult struct {
	report *DataMigrationReport
	err    error
}

type boundedFetcher struct {
	fetch   func() (*DataMigrationReport, error)
	pending chan reportResult
	stalled bool
	stalls  int
}

// get returns the report, or nil when this poll could not read one within budget. The
// bool reports whether the fetch is currently stalled.
func (b *boundedFetcher) get(budget time.Duration) (*DataMigrationReport, bool, error) {
	if b.pending == nil {
		ch := make(chan reportResult, 1)
		b.pending = ch
		go func() {
			rep, err := b.fetch()
			ch <- reportResult{report: rep, err: err}
		}()
	}
	// A fetch already known to be stuck is never waited on again: checking it costs
	// nothing, and paying the budget once per poll would be its own kind of hang.
	if b.stalled {
		budget = 0
	}

	var res reportResult
	if budget <= 0 {
		select {
		case res = <-b.pending:
		default:
			return nil, true, nil
		}
	} else {
		select {
		case res = <-b.pending:
		case <-time.After(budget):
			b.stalled = true
			b.stalls++
			return nil, true, nil
		}
	}
	b.pending, b.stalled = nil, false
	return res.report, false, res.err
}

// drain gives an outstanding fetch a bounded chance to finish. Called from t.Cleanup.
func (b *boundedFetcher) drain(budget time.Duration) bool {
	if b.pending == nil {
		return true
	}
	select {
	case <-b.pending:
		b.pending, b.stalled = nil, false
		return true
	case <-time.After(budget):
		return false
	}
}

// waitBounded runs one bounded wait, watching the import log for a crash-loop alongside
// the counts. A wait that ends without its positive signal never aborts the run: it is
// recorded on every active probe and fed into classification, because "the counts never
// arrived and the import log keeps repeating one error" IS the STUCK verdict.
//
// satisfied is handed the data-migration report already fetched for this poll, or nil when
// it could not be read; a predicate that needs the report must return false for nil.
func (r *sweepRun) waitBounded(what string, budget time.Duration, satisfied func(*DataMigrationReport) bool) {
	sample := func() waitSample {
		report, stalled, err := r.reports.get(sweepReportFetchBudget)
		switch {
		case stalled:
			// Not an error the run can act on, and not a reason to stop polling: the
			// other two signals (the logs) are still readable, and they are the ones
			// that identify a wedged importer or a dead exporter anyway.
			r.t.Logf("wait %q: `get data-migration-report` has not returned within %s and is "+
				"being polled without blocking; the counts are unavailable until it does",
				what, sweepReportFetchBudget)
			report = nil
		case err != nil:
			r.t.Logf("wait %q: cannot read the data-migration report: %v", what, err)
			report = nil
		}
		gone, goneErr := r.firstExitedCommand()
		return waitSample{
			satisfied:   satisfied(report),
			progress:    reportFingerprint(report),
			logText:     r.importLogFileText(),
			exportText:  r.exportLogFileText(),
			goneCommand: gone,
			goneErr:     goneErr,
		}
	}
	r.applyWaitResult(what, waitForSignalOrCrashLoop(budget, sweepWaitPoll, sample, time.Now, time.Sleep))
}

// firstExitedCommand reports the first migration command this fixture started that is no
// longer running, together with the error its Wait returned.
//
// Every command the sweep starts is supposed to be running for the whole of every wait:
// `export data` and `import data` stream until cutover (and at cutover they EXEC into
// their fall-back roles, keeping the same process), and `import data to source-replica`
// streams until a cutover the sweep never performs. So any of them being gone during a
// wait is by construction an end to that wait.
//
// It asks HasExited rather than IsStopped's old channel peek, because this runs on a timer
// and the answer has to be repeatable.
func (r *sweepRun) firstExitedCommand() (string, error) {
	for _, c := range r.lm.StartedCommands() {
		if exited, err := c.Runner.HasExited(); exited {
			return c.Name, err
		}
	}
	return "", nil
}

// applyWaitResult records one wait's outcome: the greppable PROBE-WAIT line, and, when the
// wait did not get its positive signal, the per-probe evidence classification will use.
func (r *sweepRun) applyWaitResult(what string, res waitResult) {
	fmt.Printf("PROBE-WAIT: %s | %s | %s | %s | %.1fs of %.0fs | %s\n",
		r.batch, r.mode, what, res.outcome,
		res.elapsed.Seconds(), res.budget.Seconds(), sanitizeDetail(res.summary()))

	if res.outcome == waitSatisfied {
		return
	}
	r.t.Logf("bounded wait %q ended as %s after %s: %s", what, res.outcome, res.elapsed, res.summary())

	// The export side died. There is nothing for the import-side attribution below to
	// work with - a dead exporter produces no events, so the import log has nothing to
	// say about any type in this batch - and running it anyway would pin an unrelated
	// leftover error on a probe as a STUCK verdict.
	if res.outcome == waitExportDied {
		r.recordExportDeath(res.quotedError, fmt.Sprintf(
			"detected during the %s wait after %.0fs", what, res.elapsed.Seconds()))
		for _, p := range r.active {
			o := r.observe(p)
			o.waitTimedOut = true
			o.waitNote = appendNote(o.waitNote, fmt.Sprintf(
				"%s wait concluded early on a dead exporter", what))
		}
		return
	}

	// A command the wait depended on is gone. Which log holds the reason depends on which
	// side died, and a clean exit has no reason to look for at all.
	if res.outcome == waitProcessGone {
		r.recordCommandExit(what, res)
		return
	}

	// A wait that ran out of clock, or one that gave up on a silent pipeline, is the
	// other place a dead exporter used to hide: zero events and no importer error reads
	// as "nothing happened" whichever way the run actually went, so the export side is
	// inspected before that conclusion is drawn.
	if res.outcome == waitTimeout || res.outcome == waitNoOutput {
		if r.exportDiedDuring(fmt.Sprintf("found after the %s wait ended as %s", what, res.outcome)) {
			for _, p := range r.active {
				o := r.observe(p)
				o.waitTimedOut = true
				o.waitNote = appendNote(o.waitNote, fmt.Sprintf("%s wait ended as %s", what, res.outcome))
			}
			return
		}
	}

	// Read the importer's output once and reuse it for every probe; each probe needs a
	// different slice of the same text.
	logText := r.importLogText()
	repeated, count := mostRepeatedError(logText, "")
	how := what + " wait expired"
	switch res.outcome {
	case waitRepeatingError:
		how = fmt.Sprintf("%s wait concluded early after %.0fs on a repeating importer error", what, res.elapsed.Seconds())
	case waitNoOutput:
		how = fmt.Sprintf("%s wait concluded early after %.0fs of total silence from every "+
			"command in the pipeline", what, res.silence.Seconds())
	}

	// Attribution first: if exactly one probe's table is named by the repeating error,
	// that probe is the poison and everyone else is collateral.
	culprit := ""
	if res.outcome == waitRepeatingError {
		if id, ok := attributeCrashLoop(r.active, res.quotedError); ok {
			culprit = id
			r.quarantine(id, res)
		}
	}

	for _, p := range r.active {
		o := r.observe(p)
		o.waitTimedOut = true
		o.waitNote = appendNote(o.waitNote, how)
		if culprit != "" && p.ID != culprit {
			o.channelWedgedBy = culprit
			continue
		}
		// Prefer an error that names this probe's table: that attributes the stall.
		if perTable, n := mostRepeatedError(logText, p.tableName()); n >= 2 {
			o.stuckDetail = fmt.Sprintf("%s repeated x%d after %s", perTable, n, how)
		} else if repeated != "" && count >= sweepCrashLoopRepeats {
			o.stuckDetail = fmt.Sprintf("%s repeated x%d after %s", repeated, count, how)
		}
	}
}

// recordCommandExit turns "a command this wait depended on is gone" into per-probe
// evidence. The evidence rules are the harness's usual ones, applied to a new signal:
//
//   - a CLEAN exit is not a failure. Nothing about a datatype follows from a process that
//     finished and left, so every probe is INCONCLUSIVE and nothing is quoted.
//   - an UNCLEAN exit is a failure of the pipeline, but it is a failure of a DATATYPE only
//     if the log carries a quotable import-failure signature. Same rule as the crash-loop
//     path, with one difference that is the entire point of this path: the required repeat
//     count is ONE, because a process that dies on the first value it cannot bind reports
//     that value exactly once before it goes.
//   - an export-side death is routed through the export machinery, so it reaches the same
//     EXPORTER_CRASHES verdict and the same publish rules as one found in the log.
func (r *sweepRun) recordCommandExit(what string, res waitResult) {
	how := fmt.Sprintf("%s exited during the %s wait after %.0fs",
		res.goneCommand, what, res.elapsed.Seconds())

	if res.goneErr == nil {
		r.markCommandExited(fmt.Sprintf("%s, before the expected counts arrived; a clean exit "+
			"is not a failure, so nothing is claimed about this type", how), how)
		return
	}

	if strings.Contains(strings.ToLower(res.goneCommand), "export") {
		// exportDiedDuring establishes the death from the export log and records it
		// against the culprit, exactly as a death found by the log-watching path is.
		if r.exportDiedDuring(how) {
			for _, p := range r.active {
				o := r.observe(p)
				o.waitTimedOut = true
				o.waitNote = appendNote(o.waitNote, how)
			}
			return
		}
		r.markCommandExited(r.exportAbortReason(how, res.goneErr), how)
		return
	}

	logText := r.importLogText()
	quoted, count := mostRepeatedError(logText, "")
	reason := r.importAbortReason(how, res.goneErr)
	if quoted == "" {
		// The process died and left nothing quotable. Real, but not attributable to a
		// datatype - and importAbortReason says so in as many words.
		r.markCommandExited(reason, how)
		return
	}

	culprit := ""
	if id, ok := attributeCrashLoop(r.active, quoted); ok {
		culprit = id
		r.quarantine(id, res)
	}
	for _, p := range r.active {
		o := r.observe(p)
		o.waitTimedOut = true
		o.commandExited = true
		o.commandExitDetail = reason
		o.waitNote = appendNote(o.waitNote, how)
		if culprit != "" && p.ID != culprit {
			o.channelWedgedBy, o.channelWedgedHow = culprit, "killed "+res.goneCommand
			continue
		}
		if perTable, n := mostRepeatedError(logText, p.tableName()); n >= 1 {
			o.stuckDetail = fmt.Sprintf("%s (x%d) - %s", perTable, n, how)
		} else if culprit == "" {
			// The error names no probe's table, so it is recorded against every probe
			// rather than pinned on one that may have nothing to do with it.
			o.stuckDetail = fmt.Sprintf("%s (x%d) - %s", quoted, count, how)
		}
	}
}

// markCommandExited records a command's exit as an environment fact on every active probe:
// the wait is over, but nothing about any datatype follows from it.
func (r *sweepRun) markCommandExited(detail, how string) {
	r.t.Logf("COMMAND EXITED: %s", detail)
	for _, p := range r.active {
		o := r.observe(p)
		o.waitTimedOut = true
		o.commandExited = true
		o.commandExitDetail = detail
		o.waitNote = appendNote(o.waitNote, how)
	}
}

// probeTableOf returns the table name of one probe id, or "" when the batch has no such
// probe - used to narrow a log scan to the probe under discussion.
func probeTableOf(probes []datatypeProbe, probeID string) string {
	for _, p := range probes {
		if p.ID == probeID {
			return p.tableName()
		}
	}
	return ""
}

// attributeCrashLoop names the probe responsible for a repeating error, which is possible
// only when EXACTLY ONE active probe's table appears in it. Two matches (or none) means
// the error does not identify a culprit, and guessing one would quarantine an innocent
// type - worse than not quarantining at all, because the guess is recorded as a finding.
func attributeCrashLoop(probes []datatypeProbe, errText string) (string, bool) {
	lower := strings.ToLower(errText)
	var matched []string
	for _, p := range probes {
		if p.ExpectVerdict != "" {
			continue // never blame a known-good control
		}
		if strings.Contains(lower, strings.ToLower(p.tableName())) {
			matched = append(matched, p.ID)
		}
	}
	if len(matched) != 1 {
		return "", false
	}
	return matched[0], true
}

// quarantine records that one probe wedged the channel: greppable for the runner, and the
// exact command that measures it in isolation.
//
// It does NOT re-run the rest of the batch in-process. Once a value has wedged the ordered
// channel, its event is still at the head of the queue and the importer will retry it
// forever, so the remaining probes can only be measured by a FRESH migration - see
// "Quarantine and continue" in DATATYPE_SWEEP.md for exactly what that would take.
func (r *sweepRun) quarantine(probeID string, res waitResult) {
	r.quarantined = append(r.quarantined, probeID)
	typeName := probeID
	for _, p := range r.active {
		if p.ID == probeID {
			typeName = p.TypeName
		}
	}
	// What the culprit did, in the words that fit: a wedged channel is still being
	// retried by a live importer, whereas a killed command is simply gone.
	did, cause := "wedged the import channel", res.quotedError
	if res.outcome == waitProcessGone {
		did = "killed " + res.goneCommand
		if quoted, _ := mostRepeatedError(r.importLogText(), probeTableOf(r.active, probeID)); quoted != "" {
			cause = quoted
		} else {
			cause = res.exitDescription()
		}
	}
	fmt.Printf("PROBE-RUN-QUARANTINE: %s | %s | %s (%s) %s after %.0fs: %s; "+
		"every other probe in this batch is collateral and must be re-run without it. "+
		"Measure it on its own with PROBE_ID=%s PROBE_MODE=%s -run TestDatatypeSweepSuspect\n",
		r.batch, r.mode, probeID, typeName, did, res.elapsed.Seconds(),
		sanitizeDetail(cause), probeID, r.mode)
	r.t.Logf("quarantined probe %s: it %s", probeID, did)
}

// ============================================================
// MEASUREMENT: value comparison
// ============================================================

type comparePhase string

const (
	phaseSnapshot  comparePhase = "snapshot"
	phaseStreaming comparePhase = "streaming"
)

// compareInto reads (id, compareExpr) from both sides for every active probe and records
// the first discrepancy per probe.
func (r *sweepRun) compareInto(from, to dbSide, phase comparePhase) {
	for _, p := range r.active {
		src, srcErr := r.fetchProbeValues(from, p)
		dst, dstErr := r.fetchProbeValues(to, p)

		var verdict, detail string
		switch {
		case srcErr != nil:
			detail = fmt.Sprintf("%s: cannot read %s: %v", phase, from, srcErr)
			verdict = verdictSilentWrong
		case dstErr != nil:
			// The destination table being unreadable after a "successful" migration is
			// exactly the silent-loss shape.
			detail = fmt.Sprintf("%s: cannot read %s: %v", phase, to, dstErr)
			verdict = verdictSilentLoss
		default:
			verdict, detail = compareProbeRows(src, dst)
			if verdict != "" {
				detail = fmt.Sprintf("%s %s->%s: %s", phase, from, to, detail)
			}
		}

		o := r.observe(p)
		// Record that this probe was actually compared in this phase, independently of
		// whether a difference was found. A clean compare records no verdict, so this
		// flag is the only evidence separating "identical" from "never measured".
		if phase == phaseSnapshot {
			o.snapshotCompared = true
		} else {
			o.streamCompared = true
		}
		if p.RecordDestValue && dstErr == nil {
			o.destSample = sampleValues(src, dst)
		}
		// Structured values for the PROBE-VALUES line. Forward direction only: in
		// FALL-BACK/FALL-FORWARD compareInto is also called with from=target, and
		// labelling the target's own value "source" would invert the report's columns.
		if from == sideSource && srcErr == nil && dstErr == nil {
			o.srcValue, o.dstValue = baselineValues(src, dst)
		}
		if verdict == "" {
			continue
		}
		if phase == phaseSnapshot {
			if o.snapshotVerdict == "" {
				o.snapshotVerdict, o.snapshotDetail = verdict, detail
			}
		} else if o.streamVerdict == "" {
			o.streamVerdict, o.streamDetail = verdict, detail
		}
	}
}

// probeRow is one row of a probe table as seen from one side: the neighbour column
// (`filler`) and the value under test, both rendered through the probe's compare
// expression.
type probeRow struct {
	filler sql.NullString
	value  sql.NullString
}

// rowLabel names the op a row id belongs to, so the detail says what actually broke
// rather than just which id disagreed.
func rowLabel(id int) string {
	switch id {
	case rowBaseline, revBaseline:
		return "update-this-column"
	case rowNullSeed, revNullSeed:
		return "NULL->value"
	case rowToDelete, revToDelete:
		return "delete"
	case rowOtherCol, revOtherCol:
		return "update-other-column"
	case rowToNull, revToNull:
		return "value->NULL"
	case rowNewInsert, revNewInsert:
		return "insert"
	default:
		return "snapshot"
	}
}

// baselineValues returns the raw source and destination text of the value under test for
// the baseline row, using the same vocabulary as sampleValues ("NULL" for a SQL NULL,
// "<row absent>" for a row that is not there) so a reader of either does not have to
// learn two conventions.
//
// Unlike sampleValues this returns the two texts UNQUOTED and separate, because they are
// destined for their own fields on the PROBE-VALUES line rather than for prose.
func baselineValues(src, dst map[int]probeRow) (string, string) {
	render := func(m map[int]probeRow, id int) string {
		row, ok := m[id]
		if !ok {
			return "<row absent>"
		}
		if !row.value.Valid {
			return "NULL"
		}
		return row.value.String
	}
	for _, id := range []int{rowBaseline, revBaseline} {
		_, inSrc := src[id]
		_, inDst := dst[id]
		if !inSrc && !inDst {
			continue
		}
		return render(src, id), render(dst, id)
	}
	return "", ""
}

// sampleValues renders the verbatim source and destination text of the value under test
// for the rows that always carry one, so the report can show whether e.g. an array
// delimiter survived rather than only that the comparison failed.
func sampleValues(src, dst map[int]probeRow) string {
	render := func(m map[int]probeRow, id int) string {
		row, ok := m[id]
		if !ok {
			return "<row absent>"
		}
		if !row.value.Valid {
			return "NULL"
		}
		return quoteVal(row.value.String)
	}
	var parts []string
	for _, id := range []int{rowBaseline, revBaseline} {
		_, inSrc := src[id]
		_, inDst := dst[id]
		if !inSrc && !inDst {
			continue
		}
		parts = append(parts, fmt.Sprintf("id=%d source=%s destination=%s",
			id, render(src, id), render(dst, id)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "verbatim: " + strings.Join(parts, " ")
}

func (r *sweepRun) fetchProbeValues(side dbSide, p datatypeProbe) (map[int]probeRow, error) {
	out := map[int]probeRow{}
	query := fmt.Sprintf("SELECT id, filler, (%s) FROM %s ORDER BY id",
		p.expandTemplate(p.compareExpr(), r.schema), p.qualifiedTable(r.schema))
	err := r.withConn(side)(func(db *sql.DB) error {
		rows, err := db.Query(query)
		if err != nil {
			return goerrors.Errorf("query %q: %w", truncate(query, 160), err)
		}
		defer rows.Close()
		for rows.Next() {
			var id int
			var row probeRow
			if err := rows.Scan(&id, &row.filler, &row.value); err != nil {
				return goerrors.Errorf("scan: %w", err)
			}
			out[id] = row
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// compareProbeRows implements PROBE_SPEC.md §1/§2 fidelity:
//
//	row present on source, absent on destination -> SILENT_LOSS
//	`filler` differs                             -> SILENT_LOSS (the row-level change
//	                                                never landed at all)
//	value on source, NULL on destination         -> SILENT_LOSS
//	NULL on source, value on destination         -> SILENT_WRONG
//	different values                             -> SILENT_WRONG
//	row deleted on source, still on destination  -> SILENT_WRONG (stale)
//
// `filler` is checked before `v` on purpose. It is a plain text column that must always
// replicate, so a filler mismatch means the change event for that row never arrived -
// a different diagnosis from "the event arrived but column v was not in it", which is
// what the queue scan detects. Rows are walked in id order so the detail is
// deterministic.
func compareProbeRows(src, dst map[int]probeRow) (string, string) {
	ids := make([]int, 0, len(src))
	for id := range src {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		s := src[id]
		d, ok := dst[id]
		if !ok {
			return verdictSilentLoss, fmt.Sprintf("[%s] row id=%d present on source, missing on destination",
				rowLabel(id), id)
		}
		if nullStringDiffers(s.filler, d.filler) {
			return verdictSilentLoss, fmt.Sprintf(
				"[%s] id=%d neighbour column filler source=%s destination=%s: the row change never landed",
				rowLabel(id), id, renderNullString(s.filler), renderNullString(d.filler))
		}
		switch {
		case s.value.Valid && !d.value.Valid:
			return verdictSilentLoss, fmt.Sprintf("[%s] id=%d source=%s destination=NULL",
				rowLabel(id), id, quoteVal(s.value.String))
		case !s.value.Valid && d.value.Valid:
			return verdictSilentWrong, fmt.Sprintf("[%s] id=%d source=NULL destination=%s",
				rowLabel(id), id, quoteVal(d.value.String))
		case s.value.Valid && d.value.Valid && s.value.String != d.value.String:
			return verdictSilentWrong, fmt.Sprintf("[%s] id=%d source=%s destination=%s",
				rowLabel(id), id, quoteVal(s.value.String), quoteVal(d.value.String))
		}
	}

	extra := make([]int, 0)
	for id := range dst {
		if _, ok := src[id]; !ok {
			extra = append(extra, id)
		}
	}
	if len(extra) > 0 {
		sort.Ints(extra)
		return verdictSilentWrong, fmt.Sprintf("[%s] stale row id=%d still on destination after source DELETE",
			rowLabel(extra[0]), extra[0])
	}
	return "", ""
}

func nullStringDiffers(a, b sql.NullString) bool {
	if a.Valid != b.Valid {
		return true
	}
	return a.Valid && a.String != b.String
}

func renderNullString(s sql.NullString) string {
	if !s.Valid {
		return "NULL"
	}
	return quoteVal(s.String)
}

// ============================================================
// MEASUREMENT: does the column appear in the event stream at all?
// ============================================================

// queueEvent mirrors the on-disk shape of a queue segment line (see
// tgtdb.Event.UnmarshalJSON). Decoded locally so the harness never depends on
// unexported behaviour of the production type.
type queueEvent struct {
	Op           string             `json:"op"`
	SchemaName   string             `json:"schema_name"`
	TableName    string             `json:"table_name"`
	Key          map[string]*string `json:"key"`
	Fields       map[string]*string `json:"fields"`
	BeforeFields map[string]*string `json:"before_fields"`
}

// recordQueueColumnPresence walks the queue segments under
// <GetCurrentExportDir()>/data/queue and records, per probe, how many events mention the
// probe's table and whether the column under test ever appears as a key in `fields` or
// `before_fields`. A column absent from every event is the signature of connector
// omission or guardrail exclusion (PROBE_SPEC.md §3).
//
// QUEUE_DIR_NAME is "queue" (cmd/eventQueue.go); the segment files are NDJSON named
// segment.<n>.ndjson.
func (r *sweepRun) recordQueueColumnPresence() {
	if !r.mode.hasCDC() {
		return
	}
	queueDir := filepath.Join(r.lm.GetCurrentExportDir(), "data", cmd.QUEUE_DIR_NAME)
	files, err := filepath.Glob(filepath.Join(queueDir, "segment.*.ndjson"))
	if err != nil || len(files) == 0 {
		note := fmt.Sprintf("no queue segments under %s", queueDir)
		if err != nil {
			note = fmt.Sprintf("cannot list queue segments under %s: %v", queueDir, err)
		}
		for _, p := range r.active {
			r.observe(p).queueScanNote = note
		}
		return
	}
	sort.Strings(files)

	byTable := map[string]int{}
	colSeen := map[string]bool{}
	for _, f := range files {
		if err := scanQueueSegment(f, byTable, colSeen); err != nil {
			r.t.Logf("queue segment %s: %v", f, err)
		}
	}

	for _, p := range r.active {
		o := r.observe(p)
		key := normalizeTableName(p.tableName())
		o.eventsForTable = byTable[key]
		o.columnSeenInEvents = colSeen[key]
	}
}

// scanQueueSegment accumulates per-table event counts and whether column "v" appeared.
func scanQueueSegment(path string, byTable map[string]int, colSeen map[string]bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// Event payloads can be large (wide arrays, long numerics); bufio's 64KiB default
	// would silently truncate a segment scan into a parse error.
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line == `\.` {
			continue
		}
		var ev queueEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue // not an event line (segment headers / markers)
		}
		key := normalizeTableName(ev.TableName)
		if key == "" {
			continue
		}
		byTable[key]++
		if _, ok := ev.Fields[sweepColumnUnderTest]; ok {
			colSeen[key] = true
		}
		if _, ok := ev.BeforeFields[sweepColumnUnderTest]; ok {
			colSeen[key] = true
		}
	}
	return sc.Err()
}

// sweepColumnUnderTest is the name of the column every probe puts its type in.
const sweepColumnUnderTest = "v"

// normalizeTableName reduces `"schema"."tbl"`, `schema.tbl` and `tbl` to a bare
// lower-case table name so event payloads and probe names can be matched.
func normalizeTableName(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), `"`, "")
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(s)
}

// ============================================================
// MEASUREMENT: was the user told?
// ============================================================

// The exact strings printed by handleUnsupportedColumnsInExportData (cmd/exportData.go).
const (
	unsupportedColsHeader   = "The following columns data export is unsupported:"
	unsupportedColsPrompt   = "Do you want to continue with the export by ignoring just these columns' data"
	unsupportedColsAccepted = "Continuing with the export by ignoring just these columns' data."
)

// recordExportWarnings greps the captured export stdout/stderr for the exclusion notice
// and records, per probe, whether that notice named this probe's table AND column.
//
// Note on the prompt: utils.AskPrompt returns true WITHOUT printing anything when
// --yes is in effect, and every export in this harness passes --yes. So in practice the
// header and the "Continuing with the export..." acceptance line appear while the
// question itself does not - which is exactly the QUIET_DROP shape from PROBE_SPEC.md
// ("the user was only warned in a prompt that --yes auto-accepts").
func (r *sweepRun) recordExportWarnings() {
	text := strings.Join([]string{
		r.lm.GetExportCommandStdout(),
		r.lm.GetExportCommandStderr(),
	}, "\n")
	promptShown := strings.Contains(text, unsupportedColsPrompt)
	for _, p := range r.active {
		if exportWarnedAboutColumn(text, p.tableName(), sweepColumnUnderTest) {
			o := r.observe(p)
			o.warned = true
			o.promptShown = promptShown
		}
	}
}

// exportWarnedAboutColumn looks for a line inside the unsupported-columns block that
// names both the table and the column. The block is printed as
//
//	The following columns data export is unsupported:
//	schema.table: [col1 col2]
//	...
func exportWarnedAboutColumn(text, table, column string) bool {
	idx := strings.Index(text, unsupportedColsHeader)
	if idx < 0 {
		return false
	}
	block := text[idx+len(unsupportedColsHeader):]
	if end := strings.Index(block, unsupportedColsAccepted); end >= 0 {
		block = block[:end]
	}
	if end := strings.Index(block, unsupportedColsPrompt); end >= 0 {
		block = block[:end]
	}
	lowerTable := strings.ToLower(table)
	for _, line := range strings.Split(block, "\n") {
		l := strings.ToLower(line)
		if strings.Contains(l, lowerTable) && strings.Contains(l, strings.ToLower(column)) {
			return true
		}
	}
	return false
}

// ============================================================
// MEASUREMENT: is the importer wedged?
// ============================================================

var (
	sqlstateRe  = regexp.MustCompile(`(?i)SQLSTATE[ :=]*([0-9A-Za-z]{5})`)
	errorLineRe = regexp.MustCompile(`(?i)\b(error|fatal|panic|exception|failed)\b`)
	// noise stripped before grouping "the same error" together
	timestampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?`)
	numberRe    = regexp.MustCompile(`\b\d+\b`)
	hexRe       = regexp.MustCompile(`0x[0-9a-fA-F]+`)

	// Signature matching for "is this line a real import failure". Keyword matching on
	// /error/ was tried twice and failed twice: `Error: (string) ""` and
	// `PKConflictAction: (string) (len=12) "ERROR-POLICY"` are both struct dumps, the
	// second one a CONFIG FIELD whose VALUE merely contains the word ERROR, and both
	// produced batches full of bogus STUCK verdicts.
	pgErrorRe       = regexp.MustCompile(`ERROR:\s+\S+`)
	importContextRe = regexp.MustCompile(`(?i)(import batch|flow=|step=|dbcontext=|executing batch|on channel \d+)`)
	// Struct-dump shapes: `(string)`, `(len=12)`, `(int64)`, and the `Field: (` opener.
	spewTypeRe  = regexp.MustCompile(`\((?:string|int\d*|uint\d*|bool|float\d*|nil|len=\d+)\)`)
	spewFieldRe = regexp.MustCompile(`^\s*\w+:\s*\(`)
	quotedRe    = regexp.MustCompile(`"[^"]*"|'[^']*'`)
)

// importLogText gathers everything the importer said: the captured stdout/stderr of the
// import commands plus the log files under <export-dir>/logs.
func (r *sweepRun) importLogText() string {
	var b strings.Builder
	b.WriteString(r.lm.GetImportCommandStdout())
	b.WriteString("\n")
	b.WriteString(r.lm.GetImportCommandStderr())
	b.WriteString("\n")
	b.WriteString(r.lm.GetImportToSourceCommandStderr())
	b.WriteString("\n")
	b.WriteString(r.importLogFileText())
	return b.String()
}

// importLogFileText is the ON-DISK half of importLogText: <export-dir>/logs only, with the
// in-memory command buffers left out.
//
// The wait loop uses this one rather than importLogText because it reads the log every two
// seconds for as long as the wait lasts, and VoyagerCommandRunner.Stdout()/Stderr() return
// bytes.Buffer.String() on a buffer that the running command's copier goroutine is
// concurrently appending to. Reading it a handful of times after a wait (which is what the
// post-hoc evidence does) is one thing; reading it 450 times during one is another, and a
// torn read would corrupt the very error text the verdict is quoted from. The importer
// writes the same batch errors to its own log file, so nothing is lost.
func (r *sweepRun) importLogFileText() string {
	var b strings.Builder
	logDir := filepath.Join(r.lm.GetCurrentExportDir(), "logs")
	files, err := filepath.Glob(filepath.Join(logDir, "*.log"))
	if err != nil {
		return b.String()
	}
	sort.Strings(files)
	for _, f := range files {
		base := strings.ToLower(filepath.Base(f))
		if !strings.Contains(base, "import") && !strings.Contains(base, "debezium") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		// Only the tail matters for "keeps repeating the same error".
		const tail = 512 * 1024
		if len(data) > tail {
			data = data[len(data)-tail:]
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String()
}

// mostRepeatedError groups error-shaped lines by their noise-stripped form and returns
// the raw text of the most frequent group (with its SQLSTATE hoisted into the detail).
// isImportFailureSignature reports whether a log line is a real import failure that may
// be quoted as evidence of a wedged importer.
//
// This is signature matching, not keyword matching, on purpose. A genuine failure in
// these logs looks like:
//
//	import batch: "...": flow=copy_normal: step=copy: ERROR: DECIMAL does not support
//	NaN yet (SQLSTATE 0A000): dbcontext=[...]
//
// so a candidate is accepted only when it carries a SQLSTATE, or a `ERROR: <something>`
// together with one of voyager's batch-context breadcrumbs. Struct-dump shapes are
// rejected outright, as is a line whose only "error" text sits inside quotes - there it
// is a field VALUE, not a message.
func isImportFailureSignature(line string) bool {
	if spewTypeRe.MatchString(line) || spewFieldRe.MatchString(line) {
		return false
	}
	// If the error word survives only inside a quoted string, it is a value.
	unquoted := quotedRe.ReplaceAllString(line, `""`)
	if !errorLineRe.MatchString(unquoted) && !sqlstateRe.MatchString(unquoted) {
		return false
	}
	if sqlstateRe.MatchString(line) {
		return true
	}
	return pgErrorRe.MatchString(line) && importContextRe.MatchString(line)
}

// mostRepeatedError returns the quoted text of the most repeated error and its count.
func mostRepeatedError(text, mustContain string) (string, int) {
	quoted, _, n := mostRepeatedErrorDetail(text, mustContain)
	return quoted, n
}

// mostRepeatedErrorDetail also returns the NOISE-STRIPPED signature the group was keyed
// on. The wait loop compares that across polls rather than the quoted sample: the sample
// is one raw line and carries its own timestamp and batch numbers, so two polls looking at
// the same crash-loop would otherwise compare unequal the moment the log tail rolls.
func mostRepeatedErrorDetail(text, mustContain string) (string, string, int) {
	counts := map[string]int{}
	sample := map[string]string{}
	needle := strings.ToLower(mustContain)

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !isImportFailureSignature(line) {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(line), needle) {
			continue
		}
		norm := timestampRe.ReplaceAllString(line, "<ts>")
		norm = hexRe.ReplaceAllString(norm, "<hex>")
		norm = numberRe.ReplaceAllString(norm, "<n>")
		counts[norm]++
		if _, ok := sample[norm]; !ok {
			sample[norm] = line
		}
	}

	best, bestN := "", 0
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic tie-breaking
	for _, k := range keys {
		if counts[k] > bestN {
			best, bestN = k, counts[k]
		}
	}
	if bestN == 0 {
		return "", "", 0
	}
	raw := sample[best]
	if m := sqlstateRe.FindStringSubmatch(raw); m != nil {
		return fmt.Sprintf("SQLSTATE %s: %s", m[1], truncate(raw, 200)), best, bestN
	}
	return truncate(raw, 220), best, bestN
}

// ============================================================
// MEASUREMENT: did the EXPORTER die?
// ============================================================

/*
The import side has had a failure detector since the crash-loop work; the export side had
none, and that was a hole big enough to hide the worst outcome in the audit in.

When the exporter dies, `export data` prints exactly one thing:

	Export of data failed! Check <dir>/logs for more details.

and exits. Every downstream measurement then reads as "nothing happened": zero events, no
repeating importer error, no value difference to compare. So the classifier concluded
INCONCLUSIVE - "the run was healthy and nothing conclusive happened" - for a run in which
voyager's exporter was killed by a datatype and NOTHING would have migrated. DOM-005
(domain over an enum) is the known instance: the Debezium connector throws

	java.lang.NullPointerException: Cannot invoke "java.sql.Array.getArray()" because the
	return value of "java.sql.ResultSet.getArray(String)" is null
	  at io.debezium.connector.postgresql.TypeRegistry.createTypeBuilderFromResultSet

while priming its type registry, at startup, before a single row is read - and
`initiate cutover` then hangs forever.

The evidence rules are the import side's, unchanged: no EXPORTER_CRASHES verdict without a
quotable cause, and "export failed but nothing usable is in any log" is said out loud
rather than dressed up as either a verdict or a clean run.
*/

var (
	// Terminal markers. Each one means the export side is GONE, not degraded: the
	// connector task ended in failure, or `export data` printed its own death notice.
	// Only these establish a death - a Java exception on its own does not, because
	// Debezium logs retriable exceptions it then recovers from, and quarantining a
	// datatype on one of those would record a guess as a finding.
	exportConnectorFailedRe = regexp.MustCompile(`(?i)Connector completed:\s*success\s*=\s*'false'`)
	exportTaskFailedRe      = regexp.MustCompile(`(?i)Unable to initialize and start connector's task class`)
	exportFailedRe          = regexp.MustCompile(`(?i)Export of data failed`)

	// A fully-qualified Java throwable: at least one lower-case package segment, then a
	// capitalised class ending in Exception/Error/Throwable. The package requirement is
	// what keeps prose ("...if the exception is retried...") out, and the \b at the end
	// is what keeps `ExceptionHandler` out.
	javaExceptionRe = regexp.MustCompile(`\b(?:[a-z][A-Za-z0-9_]*\.)+[A-Z][A-Za-z0-9_$]*(?:Exception|Error|Throwable)\b`)

	// Context that marks a line as reporting the throwable rather than merely naming it.
	exportErrorContextRe = regexp.MustCompile(`(?i)(\berror\b|\bfatal\b|\bsevere\b|caused by|error\s*=)`)
)

// exportLogFileText is the ON-DISK export-side evidence: <export-dir>/logs, restricted to
// the export and Debezium logs. The Debezium one is the important half - the connector
// writes its startup exception to debezium-<role>.log (source_db_exporter for the forward
// direction, target_db_exporter_fb/_ff after cutover) and `export data` itself only says
// "check the logs".
//
// Like importLogFileText this reads FILES only, with no in-memory command buffers, because
// the wait loop calls it every two seconds while the command's copier goroutine is
// appending to those buffers, and a torn read would corrupt the very exception text the
// verdict is quoted from.
func (r *sweepRun) exportLogFileText() string {
	var b strings.Builder
	logDir := filepath.Join(r.lm.GetCurrentExportDir(), "logs")
	files, err := filepath.Glob(filepath.Join(logDir, "*.log"))
	if err != nil {
		return b.String()
	}
	sort.Strings(files)
	for _, f := range files {
		base := strings.ToLower(filepath.Base(f))
		if !strings.Contains(base, "export") && !strings.Contains(base, "debezium") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		// A connector that dies at startup does so near the START of its log, and it
		// then writes nothing more - but a connector that dies after hours of streaming
		// dies at the END. Both tails are cheap, so take a generous one.
		const tail = 512 * 1024
		if len(data) > tail {
			data = data[len(data)-tail:]
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String()
}

// exportLogText is the full export-side evidence: the captured stdout/stderr of both
// export commands plus the on-disk logs. Used post-hoc, where a torn buffer read cannot
// corrupt a live wait.
func (r *sweepRun) exportLogText() string {
	var b strings.Builder
	b.WriteString(r.lm.GetExportCommandStdout())
	b.WriteString("\n")
	b.WriteString(r.lm.GetExportCommandStderr())
	b.WriteString("\n")
	b.WriteString(r.lm.GetExportFromTargetCommandStdout())
	b.WriteString("\n")
	b.WriteString(r.lm.GetExportFromTargetCommandStderr())
	b.WriteString("\n")
	b.WriteString(r.exportLogFileText())
	return b.String()
}

// isExportCauseLine reports whether a line names the throwable that killed the exporter,
// and may therefore be quoted as the cause. A bare class name in a class-path dump is not
// enough: the line must either report it (ERROR / Caused by / error = ...) or BE the
// stack-trace header, which starts with the throwable itself.
func isExportCauseLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if spewTypeRe.MatchString(trimmed) || spewFieldRe.MatchString(trimmed) {
		return false
	}
	loc := javaExceptionRe.FindStringIndex(trimmed)
	if loc == nil {
		return false
	}
	return loc[0] == 0 || strings.HasPrefix(trimmed, "Caused by:") || exportErrorContextRe.MatchString(trimmed)
}

// exportFailureEvidence extracts the export side's cause of death from any mixture of
// command output and log files, and reports whether a death was established at all.
//
// The two halves are deliberately separate:
//
//   - a TERMINAL MARKER is what establishes the death. Without one there is no verdict,
//     however alarming the log looks - Debezium logs exceptions it recovers from.
//   - a CAUSE LINE is what gets quoted. The real NullPointerException carries the class
//     and the message, which is the whole value of the finding; falling back to the
//     marker alone ("Export of data failed!") says only that something died.
//
// The marker line frequently IS the cause line (Debezium's ConnectorLifecycle line
// carries error = 'java.lang.NullPointerException: ...'), in which case one quote does
// both jobs.
func exportFailureEvidence(text string) (string, bool) {
	var label, markerLine, markerCause, firstCause string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if l := exportMarkerLabel(line); l != "" && label == "" {
			label, markerLine = l, line
			// Debezium's ConnectorLifecycle line IS both marker and cause, but the
			// cause sits at the END of it, behind the whole connector config, so
			// quoting the line raw and truncating would throw the exception away.
			if isExportCauseLine(line) {
				markerCause = exportCauseText(line)
			}
		}
		if firstCause == "" && isExportCauseLine(line) {
			firstCause = exportCauseText(line)
		}
	}
	if label == "" {
		return "", false
	}
	// The connector's own account of what killed it beats anything found loose in the
	// log; failing that, the FIRST throwable is the root cause and the ones after it are
	// the shutdown path reacting to it.
	cause := markerCause
	if cause == "" {
		cause = firstCause
	}
	if cause == "" {
		return truncate(markerLine, 400), true
	}
	return truncate(label+" - "+truncate(cause, 320), 400), true
}

// exportMarkerLabel returns the terminal-failure marker a line carries, or "".
func exportMarkerLabel(line string) string {
	if spewTypeRe.MatchString(line) || spewFieldRe.MatchString(line) {
		return ""
	}
	for _, re := range []*regexp.Regexp{exportConnectorFailedRe, exportTaskFailedRe, exportFailedRe} {
		if m := re.FindString(line); m != "" {
			return m
		}
	}
	return ""
}

// exportCauseText narrows a line down to the throwable it carries: the class name and
// everything after it. Everything before is a log prefix and, on the ConnectorLifecycle
// line, the connector's entire config - noise that would push the actual exception past
// any sane truncation limit.
func exportCauseText(line string) string {
	line = strings.TrimSpace(line)
	if loc := javaExceptionRe.FindStringIndex(line); loc != nil && loc[0] > 0 {
		return strings.TrimRight(strings.TrimSpace(line[loc[0]:]), `'"`)
	}
	return line
}

// attributeExportFailure names the probe an export death can be blamed on, which is
// possible only when the failure text names EXACTLY ONE active probe - by its table or by
// its type. Two matches (or none) means the failure does not identify a culprit, and a
// guess here is worse than no attribution at all: it would pin a product verdict, the
// harshest in the vocabulary, on an innocent type.
//
// It is handed the failure EVIDENCE, never the whole log: a Debezium log routinely names
// every captured table, so matching over the log would "attribute" the death to whichever
// probe happened to sort first.
func attributeExportFailure(probes []datatypeProbe, evidence string) (string, bool) {
	lower := strings.ToLower(evidence)
	var matched []string
	for _, p := range probes {
		if p.ExpectVerdict != "" {
			continue // never blame a known-good control
		}
		names := []string{p.tableName(), p.TypeName}
		for _, n := range names {
			n = strings.TrimSpace(strings.ToLower(n))
			// Single-character or trivially short type names would match anything.
			if len(n) < 4 {
				continue
			}
			if strings.Contains(lower, n) {
				matched = append(matched, p.ID)
				break
			}
		}
	}
	if len(matched) != 1 {
		return "", false
	}
	return matched[0], true
}

// recordExportDeath is the export-side twin of quarantine + applyWaitResult's attribution
// step: it pins the verdict on the culprit when the failure names one, and marks every
// other active probe INCONCLUSIVE because an exporter that dies at startup takes the whole
// run down and nothing behind it was ever measured.
//
// It is idempotent: the first death is the one that killed the run, and a later poll
// re-reading the same log must not overwrite it.
func (r *sweepRun) recordExportDeath(cause, how string) {
	if r.exportDeath != "" {
		return
	}
	r.exportDeath = cause
	r.flaked = true
	r.t.Logf("EXPORT DIED (%s): %s", how, cause)

	culprit, attributed := attributeExportFailure(r.active, cause)
	detail := fmt.Sprintf("the export side died (%s): %s", how, cause)

	if attributed {
		r.exportDeathCulprit = culprit
		fmt.Printf("PROBE-RUN-EXPORT-DIED: %s | %s | %s killed the exporter: %s; "+
			"every other probe in this batch is collateral and must be re-run without it. "+
			"Measure it on its own with PROBE_ID=%s PROBE_MODE=%s -run TestDatatypeSweepSuspect\n",
			r.batch, r.mode, culprit, sanitizeDetail(cause), culprit, r.mode)
	} else {
		fmt.Printf("PROBE-RUN-EXPORT-DIED: %s | %s | the exporter died and the failure names no "+
			"probe, so it is reported against the RUN and every probe in it is inconclusive: %s\n",
			r.batch, r.mode, sanitizeDetail(cause))
	}

	for _, p := range r.active {
		o := r.observe(p)
		if attributed && p.ID == culprit {
			o.exporterDied = true
			o.exporterDetail = detail
			continue
		}
		o.exporterDiedInRun = detail
	}
}

// publishableReason returns why one probe's verdict may be recorded even though the run as
// a whole failed its control gate, or "" when it may not be.
//
// The control gate exists to catch a BROKEN MEASUREMENT: a known-good control coming out
// wrong means the harness, containers or environment were wrong, so nothing that run
// measured can be trusted. An attributed export death is a different thing entirely. It is
// not a broken measurement, it IS the finding - and the controls going inconclusive is a
// CONSEQUENCE of that finding (the exporter they needed was dead), not evidence against
// it. Requiring a human to promote such a row by hand would mean the audit's most severe
// verdict is the only one that cannot be recorded automatically.
//
// The carve-out is deliberately narrow, and every clause is load-bearing:
//
//   - the probe must BE the attributed culprit. A batch-mate was never measured, so it
//     stays INCONCLUSIVE and stays unpublishable;
//   - the verdict actually reached must be EXPORTER_CRASHES. The marker never promotes a
//     verdict the classifier did not produce;
//   - there must be a quotable cause. No evidence, no publication - the same rule that
//     governs the verdict itself.
//
// An UNATTRIBUTED death promotes nothing: the run-level PROBE-RUN-EXPORT-DIED line is the
// record, and a human writes that row from it. One row written by hand beats a wrong row
// written automatically.
func publishableReason(probeID, verdict, culprit, cause string) string {
	if culprit == "" || probeID != culprit || verdict != verdictExporterCrashes {
		return ""
	}
	if strings.TrimSpace(cause) == "" {
		return ""
	}
	return "the exporter died with a quotable cause attributed to this probe, so this " +
		"verdict stands on its own evidence; the control gate failing is a consequence of " +
		"this finding rather than evidence against it: " + cause
}

// exportDiedDuring inspects the export side after something went wrong and, if the death
// can be quoted, records it. Returns whether it did.
//
// Every caller that is about to conclude "nothing happened" goes through here first. That
// is the entire fix: the old code reached its conclusion from the IMPORT log alone, where
// a dead exporter is indistinguishable from a quiet run.
func (r *sweepRun) exportDiedDuring(how string) bool {
	cause, ok := exportFailureEvidence(r.exportLogText())
	if !ok {
		return false
	}
	r.recordExportDeath(cause, how)
	return true
}

// exportAbortReason is importAbortReason's mirror: it turns a bare "export data failed"
// into a reason that names the actual cause, and says so explicitly when it cannot.
//
// Used only for the paths where no export death could be established, so that the run
// still records WHY it is refusing to claim one.
func (r *sweepRun) exportAbortReason(what string, err error) string {
	return fmt.Sprintf("%s: %v; NO export-side death marker (a failed connector, a Java "+
		"exception or 'Export of data failed') was found in the export command "+
		"stdout/stderr, <export-dir>/logs/yb-voyager-export-data.log or the Debezium "+
		"exporter log - the cause is unrecorded", what, err)
}

// ============================================================
// CLASSIFICATION
// ============================================================

// decideVerdict maps one observation to one verdict. Ordering is causal, not merely
// worst-first: when the guardrail excluded the column, "the guardrail excluded it" is
// the informative answer even though the value comparison also shows a loss.
func decideVerdict(mode sweepMode, o probeObservation) (string, string) {
	verdict, detail := decideVerdictCore(mode, o)
	// destSample is only set for probes that asked for it (RecordDestValue).
	if verdict != verdictSkipped && o.destSample != "" {
		detail = withNote(detail, o.destSample)
	}
	return verdict, detail
}

func decideVerdictCore(mode sweepMode, o probeObservation) (string, string) {
	if o.settledVerdict != "" {
		return o.settledVerdict, o.settledDetail
	}

	// 0. The EXPORT side died. This is checked before everything below because a dead
	//    exporter makes every later signal meaningless in the same direction: no events,
	//    no importer error, no value difference. Read from the import side alone that is
	//    indistinguishable from a healthy run in which nothing interesting happened,
	//    which is exactly how the worst outcome in the audit used to come out
	//    INCONCLUSIVE. It also outranks runAbort (BLOCKS is "voyager stopped up front
	//    with a clear error", and a connector dying on a NullPointerException is not
	//    that) and exportNeverStreamed (a JVM that never came up says nothing about a
	//    type - but one that came up and then threw says a great deal).
	//
	//    The ONE thing it does not outrank is a wedged importer. If the importer is
	//    crash-looping on a quotable error, that error was produced BY this value
	//    reaching the target, so the import side has already measured this type and its
	//    verdict stands; the export process dying afterwards does not erase it.
	importWedged := o.waitTimedOut && o.stuckDetail != ""
	if !importWedged {
		if o.exporterDied {
			return verdictExporterCrashes, withNote(o.exporterDetail, o.queueScanNote, o.waitNote)
		}
		if o.exporterDiedInRun != "" {
			return verdictInconclusive, withNote(
				"the exporter died before this probe was measured, so nothing was ever produced "+
					"for this type and no claim can be made about it: "+o.exporterDiedInRun,
				o.queueScanNote, o.waitNote)
		}
	}

	if o.runAbort != "" {
		return verdictBlocks, o.runAbort
	}

	// Environment first: if export never reached streaming, nothing downstream measured
	// anything, and no product claim of any kind can be made.
	if o.exportNeverStreamed {
		return verdictInconclusive, withNote(o.flakeDetail, o.queueScanNote, o.waitNote)
	}

	// 1. Column never appeared in the event stream.
	if mode.hasCDC() && o.eventsForTable > 0 && !o.columnSeenInEvents {
		base := fmt.Sprintf("column %q absent from all %d exported events for this table",
			sweepColumnUnderTest, o.eventsForTable)
		switch {
		case o.warned && o.promptShown:
			return verdictExcludedTold, base + "; export printed the exclusion notice and asked before continuing"
		case o.warned:
			return verdictQuietDrop, base + "; exclusion notice printed but the question was auto-accepted by --yes"
		default:
			return verdictSilentLoss, base + "; no exclusion warning in export stdout/stderr"
		}
	}

	// 2. Snapshot-only modes have no queue, so the guardrail evidence is the warning.
	if !mode.hasCDC() && o.warned && (o.snapshotVerdict != "" || o.streamVerdict != "") {
		if o.promptShown {
			return verdictExcludedTold, "exclusion notice printed and confirmed before continuing; " + o.snapshotDetail
		}
		return verdictQuietDrop, "exclusion notice printed but auto-accepted by --yes; " + o.snapshotDetail
	}

	// 2b. Someone ELSE's value wedged the ordered import channel. Every event behind it -
	//     including all of this probe's - is stuck there, so nothing was measured about
	//     this type. Neither STUCK (that blames it for another type's poison) nor a value
	//     verdict (the comparison only shows how far the channel got) may be claimed.
	if o.channelWedgedBy != "" {
		how := o.channelWedgedHow
		if how == "" {
			how = "crash-looped the import channel"
		}
		return verdictInconclusive, withNote(fmt.Sprintf(
			"probe %s %s during this run, so every event for this table was stuck behind it "+
				"and this type was never actually exercised; re-run the batch without %s",
			o.channelWedgedBy, how, o.channelWedgedBy),
			o.queueScanNote, o.waitNote)
	}

	// 3. Wedged importer: a bounded wait expired AND the import log keeps repeating one
	//    REAL error. STUCK is only ever emitted when the error text can be quoted -
	//    "the wait expired" on its own is equally consistent with nothing having run.
	if o.waitTimedOut && o.stuckDetail != "" {
		return verdictStuck, o.stuckDetail
	}
	// 3a. A command the wait depended on is gone, and nothing quotable explains it (a
	//     clean exit, or an unclean one that left no import-failure signature). The wait
	//     rightly stopped - no counts can arrive from a dead process - but a dead process
	//     is not a datatype verdict, and everything measured after it stopped shows only
	//     how far the pipeline got. This is placed after 3 on purpose: when the log DOES
	//     carry the reason, that reason is the finding.
	if o.commandExited {
		return verdictInconclusive, withNote(o.commandExitDetail, o.queueScanNote, o.waitNote)
	}
	// 3b. Wait expired, no events at all, and no quotable importer error: nothing was
	//     retrying a bad event, there simply were no events. Environment, not product.
	if o.waitTimedOut && mode.hasCDC() && o.eventsForTable == 0 {
		return verdictInconclusive, withNote(
			"streaming wait expired with zero events for this table and no repeating importer "+
				"error in the import log: no event ever flowed, so this is an environment flake "+
				"rather than a product stall",
			o.queueScanNote, o.waitNote)
	}

	// 4. Value fidelity, snapshot before streaming.
	if o.snapshotVerdict != "" {
		return o.snapshotVerdict, withNote(o.snapshotDetail, o.queueScanNote, o.waitNote)
	}
	if o.streamVerdict != "" {
		return o.streamVerdict, withNote(o.streamDetail, o.queueScanNote, o.waitNote)
	}

	// 5. Positive evidence is required before any pass. Everything below returns
	//    WORKS, so an observation that was never populated must be intercepted here:
	//    a framework wait that t.Fatalf's unwinds the test goroutine and skips
	//    compareInto / applyDelta / recordQueueColumnPresence entirely, leaving an
	//    all-zero observation that would otherwise fall through as a clean pass.
	if !o.snapshotCompared {
		return verdictInconclusive, withNote(
			"no value comparison ran for this probe: the run aborted before any measurement, "+
				"so nothing about this type was actually exercised",
			o.queueScanNote, o.waitNote)
	}
	if mode.hasCDC() {
		if !o.streamCompared {
			return verdictInconclusive, withNote(
				"snapshot was compared but the post-streaming comparison never ran, "+
					"so no change op was exercised for this type",
				o.queueScanNote, o.waitNote)
		}
		// Zero events for the table means no change op ever reached the target, so the
		// post-streaming comparison passed only because nothing had changed.
		if o.eventsForTable == 0 {
			if o.deltaOpsApplied > 0 && o.deltaConfirmed {
				return verdictSilentLoss, withNote(fmt.Sprintf(
					"%d change ops applied and confirmed on the source, but zero CDC events "+
						"reached the queue for this table", o.deltaOpsApplied),
					o.queueScanNote, o.waitNote)
			}
			return verdictInconclusive, withNote(fmt.Sprintf(
				"zero CDC events for this table and the source-side delta could not be "+
					"confirmed (%d change ops accepted), so no change op was actually exercised",
				o.deltaOpsApplied), o.queueScanNote, o.waitNote)
		}
	}

	// 6. Data is identical but the report never reached the expected counts. Say so
	//    rather than pretending a clean WORKS or inventing a STUCK without evidence.
	if o.waitTimedOut {
		return verdictWorks, withNote("values identical; migration-report counts did not reach the expectation within the timeout",
			o.queueScanNote, o.waitNote)
	}

	// 7. Nothing to report.
	detail := "snapshot identical (offline is snapshot-only: no change ops apply)"
	if mode.hasCDC() {
		detail = fmt.Sprintf(
			"snapshot + insert/update(self)/update(other)/delete + NULL transitions all identical; "+
				"column present in the event stream (%d events for this table)", o.eventsForTable)
	}
	return verdictWorks, withNote(detail, o.queueScanNote, o.waitNote)
}

// ============================================================
// BOOKKEEPING AND OUTPUT
// ============================================================

func (r *sweepRun) observe(p datatypeProbe) *probeObservation {
	o, ok := r.obs[p.ID]
	if !ok {
		o = &probeObservation{}
		r.obs[p.ID] = o
	}
	return o
}

// settle pins a final verdict for a probe, short-circuiting classification.
// abandonProbe settles a probe that could not be set up AND removes whatever it managed
// to create, on every side.
//
// This matters far beyond tidiness. applyProbeDDL creates on the source first, so a
// probe the TARGET rejects leaves a fully-created source table that never reached the
// REPLICA IDENTITY FULL step. `export data` then scans the whole schema, finds that
// table, and refuses to start the ENTIRE migration:
//
//	Tables missing replica identity full: [sweep_schema."p_mrange_003"]
//	Migration cannot proceed without the required permissions and configurations.
//
// So one probe the target cannot host took down every batch-mate before streaming began.
// A skipped probe must leave nothing behind.
func (r *sweepRun) abandonProbe(p datatypeProbe, verdict, detail string) {
	r.settle(p, verdict, detail)
	drops := p.dropDDL(r.schema)
	for _, side := range r.sides() {
		if err := r.execOn(side, drops...); err != nil {
			r.t.Logf("probe %s: cleanup on %s left something behind: %v", p.ID, side, err)
		}
	}
}

func (r *sweepRun) settle(p datatypeProbe, verdict, detail string) {
	o := r.observe(p)
	if o.settledVerdict == "" {
		o.settledVerdict, o.settledDetail = verdict, detail
	}
	r.t.Logf("probe %s settled as %s: %s", p.ID, verdict, detail)
}

// abortAll records a run-level failure on every not-yet-settled probe. A migration that
// stops or hangs on something the harness cannot answer is BLOCKS per PROBE_SPEC.md.
// markExportNeverStreamed records the Debezium-boot flake against every active probe.
// It deliberately does NOT use abortAll: BLOCKS is a product verdict ("this type stops
// the migration"), and a JVM that did not come up says nothing about any type.
func (r *sweepRun) markExportNeverStreamed(reason string) {
	// ...unless it DID come up and then died. "The JVM never started" and "the connector
	// threw while priming its type registry" produce the same symptom here - streaming
	// mode is never reached - and only the export log tells them apart. Calling the
	// second one a flake is how an exporter killed by a datatype was reported as an
	// environment problem.
	if r.exportDiedDuring("found while diagnosing why the export never reached streaming mode") {
		return
	}
	r.flaked = true
	r.t.Logf("EXPORT NEVER STREAMED: %s", reason)
	for _, p := range r.active {
		o := r.observe(p)
		o.exportNeverStreamed = true
		o.flakeDetail = reason
	}
}

// importAbortReason turns a bare "import data failed" into a reason that names the
// actual database error.
//
// A non-zero exit from `import data` says only that the command exited; the cause lives
// in the importer's own output and in <export-dir>/logs. Reporting the exit status alone
// reproduces, inside the audit tooling, exactly the failure mode this audit exists to
// find: a failure that is real but leaves nothing quotable, and is therefore
// indistinguishable from noise. So the importer log is scanned with the same signature
// matcher used for wedged-importer evidence, and when nothing matches, the detail says
// so explicitly instead of leaving it implied.
func (r *sweepRun) importAbortReason(what string, err error) string {
	reason := fmt.Sprintf("%s: %v", what, err)
	if quoted, n := mostRepeatedError(r.importLogText(), ""); quoted != "" {
		if n > 1 {
			return fmt.Sprintf("%s; importer error (x%d): %s", reason, n, quoted)
		}
		return fmt.Sprintf("%s; importer error: %s", reason, quoted)
	}
	return reason + "; NO error line matching an import-failure signature was found in " +
		"the import command stdout/stderr or <export-dir>/logs - the cause is unrecorded"
}

// abortExport handles a failing `export data` command. A non-zero exit says only that the
// command exited; whether that was a datatype killing the connector or the environment
// falling over is decided by the export logs, so they are read BEFORE anything is
// concluded. When they carry a death marker this is an EXPORTER_CRASHES finding; when they
// carry nothing usable it is a run-level abort whose reason says so in as many words,
// rather than a failure that is real but leaves nothing quotable.
func (r *sweepRun) abortExport(what string, err error) {
	if r.exportDiedDuring(fmt.Sprintf("%s: %v", what, err)) {
		return
	}
	r.abortAll(r.exportAbortReason(what, err))
}

func (r *sweepRun) abortAll(reason string) {
	r.t.Logf("run aborted: %s", reason)
	for _, p := range r.probes {
		o := r.observe(p)
		if o.runAbort == "" {
			o.runAbort = reason
		}
	}
}

// emitAll prints exactly one greppable PROBE-RESULT line per probe, then enforces the
// known-answer checks. It runs from a defer so a mid-run abort still produces a full
// matrix instead of a hole.
func (r *sweepRun) emitAll() {
	if r.emitted {
		return
	}
	r.emitted = true

	// A poison probe can wedge the whole channel, so a control coming out STUCK in that
	// run is expected collateral rather than evidence of a broken harness. Downgrade the
	// known-answer check to a log line in that case.
	poisonInRun := false
	for _, p := range r.probes {
		if p.Poison {
			poisonInRun = true
		}
	}

	inconclusive := 0
	for _, p := range r.probes {
		o := r.observe(p)
		verdict, detail := decideVerdict(r.mode, *o)
		if p.Note != "" {
			detail = detail + " [" + p.Note + "]"
		}
		fmt.Printf("PROBE-RESULT: %s | %s | %s | %s | %s\n",
			p.ID, p.TypeName, r.mode, verdict, sanitizeDetail(detail))

		// The two values as FIELDS, on their own line, so the audit tooling never has to
		// parse them back out of the human-readable detail. Emitted only when the run
		// actually read both sides; an absent line means "not measured", which is a
		// different statement from "measured and empty".
		if o.srcValue != "" || o.dstValue != "" {
			fmt.Printf("PROBE-VALUES: %s | %s | %s | %s\n",
				p.ID, r.mode, sanitizeValue(o.srcValue), sanitizeValue(o.dstValue))
		}

		// The one carve-out from the control gate: an attributed export death is the
		// finding, not a broken measurement, so its own row is publishable even though
		// every other row from this run is not. See publishableReason.
		if why := publishableReason(p.ID, verdict, r.exportDeathCulprit, r.exportDeath); why != "" {
			fmt.Printf("PROBE-PUBLISHABLE: %s | %s | %s | %s\n",
				p.ID, r.mode, verdict, sanitizeDetail(why))
		}

		if verdict == verdictInconclusive {
			inconclusive++
		}

		if p.ExpectVerdict == "" || verdict == p.ExpectVerdict || verdict == verdictSkipped {
			continue
		}
		if poisonInRun {
			r.t.Logf("known-answer control %s (%s) came out %s instead of %s; a poison probe "+
				"shares this run, so this is most likely collateral from the wedged channel: %s",
				p.ID, p.TypeName, verdict, p.ExpectVerdict, detail)
			continue
		}
		r.t.Errorf("harness sanity check failed: probe %s (%s) expected %s, got %s (%s). "+
			"A known-answer control disagreeing means the harness is wrong, not the product.",
			p.ID, p.TypeName, p.ExpectVerdict, verdict, detail)
	}

	// THE PRIMARY GATE. Every classifier bug found so far - the false WORKS on an
	// unmeasured run, the empty-error STUCK, the config-field STUCK - showed up first
	// as a known-good control coming out wrong. So a run in which either control is not
	// WORKS is not a run with one bad probe in it: it is an invalid run, and none of its
	// verdicts may be recorded. The only exception is a run that deliberately contains a
	// poison probe, where a wedged control is the expected collateral.
	if poisonInRun {
		fmt.Printf("PROBE-RUN-POISON: %s | %s | poison probe in run, control gate not applicable\n",
			r.batch, r.mode)
	} else {
		for _, p := range r.probes {
			if p.ExpectVerdict == "" {
				continue
			}
			verdict, _ := decideVerdict(r.mode, *r.observe(p))
			if verdict != p.ExpectVerdict {
				fmt.Printf("PROBE-RUN-INVALID: %s | %s | known-good control %s came out %s, not %s"+
					" - the whole run is invalid and none of its verdicts may be recorded\n",
					r.batch, r.mode, p.ID, verdict, p.ExpectVerdict)
			}
		}
	}

	// One greppable line the runner uses to decide whether to re-run. A flake must
	// reproduce before it is recorded as anything at all.
	if r.flaked || inconclusive > 0 {
		reason := "probes came out INCONCLUSIVE"
		if r.flaked {
			reason = "export never reached streaming mode"
		}
		if len(r.quarantined) > 0 {
			reason = fmt.Sprintf("import channel wedged by %s; re-run this batch without it",
				strings.Join(r.quarantined, ", "))
		}
		// An export death outranks both of the above as the reason: it takes the whole
		// run down, so every other probe's INCONCLUSIVE is its collateral and re-running
		// the batch unchanged will only reproduce it.
		if r.exportDeath != "" {
			reason = "the exporter died during this run, so nothing was produced for any " +
				"probe behind it: " + r.exportDeath
		}
		fmt.Printf("PROBE-RUN-FLAKE: %s | %s | %d inconclusive | %s\n",
			r.batch, r.mode, inconclusive, reason)
	}
}

// sanitizeValue makes a VALUE safe for a one-line, pipe-separated field without
// corrupting it.
//
// sanitizeDetail rewrites "|" to "/" and collapses runs of whitespace, which is fine for
// prose but not for a value: a text probe may legitimately contain a pipe, a newline or a
// run of spaces, and silently rewriting those would falsify the very bytes the field
// exists to record. So the structural characters are escaped REVERSIBLY instead.
func sanitizeValue(s string) string {
	s = strings.NewReplacer(
		`\`, `\\`,
		"|", `\x7c`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	).Replace(s)
	return truncate(s, 400)
}

// sanitizeDetail keeps the detail field on one line and free of the field separator.
func sanitizeDetail(s string) string {
	s = strings.NewReplacer("|", "/", "\n", " ", "\r", " ", "\t", " ").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "-"
	}
	return truncate(s, 400)
}

func withNote(detail string, notes ...string) string {
	for _, n := range notes {
		if strings.TrimSpace(n) != "" {
			detail = detail + "; " + n
		}
	}
	return detail
}

func appendNote(existing, add string) string {
	if strings.TrimSpace(existing) == "" {
		return add
	}
	if strings.Contains(existing, add) {
		return existing
	}
	return existing + "; " + add
}

// truncate cuts to at most n runes. Rune-based rather than byte-based because probe
// values legitimately contain 4-byte characters (VAL-019), and a byte slice through the
// middle of one would emit invalid UTF-8 into the report line.
func truncate(s string, n int) string {
	if len(s) <= n { // fast path: n bytes or fewer is always n runes or fewer
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func quoteVal(s string) string {
	return `"` + truncate(strings.Join(strings.Fields(s), " "), 120) + `"`
}
