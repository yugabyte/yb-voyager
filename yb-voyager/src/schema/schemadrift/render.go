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

package schemadrift

import (
	"bytes"
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

// Parsed once at init: the source is embedded at build time, so a parse failure
// is a broken build, not a runtime condition a caller could handle.
var driftReportTemplate = template.Must(template.New("drift_report").Parse(driftReportTemplateSource))

// RenderJSON marshals r as indented JSON, matching the contractual shape
// documented on Report.
func RenderJSON(r Report) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// RenderHTML renders r as a self-contained (inline CSS, no external assets,
// no JS required) single-page HTML report.
func RenderHTML(r Report) ([]byte, error) {
	view, err := newReportView(r)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := driftReportTemplate.Execute(&buf, view); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// formatTime renders the zero time.Time as "-" (e.g. an empty report window).
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format("2006-01-02 15:04:05 UTC")
}

// ─── View model: grouping/formatting logic kept in Go, not in the template ──
//
// Everything below turns a Report into display-ready strings so that
// templates/drift_report.html stays a thin iteration-and-field-access layer.

// reportView is the unexported view model handed to the HTML template.
type reportView struct {
	ChangeCount int

	SourceLine string
	WindowLine string

	ComparingSummary string
	ComparingScope   []scopeRow

	TimelineRows []timelineEntry

	Snapshots []snapshotRow
}

// scopeRow is one row of the banner's "Comparing" dropdown body (e.g. the
// list of tables, or the list of object types, actually compared).
type scopeRow struct {
	Label string // e.g. "Tables (12)" or "Object types (6)"
	Chips []string
}

// timelineEntry is one item on the vertical timeline: exactly one of the two
// fields is non-nil. Modeled as a struct-of-pointers (rather than an interface)
// so the template can branch on it with a plain {{if}}.
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

	SeverityLabel string // emoji + severity label, e.g. "⚠️ Potential impact"
	// Rendered as two paragraphs. Pre-escaped HTML; see codeSpans.
	Impact template.HTML
	Action template.HTML
}

// snapshotRow is one row of the footer's "Snapshots used" table.
type snapshotRow struct {
	Seq        string
	Label      string
	CapturedAt string
	Note       string
}

