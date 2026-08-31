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
THE COVERAGE GUARD.

The sweep's case table is a hand-written list, so on its own it can only ever be as
complete as whoever last looked at it. This test removes that dependency: it asks the
LIVE PostgreSQL catalogue what types exist, decides EMPIRICALLY which of them a user
could put in a column, and fails listing every such type that has no probe.

A new PostgreSQL release, or an extension someone adds to the test image, therefore
breaks this test rather than silently going untested.

Two rules, and only two:

 1. A type is excluded ONLY if `CREATE TABLE scratch(v <type>)` actually fails. The
    exclusion is derived at run time and carries the server's verbatim error, so the
    exclusion list can never drift from reality and nobody can quietly exclude a type on
    category grounds ("it's an index support type", "it's internal", "it's a statistics
    type"). Those judgements have been wrong before: aclitem, pg_node_tree, pg_ndistinct,
    pg_mcv_list, pg_dependencies, pg_brin_bloom_summary, pg_brin_minmax_multi_summary,
    gtsvector, ghstore, gtrgm, the gbtreekey family, ltree_gist, intbig_gkey, query_int,
    earth and the tablefunc_crosstab types ALL accept a column in PG 17, and query_int,
    earth and tablefunc_crosstab_2 store real values.

 2. Anything a column can be declared as REQUIRES a probe. The only escape is
    deliberateNonMigrationTypes below, which is for types the suite consciously does not
    migrate for a stated PRODUCT reason - each entry carries its justification and every
    run prints them, so an exclusion is visible rather than buried in code.

The guard also asserts the round trip in the other direction: every probe must appear in
the generated report data. Together the two directions make "we report on exactly what we
test" a test failure rather than a review question.

Run it on its own (needs Docker, ~2 min, no YugabyteDB container):

	go test -tags integration_live_migration ./src/testlivemigration/ \
	    -run TestDatatypeCatalogCoverage -timeout 20m -v

Set SWEEP_COVERAGE_MODE=report to print the gaps without failing, e.g. while the case
table is being extended.
*/

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// coverageScratchSchema holds the throwaway table the empirical checks use. It is
// deliberately NOT "public": a probe table's row type is itself a composite type, and
// putting it in public would make the catalogue scan see its own scratch object.
const coverageScratchSchema = "dt_coverage_scratch"

// coverageExtensions are installed before the catalogue is read, so that extension types
// are held to exactly the same standard as built-ins. The list is the union of every
// extension the case table asks for, plus the ones the suite's test images ship.
func coverageExtensions() []string {
	wanted := map[string]bool{}
	for _, p := range allSweepProbes() {
		for _, e := range p.Extensions {
			wanted[strings.ToLower(e)] = true
		}
	}
	// Extensions the sweep cares about even if no probe currently names one: their
	// absence from the case table is precisely what this guard should catch.
	for _, e := range []string{
		"hstore", "citext", "ltree", "cube", "seg", "isn", "intarray",
		"pg_trgm", "btree_gist", "btree_gin", "earthdistance", "tablefunc", "lo",
		// postgis_raster and postgis_topology carry a large family of their own types
		// (rastbandarg, topology.topoelement, ...). Installing them here rather than
		// relying on the image having done it keeps the guard's scope explicit.
		"postgis", "postgis_raster", "postgis_topology", "vector",
	} {
		wanted[e] = true
	}
	out := make([]string, 0, len(wanted))
	for e := range wanted {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

// deliberateNonMigrationTypes is the ONLY hand-maintained exclusion list, and it is not
// for types that cannot hold a value - those are derived at run time. It is for types
// that exist, accept a column, and that the suite consciously does not migrate for a
// stated PRODUCT reason.
//
// Keys are "<schema>.<typname>". Every entry MUST carry a justification, and every run
// prints the whole map, so an exclusion is a visible decision rather than a silent one.
//
// It is empty on purpose. If you are about to add an entry, the bar is: "voyager will
// never carry a column of this type, and here is why" - not "this type looks internal".
var deliberateNonMigrationTypes = map[string]string{}

// coverageProbeLiterals are tried in order to decide whether a type can hold a real
// value or only NULL. This is ADVISORY: it tells whoever writes the probe whether to
// expect a full value probe or a NULL-only one. It never gates the test, because a type
// this list happens not to have a literal for still needs a probe.
var coverageProbeLiterals = []string{
	"1", "0", "a", "", "{}", "{1,2}", "{{1,2}}", "()", "t", "2024-01-01", "(0,0)", "1.5", "<a>1</a>",
}

// compositeAllNullLiteral builds the one literal that is valid for ANY composite type of
// a given arity: a row whose every field is NULL, e.g. "(,)" for a two-field type. The
// row itself is NOT NULL, so it is a real value and distinguishable from NULL.
//
// Without this, every composite of arity >= 2 was classified NULL-only purely because the
// generic literal list happens not to contain a row literal of the right shape - an
// artifact of the harness, reported as a fact about the type.
func compositeAllNullLiteral(arity int) string {
	if arity < 1 {
		return ""
	}
	return "(" + strings.Repeat(",", arity-1) + ")"
}

// compositeArity counts a composite type's live attributes.
func compositeArity(db *sql.DB, oid uint32) (int, error) {
	var n int
	err := db.QueryRow(`
		SELECT count(*)
		FROM pg_attribute a
		JOIN pg_type t ON t.typrelid = a.attrelid
		WHERE t.oid = $1 AND a.attnum > 0 AND NOT a.attisdropped`, int64(oid)).Scan(&n)
	return n, err
}

// Value classes reported for a type that needs a probe.
const (
	valueClassFull    = "full value probe"
	valueClassNull    = "NULL-only probe"
	valueClassUnknown = "unknown"
)

// pgCatalogType is one candidate type read from the live catalogue, annotated with what
// the server actually let us do with it.
type pgCatalogType struct {
	OID        uint32
	Schema     string
	Name       string
	TypType    string
	FormatName string // format_type(oid, NULL): what to write in a CREATE TABLE

	Columnable  bool   // CREATE TABLE (v <type>) succeeded
	CreateError string // verbatim server error when it did not
	ValueClass  string // valueClassFull / valueClassNull / valueClassUnknown
	SampleValue string // the literal that was accepted, when one was
}

func (c pgCatalogType) Key() string { return c.Schema + "." + c.Name }

// coverageOutcome is the decision, split so each bucket can be reported separately.
type coverageOutcome struct {
	Missing      []pgCatalogType // column-able, no probe, no deliberate exclusion: FAILURE
	Covered      []pgCatalogType
	Deliberate   []pgCatalogType
	AutoExcluded []pgCatalogType // CREATE TABLE failed; the error is the justification
}

// classifyCoverage is the whole decision, kept free of database access so it can be
// unit-tested against a faked catalogue.
//
// covered maps a type OID to the probe id that covers it.
func classifyCoverage(types []pgCatalogType, covered map[uint32]string, deliberate map[string]string) coverageOutcome {
	var out coverageOutcome
	for _, t := range types {
		switch {
		case !t.Columnable:
			out.AutoExcluded = append(out.AutoExcluded, t)
		case covered[t.OID] != "":
			out.Covered = append(out.Covered, t)
		case deliberate[t.Key()] != "":
			out.Deliberate = append(out.Deliberate, t)
		default:
			out.Missing = append(out.Missing, t)
		}
	}
	return out
}

// ============================================================
// THE CATALOGUE-DRIVEN GUARD (needs a PostgreSQL container)
// ============================================================

func TestDatatypeCatalogCoverage(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", DatabaseName: "dtsweep_coverage"},
		SchemaNames: []string{coverageScratchSchema},
		SchemaSQL: []string{
			fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE;", coverageScratchSchema),
			fmt.Sprintf("CREATE SCHEMA %s;", coverageScratchSchema),
		},
		CleanupSQL: []string{fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE;", coverageScratchSchema)},
	})
	defer lm.Cleanup()

	if err := lm.SetupContainers(context.Background()); err != nil {
		t.Fatalf("failed to setup containers: %v", err)
	}
	if err := lm.SetupSchema(); err != nil {
		t.Fatalf("failed to setup schema: %v", err)
	}

	// Extensions first: an extension type must clear the same bar as a built-in.
	var installed, unavailable []string
	for _, ext := range coverageExtensions() {
		err := lm.WithSourceConn(func(db *sql.DB) error {
			_, e := db.Exec("CREATE EXTENSION IF NOT EXISTS " + ext)
			return e
		})
		if err != nil {
			unavailable = append(unavailable, ext)
			continue
		}
		installed = append(installed, ext)
	}
	t.Logf("extensions installed: %s", strings.Join(installed, ", "))
	if len(unavailable) > 0 {
		t.Logf("extensions NOT available on this image (their types are out of scope for this run): %s",
			strings.Join(unavailable, ", "))
	}

	var types []pgCatalogType
	var covered map[uint32]string
	err := lm.WithSourceConn(func(db *sql.DB) error {
		var e error
		types, e = readCandidateTypes(db)
		if e != nil {
			return e
		}
		annotateColumnability(t, db, types)
		covered, e = resolveProbeCoverage(t, db)
		return e
	})
	if err != nil {
		t.Fatalf("reading the catalogue: %v", err)
	}
	if len(types) == 0 {
		t.Fatal("the catalogue scan returned no candidate types; the query is wrong")
	}

	outcome := classifyCoverage(types, covered, deliberateNonMigrationTypes)

	t.Logf("catalogue scan: %d candidate types, %d covered by a probe, %d auto-excluded "+
		"(no column can be declared), %d deliberately not migrated, %d MISSING",
		len(types), len(outcome.Covered), len(outcome.AutoExcluded),
		len(outcome.Deliberate), len(outcome.Missing))

	// Auto-exclusions are printed with the server's own words, so the reason for every
	// exclusion is visible in the run rather than asserted in code.
	sortTypes(outcome.AutoExcluded)
	for _, x := range outcome.AutoExcluded {
		t.Logf("auto-excluded %s: %s", x.Key(), x.CreateError)
	}

	// The hand-maintained list is printed in full, empty or not.
	if len(deliberateNonMigrationTypes) == 0 {
		t.Logf("deliberateNonMigrationTypes is empty: every column-able type requires a probe")
	}
	for _, k := range sortedMapKeys(deliberateNonMigrationTypes) {
		t.Logf("deliberately not migrated: %s - %s", k, deliberateNonMigrationTypes[k])
	}

	if len(outcome.Missing) == 0 {
		return
	}

	sortTypes(outcome.Missing)
	var b strings.Builder
	fmt.Fprintf(&b, "%d datatype(s) can hold a column on this server but have no probe in "+
		"datatype_sweep_cases.go and no entry in deliberateNonMigrationTypes:\n", len(outcome.Missing))
	for _, m := range outcome.Missing {
		sample := m.SampleValue
		if sample != "" {
			sample = fmt.Sprintf(" (accepted literal %q)", sample)
		}
		fmt.Fprintf(&b, "  %-40s typtype=%s  declare as %-28s  -> write a %s%s\n",
			m.Key(), m.TypType, m.FormatName, m.ValueClass, sample)
	}
	b.WriteString("\nAdd a probe for each, or - only for a stated product reason - an entry in " +
		"deliberateNonMigrationTypes with its justification.\n")

	if os.Getenv("SWEEP_COVERAGE_MODE") == "report" {
		t.Logf("SWEEP_COVERAGE_MODE=report, not failing:\n%s", b.String())
		return
	}
	t.Fatal(b.String())
}

// readCandidateTypes asks the catalogue for everything a user could plausibly declare a
// column as.
//
//   - typisdefined: shell types have no storage
//   - typtype in (b,e,d,r,m,c): base, enum, domain, range, multirange, composite.
//     Pseudo-types ('p') cannot be a column at all.
//   - array types are skipped: an array is covered by its element type's probe, and the
//     case table has dedicated array probes of its own.
//   - a composite that is some table's row type is skipped: it is a table, not a type
//     someone declares a column as.
//   - schema scope is pg_catalog + public + anything owned by an installed extension.
//     Everything else (information_schema, the sweep's own scratch schema) is out.
func readCandidateTypes(db *sql.DB) ([]pgCatalogType, error) {
	const q = `
		SELECT t.oid::bigint, n.nspname, t.typname, t.typtype, format_type(t.oid, NULL)
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE t.typisdefined
		  AND t.typtype IN ('b','e','d','r','m','c')
		  AND NOT (t.typelem <> 0 AND t.typlen = -1)
		  AND left(t.typname, 1) <> '_'
		  AND NOT EXISTS (
		        SELECT 1 FROM pg_class c
		        WHERE c.oid = t.typrelid AND c.relkind <> 'c')
		  AND (
		        n.nspname IN ('pg_catalog', 'public')
		     OR EXISTS (
		          SELECT 1 FROM pg_depend d
		          WHERE d.objid = t.oid
		            AND d.classid = 'pg_type'::regclass
		            AND d.refclassid = 'pg_extension'::regclass)
		  )
		ORDER BY n.nspname, t.typname`

	rows, err := db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("querying pg_type: %w", err)
	}
	defer rows.Close()

	var out []pgCatalogType
	for rows.Next() {
		var c pgCatalogType
		var oid int64
		if err := rows.Scan(&oid, &c.Schema, &c.Name, &c.TypType, &c.FormatName); err != nil {
			return nil, err
		}
		c.OID = uint32(oid)
		c.ValueClass = valueClassUnknown
		out = append(out, c)
	}
	return out, rows.Err()
}

// annotateColumnability answers, for every candidate, the only question that may exclude
// it: can a column of this type be created at all? It then classifies the ones that can
// as full-value or NULL-only, which is advice for whoever writes the probe.
//
// Every statement runs in its own implicit transaction, so a rejected literal cannot
// poison the next attempt.
func annotateColumnability(t *testing.T, db *sql.DB, types []pgCatalogType) {
	t.Helper()
	table := coverageScratchSchema + ".cov_probe_t"

	for i := range types {
		c := &types[i]

		if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			t.Fatalf("cannot drop the coverage scratch table: %v", err)
		}
		if _, err := db.Exec(fmt.Sprintf("CREATE TABLE %s (v %s)", table, c.FormatName)); err != nil {
			c.CreateError = oneLine(err.Error())
			continue
		}
		c.Columnable = true

		if _, err := db.Exec(fmt.Sprintf("INSERT INTO %s VALUES (NULL)", table)); err == nil {
			c.ValueClass = valueClassNull
		}

		// A composite's all-NULL-fields row literal is tried first and is arity-derived,
		// so it is right for every composite rather than only the one-field ones the
		// generic list happens to cover.
		literals := coverageProbeLiterals
		if c.TypType == "c" {
			if arity, err := compositeArity(db, c.OID); err == nil {
				if lit := compositeAllNullLiteral(arity); lit != "" {
					literals = append([]string{lit}, literals...)
				}
			}
		}
		for _, lit := range literals {
			_, err := db.Exec(fmt.Sprintf("INSERT INTO %s VALUES ($1)", table), lit)
			if err == nil {
				c.ValueClass = valueClassFull
				c.SampleValue = lit
				break
			}
		}
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
		t.Logf("could not drop the coverage scratch table: %v", err)
	}
}

