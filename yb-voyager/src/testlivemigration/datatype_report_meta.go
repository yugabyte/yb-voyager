//go:build integration_live_migration

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
package testlivemigration

/*
The PROBE CATALOG: the static, per-type half of the published datatype report.

The report has two kinds of column:

  - per-RUN columns  (offline/live/fall-back verdicts, evidence) - those come from the
    results CSV that `sweepreport collect` builds out of a run's PROBE-RESULT lines.
  - per-TYPE columns (which voyager surface mentions the type, what the export-data
    guardrail does with it, whether the public docs list it) - those are properties of
    the type, not of a run, and they are built here.

Everything in the first three of those per-type columns is DERIVED AT RUN TIME from
voyager's own variables:

	srcdb.PostgresUnsupportedDataTypes
	srcdb.GetPGLiveMigrationUnsupportedDatatypes()          (PostgresUnsupportedDataTypesForDbzm minus the above)
	srcdb.GetPGLiveMigrationWithFFOrFBUnsupportedDatatypes()(YugabyteUnsupportedDataTypesForDbzm* minus the above two)
	srcdb.PostgresUnsupportedDataTypesForDbzm               (the export-data guardrail's own list)
	srcdb.GetYugabyteUnsupportedDatatypesDbzm(false)        (the fall-back / export-from-target guardrail)
	plus the runtime typtype='r' user-defined-range filter in
	PostgreSQL.GetColumnsWithSupportedTypes

so that editing one of those lists changes the report on the next run instead of leaving
a hand-typed string behind. Only the docs column is hardcoded, with the doc URL beside it.

Matching semantics are copied from the product deliberately, warts included:
utils.ContainsAnyStringFromSlice is a case-insensitive EQUALITY test against
pg_type.typname as returned by srcdb.getAllTableColumnsInfo. For an array column that
typname is the array type's own name ("_xml", not "xml"), which is why an array of an
unsupported type is reported here as NOT matched by the guardrail. That asymmetry is a
finding about voyager, not a bug in this file; assess-migration does strip the "[]" via
utils.TableColumnsDataTypes.GetBaseTypeNameFromDatatype, so the two columns legitimately
disagree for arrays.
*/

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// ============================================================
// CATALOG SHAPE
// ============================================================

