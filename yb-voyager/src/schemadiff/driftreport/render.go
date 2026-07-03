// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package driftreport

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

//go:embed templates/drift_report.html
var driftReportTemplateSource string

// RenderJSON marshals r as indented JSON, matching the contractual shape
// documented on Report.
func RenderJSON(r Report) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// RenderHTML renders r as a self-contained (inline CSS, no external assets,
// no JS required) single-page HTML report.
func RenderHTML(r Report) ([]byte, error) {
	tmpl, err := template.New("drift_report").Parse(driftReportTemplateSource)
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, newReportView(r)); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

// formatTime renders t in a readable, deterministic form for the HTML
// template. The zero time.Time renders as "-" (e.g. an empty report window).
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05 UTC")
}

// ─── View model: grouping/formatting logic kept in Go, not in the template ──
//
// Everything below turns a Report into display-ready strings so that
// templates/drift_report.html stays a thin iteration-and-field-access layer,
// mirroring the hand-authored mockup at DRIFT_REPORT_MOCKUP.html.

// reportView is the unexported view model handed to the HTML template.
type reportView struct {
	ChangeCount int

	SourceLine string
	WindowLine string

	ComparingSummary string
	ComparingScope   []scopeRow

	Timeline []timelineEntry

	Snapshots []snapshotRow
}

// scopeRow is one row of the banner's "Comparing" dropdown body (e.g. the
// list of tables, or the list of object types, actually compared).
type scopeRow struct {
	Label string   // e.g. "Tables (12)" or "Object types (6)"
	Chips []string // the chip values; nil/empty renders as a single "all" chip
}

// timelineEntry is one item on the vertical timeline: exactly one of Event or
// Interval is non-nil. Modeled as a struct-of-pointers (rather than an
// interface) so the template can branch on it with a plain {{if}}.
type timelineEntry struct {
	Event    *eventView
	Interval *intervalView
}

// eventView is a single point-event marker on the timeline spine (e.g.
// "export data: started").
type eventView struct {
	Label string
	Time  string
	Err   bool // true for a failure marker (e.g. "export data: failed")
}

// intervalView is one grouped span of findings between two captures.
type intervalView struct {
	Window   string
	Phase    string
	Count    string // e.g. "3 changes" or "1 change · live source @ ..."
	Live     bool
	Findings []findingView
}

// findingView is a single collapsible finding (<details class="finding">).
type findingView struct {
	KindClass string // k-add | k-rem | k-chg
	KindLabel string // e.g. "table added", "column type changed"

	ObjQ string // qualifier prefix, e.g. "public.orders."
	ObjS string // the highlighted subject, e.g. "amount"

	HasDef bool // *_ADDED with a non-empty NewValue
	ValDef string

	HasChange bool // *_CHANGED
	ValOld    string
	ValNew    string

	ActionStatus string // emoji + severity label, e.g. "⚠️ Potential impact"
	Guidance     string
}

// snapshotRow is one row of the footer's "Snapshots used" table.
type snapshotRow struct {
	Seq        string
	Series     string
	CapturedAt string
	Note       string
}

func newReportView(r Report) reportView {
	groups := groupByInterval(r.Diffs)
	groupsByWindow := make(map[Window]intervalGroup, len(groups))
	for _, g := range groups {
		groupsByWindow[g.Window] = g
	}

	return reportView{
		ChangeCount: r.Summary.ChangeCount,

		SourceLine: sourceLine(r.Source),
		WindowLine: formatTime(r.Window.From) + " → " + formatTime(r.Window.To),

		ComparingSummary: comparingSummary(r.Comparing),
		ComparingScope:   comparingScope(r.Comparing),

		Timeline: buildTimeline(r.Captures, groupsByWindow),

		Snapshots: snapshotRows(r.Captures),
	}
}

// sourceLine renders "<type>@<host>:<port>/<database> · <version>", omitting
// any trailing parts gracefully when the corresponding field is empty.
func sourceLine(s Source) string {
	var b strings.Builder
	b.WriteString(s.DatabaseType)
	if s.Host != "" {
		if b.Len() > 0 {
			b.WriteString("@")
		}
		b.WriteString(s.Host)
		if s.Port != 0 {
			fmt.Fprintf(&b, ":%d", s.Port)
		}
	}
	if s.Database != "" {
		if b.Len() > 0 {
			b.WriteString("/")
		}
		b.WriteString(s.Database)
	}
	if s.DatabaseVersion != "" {
		if b.Len() > 0 {
			b.WriteString(" · ")
		}
		b.WriteString(s.DatabaseVersion)
	}
	return b.String()
}

// comparingSummary renders the banner's collapsed "Comparing" summary line,
// e.g. "public · all tables · all object types".
func comparingSummary(c Comparing) string {
	return strings.Join([]string{
		joinOrAll(c.Schemas),
		listOrAllLabel(c.Tables, "all tables"),
		listOrAllLabel(c.ObjectTypes, "all object types"),
	}, " · ")
}

