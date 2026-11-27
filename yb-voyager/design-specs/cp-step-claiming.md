# Control plane step claiming (`yugabytedb` control plane)

|  |  |
| :---- | :---- |
| **Status** | Draft |
| **Author** | Shivansh Gahlot |
| **Scope** | `src/cp/yugabytedb`, `cmd/` hooks. Active only for the `yugabytedb` control plane with `VOYAGER_OPERATION_ID` set. |

## 1. Context

- The `yugabytedb` control plane writes `voyager_core.migration_state`, `metadata` and `table_data_progress` for a migration. See `src/cp/yugabytedb/voyager_core_schema.md`.
- A platform (AMP) launches a Job per operation. An **operation** is an ordered list of **steps** (for example "export-schema", then "import-schema"). One Job runs several Voyager commands in sequence.
- Before this change, nothing stops two Jobs (or a Job and a retry) from running the same step of the same migration at once, and nothing records which attempt wrote which row.
- This design adds **step claiming**: a Voyager command claims its step before doing work, keeps the claim alive, and closes it when it ends.

**Terms**

- **Migration**: identified by `YB_VOYAGER_MIGRATION_UUID` (the `migration_id` column everywhere).
- **Operation**: a row in `voyager_core.operation`, identified by the uuid in `VOYAGER_OPERATION_ID`.
- **Planned step**: a row in `voyager_core.operation_step` (`operation_id`, `step`). The platform writes it before launching the Job. It says "this operation expects this step to run".
- **Attempt**: a row in `voyager_core.step_execution` for one run of one step. Status is `running`, `succeeded`, `failed` or `stopped`.
- **Lease**: an attempt is alive while its `last_heartbeat_at` is newer than 2 minutes. Voyager only reports it; the platform's controller acts on it.
- **Claim**: inserting a `running` attempt.

**Non-goals**

- Voyager never creates or alters these tables. The platform owns the DDL (as for the existing three tables).
- No change to command behaviour or output when step claiming is off.
- `stopped` is written by the platform, not by Voyager.
- Voyager never modifies another process's attempt. Expiring, cancelling and rerunning a step belong to the platform's controller.
- Only the happy path is handled in Voyager: a stale attempt blocks a claim until the controller closes it.
- Live migration, fall-back and fall-forward commands do not claim.

## 2. When it is active

- Active only when the control plane type is `yugabytedb` **and** `VOYAGER_OPERATION_ID` is set.
- Otherwise behaviour is unchanged: no new queries, no new columns in any statement, no goroutine.

**Modes** (chosen at `Init` from the two env vars)

- Off: neither var set. Behaviour as above.
- Claim: `VOYAGER_OPERATION_ID` set. Claims, heartbeats and closes its own step execution (sections 5 to 7).
- Join: `VOYAGER_STEP_EXECUTION_ID` set. Runs inside a step execution held by its parent process:
  - `Init` runs the same table checks as claim mode.
  - When the migration UUID is resolved (where claim mode would claim), the step execution must exist, be `running` and have this migration's `migration_id`. Any mismatch fails the command before work.
  - Rows are stamped with the id. No claim, no heartbeat, no close: `CloseStep` does nothing and no done event closes it.
- Both vars set: `Init` fails.
- Child processes: `export schema` starts `assess-migration` with `VOYAGER_OPERATION_ID` removed and `VOYAGER_STEP_EXECUTION_ID` set to its own step execution id (nothing is set when it holds none). `assess-migration-bulk` children (different migrations) get both removed.

**Environment**

- `VOYAGER_OPERATION_ID`: required to activate. Must parse as a uuid, else `Init` fails.
- `VOYAGER_STEP_EXECUTION_ID`: internal, set only by Voyager for its own child process. Not part of the platform contract, and not forwarded into docker. Must parse as a positive integer, else `Init` fails.
- `EXECUTOR_REF`: optional. If set it must parse as a JSON object, else `Init` fails. Stored as-is in `step_execution.executor_ref`. If unset the column is NULL.

## 3. Data model (owned by the platform)

```sql
voyager_core.operation_step  (id bigserial PK, operation_id uuid, step text, ordinal int, UNIQUE (operation_id, step))
voyager_core.operation       (id uuid PK, migration_id uuid, ...)
voyager_core.step_execution  (id bigserial PK, migration_id uuid NOT NULL, operation_step_id bigint NOT NULL,
                              step text NOT NULL, executor_ref jsonb, started_at, last_heartbeat_at,
                              status text DEFAULT 'running' CHECK (status IN ('running','succeeded','failed','stopped')),
                              reason text, error_message text, finished_at)
UNIQUE INDEX one_running_per_step ON step_execution (migration_id, step) WHERE status = 'running'
migration_state.step_execution_id bigint NULL
metadata.step_execution_id        bigint NULL
```

- Voyager reads `operation` and `operation_step`; inserts and updates `step_execution`; writes `step_execution_id` on `migration_state` and `metadata`.
- `one_running_per_step` is what makes a claim exclusive: at most one running attempt per `(migration_id, step)`.

## 4. Step registry

Fixed in Voyager. Keyed by command (and role where a command has several):

| Command | Step |
|---|---|
| assess-migration | `assess-migration` |
| export schema | `export-schema` |
| analyze-schema | `analyze-schema` |
| import schema | `import-schema` |
| export data (source exporter role only) | `export-data-from-source` |
| import data (target importer role only) | `import-data-to-target` |
| finalize-schema-post-data-import | `finalize-schema` |
| end migration | `end-migration` |

