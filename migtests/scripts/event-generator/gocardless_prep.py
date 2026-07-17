#!/usr/bin/env python3
"""
gocardless_prep.py — turn the Voyager callhome anonymized GoCardless DDL into a
schema that loads on YB *and* can be driven by the random event generator.

The raw callhome DDL is one SQL statement per line. It is faithful to a real
production schema, which means it carries a lot that a random-IUD generator and
a fresh YB target cannot handle. This tool strips exactly those things and
nothing else.

Transforms (all line-oriented; each statement is a single line in the dump):

  1. Extensions: drop `amcheck` / `pgstattuple` (diagnostic, often absent on the
     target); keep `hstore`.
  2. Custom types: replace dangling `schema_*.type_*` refs (callhome omits custom
     TYPEs) with `text`.
  3. json / pg_catalog.json -> jsonb (json has no equality op → breaks row-hash
     and REPLICA IDENTITY FULL comparisons).
  4. FOREIGN KEYs: drop every `ALTER TABLE … ADD … FOREIGN KEY …`. The generator
     writes random rows independently; it cannot satisfy referential integrity,
     so hundreds of FKs would make almost every INSERT fail.
  5. CHECK constraints: strip inline `CONSTRAINT … CHECK (…)` from CREATE TABLE.
     Many reference anonymized `const_…` literals compared against int/date/
     numeric columns, which fail at DDL time; and random data can't satisfy them.
  6. Partitioning: flatten. Strip `PARTITION BY …(…)` from parents (partition
     bounds are anonymized `const_…` and won't cast to date/numeric), and drop
     every partition child (CREATE/ALTER/PK/index) + all ATTACH PARTITION and
     `CREATE INDEX … ON ONLY …` / `ALTER INDEX … ATTACH PARTITION …` lines.
  7. Anonymized opclasses: strip ` opclass_<hex>` from index columns.
  8. Storage params: drop ` WITH (autovacuum…)` (YB may reject).
  9. PRIMARY KEYs: keep the real ones the DDL already declares; add a first-column
     PK only for any base table left without one (rare). Needed for the generator's
     UPDATE/DELETE + PK-pool and for row-hash.

  10. Schema remap (optional, --schema NAME): rewrite the anonymized container
      schema (schema_<hex>) to NAME. Use `--schema public` so the tables land
      alongside the harness's public.cutover_table + single-schema row-hash /
      row-count validators — the existing live-migration flow then works with no
      harness changes (generator schema_name stays `public`).

Usage:  python3 gocardless_prep.py in.sql [--schema public] > out.sql   (report on stderr)
"""
import re
import sys

DROP_EXT = ("amcheck", "pgstattuple")  # hstore kept


def _strip_inline_checks(line):
    """Remove every `, CONSTRAINT <name> CHECK (<balanced>)` fragment from a
    CREATE TABLE line using a paren-balanced scan (regex can't handle the nested
    parens in CHECK bodies). Returns (new_line, count_removed)."""
    marker = " CONSTRAINT "
    out = line
    removed = 0
    while True:
        m = re.search(r",\s*CONSTRAINT\s+\w+\s+CHECK\s*\(", out)
        if not m:
            break
        start = m.start()          # at the comma
        i = m.end() - 1            # at the '(' after CHECK
        depth = 0
        while i < len(out):
            if out[i] == "(":
                depth += 1
            elif out[i] == ")":
                depth -= 1
                if depth == 0:
                    break
            i += 1
        # remove out[start : i+1]  (the ", CONSTRAINT … CHECK (…)")
        out = out[:start] + out[i + 1:]
        removed += 1
    return out, removed


def _remap_schema(text, to_schema):
    """Rewrite the anonymized container schema (schema_<hex>) to to_schema. The
    anonymized name is a unique token, so a plain replace is safe. Drops the
    CREATE SCHEMA line when targeting public (it always pre-exists)."""
    m = re.search(r"CREATE SCHEMA (schema_[0-9a-f]+);", text)
    if not m:
        return text, None
    orig = m.group(1)
    if to_schema == "public":
        text = text.replace(f"CREATE SCHEMA {orig};\n", "")
    else:
        text = text.replace(f"CREATE SCHEMA {orig};", f"CREATE SCHEMA IF NOT EXISTS {to_schema};")
    return text.replace(orig, to_schema), orig


