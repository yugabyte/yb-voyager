# Migration runbook

Exact command sequences for offline and live PG→YB migrations. Both are validated end-to-end against Docker PG17 + YugabyteDB (offline: full pipeline incl. the adaptive-parallelism boundary; live: snapshot + CDC delta + cutover + parity + end migration). Always run each `yb-voyager` invocation under `timeout` (hang detection).

Common setup for a run:
```
export YB_VOYAGER_SEND_DIAGNOSTICS=false
BIN=<path-to-branch-built yb-voyager>       # from build-voyager.sh
EXPDIR=<fresh export dir>                    # one per scenario; rm -rf then mkdir -p
# Use ARRAYS, not space-strings: this environment's default shell is zsh, which does NOT
# word-split an unquoted $VAR. `"${SRC[@]}"` expands to separate args in both bash and zsh.
SRC=(--source-db-type postgresql --source-db-host 127.0.0.1 --source-db-port 5490
     --source-db-user postgres --source-db-password postgres --source-db-name mtv_src --source-db-schema public)
TGT=(--target-db-host 127.0.0.1 --target-db-port 5491 --target-db-user yugabyte
     --target-db-password yugabyte --target-db-name mtv_tgt)
```
> Portability trap (learned the hard way): `import data ... $TGT` under zsh passes the whole
> string as one arg → `Error: unknown flag`. Always expand as `"${TGT[@]}"`.

The `--export-dir` is the shared workspace threaded through **every** command — set once, reuse. Key sub-paths to inspect: `schema/`, `schema/failed.sql` (must not exist after import), `data/`, `reports/*.json`, `logs/yb-voyager-*.log`, `metainfo/meta.db`.

## Offline PG→YB (validated)

Minimal core sequence (drop assess/analyze for a pure data run, keep them to also exercise those commands):

```
# 1. export schema
timeout 180 "$BIN" export schema --export-dir "$EXPDIR" "${SRC[@]}" --yes

# 2. analyze schema (optional but cheap; validates the report)
timeout 120 "$BIN" analyze-schema --export-dir "$EXPDIR" --output-format json --yes

# 3. export data (snapshot; uses pg_dump under the hood — pg_dump major version must match server)
timeout 300 "$BIN" export data --export-dir "$EXPDIR" "${SRC[@]}" --disable-pb true --yes

# 4. import schema
timeout 180 "$BIN" import schema --export-dir "$EXPDIR" "${TGT[@]}" --yes

# 5. import data  ← the parallelism/pool code path; where the adaptive-parallelism hang lived
timeout 300 "$BIN" import data --export-dir "$EXPDIR" "${TGT[@]}" <PARALLELISM FLAGS> --disable-pb true --yes

# 6. finalize schema post data import (FKs, indexes, triggers; --refresh-mviews to populate MVs)
timeout 180 "$BIN" finalize-schema-post-data-import --export-dir "$EXPDIR" "${TGT[@]}" --refresh-mviews true --yes

# 7. status reports (JSON) for the correctness oracle
"$BIN" export data status --export-dir "$EXPDIR" --output-format json
"$BIN" import data status --export-dir "$EXPDIR" --output-format json

# 8. end migration (backup + save reports)
timeout 120 "$BIN" end migration --export-dir "$EXPDIR" --backup-dir "$EXPDIR/backup" \
  --backup-schema-files true --backup-data-files true --backup-log-files true \
  --save-migration-reports true --yes
```

Notes:
- For PG source, do **not** pass `--target-db-schema` (only sent when source ≠ postgresql).
- `--target-db-name` on YB is conventionally the lowercased source name.
- The migtests offline import also passes `--max-retries-streaming 1 --skip-replication-checks true`; harmless for offline, unnecessary unless mirroring CI exactly.

## Live PG→YB (snapshot + CDC)

There is **no** `--enable-live-migration` flag — live mode is turned on purely by `export data --export-type snapshot-and-changes`. `import data` auto-detects live mode from the export dir and streams. `export data` and `import data` are **long-running daemons** — launch each in the background and track its PID with a cleanup trap.