// resolveProbeCoverage maps catalogue OIDs to the probe that covers them, by asking the
// SERVER to resolve each probe's ColumnDDL. Using to_regtype rather than string matching
// is what makes "int" cover int4, "timestamp with time zone" cover timestamptz and
// "public.geometry" cover the PostGIS type, with no alias table to maintain.
//
// A probe whose ColumnDDL is a template ({{p}}) creates its own type and therefore
// covers nothing in the catalogue - such types do not exist until the probe runs.
// An array probe covers its element type as well.
func resolveProbeCoverage(t *testing.T, db *sql.DB) (map[uint32]string, error) {
	t.Helper()
	covered := map[uint32]string{}
	var unresolved []string

	for _, p := range allSweepProbes() {
		ddl := strings.TrimSpace(p.ColumnDDL)
		if ddl == "" || strings.Contains(ddl, "{{") {
			continue
		}
		var oid, elem int64
		var typlen int
		err := db.QueryRow(`
			SELECT t.oid::bigint, t.typelem::bigint, t.typlen
			FROM pg_type t WHERE t.oid = to_regtype($1)`, ddl).Scan(&oid, &elem, &typlen)
		if err == sql.ErrNoRows {
			unresolved = append(unresolved, fmt.Sprintf("%s (%s)", p.ID, ddl))
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("resolving probe %s type %q: %w", p.ID, ddl, err)
		}
		if covered[uint32(oid)] == "" {
			covered[uint32(oid)] = p.ID
		}
		if elem != 0 && typlen == -1 && covered[uint32(elem)] == "" {
			covered[uint32(elem)] = p.ID
		}
	}
	if len(unresolved) > 0 {
		// Not a failure: a probe for an extension this image lacks legitimately does not
		// resolve. It is logged because a typo in a ColumnDDL looks exactly the same.
		sort.Strings(unresolved)
		t.Logf("%d probe type(s) did not resolve on this server (missing extension, or a typo "+
			"in ColumnDDL): %s", len(unresolved), strings.Join(unresolved, ", "))
	}
	return covered, nil
}