def prep(text, to_schema=None):
    lines = text.splitlines()
    while lines and not lines[-1].rstrip().endswith(";"):
        lines.pop()

    # ---- pass 1: identify partition child tables (named by ATTACH PARTITION) ----
    child_tables = set()
    for ln in lines:
        m = re.match(r"ALTER TABLE (?:ONLY )?\S+ ATTACH PARTITION (\S+?)(?:\s+FOR VALUES|\s+DEFAULT|;)", ln.strip())
        if m:
            child_tables.add(m.group(1))

    rpt = {"ext": 0, "type": 0, "json": 0, "fk": 0, "check": 0,
           "attach": 0, "on_only": 0, "child_lines": 0, "part_by": 0,
           "opclass": 0, "storage": 0, "pk_added": 0, "tables": 0,
           "seq_owned": 0, "const_idx": 0, "numeric_bounded": 0}
    out, tables, pk_tables = [], [], set()

    def refs_child(s):
        return any(re.search(r"\b" + re.escape(c) + r"\b", s) for c in child_tables)

    for ln in lines:
        s = ln.strip()

        # 1. extensions
        if s.startswith("CREATE EXTENSION") and any(e in s for e in DROP_EXT):
            rpt["ext"] += 1
            continue
        # 4. foreign keys
        if s.startswith("ALTER TABLE") and "FOREIGN KEY" in s:
            rpt["fk"] += 1
            continue
        # orphaned sequence ownership: dump lists OWNED BY before the table exists,
        # and DEFAULT nextval() clauses were already stripped, so it's cosmetic.
        if s.startswith("ALTER SEQUENCE") and "OWNED BY" in s:
            rpt["seq_owned"] += 1
            continue
        # partial / expression indexes whose predicate carries an anonymized literal
        # (a const_ placeholder or an empty-string `= ''`) that won't cast to the
        # column's type. Valid predicates (IS NULL / IS NOT NULL) are kept.
        if re.match(r"CREATE (?:UNIQUE )?INDEX", s) and ("const_" in s or "= ''" in s):
            rpt["const_idx"] += 1
            continue
        # 6. partition plumbing
        if "ATTACH PARTITION" in s:
            rpt["attach"] += 1
            continue
        if re.match(r"CREATE (?:UNIQUE )?INDEX .* ON ONLY ", s):
            rpt["on_only"] += 1
            continue
        # drop anything that targets a partition child table
        if child_tables and refs_child(s):
            rpt["child_lines"] += 1
            continue

        # per-line rewrites
        new = ln
        new, n = re.subn(r"\"?schema_[0-9a-f]+\"?\.\"?type_[0-9a-f]+\"?", "text", new); rpt["type"] += n
        if "pg_catalog.json" in new:
            rpt["json"] += new.count("pg_catalog.json"); new = new.replace("pg_catalog.json", "jsonb")
        new, n = re.subn(r"\s+opclass_[0-9a-f]+", "", new); rpt["opclass"] += n
        # bound unbounded numeric -> numeric(38,6). Unbounded numeric has no fixed
        # scale, so voyager's live CDC drops trailing zeros (60953.20 -> 60953.2)
        # while the snapshot keeps them, breaking the row-hash no-loss check. A fixed
        # scale makes snapshot and CDC store identical values on both sides.
        new, n = re.subn(r"\bnumeric\b(?!\s*\()", "numeric(38,6)", new); rpt["numeric_bounded"] += n
        new, n = re.subn(r"\s*WITH \((?:autovacuum|toast)[^)]*\)", "", new); rpt["storage"] += n
        if s.startswith("CREATE TABLE"):
            new, n = re.subn(r"\s*PARTITION BY (?:RANGE|LIST|HASH)\s*\([^)]*\)", "", new); rpt["part_by"] += n
            new, n = _strip_inline_checks(new); rpt["check"] += n

        out.append(new)

        m = re.match(r"CREATE TABLE (\S+)\s*\(\s*\"?(\w+)\"?", s)
        if m:
            tables.append((m.group(1), m.group(2))); rpt["tables"] += 1
        if "PRIMARY KEY" in s.upper():
            mt = re.match(r"CREATE TABLE (\S+)", s) or re.match(r"ALTER TABLE (?:ONLY )?(\S+)", s)
            if mt:
                pk_tables.add(mt.group(1))

    add = [f"ALTER TABLE {n} ADD PRIMARY KEY ({c});" for n, c in tables if n not in pk_tables]
    rpt["pk_added"] = len(add)
    if add:
        out += ["", "-- gocardless_prep: synthetic PKs for base tables missing one"] + add

    result = "\n".join(out) + "\n"
    rpt["remapped_from"] = None
    if to_schema:
        result, rpt["remapped_from"] = _remap_schema(result, to_schema)
    return result, rpt, len(child_tables)


def main():
    args = sys.argv[1:]
    to_schema = None
    if "--schema" in args:
        i = args.index("--schema")
        to_schema = args[i + 1]
        del args[i:i + 2]
    if len(args) != 1:
        sys.exit("usage: gocardless_prep.py in.sql [--schema NAME] > out.sql")
    with open(args[0]) as f:
        sql, rpt, n_children = prep(f.read(), to_schema=to_schema)
    sys.stdout.write(sql)
    if rpt.get("remapped_from"):
        print(f"[prep] schema remapped: {rpt['remapped_from']} -> {to_schema}", file=sys.stderr)
    print(f"[prep] base tables kept: {rpt['tables']}", file=sys.stderr)
    print(f"[prep] partition children dropped: {n_children} ({rpt['child_lines']} lines)", file=sys.stderr)
    print(f"[prep] foreign keys dropped: {rpt['fk']}", file=sys.stderr)
    print(f"[prep] const_ partial/expr indexes dropped: {rpt['const_idx']}; orphaned seq OWNED BY dropped: {rpt['seq_owned']}", file=sys.stderr)
    print(f"[prep] unbounded numeric -> numeric(38,6): {rpt['numeric_bounded']}", file=sys.stderr)
    print(f"[prep] inline CHECKs stripped: {rpt['check']}", file=sys.stderr)
    print(f"[prep] ATTACH PARTITION dropped: {rpt['attach']}; ON ONLY indexes dropped: {rpt['on_only']}", file=sys.stderr)
    print(f"[prep] PARTITION BY stripped: {rpt['part_by']}; opclass stripped: {rpt['opclass']}; storage stripped: {rpt['storage']}", file=sys.stderr)
    print(f"[prep] custom-type refs->text: {rpt['type']}; json->jsonb: {rpt['json']}; extensions dropped: {rpt['ext']}", file=sys.stderr)
    print(f"[prep] synthetic PKs added: {rpt['pk_added']}", file=sys.stderr)


if __name__ == "__main__":
    main()
