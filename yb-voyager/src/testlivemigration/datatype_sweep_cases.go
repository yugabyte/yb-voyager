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