// ============================================================
// THE OTHER HALF OF THE ROUND TRIP (no container needed)
// ============================================================

// TestDatatypeReportCoversEveryProbe asserts the second direction of the round trip:
// every probe in the case table turns into exactly one row of the generated report data.
//
// With TestDatatypeCatalogCoverage (every catalogue type has a probe) this closes the
// loop: the published report and the test suite cover the same set of types, and
// drifting apart is a test failure rather than something a reviewer has to notice.
func TestDatatypeReportCoversEveryProbe(t *testing.T) {
	entries := buildProbeCatalog()
	probes := allSweepProbes()

	if len(entries) != len(probes) {
		t.Fatalf("probe catalog has %d entries for %d probes: the report would not be a view "+
			"over the suite", len(entries), len(probes))
	}

	byID := map[string]probeCatalogEntry{}
	for _, e := range entries {
		if _, dup := byID[e.ProbeID]; dup {
			t.Errorf("probe %s appears twice in the generated report data", e.ProbeID)
		}
		byID[e.ProbeID] = e
	}

	for _, p := range probes {
		e, ok := byID[p.ID]
		if !ok {
			t.Errorf("probe %s (%s) has no row in the generated report data", p.ID, p.TypeName)
			continue
		}
		if e.TypeName == "" {
			t.Errorf("probe %s has an empty type name; the report row would be unlabelled", p.ID)
		}
		if e.Group == "" {
			t.Errorf("probe %s is in no batch, controls table or poison table, so the report "+
				"cannot group it. Add it to a batch in sweepBatches().", p.ID)
		}
		// The reporting-layer columns are derived, never blank: a blank one means the
		// derivation silently failed to classify the type.
		for name, val := range map[string]string{
			"reported_by_assess":        e.ReportedByAssess,
			"reported_by_analyze":       e.ReportedByAnalyze,
			"guardrail_action":          e.GuardrailAction,
			"guardrail_action_fallback": e.GuardrailActionFallback,
			"reported_by_docs":          e.ReportedByDocs,
		} {
			if strings.TrimSpace(val) == "" {
				t.Errorf("probe %s has an empty %s column in the generated report data", p.ID, name)
			}
		}
	}
}

