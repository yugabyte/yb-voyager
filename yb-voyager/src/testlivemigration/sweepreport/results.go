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

/*
The results model: one Row per (probe, mode) measured by one sweep run.

The sweep harness prints exactly one line per probe to stdout

	PROBE-RESULT: <id> | <type> | <mode> | <verdict> | <detail>

plus per-run gate lines

	PROBE-RUN-INVALID: <batch> | <mode> | ...
	PROBE-RUN-FLAKE:   <batch> | <mode> | ...
	PROBE-RUN-POISON:  <batch> | <mode> | ...

and, for the one verdict that survives its own run's gate failure,

	PROBE-PUBLISHABLE: <id> | <mode> | <verdict> | <why>

This file turns that text into a stable, diffable CSV. Nothing here talks to a database
or imports voyager, so it builds and tests without Docker and without a build tag.
*/

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// csvHeader is the stable column order of a results file. APPEND ONLY: a released
// results CSV must stay readable by a later differ, so never reorder or remove a column.
var csvHeader = []string{
	"run_timestamp",
	"voyager_commit",
	"pg_version",
	"yb_version",
	"probe_id",
	"type_name",
	"category",
	"mode",
	"verdict",
	"evidence",
	"source_value",
	"target_value",
	"run_status",
	// Appended after the first release of this format. Readers look columns up by name,
	// so an older CSV simply reports "" here rather than failing to parse.
	"sqlstate",
	// Appended for the timestamp/version/attribution fixes below. Same rule: an older
	// CSV just reports "" for these, a newer reader must not require them.
	"import_error",
	"timestamp_source",
}

// Run status values. Anything other than statusOK/statusAttributed means the run that
// produced the row did not satisfy the harness's own gates, so the verdict must not be
// published.
const (
	statusOK      = "OK"      // controls passed, no flake marker
	statusInvalid = "INVALID" // a known-good control did not come out WORKS
	statusFlake   = "FLAKE"   // export never streamed, or probes came out INCONCLUSIVE
	statusPoison  = "POISON"  // deliberate poison-isolation run; control gate N/A

	// statusAttributed is the other side of the control gate's most important exception.
	//
	// A datatype that wedges the importer (or the exporter) also wedges the shared
	// CONTROLS running in the same batch - CTRL-001/CTRL-002 ride the same connector and
	// the same import pipeline as every other probe in the run. So a run that probes such
	// a type can NEVER pass its own control gate: the very thing worth finding makes the
	// gate fail. Under a gate that treats every control failure as "untrustworthy run",
	// that finding is unreportable by construction - which is exactly backwards for a tool
	// whose entire job is finding types that do this.
	//
	// The fix asks what ELSE could have caused the control failure, which depends on the
	// run's shape:
	//   - a SOLO run (this probe is the only non-control probe in its go-test subtest, i.e.
	//     its (batch, mode)) has no other suspect. The controls dying is the expected
	//     consequence of the probe under test, not evidence of a broken harness, so the
	//     verdict is ATTRIBUTED: trustworthy, but flagged as reached via the solo carve-out
	//     rather than a clean gate pass.
	//   - a BATCH run (two or more non-control probes sharing the run) has no way to pin
	//     the failure on any one of them, so it stays statusInvalid: collateral damage,
	//     not attributable to any single probe's row.
	//
	// DO NOT collapse this back into a blunt "any control failure -> INVALID" gate. That
	// was tried; it silently discarded every solo-probe finding of exactly the kind this
	// harness exists to surface (measured in production: ~96 rows). If a future reader
	// looks at this and wants to "simplify", the simplification is the bug.
	statusAttributed = "ATTRIBUTED"
)

// isTrustedStatus reports whether a row's run_status means the measurement stands: either
// the run's gate genuinely passed (OK), the row was reached through the one carve-out that
// pins a control failure on the sole probe that could have caused it (ATTRIBUTED), or the
// gate does not apply (POISON, a deliberate poison-isolation run). INVALID and FLAKE are
// the only untrustworthy statuses. Used wherever "OK" alone used to be the bar - the merge
// preference in dedupe, most notably - so that a trustworthy row is never displaced by an
// untrustworthy one just because the latter happens to carry a different label than OK.
//
// An EMPTY status counts as trusted. ParseLog never produces one (Row is built with
// statusOK and the gates only ever narrow it), but ReadCSV can: a results CSV written
// before this column existed, or one edited by hand, leaves the field blank. "Absent"
// has always meant "no gate ever objected" - the report generator reads it that way too
// (`run_status or "OK"` in page/build_page.py) - and treating blank as UNtrusted here
// instead would make `diff` silently drop every row of such a CSV, reporting a clean
// no-change run where the truth is that nothing was compared at all.
func isTrustedStatus(status string) bool {
	return status == "" || status == statusOK || status == statusPoison || status == statusAttributed
}