- Any other command or role does not claim.
- `import schema --post-snapshot-import` is the same code as `finalize-schema-post-data-import`, so it claims `finalize-schema`.

## 5. Claim

- When: once, when the migration UUID is first resolved in a command (`retrieveMigrationUUID`), after command and role are known and before any work.
- One transaction:
  1. Find the planned step: `operation_step` joined to `operation` where `operation_id = $op AND step = $step`.
  2. If there is none, or `operation.migration_id` differs from this migration's UUID: fail the command with a clear error. No attempt row is written.
  3. Insert a `running` attempt `(migration_id, operation_step_id, step, executor_ref)` and take its `id`.
- If the insert hits `one_running_per_step`, the step already has a `running` attempt, live or stale: fail fast with a "step already running" error. Voyager does not touch that attempt; the controller closes it before a rerun.
- A claim failure fails the command through the normal error path before it does any work.

## 6. Heartbeat

- A goroutine runs `UPDATE step_execution SET last_heartbeat_at = now() WHERE id = $id AND status = 'running'` every 10 s.
- The heartbeat reports liveness; it never decides on its own to stop the command.
- 0 rows updated means the controller closed the attempt (a cancel): stop the command.
- Each heartbeat `UPDATE` has a 10 s timeout. A failed heartbeat is logged and retried on the next tick; the command keeps running however long the control plane is unreachable.
- Fencing is the controller's job: a process cut off from the control plane keeps running, so before rerunning a step whose attempt is still `running` the controller must stop the old process (for example, delete its Job) and then close the attempt.
- "Stop the command" is `utils.ErrExit` with a clear message, so the existing exit path (child cleanup, exit handler) runs. The goroutine itself never panics.
- The goroutine ends when the attempt closes.
- Heartbeats use their own connection pool, so they are not queued behind event writes.

## 7. Close

- Success: `status='succeeded'`, `finished_at=now()`.
- Failure (error exit, signal, or any non-success exit): `status='failed'`, `error_message=<error>`, `finished_at=now()`.
- Every close is guarded by `WHERE id = $id AND status = 'running'`. If it matches 0 rows the attempt was already closed by someone else and Voyager does not overwrite it.

**Same-transaction rule**

- For phases that write a done entry in `migration_state` (assessment.completed, schema.exported, schema.analyzed, schema.imported, data.exported, data.imported), the done entry, its `metadata` document (if any) and the attempt's close are **one transaction**.
- The close is done first in the transaction. If it matches 0 rows (attempt lost), the transaction rolls back and no done entry is written.
- Only the done event of the attempt's own step closes the step execution. Other phases that run inside a command are stamped with `step_execution_id` but do not close it.
- The assessment that `export schema` may run first is a child process (`assess-migration`) that joins the parent's step execution (section 2): its rows are stamped with the parent's id, and it never claims, heartbeats or closes. The parent's heartbeat keeps the lease.
- For steps with no done entry (`finalize-schema`, `end-migration`), and for a success exit where the done event was never sent (for example `export schema` finding the schema already exported), the attempt closes at successful command completion.
- Closing on exit runs inside the existing `PackAndSendCallhomePayloadOnExit` exit handler, above its payload-sent guard. Exit handlers run in random order, so a second handler cannot be ordered.
- "Successful command completion" is recorded by `PersistentPostRun`, which runs only when the command returned normally.
- If the done transaction failed after retries, the attempt is closed `failed` with that error, so the platform does not read a missing done entry as success.

## 8. Stamping

- While an attempt is active, every `migration_state` and `metadata` row Voyager writes carries `step_execution_id = <attempt id>`. Otherwise NULL.
- With no attempt, statements are exactly today's statements, so the column need not exist.
- `table_data_progress` is not stamped.

## 9. Init checks

- When `VOYAGER_OPERATION_ID` is set, `Init` also fails fast if any of these is missing or not usable by the connecting role:
  - `operation` (SELECT), `operation_step` (SELECT).
  - `step_execution` (INSERT, SELECT, UPDATE).
  - Columns `migration_state.step_execution_id` and `metadata.step_execution_id`.
- Same style as the existing table check.

## 10. Failure modes

| Case | Result |
|---|---|
| Planned step missing, or operation belongs to another migration | Command fails before work, no attempt row |
| Running attempt exists, live or stale | Command fails before work ("step already running"); the attempt is left as it is |
| Control plane DB unreachable at claim | Command fails before work |
| Heartbeat failing | Logged and retried; the command keeps running |
| Attempt closed under us (controller cancel) | Command stops with an error exit |
| Process killed (SIGKILL, node loss) | No close. The attempt stays `running` (its lease reads expired) until the controller closes it |
| Command exits with error | Attempt closed `failed` with the error text |

## 11. Tests

- Integration (`integration` tag, same style as `yugabytedb_test.go`, which creates the tables itself):
  - claim, heartbeat, done entry and close in one transaction;
  - failure closes `failed` with the error;
  - a running attempt, fresh or stale, makes a second claim fail and is left unchanged;
  - failing heartbeats do not stop the command;
  - an attempt closed by someone else stops the command;
  - a missing planned step, or a foreign operation, fails fast and writes nothing;
  - env var unset: no new queries, no rows;
  - `step_execution_id` stamped on `migration_state` and `metadata` rows.
- Unit: step registry mapping, env parsing.
