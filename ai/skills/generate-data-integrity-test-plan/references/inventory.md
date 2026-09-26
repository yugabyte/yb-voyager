# Live inventory (rebuilt every run)

Nothing that changes with a release is hardcoded in these skills. Flags, config keys, env knobs, guardrails, target-DDL support and framework entry points are **derived at the start of every run** from the target commit and the binary built from it, and written into the plan's `inventory` section. The catalogs in `dimensions.md` and `silent-loss-mechanisms.md` are *seeds and heuristics*; when they disagree with the inventory, the inventory wins and the disagreement is reported as drift.

Run everything against the worktree at the plan's `target_commit` and the `yb-voyager` binary built from it.

## 1. Flags

```bash
for c in "export data" "export data from target" "import data" "import data to source" \
         "import data to source-replica" "initiate cutover to target" "initiate cutover to source" \
         "archive changes" "end migration"; do
  yb-voyager $c --help 2>&1 | sed -n '/^Flags:/,/^Global Flags:/p'
done
```

Parse flag name, type and default. Also collect config-file keys from `yb-voyager/config-templates/*.yaml` and the allowed-key sets in `yb-voyager/cmd/config.go`.

- `inventory.flags`: `{command, flag, type, default}`.
- **Uncatalogued** = flags in the inventory but not mentioned in `dimensions.md` → Flags. If the change set touches an uncatalogued flag (its name appears in the diff), generate cases for it; always list uncatalogued flags in the plan summary and the hunt report.
- **Stale** = flags mentioned in `dimensions.md` but absent from `--help` *and* no longer registered in code. Hidden flags (`mustMarkFlagHidden`) don't appear in `--help`, so before reporting a flag as stale, `grep -rn --include='*.go' -e '"<flag>"' yb-voyager/cmd`; if it's registered and hidden, record it as `hidden` in `inventory.flags` instead. Report truly stale flags as catalog drift; don't generate cases for them.

## 2. Env knobs

`grep -rn --include='*.go' -E 'GetEnvAs(Int|Bool|String)|os\.Getenv\(' yb-voyager/cmd yb-voyager/src | grep -v _test.go` → `inventory.env`. Treat as internal/testing-only unless documented; include them only as test levers (e.g. small `NUM_EVENT_CHANNELS` to raise collision rates).

## 3. Guardrails (what voyager refuses up front)

Derive from code, not memory: in `yb-voyager/cmd/` find the pre-flight validations reached from `export data` and `import data` start (validation functions called before snapshot/streaming, `guardrails*` files, `validate*` functions) and extract each refusal message and its condition. Useful starting searches:

```bash
grep -rn -E 'not allowed|not supported|requires|cannot proceed|refus' yb-voyager/cmd/*.go | grep -v _test.go
grep -rln 'guardrail' yb-voyager/cmd yb-voyager/src
```

- `inventory.guardrails`: `{condition, message, file:line, stage: export|import}`.
- Cases whose configuration matches a guardrail get `expect: refused`. A guardrail that disappeared since the last run is itself worth a regression case (a refusal that stops refusing can re-open a silent-loss path).

## 4. Target DDL support

Don't keep a list of "DDL YugabyteDB can't create" — it changes per YB release. Every case with non-trivial DDL (types in indexes/PKs, deferrable constraints, exclusion constraints, partitioning variants, generated columns) is `ddl_probe: true`; the hunt probes it on the YB image the tests use and records the result in its report.

## 5. Framework entry points

```bash
grep -n '^func (lm \*LiveMigrationTest) [A-Z]' yb-voyager/src/testlivemigration/live_migration_testing_framework.go
grep -n '^type \(TestConfig\|ContainerConfig\|ChangesCount\)' -A 25 yb-voyager/src/testlivemigration/live_migration_testing_framework.go
```

→ `inventory.framework`. Cases' `phases` must map onto methods that exist here; the flow table in `dimensions.md` is a hint, the inventory is authoritative.

## 6. Code anchors

The references name code by **stable strings first** (log messages, error text, SQL shapes, flag names) and function names second. For each anchor used by the selected mechanisms, `grep -rn` it in the target commit:

- found → record `file:line` in `inventory.anchors`
- missing → search for the behaviour string instead, use what it finds, and report the stale anchor as drift

Anchors used today (update when the references change): `unexpected rows affected`, `ON CONFLICT (`, `conflict detected: event`, `duplicate key value`, `customKeyNullSentinel`, `added primary key .* as synthetic unique index`, `GetEventPartitionKey`, `hashEvent`, `GetEffectiveTableName`, `queryPGPrimaryKeyColumnsByCatalog`, `GetTableToUniqueIndexesMap`, `dedupeUniqueIndexes`.

## Drift report

Both skills end with a "Catalog drift" section: uncatalogued flags, stale flags, stale anchors (and their replacements), guardrails added/removed since the previous plan (if one is available), and templates that failed to compile. In unattended mode the hunt may open **one** separate PR updating the reference files with these corrections — never mixed into a bug PR.
