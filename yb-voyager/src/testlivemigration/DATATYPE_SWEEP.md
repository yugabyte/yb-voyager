# The PostgreSQL → YugabyteDB datatype sweep

A repeatable, self-verifying test suite that answers one question per datatype, per
migration mode:

> If a user puts a value of this type in a column, what does voyager actually do with it?

The answer is a **verdict** drawn from a fixed vocabulary, backed by evidence, emitted as
a machine-readable row. A release-to-release diff of two runs then answers "what changed
in datatype support?" mechanically rather than by re-reading a wiki page.

The published datatype report is a **view over this suite**, not a parallel document: its
rows are generated from a run's measurements plus per-type metadata derived from
voyager's own source. Two tests make the loop a build failure rather than a review
question — see [Coverage: the round trip](#coverage-the-round-trip).

---

## Quick start

```bash
# From the repo root. Coverage guard + offline + live, ~40 min, needs Docker.
migtests/scripts/run-datatype-sweep.sh

# Everything, including fall-back and fall-forward (hours).
migtests/scripts/run-datatype-sweep.sh all

# One mode.
migtests/scripts/run-datatype-sweep.sh offline
migtests/scripts/run-datatype-sweep.sh live

# One probe, on its own (the only safe way to run a poison probe).
migtests/scripts/run-datatype-sweep.sh probe HSTORE-001 LIVE

# Only the catalogue coverage guard (fast, one PostgreSQL container, no YugabyteDB).
migtests/scripts/run-datatype-sweep.sh coverage
```

Every invocation ends by writing, under `datatype-sweep-results/` (override with
`RESULTS_DIR`):

| File | What it is |
| --- | --- |
| `run-<stamp>.log` | the raw `go test` output |
| `datatype-sweep-<stamp>.csv` | one row per **(probe, mode)** — the diffable artefact |
| `probe-catalog.json` | one entry per probe — the report's static, per-type half |
| `report-rows.json` / `.csv` | the published report's row data, generated from the two above |

---

## Layout

| File | Role |
| --- | --- |
| `datatype_sweep_cases.go` | **the case table** — one `datatypeProbe` per audit row |
| `datatype_sweep_probe.go` | the runner: DDL, deltas, comparison, verdict classifier |
| `datatype_sweep_test.go` | entry points: one test per mode, plus the single-probe runner |
| `datatype_coverage_test.go` | **the coverage guard** and the report round-trip assertion |
| `datatype_sweep_cases_postgis_internal.go` | the `postgis-internal` batch: the 17 gaps the coverage guard found (pending registration) |
| `datatype_report_meta.go` | the probe catalog: per-type report columns, derived from voyager's own lists |
| `datatype_report_test.go` | emits `probe-catalog.json`; unit-tests the derivations |
| `sweepreport/` | standalone tool: `collect`, `report`, `diff` (no build tag, no Docker) |

---

## Verdict vocabulary

Worst to best. The first seven are product verdicts; the last two are facts about the
harness or the environment and are never published as product findings.

| Verdict | Plain English | What it means operationally |
| --- | --- | --- |
| `SILENT_LOSS` | **Value silently lost.** | The column exists on the target but the value is gone (or NULL) and nothing warned. The worst outcome: a user discovers it in production. |
| `SILENT_WRONG` | **Wrong value.** | The value arrived but is not the value that was written — a truncation, a timezone shift, a re-formatted array. Also silent. |
| `QUIET_DROP` | **Column dropped.** | Voyager excluded the column from the migration and did not tell the user in a way they would notice. Data loss with a paper trail nobody reads. |
| `STUCK` | **Importer stops / exporter crashes.** | The value wedges the pipeline: the importer crash-loops on the batch, or the exporter dies. Migration does not complete. One such value blocks every later event in its channel. |
| `BLOCKS` | **Migration refuses to proceed.** | Voyager stops up front with a clear error. Bad, but honest and actionable. |
| `EXCLUDED_TOLD` | **Column excluded, user told.** | The guardrail dropped the column *and* reported it. The documented, intended behaviour for an unsupported type. |
| `WORKS` | **Works.** | Snapshot and every delta operation round-tripped byte-for-byte, and the column was present in the event stream. |
| `SKIPPED` | **Not tested.** | The probe could not be set up at all: the extension is missing on this image, or the server rejected the probe's DDL or literal. No claim is made. |
| `INCONCLUSIVE` | **Inconclusive.** | The run did not actually exercise the probe, so neither a pass nor a failure can be claimed. Exists because an empty observation must never read as a clean `WORKS`. |

The report adds one label of its own, `NOT_TESTED`, for a probe that exists but that the
run being reported did not measure (e.g. a fall-back column in an offline-only run).

### Why `INCONCLUSIVE` exists

A framework wait that calls `t.Fatalf` unwinds the goroutine via `runtime.Goexit`, so the
comparison and event-scanning steps never run — while the deferred emitter still fires.
Without an explicit "nothing was measured" state, that produced a confident `WORKS` for a
run that measured nothing. `TestSweepClassifierRequiresEvidence` pins the fix.

---

## The control gate — read this before recording any result

Two known-good probes, `CTRL-001` (`int`) and `CTRL-002` (`text`), are prepended to
**every** batch. They must come out `WORKS`.

**If a control does not come out `WORKS`, the entire run is invalid and none of its
verdicts may be recorded** — not just the control's own row. A broken control means the
harness, the containers or the environment is wrong, so every other verdict in that batch
is measuring something other than the product.

The harness prints:

```
PROBE-RUN-INVALID: <batch> | <mode> | known-good control CTRL-001 came out STUCK, not WORKS ...
```

and `sweepreport collect` marks every row from that batch `run_status=INVALID`. The differ
excludes such rows from both sides, so an invalid run cannot manufacture a phantom
regression. The runner script greps for the line and shouts about it at the end.

Related markers:

- `PROBE-RUN-FLAKE` — the export never reached streaming mode, or probes came out
  `INCONCLUSIVE`. **Re-run before recording anything**; a flake must reproduce before it
  is a finding.
- `PROBE-RUN-POISON` — the run deliberately contains a poison probe, so a wedged control
  is expected collateral and the gate does not apply.
- `PROBE-RUN-EXCLUDED` — a known-poison probe was left out of a batch; run it solo.

Every classifier bug found so far — the false `WORKS` on an unmeasured run, the empty-error
`STUCK`, the config-field `STUCK` — showed up **first** as a control coming out wrong.

---

## Poison probes

Some values crash-loop the import channel. Because the channel is ordered, one such value
blocks every later event in its segment, so a poison probe in a batch destroys the batch's
other verdicts too. The runner therefore **refuses to put a `Poison` probe in a batch** and
prints a `PROBE-RUN-EXCLUDED` line saying how to run it:

```bash
migtests/scripts/run-datatype-sweep.sh probe HSTORE-001 FALL-BACK
```

A solo run uses a shorter streaming timeout: with one probe under test, a genuine stall is
obvious in minutes.

---

## Coverage: the round trip

Two tests, together, make "we report on exactly what we test" a build failure.

### 1. Every catalogue type has a probe — `TestDatatypeCatalogCoverage`

This test does **not** consult a hand-written list of types. It:

1. installs every extension the suite cares about, so extension types are held to the same
   standard as built-ins;
2. asks the live `pg_type` catalogue for every candidate a user could declare a column as
   — `typisdefined`, `typtype IN ('b','e','d','r','m','c')`, array types skipped (an array
   is covered by its element type's probe), table row types skipped, scope limited to
   `pg_catalog`, `public` and anything owned by an installed extension;
3. for each candidate, **empirically** attempts `CREATE TABLE scratch(v <type>)`;
4. maps probes onto catalogue OIDs by asking the server to resolve each probe's
   `ColumnDDL` with `to_regtype` — which is why `int` covers `int4`,
   `timestamp with time zone` covers `timestamptz` and `public.geometry` covers the
   PostGIS type, with no alias table to maintain;
5. **fails, listing every column-able type with no probe.**

There is exactly one exclusion rule:

> A type is excluded **only** if `CREATE TABLE` actually fails, and the exclusion carries
> the server's verbatim error.

No exclusions on category grounds — not "index support type", not "internal", not
"statistics type". Those judgements have been wrong repeatedly: `aclitem`, `pg_node_tree`,
`pg_ndistinct`, `pg_mcv_list`, `pg_dependencies`, `pg_brin_bloom_summary`,
`pg_brin_minmax_multi_summary`, `gtsvector`, `ghstore`, `gtrgm`, the `gbtreekey` family,
`ltree_gist`, `intbig_gkey`, `query_int`, `earth` and the `tablefunc_crosstab` types all
accept a column in PG 17, and `query_int`, `earth` and `tablefunc_crosstab_2` store real
values. Deriving the exclusion set at run time means it cannot drift from reality.

For each type that needs a probe the guard also reports whether to write a **full value
probe** or a **NULL-only probe**, by trying a NULL insert and a short list of
representative literals. That classification is advice for the probe author, never a gate.

The one hand-maintained map, `deliberateNonMigrationTypes`, is for a genuinely different
case: a type that exists, accepts a column, and that the suite consciously does not
migrate **for a stated product reason**. Every entry carries a written justification and
the whole map is printed on every run. It is empty on purpose, and the intent is that it
stays that way.

It stayed empty on its first real test. Run against the `17.8-ext` image the guard failed
with 17 missing PostGIS/raster/topology internal types (`box2df`, `gidx`, `spheroid`,
`geometry_dump`, `geomval`, the `postgis_raster` composites, the `topology.*` types).
"PostGIS helper types are not real columns" is the same shape of category judgement that
the empirical rule exists to prevent, so they got probes instead — see the
`postgis-internal` batch in `datatype_sweep_cases_postgis_internal.go`. Their verdict comes
out as the *target* rejecting the type, because YugabyteDB cannot install PostGIS at all;
that is a measurement, not an assumption.

For each missing type the guard also prints the literal it managed to insert, so upgrading
a NULL-only probe to a full-value one is a copy-paste. Composite types get an arity-derived
all-NULL-fields row literal (`(,)`), which is a genuine non-NULL value — without it every
composite of arity ≥ 2 was misreported as NULL-only, an artifact of the harness presented
as a fact about the type.

While extending the case table, `SWEEP_COVERAGE_MODE=report` prints the gaps without
failing.

### 2. Every probe appears in the report — `TestDatatypeReportCoversEveryProbe`

Needs no container. It asserts that `buildProbeCatalog()` produces exactly one entry per
probe, each with a type name, a group, and non-empty values in all five reporting-layer
columns.

---

## How the published report is generated

```
case table  ──►  probe-catalog.json   (per-TYPE columns; derived, container-free)
                        │
run log     ──►  results CSV          (per-RUN columns; verdict + evidence)
                        │
                        ▼
                 report-rows.json / .csv
```

The per-type columns are **computed at run time from voyager's own variables**, so editing
one of those lists changes the report on the next run instead of leaving a hand-typed
string behind:

| Column | Derived from |
| --- | --- |
| `reported_by_assess` | `srcdb.PostgresUnsupportedDataTypes`, `GetPGLiveMigrationUnsupportedDatatypes()`, `GetPGLiveMigrationWithFFOrFBUnsupportedDatatypes()` |
| `reported_by_analyze` | the same three lists (`queryissue` runs the same classification) |
| `guardrail_action` | `srcdb.PostgresUnsupportedDataTypesForDbzm` + the runtime `typtype='r'` user-defined-range filter in `PostgreSQL.GetColumnsWithSupportedTypes` |
| `guardrail_action_fallback` | `srcdb.GetYugabyteUnsupportedDatatypesDbzm(false)` and the gRPC list, for `export data from target` |
| `reported_by_docs` | **hardcoded** (`docsUnsupportedTypes`), with the doc URL beside it — the only column that cannot be derived |

The matching semantics are copied from the product deliberately, warts included. The
guardrail matches `pg_type.typname`, which for an array column is the **array** type's own
name (`_xml`, not `xml`), so an array of an unsupported type is reported as *not* excluded
by the guardrail even though assess-migration, which strips the `[]`, does flag it. That
disagreement is a finding about voyager, surfaced rather than smoothed over.

If a result row has no catalog entry, or a catalog entry has no measurement in a required
mode, `sweepreport report` says so; with `-strict` it exits non-zero.

---

## The results CSV

One row per **(probe, mode)**. Columns are append-only, so an old file stays readable by a
newer differ.

| Column | Meaning |
| --- | --- |
| `run_timestamp`, `voyager_commit`, `pg_version`, `yb_version` | provenance of the run |
| `probe_id` | stable audit id, e.g. `RANGE-009` |
| `type_name` | human label of the type under test |
| `category` | the batch / group the probe belongs to |
| `mode` | `OFFLINE` / `LIVE` / `FALL-BACK` / `FALL-FORWARD` |
| `verdict` | the vocabulary above |
| `evidence` | the classifier's reason: the diff, the repeated import error, the warning text |
| `source_value`, `target_value` | verbatim value on each side, when the harness recorded one |
| `run_status` | `OK` / `INVALID` / `FLAKE` / `POISON` — **anything but `OK` must not be published** |

### Reading a diff

```bash
cd yb-voyager
go run ./src/testlivemigration/sweepreport diff \
    -old results/datatype-sweep-2026-06.csv \
    -new results/datatype-sweep-2026-08.csv
```

```
REGRESSIONS (1)
  RANGE-009  CREATE TYPE AS RANGE  LIVE  WORKS -> QUIET_DROP  (column absent from the event stream)

IMPROVEMENTS (2)
  HSTORE-001 hstore                LIVE  STUCK -> WORKS
  ...
```

- **REGRESSIONS** — the verdict moved *down* the vocabulary. Gate a release on this.
- **IMPROVEMENTS** — the verdict moved *up*. Release-note material.
- **COVERAGE LOST** — a (probe, mode) that used to be measured no longer is, or moved into
  `SKIPPED`/`INCONCLUSIVE`. Treated as seriously as a regression: it usually means an
  extension went missing or a probe silently stopped running.
- **COVERAGE GAINED** — newly measured.

Moves into or out of `SKIPPED`/`INCONCLUSIVE` are never reported as regressions or
improvements: they are not product verdicts. Rows with `run_status != OK` are dropped from
both sides before comparing, and the count of dropped rows is printed.

`-fail-on-regression` makes the command exit non-zero, which is what a CI gate should use.

---

## Adding a new datatype probe (under five minutes)

1. Pick the batch in `datatype_sweep_cases.go` (`rangeProbes`, `coreScalarProbes`, …) or
   add a new one and register it in `sweepBatches()`.
2. Append an entry. **Never renumber an existing id** — ids are the audit's primary key.

```go
{
    ID: "CORE-042", Name: "macaddr8", TypeName: "macaddr8",
    ColumnDDL:    "macaddr8",
    InitialValue: "'08:00:2b:01:02:03:04:05'::macaddr8",
    AltValue:     "'02:00:00:00:00:00:00:01'::macaddr8",
},
```

   - `InitialValue` seeds the snapshot; `AltValue` is what the insert and the
     "update this column" delta write. They **must** differ, or the update proves nothing
     (`assertUniqueProbeIDs` enforces this).
   - Needs a type the probe creates? Put the DDL in `PreDDL` and name it with the `{{p}}`
     prefix so two probes can never collide: `CREATE TYPE {{schema}}.{{p}}_c AS (...)`.
   - Needs an extension? List it in `Extensions`. If it will not install, the probe
     self-reports `SKIPPED` instead of taking the batch down.
   - Text form not stable across servers (e.g. `money`, which depends on `lc_monetary`)?
     Set `CompareExpr`.
   - Want the exact bytes in the report even on a pass? Set `RecordDestValue`.
   - Known to wedge the channel? Set `Poison` **and** `PoisonNote` saying which mode
     established it and with what error.
   - Type can only ever hold NULL (a GiST index support type such as `box2df`)? It needs
     the `NullOnly` field, which is **not implemented yet** — see the header of
     `datatype_sweep_cases_postgis_internal.go`. A NULL-only column has exactly one
     possible value, so `assertUniqueProbeIDs`' "InitialValue must differ from AltValue"
     rule is unsatisfiable for it and must exempt such probes. Do not work around it by
     inventing a second spelling of NULL.

3. Run it alone:

```bash
migtests/scripts/run-datatype-sweep.sh probe CORE-042 LIVE
```

4. Confirm the guards still pass — these need no Docker:

```bash
cd yb-voyager
go test -tags integration_live_migration ./src/testlivemigration/ \
    -run 'TestDatatypeReportCoversEveryProbe|TestProbeGroupsCoverEveryProbe|TestDatatypeSweepCatalog'
```

---

## Environment traps

These have each cost a day. The runner script handles all of them; if you invoke `go test`
directly, handle them yourself.

| Trap | Symptom | Fix |
| --- | --- | --- |
| **Expired Teleport kubeconfig** | The Debezium JVM dies at startup with no error in any voyager log. Presents as a flaky run that exports zero events — an ANSI escape byte in `~/.kube/config` breaks snakeyaml before the connector starts. | `export KUBECONFIG=$(mktemp)` — point it at an empty file. |
| **An older installed `yb-voyager` on PATH** | Verdicts look plausible but describe someone else's build. | Build from the worktree and put it **first** on `PATH`. The script does this; `SKIP_BUILD=1` opts out. |
| **`PG_VERSION` newer than the local `pg_dump`** | `export data` fails immediately: pg_dump refuses a newer server. | Pin `PG_VERSION` to a patch not newer than `pg_dump --version`. |
| **Missing `DEBEZIUM_DIST_DIR`** | Live / fall-back / fall-forward cannot start. | Point it at the built Debezium server distribution. |
| **Stale lockfiles** | Re-runs refuse to start after a killed process. | Delete `<exportDir>/.*Lockfile.lck`. |
| **Concurrent sweeps** | Two runs fight over which database the shared container singleton points at. | Never run two sweeps at once; the tests deliberately do not call `t.Parallel()`. |
| **Plain `postgres:17` image** | Every postgis/pgvector probe reports `SKIPPED`. | Correct behaviour, not a failure. Use `PG_VERSION=17.8-ext` for the extension image. |

---

## Running the pieces by hand

```bash
cd yb-voyager

# One batch of one mode.
go test -tags integration_live_migration ./src/testlivemigration/ \
    -run 'TestDatatypeSweepLive/ranges' -timeout 3h -v

# One probe.
PROBE_ID=HSTORE-001 PROBE_MODE=FALL-BACK \
  go test -tags integration_live_migration ./src/testlivemigration/ \
  -run TestDatatypeSweepSuspect -timeout 1h -v

# Just the verdict lines from any run.
go test ... | grep '^PROBE-RESULT:'

# The container-free guards and derivations.
go test -tags integration_live_migration ./src/testlivemigration/ \
    -run 'TestDatatypeReportCoversEveryProbe|TestProbe|TestClassifyCoverage|TestReportingColumns'

# The report tooling's own unit tests (no build tag at all).
go test ./src/testlivemigration/sweepreport/
```

---

## Exit codes

`run-datatype-sweep.sh` exits non-zero if **any** `go test` invocation it ran failed, and
prints a `FAILED (n):` summary naming them — after writing the artefacts, so a failed run
still leaves its log, results and report behind.

A failing mode does not stop the later modes: a sweep's output *is* its verdicts, and a
mode that fails still produced them. But the failure is recorded and propagated rather
than swallowed. Both halves matter, and getting either wrong is the same bug:

- swallow the exit code and a coverage guard whose entire purpose is to fail reports
  success — the per-PR job goes green on exactly the gap it exists to catch;
- abort on the first failure and you lose the verdicts from every later mode, plus the
  summary that says why.

The nightly workflow advances its diff baseline **only after a clean diff**, so a
regression keeps failing every night until someone fixes it rather than being absorbed
into the new normal after one report.

## CI

See `.github/workflows/datatype-sweep.yml`. It is **manual dispatch + nightly**, never
per-PR: the cheap tests are seconds, but a full live sweep is tens of minutes and a
fall-back sweep is hours.

| Job | Runtime | Cadence |
| --- | --- | --- |
| catalog + round-trip + `sweepreport` unit tests | seconds, no Docker | **safe per-PR** |
| coverage guard (one PG container) | ~2 min | **safe per-PR** |
| offline sweep | ~35 s per batch, ~15 min total | nightly |
| live sweep | ~60–90 s per batch, ~30 min total | nightly |
| fall-back sweep | ~16 min per batch | weekly / on demand |
| fall-forward sweep | ~16 min per batch | weekly / on demand |
| poison probes | one container run each | on demand, isolated |

The nightly job uploads the results CSV and the generated report rows as artifacts, and
diffs against the previous night's CSV so a regression shows up as a failing step rather
than as a file somebody has to open.
