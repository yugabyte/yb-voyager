"""
Unit tests for the hstore/array INSERT-value robustness fix in utils.py:

  - hstore columns (reported by Postgres as data_type='USER-DEFINED',
    udt_name='hstore') now get a real 'k=>v' literal instead of an
    explicit NULL (generate_hstore_value, _is_hstore_udt,
    _resolve_hstore_columns, and the inline hstore relabeling in
    generate_table_schemas_bulk).
  - ARRAY columns now get a real '{...}' literal for a broader set of
    element types -- including 'character varying' (varchar[]/character
    varying[] columns), which the old substring checks
    ("varchar" in array_types / "text" in array_types) missed entirely,
    since "varchar" and "text" are not substrings of "character varying"
    (generate_array_literal, _classify_array_element).
  - Any column still unsynthesizable (an exotic USER-DEFINED type with no
    enum labels, or an ARRAY of an element type not recognized above) is
    now OMITTED from the INSERT's column list entirely -- so the column's
    DEFAULT applies -- instead of an explicit SQL NULL, which used to
    override a NOT-NULL-with-DEFAULT column's DEFAULT and raise a
    not-null violation that killed the whole worker process
    (get_insert_column_list, and build_insert_values' use of it).

Stdlib unittest, no DB, no network -- matches test_utils.py's style.
"""

import re
import unittest

import utils
from utils import (
    build_insert_values,
    generate_array_literal,
    generate_hstore_value,
    generate_random_data,
    get_insert_column_list,
)

# hstore literal shape: one or more `key=>value` pairs, comma-separated.
# Faker's word() never contains '=', '>', ',', or whitespace, so neither
# side needs quoting -- see generate_hstore_value.
_HSTORE_PAIR_RE = re.compile(r"^[^,=]+=>[^,=]+(,[^,=]+=>[^,=]+)*$")


# --------------------------------------------------------------------------
# generate_hstore_value / generate_random_data("hstore", ...)
# --------------------------------------------------------------------------

class TestHstoreValueGeneration(unittest.TestCase):
    def test_generate_hstore_value_matches_kv_literal_shape(self):
        for _ in range(20):
            value = generate_hstore_value()
            self.assertIsInstance(value, str)
            self.assertRegex(value, _HSTORE_PAIR_RE)

    def test_generate_random_data_hstore_never_returns_none(self):
        for _ in range(20):
            value = generate_random_data("hstore", "payments")
            self.assertIsNotNone(value)
            self.assertRegex(value, _HSTORE_PAIR_RE)

    def test_hstore_still_a_valid_value_when_min_col_size_bytes_set(self):
        # min_col_size_bytes only widens text/json/tsvector/ARRAY generation
        # (see generate_random_data) -- hstore must be unaffected, not None.
        value = generate_random_data("hstore", "payments", min_col_size_bytes=50)
        self.assertIsNotNone(value)
        self.assertRegex(value, _HSTORE_PAIR_RE)


# --------------------------------------------------------------------------
# generate_array_literal / generate_random_data("ARRAY", ...)
# --------------------------------------------------------------------------

