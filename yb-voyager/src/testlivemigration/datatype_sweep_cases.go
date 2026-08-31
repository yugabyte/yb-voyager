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
Datatype sweep case tables. One probe here == one row of the final audit matrix.

Conventions:
  - IDs are stable. Never renumber one; append instead.
  - {{schema}} and {{p}} are expanded by datatypeProbe.expandTemplate. Name every
    type/domain a probe creates with the {{p}} prefix so probes cannot collide and no
    probe ever has to CASCADE-drop a neighbour's table.
  - InitialValue seeds the snapshot; AltValue is what the INSERT and the
    "update this column" delta write. They must differ, or the update op proves nothing.
  - Anything that needs an extension declares it. If the extension will not install on
    any participating database the probe self-reports SKIPPED instead of taking the
    batch down. Same for a probe whose DDL or literal the server rejects.

The postgis / pgvector probes assume the custom image (PG 17.8 + postgis 3.6.4 +
postgis_raster + pgvector 0.8.6), selected with PG_VERSION=17.8-ext. Against a plain
postgres:17 they all report SKIPPED, which is a correct answer rather than a failure.
*/

// ============================================================
// CONTROLS - PROBE_SPEC.md "known-answer checks"
// ============================================================

// controlProbes are prepended to every batch. If one of these does not come out WORKS,
// the harness is broken and no other verdict in that batch can be trusted.
func controlProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "CTRL-001", Name: "plain int", TypeName: "int",
			ColumnDDL: "int", InitialValue: "42", AltValue: "-7",
			ExpectVerdict: verdictWorks,
			Note:          "known-good control",
		},
		{
			ID: "CTRL-002", Name: "plain text", TypeName: "text",
			ColumnDDL: "text", InitialValue: "'baseline'", AltValue: "'changed'",
			ExpectVerdict: verdictWorks,
			Note:          "known-good control",
		},
	}
}

// ============================================================
// RANGES AND MULTIRANGES
// ============================================================

func rangeProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "RANGE-001", Name: "int4range", TypeName: "int4range",
			ColumnDDL: "int4range", InitialValue: "'[1,10)'::int4range", AltValue: "'[5,20]'::int4range",
		},
		{
			ID: "RANGE-002", Name: "int8range", TypeName: "int8range",
			ColumnDDL:    "int8range",
			InitialValue: "'[1,9223372036854775806)'::int8range", AltValue: "'[0,10)'::int8range",
		},
		{
			ID: "RANGE-003", Name: "numrange", TypeName: "numrange",
			ColumnDDL: "numrange", InitialValue: "'[1.5,2.5)'::numrange", AltValue: "'(0.10,0.20]'::numrange",
		},
		{
			ID: "RANGE-004", Name: "tsrange", TypeName: "tsrange",
			ColumnDDL:    "tsrange",
			InitialValue: "'[2024-01-01 00:00:00,2024-02-01 12:34:56.789012)'::tsrange",
			AltValue:     "'[1999-12-31 23:59:59,2000-01-01 00:00:00]'::tsrange",
		},
		{
			ID: "RANGE-005", Name: "tstzrange", TypeName: "tstzrange",
			ColumnDDL:    "tstzrange",
			InitialValue: "'[2024-01-01 00:00:00+00,2024-02-01 00:00:00+05:30)'::tstzrange",
			AltValue:     "'[2024-06-01 00:00:00+00,)'::tstzrange",
		},
		{
			ID: "RANGE-006", Name: "daterange", TypeName: "daterange",
			ColumnDDL:    "daterange",
			InitialValue: "'[2024-01-01,2024-02-01)'::daterange", AltValue: "'[2000-01-01,2000-12-31]'::daterange",
		},
		{
			ID: "RANGE-007", Name: "empty range value", TypeName: "int4range (empty)",
			ColumnDDL:    "int4range",
			InitialValue: "'empty'::int4range", AltValue: "'[1,2)'::int4range",
			Note: "value-level edge case: the empty range",
		},
		{
			ID: "RANGE-008", Name: "unbounded range value", TypeName: "int8range (unbounded)",
			ColumnDDL:    "int8range",
			InitialValue: "'(,)'::int8range", AltValue: "'[5,)'::int8range",
			Note: "value-level edge case: both bounds infinite",
		},
		{
			ID: "RANGE-009", Name: "user-defined range type", TypeName: "CREATE TYPE AS RANGE",
			PreDDL: []string{
				"CREATE TYPE {{schema}}.{{p}}_r AS RANGE (subtype = integer, multirange_type_name = {{schema}}.{{p}}_mr)",
			},
			ColumnDDL:    "{{schema}}.{{p}}_r",
			InitialValue: "'[1,5)'::{{schema}}.{{p}}_r", AltValue: "'[10,20)'::{{schema}}.{{p}}_r",
		},
		{
			ID: "RANGE-010", Name: "array of user-defined range", TypeName: "user range[]",
			PreDDL: []string{
				"CREATE TYPE {{schema}}.{{p}}_r AS RANGE (subtype = integer, multirange_type_name = {{schema}}.{{p}}_mr)",
			},
			ColumnDDL:    "{{schema}}.{{p}}_r[]",
			InitialValue: "ARRAY['[1,5)'::{{schema}}.{{p}}_r, '[10,20)'::{{schema}}.{{p}}_r]",
			AltValue:     "ARRAY['[100,200)'::{{schema}}.{{p}}_r]",
			Note:         "array form of a user-defined type; ran clean in isolation - the STUCK seen in the first LIVE ranges batch was collateral from an export-boot flake, not this type",
		},
		{
			ID: "RANGE-011", Name: "array of built-in range", TypeName: "int4range[]",
			ColumnDDL:    "int4range[]",
			InitialValue: "ARRAY['[1,5)'::int4range, '[10,20)'::int4range]",
			AltValue:     "ARRAY['[100,200)'::int4range]",
			Note:         "contrast case for RANGE-010: array of a BUILT-IN range rather than a user-defined one",
		},
	}
}

func multirangeProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "MRANGE-001", Name: "int4multirange", TypeName: "int4multirange",
			ColumnDDL:    "int4multirange",
			InitialValue: "'{[1,5),[10,20)}'::int4multirange", AltValue: "'{[100,200)}'::int4multirange",
		},
		{
			ID: "MRANGE-002", Name: "datemultirange", TypeName: "datemultirange",
			ColumnDDL:    "datemultirange",
			InitialValue: "'{[2024-01-01,2024-02-01),[2024-06-01,2024-07-01)}'::datemultirange",
			AltValue:     "'{[2000-01-01,2000-02-01)}'::datemultirange",
		},
		{
			ID: "MRANGE-004", Name: "int8multirange", TypeName: "int8multirange",
			ColumnDDL:    "int8multirange",
			InitialValue: "'{[10,20),[30,40)}'::int8multirange",
			AltValue:     "'{[9223372036854775800,9223372036854775806)}'::int8multirange",
		},
		{
			ID: "MRANGE-005", Name: "nummultirange", TypeName: "nummultirange",
			ColumnDDL:    "nummultirange",
			InitialValue: "'{[1.5,2.5)}'::nummultirange",
			AltValue:     "'{[0.001,0.002),[10.5,20.25)}'::nummultirange",
		},
		{
			ID: "MRANGE-006", Name: "tsmultirange", TypeName: "tsmultirange",
			ColumnDDL:    "tsmultirange",
			InitialValue: `'{["2024-01-01 00:00:00","2024-02-01 12:34:56.789012")}'::tsmultirange`,
			AltValue:     `'{["1999-12-31 23:59:59","2000-01-01 00:00:00")}'::tsmultirange`,
		},
		{
			ID: "MRANGE-007", Name: "tstzmultirange", TypeName: "tstzmultirange",
			ColumnDDL:    "tstzmultirange",
			InitialValue: `'{["2024-01-01 00:00:00+00","2024-02-01 00:00:00+00")}'::tstzmultirange`,
			AltValue:     `'{["2024-06-01 00:00:00+00","2024-07-01 00:00:00+00")}'::tstzmultirange`,
			Note:         "text form is TimeZone-dependent; a pure UTC-offset difference is a harness artifact, not a product finding",
		},
		{
			ID: "MRANGE-003", Name: "user-defined multirange", TypeName: "user multirange",
			PreDDL: []string{
				"CREATE TYPE {{schema}}.{{p}}_r AS RANGE (subtype = integer, multirange_type_name = {{schema}}.{{p}}_mr)",
			},
			ColumnDDL:    "{{schema}}.{{p}}_mr",
			InitialValue: "'{[1,5),[10,20)}'::{{schema}}.{{p}}_mr",
			AltValue:     "'{[42,43)}'::{{schema}}.{{p}}_mr",
			Note:         "multirange auto-created alongside a user-defined range",
		},
	}
}

// ============================================================
// DOMAINS
// ============================================================

func domainProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "DOM-001", Name: "domain over int", TypeName: "domain(integer)",
			PreDDL:       []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS integer CHECK (VALUE > 0)"},
			ColumnDDL:    "{{schema}}.{{p}}_d",
			InitialValue: "7", AltValue: "9",
		},
		{
			ID: "DOM-002", Name: "domain over text with CHECK", TypeName: "domain(text) CHECK",
			PreDDL:       []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS text CHECK (length(VALUE) > 0)"},
			ColumnDDL:    "{{schema}}.{{p}}_d",
			InitialValue: "'abc'", AltValue: "'defg'",
		},
		{
			ID: "DOM-003", Name: "domain over xml", TypeName: "domain(xml)",
			Poison:       true,
			PoisonNote:   "POISON: deterministic BLOCKS in LIVE (import: syntax error at or near '<', SQLSTATE 42601)",
			PreDDL:       []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS xml"},
			ColumnDDL:    "{{schema}}.{{p}}_d",
			InitialValue: "'<a attr=\"1\">text</a>'::xml", AltValue: "'<b/>'::xml",
			ExpectExcluded: true,
			Note:           "does the domain form bypass the name-equality guardrail on xml?",
		},
		{
			ID: "DOM-004", Name: "domain over point", TypeName: "domain(point)",
			Poison:       true,
			PoisonNote:   "POISON: deterministic BLOCKS in LIVE (import: syntax error at or near '{', SQLSTATE 42601)",
			PreDDL:       []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS point"},
			ColumnDDL:    "{{schema}}.{{p}}_d",
			InitialValue: "'(1.5,-2.5)'::point", AltValue: "'(0,0)'::point",
			ExpectExcluded: true,
			Note:           "does the domain form bypass the name-equality guardrail on point?",
		},
		{
			ID: "DOM-005", Name: "domain over enum", TypeName: "domain(enum)",
			Poison:     true,
			PoisonNote: "POISON: deterministic BLOCKS in LIVE (export data itself dies; connector TypeRegistry NPE)",
			PreDDL: []string{
				"CREATE TYPE {{schema}}.{{p}}_e AS ENUM ('sad', 'ok', 'happy')",
				"CREATE DOMAIN {{schema}}.{{p}}_d AS {{schema}}.{{p}}_e CHECK (VALUE <> 'sad')",
			},
			ColumnDDL:    "{{schema}}.{{p}}_d",
			InitialValue: "'ok'::{{schema}}.{{p}}_e", AltValue: "'happy'::{{schema}}.{{p}}_e",
			Note: "domain-over-enum has been observed to kill the connector at startup",
		},
		{
			ID: "DOM-006", Name: "domain over numeric(12,4)", TypeName: "domain(numeric(12,4))",
			PreDDL:       []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS numeric(12,4)"},
			ColumnDDL:    "{{schema}}.{{p}}_d",
			InitialValue: "12345.6789", AltValue: "-0.0001",
		},
		{
			ID: "DOM-007", Name: "array of domain", TypeName: "domain(integer)[]",
			PreDDL:       []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS integer CHECK (VALUE > 0)"},
			ColumnDDL:    "{{schema}}.{{p}}_d[]",
			InitialValue: "ARRAY[1,2,3]::{{schema}}.{{p}}_d[]", AltValue: "ARRAY[9]::{{schema}}.{{p}}_d[]",
		},
	}
}

// ============================================================
// COMPOSITES
// ============================================================

func compositeProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "COMP-001", Name: "simple composite", TypeName: "composite",
			PreDDL:       []string{"CREATE TYPE {{schema}}.{{p}}_c AS (x integer, y text)"},
			ColumnDDL:    "{{schema}}.{{p}}_c",
			InitialValue: "ROW(1, 'a')::{{schema}}.{{p}}_c", AltValue: "ROW(-2, 'zz')::{{schema}}.{{p}}_c",
		},
		{
			ID: "COMP-002", Name: "nested composite", TypeName: "composite(composite)",
			PreDDL: []string{
				"CREATE TYPE {{schema}}.{{p}}_inner AS (x integer, y text)",
				"CREATE TYPE {{schema}}.{{p}}_outer AS (a {{schema}}.{{p}}_inner, b numeric)",
			},
			ColumnDDL:    "{{schema}}.{{p}}_outer",
			InitialValue: "ROW(ROW(1,'a')::{{schema}}.{{p}}_inner, 2.5)::{{schema}}.{{p}}_outer",
			AltValue:     "ROW(ROW(-9,'q,uote')::{{schema}}.{{p}}_inner, -0.001)::{{schema}}.{{p}}_outer",
		},
		{
			ID: "COMP-003", Name: "array of composite", TypeName: "composite[]",
			PreDDL:       []string{"CREATE TYPE {{schema}}.{{p}}_c AS (x integer, y text)"},
			ColumnDDL:    "{{schema}}.{{p}}_c[]",
			InitialValue: "ARRAY[ROW(1,'a')::{{schema}}.{{p}}_c, ROW(2,'b')::{{schema}}.{{p}}_c]",
			AltValue:     "ARRAY[ROW(3,'c')::{{schema}}.{{p}}_c]",
		},
		{
			ID: "COMP-004", Name: "composite with a NULL field", TypeName: "composite (NULL field)",
			PreDDL:       []string{"CREATE TYPE {{schema}}.{{p}}_c AS (x integer, y text)"},
			ColumnDDL:    "{{schema}}.{{p}}_c",
			InitialValue: "ROW(1, NULL)::{{schema}}.{{p}}_c", AltValue: "ROW(NULL, 'z')::{{schema}}.{{p}}_c",
			Note: "a NULL *inside* a composite is distinct from a NULL composite",
		},
	}
}

// ============================================================
// ENUMS
// ============================================================

func enumProbes() []datatypeProbe {
	// One enum shape reused by three probes; each probe creates its own copy via {{p}}.
	enumDDL := "CREATE TYPE {{schema}}.{{p}}_e AS ENUM ('plain', 'has space', 'has,comma', 'has''quote', 'has\"dquote')"
	return []datatypeProbe{
		{
			ID: "ENUM-001", Name: "plain enum", TypeName: "enum",
			PreDDL:       []string{enumDDL},
			ColumnDDL:    "{{schema}}.{{p}}_e",
			InitialValue: "'plain'::{{schema}}.{{p}}_e", AltValue: "'has space'::{{schema}}.{{p}}_e",
		},
		{
			ID: "ENUM-002", Name: "enum array", TypeName: "enum[]",
			PreDDL:       []string{enumDDL},
			ColumnDDL:    "{{schema}}.{{p}}_e[]",
			InitialValue: "ARRAY['plain','has space']::{{schema}}.{{p}}_e[]",
			AltValue:     "ARRAY['has,comma']::{{schema}}.{{p}}_e[]",
		},
		{
			ID: "ENUM-003", Name: "enum label with quote/comma/space", TypeName: "enum (quoted label)",
			PreDDL:       []string{enumDDL},
			ColumnDDL:    "{{schema}}.{{p}}_e",
			InitialValue: "'has,comma'::{{schema}}.{{p}}_e", AltValue: "'has''quote'::{{schema}}.{{p}}_e",
			Note: "labels containing a comma, a single quote, a double quote and a space",
		},
	}
}

// ============================================================
// ARRAYS
// ============================================================

func arrayProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "ARR-001", Name: "timetz array", TypeName: "timetz[]",
			ColumnDDL:      "timetz[]",
			InitialValue:   "ARRAY['12:34:56.789+05:30','00:00:00+00']::timetz[]",
			AltValue:       "ARRAY['23:59:59.999999-08:00']::timetz[]",
			ExpectExcluded: true,
			Note:           "timetz is on the unsupported list; does the ARRAY form bypass the guardrail?",
		},
		{
			ID: "ARR-002", Name: "point array", TypeName: "point[]",
			ColumnDDL:      "point[]",
			InitialValue:   "ARRAY['(1,2)','(3.5,-4.5)']::point[]",
			AltValue:       "ARRAY['(0,0)']::point[]",
			ExpectExcluded: true,
			Note:           "point is on the unsupported list; does the ARRAY form bypass the guardrail?",
		},
		{
			ID: "ARR-003", Name: "xml array", TypeName: "xml[]",
			ColumnDDL:      "xml[]",
			InitialValue:   "ARRAY['<a>1</a>','<b attr=\"x\"/>']::xml[]",
			AltValue:       "ARRAY['<c/>']::xml[]",
			ExpectExcluded: true,
			Note:           "xml is on the unsupported list; does the ARRAY form bypass the guardrail?",
		},
		{
			ID: "ARR-004", Name: "multidimensional int array", TypeName: "int[][]",
			ColumnDDL:    "int[][]",
			InitialValue: "ARRAY[[1,2,3],[4,5,6]]",
			AltValue:     "ARRAY[[9,8],[7,6]]",
			Note:         "PG stores dimensionality; a flattening round-trip is silent corruption",
		},
		{
			ID: "ARR-005", Name: "array with a NULL element", TypeName: "int[] (NULL element)",
			ColumnDDL:    "int[]",
			InitialValue: "ARRAY[1,NULL,3]::int[]",
			AltValue:     "ARRAY[NULL,NULL]::int[]",
			Note:         "a NULL element is distinct from a NULL array and from 0",
		},
		{
			ID: "ARR-006", Name: "empty array", TypeName: "int[] (empty)",
			ColumnDDL:    "int[]",
			InitialValue: "'{}'::int[]",
			AltValue:     "ARRAY[1]::int[]",
			Note:         "empty array must not collapse to NULL",
		},
		{
			ID: "ARR-007", Name: "array with non-1 lower bound", TypeName: "int[] ('[3:5]')",
			ColumnDDL:    "int[]",
			InitialValue: "'[3:5]={1,2,3}'::int[]",
			AltValue:     "'[0:2]={9,8,7}'::int[]",
			Note:         "PG keeps explicit array bounds; losing them shifts every subscript",
		},
	}
}

// ============================================================
// HSTORE
// ============================================================

func hstoreProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "HSTORE-002", Name: "plain hstore", TypeName: "hstore",
			Extensions:   []string{"hstore"},
			ColumnDDL:    "hstore",
			InitialValue: "'a=>1, b=>2'::hstore", AltValue: "'c=>3'::hstore",
			Note: "batchable hstore baseline; contrast with the HSTORE-001 known-bad control",
		},
		{
			ID: "HSTORE-003", Name: "hstore array", TypeName: "hstore[]",
			Extensions:   []string{"hstore"},
			ColumnDDL:    "hstore[]",
			InitialValue: "ARRAY['a=>1'::hstore, 'b=>2'::hstore]",
			AltValue:     "ARRAY['c=>3'::hstore]",
		},
	}
}

// hstoreNullValueProbe is PROBE_SPEC.md's known-BAD control: main mis-serializes an
// hstore entry whose VALUE is NULL. It is marked Poison, so the runner refuses to batch
// it - run it alone through TestDatatypeSweepSuspect.
func hstoreNullValueProbe() datatypeProbe {
	return datatypeProbe{
		ID: "HSTORE-001", Name: "hstore with a NULL value", TypeName: "hstore (NULL value)",
		Extensions:   []string{"hstore"},
		ColumnDDL:    "hstore",
		InitialValue: "'a=>1, b=>NULL'::hstore", AltValue: "'a=>2, b=>NULL'::hstore",
		Poison: true,
		Note:   "known-bad control: expected to fail on main",
	}
}