// probeCatalogEntry is one row of the report's static half. The json tags are the
// contract with src/testlivemigration/sweepreport/report.go - keep them in lockstep.
type probeCatalogEntry struct {
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

// probeCatalogDoc is the file TestDatatypeSweepCatalog writes.
type probeCatalogDoc struct {
	GeneratedAt   string              `json:"generated_at"`
	VoyagerCommit string              `json:"voyager_commit"`
	Entries       []probeCatalogEntry `json:"entries"`
}

// Type-kind labels. These describe the shape of the column under test, which is what
// decides whether voyager's per-name lists can even apply to it.
const (
	kindBase       = "base"
	kindArray      = "array"
	kindDomain     = "domain"
	kindEnum       = "enum"
	kindComposite  = "composite"
	kindRange      = "range (user-defined)"
	kindMultirange = "multirange (user-defined)"
)

// ============================================================
// BUILDING THE CATALOG
// ============================================================

// buildProbeCatalog turns the case table into the report's static half. It needs no
// database: every input is either the probe's own text or a package-level variable in
// srcdb, so it runs in CI without Docker.
func buildProbeCatalog() []probeCatalogEntry {
	groups := probeGroups()
	probes := allSweepProbes()

	entries := make([]probeCatalogEntry, 0, len(probes))
	for _, p := range probes {
		base := probeBaseTypeName(p)
		kind := probeTypeKind(p)
		assess, analyze := voyagerSchemaReporting(base, kind)

		entries = append(entries, probeCatalogEntry{
			ProbeID:                 p.ID,
			TypeName:                p.TypeName,
			Group:                   groups[p.ID],
			ColumnDDL:               p.ColumnDDL,
			BaseTypeName:            base,
			Kind:                    kind,
			Extensions:              p.Extensions,
			Poison:                  p.Poison,
			ReportedByAssess:        assess,
			ReportedByAnalyze:       analyze,
			GuardrailAction:         exportGuardrailAction(base, kind),
			GuardrailActionFallback: fallbackGuardrailAction(base, kind),
			ReportedByDocs:          docsMention(base, p),
			Note:                    catalogNote(p),
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Group != entries[j].Group {
			return entries[i].Group < entries[j].Group
		}
		return entries[i].ProbeID < entries[j].ProbeID
	})
	return entries
}

// probeGroups maps every probe id to the group the report renders it under: its batch
// name, or "controls" / "poison" for the two tables that are not batches.
func probeGroups() map[string]string {
	groups := map[string]string{}
	for _, p := range controlProbes() {
		groups[p.ID] = "controls"
	}
	for _, b := range sweepBatches() {
		for _, p := range b.Probes {
			groups[p.ID] = b.Name
		}
	}
	for _, p := range poisonProbes() {
		if _, already := groups[p.ID]; !already {
			groups[p.ID] = "poison"
		}
	}
	return groups
}

// catalogNote carries the probe's free-form note, plus the poison justification, into
// the report so an excluded-from-batches row explains itself.
func catalogNote(p datatypeProbe) string {
	parts := []string{}
	if strings.TrimSpace(p.Note) != "" {
		parts = append(parts, p.Note)
	}
	if p.Poison && strings.TrimSpace(p.PoisonNote) != "" {
		parts = append(parts, "poison: "+p.PoisonNote)
	}
	if p.ExpectExcluded {
		parts = append(parts, "expected to be excluded by voyager's unsupported-datatype guardrail")
	}
	return strings.Join(parts, "; ")
}

// ============================================================
// DERIVING THE TYPE'S IDENTITY FROM ITS PROBE
// ============================================================

// sqlSpellingToCatalogName maps the SQL-standard spellings a probe may legitimately use
// in ColumnDDL to the pg_type.typname that voyager actually matches against. Without
// this, a probe written as "timestamp with time zone" would silently miss TIMESTAMPTZ.
var sqlSpellingToCatalogName = map[string]string{
	"integer":                     "int4",
	"int":                         "int4",
	"smallint":                    "int2",
	"bigint":                      "int8",
	"real":                        "float4",
	"double precision":            "float8",
	"decimal":                     "numeric",
	"boolean":                     "bool",
	"character varying":           "varchar",
	"character":                   "bpchar",
	"char":                        "bpchar",
	"bit varying":                 "varbit",
	"timestamp with time zone":    "timestamptz",
	"timestamp without time zone": "timestamp",
	"time with time zone":         "timetz",
	"time without time zone":      "time",
}

// probeBaseTypeName reduces a probe's ColumnDDL to the pg_type.typname voyager's lists
// are keyed on. It returns "" for a type the probe itself creates (a template), because
// such a type has no fixed catalog name and voyager can only classify it by typtype.
func probeBaseTypeName(p datatypeProbe) string {
	ddl := strings.TrimSpace(strings.ToLower(p.ColumnDDL))
	if ddl == "" || strings.Contains(ddl, "{{") {
		return ""
	}
	// Drop array suffixes: "int4range[]", "text[][]", "int[3]".
	for {
		trimmed := strings.TrimRight(ddl, " ")
		if i := strings.LastIndex(trimmed, "["); i > 0 && strings.HasSuffix(trimmed, "]") {
			ddl = strings.TrimSpace(trimmed[:i])
			continue
		}
		break
	}
	// Drop a trailing type modifier: "numeric(10,2)", "varchar(255)", "vector(3)".
	if i := strings.Index(ddl, "("); i > 0 {
		ddl = strings.TrimSpace(ddl[:i])
	}
	// Drop schema qualification: "public.geometry" -> "geometry".
	if i := strings.LastIndex(ddl, "."); i >= 0 {
		ddl = ddl[i+1:]
	}
	ddl = strings.TrimSpace(ddl)
	if mapped, ok := sqlSpellingToCatalogName[ddl]; ok {
		return mapped
	}
	// Anything still containing a space is a multi-word spelling we do not know about.
	// Returning it verbatim is better than guessing: it will simply not match a list,
	// and the coverage guard's round-trip check will show it up.
	return ddl
}

// probeTypeKind classifies the column under test. Composite / enum / range / domain are
// read off the probe's own PreDDL, because that is what actually creates them.
func probeTypeKind(p datatypeProbe) string {
	ddl := strings.TrimSpace(strings.ToLower(p.ColumnDDL))
	isArray := strings.HasSuffix(ddl, "]")

	pre := strings.ToLower(strings.Join(p.PreDDL, "\n"))
	// Only classify by PreDDL when the column actually uses a type the PreDDL created.
	usesOwnType := strings.Contains(ddl, "{{p}}")

	kind := kindBase
	switch {
	case usesOwnType && strings.Contains(pre, "as range"):
		kind = kindRange
	case usesOwnType && strings.Contains(pre, "as enum"):
		kind = kindEnum
	case usesOwnType && strings.Contains(pre, "create domain"):
		kind = kindDomain
	case usesOwnType && strings.Contains(pre, "create type") && strings.Contains(pre, "as ("):
		kind = kindComposite
	case strings.HasSuffix(probeBaseTypeName(p), "multirange"):
		kind = kindMultirange
	}
	if isArray {
		if kind == kindBase {
			return kindArray
		}
		return kind + " (array)"
	}
	return kind
}

// ============================================================
// THE REPORTING-LAYER COLUMNS, COMPUTED FROM VOYAGER'S OWN LISTS
// ============================================================

// voyagerSchemaReporting answers "does assess-migration / analyze-schema mention this
// type, and in which bucket".
//
// Both surfaces run the same three-way classification (see
// cmd.fetchColumnsWithUnsupportedDataTypes and
// queryissue.(*ParserIssueDetector) table-column loop), so one derivation serves both;
// where they differ - assess strips "[]" from the type name, analyze-schema works from
// the parsed DDL's type name - the difference is called out in the string.
func voyagerSchemaReporting(base, kind string) (assess, analyze string) {
	offline := srcdb.PostgresUnsupportedDataTypes
	live := srcdb.GetPGLiveMigrationUnsupportedDatatypes()
	fffb := srcdb.GetPGLiveMigrationWithFFOrFBUnsupportedDatatypes()

	// A type the probe creates itself has no catalog name to match. Composites are
	// reported as UDTs for live-with-ff/fb by both surfaces; the other user-defined
	// kinds are not on any name list at all.
	if base == "" {
		if strings.HasPrefix(kind, kindComposite) {
			const s = "yes - user-defined type, reported as unsupported for live migration with fall-forward/fall-back"
			return s, s
		}
		const s = "no - user-defined type, not on any unsupported-datatype list"
		return s, s
	}

	// Arrays are matched on their ELEMENT type by both surfaces: assess-migration strips
	// the "[]" in utils.TableColumnsDataTypes.GetBaseTypeNameFromDatatype, and the
	// schema-issue detector matches the parsed column's type name. Say so in the string
	// rather than implying the array type itself is on a list.
	isArray := kind == kindArray || strings.HasSuffix(kind, "(array)")
	arrayNote := ""
	if isArray {
		arrayNote = ", matched on the element type after stripping []"
	}

	switch {
	case utils.ContainsAnyStringFromSlice(offline, base):
		s := fmt.Sprintf("yes - unsupported for ALL modes (srcdb.PostgresUnsupportedDataTypes contains %q%s)", base, arrayNote)
		return s, s
	case utils.ContainsAnyStringFromSlice(live, base):
		s := fmt.Sprintf("yes - unsupported for live migration (srcdb.GetPGLiveMigrationUnsupportedDatatypes contains %q%s)", base, arrayNote)
		return s, s
	case utils.ContainsAnyStringFromSlice(fffb, base):
		s := fmt.Sprintf("yes - unsupported for live migration with fall-forward/fall-back "+
			"(srcdb.GetPGLiveMigrationWithFFOrFBUnsupportedDatatypes contains %q%s)", base, arrayNote)
		return s, s
	}
	const none = "no - not on any unsupported-datatype list"
	return none, none
}

// exportGuardrailAction answers "what does `export data` from PostgreSQL do with a
// column of this type", derived from PostgreSQL.GetColumnsWithSupportedTypes.
//
// Two rules there, both reproduced here:
//  1. name is in srcdb.PostgresUnsupportedDataTypesForDbzm  -> column excluded
//  2. the type is a user-defined RANGE type (a runtime pg_type typtype='r' query,
//     getAllUserDefinedRangeTypes) -> column excluded
//
// The list is only consulted for live migration; an offline export carries every column.
func exportGuardrailAction(base, kind string) string {
	if strings.HasPrefix(kind, kindRange) || strings.HasPrefix(kind, kindMultirange) {
		if base == "" {
			return "excluded from the CDC stream - user-defined range/multirange type, caught by the " +
				"runtime typtype='r' filter in PostgreSQL.getAllUserDefinedRangeTypes (live modes only)"
		}
	}
	// Arrays are checked BEFORE the name list on purpose. The guardrail matches
	// pg_type.typname as returned by srcdb.getAllTableColumnsInfo, and for an array
	// column that is the ARRAY type's own name ("_xml"), never the element's - so an
	// array of an unsupported type is not excluded even though the element would be.
	if kind == kindArray || strings.HasSuffix(kind, "(array)") {
		return "no action - the guardrail matches pg_type.typname, which for an array column is the " +
			"array type's own name (e.g. \"_xml\"), so an array is never matched by the element type's entry"
	}
	if base != "" && utils.ContainsAnyStringFromSlice(srcdb.PostgresUnsupportedDataTypesForDbzm, base) {
		return fmt.Sprintf("excluded from the CDC stream - srcdb.PostgresUnsupportedDataTypesForDbzm contains %q "+
			"(live modes only; an offline export carries the column)", base)
	}
	return "no action - column is exported"
}

// fallbackGuardrailAction is the same question for the reverse direction: export data
// FROM TARGET, i.e. YugabyteDB.GetColumnsWithSupportedTypes, which uses its own list.
// Default (logical) connector, matching the harness's own configuration.
func fallbackGuardrailAction(base, kind string) string {
	if base == "" {
		if strings.HasPrefix(kind, kindComposite) {
			return "excluded from the reverse CDC stream - user-defined type, dropped by " +
				"YugabyteDB.filterUnsupportedUserDefinedDatatypes"
		}
		return "not on the YugabyteDB unsupported list by name"
	}
	list := srcdb.GetYugabyteUnsupportedDatatypesDbzm(false) // logical connector is the default
	if utils.ContainsAnyStringFromSlice(list, base) {
		return fmt.Sprintf("excluded from the reverse CDC stream - "+
			"srcdb.YugabyteUnsupportedDataTypesForDbzmLogical contains %q", base)
	}
	if utils.ContainsAnyStringFromSlice(srcdb.YugabyteUnsupportedDataTypesForDbzmGrpc, base) {
		return fmt.Sprintf("exported with the logical connector; excluded with the gRPC connector "+
			"(srcdb.YugabyteUnsupportedDataTypesForDbzmGrpc contains %q)", base)
	}
	return "no action - column is exported"
}

// ============================================================
// THE DOCS COLUMN (the only hardcoded one)
// ============================================================

// docsUnsupportedTypes is the set of datatypes the PUBLIC DOCS call out as unsupported
// or specially handled. This is the one column that cannot be derived from voyager's
// source, so it is typed by hand and must be re-read when the docs page changes.
//
// Source: "Datatype mappings" and "Known issues - PostgreSQL" in the YugabyteDB Voyager
// documentation:
//
//	https://docs.yugabyte.com/preview/yugabyte-voyager/reference/datatype-mapping-pg/
//	https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/
//
// Keys are pg_type.typname, lower case, matching probeBaseTypeName's output.
var docsUnsupportedTypes = map[string]string{
	"xml":            "listed as unsupported for migration",
	"lo":             "listed as unsupported (large objects)",
	"pg_lsn":         "listed as unsupported",
	"txid_snapshot":  "listed as unsupported",
	"xid":            "listed as unsupported",
	"geometry":       "listed as unsupported (PostGIS)",
	"geography":      "listed as unsupported (PostGIS)",
	"box2d":          "listed as unsupported (PostGIS)",
	"box3d":          "listed as unsupported (PostGIS)",
	"topogeometry":   "listed as unsupported (PostGIS)",
	"raster":         "listed as unsupported (PostGIS)",
	"int4multirange": "listed as unsupported (multirange)",
	"int8multirange": "listed as unsupported (multirange)",
	"nummultirange":  "listed as unsupported (multirange)",
	"tsmultirange":   "listed as unsupported (multirange)",
	"tstzmultirange": "listed as unsupported (multirange)",
	"datemultirange": "listed as unsupported (multirange)",
	"point":          "listed as unsupported for live migration (geometric)",
	"line":           "listed as unsupported for live migration (geometric)",
	"lseg":           "listed as unsupported for live migration (geometric)",
	"box":            "listed as unsupported for live migration (geometric)",
	"path":           "listed as unsupported for live migration (geometric)",
	"polygon":        "listed as unsupported for live migration (geometric)",
	"circle":         "listed as unsupported for live migration (geometric)",
	"timetz":         "listed as unsupported for live migration",
	"vector":         "listed as unsupported for live migration (pgvector)",
	"tsquery":        "listed as unsupported for live migration with fall-forward/fall-back",
	"tsvector":       "documented as supported with the logical connector only",
	"hstore":         "documented as supported with the logical connector only",
	"citext":         "documented as supported with the logical connector only",
	"ltree":          "documented as supported with the logical connector only",
	"user-defined":   "user-defined types are documented as unsupported for live migration with fall-forward/fall-back",
}

// docsMention is the report's docs column for one probe.
func docsMention(base string, p datatypeProbe) string {
	if base == "" {
		if strings.HasPrefix(probeTypeKind(p), kindComposite) {
			return docsUnsupportedTypes["user-defined"]
		}
		return "not mentioned"
	}
	if s, ok := docsUnsupportedTypes[base]; ok {
		return s
	}
	return "not mentioned as unsupported"
}
