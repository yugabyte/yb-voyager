# The `voyager_core` schema

The `yugabytedb` control plane writes three tables in a schema named
`voyager_core`. Voyager does not create them: whoever runs the control plane
database owns the schema and must create it before a command runs. This is the
shape Voyager expects. The owner may add constraints of its own, such as a
foreign key on `migration_id`.

```sql
CREATE SCHEMA voyager_core;

CREATE TABLE voyager_core.migration_state (
    id            bigserial PRIMARY KEY,
    migration_id  uuid NOT NULL,
    name          text NOT NULL,
    scope         text NOT NULL DEFAULT '',
    value         boolean NOT NULL DEFAULT true,
    detail        jsonb,
    set_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX migration_state_current_idx
    ON voyager_core.migration_state (migration_id, name, scope, set_at DESC, id DESC);

CREATE TABLE voyager_core.metadata (
    id               bigserial PRIMARY KEY,
    migration_id     uuid NOT NULL,
    kind             text NOT NULL,
    payload          jsonb NOT NULL,
    payload_version  text NOT NULL,
    generated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX metadata_current_idx
    ON voyager_core.metadata (migration_id, kind, generated_at DESC, id DESC);

CREATE TABLE voyager_core.table_data_progress (
    migration_id             uuid NOT NULL,
    db_role                  text NOT NULL,
    direction                text NOT NULL,
    schema_name              text NOT NULL,
    table_name               text NOT NULL,
    status                   text NOT NULL DEFAULT 'not_started',
    snapshot_rows_estimated  bigint,
    snapshot_rows            bigint NOT NULL DEFAULT 0,
    snapshot_rows_errored    bigint NOT NULL DEFAULT 0,
    cdc_inserts              bigint NOT NULL DEFAULT 0,
    cdc_updates              bigint NOT NULL DEFAULT 0,
    cdc_deletes              bigint NOT NULL DEFAULT 0,
    details                  jsonb,
    updated_at               timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (migration_id, db_role, direction, schema_name, table_name)
);
```

## `migration_state`

Append-only boolean entries about a migration. The current value of an entry is
its newest row per `(migration_id, name, scope)`, ordered by `set_at`, then
`id`. `id` alone is not enough: sequence values are handed out in cached blocks
per session, so they do not follow write order.

In an offline migration, each command sets one entry when it starts and one
when it completes:

| Command | Started entry | Done entry |
|---|---|---|
| assess-migration | `assessment.started` | `assessment.completed` |
| export schema | `schema.export_started` | `schema.exported` |
| analyze-schema | `schema.analysis_started` | `schema.analyzed` |
| import schema | `schema.import_started` | `schema.imported` |
| export data | `data.export_started` | `data.exported` |
| import data | `data.import_started` | `data.imported` |

Starting a command whose done entry is currently true appends that entry as
false, so a re-run is not read as done while it is in progress. Nothing is
written when a command fails.

Voyager writes `scope` as `''` and leaves `detail` NULL.

## `metadata`

Append-only documents a command produces. The current document of a kind is its
newest row per `(migration_id, kind)`, ordered by `generated_at`, then `id`.

| `kind` | Written by | `payload` |
|---|---|---|
| `assessment` | assess-migration, on completion | the assessment report |
| `schema_analysis` | analyze-schema, on completion | the schema analysis report |

`payload_version` is the version of the document's format.

## `table_data_progress`

One row per table per stream, upserted in place while a command runs.

| Command | `db_role` | `direction` |
|---|---|---|
| export data | `source` | `export` |
| import data | `target` | `import` |

- `status`: `not_started`, `in_progress`, `done` or `completed`.
- `snapshot_rows_estimated`: the table's total row count as Voyager estimates it.
- `snapshot_rows`: rows exported or imported so far.

Voyager does not write `snapshot_rows_errored`, the `cdc_*` counters or
`details`.

## Step claiming tables (platform-owned)

Only used when `VOYAGER_OPERATION_ID` is set. The platform creates these;
Voyager checks they exist at `Init` and never issues DDL. Design:
[`design-specs/cp-step-claiming.md`](../../../design-specs/cp-step-claiming.md).

```sql
CREATE TABLE voyager_core.operation (
    id uuid PRIMARY KEY, migration_id uuid, sequence_id bigint, issued_by text,
    name text, config jsonb, idempotency_key text,
    created_at timestamptz DEFAULT now()
);
CREATE TABLE voyager_core.operation_step (
    id bigserial PRIMARY KEY, operation_id uuid, step text, ordinal int,
    UNIQUE (operation_id, step)
);
CREATE TABLE voyager_core.step_execution (
    id                bigserial PRIMARY KEY,
    migration_id      uuid NOT NULL,
    operation_step_id bigint NOT NULL,
    step              text NOT NULL,
    executor_ref      jsonb,
    started_at        timestamptz DEFAULT now(),
    last_heartbeat_at timestamptz DEFAULT now(),
    status            text DEFAULT 'running'
                      CHECK (status IN ('running','succeeded','failed','stopped')),
    reason            text,
    error_message     text,
    finished_at       timestamptz
);
CREATE UNIQUE INDEX one_running_per_step
    ON voyager_core.step_execution (migration_id, step) WHERE status = 'running';
ALTER TABLE voyager_core.migration_state ADD COLUMN step_execution_id bigint;
ALTER TABLE voyager_core.metadata ADD COLUMN step_execution_id bigint;
```

What Voyager does with them:

- Reads `operation` and `operation_step`: the step must be planned for the
  operation, and the operation must belong to this migration.
- Inserts a `running` `step_execution` row when a command starts (the claim),
  heartbeats `last_heartbeat_at` every 10 s, and closes the row.
  `executor_ref` is the `EXECUTOR_REF` env var, as-is.
- Never modifies another process's row. If the step already has a `running`
  row, live or stale, the claim fails; the platform closes that row.
- Close: `succeeded`, or `failed` with `error_message`. `stopped` is never
  written by Voyager.
- A child process joined to its parent's step execution (internal
  `VOYAGER_STEP_EXECUTION_ID`) only checks the row is `running` and belongs to
  its migration, and stamps its rows. It never inserts, heartbeats or closes it.
- While an attempt is open, `migration_state` and `metadata` rows carry its
  id in `step_execution_id`. Otherwise the column is NULL, and Voyager does not
  name it in its statements.

Command to step:

| Command | Step |
|---|---|
| assess-migration | `assess-migration` |
| export schema | `export-schema` |
| analyze-schema | `analyze-schema` |
| import schema | `import-schema` |
| export data (source exporter) | `export-data-from-source` |
| import data (target importer) | `import-data-to-target` |
| finalize-schema-post-data-import, `import schema --post-snapshot-import` | `finalize-schema` |
| end migration | `end-migration` |

For the phases with a done entry (the table under `migration_state`), the done
entry, its `metadata` document and the attempt's close are one transaction.