// ============================================================
// SYSTEM / IDENTIFIER TYPES
// ============================================================

func systemTypeProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "SYS-001", Name: "xid", TypeName: "xid",
			ColumnDDL: "xid", InitialValue: "'12345'::xid", AltValue: "'67890'::xid",
		},
		{
			ID: "SYS-002", Name: "xid8", TypeName: "xid8",
			ColumnDDL: "xid8", InitialValue: "'123456789'::xid8", AltValue: "'42'::xid8",
		},
		{
			ID: "SYS-003", Name: "tid", TypeName: "tid",
			Poison:     true,
			PoisonNote: "POISON: deterministic BLOCKS in LIVE (import: invalid input syntax for type tid, SQLSTATE 22P02)",
			ColumnDDL:  "tid", InitialValue: "'(0,1)'::tid", AltValue: "'(17,5)'::tid",
		},
		{
			ID: "SYS-004", Name: "regclass", TypeName: "regclass",
			Poison:     true,
			PoisonNote: "POISON: deterministic BLOCKS in LIVE (import: relation '\\x70675f74797065' does not exist, SQLSTATE 42P01)",
			ColumnDDL:  "regclass", InitialValue: "'pg_class'::regclass", AltValue: "'pg_type'::regclass",
			Note: "an OID reference whose text form is resolved against the local catalog",
		},
		{
			ID: "SYS-005", Name: "pg_snapshot", TypeName: "pg_snapshot",
			ColumnDDL: "pg_snapshot", InitialValue: "'10:20:14,15'::pg_snapshot", AltValue: "'30:40:'::pg_snapshot",
		},
		{
			ID: "SYS-006", Name: "int2vector", TypeName: "int2vector",
			ColumnDDL: "int2vector", InitialValue: "'1 2 3'::int2vector", AltValue: "'7 8'::int2vector",
		},
		{
			ID: "SYS-007", Name: "jsonpath", TypeName: "jsonpath",
			ColumnDDL: "jsonpath", InitialValue: "'$.a[*] ? (@ > 2)'::jsonpath", AltValue: "'$.b.c'::jsonpath",
		},
		{
			ID: "SYS-008", Name: "pg_lsn", TypeName: "pg_lsn",
			ColumnDDL: "pg_lsn", InitialValue: "'16/B374D848'::pg_lsn", AltValue: "'0/0'::pg_lsn",
		},
		{
			ID: "SYS-009", Name: "txid_snapshot", TypeName: "txid_snapshot",
			ColumnDDL: "txid_snapshot", InitialValue: "'10:20:14,15'::txid_snapshot", AltValue: "'30:40:'::txid_snapshot",
		},
		{
			ID: "SYS-010", Name: "oid", TypeName: "oid",
			ColumnDDL: "oid", InitialValue: "'4294967295'::oid", AltValue: "'1'::oid",
			Note: "oid is unsigned 32-bit; a signed round-trip turns 4294967295 into -1",
		},
	}
}

// ============================================================
// MISC SCALARS
// ============================================================

func miscTypeProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "MISC-001", Name: "tsquery", TypeName: "tsquery",
			ColumnDDL: "tsquery", InitialValue: "'fat & rat'::tsquery", AltValue: "'cat | dog'::tsquery",
		},
		{
			ID: "MISC-002", Name: "tsvector", TypeName: "tsvector",
			ColumnDDL: "tsvector", InitialValue: "'a b c'::tsvector", AltValue: "'x:1 y:2'::tsvector",
		},
		{
			ID: "MISC-003", Name: "citext", TypeName: "citext",
			Extensions: []string{"citext"},
			ColumnDDL:  "citext", InitialValue: "'MixedCase'::citext", AltValue: "'OTHER value'::citext",
		},
		{
			ID: "MISC-004", Name: "ltree", TypeName: "ltree",
			Extensions: []string{"ltree"},
			ColumnDDL:  "ltree", InitialValue: "'Top.Science.Astronomy'::ltree", AltValue: "'Top.Art'::ltree",
		},
		{
			ID: "MISC-005", Name: "money", TypeName: "money",
			ColumnDDL: "money", InitialValue: "'1234.56'::money", AltValue: "'-9.99'::money",
			// money's text form depends on lc_monetary, which need not match between the
			// PG source and the YB target. Compare the numeric value instead.
			CompareExpr: "(v::numeric)::text",
			Note:        "compared as numeric because the text form is lc_monetary dependent",
		},
		{
			ID: "MISC-006", Name: "macaddr8", TypeName: "macaddr8",
			ColumnDDL:    "macaddr8",
			InitialValue: "'08:00:2b:01:02:03:04:05'::macaddr8", AltValue: "'01:02:03:04:05:06:07:08'::macaddr8",
		},
		{
			ID: "MISC-007", Name: "inet", TypeName: "inet",
			ColumnDDL: "inet", InitialValue: "'192.168.1.5/24'::inet", AltValue: "'2001:db8::1/128'::inet",
			Note: "the /24 netmask must survive; inet is not just an address",
		},
		{
			ID: "MISC-008", Name: "cidr", TypeName: "cidr",
			ColumnDDL: "cidr", InitialValue: "'10.0.0.0/8'::cidr", AltValue: "'2001:db8::/32'::cidr",
		},
		{
			ID: "MISC-009", Name: "bit(3)", TypeName: "bit(3)",
			ColumnDDL: "bit(3)", InitialValue: "B'101'", AltValue: "B'010'",
		},
		{
			ID: "MISC-010", Name: "varbit(100)", TypeName: "varbit(100)",
			ColumnDDL: "varbit(100)", InitialValue: "B'1010101'::varbit(100)", AltValue: "B'1'::varbit(100)",
		},
		{
			ID: "MISC-011", Name: "varbit over 64 bits", TypeName: "varbit (>64 bits)",
			ColumnDDL:    "varbit",
			InitialValue: "B'1010101010101010101010101010101010101010101010101010101010101010101010101010'::varbit",
			AltValue:     "B'1111111111111111111111111111111111111111111111111111111111111111111111111111'::varbit",
			Note:         "76 bits: wider than any integer the connector might squeeze it into",
		},
		{
			ID: "MISC-012", Name: "timetz scalar", TypeName: "timetz",
			ColumnDDL:    "timetz",
			InitialValue: "'12:34:56.789+05:30'::timetz", AltValue: "'23:59:59.999999-08:00'::timetz",
			ExpectExcluded: true,
			Note:           "the scalar form the guardrail is written for; contrast with ARR-001",
		},
	}
}

// ============================================================
// VALUE-LEVEL EDGE CASES
// ============================================================

func valueEdgeProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "VAL-001", Name: "numeric NaN", TypeName: "numeric (NaN)",
			ColumnDDL: "numeric", InitialValue: "'NaN'::numeric", AltValue: "1",
		},
		{
			ID: "VAL-002", Name: "numeric +Infinity", TypeName: "numeric (+Infinity)",
			ColumnDDL: "numeric", InitialValue: "'Infinity'::numeric", AltValue: "'-Infinity'::numeric",
		},
		{
			ID: "VAL-003", Name: "numeric -Infinity", TypeName: "numeric (-Infinity)",
			ColumnDDL: "numeric", InitialValue: "'-Infinity'::numeric", AltValue: "'Infinity'::numeric",
		},
		{
			ID: "VAL-004", Name: "numeric trailing zeros", TypeName: "numeric (0.000)",
			ColumnDDL: "numeric", InitialValue: "0.000", AltValue: "0.00000",
			Note: "PG preserves the scale: 0.000 and 0.00000 are distinct text forms of zero",
		},
		{
			ID: "VAL-005", Name: "numeric(130,60)", TypeName: "numeric(130,60)",
			ColumnDDL: "numeric(130,60)",
			InitialValue: "'1234567890123456789012345678901234567890123456789012345678901234567890" +
				".123456789012345678901234567890123456789012345678901234567890'::numeric(130,60)",
			AltValue: "'0.000000000000000000000000000000000000000000000000000000000001'::numeric(130,60)",
			Note:     "70 integer digits + 60 fractional digits: far wider than float64 or int64",
		},
		{
			ID: "VAL-006", Name: "float8 NaN", TypeName: "float8 (NaN)",
			ColumnDDL: "float8", InitialValue: "'NaN'::float8", AltValue: "1.5",
		},
		{
			ID: "VAL-007", Name: "float8 infinities", TypeName: "float8 (+/-Infinity)",
			ColumnDDL: "float8", InitialValue: "'Infinity'::float8", AltValue: "'-Infinity'::float8",
		},
		{
			ID: "VAL-008", Name: "float8 negative zero", TypeName: "float8 (-0.0)",
			ColumnDDL: "float8", InitialValue: "'-0.0'::float8", AltValue: "'0.0'::float8",
			Note: "-0.0 and 0.0 have different text forms; a round-trip through 0 loses the sign",
		},
		{
			ID: "VAL-009", Name: "BC date", TypeName: "date (BC)",
			ColumnDDL: "date", InitialValue: "'0044-03-15 BC'::date", AltValue: "'0001-01-01 BC'::date",
		},
		{
			ID: "VAL-010", Name: "BC timestamp", TypeName: "timestamp (BC)",
			ColumnDDL:    "timestamp",
			InitialValue: "'0044-03-15 12:00:00 BC'::timestamp", AltValue: "'4713-01-01 00:00:00 BC'::timestamp",
		},
		{
			ID: "VAL-011", Name: "date infinity", TypeName: "date (infinity)",
			ColumnDDL: "date", InitialValue: "'infinity'::date", AltValue: "'2024-01-01'::date",
		},
		{
			ID: "VAL-012", Name: "date -infinity", TypeName: "date (-infinity)",
			ColumnDDL: "date", InitialValue: "'-infinity'::date", AltValue: "'infinity'::date",
		},
		{
			ID: "VAL-013", Name: "timestamp infinity", TypeName: "timestamp (infinity)",
			ColumnDDL:    "timestamp",
			InitialValue: "'infinity'::timestamp", AltValue: "'-infinity'::timestamp",
		},
		{
			ID: "VAL-014", Name: "timestamptz infinity", TypeName: "timestamptz (infinity)",
			ColumnDDL:    "timestamptz",
			InitialValue: "'infinity'::timestamptz", AltValue: "'-infinity'::timestamptz",
		},
		{
			ID: "VAL-015", Name: "time 24:00:00", TypeName: "time (24:00:00)",
			ColumnDDL: "time", InitialValue: "'24:00:00'::time", AltValue: "'00:00:00'::time",
			Note: "PG's only legal hour-24 value; distinct from 00:00:00",
		},
		{
			ID: "VAL-016", Name: "timestamp near PG maximum", TypeName: "timestamp (294247 AD)",
			ColumnDDL:    "timestamp",
			InitialValue: "'294247-01-10 04:00:54'::timestamp", AltValue: "'200000-06-15 12:00:00'::timestamp",
			Note: "close to PG's finite timestamp ceiling; overflows a millisecond epoch",
		},
		{
			ID: "VAL-017", Name: "maximum date", TypeName: "date (5874897-12-31)",
			ColumnDDL: "date", InitialValue: "'5874897-12-31'::date", AltValue: "'1000000-01-01'::date",
		},
		{
			ID: "VAL-018", Name: "huge mixed-sign interval", TypeName: "interval (mixed sign)",
			ColumnDDL:    "interval",
			InitialValue: "'100000 years -11 mons 30 days -23:59:59.999999'::interval",
			AltValue:     "'-1 year 2 mons 3 days -04:05:06.789'::interval",
			Note:         "months/days/microseconds are independent fields and can disagree in sign",
		},
		{
			ID: "VAL-019", Name: "text with emoji, quotes, backslashes", TypeName: "text (escapes)",
			ColumnDDL:    "text",
			InitialValue: `$$4-byte emoji: ` + "\U0001F389" + ` "double" 'single' C:\path\to\file literal-backslash-n: \n$$`,
			AltValue:     `$$plain replacement$$`,
			Note:         "dollar-quoted so the literal reaches PG unescaped",
		},
		{
			ID: "VAL-020", Name: "bytea containing 0x00", TypeName: "bytea (NUL byte)",
			ColumnDDL:    "bytea",
			InitialValue: `'\x00deadbeef00'::bytea`, AltValue: `'\x00'::bytea`,
			Note: "a NUL byte terminates a C string; it must not truncate the value",
		},
		{
			ID: "VAL-021", Name: "empty string vs NULL", TypeName: "text (empty string)",
			ColumnDDL: "text", InitialValue: "''", AltValue: "'x'",
			Note: "'' and NULL must stay distinct across the whole NULL-transition op set",
		},
	}
}