class TestArrayValueGeneration(unittest.TestCase):
    def test_character_varying_array_produces_quoted_elements(self):
        # This is the exact regression: 'character varying' (the regtype
        # text Postgres reports for varchar[]/character varying[] columns)
        # is not a substring of "varchar" or "text", so the old code fell
        # through with no return -- an implicit NULL. Confirm it's now a
        # valid '{"...","..."}' literal.
        value = generate_random_data("ARRAY", "account_information_consents", array_types="character varying")
        self.assertIsNotNone(value)
        self.assertTrue(value.startswith("{") and value.endswith("}"))
        self.assertIn('"', value)
        self.assertNotIn("NULL", value)

    def test_text_array_produces_quoted_elements(self):
        value = generate_array_literal("text")
        self.assertIsNotNone(value)
        self.assertTrue(value.startswith("{") and value.endswith("}"))
        self.assertIn('"', value)

    def test_integer_array_produces_bare_unquoted_elements(self):
        value = generate_array_literal("integer")
        self.assertIsNotNone(value)
        self.assertTrue(value.startswith("{") and value.endswith("}"))
        self.assertNotIn('"', value)
        for elem in value.strip("{}").split(","):
            int(elem)  # raises if not a bare integer literal

    def test_boolean_array_produces_true_false_tokens(self):
        value = generate_array_literal("boolean")
        self.assertIsNotNone(value)
        for elem in value.strip("{}").split(","):
            self.assertIn(elem, ("true", "false"))

    def test_uuid_array_produces_quoted_uuids(self):
        value = generate_array_literal("uuid")
        self.assertIsNotNone(value)
        for elem in value.strip("{}").split(","):
            self.assertTrue(elem.startswith('"') and elem.endswith('"'))

    def test_unrecognized_element_type_returns_none_not_a_guess(self):
        # date[]/timestamp[]/json[]/geometric/composite element types: no
        # safe literal to guess (unlike text, a bare fake word is not valid
        # input for e.g. a date column) -- must return None so the caller
        # omits the column instead of risking an invalid-syntax error.
        for unsupported in ("date", "timestamp without time zone", "json", "point", "my_enum_type"):
            self.assertIsNone(generate_array_literal(unsupported))

    def test_generate_random_data_array_of_unrecognized_type_returns_none(self):
        value = generate_random_data("ARRAY", "vrf_verified_products", array_types="date")
        self.assertIsNone(value)

    def test_no_array_types_metadata_returns_none(self):
        self.assertIsNone(generate_array_literal(None))
        self.assertIsNone(generate_array_literal(""))

    def test_bracket_suffixed_element_type_is_still_classified(self):
        # get_array_element_type/generate_table_schemas_bulk resolve the
        # element type via `udt_name::regtype`, where udt_name for an ARRAY
        # column is the array type's OWN name (e.g. '_int4', '_bool') --
        # regtype's output function renders that as "<element>[]"
        # (confirmed against a live Postgres: an integer[] column's
        # resolved "element type" comes back as 'integer[]', not
        # 'integer'). Every recognized kind must still classify correctly
        # with that trailing '[]' present -- a real bug caught by a live
        # smoke test: the numeric branch used exact string-set membership,
        # which 'integer[]' != 'integer' fails, silently omitting every
        # int[]/numeric[]/etc. column and always falling back to its
        # DEFAULT instead of ever generating a real value.
        cases = {
            "integer[]": False,
            "bigint[]": False,
            "smallint[]": False,
            "numeric[]": False,
            "double precision[]": False,
            "boolean[]": False,
            "uuid[]": True,  # quoted, same as the "string" kind
            "character varying[]": True,  # string kind -> quoted
            "text[]": True,
        }
        for element_type, expect_quoted in cases.items():
            value = generate_array_literal(element_type)
            self.assertIsNotNone(value, element_type)
            self.assertTrue(value.startswith("{") and value.endswith("}"), element_type)
            has_quotes = '"' in value
            self.assertEqual(has_quotes, expect_quoted, f"{element_type}: {value}")


# --------------------------------------------------------------------------
# _is_hstore_udt / _resolve_hstore_columns (schema-builder detection)
# --------------------------------------------------------------------------

class _FakeUdtCursor:
    """Minimal fake cursor for _resolve_hstore_columns: answers the
    'SELECT udt_name FROM information_schema.columns WHERE ...' query with
    a canned udt_name for the (table_name, column_name) bound in the last
    two positional params, ignoring the SQL text itself."""

    def __init__(self, udt_by_column):
        self._udt_by_column = udt_by_column
        self.last_params = None

    def execute(self, sql, params=None):
        self.last_params = params

    def fetchone(self):
        table_name, column_name = self.last_params[-2:]
        udt = self._udt_by_column.get((table_name, column_name))
        return (udt,) if udt is not None else None


class TestIsHstoreUdt(unittest.TestCase):
    def test_recognizes_plain_and_qualified_forms(self):
        for value in ("hstore", "HSTORE", "public.hstore", '"hstore"', "  hstore  "):
            self.assertTrue(utils._is_hstore_udt(value), value)

    def test_rejects_other_types_and_none(self):
        for value in (None, "", "citext", "my_enum_type", "text", "hstorex"):
            self.assertFalse(utils._is_hstore_udt(value), value)


class TestResolveHstoreColumns(unittest.TestCase):
    def test_relabels_only_the_hstore_column(self):
        columns = {"id": "integer", "metadata": "USER-DEFINED", "status": "USER-DEFINED"}
        cur = _FakeUdtCursor({
            ("payments", "metadata"): "hstore",
            ("payments", "status"): "my_enum_type",
        })
        utils._resolve_hstore_columns(cur, "payments", "public", columns)
        self.assertEqual(columns["metadata"], "hstore")
        self.assertEqual(columns["status"], "USER-DEFINED")  # not hstore -- untouched
        self.assertEqual(columns["id"], "integer")  # not USER-DEFINED -- never queried


