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
Emitting the probe catalog, and the unit tests for the derivations behind it.

Nothing here needs Docker: the catalog is built from the case table's own text plus
package-level variables in srcdb, so it can be generated in CI on its own.

	SWEEP_CATALOG_OUT=results/probe-catalog.json \
	  go test -tags integration_live_migration ./src/testlivemigration/ \
	  -run TestDatatypeSweepCatalog
*/

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDatatypeSweepCatalog writes the probe catalog: the static half of the published
// report, generated from the case table rather than typed by hand.
//
// It always runs (a catalog that only exists when someone remembers to ask for it is the
// drift this whole design removes); SWEEP_CATALOG_OUT only chooses where it lands.
func TestDatatypeSweepCatalog(t *testing.T) {
	out := os.Getenv("SWEEP_CATALOG_OUT")
	if out == "" {
		out = filepath.Join("results", "probe-catalog.json")
	}

	doc := probeCatalogDoc{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		VoyagerCommit: currentVoyagerCommit(),
		Entries:       buildProbeCatalog(),
	}
	if len(doc.Entries) == 0 {
		t.Fatal("the probe catalog is empty; allSweepProbes() returned nothing")
	}

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshalling the probe catalog: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(out), err)
	}
	if err := os.WriteFile(out, append(b, '\n'), 0o644); err != nil {
		t.Fatalf("writing %s: %v", out, err)
	}
	t.Logf("wrote %d catalog entries to %s", len(doc.Entries), out)
}