Extra setup vs offline (all validated end-to-end):
- **`REPLICA IDENTITY FULL` on every source table** — required, or `export data` aborts at the guardrail: `Tables missing replica identity full: [...]`. `ALTER TABLE <t> REPLICA IDENTITY FULL;` for each. (Tables also MUST have a primary key — no-PK tables are unsupported for live.)
- Source needs replication privileges. A **superuser** (e.g. `postgres`) satisfies this directly (validated). Otherwise run the guardrail grant SQL: `psql ... -f /opt/yb-voyager/guardrails-scripts/yb-voyager-pg-grant-migration-permissions.sql -v voyager_user=... -v is_live_migration=1` (fall-back adds `-v is_live_migration_fall_back=1`). See `migtests/scripts/functions.sh:196` / `940`.
- PG `wal_level=logical` (the Docker source sets this).
- Debezium server present at `/opt/yb-voyager/debezium-server` → requires the **installer build** (`install-yb-voyager -v` / `-l`), not bare `go build`. (`-v` fast-rebuilds Go+Java from the worktree; note its final step `sudo mv`s the binary to `/usr/local/bin` and fails without a sudo terminal — the built binary still lands at `$(go env GOPATH)/bin/yb-voyager`, usable directly.)
- `export data` prints the created replication slot name; `cutover`/`end migration` drop it. `end migration` requires `--backup-dir` to **already exist** (`mkdir -p` it first) or it aborts with `backup-dir "..." doesn't exists`.

Sequence:
```
# schema first (same as offline: export schema, analyze, import schema)

# snapshot + changes exporter (background daemon)
timeout 900 "$BIN" export data --export-dir "$EXPDIR" "${SRC[@]}" --export-type snapshot-and-changes --disable-pb=true --yes &
# importer (background daemon; auto-detects live mode)
timeout 900 "$BIN" import data --export-dir "$EXPDIR" "${TGT[@]}" --max-retries-streaming 1 --skip-replication-checks true --disable-pb true --yes &
```

### Detecting "snapshot done" and "caught up" (critical state-transition gates)

`import data status` returns exit 1 in live mode and `get data-migration-report` does not signal snapshot completion — so use these instead:

1. **Snapshot import complete** → tail the import log for the marker string:
   ```
   grep -q "snapshot data import complete" "$EXPDIR/logs/yb-voyager-import-data.log"
   ```
2. **Generate delta / CDC events** on the source (e.g. run a `source_delta.sql` of INSERT/UPDATE/DELETE), then confirm the exporter captured them by summing `exported_inserts+updates+deletes` from:
   ```
   "$BIN" get data-migration-report --export-dir "$EXPDIR" --output-format json
   ```
   (matches `count_exported_events` in `functions.sh:696`).
3. **Cutover + drain** — the real "caught up" gate:
   ```
   timeout 120 "$BIN" initiate cutover to target --export-dir "$EXPDIR" --prepare-for-fall-back false --yes
   # poll until COMPLETED (the daemons drain remaining events and exit)
   "$BIN" cutover status --export-dir "$EXPDIR"     # row "source → target" column == COMPLETED
   ```
4. `get data-migration-report` (final counts) → `end migration`.

### Fall-forward vs fall-back (later scope; command deltas)

- **Plain live**: one-way; `--prepare-for-fall-back false`; no reverse flow.
- **Fall-forward** (YB primary, keep a source-replica in sync): additional `import data to source-replica --source-replica-db-* ...` daemon; switchover via `initiate cutover to source-replica`.
- **Fall-back** (keep the original source in sync): first cutover with `--prepare-for-fall-back true`; before it, `setup_fallback_environment` disables triggers + drops FKs on the source; reverse via `initiate cutover to source`; re-enable triggers after.

### Resumption / state-transition testing

To exercise resumability (a QA priority): kill the `export data` / `import data` daemon mid-run and relaunch the identical command — it must resume from checkpoint without data loss or duplication. The `migtests/scripts/event-generator/generator.py` produces continuous weighted INSERT/UPDATE/DELETE traffic (op weights INSERT:3 UPDATE:2 DELETE:1, deterministic seeds) for driving load during resumption tests.

## Config-file mode (must match flag mode)

Every command accepts `-c <config.yaml>` instead of flags. Generate one from the templates:
```
python3 migtests/scripts/generate_voyager_config_file.py \
  --template migtests/scripts/config-templates/offline-migration.yaml \
  --output <dir>/generated-config.yaml
"$BIN" import data -c <dir>/generated-config.yaml --yes
```
Config YAML shape: top-level `export-dir`; `source:` / `target:` blocks with `db-host/db-port/db-name/db-user/db-password` (hyphenated, no `--`); per-command override blocks (`export-data:`, `import-data:` with e.g. `adaptive-parallelism: balanced|disabled`, `export-data-from-source: export-type: snapshot-and-changes`, `initiate-cutover-to-target: prepare-for-fall-back`). A feature that adds/changes a flag **must** be tested in both modes — a common miss.