# --------------------------------------------------------------------------
# get_insert_column_list: the omit-for-default fallback
# --------------------------------------------------------------------------

class TestGetInsertColumnList(unittest.TestCase):
    def _schema(self, columns, primary_key=None, array_types=None, enum_values=None):
        return {
            "t": {
                "columns": columns,
                "primary_key": primary_key,
                "array_types": array_types or {},
                "enum_values": enum_values or {},
                "bit_info": {},
            }
        }

    def test_hstore_column_is_included(self):
        schema = self._schema({"id": "integer", "metadata": "hstore"})
        self.assertEqual(get_insert_column_list(schema, "t"), ["id", "metadata"])

    def test_recognized_array_column_is_included(self):
        schema = self._schema(
            {"id": "integer", "tags": "ARRAY"}, array_types={"tags": "character varying"},
        )
        self.assertEqual(get_insert_column_list(schema, "t"), ["id", "tags"])

    def test_user_defined_without_enum_is_omitted(self):
        schema = self._schema({"id": "integer", "junk": "USER-DEFINED"})
        self.assertEqual(get_insert_column_list(schema, "t"), ["id"])

    def test_user_defined_with_enum_is_included(self):
        schema = self._schema(
            {"id": "integer", "status": "USER-DEFINED"},
            enum_values={"status": ["active", "inactive"]},
        )
        self.assertEqual(get_insert_column_list(schema, "t"), ["id", "status"])

    def test_array_of_unrecognized_element_type_is_omitted(self):
        schema = self._schema(
            {"id": "integer", "birthdays": "ARRAY"}, array_types={"birthdays": "date"},
        )
        self.assertEqual(get_insert_column_list(schema, "t"), ["id"])

    def test_column_override_forces_inclusion_despite_unsynthesizable_type(self):
        schema = self._schema({"id": "integer", "junk": "USER-DEFINED"})
        overrides = {"t": {"junk": {"type": "choice", "values": ["x"]}}}
        self.assertEqual(
            get_insert_column_list(schema, "t", column_overrides=overrides), ["id", "junk"],
        )

    def test_unique_value_fn_forces_inclusion_despite_unsynthesizable_type(self):
        schema = self._schema({"id": "integer", "junk": "USER-DEFINED"})
        self.assertEqual(
            get_insert_column_list(schema, "t", unique_value_fns={"junk": lambda: "u1"}),
            ["id", "junk"],
        )

    def test_pk_value_fn_forces_inclusion_of_pk_column(self):
        # A non-integer/otherwise-unsynthesizable single-column PK would
        # never actually get a pk_value_fn in practice, but the priority
        # rule (pk_value_fn always wins for the PK column) must hold
        # regardless.
        schema = self._schema({"id": "USER-DEFINED"}, primary_key=["id"])
        self.assertEqual(
            get_insert_column_list(schema, "t", pk_value_fn=lambda: 1), ["id"],
        )

    def test_ordinary_columns_all_included_unchanged(self):
        schema = self._schema({
            "id": "integer",
            "name": "character varying(50)",
            "active": "boolean",
            "created_at": "timestamp without time zone",
            "payload": "jsonb",
            "token": "uuid",
        })
        self.assertEqual(
            get_insert_column_list(schema, "t"),
            ["id", "name", "active", "created_at", "payload", "token"],
        )


# --------------------------------------------------------------------------
# build_insert_values: end-to-end (hstore / array get real values; the
# still-unsynthesizable case is omitted, not NULL'd)
# --------------------------------------------------------------------------

