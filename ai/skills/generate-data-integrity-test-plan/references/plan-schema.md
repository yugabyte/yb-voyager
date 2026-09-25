# Plan file format

One JSON document. `hunt-data-integrity-bugs` validates it before running and rejects cases with missing required fields.

```jsonc
{
  "version": 1,
  "generated_at": "2026-09-25T10:00:00Z",
  "target_commit": "397711e0c…",           // what the hunt builds and tests
  "change_set": { "base": "…", "head": "…", "commits": ["sha subject", "…"], "prs": [3814] },
  "areas": ["partitions", "apply-sql"],
  "assumptions": ["no YB version pinned; hunt uses the testcontainers default"],
  "inventory": {                           // built in Step 0.5 (references/inventory.md)
    "flags": [{"command": "import data", "flag": "--use-partition-root", "type": "bool", "default": "true"}],
    "uncatalogued_flags": [], "stale_flags": [], "env": ["NUM_EVENT_CHANNELS"],
    "guardrails": [{"condition": "DEFERRABLE unique/PK constraint", "message": "…", "where": "cmd/…:123", "stage": "export"}],
    "framework": ["StartExportData", "StartImportDataWithEnv", "…"], "anchors": {"unexpected rows affected": "src/tgtdb/yugabytedb.go:1268"},
    "drift": ["anchor GetEffectiveTableName moved to …"]
  },
  "batches": [                              // cases that may share one migration
    { "id": "B1", "flow": "live", "case_ids": ["C3", "C4"], "import_flags": {"--use-partition-root": "false"} }
  ],
  "cases": [ /* Case, see below */ ]
}
```

## Case

| Field | Required | Meaning |
|---|---|---|
| `id` | ✓ | `C1`, `C2`, … unique in the plan |
| `title` | ✓ | one line, symptom-oriented |
| `priority` | ✓ | `P0` \| `P1` \| `P2` |
| `mechanism` | ✓ | `M1`…`M10` from `silent-loss-mechanisms.md` |
| `areas` | ✓ | areas from the SKILL's Step 1 table |
| `linked_change` | | commit SHA / file:function this case targets; `baseline` for baseline cases |
| `why_it_can_fail` | ✓ | the exact sequence that would produce silent loss if the code is wrong |
| `flow` | ✓ | `live` \| `fallback` \| `fallforward` \| `changes-only` \| `iterative` \| `offline` |
| `schemas` | ✓ | schema names passed to the framework (`SchemaNames`) |
| `schema_sql` | ✓ | DDL run on source **and** target |
| `source_setup_sql` | | source-only statements (REPLICA IDENTITY FULL, source-only constraints) |
| `target_setup_sql` | | target-only statements (to create deliberate drift) |
| `initial_sql` | ✓ | snapshot data |
| `phases` | ✓ | ordered list of `{ "sql": [...] }` or `{ "action": "kill_import" \| "kill_export" \| "stop_import" \| "resume_import" \| "cutover_to_target" \| "cutover_to_source" \| "wait_streaming" \| "ddl_source" \| "ddl_target", "args": {...} }` |
| `export_flags` / `import_flags` | | CLI flags; `import_env` for env vars (e.g. `NUM_EVENT_CHANNELS`) |
| `snapshot_rows` | ✓ | map `"schema"."table"` → rows expected after snapshot |
| `tables` | ✓ | map table-as-in-SELECT → ORDER BY for the oracle |
| `oracle_extra` | | extra checks: `per_partition_counts`, `sequence_after_cutover`, `no_rows_affected_warnings`, `queue_contains` |
| `expect` | ✓ | `consistent` \| `refused` \| `loud` |
| `ddl_probe` | | `true` if any DDL may be unsupported on YB — probe first |
| `value_fuzz` | | only for `value-encoding` changes: `{ "columns": [{"name", "type"}], "rows": N, "seed": S, "ops": ["insert", "update"], "edge_values": true }` — the hunt generates randomized and edge values per type |

## Worked example (the case that found yb-voyager#3834)

```json
{
  "id": "C7",
  "title": "Root without PK + default --use-partition-root: UPDATE/DELETE on a leaf-local PK hit every leaf",
  "priority": "P0",
  "mechanism": "M3",
  "areas": ["partitions", "apply-sql", "guardrails"],
  "linked_change": "baseline",
  "why_it_can_fail": "Event.Key is the leaf PK {id}; with use-partition-root=true the statement runs on the root as WHERE id=…, which matches the same id in every leaf. Rows-affected≠1 is only a WARN, and with no INSERTs nothing trips ON CONFLICT.",
  "flow": "live",
  "schemas": ["kp"],
  "schema_sql": [
    "CREATE SCHEMA IF NOT EXISTS kp;",
    "CREATE TABLE kp.lr (id int NOT NULL, region text NOT NULL, v int) PARTITION BY LIST (region);",
    "CREATE TABLE kp.lr_r1 PARTITION OF kp.lr FOR VALUES IN ('r1');",
    "CREATE TABLE kp.lr_r2 PARTITION OF kp.lr FOR VALUES IN ('r2');",
    "ALTER TABLE kp.lr_r1 ADD PRIMARY KEY (id);",
    "ALTER TABLE kp.lr_r2 ADD PRIMARY KEY (id);"
  ],
  "source_setup_sql": ["ALTER TABLE kp.lr REPLICA IDENTITY FULL;", "ALTER TABLE kp.lr_r1 REPLICA IDENTITY FULL;", "ALTER TABLE kp.lr_r2 REPLICA IDENTITY FULL;"],
  "initial_sql": [
    "INSERT INTO kp.lr SELECT i, 'r1', 0 FROM generate_series(1,20) i;",
    "INSERT INTO kp.lr SELECT i, 'r2', 0 FROM generate_series(1,20) i;"
  ],
  "phases": [
    { "sql": ["UPDATE kp.lr SET v = 1 WHERE id <= 5 AND region = 'r1';", "DELETE FROM kp.lr WHERE id = 20 AND region = 'r2';"] },
    { "action": "wait_streaming", "args": { "\"kp\".\"lr\"": { "inserts": 0, "updates": 5, "deletes": 1 } } }
  ],
  "import_flags": { "--cdc-partition-key": "auto" },
  "snapshot_rows": { "\"kp\".\"lr\"": 40 },
  "tables": { "kp.lr": "id, region" },
  "oracle_extra": ["per_partition_counts", "no_rows_affected_warnings"],
  "expect": "refused"
}
```

`expect: refused` records what *should* happen (a guardrail). The observed outcome was silent corruption, so the hunt classified it `SILENT` and opened a PR.
