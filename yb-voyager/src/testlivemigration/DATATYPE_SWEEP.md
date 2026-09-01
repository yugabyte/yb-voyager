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

Worst to best. The first eight are product verdicts; the last two are facts about the
harness or the environment and are never published as product findings.

| Verdict | Plain English | What it means operationally |
| --- | --- | --- |
| `SILENT_LOSS` | **Value silently lost.** | The column exists on the target but the value is gone (or NULL) and nothing warned. The worst outcome: a user discovers it in production. |
| `SILENT_WRONG` | **Wrong value.** | The value arrived but is not the value that was written — a truncation, a timezone shift, a re-formatted array. Also silent. |
| `QUIET_DROP` | **Column dropped.** | Voyager excluded the column from the migration and did not tell the user in a way they would notice. Data loss with a paper trail nobody reads. |
| `EXPORTER_CRASHES` | **Export fails.** | `export data` dies — usually the Debezium connector, at startup, before a single row is read. **Nothing** migrates, and `initiate cutover` then waits forever. |
| `STUCK` | **Import fails.** | The value wedges the import channel: the importer crash-loops on the batch and cannot get past it. One such value blocks every later event in its channel. |
| `BLOCKS` | **Migration refuses to proceed.** | Voyager stops up front with a clear error. Bad, but honest and actionable. |
| `EXCLUDED_TOLD` | **Column excluded, user told.** | The guardrail dropped the column *and* reported it. The documented, intended behaviour for an unsupported type. |
| `WORKS` | **Works.** | Snapshot and every delta operation round-tripped byte-for-byte, and the column was present in the event stream. |
| `SKIPPED` | **Not tested.** | The probe could not be set up at all: the extension is missing on this image, or the server rejected the probe's DDL or literal. No claim is made. |
| `INCONCLUSIVE` | **Inconclusive.** | The run did not actually exercise the probe, so neither a pass nor a failure can be claimed. Exists because an empty observation must never read as a clean `WORKS`. |

The report adds one label of its own, `NOT_TESTED`, for a probe that exists but that the
run being reported did not measure (e.g. a fall-back column in an offline-only run).

### `STUCK` and `EXPORTER_CRASHES` are a pair, and must never collapse

They are the same shape — the migration does not complete — and the only difference is
which process dies. That difference is the whole finding:

- **`STUCK`** means a value could not be **applied**. It was produced, exported and
  delivered; the target refused it. Everything ahead of it in the channel did migrate.
- **`EXPORTER_CRASHES`** means nothing was ever **produced**. There is no event stream at
  all, so nothing migrated and `initiate cutover` hangs.