// currentVoyagerCommit stamps the catalog with the build it describes. VOYAGER_COMMIT
// wins so a CI job can pass the commit it actually built.
func currentVoyagerCommit() string {
	if c := os.Getenv("VOYAGER_COMMIT"); c != "" {
		return c
	}
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// ============================================================
// THE DERIVATIONS
// ============================================================

func TestProbeBaseTypeNameNormalisation(t *testing.T) {
	cases := []struct {
		ddl  string
		want string
	}{
		{"int4range", "int4range"},
		{"int4range[]", "int4range"},
		{"text[][]", "text"},
		{"numeric(10,2)", "numeric"},
		{"varchar(255)", "varchar"},
		{"public.geometry(Point,4326)", "geometry"},
		{"timestamp with time zone", "timestamptz"},
		{"time with time zone", "timetz"},
		{"double precision", "float8"},
		{"character varying", "varchar"},
		{"int", "int4"},
		{"{{schema}}.{{p}}_r", ""}, // the probe creates this type; no catalogue name
		{"", ""},
	}
	for _, c := range cases {
		got := probeBaseTypeName(datatypeProbe{ColumnDDL: c.ddl})
		if got != c.want {
			t.Errorf("probeBaseTypeName(%q) = %q, want %q", c.ddl, got, c.want)
		}
	}
}

func TestProbeTypeKindFromPreDDL(t *testing.T) {
	cases := []struct {
		name string
		p    datatypeProbe
		want string
	}{
		{"plain base type", datatypeProbe{ColumnDDL: "int4"}, kindBase},
		{"array of a base type", datatypeProbe{ColumnDDL: "int4[]"}, kindArray},
		{"built-in multirange", datatypeProbe{ColumnDDL: "int4multirange"}, kindMultirange},
		{
			"user-defined range",
			datatypeProbe{
				ColumnDDL: "{{schema}}.{{p}}_r",
				PreDDL:    []string{"CREATE TYPE {{schema}}.{{p}}_r AS RANGE (subtype = integer)"},
			},
			kindRange,
		},
		{
			"user-defined composite",
			datatypeProbe{
				ColumnDDL: "{{schema}}.{{p}}_c",
				PreDDL:    []string{"CREATE TYPE {{schema}}.{{p}}_c AS (a int, b text)"},
			},
			kindComposite,
		},
		{
			"array of a user-defined enum",
			datatypeProbe{
				ColumnDDL: "{{schema}}.{{p}}_e[]",
				PreDDL:    []string{"CREATE TYPE {{schema}}.{{p}}_e AS ENUM ('a','b')"},
			},
			kindEnum + " (array)",
		},
		{
			"domain",
			datatypeProbe{
				ColumnDDL: "{{schema}}.{{p}}_d",
				PreDDL:    []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS int CHECK (VALUE > 0)"},
			},
			kindDomain,
		},
		{
			// PreDDL that creates a helper type the column does NOT use must not
			// reclassify the column.
			"PreDDL unrelated to the column",
			datatypeProbe{
				ColumnDDL: "int4",
				PreDDL:    []string{"CREATE TYPE other AS ENUM ('a')"},
			},
			kindBase,
		},
	}
	for _, c := range cases {
		if got := probeTypeKind(c.p); got != c.want {
			t.Errorf("%s: probeTypeKind = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestReportingColumnsTrackVoyagersLists pins the property that matters: these columns
// are computed from voyager's variables, so a type's classification changes when the
// product's list changes. The assertions name types by their membership rather than
// restating the list, so editing the list moves this test with it.
func TestReportingColumnsTrackVoyagersLists(t *testing.T) {
	type check struct {
		ddl      string
		wantSubs string
	}
	// xml is on PostgresUnsupportedDataTypes -> unsupported for ALL modes.
	// point is on ...ForDbzm but not the offline list -> unsupported for live only.
	// int4 is on neither.
	for _, c := range []check{
		{"xml", "ALL modes"},
		{"point", "live migration"},
		{"int4", "not on any unsupported-datatype list"},
	} {
		base := probeBaseTypeName(datatypeProbe{ColumnDDL: c.ddl})
		assess, analyze := voyagerSchemaReporting(base, kindBase)
		if !strings.Contains(assess, c.wantSubs) {
			t.Errorf("assess column for %s = %q, want it to mention %q", c.ddl, assess, c.wantSubs)
		}
		if assess != analyze {
			t.Errorf("assess and analyze disagree for a non-array type %s:\n  %q\n  %q", c.ddl, assess, analyze)
		}
	}

	// The export-data guardrail is derived from PostgresUnsupportedDataTypesForDbzm.
	if got := exportGuardrailAction("xml", kindBase); !strings.Contains(got, "excluded from the CDC stream") {
		t.Errorf("guardrail action for xml = %q, want an exclusion", got)
	}
	if got := exportGuardrailAction("int4", kindBase); !strings.Contains(got, "no action") {
		t.Errorf("guardrail action for int4 = %q, want no action", got)
	}

	// A user-defined range is caught by the runtime typtype='r' filter, not by a list.
	if got := exportGuardrailAction("", kindRange); !strings.Contains(got, "typtype='r'") {
		t.Errorf("guardrail action for a user-defined range = %q, want the typtype filter", got)
	}

	// Arrays: the guardrail matches the ARRAY type's own typname, so it never fires.
	if got := exportGuardrailAction("xml", kindArray); !strings.Contains(got, "array type's own name") {
		t.Errorf("guardrail action for xml[] = %q, want the array-typname caveat", got)
	}
}

func TestFallbackGuardrailUsesTheYugabyteList(t *testing.T) {
	// tsvector is unsupported with the gRPC connector but fine with the logical one,
	// which is what the harness runs; the column must say so rather than flatten it.
	got := fallbackGuardrailAction("tsvector", kindBase)
	if !strings.Contains(got, "gRPC") {
		t.Errorf("fall-back guardrail for tsvector = %q, want the connector distinction", got)
	}
	if !strings.Contains(fallbackGuardrailAction("point", kindBase), "excluded") {
		t.Error("point is on YugabyteUnsupportedDataTypesForDbzmLogical and must be reported excluded")
	}
	if !strings.Contains(fallbackGuardrailAction("int4", kindBase), "no action") {
		t.Error("int4 is on no list and must be reported as exported")
	}
}

func TestProbeGroupsCoverEveryProbe(t *testing.T) {
	groups := probeGroups()
	for _, p := range allSweepProbes() {
		if groups[p.ID] == "" {
			t.Errorf("probe %s (%s) belongs to no group; the report cannot place it", p.ID, p.TypeName)
		}
	}
}