// ============================================================
// SET LOGIC, TESTED WITHOUT A DATABASE
// ============================================================

func TestClassifyCoverageSetLogic(t *testing.T) {
	types := []pgCatalogType{
		{OID: 1, Schema: "pg_catalog", Name: "int4", Columnable: true},
		{OID: 2, Schema: "pg_catalog", Name: "gtsvector", Columnable: true}, // column-able: needs a probe
		{OID: 3, Schema: "pg_catalog", Name: "impossible", Columnable: false, // cannot be a column
			CreateError: `column "v" has pseudo-type internal`},
		{OID: 4, Schema: "public", Name: "deliberate", Columnable: true},
	}
	covered := map[uint32]string{1: "CTRL-001"}
	deliberate := map[string]string{"public.deliberate": "product reason"}

	got := classifyCoverage(types, covered, deliberate)

	if len(got.Covered) != 1 || got.Covered[0].Name != "int4" {
		t.Errorf("covered = %v, want just int4", names(got.Covered))
	}
	if len(got.AutoExcluded) != 1 || got.AutoExcluded[0].Name != "impossible" {
		t.Errorf("auto-excluded = %v, want just the type CREATE TABLE rejected", names(got.AutoExcluded))
	}
	if len(got.Deliberate) != 1 || got.Deliberate[0].Name != "deliberate" {
		t.Errorf("deliberate = %v, want just the documented one", names(got.Deliberate))
	}
	if len(got.Missing) != 1 || got.Missing[0].Name != "gtsvector" {
		t.Errorf("missing = %v: a column-able type with no probe MUST be reported missing, "+
			"whatever category it looks like it belongs to", names(got.Missing))
	}
}

