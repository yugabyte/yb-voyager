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

package main

// These tests need neither Docker nor a build tag:
//
//	go test ./src/testlivemigration/sweepreport/

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleLog = `
=== RUN   TestDatatypeSweepLive
=== RUN   TestDatatypeSweepLive/ranges
    datatype_sweep_probe.go:100: setting up containers
PROBE-RESULT: CTRL-001 | int | LIVE | WORKS | snapshot + delta identical
PROBE-RESULT: RANGE-001 | int4range | LIVE | WORKS | snapshot + delta identical
PROBE-RESULT: RANGE-009 | CREATE TYPE AS RANGE | LIVE | QUIET_DROP | column absent from the event stream; verbatim: id=1 source="[1,5)" destination=NULL
--- PASS: TestDatatypeSweepLive/ranges (61.00s)
=== RUN   TestDatatypeSweepLive/hstore
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer repeats: cannot parse
PROBE-RESULT: HSTORE-001 | hstore | LIVE | STUCK | importer repeats: cannot parse
PROBE-RUN-INVALID: hstore | LIVE | known-good control CTRL-001 came out STUCK, not WORKS - the whole run is invalid
--- FAIL: TestDatatypeSweepLive/hstore (90.00s)
`

// An export-death run: the connector died, so the controls could not be measured and the
// run fails its gate - but the death itself was attributed, quoted, and is the finding.
const exportDeathLog = `
=== RUN   TestDatatypeSweepLive
=== RUN   TestDatatypeSweepLive/domains
PROBE-RESULT: CTRL-001 | int | LIVE | INCONCLUSIVE | the exporter died before this probe was measured
PROBE-RESULT: CTRL-002 | text | LIVE | INCONCLUSIVE | the exporter died before this probe was measured
PROBE-RESULT: DOM-005 | domain(enum) | LIVE | EXPORTER_CRASHES | the export side died: Connector completed: success = 'false' - io.debezium.DebeziumException: no converter for sweep_schema.p_dom_005.v
PROBE-PUBLISHABLE: DOM-005 | LIVE | EXPORTER_CRASHES | the exporter died with a quotable cause attributed to this probe
PROBE-RUN-EXPORT-DIED: domains | LIVE | DOM-005 killed the exporter
PROBE-RUN-INVALID: domains | LIVE | known-good control CTRL-001 came out INCONCLUSIVE, not WORKS
PROBE-RUN-FLAKE: domains | LIVE | 2 inconclusive | the exporter died during this run
--- FAIL: TestDatatypeSweepLive/domains (12.00s)
`