class TestBuildInsertValuesHstoreAndArray(unittest.TestCase):
    def test_hstore_column_gets_a_real_value_not_null(self):
        schema = {
            "payments": {
                "columns": {"id": "integer", "metadata": "hstore"},
                "primary_key": ["id"],
                "array_types": {},
                "enum_values": {},
                "bit_info": {},
            }
        }
        values_str, _ = build_insert_values(schema, "payments", 3)
        self.assertNotIn("NULL", values_str)
        # 2 columns * 3 rows -> 6 quoted values -> 12 single quotes.
        self.assertEqual(values_str.count("'"), 12)
        for pairs in re.findall(r"'([^']*=>[^']*)'", values_str):
            self.assertRegex(pairs, _HSTORE_PAIR_RE)

    def test_varchar_array_column_gets_a_real_value_not_null(self):
        schema = {
            "account_information_consents": {
                "columns": {"id": "integer", "access_scopes": "ARRAY"},
                "primary_key": ["id"],
                "array_types": {"access_scopes": "character varying"},
                "enum_values": {},
                "bit_info": {},
            }
        }
        values_str, _ = build_insert_values(schema, "account_information_consents", 2)
        self.assertNotIn("NULL", values_str)
        self.assertEqual(values_str.count("'"), 8)  # 2 columns * 2 rows * 2 quotes
        for arr in re.findall(r"'(\{[^']*\})'", values_str):
            self.assertTrue(arr.startswith("{") and arr.endswith("}"))

    def test_unsynthesizable_column_is_omitted_not_nulled(self):
        # 'junk' (USER-DEFINED, no enum labels -- a composite type, or any
        # other extension scalar type this generator doesn't recognize) can
        # never get a real value. It must be dropped from the column list
        # and from every row's value tuple -- never an explicit NULL that
        # could violate a NOT NULL constraint on a column with a DEFAULT.
        schema = {
            "widgets": {
                "columns": {"id": "integer", "junk": "USER-DEFINED", "amount": "integer"},
                "primary_key": ["id"],
                "array_types": {},
                "enum_values": {},
                "bit_info": {},
            }
        }
        columns = get_insert_column_list(schema, "widgets")
        self.assertEqual(columns, ["id", "amount"])

        values_str, pk_values = build_insert_values(schema, "widgets", 1)
        self.assertNotIn("NULL", values_str)
        # Exactly 2 quoted values (id, amount) -- 'junk' contributes none.
        self.assertEqual(values_str.count("'"), 4)
        self.assertEqual(len(pk_values), 1)
        self.assertIsInstance(pk_values[0], int)

    def test_omitted_only_column_yields_empty_row(self):
        # Degenerate case: the table's only column is unsynthesizable and
        # has no PK. get_insert_column_list must return [] and
        # build_insert_values must emit an empty column list / empty value
        # tuple per row -- never a NULL placeholder.
        schema = {
            "oddities": {
                "columns": {"junk": "USER-DEFINED"},
                "primary_key": None,
                "array_types": {},
                "enum_values": {},
                "bit_info": {},
            }
        }
        self.assertEqual(get_insert_column_list(schema, "oddities"), [])
        values_str, pk_values = build_insert_values(schema, "oddities", 2)
        self.assertEqual(values_str, "(), ()")
        self.assertEqual(pk_values, [None, None])

    def test_array_of_unrecognized_type_is_omitted_not_nulled(self):
        schema = {
            "vrf_verified_products": {
                "columns": {"id": "integer", "capabilities": "ARRAY", "amount": "integer"},
                "primary_key": ["id"],
                "array_types": {"capabilities": "date"},
                "enum_values": {},
                "bit_info": {},
            }
        }
        columns = get_insert_column_list(schema, "vrf_verified_products")
        self.assertEqual(columns, ["id", "amount"])
        values_str, _ = build_insert_values(schema, "vrf_verified_products", 1)
        self.assertNotIn("NULL", values_str)
        self.assertEqual(values_str.count("'"), 4)

    def test_regular_types_unaffected(self):
        # Sanity: ordinary types produce the same shape of output as
        # before -- no NULLs, no omissions, one value per column per row.
        schema = {
            "orders": {
                "columns": {
                    "id": "integer",
                    "name": "character varying(50)",
                    "active": "boolean",
                    "created_at": "timestamp without time zone",
                    "payload": "jsonb",
                    "token": "uuid",
                    "amount": "numeric(7,2)",
                },
                "primary_key": ["id"],
                "array_types": {},
                "enum_values": {},
                "bit_info": {},
            }
        }
        self.assertEqual(
            get_insert_column_list(schema, "orders"),
            ["id", "name", "active", "created_at", "payload", "token", "amount"],
        )
        values_str, pk_values = build_insert_values(schema, "orders", 2)
        self.assertNotIn("NULL", values_str)
        self.assertEqual(values_str.count("'"), 28)  # 7 columns * 2 rows * 2 quotes
        self.assertEqual(len(pk_values), 2)


if __name__ == "__main__":
    unittest.main()