func TestClassifyCoverageHasNoCategoryEscapeHatch(t *testing.T) {
	// Every one of these accepts a column in PG 17 and has been wrongly excluded on
	// "it's internal" grounds before. With no probe they must all come out missing.
	var types []pgCatalogType
	for i, name := range []string{
		"aclitem", "pg_node_tree", "pg_ndistinct", "pg_mcv_list", "pg_dependencies",
		"pg_brin_bloom_summary", "pg_brin_minmax_multi_summary", "gtsvector", "ghstore",
		"gtrgm", "gbtreekey16", "gbtreekey_var", "ltree_gist", "intbig_gkey",
		"query_int", "earth", "tablefunc_crosstab_2",
	} {
		types = append(types, pgCatalogType{OID: uint32(i + 1), Schema: "pg_catalog", Name: name, Columnable: true})
	}
	got := classifyCoverage(types, map[uint32]string{}, deliberateNonMigrationTypes)
	if len(got.Missing) != len(types) {
		t.Fatalf("only %d of %d column-able types were reported missing; something is "+
			"excluding types by category: %v", len(got.Missing), len(types), names(got.Covered))
	}
}

// ============================================================
// SMALL HELPERS
// ============================================================

func names(types []pgCatalogType) []string {
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, t.Name)
	}
	return out
}

func sortTypes(types []pgCatalogType) {
	sort.SliceStable(types, func(i, j int) bool { return types[i].Key() < types[j].Key() })
}

func sortedMapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
