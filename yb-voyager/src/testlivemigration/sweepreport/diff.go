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
Release-to-release diff of two results CSVs.

The question this answers mechanically is "what changed in datatype support between
these two builds": which (probe, mode) pairs got worse, which got better, which appeared
and which disappeared.
*/

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// verdictRank orders the product verdicts from worst to best. A DROP in rank between two
// runs is a regression; a RISE is an improvement.
//
// SKIPPED and INCONCLUSIVE are deliberately absent: they are harness/environment facts,
// not product verdicts, so a move into or out of them is reported as a coverage change
// rather than as a regression or an improvement.
// EXPORTER_CRASHES ranks BELOW STUCK because a wedged importer still delivered everything
// ahead of the poison value, while a dead exporter produced nothing at all and leaves
// `initiate cutover` waiting forever. The two are separate ranks, never one label: import
// failure means a value could not be APPLIED, export failure means nothing was ever
// PRODUCED, and a fix that moves a type from one to the other has changed something real.
var verdictRank = map[string]int{
	"SILENT_LOSS":      0, // value silently lost
	"SILENT_WRONG":     1, // value silently altered
	"QUIET_DROP":       2, // column dropped with no user-visible warning
	"EXPORTER_CRASHES": 3, // the exporter dies: nothing is produced at all
	"STUCK":            4, // the importer wedges on the value
	"BLOCKS":           5, // migration refuses to proceed
	"EXCLUDED_TOLD":    6, // column excluded AND the user was told
	"WORKS":            7, // full fidelity round trip
}

// rank returns the verdict's ordering position and whether it is a product verdict.
func rank(v string) (int, bool) {
	r, ok := verdictRank[strings.TrimSpace(strings.ToUpper(v))]
	return r, ok
}

type Change struct {
	ProbeID  string
	TypeName string
	Category string
	Mode     string
	Old      string
	New      string
	Detail   string
}

func (c Change) String() string {
	s := fmt.Sprintf("%-16s %-28s %-12s %s -> %s", c.ProbeID, c.TypeName, c.Mode, c.Old, c.New)
	if c.Detail != "" {
		s += "  (" + c.Detail + ")"
	}
	return s
}

type DiffResult struct {
	Regressions  []Change
	Improvements []Change
	CoverageGain []Change // newly measured, or moved out of SKIPPED/INCONCLUSIVE
	CoverageLoss []Change // no longer measured, or moved into SKIPPED/INCONCLUSIVE

	// ErrorCodeChanges are pairs whose VERDICT is identical but whose SQLSTATE moved.
	// Not a regression - the outcome is the same - but it means the type is now failing
	// for a different reason, which is how a fix that only moved the error shows up.
	// Reportable rather than gating.
	ErrorCodeChanges []Change

	Unchanged     int
	SkippedOldBad int // rows in the old file whose run did not pass its gates
	SkippedNewBad int // ditto for the new file
}

// HasRegressions is what a caller should gate a release on.
func (d DiffResult) HasRegressions() bool { return len(d.Regressions) > 0 || len(d.CoverageLoss) > 0 }

// Diff compares two results sets by (probe id, mode).
//
// Rows whose run_status is not OK are excluded from both sides before comparing: a run
// whose known-good controls failed is an invalid run, and comparing against it would
// manufacture phantom regressions. They are counted so the caller can say so.
func Diff(old, new []Row) DiffResult {
	var res DiffResult

	index := func(rows []Row, badCount *int) map[string]Row {
		m := map[string]Row{}
		for _, r := range rows {
			if r.RunStatus != "" && r.RunStatus != statusOK {
				*badCount++
				continue
			}
			m[r.Key()] = r
		}
		return m
	}
	oldIdx := index(old, &res.SkippedOldBad)
	newIdx := index(new, &res.SkippedNewBad)

	keys := map[string]bool{}
	for k := range oldIdx {
		keys[k] = true
	}
	for k := range newIdx {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	for _, k := range sorted {
		o, hadOld := oldIdx[k]
		n, hadNew := newIdx[k]

		switch {
		case !hadOld:
			res.CoverageGain = append(res.CoverageGain, change(n, "-", n.Verdict, "new probe/mode"))
			continue
		case !hadNew:
			res.CoverageLoss = append(res.CoverageLoss, change(o, o.Verdict, "-", "probe/mode no longer measured"))
			continue
		}

		if o.Verdict == n.Verdict {
			if o.SQLState != n.SQLState && (o.SQLState != "" || n.SQLState != "") {
				c := change(n, o.Verdict, n.Verdict, n.Evidence)
				c.Old, c.New = orDash(o.SQLState), orDash(n.SQLState)
				c.Detail = "verdict unchanged (" + n.Verdict + "); " + c.Detail
				res.ErrorCodeChanges = append(res.ErrorCodeChanges, c)
				continue
			}
			res.Unchanged++
			continue
		}
		oldRank, oldIsProduct := rank(o.Verdict)
		newRank, newIsProduct := rank(n.Verdict)

		switch {
		case oldIsProduct && newIsProduct && newRank < oldRank:
			res.Regressions = append(res.Regressions, change(n, o.Verdict, n.Verdict, n.Evidence))
		case oldIsProduct && newIsProduct && newRank > oldRank:
			res.Improvements = append(res.Improvements, change(n, o.Verdict, n.Verdict, n.Evidence))
		case !oldIsProduct && newIsProduct:
			res.CoverageGain = append(res.CoverageGain, change(n, o.Verdict, n.Verdict, "now measurable"))
		case oldIsProduct && !newIsProduct:
			res.CoverageLoss = append(res.CoverageLoss, change(n, o.Verdict, n.Verdict, n.Evidence))
		default:
			// both non-product (e.g. SKIPPED -> INCONCLUSIVE): not a product change.
			res.Unchanged++
		}
	}
	return res
}

func change(r Row, oldV, newV, detail string) Change {
	return Change{
		ProbeID:  r.ProbeID,
		TypeName: r.TypeName,
		Category: r.Category,
		Mode:     r.Mode,
		Old:      oldV,
		New:      newV,
		Detail:   trimTo(detail, 140),
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func trimTo(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// PrintDiff writes a human-readable summary. Regressions come first and are never
// silently folded into a count.
func PrintDiff(w io.Writer, d DiffResult, oldPath, newPath string) {
	fmt.Fprintf(w, "datatype sweep diff\n  old: %s\n  new: %s\n\n", oldPath, newPath)

	section := func(title string, list []Change) {
		fmt.Fprintf(w, "%s (%d)\n", title, len(list))
		if len(list) == 0 {
			fmt.Fprintf(w, "  none\n\n")
			return
		}
		for _, c := range list {
			fmt.Fprintf(w, "  %s\n", c)
		}
		fmt.Fprintln(w)
	}

	section("REGRESSIONS", d.Regressions)
	section("IMPROVEMENTS", d.Improvements)
	section("COVERAGE LOST", d.CoverageLoss)
	section("COVERAGE GAINED", d.CoverageGain)
	section("SAME VERDICT, DIFFERENT SQLSTATE", d.ErrorCodeChanges)

	fmt.Fprintf(w, "unchanged: %d\n", d.Unchanged)
	if d.SkippedOldBad > 0 || d.SkippedNewBad > 0 {
		fmt.Fprintf(w, "excluded rows from runs that failed their control gate: old=%d new=%d\n",
			d.SkippedOldBad, d.SkippedNewBad)
	}
}