// ============================================================
// POSTGIS
// ============================================================

// postgisProbes require the custom image (PG 17.8 + postgis 3.6.4 + postgis_raster).
// Against a plain postgres:17 every probe here reports
// `SKIPPED | extension unavailable: postgis on <side>`.
//
// The interesting question for these is not only whether data survives, but whether
// voyager's name-equality guardrail catches the SCALAR while MISSING the array and
// domain forms of the same type - and whether the user is told either way.
func postgisProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "GEO-001", Name: "geometry", TypeName: "geometry",
			Extensions:     []string{"postgis"},
			ColumnDDL:      "geometry",
			InitialValue:   "'SRID=4326;LINESTRING(0 0, 1 1)'::geometry",
			AltValue:       "'POINT(9 9)'::geometry",
			ExpectExcluded: true,
			Note:           "scalar form: the shape the guardrail is written for",
		},
		{
			ID: "GEO-002", Name: "geometry(Point,4326)", TypeName: "geometry(Point,4326)",
			Extensions:     []string{"postgis"},
			ColumnDDL:      "geometry(Point,4326)",
			InitialValue:   "'SRID=4326;POINT(-71.060316 42.358431)'::geometry(Point,4326)",
			AltValue:       "'SRID=4326;POINT(0 0)'::geometry(Point,4326)",
			ExpectExcluded: true,
			Note:           "typmod-constrained geometry; the SRID must survive",
		},
		{
			ID: "GEO-003", Name: "geography", TypeName: "geography",
			Extensions:     []string{"postgis"},
			ColumnDDL:      "geography",
			InitialValue:   "'SRID=4326;POINT(-71.060316 42.358431)'::geography",
			AltValue:       "'SRID=4326;POINT(1 1)'::geography",
			ExpectExcluded: true,
		},
		{
			ID: "GEO-004", Name: "geometry array", TypeName: "geometry[]",
			Extensions:      []string{"postgis"},
			ColumnDDL:       "geometry[]",
			InitialValue:    "ARRAY['POINT(1 1)'::geometry, 'LINESTRING(0 0, 2 2)'::geometry]",
			AltValue:        "ARRAY['POINT(7 8)'::geometry]",
			ExpectExcluded:  false,
			RecordDestValue: true,
			Note:            "KEY CASE: does the array form bypass the name-equality guardrail?",
		},
		{
			ID: "GEO-005", Name: "domain over geometry", TypeName: "domain(geometry)",
			Extensions:      []string{"postgis"},
			PreDDL:          []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS geometry"},
			ColumnDDL:       "{{schema}}.{{p}}_d",
			InitialValue:    "'POINT(5 6)'::geometry",
			AltValue:        "'POINT(6 7)'::geometry",
			ExpectExcluded:  false,
			RecordDestValue: true,
			Note:            "KEY CASE: does the domain form bypass the name-equality guardrail?",
		},
		{
			ID: "GEO-006", Name: "domain over geometry(Point,4326)", TypeName: "domain(geometry(Point,4326))",
			Extensions:      []string{"postgis"},
			PreDDL:          []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS geometry(Point,4326)"},
			ColumnDDL:       "{{schema}}.{{p}}_d",
			InitialValue:    "'SRID=4326;POINT(-71.060316 42.358431)'::geometry(Point,4326)",
			AltValue:        "'SRID=4326;POINT(2 3)'::geometry(Point,4326)",
			RecordDestValue: true,
			Note:            "domain over a typmod-constrained geometry",
		},
		{
			ID: "GEO-007", Name: "box2d", TypeName: "box2d",
			Extensions:   []string{"postgis"},
			ColumnDDL:    "box2d",
			InitialValue: "'BOX(0 0,1 1)'::box2d",
			AltValue:     "'BOX(2 2,3 3)'::box2d",
		},
		{
			ID: "GEO-008", Name: "raster", TypeName: "raster",
			// postgis_raster is a separate extension; if the image lacks it this probe
			// self-reports SKIPPED rather than failing the batch.
			Extensions:   []string{"postgis", "postgis_raster"},
			ColumnDDL:    "raster",
			InitialValue: "ST_AddBand(ST_MakeEmptyRaster(2, 2, 0, 0, 1), '8BUI'::text, 1, 0)",
			AltValue:     "ST_AddBand(ST_MakeEmptyRaster(1, 1, 0, 0, 1), '8BUI'::text, 5, 0)",
		},
		{
			ID: "GEO-009", Name: "box3d", TypeName: "box3d",
			Extensions:   []string{"postgis"},
			ColumnDDL:    "box3d",
			InitialValue: "'BOX3D(0 0 0,1 1 1)'::box3d",
			AltValue:     "'BOX3D(2 2 2,3 3 3)'::box3d",
			Note:         "named in voyager's unsupported list and never probed; contrast with box2d (GEO-007)",
		},
		{
			ID: "GEO-010", Name: "topogeometry", TypeName: "topology.topogeometry",
			Extensions: []string{"postgis", "postgis_topology"},
			// toTopoGeom needs a topology and a registered layer to exist first. The
			// layer is registered against a throwaway table so that it does not depend
			// on the probe table, which is created after PreDDL has run.
			// The helper table sits in its own unexported schema so voyager never picks
			// it up as a table to migrate.
			PreDDL: []string{
				"SELECT topology.CreateTopology('{{p}}_topo')",
				"CREATE SCHEMA IF NOT EXISTS {{p}}_aux",
				"CREATE TABLE {{p}}_aux.lyr (id int PRIMARY KEY)",
				"SELECT topology.AddTopoGeometryColumn('{{p}}_topo', '{{p}}_aux', 'lyr', 'g', 'POINT')",
			},
			ColumnDDL:       "topology.topogeometry",
			InitialValue:    "topology.toTopoGeom('POINT(1 1)'::geometry, '{{p}}_topo', 1)",
			AltValue:        "topology.toTopoGeom('POINT(2 2)'::geometry, '{{p}}_topo', 1)",
			RecordDestValue: true,
			Note: "named in voyager's unsupported list and never probed; the value is a tuple of " +
				"(topology_id, layer_id, id, type), meaningless without the topology schema that " +
				"holds the actual geometry",
		},
	}
}

// arrayDelimiterProbes target a specific, easy-to-miss corruption vector.
//
// geometry and geography carry pg_type.typdelim = ':', not the usual ','. So the TEXT
// form of a geometry array is colon-delimited:
//
//	'{"POINT(3 4)":"SRID=4326;POINT(5 6)"}'::geometry[]
//
// and the comma form is rejected outright with "malformed array literal". Voyager's
// approach to arrays is to stringify them and trust the text form to round-trip, so a
// non-comma delimiter is exactly the shape that corrupts quietly. Both probes here set
// RecordDestValue so the PROBE-RESULT detail carries the actual destination text,
// letting the report say whether the delimiter survived rather than just pass/fail.
func arrayDelimiterProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "ARRAY-DELIM-001", Name: "geometry[] via colon-delimited text literal",
			TypeName:        "geometry[] (typdelim ':')",
			Extensions:      []string{"postgis"},
			ColumnDDL:       "geometry[]",
			InitialValue:    `'{"POINT(3 4)":"SRID=4326;POINT(5 6)"}'::geometry[]`,
			AltValue:        `'{"POINT(1 1)":"POINT(2 2)"}'::geometry[]`,
			RecordDestValue: true,
			Note:            "geometry typdelim is ':' - the comma form raises malformed array literal",
		},
		{
			ID: "ARRAY-DELIM-002", Name: "box2d[] contrast case (comma-delimited)",
			TypeName:        "box2d[] (typdelim ',')",
			Extensions:      []string{"postgis"},
			ColumnDDL:       "box2d[]",
			InitialValue:    "ARRAY['BOX(0 0,1 1)'::box2d, 'BOX(2 2,3 3)'::box2d]",
			AltValue:        "ARRAY['BOX(4 4,5 5)'::box2d]",
			RecordDestValue: true,
			Note:            "comma-delimited contrast for ARRAY-DELIM-001; elements themselves contain commas",
		},
	}
}

