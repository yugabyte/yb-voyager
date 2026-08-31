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
The published datatype report is a VIEW OVER THE SUITE, never a parallel artefact.

Its row data is produced here by joining

	the probe catalog  (one entry per probe; static per-type facts, generated from the
	                    case table by TestDatatypeSweepCatalog, including the
	                    reporting-layer columns computed from voyager's own variables)

with

	the results CSV    (one row per measured (probe, mode) pair)

A type that has a probe but no measurement shows NOT_TESTED rather than being quietly
absent, and a measurement with no catalog entry is an ERROR: it means the report and the
suite have drifted, which is exactly what this join exists to make impossible.
*/

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// verdictNotTested is what the report shows for a probe the run did not measure. It is
// a report-layer label, not one of the harness verdicts.
const verdictNotTested = "NOT_TESTED"

// reportModes is the mode order of the published table.
var reportModes = []string{"OFFLINE", "LIVE", "FALL-BACK", "FALL-FORWARD"}

// CatalogEntry mirrors the JSON emitted by TestDatatypeSweepCatalog. Keep the json tags
// in lockstep with datatype_report_meta.go.
type CatalogEntry struct {
	ProbeID      string   `json:"probe_id"`
	TypeName     string   `json:"type_name"`
	Group        string   `json:"group"`
	ColumnDDL    string   `json:"column_ddl"`
	BaseTypeName string   `json:"base_type_name"`
	Kind         string   `json:"kind"`
	Extensions   []string `json:"extensions,omitempty"`
	Poison       bool     `json:"poison,omitempty"`

	ReportedByAssess        string `json:"reported_by_assess"`
	ReportedByAnalyze       string `json:"reported_by_analyze"`
	GuardrailAction         string `json:"guardrail_action"`
	GuardrailActionFallback string `json:"guardrail_action_fallback"`
	ReportedByDocs          string `json:"reported_by_docs"`

	Note string `json:"note,omitempty"`
}

// Catalog is the whole generated catalog file.
type Catalog struct {
	GeneratedAt   string         `json:"generated_at"`
	VoyagerCommit string         `json:"voyager_commit"`
	Entries       []CatalogEntry `json:"entries"`
}

// ModeResult is one cell of the report table.
type ModeResult struct {
	Verdict     string `json:"verdict"`
	Evidence    string `json:"evidence,omitempty"`
	SourceValue string `json:"source_value,omitempty"`
	TargetValue string `json:"target_value,omitempty"`
	RunStatus   string `json:"run_status,omitempty"`
}

// ReportRow is one published row: a type, its group, one verdict per mode, and the
// reporting-layer columns.
type ReportRow struct {
	ProbeID      string `json:"probe_id"`
	TypeName     string `json:"type_name"`
	Group        string `json:"group"`
	Kind         string `json:"kind,omitempty"`
	BaseTypeName string `json:"base_type_name,omitempty"`

	Offline     ModeResult `json:"offline"`
	Live        ModeResult `json:"live"`
	FallBack    ModeResult `json:"fall_back"`
	FallForward ModeResult `json:"fall_forward"`

	ReportedByAssess        string `json:"reported_by_assess"`
	ReportedByAnalyze       string `json:"reported_by_analyze"`
	GuardrailAction         string `json:"guardrail_action"`
	GuardrailActionFallback string `json:"guardrail_action_fallback"`
	ReportedByDocs          string `json:"reported_by_docs"`

	Note string `json:"note,omitempty"`
}

// ReportDoc is the artefact the HTML page loads (or that a person pastes in).
type ReportDoc struct {
	GeneratedAt   string      `json:"generated_at"`
	VoyagerCommit string      `json:"voyager_commit"`
	PGVersion     string      `json:"pg_version"`
	YBVersion     string      `json:"yb_version"`
	Rows          []ReportRow `json:"rows"`
}

func ReadCatalog(path string) (*Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parsing catalog %s: %w", path, err)
	}
	if len(c.Entries) == 0 {
		return nil, fmt.Errorf("catalog %s has no entries", path)
	}
	return &c, nil
}