// TestPublishableMarkerPromotesOnlyItsRow pins the one carve-out from the control gate.
// The gate catches a BROKEN MEASUREMENT; an attributed export death is not one, it is the
// finding, and the controls going inconclusive is a consequence of it. So that row - and
// strictly only that row - survives its run's INVALID status.
func TestPublishableMarkerPromotesOnlyItsRow(t *testing.T) {
	rows, err := ParseLog(strings.NewReader(exportDeathLog), RunMeta{}, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	byKey := map[string]Row{}
	for _, r := range rows {
		byKey[r.Key()] = r
	}

	dom, ok := byKey["DOM-005|LIVE"]
	if !ok {
		t.Fatalf("no row for DOM-005|LIVE; got %v", byKey)
	}
	if dom.RunStatus != statusOK {
		t.Errorf("DOM-005 run_status = %q, want %q: an attributed export death is publishable",
			dom.RunStatus, statusOK)
	}
	if dom.Verdict != "EXPORTER_CRASHES" {
		t.Errorf("DOM-005 verdict = %q, want EXPORTER_CRASHES", dom.Verdict)
	}

	// The batch-mates were never measured. They must stay unpublishable, or the report
	// would show an inconclusive row as a result.
	for _, id := range []string{"CTRL-001", "CTRL-002"} {
		row := byKey[id+"|LIVE"]
		if row.RunStatus == statusOK {
			t.Errorf("%s run_status = %q, want anything but OK: it was never measured",
				id, row.RunStatus)
		}
	}
}

// TestPublishableMarkerCannotPromoteAnotherVerdict: the marker names the verdict it is
// promoting, and a row whose verdict does not match keeps its run's status. Without that,
// a stale or mis-emitted marker could launder an unproven verdict past the gate.
func TestPublishableMarkerCannotPromoteAnotherVerdict(t *testing.T) {
	log := `
=== RUN   TestDatatypeSweepLive/domains
PROBE-RESULT: DOM-005 | domain(enum) | LIVE | SILENT_LOSS | zero events for this table
PROBE-PUBLISHABLE: DOM-005 | LIVE | EXPORTER_CRASHES | attributed export death
PROBE-RUN-INVALID: domains | LIVE | control failed
`
	rows, err := ParseLog(strings.NewReader(log), RunMeta{}, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].RunStatus != statusInvalid {
		t.Errorf("run_status = %q, want %q: the marker names a different verdict than the row",
			rows[0].RunStatus, statusInvalid)
	}
}

func TestParseLogExtractsRowsAndGates(t *testing.T) {
	meta := RunMeta{Timestamp: "2026-08-31T00:00:00Z", VoyagerCommit: "abc123", PGVersion: "17.8", YBVersion: "2025.2.1.0"}
	rows, err := ParseLog(strings.NewReader(sampleLog), meta, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}

	byKey := map[string]Row{}
	for _, r := range rows {
		byKey[r.Key()] = r
	}

	// CTRL-001 appears in BOTH batches of this file - WORKS in ranges (whose own gate
	// passed) and STUCK in hstore (whose own gate failed). Row.Key() is (probe_id, mode)
	// only, so these two rows collide, and the dedupe merge preference (Defect 3) must pick
	// the row from the TRUSTED batch over the one from the untrusted batch, regardless of
	// which one was parsed later - the old "last occurrence wins" behavior is exactly the
	// bug Defect 3 removed.
	ctrl, ok := byKey["CTRL-001|LIVE"]
	if !ok {
		t.Fatalf("no row for CTRL-001|LIVE; got %v", byKey)
	}
	if ctrl.Verdict != "WORKS" {
		t.Errorf("CTRL-001 verdict = %q, want WORKS: the trusted (ranges) measurement must win over "+
			"the untrusted (hstore) one, even though hstore was parsed later", ctrl.Verdict)
	}
	if ctrl.RunStatus != statusOK {
		t.Errorf("CTRL-001 run_status = %q, want %q", ctrl.RunStatus, statusOK)
	}

	rng := byKey["RANGE-001|LIVE"]
	if rng.Category != "ranges" {
		t.Errorf("RANGE-001 category = %q, want %q (from the === RUN subtest name)", rng.Category, "ranges")
	}
	// The data-derived control gate (Defect 2) is scoped to (file, batch, mode) - the same
	// key the marker gate already uses, built from the row's own "=== RUN" attribution
	// rather than from a marker line's text - so a control failing in the UNRELATED hstore
	// batch must never touch the ranges batch, whose own CTRL-001 came out WORKS. Grouping
	// by mode alone (ignoring batch) was tried and is wrong: it lets one batch's failure
	// contaminate an unrelated batch sharing only the same mode.
	if rng.RunStatus != statusOK {
		t.Errorf("RANGE-001 run_status = %q, want %q: the ranges batch passed its own gate", rng.RunStatus, statusOK)
	}
	if rng.PGVersion != "17.8" || rng.VoyagerCommit != "abc123" {
		t.Errorf("run metadata not stamped onto the row: %+v", rng)
	}

	drop := byKey["RANGE-009|LIVE"]
	if drop.SourceValue != "[1,5)" || drop.TargetValue != "NULL" {
		t.Errorf("verbatim values not lifted out of the detail: source=%q target=%q", drop.SourceValue, drop.TargetValue)
	}

	// HSTORE-001 is the ONLY non-control probe in the hstore batch, and that batch's own
	// CTRL-001 failed (STUCK) right alongside it: the hstore value wedged the shared
	// importer connector and took the control down with it. Nothing else in this batch
	// could have caused that failure, so this is exactly the solo carve-out (see
	// statusAttributed): ATTRIBUTED, not INVALID.
	hstore := byKey["HSTORE-001|LIVE"]
	if hstore.RunStatus != statusAttributed {
		t.Errorf("HSTORE-001 run_status = %q, want %q: it is the sole probe in a batch whose own control failed",
			hstore.RunStatus, statusAttributed)
	}
}

func TestParseLogPrefersCatalogCategory(t *testing.T) {
	rows, err := ParseLog(strings.NewReader(sampleLog), RunMeta{}, func(id string) string {
		if id == "RANGE-001" {
			return "from-catalog"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	for _, r := range rows {
		if r.ProbeID == "RANGE-001" && r.Category != "from-catalog" {
			t.Fatalf("catalog group should win over the subtest name, got %q", r.Category)
		}
		if r.ProbeID == "RANGE-009" && r.Category != "ranges" {
			t.Fatalf("without a catalog group the subtest name is the fallback, got %q", r.Category)
		}
	}
}

// TestParseLogPrefersStructuredValues pins that the PROBE-VALUES line wins over the
// regex scrape of the prose detail, and that the escaping survives the round trip.
func TestParseLogPrefersStructuredValues(t *testing.T) {
	log := `=== RUN   TestDatatypeSweepLive/core
PROBE-RESULT: CORE-001 | text | LIVE | SILENT_WRONG | mismatch; verbatim: id=1 source="from-the-prose" destination="also-prose"
PROBE-VALUES: CORE-001 | LIVE | a\x7cb | c\nd
PROBE-RESULT: CORE-002 | int | LIVE | SILENT_WRONG | mismatch; verbatim: id=1 source="only-prose" destination=NULL
`
	rows, err := ParseLog(strings.NewReader(log), RunMeta{}, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	byKey := map[string]Row{}
	for _, r := range rows {
		byKey[r.Key()] = r
	}

	got := byKey["CORE-001|LIVE"]
	if got.SourceValue != "a|b" {
		t.Errorf("source = %q, want the unescaped %q from PROBE-VALUES, not the prose", got.SourceValue, "a|b")
	}
	if got.TargetValue != "c\nd" {
		t.Errorf("target = %q, want the unescaped newline from PROBE-VALUES", got.TargetValue)
	}

	// Without a PROBE-VALUES line the prose scrape is still the fallback.
	if fb := byKey["CORE-002|LIVE"]; fb.SourceValue != "only-prose" || fb.TargetValue != "NULL" {
		t.Errorf("fallback scrape broke: source=%q target=%q", fb.SourceValue, fb.TargetValue)
	}
}

// TestUnescapeValueIsAFaithfulInverse is the property that matters: whatever the harness
// writes, the collector must read back byte-for-byte. The escaping exists so that a value
// containing a pipe is recorded rather than rewritten.
func TestUnescapeValueIsAFaithfulInverse(t *testing.T) {
	// sanitizeValue in the harness, reproduced here so the test states the contract
	// rather than importing across the build tag.
	sanitize := func(s string) string {
		return strings.NewReplacer(
			`\`, `\\`,
			"|", `\x7c`,
			"\n", `\n`,
			"\r", `\r`,
			"\t", `\t`,
		).Replace(s)
	}

	for _, original := range []string{
		"plain",
		"a|b",
		"line1\nline2",
		"tab\there",
		`literal backslash \ here`,
		// The adversarial one: text that already looks like the pipe escape. A decoder
		// using chained replacements turns this into "a|b".
		`a\x7cb`,
		`\\x7c`,
		`trailing backslash \`,
		"",
		"{\"k\": \"v|w\"}",
	} {
		if got := unescapeValue(sanitize(original)); got != original {
			t.Errorf("round trip changed the value:\n original %q\n escaped  %q\n decoded  %q",
				original, sanitize(original), got)
		}
	}
}

func TestSQLStateExtraction(t *testing.T) {
	cases := map[string]string{
		`import data failed; importer error (x14): ERROR: cannot accept a value of type gtsvector (SQLSTATE 0A000)`: "0A000",
		`ERROR: invalid input syntax for type integer (22P02)`:                                                      "22P02",
		`snapshot source->target: [insert] id=6 row absent`:                                                         "",
		`SQLSTATE: 42883 function does not exist`:                                                                   "42883",
	}
	for detail, want := range cases {
		if got := sqlStateOf(detail); got != want {
			t.Errorf("sqlStateOf(%q) = %q, want %q", detail, got, want)
		}
	}
}

func TestCSVRoundTrip(t *testing.T) {
	rows := []Row{
		{ProbeID: "A-001", TypeName: "int", Mode: "LIVE", Verdict: "WORKS", Evidence: "fine", RunStatus: statusOK},
		{ProbeID: "B-001", TypeName: "hstore, with comma", Mode: "OFFLINE", Verdict: "SILENT_LOSS", Evidence: `has "quotes"`, RunStatus: statusOK},
	}
	path := filepath.Join(t.TempDir(), "out.csv")
	if err := WriteCSV(path, rows); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	got, err := ReadCSV(path)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(got) != len(rows) {
		t.Fatalf("round trip changed the row count: %d -> %d", len(rows), len(got))
	}
	for i := range rows {
		if got[i] != rows[i] {
			t.Errorf("row %d changed:\n got %+v\nwant %+v", i, got[i], rows[i])
		}
	}
}

func TestDiffClassifiesChanges(t *testing.T) {
	old := []Row{
		{ProbeID: "P-1", Mode: "LIVE", Verdict: "WORKS", RunStatus: statusOK},
		{ProbeID: "P-2", Mode: "LIVE", Verdict: "QUIET_DROP", RunStatus: statusOK},
		{ProbeID: "P-3", Mode: "LIVE", Verdict: "WORKS", RunStatus: statusOK},
		{ProbeID: "P-4", Mode: "LIVE", Verdict: "SKIPPED", RunStatus: statusOK},
		{ProbeID: "P-6", Mode: "LIVE", Verdict: "WORKS", RunStatus: statusInvalid},
	}
	nw := []Row{
		{ProbeID: "P-1", Mode: "LIVE", Verdict: "SILENT_LOSS", RunStatus: statusOK},   // regression
		{ProbeID: "P-2", Mode: "LIVE", Verdict: "EXCLUDED_TOLD", RunStatus: statusOK}, // improvement
		{ProbeID: "P-3", Mode: "LIVE", Verdict: "WORKS", RunStatus: statusOK},         // unchanged
		{ProbeID: "P-4", Mode: "LIVE", Verdict: "WORKS", RunStatus: statusOK},         // coverage gain
		{ProbeID: "P-5", Mode: "LIVE", Verdict: "WORKS", RunStatus: statusOK},         // new probe
		{ProbeID: "P-6", Mode: "LIVE", Verdict: "SILENT_LOSS", RunStatus: statusOK},   // old side invalid -> gain, not regression
	}

	d := Diff(old, nw)
	if len(d.Regressions) != 1 || d.Regressions[0].ProbeID != "P-1" {
		t.Errorf("regressions = %v, want exactly P-1", d.Regressions)
	}
	if len(d.Improvements) != 1 || d.Improvements[0].ProbeID != "P-2" {
		t.Errorf("improvements = %v, want exactly P-2", d.Improvements)
	}
	if d.Unchanged != 1 {
		t.Errorf("unchanged = %d, want 1", d.Unchanged)
	}
	gains := map[string]bool{}
	for _, c := range d.CoverageGain {
		gains[c.ProbeID] = true
	}
	for _, want := range []string{"P-4", "P-5", "P-6"} {
		if !gains[want] {
			t.Errorf("expected %s in coverage gains, got %v", want, d.CoverageGain)
		}
	}
	if d.SkippedOldBad != 1 {
		t.Errorf("rows from an invalid run must be excluded, counted %d", d.SkippedOldBad)
	}
	if !d.HasRegressions() {
		t.Error("HasRegressions should be true when there is a regression")
	}

	var buf bytes.Buffer
	PrintDiff(&buf, d, "old.csv", "new.csv")
	if !strings.Contains(buf.String(), "REGRESSIONS (1)") {
		t.Errorf("PrintDiff output missing the regression section:\n%s", buf.String())
	}
}

// TestDiffReportsSQLStateMoveWithoutCallingItARegression: the outcome is the same, but
// the type is failing for a different reason. That is reportable and must not be folded
// into the unchanged count, nor gate a release.
func TestDiffReportsSQLStateMoveWithoutCallingItARegression(t *testing.T) {
	old := []Row{
		{ProbeID: "P-1", Mode: "OFFLINE", Verdict: "BLOCKS", SQLState: "0A000", RunStatus: statusOK},
		{ProbeID: "P-2", Mode: "OFFLINE", Verdict: "BLOCKS", SQLState: "0A000", RunStatus: statusOK},
	}
	nw := []Row{
		{ProbeID: "P-1", Mode: "OFFLINE", Verdict: "BLOCKS", SQLState: "22P02", RunStatus: statusOK},
		{ProbeID: "P-2", Mode: "OFFLINE", Verdict: "BLOCKS", SQLState: "0A000", RunStatus: statusOK},
	}

	d := Diff(old, nw)
	if len(d.ErrorCodeChanges) != 1 || d.ErrorCodeChanges[0].ProbeID != "P-1" {
		t.Fatalf("error-code changes = %v, want exactly P-1", d.ErrorCodeChanges)
	}
	if got := d.ErrorCodeChanges[0]; got.Old != "0A000" || got.New != "22P02" {
		t.Errorf("change should carry the two SQLSTATEs, got %s -> %s", got.Old, got.New)
	}
	if d.Unchanged != 1 {
		t.Errorf("unchanged = %d, want 1 (only P-2)", d.Unchanged)
	}
	if len(d.Regressions) != 0 || d.HasRegressions() {
		t.Error("a SQLSTATE move with an unchanged verdict must not gate a release")
	}
}

func TestDiffTreatsLostMeasurementAsCoverageLoss(t *testing.T) {
	d := Diff(
		[]Row{{ProbeID: "P-1", Mode: "LIVE", Verdict: "WORKS", RunStatus: statusOK}},
		nil,
	)
	if len(d.CoverageLoss) != 1 {
		t.Fatalf("coverage loss = %v, want one entry", d.CoverageLoss)
	}
	if !d.HasRegressions() {
		t.Error("lost coverage must fail the same gate as a regression")
	}
}

func TestBuildReportIsAViewOverTheSuite(t *testing.T) {
	cat := &Catalog{
		GeneratedAt: "2026-08-31T00:00:00Z",
		Entries: []CatalogEntry{
			{ProbeID: "P-1", TypeName: "int4range", Group: "ranges", ReportedByAssess: "no", GuardrailAction: "no action"},
			{ProbeID: "P-2", TypeName: "xml", Group: "core", ReportedByAssess: "unsupported (offline and live)"},
		},
	}
	rows := []Row{
		{ProbeID: "P-1", Mode: "OFFLINE", Verdict: "WORKS", RunStatus: statusOK},
		{ProbeID: "P-1", Mode: "LIVE", Verdict: "QUIET_DROP", Evidence: "column absent", RunStatus: statusOK},
		{ProbeID: "P-9", Mode: "LIVE", Verdict: "WORKS", RunStatus: statusOK}, // orphan
	}

	doc, problems := BuildReport(cat, rows, []string{"OFFLINE"})
	if len(doc.Rows) != 2 {
		t.Fatalf("report must have exactly one row per catalog entry, got %d", len(doc.Rows))
	}

	byID := map[string]ReportRow{}
	for _, r := range doc.Rows {
		byID[r.ProbeID] = r
	}
	if byID["P-1"].Live.Verdict != "QUIET_DROP" || byID["P-1"].Offline.Verdict != "WORKS" {
		t.Errorf("P-1 cells wrong: %+v", byID["P-1"])
	}
	if byID["P-1"].FallBack.Verdict != verdictNotTested {
		t.Errorf("an unmeasured mode must read %s, got %q", verdictNotTested, byID["P-1"].FallBack.Verdict)
	}
	if byID["P-2"].ReportedByAssess != "unsupported (offline and live)" {
		t.Errorf("reporting-layer columns must come through from the catalog: %+v", byID["P-2"])
	}
	if byID["P-1"].Live.Evidence != "column absent" {
		t.Errorf("evidence must be carried into the report cell: %+v", byID["P-1"].Live)
	}

	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "P-9") {
		t.Errorf("a result with no catalog entry is drift and must be reported: %v", problems)
	}
	if !strings.Contains(joined, "P-2") {
		t.Errorf("a catalog entry with no measurement in a required mode must be reported: %v", problems)
	}

	// bestEvidence prefers the worst product verdict's evidence.
	if got := bestEvidence(byID["P-1"]); got != "column absent" {
		t.Errorf("bestEvidence = %q, want the QUIET_DROP evidence", got)
	}
}

// ============================================================
// Defects 1-3: multi-log collect, the data-derived control gate (including its
// solo-probe/ATTRIBUTED carve-out), and the trust-ranked dedupe merge preference.
// ============================================================

// TestSweepMergeAcrossLogsPrefersCleanRun is requirement (a): two logs, one whose control
// gate failed and one whose control gate passed, measuring the same probe. Each log is
// parsed on its own (Defect 1), so run A's failure never touches run B's rows - and the
// merge (Defect 3) must keep run B's clean measurement rather than whichever log happened
// to be parsed last.
func TestSweepMergeAcrossLogsPrefersCleanRun(t *testing.T) {
	// Two non-control probes share the batch, so this run's failure is definitely
	// collateral (INVALID), not the solo/ATTRIBUTED carve-out - keeping this test about
	// the merge, not about which gate outcome produced the INVALID.
	logA := `
=== RUN   TestDatatypeSweepLive/mixed
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer wedged
PROBE-RESULT: RANGE-001 | int4range | LIVE | SILENT_LOSS | value dropped
PROBE-RESULT: OTHER-001 | foo | LIVE | WORKS | fine
PROBE-RUN-INVALID: mixed | LIVE | control failed
`
	logB := `
=== RUN   TestDatatypeSweepLive/mixed
PROBE-RESULT: CTRL-001 | int | LIVE | WORKS | fine
PROBE-RESULT: RANGE-001 | int4range | LIVE | WORKS | fine
`
	rowsA, err := ParseLog(strings.NewReader(logA), RunMeta{Timestamp: "2026-08-01T00:00:00Z"}, nil)
	if err != nil {
		t.Fatalf("ParseLog A: %v", err)
	}
	rowsB, err := ParseLog(strings.NewReader(logB), RunMeta{Timestamp: "2026-08-02T00:00:00Z"}, nil)
	if err != nil {
		t.Fatalf("ParseLog B: %v", err)
	}

	// Setup sanity: confirm each file's own gate came out as expected before merging.
	find := func(rows []Row, key string) Row {
		for _, r := range rows {
			if r.Key() == key {
				return r
			}
		}
		t.Fatalf("no row for %s", key)
		return Row{}
	}
	if got := find(rowsA, "RANGE-001|LIVE").RunStatus; got != statusInvalid {
		t.Fatalf("setup: run A's RANGE-001 run_status = %q, want %q", got, statusInvalid)
	}
	if got := find(rowsB, "RANGE-001|LIVE").RunStatus; got != statusOK {
		t.Fatalf("setup: run B's RANGE-001 run_status = %q, want %q", got, statusOK)
	}

	merged := dedupe(append(append([]Row{}, rowsA...), rowsB...))
	got := find(merged, "RANGE-001|LIVE")
	if got.Verdict != "WORKS" || got.RunStatus != statusOK {
		t.Errorf("merged RANGE-001 = verdict %q run_status %q, want WORKS/%s (run B's clean measurement, not run A's)",
			got.Verdict, got.RunStatus, statusOK)
	}
}

// TestSweepBatchControlFailureInvalidatesWithoutMarker is requirement (b) / Defect 2: a
// control verdict other than WORKS must invalidate its (file, mode), even with no
// PROBE-RUN-INVALID marker line at all - the marker is not the only signal, and here it is
// the ONLY signal missing. This run has two non-control probes, so it is a BATCH run and
// nothing in it is attributable to any single probe (see TestSweepSoloRunWithFailedControlIsAttributed
// for the one-probe case, which comes out differently).
func TestSweepBatchControlFailureInvalidatesWithoutMarker(t *testing.T) {
	log := `
=== RUN   TestDatatypeSweepLive/batch
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer wedged
PROBE-RESULT: A-001 | foo | LIVE | WORKS | fine
PROBE-RESULT: B-001 | bar | LIVE | WORKS | fine
`
	rows, err := ParseLog(strings.NewReader(log), RunMeta{}, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	for _, r := range rows {
		if r.RunStatus != statusInvalid {
			t.Errorf("%s run_status = %q, want %q: no marker line fired, but the control failed",
				r.Key(), r.RunStatus, statusInvalid)
		}
	}
}

// TestSweepSoloRunWithFailedControlIsAttributed is requirement (e): a datatype that wedges
// the importer also wedges the shared controls riding the same connector, so a solo probe
// of such a type can NEVER pass its own control gate. A blunt gate would make that finding
// unreportable by construction. When this probe is the ONLY non-control probe in its
// (file, mode), nothing else could have caused the control failure, so its own row is
// ATTRIBUTED - trustworthy - even though the run's controls did not come out WORKS.
func TestSweepSoloRunWithFailedControlIsAttributed(t *testing.T) {
	log := `
=== RUN   TestDatatypeSweepLive/solo
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer wedged by the probe under test
PROBE-RESULT: WEDGE-001 | poisontype | LIVE | STUCK | importer wedges on this value
`
	rows, err := ParseLog(strings.NewReader(log), RunMeta{}, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	byKey := map[string]Row{}
	for _, r := range rows {
		byKey[r.Key()] = r
	}

	wedge, ok := byKey["WEDGE-001|LIVE"]
	if !ok {
		t.Fatalf("no row for WEDGE-001|LIVE; got %v", byKey)
	}
	if wedge.RunStatus != statusAttributed {
		t.Errorf("WEDGE-001 run_status = %q, want %q: it was the only probe in this run, so the "+
			"control failure is attributable to it, not evidence of a broken measurement", wedge.RunStatus, statusAttributed)
	}
	if !isTrustedStatus(wedge.RunStatus) {
		t.Errorf("%q must be a trusted status: an ATTRIBUTED row is real evidence, not noise", wedge.RunStatus)
	}
	if wedge.Verdict != "STUCK" {
		t.Errorf("WEDGE-001 verdict = %q, want STUCK: ATTRIBUTED must not change what was observed", wedge.Verdict)
	}

	// The control's OWN row is a different story: CTRL-001 itself was never actually
	// measured (it wedged too), so it does not get the solo carve-out - only the probe
	// under test does.
	if ctrl := byKey["CTRL-001|LIVE"]; ctrl.RunStatus != statusInvalid {
		t.Errorf("CTRL-001 run_status = %q, want %q: the control itself was never measured, "+
			"so it is not promoted even when its probe is", ctrl.RunStatus, statusInvalid)
	}
}

// TestSweepBatchRunWithFailedControlStaysInvalid is requirement (f): three probes sharing
// one (file, mode) whose controls failed. Any one of the three could have wedged the
// shared channel, so the failure cannot be pinned on any single row - all three (and the
// controls) must stay INVALID rather than being promoted.
func TestSweepBatchRunWithFailedControlStaysInvalid(t *testing.T) {
	log := `
=== RUN   TestDatatypeSweepLive/batch3
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer wedged
PROBE-RESULT: A-001 | foo | LIVE | STUCK | maybe this one wedged it
PROBE-RESULT: B-001 | bar | LIVE | WORKS | fine
PROBE-RESULT: C-001 | baz | LIVE | WORKS | fine
`
	rows, err := ParseLog(strings.NewReader(log), RunMeta{}, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4", len(rows))
	}
	for _, r := range rows {
		if r.RunStatus != statusInvalid {
			t.Errorf("%s run_status = %q, want %q: three non-control probes shared this run, "+
				"so the control failure cannot be pinned on any one of them", r.Key(), r.RunStatus, statusInvalid)
		}
	}
}

// TestSweepBatchInvalidRowDoesNotBeatCleanRunAcrossLogs is requirement (g): the same probe
// appears in a contaminated BATCH run (INVALID, per (f) above) in one log and in a clean
// run in another log. The merge must keep the clean run's row.
func TestSweepBatchInvalidRowDoesNotBeatCleanRunAcrossLogs(t *testing.T) {
	dirty := `
=== RUN   TestDatatypeSweepLive/batch3
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer wedged
PROBE-RESULT: A-001 | foo | LIVE | STUCK | collateral damage
PROBE-RESULT: B-001 | bar | LIVE | WORKS | fine
PROBE-RESULT: C-001 | baz | LIVE | WORKS | fine
`
	clean := `
=== RUN   TestDatatypeSweepLive/batch3
PROBE-RESULT: CTRL-001 | int | LIVE | WORKS | fine
PROBE-RESULT: A-001 | foo | LIVE | WORKS | fine
`
	rowsDirty, err := ParseLog(strings.NewReader(dirty), RunMeta{Timestamp: "2026-08-01T00:00:00Z"}, nil)
	if err != nil {
		t.Fatalf("ParseLog dirty: %v", err)
	}
	rowsClean, err := ParseLog(strings.NewReader(clean), RunMeta{Timestamp: "2026-08-02T00:00:00Z"}, nil)
	if err != nil {
		t.Fatalf("ParseLog clean: %v", err)
	}

	merged := dedupe(append(append([]Row{}, rowsDirty...), rowsClean...))
	var got Row
	for _, r := range merged {
		if r.Key() == "A-001|LIVE" {
			got = r
		}
	}
	if got.Verdict != "WORKS" || got.RunStatus != statusOK {
		t.Errorf("merged A-001 = verdict %q run_status %q, want WORKS/%s (the clean run, not the batch-contaminated one)",
			got.Verdict, got.RunStatus, statusOK)
	}
}

// TestSweepBatchGateIsPerSubtestNotPerMode is requirement (h): the regression this fix
// exists for. ONE log contains TWO batches in the SAME mode - batch A's controls fail,
// batch B's controls pass, exactly like a real multi-batch OFFLINE log (e.g.
// coverage/off_wave2c.log's catalogstats/exttypes/indexkeys subtests). Grouping the
// data-derived gate by (file, mode) instead of (file, batch, mode) gets this wrong in one
// of two directions: either A's failure contaminates B's good rows (too strict), or B's
// pass blesses A's bad rows (too lenient, and exactly the mirror-image bug an independent
// reimplementation of this gate produced). Neither is acceptable: A's rows must be
// INVALID and B's rows must be OK, asserted on the specific probe rows.
func TestSweepBatchGateIsPerSubtestNotPerMode(t *testing.T) {
	log := `
=== RUN   TestDatatypeSweepOffline
=== RUN   TestDatatypeSweepOffline/batch-a
PROBE-RESULT: CTRL-001 | int | OFFLINE | BLOCKS | control probe itself was blocked
PROBE-RESULT: A-001 | foo | OFFLINE | BLOCKS | migration refuses to proceed
PROBE-RESULT: A-002 | bar | OFFLINE | BLOCKS | migration refuses to proceed
PROBE-RUN-INVALID: batch-a | OFFLINE | known-good control CTRL-001 came out BLOCKS, not WORKS
=== RUN   TestDatatypeSweepOffline/batch-b
PROBE-RESULT: CTRL-001 | int | OFFLINE | WORKS | fine
PROBE-RESULT: B-001 | baz | OFFLINE | WORKS | fine
`
	rows, err := ParseLog(strings.NewReader(log), RunMeta{}, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	byKeyAndCategory := map[string]Row{}
	for _, r := range rows {
		byKeyAndCategory[r.Category+"/"+r.Key()] = r
	}

	for _, id := range []string{"A-001", "A-002"} {
		row, ok := byKeyAndCategory["batch-a/"+id+"|OFFLINE"]
		if !ok {
			t.Fatalf("no row for batch-a/%s|OFFLINE; got %v", id, byKeyAndCategory)
		}
		if row.RunStatus != statusInvalid {
			t.Errorf("%s (batch-a) run_status = %q, want %q: its OWN batch's control failed",
				id, row.RunStatus, statusInvalid)
		}
	}

	b1, ok := byKeyAndCategory["batch-b/B-001|OFFLINE"]
	if !ok {
		t.Fatalf("no row for batch-b/B-001|OFFLINE; got %v", byKeyAndCategory)
	}
	if b1.RunStatus != statusOK {
		t.Errorf("B-001 (batch-b) run_status = %q, want %q: batch-b's OWN control passed - "+
			"batch-a's unrelated failure in the same mode must not contaminate it", b1.RunStatus, statusOK)
	}
}

// TestSweepSoloCountIsPerBatchNotPerMode is requirement (i): batch A has three probes with
// failed controls (collateral, INVALID); batch B, in the SAME mode and the SAME file, has
// exactly one probe with its own failed controls. Batch B's probe must be recognised as
// solo WITHIN ITS OWN BATCH and come out ATTRIBUTED, even though the file as a whole has
// more than one non-control probe across both batches - proving the solo count is scoped
// per (batch, mode), not per (file, mode).
func TestSweepSoloCountIsPerBatchNotPerMode(t *testing.T) {
	log := `
=== RUN   TestDatatypeSweepLive/batch-a
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer wedged
PROBE-RESULT: A-001 | foo | LIVE | STUCK | collateral damage
PROBE-RESULT: A-002 | bar | LIVE | WORKS | fine
PROBE-RESULT: A-003 | baz | LIVE | WORKS | fine
=== RUN   TestDatatypeSweepLive/batch-b
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer wedged by the solo probe under test
PROBE-RESULT: WEDGE-001 | poisontype | LIVE | STUCK | importer wedges on this value
`
	rows, err := ParseLog(strings.NewReader(log), RunMeta{}, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	byKeyAndCategory := map[string]Row{}
	for _, r := range rows {
		byKeyAndCategory[r.Category+"/"+r.Key()] = r
	}

	for _, id := range []string{"A-001", "A-002", "A-003"} {
		row, ok := byKeyAndCategory["batch-a/"+id+"|LIVE"]
		if !ok {
			t.Fatalf("no row for batch-a/%s|LIVE; got %v", id, byKeyAndCategory)
		}
		if row.RunStatus != statusInvalid {
			t.Errorf("%s (batch-a, 3 probes) run_status = %q, want %q: the failure cannot be pinned "+
				"on any one of three probes sharing the batch", id, row.RunStatus, statusInvalid)
		}
	}

	wedge, ok := byKeyAndCategory["batch-b/WEDGE-001|LIVE"]
	if !ok {
		t.Fatalf("no row for batch-b/WEDGE-001|LIVE; got %v", byKeyAndCategory)
	}
	if wedge.RunStatus != statusAttributed {
		t.Errorf("WEDGE-001 (batch-b, solo) run_status = %q, want %q: it is the sole probe in ITS OWN "+
			"batch, even though batch-a (same file, same mode) has three - the solo count must be per "+
			"batch, not per (file, mode)", wedge.RunStatus, statusAttributed)
	}
}

// TestSweepDedupePreferenceOrder is requirement (c): the three-step preference dedupe uses
// to pick a winner between two rows sharing a (probe_id, mode) key, tested directly against
// preferRow so each rule is pinned in isolation.
func TestSweepDedupePreferenceOrder(t *testing.T) {
	cases := []struct {
		name            string
		incumbent       Row
		candidate       Row
		wantCandWins    bool
		wantCandWinsWhy string
	}{
		{
			name:            "trust beats a later, more severe verdict",
			incumbent:       Row{Verdict: "WORKS", RunStatus: statusOK, RunTimestamp: "2026-01-01T00:00:00Z"},
			candidate:       Row{Verdict: "SILENT_LOSS", RunStatus: statusInvalid, RunTimestamp: "2099-01-01T00:00:00Z"},
			wantCandWins:    false,
			wantCandWinsWhy: "a WORKS row from a passing run must never be displaced by a failing row from a run that didn't pass its own gate, no matter when the failing row was parsed",
		},
		{
			name:            "trust beats an earlier verdict too, in the other direction",
			incumbent:       Row{Verdict: "SILENT_LOSS", RunStatus: statusInvalid, RunTimestamp: "2099-01-01T00:00:00Z"},
			candidate:       Row{Verdict: "WORKS", RunStatus: statusOK, RunTimestamp: "2026-01-01T00:00:00Z"},
			wantCandWins:    true,
			wantCandWinsWhy: "a trustworthy row replaces an untrustworthy one even if it is chronologically older",
		},
		{
			name:            "ATTRIBUTED is trusted, same as OK",
			incumbent:       Row{Verdict: "STUCK", RunStatus: statusInvalid, RunTimestamp: "2026-01-01T00:00:00Z"},
			candidate:       Row{Verdict: "STUCK", RunStatus: statusAttributed, RunTimestamp: "2026-01-01T00:00:00Z"},
			wantCandWins:    true,
			wantCandWinsWhy: "ATTRIBUTED must be trusted everywhere OK is trusted",
		},
		{
			name:            "an observation beats INCONCLUSIVE at equal trust",
			incumbent:       Row{Verdict: "INCONCLUSIVE", RunStatus: statusOK, RunTimestamp: "2026-01-01T00:00:00Z"},
			candidate:       Row{Verdict: "STUCK", RunStatus: statusOK, RunTimestamp: "2020-01-01T00:00:00Z"},
			wantCandWins:    true,
			wantCandWinsWhy: "an actual observation beats \"we could not tell\", even one recorded earlier",
		},
		{
			name:            "INCONCLUSIVE does not beat an observation at equal trust",
			incumbent:       Row{Verdict: "STUCK", RunStatus: statusOK, RunTimestamp: "2020-01-01T00:00:00Z"},
			candidate:       Row{Verdict: "INCONCLUSIVE", RunStatus: statusOK, RunTimestamp: "2099-01-01T00:00:00Z"},
			wantCandWins:    false,
			wantCandWinsWhy: "a later INCONCLUSIVE must not displace an earlier real observation",
		},
		{
			name:            "later run_timestamp wins the final tiebreak",
			incumbent:       Row{Verdict: "WORKS", RunStatus: statusOK, RunTimestamp: "2026-01-01T00:00:00Z"},
			candidate:       Row{Verdict: "SILENT_LOSS", RunStatus: statusOK, RunTimestamp: "2026-06-01T00:00:00Z"},
			wantCandWins:    true,
			wantCandWinsWhy: "a later re-run supersedes what it re-ran, even to a worse verdict - that is a deliberate rule, not the SQLSTATE bug",
		},
		{
			name:            "a tied timestamp (same log) keeps the later-parsed row",
			incumbent:       Row{Verdict: "WORKS", RunStatus: statusOK, RunTimestamp: "2026-01-01T00:00:00Z"},
			candidate:       Row{Verdict: "SILENT_LOSS", RunStatus: statusOK, RunTimestamp: "2026-01-01T00:00:00Z"},
			wantCandWins:    true,
			wantCandWinsWhy: "same-timestamp rows come from the same log, and the later one in parse order is the final attempt",
		},
		{
			name: "SQLSTATE presence is not a tiebreaker",
			incumbent: Row{Verdict: "SILENT_LOSS", RunStatus: statusOK, RunTimestamp: "2026-01-01T00:00:00Z",
				SQLState: "0A000"},
			candidate:       Row{Verdict: "SILENT_LOSS", RunStatus: statusOK, RunTimestamp: "2026-01-01T00:00:00Z"},
			wantCandWins:    true,
			wantCandWinsWhy: "the candidate has no SQLSTATE at all, yet still wins the tied timestamp - carrying a SQLSTATE must give a row no edge, in either direction",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := preferRow(tc.incumbent, tc.candidate); got != tc.wantCandWins {
				t.Errorf("preferRow(incumbent, candidate) = %v, want %v: %s", got, tc.wantCandWins, tc.wantCandWinsWhy)
			}
		})
	}
}

// TestSweepPoisonSurvivesTheDataDerivedGate is requirement (d), the POISON half: a
// poison-isolation run's control gate is N/A by design (its whole point is a control
// probe dying next to a deliberately poisonous one), so the Defect 2 data-derived gate -
// which would otherwise treat that exact shape as an untrustworthy run - must never
// overwrite a marker-declared POISON status. POISON must also be trusted the same as OK
// in the dedupe merge preference.
func TestSweepPoisonSurvivesTheDataDerivedGate(t *testing.T) {
	log := `
=== RUN   TestDatatypeSweepLive/poison
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer wedged by the poison probe
PROBE-RESULT: POISON-001 | badtype | LIVE | BLOCKS | deliberate poison probe
PROBE-RUN-POISON: poison | LIVE | deliberate poison probe in this batch; control gate N/A
`
	rows, err := ParseLog(strings.NewReader(log), RunMeta{}, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	byKey := map[string]Row{}
	for _, r := range rows {
		byKey[r.Key()] = r
	}
	for _, key := range []string{"CTRL-001|LIVE", "POISON-001|LIVE"} {
		if got := byKey[key].RunStatus; got != statusPoison {
			t.Errorf("%s run_status = %q, want %q: the data-derived gate must not override a marker-declared POISON run",
				key, got, statusPoison)
		}
	}

	// POISON must be trusted like OK for the merge: an INVALID row for the same probe,
	// even a chronologically later one, must not displace it.
	laterInvalid := Row{ProbeID: "POISON-001", Mode: "LIVE", Verdict: "SILENT_LOSS",
		RunStatus: statusInvalid, RunTimestamp: "2099-01-01T00:00:00Z"}
	if preferRow(byKey["POISON-001|LIVE"], laterInvalid) {
		t.Error("an INVALID row must not displace a trusted POISON row in the dedupe merge")
	}
}

// TestSweepPublishableSurvivesTheDataDerivedGate is requirement (d), the PROBE-PUBLISHABLE
// half, exercised together with a solo-run data-derived gate rather than the marker-only
// scenario the older TestPublishableMarker* tests cover: the promotion must win even when
// the data-derived gate (not just the marker) is what put the row at risk.
func TestSweepPublishableSurvivesTheDataDerivedGate(t *testing.T) {
	log := `
=== RUN   TestDatatypeSweepLive/domains
PROBE-RESULT: CTRL-001 | int | LIVE | INCONCLUSIVE | the exporter died before this probe was measured
PROBE-RESULT: DOM-005 | domain(enum) | LIVE | EXPORTER_CRASHES | the export side died
PROBE-PUBLISHABLE: DOM-005 | LIVE | EXPORTER_CRASHES | the exporter died with a quotable cause attributed to this probe
`
	// Deliberately NO PROBE-RUN-INVALID marker line: only the data-derived gate is in play.
	rows, err := ParseLog(strings.NewReader(log), RunMeta{}, nil)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	byKey := map[string]Row{}
	for _, r := range rows {
		byKey[r.Key()] = r
	}
	dom, ok := byKey["DOM-005|LIVE"]
	if !ok {
		t.Fatalf("no row for DOM-005|LIVE; got %v", byKey)
	}
	// Without the PUBLISHABLE marker this row would land on ATTRIBUTED (solo run, dead
	// control) rather than INVALID - but PUBLISHABLE promotes explicitly to OK, and that
	// promotion must still win.
	if dom.RunStatus != statusOK {
		t.Errorf("DOM-005 run_status = %q, want %q: PROBE-PUBLISHABLE must promote to OK even when "+
			"the data-derived gate (not the marker) is what put the row at risk", dom.RunStatus, statusOK)
	}
}

// TestSweepCollectAcceptsRepeatedLogFlag exercises the actual `collect` CLI surface for
// Defect 1: -log is now flag.Var-backed and repeatable, rather than a single flag.String
// that forced callers to `cat` logs together (which destroys the run boundary ParseLog's
// gate relies on). Two real files, one dirty and one clean, must merge the same way the
// lower-level ParseLog+dedupe tests above already verified.
func TestSweepCollectAcceptsRepeatedLogFlag(t *testing.T) {
	dir := t.TempDir()
	logA := filepath.Join(dir, "a.log")
	logB := filepath.Join(dir, "b.log")
	writeLog(t, logA, `
=== RUN   TestDatatypeSweepLive/mixed
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer wedged
PROBE-RESULT: RANGE-001 | int4range | LIVE | SILENT_LOSS | value dropped
PROBE-RESULT: OTHER-001 | foo | LIVE | WORKS | fine
PROBE-RUN-INVALID: mixed | LIVE | control failed
`)
	writeLog(t, logB, `
=== RUN   TestDatatypeSweepLive/mixed
PROBE-RESULT: CTRL-001 | int | LIVE | WORKS | fine
PROBE-RESULT: RANGE-001 | int4range | LIVE | WORKS | fine
`)

	out := filepath.Join(dir, "out.csv")
	if err := cmdCollect([]string{"-log", logA, "-log", logB, "-out", out}); err != nil {
		t.Fatalf("cmdCollect: %v", err)
	}
	got := readRow(t, out, "RANGE-001|LIVE")
	if got.Verdict != "WORKS" || got.RunStatus != statusOK {
		t.Errorf("collect -log %s -log %s: RANGE-001 = verdict %q run_status %q, want WORKS/%s",
			logA, logB, got.Verdict, got.RunStatus, statusOK)
	}
}

// TestSweepCollectLogsGlobMergesFiles covers the -logs glob form of Defect 1, and that it
// composes with a single -log path rather than replacing it.
func TestSweepCollectLogsGlobMergesFiles(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, filepath.Join(dir, "run-1.log"), `
=== RUN   TestDatatypeSweepLive/mixed
PROBE-RESULT: CTRL-001 | int | LIVE | STUCK | importer wedged
PROBE-RESULT: RANGE-001 | int4range | LIVE | SILENT_LOSS | value dropped
PROBE-RESULT: OTHER-001 | foo | LIVE | WORKS | fine
PROBE-RUN-INVALID: mixed | LIVE | control failed
`)
	extra := filepath.Join(dir, "extra.log")
	writeLog(t, extra, `
=== RUN   TestDatatypeSweepLive/mixed
PROBE-RESULT: CTRL-001 | int | LIVE | WORKS | fine
PROBE-RESULT: RANGE-001 | int4range | LIVE | WORKS | fine
`)

	out := filepath.Join(dir, "out.csv")
	glob := filepath.Join(dir, "run-*.log")
	if err := cmdCollect([]string{"-logs", glob, "-log", extra, "-out", out}); err != nil {
		t.Fatalf("cmdCollect: %v", err)
	}
	got := readRow(t, out, "RANGE-001|LIVE")
	if got.Verdict != "WORKS" || got.RunStatus != statusOK {
		t.Errorf("collect -logs %s -log %s: RANGE-001 = verdict %q run_status %q, want WORKS/%s",
			glob, extra, got.Verdict, got.RunStatus, statusOK)
	}
}

func writeLog(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func readRow(t *testing.T, csvPath, key string) Row {
	t.Helper()
	rows, err := ReadCSV(csvPath)
	if err != nil {
		t.Fatalf("ReadCSV %s: %v", csvPath, err)
	}
	for _, r := range rows {
		if r.Key() == key {
			return r
		}
	}
	t.Fatalf("no row for %s in %s", key, csvPath)
	return Row{}
}

func TestWriteReportCSVHasOneRowPerProbe(t *testing.T) {
	doc := &ReportDoc{Rows: []ReportRow{
		{ProbeID: "P-1", TypeName: "int", Group: "core", Offline: ModeResult{Verdict: "WORKS"}},
		{ProbeID: "P-2", TypeName: "xml", Group: "core", Offline: ModeResult{Verdict: "BLOCKS"}},
	}}
	path := filepath.Join(t.TempDir(), "report.csv")
	if err := WriteReportCSV(path, doc); err != nil {
		t.Fatalf("WriteReportCSV: %v", err)
	}
	got, err := ReadCSV(path) // reuses the generic reader only to count lines
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("report CSV has %d data rows, want 2", len(got))
	}
}