// ============================================================
// PGVECTOR
// ============================================================

func pgvectorProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "VEC-001", Name: "vector(3)", TypeName: "vector(3)",
			Extensions:     []string{"vector"},
			ColumnDDL:      "vector(3)",
			InitialValue:   "'[1,2,3]'::vector(3)",
			AltValue:       "'[4,5,6]'::vector(3)",
			ExpectExcluded: true,
			Note:           "scalar form: the shape the guardrail is written for",
		},
		{
			ID: "VEC-002", Name: "vector array", TypeName: "vector[]",
			Extensions:      []string{"vector"},
			ColumnDDL:       "vector[]",
			InitialValue:    "ARRAY['[1,2,3]'::vector, '[4,5,6]'::vector]",
			AltValue:        "ARRAY['[7,8,9]'::vector]",
			RecordDestValue: true,
			Note:            "KEY CASE: does the array form bypass the name-equality guardrail?",
		},
		{
			ID: "VEC-003", Name: "domain over vector(3)", TypeName: "domain(vector(3))",
			Extensions:      []string{"vector"},
			PreDDL:          []string{"CREATE DOMAIN {{schema}}.{{p}}_d AS vector(3)"},
			ColumnDDL:       "{{schema}}.{{p}}_d",
			InitialValue:    "'[0.1,0.2,0.3]'::vector(3)",
			AltValue:        "'[0.4,0.5,0.6]'::vector(3)",
			RecordDestValue: true,
			Note:            "KEY CASE: does the domain form bypass the name-equality guardrail?",
		},
		{
			ID: "VEC-004", Name: "halfvec(3)", TypeName: "halfvec(3)",
			Extensions:      []string{"vector"},
			ColumnDDL:       "halfvec(3)",
			InitialValue:    "'[1,2,3]'::halfvec(3)",
			AltValue:        "'[4,5,6]'::halfvec(3)",
			RecordDestValue: true,
			Note: "pgvector 0.8 half-precision vector; pgvector is available on YugabyteDB, " +
				"so unlike postgis this is a live target concern rather than an offline-only one",
		},
		{
			ID: "VEC-005", Name: "sparsevec(5)", TypeName: "sparsevec(5)",
			Extensions:      []string{"vector"},
			ColumnDDL:       "sparsevec(5)",
			InitialValue:    "'{1:1,3:2}/5'::sparsevec(5)",
			AltValue:        "'{2:7}/5'::sparsevec(5)",
			RecordDestValue: true,
			Note:            "pgvector 0.8 sparse vector; the text form carries both the index:value pairs and the dimension",
		},
	}
}

// ============================================================
// CORE SCALARS
// ============================================================

// coreScalarProbes cover the everyday base types. They were left out of the first pass
// on the assumption that "obviously supported" needs no evidence; a catalogue diff
// against PG 17 showed they had simply never been measured. Each gets its own probe so
// the audit matrix has one row per type rather than one row per "common scalars" bundle.
func coreScalarProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "CORE-001", Name: "bool", TypeName: "bool",
			ColumnDDL: "bool", InitialValue: "true", AltValue: "false",
		},
		{
			ID: "CORE-002", Name: "int2", TypeName: "int2 (smallint)",
			ColumnDDL: "int2", InitialValue: "32767::int2", AltValue: "(-32768)::int2",
			Note: "both endpoints of the signed 16-bit range",
		},
		{
			ID: "CORE-003", Name: "int8", TypeName: "int8 (bigint)",
			ColumnDDL: "int8", InitialValue: "9223372036854775807::int8", AltValue: "(-9223372036854775808)::int8",
			Note: "both endpoints of the signed 64-bit range",
		},
		{
			ID: "CORE-004", Name: "float4", TypeName: "float4 (real)",
			ColumnDDL: "float4", InitialValue: "3.25::float4", AltValue: "(-1.5)::float4",
			Note: "exactly-representable binary fractions, so the text form cannot drift on rounding",
		},
		{
			ID: "CORE-005", Name: "numeric without typmod", TypeName: "numeric",
			ColumnDDL:    "numeric",
			InitialValue: "'12345678901234567890.123456789'::numeric",
			AltValue:     "'-0.000001'::numeric",
			Note:         "unconstrained numeric: no typmod for the connector to read a scale from",
		},
		{
			ID: "CORE-006", Name: "uuid", TypeName: "uuid",
			ColumnDDL:    "uuid",
			InitialValue: "'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'::uuid",
			AltValue:     "'00000000-0000-0000-0000-000000000001'::uuid",
		},
		{
			ID: "CORE-007", Name: "json", TypeName: "json",
			ColumnDDL:    "json",
			InitialValue: `'{"k":[1,2]}'::json`, AltValue: `'{"z":true,"a":null}'::json`,
			Note: "json preserves the input text verbatim, unlike jsonb",
		},
		{
			ID: "CORE-008", Name: "jsonb", TypeName: "jsonb",
			ColumnDDL:    "jsonb",
			InitialValue: `'{"k":[1,2]}'::jsonb`, AltValue: `'{"z":true,"a":null}'::jsonb`,
		},
		{
			ID: "CORE-009", Name: "varchar(50)", TypeName: "varchar(50)",
			ColumnDDL:    "varchar(50)",
			InitialValue: "'hello varchar'::varchar(50)", AltValue: "'other value'::varchar(50)",
		},
		{
			ID: "CORE-010", Name: "char(10)", TypeName: "bpchar / char(10)",
			ColumnDDL:    "char(10)",
			InitialValue: "'abc'::char(10)", AltValue: "'xyz'::char(10)",
			Note: "blank-padded: the stored value is 10 chars wide, and the padding must survive",
		},
		{
			ID: "CORE-011", Name: `"char"`, TypeName: `"char" (1-byte internal)`,
			ColumnDDL:    `"char"`,
			InitialValue: `'a'::"char"`, AltValue: `'Z'::"char"`,
			Note: `the single-byte internal type, not char(1)`,
		},
		{
			ID: "CORE-012", Name: "name", TypeName: "name",
			ColumnDDL:    "name",
			InitialValue: "'some_identifier'::name", AltValue: "'other_identifier'::name",
		},
		{
			ID: "CORE-013", Name: "macaddr", TypeName: "macaddr",
			ColumnDDL:    "macaddr",
			InitialValue: "'08:00:2b:01:02:03'::macaddr", AltValue: "'01:02:03:04:05:06'::macaddr",
			Note: "the 6-byte form; MISC-006 covers macaddr8",
		},
		{
			ID: "CORE-014", Name: "bytea", TypeName: "bytea",
			ColumnDDL:    "bytea",
			InitialValue: `'\xdeadbeef00'::bytea`, AltValue: `'\x0102'::bytea`,
			Note: "plain binary; VAL-020 covers the embedded-NUL edge case",
		},
		{
			ID: "CORE-015", Name: "date", TypeName: "date",
			ColumnDDL:    "date",
			InitialValue: "'2024-02-29'::date", AltValue: "'1999-12-31'::date",
			Note: "a leap day, so an off-by-one day conversion cannot hide",
		},
		{
			ID: "CORE-016", Name: "time", TypeName: "time",
			ColumnDDL:    "time",
			InitialValue: "'12:34:56.789012'::time", AltValue: "'23:59:59'::time",
			Note: "microsecond precision must survive",
		},
		{
			ID: "CORE-017", Name: "timestamp", TypeName: "timestamp",
			ColumnDDL:    "timestamp",
			InitialValue: "'2024-01-15 12:34:56.789012'::timestamp",
			AltValue:     "'1970-01-01 00:00:00'::timestamp",
		},
		{
			ID: "CORE-018", Name: "timestamptz", TypeName: "timestamptz",
			ColumnDDL:    "timestamptz",
			InitialValue: "'2024-01-15 12:34:56.789012+05:30'::timestamptz",
			AltValue:     "'2000-01-01 00:00:00+00'::timestamptz",
			// The default v::text renders in the session TimeZone, which need not match
			// between the PG source and the YB target. Normalising to UTC keeps a
			// GUC difference from masquerading as a datatype finding.
			CompareExpr: "(v AT TIME ZONE 'UTC')::text",
			Note:        "compared normalised to UTC because the text form is TimeZone dependent",
		},
		{
			ID: "CORE-019", Name: "interval", TypeName: "interval",
			ColumnDDL:    "interval",
			InitialValue: "'1 year 2 mons 3 days 04:05:06.789'::interval",
			AltValue:     "'-15 days'::interval",
			Note:         "all three interval fields (months, days, microseconds) populated at once",
		},
	}
}

// ============================================================
// BUILT-IN GEOMETRIC SCALARS
// ============================================================

