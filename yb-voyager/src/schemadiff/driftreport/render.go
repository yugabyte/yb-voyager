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
	"html/template"
	"strings"
	"time"
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
	funcMap := template.FuncMap{
		"formatTime": formatTime,
	}
	tmpl, err := template.New("drift_report").Funcs(funcMap).Parse(driftReportTemplateSource)
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
	return t.Format(time.RFC3339)
}

// ─── View model: grouping/formatting logic kept in Go, not in the template ──

// reportView is the unexported view model handed to the HTML template. It
// pre-computes display strings and groups diffs by interval so the template
// itself stays a thin iteration-and-field-access layer.
type reportView struct {
	Report

	ComparingSchemas     string
	ComparingTables      string
	ComparingObjectTypes string

	Groups []intervalGroup
}

// intervalGroup is all DiffEntries that share the same Window (and therefore
// the same Phase), rendered together as one timeline section.
type intervalGroup struct {
	Window Window
	Phase  string
	Diffs  []DiffEntry
}

func newReportView(r Report) reportView {
	return reportView{
		Report:               r,
		ComparingSchemas:     joinOrAll(r.Comparing.Schemas),
		ComparingTables:      joinOrAll(r.Comparing.Tables),
		ComparingObjectTypes: joinOrAll(r.Comparing.ObjectTypes),
		Groups:               groupByInterval(r.Diffs),
	}
}

// joinOrAll joins items with ", ", or returns "all" when items is empty.
func joinOrAll(items []string) string {
	if len(items) == 0 {
		return "all"
	}
	return strings.Join(items, ", ")
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