// listOrAllLabel joins items with ", ", or returns allLabel when items is
// empty.
func listOrAllLabel(items []string, allLabel string) string {
	if len(items) == 0 {
		return allLabel
	}
	return strings.Join(items, ", ")
}

// joinOrAll joins items with ", ", or returns "all" when items is empty.
func joinOrAll(items []string) string {
	return listOrAllLabel(items, "all")
}

// comparingScope builds the expandable "Comparing" dropdown body rows: one
// for tables, one for object types. An empty list renders a single "all"
// chip rather than enumerating the (unfiltered) full source scope.
func comparingScope(c Comparing) []scopeRow {
	return []scopeRow{
		{Label: fmt.Sprintf("Tables (%d)", len(c.Tables)), Chips: c.Tables},
		{Label: fmt.Sprintf("Object types (%d)", len(c.ObjectTypes)), Chips: c.ObjectTypes},
	}
}

// groupByInterval groups diffs (already in a stable, seq-ascending order) by
// their Window, preserving first-seen order of intervals.
func groupByInterval(diffs []DiffEntry) []intervalGroup {
	var groups []intervalGroup
	index := make(map[Window]int)
	for _, d := range diffs {
		i, ok := index[d.Window]
		if !ok {
			i = len(groups)
			index[d.Window] = i
			groups = append(groups, intervalGroup{Window: d.Window, Phase: d.Phase})
		}
		groups[i].Diffs = append(groups[i].Diffs, d)
	}
	return groups
}

// intervalGroup is all DiffEntries that share the same Window (and therefore
// the same Phase), rendered together as one timeline section.
type intervalGroup struct {
	Window Window
	Phase  string
	Diffs  []DiffEntry
}

// buildTimeline interleaves point-event markers and interval blocks in
// chronological order. It walks Captures pairwise: for capture i it emits
// (a) the point-event marker for capture i, if any, then (b) the interval
// group for the (i, i+1) pair, if that pair produced any diffs. This
// reproduces the mockup's ordering exactly, since an interval's Window
// always matches a consecutive capture pair and groupByInterval only
// includes windows that actually have diffs (so a diff-less pair
// contributes no interval, matching the mockup's skipped spans).
func buildTimeline(captures []Capture, groupsByWindow map[Window]intervalGroup) []timelineEntry {
	var timeline []timelineEntry
	for i, c := range captures {
		if ev, ok := deriveEvent(c); ok {
			timeline = append(timeline, timelineEntry{Event: &ev})
		}
		if i+1 >= len(captures) {
			continue
		}
		next := captures[i+1]
		w := Window{From: c.CapturedAt, To: next.CapturedAt}
		if g, ok := groupsByWindow[w]; ok {
			iv := newIntervalView(g, next)
			timeline = append(timeline, timelineEntry{Interval: &iv})
		}
	}
	return timeline
}

// deriveEvent derives the point-event marker (if any) for a single capture,
// from its Series and Reason. Periodic captures and the live read never
// produce a marker; ok is false in that case (and for any Series/Reason
// combination not in the known vocabulary).
func deriveEvent(c Capture) (eventView, bool) {
	t := formatTime(c.CapturedAt)
	switch c.Series {
	case schemasnapshot.LabelExportSchema:
		return eventView{Label: "export schema: completed", Time: t}, true
	case schemasnapshot.LabelExportDataFromSourceStart:
		switch c.Reason {
		case schemasnapshot.ReasonInitial:
			return eventView{Label: "export data: started", Time: t}, true
		case schemasnapshot.ReasonResume:
			return eventView{Label: "export data: restarted", Time: t}, true
		case schemasnapshot.ReasonCleanRestart:
			return eventView{Label: "export data: restarted (start-clean)", Time: t}, true
		}
	case schemasnapshot.LabelExportDataFromSourceExit:
		switch c.Reason {
		case schemasnapshot.ReasonInterrupt:
			return eventView{Label: "export data: stopped", Time: t}, true
		case schemasnapshot.ReasonComplete:
			return eventView{Label: "export data: completed", Time: t}, true
		case schemasnapshot.ReasonError:
			return eventView{Label: "⚠ export data: failed", Time: t, Err: true}, true
		case schemasnapshot.ReasonCutover:
			return eventView{Label: "cutover: to target", Time: t}, true
		}
	}
	return eventView{}, false
}

// newIntervalView builds the display view for one interval group. next is
// the capture that closes the interval's window (the "to" side of the
// pair); the interval is "live" when next is the live read of the source.
func newIntervalView(g intervalGroup, next Capture) intervalView {
	live := next.Series == SeriesSourceLive
	count := changeCountLabel(len(g.Diffs))
	if live {
		count = fmt.Sprintf("%s · live source @ %s", count, formatTime(next.CapturedAt))
	}

	findings := make([]findingView, len(g.Diffs))
	for i, d := range g.Diffs {
		findings[i] = newFindingView(d)
	}

	return intervalView{
		Window:   formatTime(g.Window.From) + " → " + formatTime(g.Window.To),
		Phase:    g.Phase,
		Count:    count,
		Live:     live,
		Findings: findings,
	}
}