`EXPORTER_CRASHES` therefore ranks *below* `STUCK` in the differ, and a type moving from
one to the other is a real change rather than noise. `DOM-005` (`domain(enum)`) is the
established instance: the Debezium connector throws a `NullPointerException` in
`TypeRegistry.prime` while building its type registry, at startup.

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
- `PROBE-RUN-QUARANTINE` — a probe was caught crash-looping the import channel *during*
  the run (see [Failure is detected, not waited out](#failure-is-detected-not-waited-out)).
  It names the probe, quotes the error, and prints the solo command. Its batch-mates are
  collateral: their events were stuck behind it, so they come out `INCONCLUSIVE` and the
  batch has to be re-run without the named probe.
- `PROBE-RUN-EXPORT-DIED` — the export side died during the run. It quotes the cause and,
  when the failure names exactly one probe, names the culprit and prints the solo command.
  An exporter that dies at startup takes the **whole** run down, so every other probe is
  collateral and comes out `INCONCLUSIVE`; re-running the batch unchanged only reproduces
  it.
- `PROBE-PUBLISHABLE` — the one row that survives its run's failed gate: an attributed
  export death. See [The one carve-out](#the-one-carve-out-an-attributed-export-death).
- `PROBE-WAIT` — one line per bounded wait: how long it took, out of what budget, and why
  it ended (`counts-satisfied` / `repeating-error` / `exporter-died` / `no-output` /
  `timeout`).

Every classifier bug found so far — the false `WORKS` on an unmeasured run, the empty-error
`STUCK`, the config-field `STUCK` — showed up **first** as a control coming out wrong.

### The one carve-out: an attributed export death

The gate exists to catch a **broken measurement**. An attributed `EXPORTER_CRASHES` is not
one — it *is* the finding, and the controls coming out `INCONCLUSIVE` is a **consequence**
of it (the exporter they needed was dead) rather than evidence against it. Requiring a
human to promote that row by hand would make the audit's most severe verdict the only one
that cannot be recorded automatically.

So the harness emits

```
PROBE-PUBLISHABLE: <id> | <mode> | EXPORTER_CRASHES | <why>
```

and `sweepreport collect` marks **that row only** `run_status=OK`, overriding the run's
`INVALID`. Every clause is load-bearing (`publishableReason`, and
`TestExportDeathVerdictIsPublishable`):

- the probe must **be** the attributed culprit — a batch-mate was never measured;
- the verdict actually reached must be `EXPORTER_CRASHES` — the marker never promotes a
  verdict the classifier did not produce, and the parser re-checks the verdict on the row
  before honouring it;
- there must be a quotable cause — no evidence, no publication.

An **unattributed** death promotes nothing. The run-level `PROBE-RUN-EXPORT-DIED` line is
the record and a human writes that row from it: one row written by hand beats a wrong row
written automatically.

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

A probe may pin any of the first three, plus the docs column, with the optional
`ReportedByAssess` / `ReportedByAnalyze` / `GuardrailAction` / `ReportedByDocs` fields.
**Empty means derive**, which is right for almost every probe. A pinned cell is rendered
with a `[pinned by the probe, not derived]` suffix, so a stale hand-written string can
never masquerade as a live derivation — which is the failure this generation step exists
to remove. Only pin one when the name-based derivation is demonstrably wrong, and say why
in `Note`.

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
| `source_value`, `target_value` | verbatim value on each side, from the harness's own `PROBE-VALUES:` line (a structured field, not scraped out of the prose evidence). Pipes, newlines and tabs are escaped reversibly so a value containing one is recorded rather than rewritten |
| `run_status` | `OK` / `INVALID` / `FLAKE` / `POISON` — **anything but `OK` must not be published**. One row per run can be `OK` inside an `INVALID` run: an attributed export death, promoted by its `PROBE-PUBLISHABLE` line |
| `sqlstate` | the SQLSTATE from the importer error, when there was one. `import data` failures carry the real database error (`importAbortReason`), so a diff can report "same verdict, different SQLSTATE" — the shape a fix that only *moved* the error takes |

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

- **SAME VERDICT, DIFFERENT SQLSTATE** — the outcome did not change but the reason did.
  Reportable, never a gate.

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
| **A memory-starved host** | Every live run times out with failing controls, so whole batches are discarded — it looks exactly like a product stall. Observed at 21 MB free with 3.1 GB swapped: the Debezium JVM is starved and never reaches streaming mode. Five batches were lost to this before it was recognised. | Check free memory and swap BEFORE a live run. Verdicts from a starved host are worthless; the control gate will (correctly) mark the run `INVALID`, so do not record them. |
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

## Failure is detected, not waited out

This used to be the suite's biggest weakness, and the section is kept as the record of
what it cost and what was done about it.

Every wait was *positive-signal*: it waited for the expected event counts to arrive and
could only conclude when the clock ran out. So the suite was fast when everything worked
and pathologically slow exactly when it found something — the cost of a run scaled with
the number of *failures*, which is backwards for a tool whose purpose is finding them.

| Situation | Was | Now |
| --- | --- | --- |
| A healthy probe | ~50 s | ~50 s (unchanged: the positive signal still wins) |
| A failing probe, solo run | **240 s** (`sweepSoloStreamingTimeout`) | **~4 s** |
| A failing probe, batched run | **900 s** (`sweepStreamingTimeout`) | **~4 s** |
| A batch poisoned by one wedged value | ~48 min, then **discarded entirely** | seconds, and the culprit is named |
| A stall that logs nothing | the full budget | the full budget — **deliberately unchanged** |

### The negative signal

**A wedged importer does not go quiet.** It retries the same batch and logs the *same
error* every few seconds. That is a signal that arrives in seconds, unlike a count that
will never arrive at all — so `waitBounded` now watches for it alongside the counts.

`waitForSignalOrCrashLoop` (in `datatype_sweep_probe.go`) polls every 2 s and concludes
early when **all three** hold:

1. the same import-failure signature has been logged `sweepCrashLoopRepeats` (3) times;
2. it was still the most repeated signature `sweepCrashLoopPolls` (2) polls running;
3. the observed counts did not advance across those polls.

All three are load-bearing. The repeat count alone fires on a transient error the importer
then gets past; frozen counts alone are just a slow pipeline; and the *signature* test is
`isImportFailureSignature`, the same function the post-hoc evidence uses — so a spew dump
or a config field whose value merely contains the word `ERROR` can no more terminate a
wait than it can produce a `STUCK` verdict. The evidence rules are unchanged: a failure
verdict still requires a quotable error with a real signature, and zero events with no
error is still `INCONCLUSIVE`.

The counts come from one `get data-migration-report` per poll, fingerprinted by
`reportFingerprint`, which the completion predicate and the progress check share
(`snapshotComplete` / `streamingComplete` were split out of the framework's
`snapshotPhaseCompleted` / `streamingPhaseCompleted` for exactly this, so the sweep reuses
the framework's definition of "complete" instead of drifting from a copy of it).

**The long timeout is kept, and means something different.** A wait that expires with
nothing quotable in the log is a stall that logged *nothing* — an environment fact, not a
datatype verdict. It classifies `INCONCLUSIVE`, and the two outcomes must never collapse
into one label (`TestWaitOutcomesStayDistinguishable`).

### The second negative signal: a dead exporter

The crash-loop detector watches the **import** log, which is also the only place the
post-hoc evidence used to look. That left the export side with no detector at all, and it
hid the most severe outcome in the audit behind the blandest label in the vocabulary.

When `export data` dies it prints exactly one thing —
`Export of data failed! Check <dir>/logs for more details.` — and exits. From the import
side that is indistinguishable from a healthy run in which nothing happened: zero events,
no repeating importer error, no value difference to compare. So it classified
`INCONCLUSIVE`, meaning "the run was healthy and nothing conclusive happened", for a run in
which *nothing would have migrated*.

So every wait now also polls `<export-dir>/logs/debezium-<role>.log` and
`yb-voyager-export-data.log`, and every place that was about to conclude "nothing
happened" reads the export side first (`exportDiedDuring`). The rules mirror the import
side's exactly:

- **A terminal marker is required.** `Connector completed: success = 'false'`,
  `Unable to initialize and start connector's task class`, or `Export of data failed`. A
  Java exception on its own is *not* enough — Debezium logs exceptions it then recovers
  from, and a verdict off one of those would record a guess as a finding.
- **The cause is quoted, narrowed rather than truncated.** Debezium's `ConnectorLifecycle`
  line carries the throwable at the very end, behind the whole connector config, so the
  evidence is cut from the exception's class name onwards. Truncating instead would throw
  the exception away *and* feed the config's `table.include.list` — which names every probe
  in the batch — into attribution.
- **No cause, no verdict.** `export data` failing with nothing usable in any log is a
  run-level abort whose reason says so in as many words (`exportAbortReason`).
- **Attribution is exact-one, as with quarantine.** If the failure names exactly one active
  probe (by table or by type) that probe gets `EXPORTER_CRASHES`; otherwise the finding is
  reported against the **run** and every active probe is `INCONCLUSIVE`. The real DOM-005
  `NullPointerException` names neither a table nor a type, so it lands in the second case.
- **Batch-mates are never a product verdict.** An exporter that dies at startup takes the
  whole run down, so its batch-mates were never measured: `INCONCLUSIVE` with "the exporter
  died before this probe was measured", never `WORKS` and never a failure of their own.
- **The import side is not shadowed.** A wedged importer's error was produced *by* this
  value reaching the target, so the import side has already measured that type; the export
  process dying afterwards does not erase it. `STUCK` wins whenever both are present.

The wait concludes on the first poll that sees a death — no repeat count, because a
connector that reported itself completed-with-failure does not un-complete — so a run
killed at export startup costs seconds instead of the full 900 s budget.

The run as a whole still fails its control gate — the collateral probes are `INCONCLUSIVE`
and `INCONCLUSIVE` is not `WORKS` — but an **attributed** `EXPORTER_CRASHES` row is
published anyway, via `PROBE-PUBLISHABLE`. See
[The one carve-out](#the-one-carve-out-an-attributed-export-death) for why, and for the
three conditions that keep it from becoming a way past the gate.

### The third hang: a poll that never returns

A wait loop is only as bounded as the slowest thing inside one poll, and one poll shells
out to `get data-migration-report`.

`VoyagerCommandRunner` hands the child an `io.MultiWriter` for stdout and stderr. Because
that is not an `*os.File`, `os/exec` creates an OS pipe and a copier goroutine, and
`Cmd.Wait()` then blocks until the pipe reaches **EOF** — which needs every descendant that
inherited the write end to exit, not just the process that was started. One wedged
grandchild (the Debezium JVM is the obvious candidate) blocks the reader indefinitely, in
`goroutine [IO wait]`, with the voyager process itself already dead. The framework already
documents the same hazard on `WaitForAsyncCompletion`.

When that happens inside a poll it takes the whole loop with it: no polling, no crash-loop
detection, no export-death detection, and — because `PROBE-WAIT` is only printed when a
wait *ends* — **not one line of output**. A `catalogstats` live batch spent 13 minutes
exactly that way.

Two changes bound it:

- **`boundedFetcher`** (`TestBoundedFetcherSurvivesAWedgedCommand`). At most one fetch is
  outstanding, so a wedged one leaks a single goroutine rather than one per poll. The first
  poll to exceed `sweepReportFetchBudget` (2 min) gives up and reports the counts as
  unreadable — which the loop already handles, since `satisfied(nil)` is false and the
  fingerprint becomes `<report-unavailable>`. Every later poll checks that same outstanding
  fetch **without blocking**, so the loop keeps its 2 s cadence instead of paying the
  budget again and again; and if the fetch ever completes the fetcher resets. The leaked
  goroutine logs through `t`, so the run drains it from `t.Cleanup`, which runs before the
  test is marked complete.
- **`waitNoOutput`** (`TestSweepWaitConcludesOnTotalSilence`). When *nothing* moves — not
  the counts, not the import log, not the export log — for `sweepSilenceGrace` (5 min), the
  wait concludes instead of sitting out the rest of its budget. It classifies
  `INCONCLUSIVE`: a pipeline that wrote nothing down says nothing about a datatype. The
  activity check **hashes** both logs rather than measuring them, because both are read as
  a 512 KiB tail — once a log passes that size its length stops changing while its content
  does not, and a length-only check would call a crash-looping importer silent
  (`TestSweepWaitSilenceDoesNotFireOnABusyPipeline`).

**What is still unbounded.** `Cmd.Wait()` is reached from more than the report fetch:
`GracefulStop` waits on `stopChan` with no bound *after* its SIGKILL, and a synchronous
`Run()` (offline's `export data` / `import data`) blocks in it directly. SIGKILL does not
reach the Debezium JVM, so both can still hang on the same pipe. Fixing that properly means
either giving the child an `*os.File` (a real pipe the harness closes itself) or killing
the whole process group — a change to shared framework code that every live test depends
on, so it wants its own PR and a real run behind it.

### Seeing the saving

Every wait prints its own accounting line, so the next person can read the saving instead
of trusting it:

```
PROBE-WAIT: ranges | LIVE | forward streaming | counts-satisfied | 31.9s of 900s | expected counts reached after 16 polls
PROBE-WAIT: json | LIVE | forward streaming | repeating-error | 2.0s of 900s | SQLSTATE 22P02: [import data] error executing batch on channel 3: ... repeated x5 with the observed counts frozen across 2 polls; concluded without waiting out the remaining 898s of the budget
PROBE-WAIT: domains | LIVE | forward streaming | exporter-died | 0.0s of 900s | the export side is dead - Connector completed: success = 'false' - java.lang.NullPointerException: Cannot invoke "java.sql.Array.getArray()" ...; no event can arrive from a dead exporter, so the remaining 900s of the budget was not waited out
PROBE-WAIT: geo | LIVE | forward streaming | timeout | 900.0s of 900s | budget exhausted with no repeating importer error in the log: a stall that logged nothing, which is an environment fact and not a datatype verdict
```

`run-datatype-sweep.sh` greps them into a "wait accounting" block at the end of a run.

### Quarantine: what is done, and what is left

When a crash-loop is detected, the runner attributes it: if the repeated error names
**exactly one** active probe's table, that probe is the culprit (`attributeCrashLoop` —
two matches or none means no attribution, because a guess would quarantine an innocent
type *and record the guess as a finding*). A control is never a candidate.

What happens then:

- the culprit gets `STUCK` with the error and its SQLSTATE quoted;
- every batch-mate gets `INCONCLUSIVE` with `channelWedgedBy` naming the culprit — they
  were stuck behind it in the ordered channel and were never measured, so neither `STUCK`
  (that blames them for another type's poison) nor their truncated value comparison (that
  manufactures a `SILENT_LOSS`) may be claimed;
- `PROBE-RUN-QUARANTINE` names the culprit, the error, and the solo command;
- the control gate is untouched: `INCONCLUSIVE` is not `WORKS`, so the run is still
  `PROBE-RUN-INVALID` and none of its verdicts are published.

**What is NOT done: automatically re-running the rest of the batch in the same test.** It
was left out deliberately, not forgotten. Once a value has wedged the ordered channel its
event sits at the head of the queue and the importer retries it forever, so the surviving
probes cannot be measured by anything short of a *fresh migration*. Precisely what that
would take:

1. **Restructure `runDatatypeSweep` into an attempt loop.** The `defer r.lm.Cleanup()` /
   `defer r.emitAll()` pair is function-scoped, so one attempt has to become its own
   function — `runSweepAttempt(t, mode, batch, probes) []string` returning
   `r.quarantined` — called at most twice, with the second call's probe list being the
   first's minus the quarantined ids. `emitAll` must fire for the culprit on attempt 1 and
   for the survivors on attempt 2, and exactly once per probe id in total, or the results
   CSV gets two rows for one (probe, mode) and the differ sees a phantom.
2. **Prove a second `LiveMigrationTest` in one test is safe.** Each attempt gets a fresh
   export dir (`CreateTempExportDir`) but the *same* derived database name, and the
   containers are shared singletons that are not restarted. The two known hazards are
   (a) `Cleanup`'s `DropDatabase` failing while a logical replication slot from the first
   attempt still exists on the source — the error is only logged, so attempt 2 would
   silently start against a half-cleaned database; and (b) the wedged `import data`
   process from attempt 1 having to be fully dead before attempt 2 starts, which
   `Cleanup`'s 20 s `GracefulStop` should handle but has never been checked with a
   crash-looping importer. Both need a real live run to establish, which is why this half
   was not shipped blind: a re-run path that fails intermittently costs *two* batches
   instead of saving one.
3. **Decide the attempt budget.** One retry, not a bisect: a second poison probe in the
   surviving set would wedge attempt 2 as well, and at that point the batch is a manual
   job. Attempt 2's `PROBE-RUN-QUARANTINE` line is what says so.

Until then the loop is: the run names the culprit in seconds, and the batch is re-run
without it — the manual bisect it replaces used to cost ~48 min *per poison probe*.

The `Poison` field stays useful even with detection: a probe already *known* to wedge a
channel should not be put in a batch at all, and `excludeBatchedPoison` still keeps it out
before a container is even started.

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
