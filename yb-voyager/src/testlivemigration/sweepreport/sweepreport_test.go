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

	// A duplicated probe id (CTRL-001 appears in both batches) keeps the LAST occurrence.
	ctrl, ok := byKey["CTRL-001|LIVE"]
	if !ok {
		t.Fatalf("no row for CTRL-001|LIVE; got %v", byKey)
	}
	if ctrl.Verdict != "STUCK" {
		t.Errorf("CTRL-001 verdict = %q, want the later STUCK", ctrl.Verdict)
	}
	if ctrl.RunStatus != statusInvalid {
		t.Errorf("CTRL-001 run_status = %q, want %q", ctrl.RunStatus, statusInvalid)
	}

	rng := byKey["RANGE-001|LIVE"]
	if rng.Category != "ranges" {
		t.Errorf("RANGE-001 category = %q, want %q (from the === RUN subtest name)", rng.Category, "ranges")
	}
	if rng.RunStatus != statusOK {
		t.Errorf("RANGE-001 run_status = %q, want %q: the ranges batch passed its gate", rng.RunStatus, statusOK)
	}
	if rng.PGVersion != "17.8" || rng.VoyagerCommit != "abc123" {
		t.Errorf("run metadata not stamped onto the row: %+v", rng)
	}

	drop := byKey["RANGE-009|LIVE"]
	if drop.SourceValue != "[1,5)" || drop.TargetValue != "NULL" {
		t.Errorf("verbatim values not lifted out of the detail: source=%q target=%q", drop.SourceValue, drop.TargetValue)
	}

	hstore := byKey["HSTORE-001|LIVE"]
	if hstore.RunStatus != statusInvalid {
		t.Errorf("HSTORE-001 run_status = %q, want %q", hstore.RunStatus, statusInvalid)
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