// changeCountLabel renders "1 change" or "N changes".
func changeCountLabel(n int) string {
	if n == 1 {
		return "1 change"
	}
	return fmt.Sprintf("%d changes", n)
}

// newFindingView builds the display view for a single DiffEntry.
func newFindingView(d DiffEntry) findingView {
	objQ, objS := objectPath(d)

	fv := findingView{
		KindClass:    kindClass(d.Type),
		KindLabel:    kindLabel(d.Type),
		ObjQ:         objQ,
		ObjS:         objS,
		ActionStatus: actionStatus(d.Status),
		Guidance:     d.Guidance,
	}

	switch {
	case strings.HasSuffix(d.Type, "_ADDED"):
		if def := stringifyValue(d.Property, d.NewValue); def != "" {
			fv.HasDef = true
			fv.ValDef = def
		}
	case strings.HasSuffix(d.Type, "_DROPPED"):
		// No value chip for drops, matching the mockup.
	default: // *_CHANGED
		fv.HasChange = true
		fv.ValOld = stringifyValue(d.Property, d.OldValue)
		fv.ValNew = stringifyValue(d.Property, d.NewValue)
	}

	return fv
}

// kindClass classifies a DiffEntry.Type string into the CSS kind bucket used
// for colour: k-add for *_ADDED, k-rem for *_DROPPED, k-chg for everything
// else (the *_CHANGED findings).
func kindClass(diffType string) string {
	switch {
	case strings.HasSuffix(diffType, "_ADDED"):
		return "k-add"
	case strings.HasSuffix(diffType, "_DROPPED"):
		return "k-rem"
	default:
		return "k-chg"
	}
}

// kindLabel renders a DiffEntry.Type string (e.g. "COLUMN_TYPE_CHANGED") as
// its lowercase, space-separated display label ("column type changed").
func kindLabel(diffType string) string {
	return strings.ToLower(strings.ReplaceAll(diffType, "_", " "))
}

// objectPath splits a DiffEntry's object identity into the muted qualifier
// prefix (q) and the highlighted subject (s), matching the mockup's
// <span class="q">...</span><span class="s">...</span> split. A column-level
// finding (SubObject set) qualifies down to the column; a table-level
// finding qualifies down to the table.
func objectPath(d DiffEntry) (q, s string) {
	if d.SubObject != "" {
		return d.Object.Schema + "." + d.Object.Name + ".", d.SubObject
	}
	return d.Object.Schema + ".", d.Object.Name
}

// actionStatusText maps each Status to its emoji + severity label, as shown
// in a finding's "Impact & action" block.
var actionStatusText = map[Status]string{
	StatusAdvisory:            "ℹ️ Advisory",
	StatusPotentialImpact:     "⚠️ Potential impact",
	StatusBreaksRecoverable:   "⛔ Breaks the migration — recoverable",
	StatusBreaksUnrecoverable: "🚨 Breaks the migration — unrecoverable",
}

// actionStatus renders status's emoji + severity label, falling back to the
// raw status string for any value outside the known vocabulary.
func actionStatus(status string) string {
	if s, ok := actionStatusText[Status(status)]; ok {
		return s
	}
	return status
}

// stringifyValue renders a DiffEntry.OldValue/NewValue (an `any` whose
// dynamic type tracks property, per Difference's field docs) as display
// text:
//   - nil                                -> ""
//   - string                             -> as-is
//   - bool, property == "not_null"       -> "NOT NULL" / "NULL"
//   - bool, otherwise                    -> "true" / "false"
//   - schemasnapshot.ObjectRef           -> "schema.name"
//   - []schemasnapshot.ObjectRef         -> "schema.name, schema.name, ..."
//   - anything else                      -> fmt.Sprintf("%v", value)
func stringifyValue(property string, value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if property == "not_null" {
			if v {
				return "NOT NULL"
			}
			return "NULL"
		}
		if v {
			return "true"
		}
		return "false"
	case schemasnapshot.ObjectRef:
		return v.Schema + "." + v.Name
	case []schemasnapshot.ObjectRef:
		parts := make([]string, len(v))
		for i, ref := range v {
			parts[i] = ref.Schema + "." + ref.Name
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// snapshotRows builds the footer's "Snapshots used" table rows from
// Captures. The live read (Series == SeriesSourceLive) shows "—" for its
// sequence number, since it is never persisted/numbered like a stored
// snapshot.
func snapshotRows(captures []Capture) []snapshotRow {
	rows := make([]snapshotRow, len(captures))
	for i, c := range captures {
		seq := fmt.Sprintf("%d", c.Seq)
		note := ""
		if c.Series == SeriesSourceLive {
			seq = "—"
			note = "read fresh at report time · not stored"
		} else if c.Reason != "" {
			note = "reason: " + c.Reason
		}
		rows[i] = snapshotRow{
			Seq:        seq,
			Series:     c.Series,
			CapturedAt: formatTime(c.CapturedAt),
			Note:       note,
		}
	}
	return rows
}
