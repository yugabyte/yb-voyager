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
PostGIS / raster / topology INTERNAL types - the 17 gaps the coverage guard found.

These are the types TestDatatypeCatalogCoverage reported MISSING when run against the
PG 17.8-ext image (PostGIS 3.6.4 + postgis_raster + postgis_topology + pgvector):

	box2df, gidx, spheroid, geometry_dump, geomval, addbandarg, agg_count,
	agg_samealignment, rastbandarg, reclassarg, summarystats, unionarg, valid_detail,
	topology.getfaceedges_returntype, topology.topoelement, topology.topoelementarray,
	topology.validatetopology_returntype

They are here rather than in deliberateNonMigrationTypes ON PURPOSE. "PostGIS helper
types are not real columns" is a CATEGORY JUDGEMENT, and category judgements about which
types are real have been wrong every time they have been made. Every one of these accepts
`CREATE TABLE t(v <type>)` on a PostGIS-enabled server, so a user can have one in a
column, so it gets a probe and its behaviour becomes a MEASUREMENT.

The expected measurement, for the record, is that the probe reports SKIPPED with
"extension unavailable: postgis on target": YugabyteDB cannot install PostGIS at all, so
the type does not exist on the target. That is a real, recorded finding about the target
rather than an assumption about the type - which is the whole point of not short-circuiting
them into an exclusion list.

They are grouped as one batch, "postgis-internal", so the report renders them as a single
collapsible block instead of 17 lines of noise.

Registered as the "postgis-internal" batch in sweepBatches(). Every probe sets NullOnly,
which exempts it from the "InitialValue must differ from AltValue" guard: a NULL-only
column has exactly one possible value, so demanding two distinct ones is unsatisfiable.

Why NULL-only, and how to upgrade them:

The coverage guard classified all 17 as NULL-only. For box2df and gidx that is inherent -
they are GiST index support types whose input function rejects every literal. For the
composites it was partly an artifact of the guard's generic literal list, which had no row
literal of the right arity; that is now fixed (compositeAllNullLiteral builds the
all-NULL-fields row "(,)" for any arity, which is a genuine non-NULL value).

So the upgrade path is mechanical and NOT yet walked: re-run the coverage guard on the
-ext image, and any of these that now report "full value probe" come back with the exact
literal that was accepted. Swap that literal in as InitialValue, pick a second distinct
one, and drop NullOnly for that probe.

Deliberately NOT guessing the field lists of geometry_dump, summarystats and friends here.
A wrong guess makes the probe report SKIPPED("initial data rejected"), which is a
case-table bug wearing the costume of a product finding - and NullOnly is documented as
something a probe may only claim after the literal has actually been refused at run time.
For box2df and gidx that refusal is on record; for the other fifteen it is the guard's
generic-literal verdict, which the arity fix may well overturn. Until that re-run happens
these fifteen are honest but provisional, and the Note on each says so.
*/

// postgisInternalProbes covers the PostGIS/raster/topology internal types.
//
// Every probe declares the extension that owns the type, so on a server without it the
// probe self-reports SKIPPED instead of taking the batch down - and on YugabyteDB, where
// PostGIS cannot be installed at all, that SKIPPED IS the finding.
func postgisInternalProbes() []datatypeProbe {
	// nullOnly builds a probe for a type that can hold NULL and nothing else.
	// InitialValue and AltValue are deliberately identical: see the PENDING note above.
	nullOnly := func(id, name, ext, ddl, note string) datatypeProbe {
		return datatypeProbe{
			ID: id, Name: name, TypeName: name,
			Extensions:   []string{ext},
			ColumnDDL:    ddl,
			InitialValue: "NULL::" + ddl,
			AltValue:     "NULL::" + ddl,
			NullOnly:     true,
			Note:         note,
		}
	}

	// provisional wraps nullOnly for the types whose NULL-only status rests on the
	// coverage guard's GENERIC literal list rather than on a recorded refusal. The
	// arity-derived composite literal may well overturn it, so the caveat travels with
	// the probe into the report instead of living only in this file's header.
	provisional := func(id, name, ext, ddl, note string) datatypeProbe {
		return nullOnly(id, name, ext, ddl, note+
			"; PROVISIONAL NULL-only - no literal was accepted by the coverage guard's generic list, "+
			"re-check with the arity-derived composite literal and upgrade to a full-value probe if one is accepted")
	}

	return []datatypeProbe{
		// --- GiST index support types: no input function accepts any literal. -------
		nullOnly("POSTGIS-INT-001", "box2df", "postgis", "box2df",
			"GiST index support type for geometry; input function rejects every literal, so NULL is the only value a column can hold"),
		nullOnly("POSTGIS-INT-002", "gidx", "postgis", "gidx",
			"GiST index support type for the n-dimensional index; NULL-only"),

		// --- spheroid: has an input function, but of a shape the guard's generic
		// --- literal list does not cover. Left NULL-only until the guard reports the
		// --- literal it accepts.
		provisional("POSTGIS-INT-003", "spheroid", "postgis", "spheroid",
			"parameterises geodetic calculations; the guard reported NULL-only - re-check, spheroid does have an input function of the form SPHEROID[\"name\",a,rf]"),

		// --- PostGIS composite return types ---------------------------------------
		provisional("POSTGIS-INT-004", "geometry_dump", "postgis", "geometry_dump",
			"composite returned by ST_Dump; a user can persist one in a column"),
		provisional("POSTGIS-INT-005", "geomval", "postgis", "geomval",
			"composite (geometry, value) returned by the raster/vector conversions"),
		provisional("POSTGIS-INT-006", "valid_detail", "postgis", "valid_detail",
			"composite returned by ST_IsValidDetail"),

		// --- postgis_raster composite types ----------------------------------------
		provisional("POSTGIS-INT-007", "addbandarg", "postgis_raster", "addbandarg",
			"composite argument type for ST_AddBand"),
		provisional("POSTGIS-INT-008", "agg_count", "postgis_raster", "agg_count",
			"composite used by the raster aggregate machinery"),
		provisional("POSTGIS-INT-009", "agg_samealignment", "postgis_raster", "agg_samealignment",
			"composite used by the raster alignment aggregate"),
		provisional("POSTGIS-INT-010", "rastbandarg", "postgis_raster", "rastbandarg",
			"composite (raster, band index) argument type"),
		provisional("POSTGIS-INT-011", "reclassarg", "postgis_raster", "reclassarg",
			"composite argument type for ST_Reclass"),
		provisional("POSTGIS-INT-012", "summarystats", "postgis_raster", "summarystats",
			"composite returned by ST_SummaryStats; a plausible thing to persist"),
		provisional("POSTGIS-INT-013", "unionarg", "postgis_raster", "unionarg",
			"composite argument type for ST_Union"),

		// --- postgis_topology types ------------------------------------------------
		provisional("POSTGIS-INT-014", "topology.getfaceedges_returntype", "postgis_topology",
			"topology.getfaceedges_returntype",
			"composite returned by topology.GetFaceEdges"),
		provisional("POSTGIS-INT-015", "topology.topoelement", "postgis_topology",
			"topology.topoelement",
			"DOMAIN over integer[] with a CHECK; the domain form is exactly the shape that has bypassed name-equality guardrails before"),
		provisional("POSTGIS-INT-016", "topology.topoelementarray", "postgis_topology",
			"topology.topoelementarray",
			"DOMAIN over a two-dimensional integer[]"),
		provisional("POSTGIS-INT-017", "topology.validatetopology_returntype", "postgis_topology",
			"topology.validatetopology_returntype",
			"composite returned by topology.ValidateTopology"),
	}
}