// BuildReport joins the catalog with the results. It returns the document plus the list
// of problems that make the join untrustworthy:
//
//   - a result row whose probe id is not in the catalog (suite/report drift)
//   - a catalog entry with no measurement in a required mode (only when requiredModes
//     is non-empty)
func BuildReport(cat *Catalog, rows []Row, requiredModes []string) (*ReportDoc, []string) {
	byProbeMode := map[string]Row{}
	for _, r := range rows {
		byProbeMode[r.Key()] = r
	}
	known := map[string]bool{}
	for _, e := range cat.Entries {
		known[e.ProbeID] = true
	}

	var problems []string
	orphans := map[string]bool{}
	for _, r := range rows {
		if !known[r.ProbeID] {
			orphans[r.ProbeID] = true
		}
	}
	for _, id := range sortedKeys(orphans) {
		problems = append(problems, fmt.Sprintf(
			"result row for probe %q has no catalog entry: the report and the case table have drifted", id))
	}

	doc := &ReportDoc{
		GeneratedAt:   cat.GeneratedAt,
		VoyagerCommit: cat.VoyagerCommit,
	}
	for _, r := range rows {
		if doc.PGVersion == "" {
			doc.PGVersion = r.PGVersion
		}
		if doc.YBVersion == "" {
			doc.YBVersion = r.YBVersion
		}
		if doc.VoyagerCommit == "" {
			doc.VoyagerCommit = r.VoyagerCommit
		}
	}

	cell := func(id, mode string) ModeResult {
		r, ok := byProbeMode[id+"|"+mode]
		if !ok {
			return ModeResult{Verdict: verdictNotTested}
		}
		return ModeResult{
			Verdict:     r.Verdict,
			Evidence:    r.Evidence,
			SourceValue: r.SourceValue,
			TargetValue: r.TargetValue,
			RunStatus:   r.RunStatus,
		}
	}

	for _, e := range cat.Entries {
		row := ReportRow{
			ProbeID:                 e.ProbeID,
			TypeName:                e.TypeName,
			Group:                   e.Group,
			Kind:                    e.Kind,
			BaseTypeName:            e.BaseTypeName,
			Offline:                 cell(e.ProbeID, "OFFLINE"),
			Live:                    cell(e.ProbeID, "LIVE"),
			FallBack:                cell(e.ProbeID, "FALL-BACK"),
			FallForward:             cell(e.ProbeID, "FALL-FORWARD"),
			ReportedByAssess:        e.ReportedByAssess,
			ReportedByAnalyze:       e.ReportedByAnalyze,
			GuardrailAction:         e.GuardrailAction,
			GuardrailActionFallback: e.GuardrailActionFallback,
			ReportedByDocs:          e.ReportedByDocs,
			Note:                    e.Note,
		}
		doc.Rows = append(doc.Rows, row)

		for _, m := range requiredModes {
			if cell(e.ProbeID, m).Verdict == verdictNotTested {
				problems = append(problems, fmt.Sprintf(
					"probe %s (%s) has no %s measurement in the supplied results", e.ProbeID, e.TypeName, m))
			}
		}
	}

	sort.SliceStable(doc.Rows, func(i, j int) bool {
		if doc.Rows[i].Group != doc.Rows[j].Group {
			return doc.Rows[i].Group < doc.Rows[j].Group
		}
		return doc.Rows[i].ProbeID < doc.Rows[j].ProbeID
	})
	return doc, problems
}

var reportCSVHeader = []string{
	"probe_id", "type_name", "group", "kind", "base_type_name",
	"offline_verdict", "live_verdict", "fall_back_verdict", "fall_forward_verdict",
	"evidence",
	"reported_by_assess", "reported_by_analyze",
	"guardrail_action", "guardrail_action_fallback", "reported_by_docs",
	"note",
}

// WriteReportCSV writes the same rows in the flat shape a spreadsheet or an HTML table
// wants. The evidence column carries the most informative non-empty evidence string,
// preferring the mode whose verdict is worst, since that is the one worth quoting.
func WriteReportCSV(path string, doc *ReportDoc) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(reportCSVHeader); err != nil {
		return err
	}
	for _, r := range doc.Rows {
		rec := []string{
			r.ProbeID, r.TypeName, r.Group, r.Kind, r.BaseTypeName,
			r.Offline.Verdict, r.Live.Verdict, r.FallBack.Verdict, r.FallForward.Verdict,
			bestEvidence(r),
			r.ReportedByAssess, r.ReportedByAnalyze,
			r.GuardrailAction, r.GuardrailActionFallback, r.ReportedByDocs,
			r.Note,
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// bestEvidence picks the evidence string worth publishing for a row: the one attached to
// the worst product verdict, falling back to any non-empty evidence.
func bestEvidence(r ReportRow) string {
	cells := []ModeResult{r.Offline, r.Live, r.FallBack, r.FallForward}
	best := ""
	bestRank := len(verdictRank) + 1
	for _, c := range cells {
		if strings.TrimSpace(c.Evidence) == "" || c.Evidence == "-" {
			continue
		}
		rk, isProduct := rank(c.Verdict)
		if !isProduct {
			rk = len(verdictRank) // non-product verdicts lose to any product verdict
		}
		if rk < bestRank {
			bestRank = rk
			best = c.Evidence
		}
	}
	return best
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
