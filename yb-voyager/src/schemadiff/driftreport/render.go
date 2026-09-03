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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
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
	// Rendered as two paragraphs, per the mockup. Pre-escaped HTML; see codeSpans.
	Impact template.HTML
	Action template.HTML
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

		Timeline: buildTimeline(r.Captures, groupsByWindow, r.Source.DatabaseType),

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

// comparingSummary renders the banner's collapsed summary line, e.g.
// "public · all 12 tables · all 2 object types". Counts rather than a bare "all",
// so the reader learns how much was actually compared.
func comparingSummary(c Comparing) string {
	return strings.Join([]string{
		joinOrAll(c.Schemas),
		scopeCountLabel(len(c.Tables), c.TablesFiltered, "table"),
		scopeCountLabel(len(c.ObjectTypes), c.ObjectTypesFiltered, "object type"),
	}, " · ")
}

// scopeCountLabel renders one dimension: "all 12 tables", or "12 tables (filtered)".
// An empty set reads "no tables" rather than claiming "all".
func scopeCountLabel(n int, filtered bool, noun string) string {
	plural := noun + "s"
	if n == 1 {
		plural = noun
	}
	switch {
	case n == 0:
		return "no " + noun + "s"
	case filtered:
		return fmt.Sprintf("%d %s (filtered)", n, plural)
	default:
		return fmt.Sprintf("all %d %s", n, plural)
	}
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

// maxScopeChips caps the names the dropdown enumerates; a 1000-table schema would
// otherwise bury the rest of the report.
const maxScopeChips = 50

// comparingScope builds the "Comparing" dropdown rows, one per dimension. Both
// enumerate the names actually compared.
func comparingScope(c Comparing) []scopeRow {
	return []scopeRow{
		{Label: scopeRowLabel("Tables", len(c.Tables), c.TablesFiltered), Chips: cappedChips(c.Tables)},
		{Label: scopeRowLabel("Object types", len(c.ObjectTypes), c.ObjectTypesFiltered), Chips: cappedChips(c.ObjectTypes)},
	}
}

// scopeRowLabel marks whether the count is everything there was, or a filter's result.
func scopeRowLabel(title string, n int, filtered bool) string {
	if filtered {
		return fmt.Sprintf("%s (%d, filtered)", title, n)
	}
	return fmt.Sprintf("%s (%d)", title, n)
}

// cappedChips truncates to maxScopeChips, appending a "+N more" chip.
func cappedChips(items []string) []string {
	if len(items) <= maxScopeChips {
		return items
	}
	out := make([]string, 0, maxScopeChips+1)
	out = append(out, items[:maxScopeChips]...)
	return append(out, fmt.Sprintf("+%d more", len(items)-maxScopeChips))
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
func buildTimeline(captures []Capture, groupsByWindow map[Window]intervalGroup, dbType string) []timelineEntry {
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
			iv := newIntervalView(g, next, dbType)
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
func newIntervalView(g intervalGroup, next Capture, dbType string) intervalView {
	live := next.Series == SeriesSourceLive
	count := changeCountLabel(len(g.Diffs))
	if live {
		count = fmt.Sprintf("%s · live source @ %s", count, formatTime(next.CapturedAt))
	}

	findings := make([]findingView, len(g.Diffs))
	for i, d := range g.Diffs {
		findings[i] = newFindingView(d, dbType)
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
func newFindingView(d DiffEntry, dbType string) findingView {
	objQ, objS := objectPath(d, dbType)

	fv := findingView{
		KindClass:    kindClass(d.Operation),
		KindLabel:    kindLabel(d.Type),
		ObjQ:         objQ,
		ObjS:         objS,
		ActionStatus: actionStatus(d.Status),
		Impact:       codeSpans(d.Impact),
		Action:       codeSpans(d.Action),
	}

	switch d.Operation {
	case string(schemadiff.OpAdded):
		if def := stringifyValue(d.Attribute, d.NewValue, dbType); def != "" {
			fv.HasDef = true
			fv.ValDef = def
		}
	case string(schemadiff.OpDropped):
		// No value chip for drops, matching the mockup.
	default: // OpChanged
		fv.HasChange = true
		fv.ValOld = stringifyValue(d.Attribute, d.OldValue, dbType)
		fv.ValNew = stringifyValue(d.Attribute, d.NewValue, dbType)
	}

	return fv
}

// kindClass classifies a DiffEntry.Operation into the CSS kind bucket used
// for colour: k-add for ADDED, k-rem for DROPPED, k-chg for everything else
// (CHANGED findings).
func kindClass(operation string) string {
	switch operation {
	case string(schemadiff.OpAdded):
		return "k-add"
	case string(schemadiff.OpDropped):
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

// objectPath splits an object identity into the muted qualifier (q) and the
// highlighted subject (s), per the mockup. A COLUMN finding qualifies down to the
// column, a table-level one to the table.
//
// Every part is minimally quoted, so a special identifier renders as valid SQL
// (sales."MixedCase", not the ambiguous sales.MixedCase). q+s equals ForDisplay.
func objectPath(d DiffEntry, dbType string) (q, s string) {
	if d.ObjectType == string(schemadiff.ObjectTypeColumn) {
		return d.Object.ForDisplay(dbType) + ".", minQuoted(d.SubObject, dbType)
	}
	return minQuoted(d.Object.Schema, dbType) + ".", minQuoted(d.Object.Name, dbType)
}

// minQuoted renders a single identifier part with quotes only where they are
// needed for dbType (e.g. mixed case, embedded spaces, reserved words).
func minQuoted(name, dbType string) string {
	if name == "" {
		return ""
	}
	return sqlname.NewIdentifier(dbType, name).MinQuoted
}

// codeSpans turns `backticked` runs in guidance text into <code> elements.
//
// Everything is HTML-escaped FIRST and only the delimiters are then replaced, so
// guidance text can never inject markup. Unpaired backticks are left escaped.
func codeSpans(s string) template.HTML {
	escaped := template.HTMLEscapeString(s)
	parts := strings.Split(escaped, "`")
	if len(parts)%2 == 0 {
		// Odd number of backticks: no sane pairing, so render it verbatim.
		return template.HTML(escaped)
	}
	var b strings.Builder
	for i, p := range parts {
		if i%2 == 1 {
			b.WriteString("<code>")
			b.WriteString(p)
			b.WriteString("</code>")
			continue
		}
		b.WriteString(p)
	}
	return template.HTML(b.String())
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
// dynamic type tracks attribute, per Difference's field docs) as display
// text:
//   - nil                                    -> ""
//   - string                                 -> as-is
//   - bool, attribute == AttrNullability      -> "NOT NULL" / "NULL"
//   - bool, otherwise                        -> "true" / "false"
//   - schemasnapshot.ObjectRef                -> "schema.name"
//   - []schemasnapshot.ObjectRef              -> "schema.name, schema.name, ..."
//   - schemasnapshot.Column                    -> the column's definition
//     (COLUMN_ADDED/DROPPED's whole added/dropped column), e.g.
//     "integer NOT NULL DEFAULT 0"
//   - []schemasnapshot.Column                  -> a concise column-count
//     summary (TABLE_ADDED/DROPPED's whole added/dropped table's columns),
//     e.g. "3 columns"
//   - anything else                          -> fmt.Sprintf("%v", value)
func stringifyValue(attribute string, value any, dbType string) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if attribute == string(schemadiff.AttrNullability) {
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
		return v.ForDisplay(dbType)
	case []schemasnapshot.ObjectRef:
		parts := make([]string, len(v))
		for i, ref := range v {
			parts[i] = ref.ForDisplay(dbType)
		}
		return strings.Join(parts, ", ")
	case schemasnapshot.Column:
		return stringifyColumnDef(v)
	case schemasnapshot.Table:
		// TABLE_ADDED/DROPPED carry the whole Table; render its full column list
		// ("id integer NOT NULL, email text, ...") rather than just a count.
		if len(v.Columns) == 0 {
			return "no columns"
		}
		parts := make([]string, len(v.Columns))
		for i, c := range v.Columns {
			parts[i] = c.Name + " " + stringifyColumnDef(c)
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// stringifyColumnDef renders a whole Column (COLUMN_ADDED's NewValue or
// COLUMN_DROPPED's OldValue) as a concise, human-readable definition, e.g.
// "integer NOT NULL DEFAULT 0".
func stringifyColumnDef(c schemasnapshot.Column) string {
	var b strings.Builder
	b.WriteString(c.DataType)
	if c.NotNull {
		b.WriteString(" NOT NULL")
	}
	if c.Default != "" {
		fmt.Fprintf(&b, " DEFAULT %s", c.Default)
	}
	return b.String()
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