// geometricScalarProbes cover PostgreSQL's own geometric types. Only their array and
// domain forms had been probed before, via the array/domain batches, so the plain scalar
// column - by far the common shape in real schemas - was untested.
//
// Note these are the core types (pg_catalog point/line/box/...), not PostGIS geometry;
// the postgis batch covers that separately and needs the -ext image.
func geometricScalarProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "GEO2-001", Name: "point", TypeName: "point",
			ColumnDDL:    "point",
			InitialValue: "'(1.5,2.5)'::point", AltValue: "'(3,4)'::point",
		},
		{
			ID: "GEO2-002", Name: "line", TypeName: "line",
			ColumnDDL:    "line",
			InitialValue: "'{1,-1,0}'::line", AltValue: "'{0,1,-5}'::line",
			Note: "infinite line, stored as the three coefficients A,B,C",
		},
		{
			ID: "GEO2-003", Name: "lseg", TypeName: "lseg",
			ColumnDDL:    "lseg",
			InitialValue: "'[(0,0),(1,1)]'::lseg", AltValue: "'[(2,2),(3,3)]'::lseg",
		},
		{
			ID: "GEO2-004", Name: "box", TypeName: "box",
			ColumnDDL:    "box",
			InitialValue: "'((0,0),(1,1))'::box", AltValue: "'((2,2),(3,3))'::box",
			Note: "box normalises its corners on input, so the stored text need not match the literal",
		},
		{
			ID: "GEO2-005", Name: "path", TypeName: "path",
			ColumnDDL:    "path",
			InitialValue: "'[(0,0),(1,1),(2,0)]'::path", AltValue: "'((0,0),(1,1),(2,0))'::path",
			Note: "open path vs closed path: the bracket style is part of the value, not formatting",
		},
		{
			ID: "GEO2-006", Name: "polygon", TypeName: "polygon",
			ColumnDDL:    "polygon",
			InitialValue: "'((0,0),(1,1),(2,0))'::polygon", AltValue: "'((0,0),(5,5),(5,0))'::polygon",
		},
		{
			ID: "GEO2-007", Name: "circle", TypeName: "circle",
			ColumnDDL:    "circle",
			InitialValue: "'<(1,1),5>'::circle", AltValue: "'<(0,0),2>'::circle",
		},
	}
}

// ============================================================
// OBJECT-IDENTIFIER (reg*) FAMILY
// ============================================================

// regTypeProbes cover the OID-alias family. Only regclass had been probed, and it is
// established poison (SYS-004): its value travels as text and is re-resolved against the
// DESTINATION catalog, which is a different catalog. Every type here shares that
// mechanism, so the interesting question is whether they all share the outcome.
//
// Literals are chosen to be unambiguous and to name objects that exist in a stock
// PostgreSQL and in YugabyteDB's YSQL catalog, so a failure is about the transport
// rather than about the object being absent on the target.
func regTypeProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "REG-001", Name: "regtype", TypeName: "regtype",
			ColumnDDL:    "regtype",
			Poison:       true,
			PoisonNote:   "POISON: the reg* family travels as raw bytes and is re-resolved against the DESTINATION catalog. Established in the LIVE regtypes batch, which came out PROBE-RUN-INVALID: the importer reported ERROR: function 0x76657273696f6e does not exist (SQLSTATE 42883), and 76657273696f6e is the hex of the regproc value version. Same chain as regclass (SYS-004) and tid (SYS-003). Must be run solo.",
			InitialValue: "'pg_catalog.int4'::regtype", AltValue: "'pg_catalog.text'::regtype",
		},
		{
			ID: "REG-002", Name: "regproc", TypeName: "regproc",
			ColumnDDL:    "regproc",
			Poison:       true,
			PoisonNote:   "POISON: the reg* family travels as raw bytes and is re-resolved against the DESTINATION catalog. Established in the LIVE regtypes batch, which came out PROBE-RUN-INVALID: the importer reported ERROR: function 0x76657273696f6e does not exist (SQLSTATE 42883), and 76657273696f6e is the hex of the regproc value version. Same chain as regclass (SYS-004) and tid (SYS-003). Must be run solo.",
			InitialValue: "'now'::regproc", AltValue: "'version'::regproc",
			Note: "both names have exactly one function, so the bare-name form is unambiguous",
		},
		{
			ID: "REG-003", Name: "regprocedure", TypeName: "regprocedure",
			ColumnDDL:    "regprocedure",
			Poison:       true,
			PoisonNote:   "POISON: the reg* family travels as raw bytes and is re-resolved against the DESTINATION catalog. Established in the LIVE regtypes batch, which came out PROBE-RUN-INVALID: the importer reported ERROR: function 0x76657273696f6e does not exist (SQLSTATE 42883), and 76657273696f6e is the hex of the regproc value version. Same chain as regclass (SYS-004) and tid (SYS-003). Must be run solo.",
			InitialValue: "'pg_catalog.abs(int4)'::regprocedure",
			AltValue:     "'pg_catalog.upper(text)'::regprocedure",
			Note:         "argument-typed form; carries the signature, not just the name",
		},
		{
			ID: "REG-004", Name: "regoper", TypeName: "regoper",
			ColumnDDL:    "regoper",
			Poison:       true,
			PoisonNote:   "POISON: the reg* family travels as raw bytes and is re-resolved against the DESTINATION catalog. Established in the LIVE regtypes batch, which came out PROBE-RUN-INVALID: the importer reported ERROR: function 0x76657273696f6e does not exist (SQLSTATE 42883), and 76657273696f6e is the hex of the regproc value version. Same chain as regclass (SYS-004) and tid (SYS-003). Must be run solo.",
			InitialValue: "'pg_catalog.|/'::regoper", AltValue: "'pg_catalog.||/'::regoper",
			Note: "prefix sqrt and cbrt: each has exactly one operator, so the bare form resolves",
		},
		{
			ID: "REG-005", Name: "regoperator", TypeName: "regoperator",
			ColumnDDL:    "regoperator",
			Poison:       true,
			PoisonNote:   "POISON: the reg* family travels as raw bytes and is re-resolved against the DESTINATION catalog. Established in the LIVE regtypes batch, which came out PROBE-RUN-INVALID: the importer reported ERROR: function 0x76657273696f6e does not exist (SQLSTATE 42883), and 76657273696f6e is the hex of the regproc value version. Same chain as regclass (SYS-004) and tid (SYS-003). Must be run solo.",
			InitialValue: "'pg_catalog.=(integer,integer)'::regoperator",
			AltValue:     "'pg_catalog.+(integer,integer)'::regoperator",
		},
		{
			ID: "REG-006", Name: "regconfig", TypeName: "regconfig",
			ColumnDDL:    "regconfig",
			Poison:       true,
			PoisonNote:   "POISON: the reg* family travels as raw bytes and is re-resolved against the DESTINATION catalog. Established in the LIVE regtypes batch, which came out PROBE-RUN-INVALID: the importer reported ERROR: function 0x76657273696f6e does not exist (SQLSTATE 42883), and 76657273696f6e is the hex of the regproc value version. Same chain as regclass (SYS-004) and tid (SYS-003). Must be run solo.",
			InitialValue: "'pg_catalog.english'::regconfig", AltValue: "'pg_catalog.simple'::regconfig",
			Note: "text-search configuration reference",
		},
		{
			ID: "REG-007", Name: "regdictionary", TypeName: "regdictionary",
			ColumnDDL:    "regdictionary",
			Poison:       true,
			PoisonNote:   "POISON: the reg* family travels as raw bytes and is re-resolved against the DESTINATION catalog. Established in the LIVE regtypes batch, which came out PROBE-RUN-INVALID: the importer reported ERROR: function 0x76657273696f6e does not exist (SQLSTATE 42883), and 76657273696f6e is the hex of the regproc value version. Same chain as regclass (SYS-004) and tid (SYS-003). Must be run solo.",
			InitialValue: "'pg_catalog.english_stem'::regdictionary",
			AltValue:     "'pg_catalog.simple'::regdictionary",
		},
		{
			ID: "REG-008", Name: "regnamespace", TypeName: "regnamespace",
			ColumnDDL:    "regnamespace",
			Poison:       true,
			PoisonNote:   "POISON: the reg* family travels as raw bytes and is re-resolved against the DESTINATION catalog. Established in the LIVE regtypes batch, which came out PROBE-RUN-INVALID: the importer reported ERROR: function 0x76657273696f6e does not exist (SQLSTATE 42883), and 76657273696f6e is the hex of the regproc value version. Same chain as regclass (SYS-004) and tid (SYS-003). Must be run solo.",
			InitialValue: "'pg_catalog'::regnamespace", AltValue: "'public'::regnamespace",
		},
		{
			ID: "REG-009", Name: "regrole", TypeName: "regrole",
			ColumnDDL:    "regrole",
			Poison:       true,
			PoisonNote:   "POISON: the reg* family travels as raw bytes and is re-resolved against the DESTINATION catalog. Established in the LIVE regtypes batch, which came out PROBE-RUN-INVALID: the importer reported ERROR: function 0x76657273696f6e does not exist (SQLSTATE 42883), and 76657273696f6e is the hex of the regproc value version. Same chain as regclass (SYS-004) and tid (SYS-003). Must be run solo.",
			InitialValue: "'pg_read_all_stats'::regrole", AltValue: "'pg_monitor'::regrole",
			Note: "built-in default roles, so the value resolves on source and target alike",
		},
		{
			ID: "REG-010", Name: "regcollation", TypeName: "regcollation",
			ColumnDDL:    "regcollation",
			Poison:       true,
			PoisonNote:   "POISON: the reg* family travels as raw bytes and is re-resolved against the DESTINATION catalog. Established in the LIVE regtypes batch, which came out PROBE-RUN-INVALID: the importer reported ERROR: function 0x76657273696f6e does not exist (SQLSTATE 42883), and 76657273696f6e is the hex of the regproc value version. Same chain as regclass (SYS-004) and tid (SYS-003). Must be run solo.",
			InitialValue: `'pg_catalog."C"'::regcollation`, AltValue: `'pg_catalog."POSIX"'::regcollation`,
		},
	}
}

// ============================================================
// REMAINING CATALOGUE TYPES
// ============================================================

// catalogTypeProbes are the leftovers from the PG 17 catalogue diff that are still
// user-facing (a user can declare a column of this type and put a value in it) but do
// not belong to any of the families above.
func catalogTypeProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "CAT-001", Name: "cid", TypeName: "cid",
			ColumnDDL:    "cid",
			InitialValue: "'42'::cid", AltValue: "'7'::cid",
			Note: "command id; the sibling of xid (SYS-001) and tid (SYS-003, poison)",
		},
		{
			ID: "CAT-002", Name: "oidvector", TypeName: "oidvector",
			ColumnDDL:    "oidvector",
			InitialValue: "'1 2 3'::oidvector", AltValue: "'25 23'::oidvector",
			Note: "space-delimited vector, like int2vector (SYS-006) but of oids",
		},
		{
			ID: "CAT-003", Name: "refcursor", TypeName: "refcursor",
			ColumnDDL:    "refcursor",
			InitialValue: "'mycursor'::refcursor", AltValue: "'other_cursor'::refcursor",
			Note: "storable as a plain cursor name; the portal it names is not migrated, only the string",
		},
	}
}

