"""Unit tests for the orchestrator `presplit_tables` action.

Verifies that `SPLIT INTO <n> TABLETS` is appended to exactly the named tables'
inline-PK CREATE statements in the exported schema (schema/tables/table.sql),
that it is idempotent, and that it fails loudly on a missing / empty table list.

Run: python3 -m pytest test_presplit_tables.py   (from this directory)
"""
import os
import types
import tempfile

import pytest

import orchestrator as O

SAMPLE = (
    "CREATE TABLE public.cutover_table (id int NOT NULL, status text, "
    "CONSTRAINT cutover_table_pkey PRIMARY KEY (id));\n"
    "ALTER TABLE ONLY public.cutover_table REPLICA IDENTITY FULL ;\n"
    "CREATE TABLE public.table_017d77bfa1eae59d (col_a varchar NOT NULL, col_b text, "
    "CONSTRAINT c1 PRIMARY KEY (col_a));\n"
    "ALTER TABLE ONLY public.table_017d77bfa1eae59d REPLICA IDENTITY FULL ;\n"
    "CREATE TABLE public.table_01b166ea768098a2 (col_a varchar NOT NULL, "
    "CONSTRAINT c2 PRIMARY KEY (col_a));\n"
    "CREATE TABLE public.table_untouched (col_a varchar NOT NULL, "
    "CONSTRAINT c3 PRIMARY KEY (col_a));\n"
)


def _make_ctx(d):
    ed = os.path.join(d, "spike-export-dir")
    os.makedirs(os.path.join(ed, "schema", "tables"))
    path = os.path.join(ed, "schema", "tables", "table.sql")
    with open(path, "w") as f:
        f.write(SAMPLE)
    ctx = types.SimpleNamespace(cfg={"export_dir": "spike-export-dir"}, test_root=d)
    return ctx, path


def _lines_for(path, prefix):
    return [l for l in open(path).read().splitlines() if l.startswith(prefix)]


def test_appends_split_to_named_tables_only():
    with tempfile.TemporaryDirectory() as d:
        ctx, path = _make_ctx(d)
        O.get_action("presplit_tables")(
            {"tablets": 6,
             "tables": ["table_017d77bfa1eae59d", "table_01b166ea768098a2"]}, ctx)
        a = _lines_for(path, "CREATE TABLE public.table_017d77bfa1eae59d")[0]
        b = _lines_for(path, "CREATE TABLE public.table_01b166ea768098a2")[0]
        u = _lines_for(path, "CREATE TABLE public.table_untouched")[0]
        assert a.endswith("PRIMARY KEY (col_a)) SPLIT INTO 6 TABLETS;")
        assert b.endswith("PRIMARY KEY (col_a)) SPLIT INTO 6 TABLETS;")
        assert "SPLIT" not in u and u.endswith("PRIMARY KEY (col_a));")
        # substring match must not accidentally hit cutover_table etc.
        assert all("cutover_table" not in l for l in open(path) if "SPLIT" in l)


def test_idempotent():
    with tempfile.TemporaryDirectory() as d:
        ctx, path = _make_ctx(d)
        for _ in range(2):
            O.get_action("presplit_tables")(
                {"tablets": 6, "tables": ["table_017d77bfa1eae59d"]}, ctx)
        a = _lines_for(path, "CREATE TABLE public.table_017d77bfa1eae59d")[0]
        assert a.count("SPLIT INTO") == 1


def test_missing_table_raises():
    with tempfile.TemporaryDirectory() as d:
        ctx, _ = _make_ctx(d)
        with pytest.raises(ValueError, match="not found"):
            O.get_action("presplit_tables")({"tables": ["nope"]}, ctx)


def test_empty_list_raises():
    with tempfile.TemporaryDirectory() as d:
        ctx, _ = _make_ctx(d)
        with pytest.raises(ValueError):
            O.get_action("presplit_tables")({"tables": []}, ctx)