// Row is one measured (probe, mode) pair.
type Row struct {
	RunTimestamp  string `json:"run_timestamp"`
	VoyagerCommit string `json:"voyager_commit"`
	PGVersion     string `json:"pg_version"`
	YBVersion     string `json:"yb_version"`
	ProbeID       string `json:"probe_id"`
	TypeName      string `json:"type_name"`
	Category      string `json:"category"`
	Mode          string `json:"mode"`
	Verdict       string `json:"verdict"`
	Evidence      string `json:"evidence"`
	SourceValue   string `json:"source_value"`
	TargetValue   string `json:"target_value"`
	RunStatus     string `json:"run_status"`

	// SQLState is the five-character SQLSTATE lifted out of the evidence when the
	// importer recorded one. sweepRun.importAbortReason now quotes the real database
	// error rather than a bare exit status, so a genuine product stall carries its code.
	// It gets its own column because "same verdict, different SQLSTATE" between two
	// releases is a real change that a prose diff of the evidence would bury.
	SQLState string `json:"sqlstate,omitempty"`

	// ImportError is the classified importer error line (trimmed, pipes replaced) found
	// in the same log as a solo ATTRIBUTED row whose own PROBE-RESULT detail did not carry
	// a SQLSTATE. See fillAttributedSQLState: the detail text for these rows describes a
	// comparison outcome ("row missing on destination"), not the import error that caused
	// it, so the SQLSTATE and the error text are lifted from elsewhere in the same log.
	ImportError string `json:"import_error,omitempty"`

	// TimestampSource records how RunTimestamp was obtained: an explicit -timestamp flag,
	// a RUN-META line, the first timestamp found in the log body, the run-<stamp>.log
	// filename, the log file's own mtime, or (last resort) the moment collect ran. Without
	// this a reader cannot tell a measured run time from a guess.
	TimestampSource string `json:"timestamp_source,omitempty"`
}

// Key identifies a row across runs. A results file holds at most one row per key.
func (r Row) Key() string { return r.ProbeID + "|" + r.Mode }

func (r Row) record() []string {
	return []string{
		r.RunTimestamp, r.VoyagerCommit, r.PGVersion, r.YBVersion,
		r.ProbeID, r.TypeName, r.Category, r.Mode, r.Verdict,
		r.Evidence, r.SourceValue, r.TargetValue, r.RunStatus, r.SQLState,
		r.ImportError, r.TimestampSource,
	}
}

func rowFromRecord(header []string, rec []string) Row {
	get := func(name string) string {
		for i, h := range header {
			if h == name && i < len(rec) {
				return rec[i]
			}
		}
		return ""
	}
	return Row{
		RunTimestamp:    get("run_timestamp"),
		VoyagerCommit:   get("voyager_commit"),
		PGVersion:       get("pg_version"),
		YBVersion:       get("yb_version"),
		ProbeID:         get("probe_id"),
		TypeName:        get("type_name"),
		Category:        get("category"),
		Mode:            get("mode"),
		Verdict:         get("verdict"),
		Evidence:        get("evidence"),
		SourceValue:     get("source_value"),
		TargetValue:     get("target_value"),
		RunStatus:       get("run_status"),
		SQLState:        get("sqlstate"),
		ImportError:     get("import_error"),
		TimestampSource: get("timestamp_source"),
	}
}

// RunMeta is the per-run provenance stamped onto every row.
type RunMeta struct {
	Timestamp     string
	VoyagerCommit string
	PGVersion     string
	YBVersion     string

	// TimestampSource says how Timestamp was obtained. Set by deriveRunMeta; a caller that
	// builds a RunMeta by hand (tests, an explicit -timestamp) leaves it empty or sets
	// tsSourceExplicit itself.
	TimestampSource string
}

// ============================================================
// PARSING A go test LOG
// ============================================================