// ============================================================
// CATALOGUE / STATISTICS TYPES
// ============================================================

// catalogStatsProbes cover the types PostgreSQL uses for ACLs and for extended
// statistics. They were originally written off as "internal", which was wrong: a user
// can declare a column of every one of them, and five of the seven can hold a real
// value. The rule this batch encodes is that a type is only excluded when a column of
// that type cannot be created AT ALL, demonstrated at run time with the error quoted -
// never assumed from what the type looks like or what it is normally used for.
//
// pg_node_tree, pg_ndistinct, pg_dependencies and pg_mcv_list all refuse a literal
// ("cannot accept a value of type <t>"), so their values are selected out of the
// catalogue instead - which is exactly how a real schema would ever come to hold one.
// PreDDL builds the catalogue rows those selects read.
func catalogStatsProbes() []datatypeProbe {
	// One helper table carries three statistics objects, so each stats probe has two
	// genuinely different values to move between without three separate source tables.
	// b is a function of a and e is a function of d, so x1 and x3 record real functional
	// dependencies; x2 (a,c) records none, which is why it is not used for
	// pg_dependencies.
	//
	// The helper lives in its own {{p}}_aux schema, NOT in the sweep schema. PreDDL runs
	// on every participating database, so a seeded helper table inside the exported
	// schema would exist on the target before import and then have the same rows COPYed
	// into it - which is exactly how the first attempt failed, with
	// `ERROR: duplicate key value violates unique constraint "p_catstat_003_s_pkey"
	// (SQLSTATE 23505)` taking the controls down with it. An unexported schema keeps the
	// helper out of voyager's table list entirely.
	statsPreDDL := []string{
		"CREATE SCHEMA IF NOT EXISTS {{p}}_aux",
		"CREATE TABLE {{p}}_aux.s (id int PRIMARY KEY, a int, b int, c int, d int, e int)",
		"INSERT INTO {{p}}_aux.s SELECT i, i%5, (i%5)*2, i%9, i%3, (i%3)*4 FROM generate_series(1,1000) i",
		"CREATE STATISTICS {{p}}_aux.x1 (ndistinct, dependencies, mcv) ON a, b FROM {{p}}_aux.s",
		"CREATE STATISTICS {{p}}_aux.x2 (ndistinct, dependencies, mcv) ON a, c FROM {{p}}_aux.s",
		"CREATE STATISTICS {{p}}_aux.x3 (ndistinct, dependencies, mcv) ON d, e FROM {{p}}_aux.s",
		"ANALYZE {{p}}_aux.s",
	}
	// statExpr reads one statistics column back out of the catalogue by object name.
	// The statistics objects live in the {{p}}_aux schema and are named x1/x2/x3 there,
	// so the lookup must match on schema AND name - matching on name alone silently
	// returns no row, and the probe then measures a NULL while still reporting WORKS.
	statExpr := func(col, obj string) string {
		return "(SELECT sd." + col + " FROM pg_statistic_ext se " +
			"JOIN pg_statistic_ext_data sd ON sd.stxoid = se.oid " +
			"JOIN pg_namespace sn ON sn.oid = se.stxnamespace " +
			"WHERE sn.nspname = '{{p}}_aux' AND se.stxname = '" + obj + "' " +
			"AND sd." + col + " IS NOT NULL LIMIT 1)"
	}

	return []datatypeProbe{
		{
			ID: "CATSTAT-001", Name: "aclitem", TypeName: "aclitem",
			ColumnDDL:    "aclitem",
			InitialValue: "'pg_monitor=arwdDxt/pg_monitor'::aclitem",
			AltValue:     "'pg_read_all_stats=r/pg_monitor'::aclitem",
			Note: "grantee and grantor are built-in default roles, so the value resolves " +
				"on source and target alike rather than depending on the container's superuser name",
		},
		{
			ID: "CATSTAT-002", Name: "pg_node_tree", TypeName: "pg_node_tree",
			PreDDL: []string{
				"CREATE SCHEMA IF NOT EXISTS {{p}}_aux",
				"CREATE TABLE {{p}}_aux.s (id int PRIMARY KEY, a int DEFAULT 42, b int DEFAULT 7)",
			},
			ColumnDDL: "pg_node_tree",
			InitialValue: "(SELECT ad.adbin FROM pg_attrdef ad JOIN pg_class cl ON cl.oid = ad.adrelid " +
				"JOIN pg_namespace ns ON ns.oid = cl.relnamespace " +
				"WHERE ns.nspname = '{{p}}_aux' AND cl.relname = 's' AND ad.adnum = 2)",
			AltValue: "(SELECT ad.adbin FROM pg_attrdef ad JOIN pg_class cl ON cl.oid = ad.adrelid " +
				"JOIN pg_namespace ns ON ns.oid = cl.relnamespace " +
				"WHERE ns.nspname = '{{p}}_aux' AND cl.relname = 's' AND ad.adnum = 3)",
			RecordDestValue: true,
			Note: "no literal is accepted (PG 17.8: `ERROR: cannot accept a value of type pg_node_tree`), " +
				"so the value is the serialised default expression of a real column, read out of pg_attrdef",
		},
		{
			ID: "CATSTAT-003", Name: "pg_ndistinct", TypeName: "pg_ndistinct",
			PreDDL:          statsPreDDL,
			ColumnDDL:       "pg_ndistinct",
			InitialValue:    statExpr("stxdndistinct", "x1"),
			AltValue:        statExpr("stxdndistinct", "x2"),
			RecordDestValue: true,
			Note: "no literal is accepted (PG 17.8: `ERROR: cannot accept a value of type pg_ndistinct`), " +
				"so the value comes from pg_statistic_ext_data",
		},
		{
			ID: "CATSTAT-004", Name: "pg_dependencies", TypeName: "pg_dependencies",
			PreDDL:          statsPreDDL,
			ColumnDDL:       "pg_dependencies",
			InitialValue:    statExpr("stxddependencies", "x1"),
			AltValue:        statExpr("stxddependencies", "x3"),
			RecordDestValue: true,
			Note: "no literal is accepted (PG 17.8: `ERROR: cannot accept a value of type pg_dependencies`); " +
				"x1 and x3 are the two statistics objects with a real functional dependency, so both values are non-NULL",
		},
		{
			ID: "CATSTAT-005", Name: "pg_mcv_list", TypeName: "pg_mcv_list",
			PreDDL:          statsPreDDL,
			ColumnDDL:       "pg_mcv_list",
			InitialValue:    statExpr("stxdmcv", "x1"),
			AltValue:        statExpr("stxdmcv", "x2"),
			RecordDestValue: true,
			Note:            "no literal is accepted; the value is a real MCV list read out of pg_statistic_ext_data",
		},
		{
			ID: "CATSTAT-006", Name: "pg_brin_bloom_summary", TypeName: "pg_brin_bloom_summary",
			ColumnDDL:    "pg_brin_bloom_summary",
			NullOnly:     true,
			InitialValue: "NULL", AltValue: "NULL",
			Note: "NULL-only: the column is creatable and NULL is storable, but no value is - " +
				"PG 17.8 answers `ERROR: cannot accept a value of type pg_brin_bloom_summary`, and a real " +
				"summary exists only inside a BRIN index page, unreachable from SQL",
		},
		{
			ID: "CATSTAT-007", Name: "pg_brin_minmax_multi_summary", TypeName: "pg_brin_minmax_multi_summary",
			ColumnDDL:    "pg_brin_minmax_multi_summary",
			NullOnly:     true,
			InitialValue: "NULL", AltValue: "NULL",
			Note: "NULL-only: PG 17.8 answers `ERROR: cannot accept a value of type brin_minmax_multi_summary` " +
				"(note the error names the type without the pg_ prefix)",
		},
	}
}

// ============================================================
// INDEX-SUPPORT KEY TYPES AND OTHER CONTRIB COMPOSITES
// ============================================================

