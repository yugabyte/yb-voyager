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
}

// Run status values. Anything other than statusOK means the run that produced the row
// did not satisfy the harness's own gates, so the verdict must not be published.
const (
	statusOK      = "OK"      // controls passed, no flake marker
	statusInvalid = "INVALID" // a known-good control did not come out WORKS
	statusFlake   = "FLAKE"   // export never streamed, or probes came out INCONCLUSIVE
	statusPoison  = "POISON"  // deliberate poison-isolation run; control gate N/A
)

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
}

// Key identifies a row across runs. A results file holds at most one row per key.
func (r Row) Key() string { return r.ProbeID + "|" + r.Mode }

func (r Row) record() []string {
	return []string{
		r.RunTimestamp, r.VoyagerCommit, r.PGVersion, r.YBVersion,
		r.ProbeID, r.TypeName, r.Category, r.Mode, r.Verdict,
		r.Evidence, r.SourceValue, r.TargetValue, r.RunStatus, r.SQLState,
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
		RunTimestamp:  get("run_timestamp"),
		VoyagerCommit: get("voyager_commit"),
		PGVersion:     get("pg_version"),
		YBVersion:     get("yb_version"),
		ProbeID:       get("probe_id"),
		TypeName:      get("type_name"),
		Category:      get("category"),
		Mode:          get("mode"),
		Verdict:       get("verdict"),
		Evidence:      get("evidence"),
		SourceValue:   get("source_value"),
		TargetValue:   get("target_value"),
		RunStatus:     get("run_status"),
		SQLState:      get("sqlstate"),
	}
}

// RunMeta is the per-run provenance stamped onto every row.
type RunMeta struct {
	Timestamp     string
	VoyagerCommit string
	PGVersion     string
	YBVersion     string
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
	goTestRunRe = regexp.MustCompile(`=== RUN\s+TestDatatypeSweep(\w+)/([\w.\-]+)`)
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
	// "<id>|<mode>" -> the verdict the harness declared publishable despite its run's
	// gate. Keyed by verdict as well as by row so a marker can never promote a verdict
	// the classifier did not actually produce.
	publishable := map[string]string{}
	curBatch := ""
	curMode := ""

	for sc.Scan() {
		line := sc.Text()

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
			RunTimestamp:  meta.Timestamp,
			VoyagerCommit: meta.VoyagerCommit,
			PGVersion:     meta.PGVersion,
			YBVersion:     meta.YBVersion,
			ProbeID:       fields[0],
			TypeName:      fields[1],
			Mode:          fields[2],
			Verdict:       fields[3],
			Evidence:      detail,
			SourceValue:   src,
			TargetValue:   dst,
			RunStatus:     statusOK,
			SQLState:      sqlStateOf(detail),
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
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading test log: %w", err)
	}

	for i := range rows {
		if st, ok := runStatus[rowBatch[i]]; ok {
			rows[i].RunStatus = st
		}
		// The carve-out, applied AFTER the run status so it can override it. An
		// attributed export death is the finding rather than a broken measurement, and
		// the controls going inconclusive is a consequence of it - so that one row is
		// publishable even though its run is not. It promotes only the exact
		// (probe, mode, verdict) the harness named; every other row from the run keeps
		// the run's status and stays out of the report and the differ.
		if v, ok := publishable[rows[i].Key()]; ok && v == rows[i].Verdict {
			rows[i].RunStatus = statusOK
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

// dedupe keeps the LAST row for a key. A re-run of a batch inside the same log (the
// harness re-runs a flaked batch) should be represented by its final attempt.
func dedupe(rows []Row) []Row {
	idx := map[string]int{}
	var out []Row
	for _, r := range rows {
		if i, seen := idx[r.Key()]; seen {
			out[i] = r
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