var (
	// PROBE-RESULT: <id> | <type> | <mode> | <verdict> | <detail>
	probeResultRe = regexp.MustCompile(`PROBE-RESULT:\s*(.*)$`)
	// PROBE-RUN-INVALID / -FLAKE / -POISON: <batch> | <mode> | <reason>
	probeRunRe = regexp.MustCompile(`PROBE-RUN-(INVALID|FLAKE|POISON):\s*([^|]*)\|([^|]*)\|`)
	// === RUN   TestDatatypeSweepLive/ranges  -- gives us the batch (category) of the
	// probe lines that follow. The sweep never calls t.Parallel(), so subtests do not
	// interleave and "most recent RUN line wins" is exact.
	//
	// The subtest half is OPTIONAL. A solo run logs a bare "=== RUN
	// TestDatatypeSweepSuspect" with no subtest at all; when the subtest was mandatory
	// that line did not match, so a new top-level test silently inherited whatever batch
	// the previous one left open. Matching it (with an empty batch) resets the attribution
	// instead.
	goTestRunRe = regexp.MustCompile(`=== RUN\s+TestDatatypeSweep(\w+)(?:/([\w.\-]+))?`)
	// PROBE-VALUES: <id> | <mode> | <source> | <destination>
	// The structured form, preferred over verbatimRe below.
	probeValuesRe = regexp.MustCompile(`PROBE-VALUES:\s*(.*)$`)
	// PROBE-PUBLISHABLE: <id> | <mode> | <verdict> | <why>
	probePublishableRe = regexp.MustCompile(`PROBE-PUBLISHABLE:\s*(.*)$`)
	// Values the harness embeds in the PROSE detail for the baseline row. This is the
	// FALLBACK, kept so logs from before the PROBE-VALUES line still yield values.
	verbatimRe = regexp.MustCompile(`id=\d+ source=(NULL|<row absent>|"(?:[^"]*)") destination=(NULL|<row absent>|"(?:[^"]*)")`)
	// A PostgreSQL SQLSTATE as it appears in an importer error, e.g. "(0A000)" or
	// "SQLSTATE 22P02". Five characters, digits and upper-case letters, first a digit.
	sqlStateRe = regexp.MustCompile(`(?:SQLSTATE:?\s*|\()([0-9][0-9A-Z]{4})\b`)

	// importErrorLineRe pulls a classified importer error with a quotable SQLSTATE out of
	// a plain log line, e.g.
	//   ... step=copy: ERROR: DECIMAL does not support NaN yet (SQLSTATE 0A000): dbcontext=...
	// Used only by the SQLSTATE lift for solo ATTRIBUTED rows: such a row's own
	// PROBE-RESULT detail often describes a COMPARISON outcome ("row missing on
	// destination"), not the import error that produced it, so the SQLSTATE has to be
	// lifted from elsewhere in the log.
	importErrorLineRe = regexp.MustCompile(`(ERROR:.*\(SQLSTATE\s*([0-9][0-9A-Z]{4})\))`)

	// importErrNoiseRe recognizes importer errors that are harness/metadata bookkeeping
	// rather than a statement about the datatype under test. The common one is
	//   ERROR: relation "ybvoyager_metadata.ybvoyager_imported_event_count_by_table"
	//          does not exist (SQLSTATE 42P01)
	// which is voyager's OWN reporting query failing because the import already died. It
	// often precedes the real error in the log, so a plain "first error wins" lift reports
	// 42P01 for a type whose actual problem was something else entirely. Never lifted.
	importErrNoiseRe = regexp.MustCompile(
		`(?i)ybvoyager_metadata|ybvoyager_imported_event_count|ybvoyager_import_data_batches`)

	// simpleTypeTokenRe matches a probe TypeName that is one plain identifier ("vector",
	// "hstore"). Decorated names ("domain(xml)", "numeric (NaN)") are rejected: matching
	// those against error prose produces false positives, not evidence.
	simpleTypeTokenRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,}$`)

	// containerSetupFailureRe recognizes a harness/environment failure during container
	// setup - the probe was never actually exercised, so a row whose evidence matches this
	// is never a datatype finding and must never read ATTRIBUTED or OK, solo run or not.
	containerSetupFailureRe = regexp.MustCompile(
		`(?i)container setup failed|insufficient disk space|manifest for .*? not found|no space left on device`)
)

// ParseLog turns a captured `go test` log into rows.
//
// categoryFor, when non-nil, is the authoritative probe-id -> group mapping from the
// generated probe catalog. The `=== RUN` batch name is only a fallback for logs
// collected without a catalog (e.g. a PROBE_ID solo run, whose subtest is "solo_...").
func ParseLog(r io.Reader, meta RunMeta, categoryFor func(probeID string) string) ([]Row, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)

	var rows []Row
	// status per "<batch>|<mode>"; probes inherit the status of the run they came from.
	runStatus := map[string]string{}
	// rows are attributed to a batch so a later gate line can retro-mark them.
	rowBatch := map[int]string{}
	// the bare batch name of each row (rowBatch without the mode half), used to find the
	// run's importer errors - see importErrByBatch below.
	rowBatchName := map[int]string{}
	// "<id>|<mode>" -> the verdict the harness declared publishable despite its run's
	// gate. Keyed by verdict as well as by row so a marker can never promote a verdict
	// the classifier did not actually produce.
	publishable := map[string]string{}
	// "<batch>" -> the classified importer errors seen while scanning that batch, in file
	// order. Fallback evidence for a solo ATTRIBUTED row whose own detail carries no
	// SQLSTATE (see the sqlstate column's doc in DATATYPE_SWEEP.md).
	//
	// Keyed by BATCH ALONE, deliberately, not by the "<batch>|<mode>" group key the gates
	// use. A solo run's "=== RUN TestDatatypeSweepSuspect" line carries neither a subtest
	// nor a recognizable mode, so a group key built from it is "|" - while the rows from
	// that same run look themselves up under "|LIVE", because a row's mode comes from its
	// own PROBE-RESULT line, not from the RUN line. The two keys never matched and the
	// lift never fired on a real solo log. The batch alone is enough: within one log a
	// batch is one go-test subtest of one mode, so the batch already implies the mode, and
	// the row supplies the mode itself.
	importErrByBatch := map[string][]importErrEvidence{}
	curBatch := ""
	curMode := ""

	for sc.Scan() {
		line := sc.Text()

		if m := importErrorLineRe.FindStringSubmatch(line); m != nil {
			// Keep several candidates, not just the first: which one is the best evidence
			// for a given row is decided later, by pickImportError.
			if c := importErrByBatch[curBatch]; len(c) < maxImportErrCandidates {
				importErrByBatch[curBatch] = append(c, importErrEvidence{
					sqlstate: m[2],
					text:     sanitizeImportError(m[1]),
					noise:    importErrNoiseRe.MatchString(m[1]),
					haystack: strings.ToLower(trimTo(line, 1000)),
				})
			}
		}

		if m := goTestRunRe.FindStringSubmatch(line); m != nil {
			curMode = modeFromTestName(m[1])
			curBatch = m[2]
			continue
		}

		if m := probeRunRe.FindStringSubmatch(line); m != nil {
			batch := strings.TrimSpace(m[2])
			mode := strings.TrimSpace(m[3])
			key := batch + "|" + mode
			switch m[1] {
			case "INVALID":
				runStatus[key] = statusInvalid
			case "POISON":
				runStatus[key] = statusPoison
			case "FLAKE":
				// INVALID is the stronger signal; never downgrade it.
				if runStatus[key] != statusInvalid {
					runStatus[key] = statusFlake
				}
			}
			continue
		}

		if m := probePublishableRe.FindStringSubmatch(line); m != nil {
			f := splitPipes(m[1])
			if len(f) < 3 {
				return nil, fmt.Errorf("malformed PROBE-PUBLISHABLE line (want 4 pipe-separated fields): %q", line)
			}
			mode := f[1]
			if mode == "" {
				mode = curMode
			}
			publishable[f[0]+"|"+mode] = f[2]
			continue
		}

		// The structured values line. It follows its own PROBE-RESULT line, so the row
		// is already in `rows` and is patched in place.
		if m := probeValuesRe.FindStringSubmatch(line); m != nil {
			f := splitPipes(m[1])
			if len(f) < 4 {
				return nil, fmt.Errorf("malformed PROBE-VALUES line (want 4 pipe-separated fields): %q", line)
			}
			id, mode := f[0], f[1]
			if mode == "" {
				mode = curMode
			}
			for i := range rows {
				if rows[i].ProbeID == id && rows[i].Mode == mode {
					rows[i].SourceValue = unescapeValue(f[2])
					rows[i].TargetValue = unescapeValue(f[3])
				}
			}
			continue
		}

		m := probeResultRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		fields := splitPipes(m[1])
		if len(fields) < 4 {
			return nil, fmt.Errorf("malformed PROBE-RESULT line (want 5 pipe-separated fields): %q", line)
		}
		detail := ""
		if len(fields) >= 5 {
			detail = fields[4]
		}
		src, dst := verbatimValues(detail)

		row := Row{
			RunTimestamp:    meta.Timestamp,
			VoyagerCommit:   meta.VoyagerCommit,
			PGVersion:       meta.PGVersion,
			YBVersion:       meta.YBVersion,
			ProbeID:         fields[0],
			TypeName:        fields[1],
			Mode:            fields[2],
			Verdict:         fields[3],
			Evidence:        detail,
			SourceValue:     src,
			TargetValue:     dst,
			RunStatus:       statusOK,
			SQLState:        sqlStateOf(detail),
			TimestampSource: meta.TimestampSource,
		}
		if categoryFor != nil {
			row.Category = categoryFor(row.ProbeID)
		}
		if row.Category == "" {
			row.Category = curBatch
		}
		if row.Mode == "" {
			row.Mode = curMode
		}
		rowBatch[len(rows)] = curBatch + "|" + row.Mode
		rowBatchName[len(rows)] = curBatch
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading test log: %w", err)
	}

	// The marker line's key (<batch>|<mode>, copied by the harness into text) can
	// silently fail to match rowBatch's key built from the "=== RUN" line - a batch-name
	// formatting difference between the two lines has been observed in practice - and
	// when the keys don't match, a run whose control genuinely failed is left at
	// run_status=OK because runStatus[rowBatch[i]] simply misses. A silent miss like that
	// is worse than a marker that never fires at all: nothing downstream can tell the
	// difference between "the gate passed" and "the gate never ran".
	//
	// So the gate is ALSO derived straight from the row data, which cannot go out of sync
	// with itself the way two independently-formatted strings can: for each (batch, mode)
	// group, if any of that group's CTRL-* rows came out something other than WORKS, every
	// row in the group is untrustworthy - marker or no marker, matching key or not. A
	// SKIPPED control is not a failure (it means the mode was never exercised, which is a
	// coverage fact, not a broken measurement), so it is ignored here.
	//
	// The group key is rowBatch[i] - the SAME "<batch>|<mode>" key the marker gate already
	// uses, built from the row's own "=== RUN" attribution rather than from a marker line's
	// text. This has to be (file, batch, mode), not (file, mode): a single log routinely
	// contains SEVERAL batches in the same mode (e.g. three "=== RUN
	// TestDatatypeSweepOffline/<batch>" subtests in one OFFLINE log), each batch its own
	// go-test subtest with its own control probes. Grouping by mode alone was tried and is
	// wrong in BOTH directions: a bad batch's failure would contaminate its unrelated
	// neighbours in the same mode (discarding good measurements), or - if the gate instead
	// only required ONE passing control anywhere in the mode - a spoiled batch could be
	// blessed by a clean neighbour's controls (publishing bad ones). Keying on the same
	// per-row batch attribution the marker gate uses keeps each subtest's verdict from
	// leaking into any other subtest's, in either direction.
	//
	// Rows with no recorded batch (curBatch == "", e.g. a hand-trimmed fixture or a stdin
	// fragment with no "=== RUN" line at all) get rowBatch[i] == "|"+mode for every such
	// row, which groups them together by mode alone rather than merging them into whatever
	// real batch happens to be currently open - this is the deliberate, documented fallback
	// for that degenerate case, not an oversight.
	//
	// A control dying is not always collateral damage, though: see statusAttributed's doc
	// for the SOLO-run carve-out this gate must make before it lands on statusInvalid. That
	// needs to know how many DISTINCT non-control probes shared this (batch, mode) group -
	// one means there was nothing else that could have caused the control failure, two or
	// more means the failure cannot be pinned on any single one of them. This count is
	// PER GROUP: a 3-probe batch is never "solo" just because it shares a file with two
	// other batches in the same mode.
	ctrlBad := map[string]bool{}                  // "<batch>|<mode>" -> a CTRL-* row in that group failed
	nonCtrlProbes := map[string]map[string]bool{} // "<batch>|<mode>" -> set of distinct non-CTRL-* probe ids
	for i, r := range rows {
		key := rowBatch[i]
		if strings.HasPrefix(r.ProbeID, "CTRL-") {
			if r.Verdict != "WORKS" && r.Verdict != "SKIPPED" {
				ctrlBad[key] = true
			}
			continue
		}
		if nonCtrlProbes[key] == nil {
			nonCtrlProbes[key] = map[string]bool{}
		}
		nonCtrlProbes[key][r.ProbeID] = true
	}

	for i := range rows {
		if st, ok := runStatus[rowBatch[i]]; ok {
			rows[i].RunStatus = st
		}
		// The data-derived gate, applied after the marker so it can catch what the marker
		// missed - but never applied to a POISON run: a poison-isolation run's control
		// gate is N/A by design (see statusPoison), so a control coming out non-WORKS
		// there is expected, not evidence the run is untrustworthy.
		//
		// Three-way, not binary: when this (batch, mode) group had exactly one non-control
		// probe, that probe was the ONLY thing that could have caused its controls to die,
		// so ITS row is ATTRIBUTED rather than INVALID - see statusAttributed. The controls'
		// OWN rows do not get this promotion even in that case: CTRL-001/002 still came out
		// something other than WORKS, so they were never actually measured and stay
		// INVALID, exactly as they would in a run with no carve-out at all. Two or more
		// non-control probes means the failure cannot be pinned on any one of them, so
		// nothing in that group is attributable: everything stays INVALID.
		group := rowBatch[i]
		if rows[i].RunStatus != statusPoison && ctrlBad[group] {
			if !strings.HasPrefix(rows[i].ProbeID, "CTRL-") && len(nonCtrlProbes[group]) == 1 {
				rows[i].RunStatus = statusAttributed
				// The row's own detail is a comparison outcome, not an import error, so it
				// often carries no SQLSTATE even though the control died of a real,
				// classified importer error right next to it in the same group. Surface
				// that evidence rather than leaving the column blank - see ImportError's
				// doc. Never overwrites a SQLSTATE the detail already carried.
				if rows[i].SQLState == "" {
					if ie, ok := pickImportError(importErrByBatch[rowBatchName[i]], rows[i]); ok {
						rows[i].SQLState = ie.sqlstate
						rows[i].ImportError = ie.text
					}
				}
			} else {
				rows[i].RunStatus = statusInvalid
			}
		}
		// The PROBE-PUBLISHABLE carve-out, applied AFTER both run-status signals so it can
		// override either of them. An attributed export death is the finding rather than a
		// broken measurement, and the controls going inconclusive is a consequence of it -
		// so that one row is publishable even though its run (and its mode, under the
		// data-derived gate above) is not. It promotes only the exact (probe, mode,
		// verdict) the harness named; every other row from the run keeps its status and
		// stays out of the report and the differ.
		if v, ok := publishable[rows[i].Key()]; ok && v == rows[i].Verdict {
			rows[i].RunStatus = statusOK
		}

		// A container-setup failure (disk exhaustion, a bad image tag, ...) means the
		// probe was never actually exercised: whatever verdict the classifier printed is a
		// harness/environment artefact, not a measurement of the type. This overrides
		// EVERYTHING computed above, including a solo ATTRIBUTED promotion and a
		// PROBE-PUBLISHABLE override - such a row must never be published as a finding.
		if containerSetupFailureRe.MatchString(rows[i].Evidence) {
			rows[i].RunStatus = statusInvalid
		}
	}
	return dedupe(rows), nil
}

// modeFromTestName maps the test-function suffix to the mode string the harness prints.
func modeFromTestName(suffix string) string {
	switch strings.ToLower(suffix) {
	case "offline":
		return "OFFLINE"
	case "live":
		return "LIVE"
	case "fallback":
		return "FALL-BACK"
	case "fallforward":
		return "FALL-FORWARD"
	default:
		return ""
	}
}

// splitPipes splits the PROBE-RESULT payload. Only the first four separators are
// structural; the harness already replaced any "|" inside the detail with "/", but
// splitting with a limit keeps that guarantee local.
func splitPipes(s string) []string {
	parts := strings.SplitN(s, "|", 5)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// verbatimValues lifts the source/destination sample the harness embeds in the detail
// (`verbatim: id=1 source="..." destination="..."`) into their own columns. Best effort:
// a detail without a verbatim section yields two empty strings.
func verbatimValues(detail string) (string, string) {
	m := verbatimRe.FindStringSubmatch(detail)
	if m == nil {
		return "", ""
	}
	return strings.Trim(m[1], `"`), strings.Trim(m[2], `"`)
}

// unescapeValue reverses sanitizeValue in the harness, which escapes rather than rewrites
// the structural characters so a value containing a pipe or a newline survives the round
// trip byte-for-byte.
//
// This has to be a single left-to-right scan, not chained replacements. The harness
// escapes the backslash first, so a value that literally contains `\x7c` is emitted as
// `\\x7c` - and any decoder that looks for `\x7c` anywhere in the string finds it at
// offset 1 and turns a literal backslash-x-7-c into a pipe. Scanning consumes the
// backslash escape before it can be misread as the start of another one.
func unescapeValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			i++
			continue
		}
		switch {
		case strings.HasPrefix(s[i:], `\\`):
			b.WriteByte('\\')
			i += 2
		case strings.HasPrefix(s[i:], `\x7c`):
			b.WriteByte('|')
			i += 4
		case strings.HasPrefix(s[i:], `\n`):
			b.WriteByte('\n')
			i += 2
		case strings.HasPrefix(s[i:], `\r`):
			b.WriteByte('\r')
			i += 2
		case strings.HasPrefix(s[i:], `\t`):
			b.WriteByte('\t')
			i += 2
		default:
			// A lone backslash the harness did not write. Pass it through rather than
			// guessing, so an unknown escape is visible instead of silently eaten.
			b.WriteByte('\\')
			i++
		}
	}
	return b.String()
}

// sqlStateOf lifts a SQLSTATE out of the evidence, or "" when there is none.
func sqlStateOf(detail string) string {
	m := sqlStateRe.FindStringSubmatch(detail)
	if m == nil {
		return ""
	}
	return m[1]
}

// importErrEvidence is one classified importer error line seen while scanning a batch.
// See importErrByBatch in ParseLog.
type importErrEvidence struct {
	sqlstate string
	text     string // the quotable "ERROR: ... (SQLSTATE ...)" clause, sanitized for a CSV cell
	noise    bool   // harness/metadata bookkeeping, not a statement about the datatype
	haystack string // the whole log line, lower-cased, for the "does it name the probe?" test
}

// maxImportErrCandidates caps how many importer errors are remembered per batch. A wedged
// import can repeat the same error thousands of times; the useful candidates are all near
// the front.
const maxImportErrCandidates = 32

// pickImportError chooses which of a run's classified importer errors to lift onto a solo
// ATTRIBUTED row, in this order:
//
//  1. Harness/metadata bookkeeping errors are dropped outright (importErrNoiseRe). They
//     are voyager's own reporting queries failing because the import already died, they
//     often come FIRST in the log, and they say nothing about the type under test.
//  2. An error that names the probe - its table p_<id>, or its type name - beats a generic
//     one from the same run.
//  3. Otherwise the first surviving error in file order wins.
//
// Reports false when the run had no usable importer error at all, which leaves both the
// sqlstate and import_error columns empty rather than filling them with noise.
func pickImportError(cands []importErrEvidence, r Row) (importErrEvidence, bool) {
	var first importErrEvidence
	found := false
	for _, c := range cands {
		if c.noise {
			continue
		}
		if mentionsProbe(c.haystack, r) {
			return c, true
		}
		if !found {
			first, found = c, true
		}
	}
	return first, found
}

// probeTableName is the table the harness creates for a probe: p_<id>, lower-cased with
// "-" turned into "_" (VAL-006 -> p_val_006).
func probeTableName(probeID string) string {
	return "p_" + strings.ToLower(strings.ReplaceAll(probeID, "-", "_"))
}

// mentionsProbe reports whether a (lower-cased) log line names the probe under test.
func mentionsProbe(lowerLine string, r Row) bool {
	if r.ProbeID != "" && strings.Contains(lowerLine, probeTableName(r.ProbeID)) {
		return true
	}
	if t := strings.ToLower(strings.TrimSpace(r.TypeName)); simpleTypeTokenRe.MatchString(t) {
		return strings.Contains(lowerLine, t)
	}
	return false
}

// sanitizeImportError trims an importer error line to something worth putting in a CSV
// cell: pipes replaced (the same convention the harness's own detail text uses, see
// splitPipes), capped at 200 characters.
func sanitizeImportError(s string) string {
	return trimTo(strings.ReplaceAll(s, "|", "/"), 200)
}

// ============================================================
// RUN PROVENANCE: TIMESTAMP, COMMIT, PG/YB VERSION
// ============================================================
//
// collect used to stamp every row in every log with ONE timestamp: either an explicit
// -timestamp flag, or (when that flag was empty, as it always was in practice) the moment
// `collect` happened to run - not when the sweep itself ran. Feeding several logs collected
// together to `collect` then gave every row from every run the SAME timestamp, which makes
// preferRow's "later run wins" tie-break inert: ties are broken by parse order instead,
// which is filename order, not truth. That silently lost real WORKS verdicts to a later
// SKIPPED run parsed after them.
//
// The fix: derive a timestamp PER LOG FILE, only when the caller did not pin one with
// -timestamp. THE PRIORITY ORDER, highest first:
//
//  1. explicit     - an explicit -timestamp flag on the command line.
//  2. run-meta     - a `RUN-META: started=<RFC3339> ...` line, if the harness ever prints
//     one (it does not yet - parsed defensively so collect picks it up for
//     free the day probe.go grows one).
//  3. log-body     - the first ZONED timestamp in the log's own text (voyager's logrus
//     lines, or a "... UTC" server line). A zone makes it unambiguous.
//  4. filename     - the `run-<YYYYMMDDTHHMMSSZ>.log` name run-datatype-sweep.sh writes.
//     That stamp is generated with `date -u`, so it is true UTC.
//  5. log-body-local - the first ZONELESS timestamp in the log's own text. Go's default
//     log package stamp ("2026/09/03 16:18:13"), which testcontainers
//     uses, is the MACHINE'S LOCAL time with no zone printed. It is
//     parsed in time.Local and ranked BELOW the filename, because
//     reading it as UTC put a run 5h30m (Asia/Kolkata) away from when
//     it actually ran - and the filename right next to it was correct.
//  6. mtime        - the log file's own mtime.
//  7. collect-time - the moment collect ran. The absolute last resort, not the default.
//
// Each row records which of these produced its timestamp in TimestampSource, so a reader
// can tell a measured time from a guess, and a local-clock reading from a UTC one.
const (
	tsSourceExplicit     = "explicit"       // an explicit -timestamp flag on the command line
	tsSourceRunMeta      = "run-meta"       // a RUN-META line in the log (not emitted yet)
	tsSourceLogBody      = "log-body"       // the first ZONED timestamp in the log's own text
	tsSourceFilename     = "filename"       // the run-<stamp>.log filename pattern (true UTC)
	tsSourceLogBodyLocal = "log-body-local" // a ZONELESS log stamp, read in the local zone
	tsSourceMtime        = "mtime"          // the log file's own mtime
	tsSourceNow          = "collect-time"   // last resort: nothing else was derivable
)

var (
	// RUN-META: started=<RFC3339> commit=<sha> pg=<ver> yb=<ver> - not emitted by the
	// harness today. commit/pg/yb are all optional; only `started` is required for the
	// line to match at all.
	runMetaRe = regexp.MustCompile(`RUN-META:\s*started=(\S+)(?:\s+commit=(\S+))?(?:\s+pg=(\S+))?(?:\s+yb=(\S+))?`)

	// runLogFilenameRe matches the run-<stamp>.log filename run-datatype-sweep.sh writes.
	runLogFilenameRe = regexp.MustCompile(`^run-(\d{8}T\d{6}Z)\.log$`)

	// pgImageRe / ybImageRe pull the source/target versions out of the testcontainers
	// image-pull lines ("...for image postgres:17.8-ext", "...image
	// yugabytedb/yugabyte:2026.1.0.0-b118"). Used only when nothing more direct is
	// available - see deriveRunMeta.
	pgImageRe = regexp.MustCompile(`\bpostgres:(\d+(?:\.\d+){0,3}(?:-[A-Za-z0-9]+)?)`)
	ybImageRe = regexp.MustCompile(`yugabytedb/yugabyte:(\d+(?:\.\d+){0,3}(?:-[A-Za-z0-9]+)?)`)

	// tsCandidateRe finds the first recognizable timestamp anywhere in a log's body. The
	// three alternatives are tried in the order they occur in the file, NOT in a fixed
	// format-preference order: an RFC3339-ish stamp (voyager's own logrus lines), Go's
	// default log-package stamp (testcontainers' banner - present in essentially every
	// sweep log, and usually the earliest timestamp in the file), or a PostgreSQL server
	// log line.
	tsCandidateRe = regexp.MustCompile(
		`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})` +
			`|\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}` +
			`|\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)? UTC`)

	// tsLayouts are tried in order against whatever tsCandidateRe matched. `zoned` says
	// whether the text itself carries a zone; a zoneless layout is parsed in time.Local,
	// because the only zoneless producer here is Go's log package printing the machine's
	// own wall clock.
	tsLayouts = []struct {
		layout string
		zoned  bool
	}{
		{time.RFC3339Nano, true},
		{time.RFC3339, true},
		{"2006-01-02T15:04:05Z0700", true},
		{"2006/01/02 15:04:05", false},
		{"2006-01-02 15:04:05 MST", true},
		{"2006-01-02 15:04:05.999999 MST", true},
		{"2006-01-02 15:04:05.999 MST", true},
	}
)

// bodyTimestamp is the first timestamp found in a log's body.
type bodyTimestamp struct {
	t time.Time
	// zoned is true when the log text stated a zone. When it is false the time was read
	// in time.Local, which is a guess about the machine that produced the log - see the
	// priority order above, where such a stamp ranks below the filename's true UTC.
	zoned bool
}

// firstBodyTimestamp returns the first timestamp found anywhere in content, in file order.
func firstBodyTimestamp(content []byte) (bodyTimestamp, bool) {
	loc := tsCandidateRe.FindIndex(content)
	if loc == nil {
		return bodyTimestamp{}, false
	}
	s := string(content[loc[0]:loc[1]])
	for _, l := range tsLayouts {
		if l.zoned {
			if t, err := time.Parse(l.layout, s); err == nil {
				return bodyTimestamp{t: t, zoned: true}, true
			}
			continue
		}
		if t, err := time.ParseInLocation(l.layout, s, time.Local); err == nil {
			return bodyTimestamp{t: t}, true
		}
	}
	return bodyTimestamp{}, false
}

// deriveRunMeta fills in whatever fields of base are empty, from the log's own content and
// path. A field the caller already set (an explicit CLI flag) is never overridden - that
// is what lets -timestamp/-pg-version/-yb-version keep working exactly as before for a
// caller that wants one fixed value stamped on every row.
func deriveRunMeta(path string, content []byte, base RunMeta) RunMeta {
	m := base
	started, commit, pg, yb, hasMeta := parseRunMeta(content)
	// RUN-META degrades every field to the literal "unknown" rather than failing the run
	// (emitRunMeta / serverVersion in datatype_sweep_probe.go): a DB that was unreachable
	// when the header was printed still yields "pg=unknown yb=unknown". That is not a
	// version, so treat it as absent and let the testcontainers image tag below fill it in.
	commit, pg, yb = blankUnknown(commit), blankUnknown(pg), blankUnknown(yb)

	if m.Timestamp == "" {
		if hasMeta {
			if t, err := time.Parse(time.RFC3339, started); err == nil {
				m.Timestamp = t.UTC().Format(time.RFC3339)
				m.TimestampSource = tsSourceRunMeta
			}
		}
		if m.Timestamp == "" {
			m.Timestamp, m.TimestampSource = deriveTimestampFallback(path, content)
		}
	} else {
		m.TimestampSource = tsSourceExplicit
	}

	if m.VoyagerCommit == "" && commit != "" {
		m.VoyagerCommit = commit
	}
	if m.PGVersion == "" {
		if pg != "" {
			m.PGVersion = pg
		} else if v := pgImageRe.FindSubmatch(content); v != nil {
			m.PGVersion = string(v[1])
		}
	}
	if m.YBVersion == "" {
		if yb != "" {
			m.YBVersion = yb
		} else if v := ybImageRe.FindSubmatch(content); v != nil {
			m.YBVersion = string(v[1])
		}
	}
	return m
}

// blankUnknown maps the harness's "unknown" placeholder to "", so the callers' "is this
// field empty?" checks treat it as missing.
func blankUnknown(s string) string {
	if strings.EqualFold(s, "unknown") {
		return ""
	}
	return s
}

// parseRunMeta extracts a RUN-META line's fields, if the log has one.
func parseRunMeta(content []byte) (started, commit, pg, yb string, ok bool) {
	m := runMetaRe.FindSubmatch(content)
	if m == nil {
		return "", "", "", "", false
	}
	return string(m[1]), string(m[2]), string(m[3]), string(m[4]), true
}

// deriveTimestampFallback is steps 3-7 of the priority order documented above RunMeta.
func deriveTimestampFallback(path string, content []byte) (string, string) {
	body, hasBody := firstBodyTimestamp(content)
	// A zoned body stamp is unambiguous, so it outranks everything below it.
	if hasBody && body.zoned {
		return body.t.UTC().Format(time.RFC3339), tsSourceLogBody
	}
	// The filename stamp is written with `date -u`, so it is real UTC - unlike a zoneless
	// body stamp, which is only as good as a guess about the producing machine's zone.
	if m := runLogFilenameRe.FindStringSubmatch(filepath.Base(path)); m != nil {
		if t, err := time.Parse("20060102T150405Z", m[1]); err == nil {
			return t.UTC().Format(time.RFC3339), tsSourceFilename
		}
	}
	if hasBody {
		return body.t.UTC().Format(time.RFC3339), tsSourceLogBodyLocal
	}
	if path != "" && path != "-" {
		if fi, err := os.Stat(path); err == nil {
			return fi.ModTime().UTC().Format(time.RFC3339), tsSourceMtime
		}
	}
	return time.Now().UTC().Format(time.RFC3339), tsSourceNow
}

// dedupe collapses a row set down to one row per (probe_id, mode) key, keeping the BEST
// row for that key rather than blindly keeping whichever one was seen first or last.
//
// This one function merges two different situations under one rule:
//   - reruns WITHIN one log (the harness re-runs a batch that flaked), and
//   - independent runs ACROSS MULTIPLE logs supplied to `collect` (see the collect -log /
//     -logs flags): each log is parsed on its own by its own ParseLog call, and the
//     per-file row sets are concatenated and land here to be merged.
//
// The preference order, evaluated in this sequence and stopping at the first strict
// winner - see preferRow:
//
//  1. A trustworthy row beats an untrustworthy one (isTrustedStatus). This is the fix for
//     the historical bug here: dedupe used to do `out[i] = r` unconditionally on a repeat
//     key, so whichever row was parsed LAST won regardless of quality - a row from a run
//     whose controls FAILED could silently overwrite a row from a run whose controls
//     PASSED. A WORKS verdict from a clean run must never be displaced by a worse verdict
//     from a run that did not pass its own gate, no matter which one was parsed later.
//  2. Between two rows of equal trust, an actual observation beats "we could not tell":
//     any real verdict wins over a NON-OBSERVATION (see isNonObservation) - INCONCLUSIVE,
//     or a SKIPPED that skipped because an extension was missing from the image.
//  3. Otherwise the row with the later (or equal) run_timestamp wins - a re-run supersedes
//     what it re-ran. Rows from the same log share a timestamp, so on a true tie this falls
//     back to "the later one in parse order wins", which is exactly the within-log rerun
//     behavior dedupe has always had.
//
// Deliberately absent: any preference keyed on SQLState, and any preference for a FAILURE
// over a WORKS. An earlier version of this merge preferred whichever row carried a
// SQLSTATE, on the theory that a row with more detail is more informative - but SQLSTATE
// only appears on failures, so that rule quietly rewrote "keep the best evidence" into
// "keep the worst outcome": every probe's merged row became its worst run instead of its
// representative one. Do not resurrect that rule.
func dedupe(rows []Row) []Row {
	idx := map[string]int{}
	var out []Row
	for _, r := range rows {
		if i, seen := idx[r.Key()]; seen {
			if preferRow(out[i], r) {
				out[i] = r // rows move WHOLE: never splice a verdict from one row onto
				// the run_status or sqlstate of another.
			}
			continue
		}
		idx[r.Key()] = len(out)
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ProbeID != out[j].ProbeID {
			return out[i].ProbeID < out[j].ProbeID
		}
		return out[i].Mode < out[j].Mode
	})
	return out
}

// preferRow reports whether candidate should replace incumbent under the same
// (probe_id, mode) key. See dedupe for the full rule and why it is ordered this way.
func preferRow(incumbent, candidate Row) bool {
	incTrusted, candTrusted := isTrustedStatus(incumbent.RunStatus), isTrustedStatus(candidate.RunStatus)
	if incTrusted != candTrusted {
		return candTrusted
	}

	incNonObs, candNonObs := isNonObservation(incumbent), isNonObservation(candidate)
	if incNonObs != candNonObs {
		return !candNonObs
	}

	return candidate.RunTimestamp >= incumbent.RunTimestamp
}

// extensionUnavailableRe recognizes a SKIPPED whose reason is that the extension was not
// installed in the image the run happened to use ("extension unavailable: vector on
// source"). That is a fact about the CONTAINER, not about the datatype.
var extensionUnavailableRe = regexp.MustCompile(`(?i)extension (unavailable|missing|not available)`)

// isNonObservation reports whether a row measured nothing about the type, and so must lose
// to any real verdict for the same cell regardless of which run is newer.
//
// Two shapes qualify:
//   - INCONCLUSIVE - the classifier could not tell.
//   - SKIPPED because an extension was unavailable - the probe never ran. Observed in
//     production: VEC-001..005 OFFLINE had been measured WORKS with clean controls, then a
//     later run on an image without pgvector reported SKIPPED, and the plain "later run
//     wins" tie-break let that overwrite five real measurements with a fact about the image.
//
// A SKIPPED for any OTHER reason is deliberately NOT here. "DDL rejected: target: ERROR:
// type not yet supported in Yugabyte (SQLSTATE 0A000)" is the server refusing the type -
// a real observation about the product, which keeps its normal rank and can supersede an
// earlier row like any other verdict.
func isNonObservation(r Row) bool {
	if strings.EqualFold(r.Verdict, "INCONCLUSIVE") {
		return true
	}
	return strings.EqualFold(r.Verdict, "SKIPPED") && extensionUnavailableRe.MatchString(r.Evidence)
}

// ============================================================
// CSV / JSON IO
// ============================================================

func WriteCSV(path string, rows []Row) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(csvHeader); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write(r.record()); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func ReadCSV(path string) ([]Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rd := csv.NewReader(f)
	rd.FieldsPerRecord = -1 // tolerate an older/newer column count
	recs, err := rd.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(recs) == 0 {
		return nil, nil
	}
	header := recs[0]
	rows := make([]Row, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		rows = append(rows, rowFromRecord(header, rec))
	}
	return rows, nil
}

func WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// DefaultResultsPath is where a run lands when the caller sets no path.
func DefaultResultsPath(dir string, meta RunMeta) string {
	stamp := strings.NewReplacer(":", "", "-", "").Replace(meta.Timestamp)
	if stamp == "" {
		stamp = time.Now().UTC().Format("20060102T150405Z")
	}
	return filepath.Join(dir, fmt.Sprintf("datatype-sweep-%s.csv", stamp))
}