func newReportView(r Report) (reportView, error) {
	timeline, err := buildTimeline(r.CapturePoints, groupByInterval(r.Drifts), r.Source.DatabaseType)
	if err != nil {
		return reportView{}, err
	}
	return reportView{
		ChangeCount: r.Summary.ChangeCount,

		SourceLine: sourceLine(r.Source),
		WindowLine: formatTime(r.Window.From) + " → " + formatTime(r.Window.To),

		ComparingSummary: comparingSummary(r.Comparing),
		ComparingScope:   comparingScope(r.Comparing),

		TimelineRows: timeline,

		Snapshots: snapshotRows(r.CapturePoints),
	}, nil
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
// "public · 12 tables · 2 object types". Counts rather than a bare "all", so the
// reader learns how much was actually compared.
func comparingSummary(c Comparing) string {
	return strings.Join([]string{
		schemaLabel(c.Schemas),
		scopeCountLabel(len(c.Tables), "table"),
		scopeCountLabel(len(c.ObjectTypes), "object type"),
	}, " · ")
}

func scopeCountLabel(n int, noun string) string {
	plural := noun + "s"
	if n == 1 {
		plural = noun
	}
	if n == 0 {
		return "no " + plural
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// schemaLabel names the schemas compared, capped like every other list in the
// report. An empty set reads "no schemas" rather than claiming "all": the schema
// list is the migration's whole universe, so "all" would be a claim, not a default.
func schemaLabel(schemas []string) string {
	if len(schemas) == 0 {
		return "no schemas"
	}
	noun := "schemas "
	if len(schemas) == 1 {
		noun = "schema "
	}
	return noun + strings.Join(cappedChips(schemas), ", ")
}

// maxScopeChips caps the names the dropdown enumerates; a 1000-table schema would
// otherwise bury the rest of the report.
const maxScopeChips = 50

// comparingScope builds the "Comparing" dropdown rows, one per dimension. Both
// enumerate the names actually compared.
func comparingScope(c Comparing) []scopeRow {
	return []scopeRow{
		{Label: fmt.Sprintf("Tables (%d)", len(c.Tables)), Chips: cappedChips(c.Tables)},
		{Label: fmt.Sprintf("Object types (%d)", len(c.ObjectTypes)), Chips: cappedChips(c.ObjectTypes)},
	}
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

// groupByInterval groups drifts (already in a stable, seq-ascending order) by
// their Window, preserving first-seen order of intervals.
func groupByInterval(drifts []DriftEntry) []intervalGroup {
	var groups []intervalGroup
	index := make(map[Window]int)
	for _, d := range drifts {
		i, ok := index[d.Window]
		if !ok {
			i = len(groups)
			index[d.Window] = i
			groups = append(groups, intervalGroup{Window: d.Window, Phase: d.Phase})
		}
		groups[i].Drifts = append(groups[i].Drifts, d)
	}
	return groups
}

// intervalGroup is all DriftEntries that share the same Window (and therefore
// the same Phase), rendered together as one timeline section.
type intervalGroup struct {
	Window Window
	Phase  string
	Drifts []DriftEntry
}

// buildTimeline interleaves point-event markers and interval blocks chronologically.
//
// Intervals are matched on the capture that OPENS them, never on the (i, i+1) pair:
// a failed capture is bridged (see BuildReport), so an interval's window can span one
// and would match no consecutive pair at all.
func buildTimeline(capturePoints []CapturePoint, groups []intervalGroup, dbType string) ([]timelineEntry, error) {
	capturePointAt := make(map[time.Time]CapturePoint, len(capturePoints))
	for _, c := range capturePoints {
		capturePointAt[c.CapturedAt] = c
	}
	groupFrom := make(map[time.Time]intervalGroup, len(groups))
	for _, g := range groups {
		if _, ok := capturePointAt[g.Window.From]; !ok {
			return nil, fmt.Errorf("interval %s: no capture point opens it", intervalLabel(g.Window))
		}
		if _, ok := capturePointAt[g.Window.To]; !ok {
			return nil, fmt.Errorf("interval %s: no capture point closes it", intervalLabel(g.Window))
		}
		if other, ok := groupFrom[g.Window.From]; ok {
			return nil, fmt.Errorf("intervals %s and %s open at the same capture point",
				intervalLabel(other.Window), intervalLabel(g.Window))
		}
		groupFrom[g.Window.From] = g
	}
	var timeline []timelineEntry
	for _, c := range capturePoints {
		if ev, ok := deriveEvent(c); ok {
			timeline = append(timeline, timelineEntry{Event: &ev})
		}
		if g, ok := groupFrom[c.CapturedAt]; ok {
			iv, err := newIntervalView(g, capturePointAt[g.Window.To], dbType)
			if err != nil {
				return nil, fmt.Errorf("interval %s: %w", intervalLabel(g.Window), err)
			}
			timeline = append(timeline, timelineEntry{Interval: &iv})
		}
	}
	return timeline, nil
}

func intervalLabel(w Window) string {
	return formatTime(w.From) + " → " + formatTime(w.To)
}

// deriveEvent derives the point-event marker (if any) for a single capture,
// from its Label and Reason. A Label/Reason outside the known vocabulary also
// gets no marker rather than an error: an export dir written by a newer voyager
// can carry one, and the footer still lists it.
func deriveEvent(c CapturePoint) (eventView, bool) {
	t := formatTime(c.CapturedAt)
	// A bridged point always gets a marker, whatever its label: without one the
	// reader sees a stretch of timeline that silently contributed nothing. The
	// footer's Snapshots table carries the reason.
	if c.Excluded != "" {
		return eventView{Label: "⚠ not compared", Time: t, Err: true}, true
	}
	switch c.Label {
	case schemasnapshot.LabelExportDataFromSourcePeriodic, schemasnapshot.LabelSourceLive:
		return eventView{}, false
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
func newIntervalView(g intervalGroup, next CapturePoint, dbType string) (intervalView, error) {
	live := next.Label == schemasnapshot.LabelSourceLive
	count := changeCountLabel(len(g.Drifts))
	if live {
		count = fmt.Sprintf("%s · live source @ %s", count, formatTime(next.CapturedAt))
	}

	findings := make([]findingView, len(g.Drifts))
	for i, d := range g.Drifts {
		fv, err := newFindingView(d, dbType)
		if err != nil {
			return intervalView{}, fmt.Errorf("%s finding on %s: %w", d.Type, d.Object.ForDisplay(dbType), err)
		}
		findings[i] = fv
	}

	return intervalView{
		Window:   intervalLabel(g.Window),
		Phase:    g.Phase,
		Count:    count,
		Live:     live,
		Findings: findings,
	}, nil
}

func changeCountLabel(n int) string {
	if n == 1 {
		return "1 change"
	}
	return fmt.Sprintf("%d changes", n)
}

func newFindingView(d DriftEntry, dbType string) (findingView, error) {
	objQ, objS, err := objectPath(d, dbType)
	if err != nil {
		return findingView{}, err
	}
	severity, err := severityLabel(d.Severity)
	if err != nil {
		return findingView{}, err
	}

	fv := findingView{
		KindLabel:     kindLabel(d.Type),
		ObjQ:          objQ,
		ObjS:          objS,
		SeverityLabel: severity,
		Impact:        codeSpans(d.Impact),
		Action:        codeSpans(d.Action),
	}

	switch d.Operation {
	case schemadiff.OpAdded:
		fv.KindClass = "k-add"
		def, err := stringifyValue(d.Attribute, d.NewValue, dbType)
		if err != nil {
			return findingView{}, fmt.Errorf("new value: %w", err)
		}
		if def != "" {
			fv.HasDef = true
			fv.ValDef = def
		}
	case schemadiff.OpDropped:
		// A drop has no value worth showing.
		fv.KindClass = "k-rem"
	case schemadiff.OpChanged:
		fv.KindClass = "k-chg"
		fv.HasChange = true
		if fv.ValOld, err = stringifyValue(d.Attribute, d.OldValue, dbType); err != nil {
			return findingView{}, fmt.Errorf("old value: %w", err)
		}
		if fv.ValNew, err = stringifyValue(d.Attribute, d.NewValue, dbType); err != nil {
			return findingView{}, fmt.Errorf("new value: %w", err)
		}
	default:
		return findingView{}, fmt.Errorf("unexpected operation %q", d.Operation)
	}

	return fv, nil
}

func kindLabel(diffType schemadiff.DiffType) string {
	return strings.ToLower(strings.ReplaceAll(string(diffType), "_", " "))
}

// objectPath splits an object identity into the muted qualifier (q) and the
// highlighted subject (s). A COLUMN finding qualifies down to the
// column, a table-level one to the table.
//
// Every part is minimally quoted, so a special identifier renders as valid SQL
// (sales."MixedCase", not the ambiguous sales.MixedCase). q+s equals ForDisplay.
func objectPath(d DriftEntry, dbType string) (q, s string, err error) {
	if d.Object.Schema == "" || d.Object.Name == "" {
		return "", "", fmt.Errorf("object identity %+v has an empty schema or name", d.Object)
	}
	if d.ObjectType == schemadiff.ObjectTypeColumn {
		if d.SubObject == "" {
			return "", "", fmt.Errorf("column finding on %s has no column name", d.Object.ForDisplay(dbType))
		}
		return d.Object.ForDisplay(dbType) + ".", minQuoted(d.SubObject, dbType), nil
	}
	return minQuoted(d.Object.Schema, dbType) + ".", minQuoted(d.Object.Name, dbType), nil
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
// guidance text can never inject markup. An unpaired backtick stays literal text.
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

// severityLabelText maps each Severity to its emoji + label, as shown in a
// finding's "Impact & action" block.
var severityLabelText = map[Severity]string{
	SeverityAdvisory:            "ℹ️ Advisory",
	SeverityPotentialImpact:     "⚠️ Potential impact",
	SeverityBreaksRecoverable:   "⛔ Breaks the migration — recoverable",
	SeverityBreaksUnrecoverable: "🚨 Breaks the migration — unrecoverable",
}

func severityLabel(sev Severity) (string, error) {
	s, ok := severityLabelText[sev]
	if !ok {
		return "", fmt.Errorf("unexpected severity %q", sev)
	}
	return s, nil
}

// stringifyValue renders a DriftEntry.OldValue/NewValue (an `any` whose
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
//   - schemasnapshot.Table                     -> the table's full column list
//     (TABLE_ADDED/DROPPED's whole added/dropped table), e.g.
//     "id integer NOT NULL, email text"; "no columns" when it has none
//   - anything else                          -> an error
func stringifyValue(attribute schemadiff.Attribute, value any, dbType string) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case bool:
		if attribute == schemadiff.AttrNullability {
			if v {
				return "NOT NULL", nil
			}
			return "NULL", nil
		}
		if v {
			return "true", nil
		}
		return "false", nil
	case schemasnapshot.ObjectRef:
		return v.ForDisplay(dbType), nil
	case []schemasnapshot.ObjectRef:
		parts := make([]string, len(v))
		for i, ref := range v {
			parts[i] = ref.ForDisplay(dbType)
		}
		return strings.Join(parts, ", "), nil
	case schemasnapshot.Column:
		return stringifyColumnDef(v), nil
	case schemasnapshot.Table:
		if len(v.Columns) == 0 {
			return "no columns", nil
		}
		parts := make([]string, len(v.Columns))
		for i, c := range v.Columns {
			parts[i] = minQuoted(c.Name, dbType) + " " + stringifyColumnDef(c)
		}
		return strings.Join(parts, ", "), nil
	default:
		return "", fmt.Errorf("unexpected value type %T for attribute %q", value, attribute)
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
// Report.CapturePoints. The live read (Label == schemasnapshot.LabelSourceLive) shows "—" for its
// sequence number, since it is never persisted/numbered like a stored
// snapshot.
func snapshotRows(capturePoints []CapturePoint) []snapshotRow {
	rows := make([]snapshotRow, len(capturePoints))
	for i, c := range capturePoints {
		seq := fmt.Sprintf("%d", i+1)
		if c.Label == schemasnapshot.LabelSourceLive {
			seq = "—"
		}
		note := ""
		switch {
		case c.Excluded != "":
			note = "not compared: " + c.Excluded
		case c.Label == schemasnapshot.LabelSourceLive:
			note = "read fresh at report time · not stored"
		case c.Reason != "":
			note = "reason: " + c.Reason
		}
		rows[i] = snapshotRow{
			Seq:        seq,
			Label:      c.Label,
			CapturedAt: formatTime(c.CapturedAt),
			Note:       note,
		}
	}
	return rows
}
