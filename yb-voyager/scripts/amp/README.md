# AMP migration assessment (POC)

A minimal migration assessment for a PostgreSQL → YugabyteDB AMP migration.

AMP's compute is PostgreSQL, so none of voyager's YugabyteDB compatibility
analysis applies. This assessment reports two things: complexity `LOW`, and a
vCPU recommendation matched to the source database's own capacity. It does not
analyse the schema — no datatypes, extensions, PL/pgSQL or query constructs.

It writes the same control-plane rows that `yb-voyager assess-migration` writes,
with a payload shaped like `AssessMigrationPayloadYugabyteD`, so the AMP
controller can read it unchanged.

> This is throwaway scaffolding for the POC. The real implementation belongs
> inside `yb-voyager assess-migration --target-db-type yugabytedb-amp`.

## Running it

```
yb-voyager-amp-assess-migration.sh <pg_connection_string> <schema_list> <output_dir>
```

| Argument | Meaning |
| --- | --- |
| `pg_connection_string` | Source PostgreSQL, e.g. `postgresql://user@host:5432/dbname`. Quote it. |
| `schema_list` | Pipe-separated, e.g. `public\|sales`. Recorded on the control-plane row; not otherwise used. |
| `output_dir` | Where the local copy of the report is written. |

| Environment variable | Required | Meaning |
| --- | --- | --- |
| `PGPASSWORD` | if the source needs one | Source database password |
| `YB_VOYAGER_MIGRATION_UUID` | yes | Migration UUID the control-plane rows are keyed by (AMP's `migration.id`) |
| `CONTROL_PLANE_TYPE` | no | `yugabyted` to write control-plane rows. Anything else, or unset, writes only the local report. |
| `YUGABYTED_DB_CONN_STRING` | when `CONTROL_PLANE_TYPE=yugabyted` | Control-plane database |

Full run:

```sh
PGPASSWORD=secret \
YB_VOYAGER_MIGRATION_UUID=90b8b0b8-1111-2222-3333-444455556666 \
CONTROL_PLANE_TYPE=yugabyted \
YUGABYTED_DB_CONN_STRING='postgresql://yugabyte:yugabyte@127.0.0.1:5433' \
./yb-voyager-amp-assess-migration.sh 'postgresql://user@dbhost:5432/prod' 'public' /tmp/amp-assessment
```

Local report only, no control plane:

```sh
YB_VOYAGER_MIGRATION_UUID=90b8b0b8-1111-2222-3333-444455556666 \
./yb-voyager-amp-assess-migration.sh 'postgresql://user@dbhost:5432/prod' 'public' /tmp/amp-assessment
```

Requires `psql` on `PATH`. Nothing else — no jq, no python, no voyager install.

### Output

- `<output_dir>/assessment/reports/migration_assessment_report.json`
- Two rows in `ybvoyager_visualizer.ybvoyager_visualizer_metadata`, if the
  control plane is enabled.

## Files

| File | Responsibility |
| --- | --- |
| `yb-voyager-amp-assess-migration.sh` | Driver. Validates input, sequences the steps, runs the vCPU ladder, assembles the payload. |
| `amp-source-facts.psql` | Everything cheap from the source, in one round trip. |
| `amp-source-os-capacity.psql` | The `exact` tier — reads real CPU limits off the source host. |
| `amp-controlplane-bootstrap.psql` | Control-plane schema DDL, copied verbatim from voyager. |

### Flow

```
validate args + env
   │
   ├─ amp-source-facts.psql ──────────────► 10 fields, one psql round trip
   │
   ├─ [if control plane enabled]
   │     amp-controlplane-bootstrap.psql
   │     SEQ = MAX(invocation_sequence) + 1
   │     INSERT seq=SEQ, status='IN PROGRESS', payload=''
   │
   ├─ vCPU ladder ────────────────────────► VCPUS, CONFIDENCE, DETECTION_METHOD
   │
   ├─ build payload, write local report
   │
   └─ [if control plane enabled]
         INSERT seq=SEQ+1, status='COMPLETED', payload=<json>
```

The `IN PROGRESS` row is written *before* any detection work, mirroring voyager.
If detection dies, the control plane shows a started-never-finished assessment
rather than nothing at all.

### `amp-source-facts.psql`

Always runs, against the source. Emits one pipe-separated row (`psql -tA`) that
the driver reads positionally — **field order is load-bearing**.

| # | Field | Used for |
| --- | --- | --- |
| 1–3 | `is_superuser`, `pg_has_role(pg_read_server_files)`, `has_function_privilege(pg_read_file(...))` | Whether the `exact` tier is attempted |
| 4–5 | `max_worker_processes`, `max_parallel_workers` | The `high` tier |
| 6–10 | database name, server version, address, port, `pg_database_size` | Control-plane row columns and `TotalDBSize` |

Its real job is being safe on a locked-down managed instance: every
`pg_settings` lookup is wrapped in `coalesce(..., '0')` so a missing parameter
produces a parseable value instead of an empty field that would shift every
value after it.

Reading `/proc` needs **both** membership in `pg_read_server_files` **and**
`EXECUTE` on the specific `pg_read_file` overload. Either one alone still fails,
which is why fields 2 and 3 are separate.

### `amp-source-os-capacity.psql`

Runs against the source, only when the capability probe passed. Four
pipe-separated fields: cgroup `cpu.max`, `Cpus_allowed_list` from
`/proc/self/status`, the `/proc/cpuinfo` processor count, and `MemTotal`.

Its job is producing a number that is true *for the database*, not for the
machine underneath it. Three things it deliberately handles:

- **`/proc` is host-scoped.** A container capped at 1.5 CPUs still reports every
  host core in `/proc/cpuinfo`, so that field is a last resort, not the primary.
- **The cgroup path is not `/sys/fs/cgroup/`.** `cpu.max` exists only on non-root
  cgroups, so the path is rebuilt from the `0::` line of `/proc/self/cgroup`.
- **cpuset restrictions are invisible to the quota.** `--cpuset-cpus=0-1` leaves
  `cpu.max` at `max 100000`; only `Cpus_allowed_list` shows the truth. Both are
  read and the smaller wins.

Every read uses `pg_read_file`'s 4-argument form so `missing_ok` turns an absent
path into NULL rather than an error — which is what lets the file run harmlessly
on a non-Linux host and return empty strings. Reads are length-bounded because
the buffer is `palloc`'d up front.

cgroup **v1 is not handled**. Such hosts fall back to `Cpus_allowed_list`, still
cpuset-correct, only missing a CFS quota.

### `amp-controlplane-bootstrap.psql`

Pure DDL, no output, run against the control plane. Copied verbatim from
voyager's `setupDatabase()` (`yb-voyager/src/cp/yugabyted/yugabyted.go`):
`CREATE SCHEMA IF NOT EXISTS`, the `CREATE TABLE IF NOT EXISTS` with its original
11 columns, then 7 `ALTER`s adding `host_ip` / `port` / `db_version` /
`voyager_info` and widening three `VARCHAR(250)`s to `TEXT`.

It is deliberately *not* collapsed into one tidy 15-column `CREATE TABLE`.
Voyager grew those four columns later, and matching its exact statement sequence
means a database bootstrapped by this script and one bootstrapped by voyager end
up identical — same columns, same order, same types — in either order.

## The vCPU ladder

| Confidence | Rule |
| --- | --- |
| `exact` | Read the host: `min(Cpus_allowed_list, cgroup cpu.max quota)`, falling back to `/proc/cpuinfo` |
| `high` | `max_worker_processes > 8 → ÷2`, else `max_parallel_workers > 8 → ×2` |
| `unknown` | `-1` — the caller decides what to do |

The `> 8` gate is the whole trick. PostgreSQL ships both parameters at exactly
8, so any larger value means something sized them from the machine, using the
documented formulas `max_worker_processes = GREATEST(vCPU*2, 8)` and
`max_parallel_workers = GREATEST(vCPU/2, 8)`.

Two consequences to be aware of:

- The formulas saturate at their floor of 8. `max_parallel_workers` only exceeds
  8 above 16 vCPU, so small and mid-sized managed instances commonly fall
  through to `-1` rather than `high`.
- A parameter that was hand-edited away from its default is indistinguishable
  from one the platform computed, and produces a confident wrong answer.
  `DetectionMethod` in the payload records which GUC was inverted.

## Payload

`json_build_object`, not `jsonb_build_object` — jsonb sorts keys, json preserves
insertion order, so the output matches Go's struct marshal order field for
field.

Shape is `AssessMigrationPayloadYugabyteD` with `PayloadVersion` `1.8-amp.1`:

- `MigrationComplexity` hard-coded `LOW`
- `Sizing.SizingRecommendation` with `NumNodes: 1`, the detected `VCPUsPerInstance`,
  and `MemoryPerInstance` fixed at 16 (AMP provisioning takes vCPU only). Colocated
  and sharded arrays empty; connection counts and import-time estimates zero.
- `SourceEnvironment` — an addition, not in voyager's payload: `VCPUs`,
  `MemoryGiB`, `DetectionMethod`, `Confidence`
- `SourceSizeDetails` carries `TotalDBSize` only
- `SchemaSummary`, `ParsedSchemaSummary`, `AssessmentIssues` present but empty

When vCPU is `-1`, `Sizing.FailureReasoning` explains why.

## Gotchas if you edit this

- `psql -c` does **not** interpolate `:'var'`. Variable interpolation only
  happens for script input, so both parameterised statements are fed on stdin
  heredocs.
- There is no `ON CONFLICT` on the control-plane table, matching voyager. The
  sequence is read fresh from the table each run, so a second run lands at 3 and
  4 rather than colliding on the primary key.
- The driver is `#!/bin/bash` and uses `read -ra`, which behaves differently
  under zsh. Run it with bash.