// indexKeyTypeProbes cover the GiST/GIN key types and the tablefunc crosstab
// composites. Most of them refuse every literal and can therefore only ever hold NULL -
// but "only NULL is storable" is not the same as "not migratable": the type still has to
// exist on the target, the column still has to survive the snapshot, and the rows still
// have to travel through CDC. Each NULL-only probe records the exact PG 17.8 error that
// established it, so the exclusion is demonstrated rather than assumed.
//
// query_int and the three crosstab composites do store real values and are probed with
// them.
func indexKeyTypeProbes() []datatypeProbe {
	nullOnly := func(id, name, ddl, ext, errText string) datatypeProbe {
		p := datatypeProbe{
			ID: id, Name: name, TypeName: name,
			ColumnDDL:    ddl,
			NullOnly:     true,
			InitialValue: "NULL", AltValue: "NULL",
			Note: "NULL-only: the column is creatable and NULL is storable, but PG 17.8 " +
				"refuses every literal with `ERROR: " + errText + "`",
		}
		if ext != "" {
			p.Extensions = []string{ext}
		}
		return p
	}

	return []datatypeProbe{
		nullOnly("IDXKEY-001", "gtsvector", "gtsvector", "",
			"cannot accept a value of type gtsvector"),
		nullOnly("IDXKEY-002", "ghstore", "ghstore", "hstore",
			"cannot accept a value of type ghstore"),
		nullOnly("IDXKEY-003", "gtrgm", "gtrgm", "pg_trgm",
			"cannot accept a value of type gtrgm"),
		nullOnly("IDXKEY-004", "gbtreekey16", "gbtreekey16", "btree_gist",
			"cannot accept a value of type gbtreekey16"),
		nullOnly("IDXKEY-005", "gbtreekey2", "gbtreekey2", "btree_gist",
			"cannot accept a value of type gbtreekey2"),
		nullOnly("IDXKEY-006", "gbtreekey32", "gbtreekey32", "btree_gist",
			"cannot accept a value of type gbtreekey32"),
		nullOnly("IDXKEY-007", "gbtreekey4", "gbtreekey4", "btree_gist",
			"cannot accept a value of type gbtreekey4"),
		nullOnly("IDXKEY-008", "gbtreekey8", "gbtreekey8", "btree_gist",
			"cannot accept a value of type gbtreekey8"),
		nullOnly("IDXKEY-009", "gbtreekey_var", "gbtreekey_var", "btree_gist",
			"cannot accept a value of type gbtreekey_var"),
		nullOnly("IDXKEY-010", "ltree_gist", "ltree_gist", "ltree",
			"cannot accept a value of type ltree_gist"),
		nullOnly("IDXKEY-011", "intbig_gkey", "intbig_gkey", "intarray",
			"cannot accept a value of type intbig_gkey"),
		{
			ID: "IDXKEY-012", Name: "query_int", TypeName: "query_int",
			Extensions:   []string{"intarray"},
			ColumnDDL:    "query_int",
			InitialValue: "'1&2'::query_int", AltValue: "'3|4'::query_int",
			Note: "does store a real value; PG normalises '1&2' to '1 & 2' on output",
		},
		{
			ID: "IDXKEY-013", Name: "tablefunc_crosstab_2", TypeName: "tablefunc_crosstab_2",
			Extensions:   []string{"tablefunc"},
			ColumnDDL:    "tablefunc_crosstab_2",
			InitialValue: "ROW('r','c1','c2')::tablefunc_crosstab_2",
			AltValue:     "ROW('r2','x','y')::tablefunc_crosstab_2",
			Note:         "an extension-owned composite type, not a user-defined one",
		},
		{
			ID: "IDXKEY-014", Name: "tablefunc_crosstab_3", TypeName: "tablefunc_crosstab_3",
			Extensions:   []string{"tablefunc"},
			ColumnDDL:    "tablefunc_crosstab_3",
			InitialValue: "ROW('r','c1','c2','c3')::tablefunc_crosstab_3",
			AltValue:     "ROW('r2','x','y','z')::tablefunc_crosstab_3",
		},
		{
			ID: "IDXKEY-015", Name: "tablefunc_crosstab_4", TypeName: "tablefunc_crosstab_4",
			Extensions:   []string{"tablefunc"},
			ColumnDDL:    "tablefunc_crosstab_4",
			InitialValue: "ROW('r','c1','c2','c3','c4')::tablefunc_crosstab_4",
			AltValue:     "ROW('r2','w','x','y','z')::tablefunc_crosstab_4",
		},
	}
}

// ============================================================
// CONTRIB EXTENSION TYPES
// ============================================================

// extensionTypeProbes cover contrib types that ship with PostgreSQL but had never been
// probed. Every literal here was verified storable on PG 17.8 first, so a SKIPPED
// verdict from one of these means the extension or the type is genuinely unavailable on
// a participating database, not that the case table guessed wrong.
func extensionTypeProbes() []datatypeProbe {
	return []datatypeProbe{
		{
			ID: "EXT-001", Name: "cube", TypeName: "cube",
			Extensions:   []string{"cube"},
			ColumnDDL:    "cube",
			InitialValue: "'(1,2),(3,4)'::cube", AltValue: "'(0,0,0)'::cube",
			Note: "a 2-D interval cube and a 3-D point cube: dimensionality is part of the value",
		},
		{
			ID: "EXT-002", Name: "seg", TypeName: "seg",
			Extensions:   []string{"seg"},
			ColumnDDL:    "seg",
			InitialValue: "'1.5 .. 2.5'::seg", AltValue: "'3.0 .. 4.0'::seg",
		},
		{
			ID: "EXT-003", Name: "isbn13", TypeName: "isbn13",
			Extensions:   []string{"isn"},
			ColumnDDL:    "isbn13",
			InitialValue: "'978-0-393-04002-9'::isbn13", AltValue: "'978-0-13-235088-4'::isbn13",
		},
		{
			ID: "EXT-004", Name: "ean13", TypeName: "ean13",
			Extensions:   []string{"isn"},
			ColumnDDL:    "ean13",
			InitialValue: "'978-0-393-04002-9'::ean13", AltValue: "'978-0-13-235088-4'::ean13",
		},
		{
			ID: "EXT-005", Name: "isbn", TypeName: "isbn",
			Extensions:   []string{"isn"},
			ColumnDDL:    "isbn",
			InitialValue: "'0-393-04002-X'::isbn", AltValue: "'0-8044-2957-X'::isbn",
			Note: "check digit X: the value is not purely numeric",
		},
		{
			ID: "EXT-006", Name: "ismn", TypeName: "ismn",
			Extensions:   []string{"isn"},
			ColumnDDL:    "ismn",
			InitialValue: "'M-345-24236-4'::ismn", AltValue: "'M-2306-7118-7'::ismn",
			Note: "input is re-hyphenated on output (M-345-24236-4 stores as M-3452-4236-4)",
		},
		{
			ID: "EXT-007", Name: "ismn13", TypeName: "ismn13",
			Extensions:   []string{"isn"},
			ColumnDDL:    "ismn13",
			InitialValue: "'979-0-345-24236-4'::ismn13", AltValue: "'M-2306-7118-7'::ismn13",
		},
		{
			ID: "EXT-008", Name: "issn", TypeName: "issn",
			Extensions:   []string{"isn"},
			ColumnDDL:    "issn",
			InitialValue: "'1436-4522'::issn", AltValue: "'0264-2875'::issn",
		},
		{
			ID: "EXT-009", Name: "issn13", TypeName: "issn13",
			Extensions:   []string{"isn"},
			ColumnDDL:    "issn13",
			InitialValue: "'1436-4522'::issn13", AltValue: "'0264-2875'::issn13",
			Note: "the 8-digit ISSN is widened on input (1436-4522 stores as 977-1436-452-00-8)",
		},
		{
			ID: "EXT-010", Name: "upc", TypeName: "upc",
			Extensions:   []string{"isn"},
			ColumnDDL:    "upc",
			InitialValue: "'036000291452'::upc", AltValue: "'012345678905'::upc",
		},
		{
			ID: "EXT-011", Name: "lquery", TypeName: "lquery",
			Extensions:   []string{"ltree"},
			ColumnDDL:    "lquery",
			InitialValue: "'*.foo.*'::lquery", AltValue: "'Top.*{1,2}.bar'::lquery",
			Note: "ltree query pattern; MISC-004 covers plain ltree",
		},
		{
			ID: "EXT-012", Name: "ltxtquery", TypeName: "ltxtquery",
			Extensions:   []string{"ltree"},
			ColumnDDL:    "ltxtquery",
			InitialValue: "'a & b'::ltxtquery", AltValue: "'c | d'::ltxtquery",
		},
		{
			ID: "EXT-013", Name: "earth", TypeName: "earth (domain over cube)",
			// cube first: earthdistance depends on it, and the harness installs
			// extensions in the order listed rather than with CASCADE.
			Extensions:   []string{"cube", "earthdistance"},
			ColumnDDL:    "earth",
			InitialValue: "ll_to_earth(45,9)", AltValue: "ll_to_earth(-33.86,151.21)",
			RecordDestValue: true,
			Note: "a domain over cube, so it is also a second data point on whether the domain " +
				"form bypasses the unsupported-datatype guardrail",
		},
		{
			ID: "EXT-014", Name: "lo", TypeName: "lo (domain over oid)",
			Extensions:   []string{"lo"},
			ColumnDDL:    "lo",
			InitialValue: "'12345'::lo", AltValue: "'67890'::lo",
			Note: "named in voyager's own unsupported list and never probed; the column holds " +
				"only the large-object OID, and the large object itself is not part of the value",
		},
	}
}

// ============================================================
// BATCH TABLE
// ============================================================

// sweepBatches is the batch table every mode iterates. Each entry becomes a subtest
// named after Name, so `-run 'TestDatatypeSweepLive/ranges'` runs exactly one batch.
// Poison probes are deliberately absent: they live in poisonProbes() and must be run
// one at a time through TestDatatypeSweepSuspect.
func sweepBatches() []sweepBatch {
	return []sweepBatch{
		{Name: "controls", Probes: nil}, // controls are prepended to every batch anyway
		{Name: "ranges", Probes: rangeProbes()},
		{Name: "multiranges", Probes: multirangeProbes()},
		{Name: "domains", Probes: domainProbes()},
		{Name: "composites", Probes: compositeProbes()},
		{Name: "enums", Probes: enumProbes()},
		{Name: "arrays", Probes: arrayProbes()},
		{Name: "hstore", Probes: hstoreProbes()},
		{Name: "core", Probes: coreScalarProbes()},
		{Name: "geo2", Probes: geometricScalarProbes()},
		{Name: "regtypes", Probes: regTypeProbes()},
		{Name: "catalogtypes", Probes: catalogTypeProbes()},
		{Name: "catalogstats", Probes: catalogStatsProbes()},
		{Name: "indexkeys", Probes: indexKeyTypeProbes()},
		{Name: "exttypes", Probes: extensionTypeProbes()},
		{Name: "system", Probes: systemTypeProbes()},
		{Name: "misc", Probes: miscTypeProbes()},
		{Name: "values", Probes: valueEdgeProbes()},
		{Name: "postgis", Probes: postgisProbes()},
		{Name: "arraydelim", Probes: arrayDelimiterProbes()},
		{Name: "pgvector", Probes: pgvectorProbes()},
	}
}

// poisonProbes are the probes expected to wedge a channel. They are never batched.
func poisonProbes() []datatypeProbe {
	return []datatypeProbe{
		hstoreNullValueProbe(),
	}
}

// allSweepProbes is every probe the harness knows about, batched or not. Used by the
// single-probe selector and by the duplicate-id guard.
func allSweepProbes() []datatypeProbe {
	var out []datatypeProbe
	out = append(out, controlProbes()...)
	for _, b := range sweepBatches() {
		out = append(out, b.Probes...)
	}
	out = append(out, poisonProbes()...)
	return out
}

// findProbeByID looks up one probe for TestDatatypeSweepSuspect.
func findProbeByID(id string) (datatypeProbe, bool) {
	for _, p := range allSweepProbes() {
		if p.ID == id {
			return p, true
		}
	}
	return datatypeProbe{}, false
}
