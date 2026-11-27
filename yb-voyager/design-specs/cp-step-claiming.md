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
- Live migration, fall-back and fall-forward are not supported under `VOYAGER_OPERATION_ID`. `export data` and `import data` claim whatever the export type, and the `syscall.Exec` handoff at cutover runs no exit handler, so the step execution stays `running`. Live needs its own claiming design.

## 2. When it is active

- Active only when the control plane type is `yugabytedb` **and** `VOYAGER_OPERATION_ID` is set.
- Otherwise behaviour is unchanged: no new queries, no new columns in any statement, no goroutine.
- Contract with the platform: with `VOYAGER_OPERATION_ID` set, the platform runs only commands that map to a step (section 4).

**Modes** (chosen at `Init` from the two env vars)

- Off: neither var set. Behaviour as above.
- Claim: `VOYAGER_OPERATION_ID` set. Claims, heartbeats and closes its own step execution (sections 5 to 7).
- Join: `VOYAGER_STEP_EXECUTION_ID` set. Runs inside a step execution held by its parent process:
  - `Init` runs the same table checks as claim mode.
  - Where claim mode would claim (section 5), the step execution must exist, be `running` and have this migration's `migration_id`. Any mismatch fails the command before work.
  - Rows are stamped with the id. No claim, no heartbeat, no close: `CloseStep` does nothing and no done event closes it.
- Both vars set: `Init` fails.
- Child processes: `export schema` starts `assess-migration` with `VOYAGER_OPERATION_ID` removed and `VOYAGER_STEP_EXECUTION_ID` set to its own step execution id (nothing is set when it holds none). Every other child Voyager process gets both removed, whatever the parent's control plane (`assess-migration-bulk` children are different migrations).

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
| export data, export data from source | `export-data-from-source` |
| import data, import data to target | `import-data-to-target` |
| finalize-schema-post-data-import | `finalize-schema` |
| end migration | `end-migration` |

- Any other command does not claim, including `export data from target`, `import data to source`, `import data to source-replica` and `import data file`.
- The step comes from the command path and the already-parsed `--post-snapshot-import` flag only, because the exporter and importer roles are not set yet when the claim runs (section 5).
- `import schema --post-snapshot-import` is the same code as `finalize-schema-post-data-import`, so it claims `finalize-schema`.

## 5. Claim

- When: once per process, in the root command's `PersistentPreRun`, after the control plane is set up and before any of the command's own code (so before `export schema`'s early returns and its `--start-clean` wipe). `assess-migration-bulk` does not claim.
- Migration id: the migration UUID in the migration status record when the metaDB already exists, else `YB_VOYAGER_MIGRATION_UUID`, else none. With no id, a claim fails with "migration id unknown: YB_VOYAGER_MIGRATION_UUID is not set and the migration has no status record". Join mode resolves the id the same way (the child shares the export dir, so the record exists). The id is resolved only in claim or join mode, so a bad `YB_VOYAGER_MIGRATION_UUID` changes nothing when step claiming is off.
- Step: the registry (section 4), from the command path and flags.
- The claim goes through the `cp.ControlPlane` interface (`ClaimStep`); control planes other than `yugabytedb`, and `yugabytedb` with claiming off, do nothing.
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

- The step execution stays `running` for the whole command. A phase's done entry (assessment.completed, schema.exported, schema.analyzed, schema.imported, data.exported, data.imported) and its `metadata` document are an ordinary stamped write and never close it, because commands keep working after sending their done event.
- If the write of the held step's own done event fails (after retries, or because its payload is invalid), the error is kept. The end-of-command close is then `failed` with "recording the step's completion in the control plane failed: <error>", unless the command itself failed, in which case that error is recorded. The platform therefore does not read a missing done entry as success.
- If the controller closes the attempt mid-command (a cancel), the heartbeat notices and stops the command (section 6). Voyager does not check the status before writing a done entry.
- The assessment that `export schema` may run first is a child process (`assess-migration`) that joins the parent's step execution (section 2): its rows are stamped with the parent's id, and it never claims, heartbeats or closes. The parent's heartbeat keeps the lease.
- For steps with no done entry (`finalize-schema`, `end-migration`), and for a success exit where the done event was never sent (for example `export schema` finding the schema already exported), the close is the same: `succeeded` at command end.
- Close sites, all through `ControlPlane.CloseStep`:
  - Success: `PersistentPostRun`, which runs only when the command returned normally. It calls `Finalize` first so every queued event is written, then `CloseStep(nil)`.
  - Failure: an exit handler registered right after a successful claim. It closes with the command's error, or "command was interrupted before it completed" when there is none. It is a handler of its own, not part of the callhome handler, so its position among the exit handlers does not matter: `CloseStep` is idempotent, and on a successful run `PersistentPostRun` has already closed.
  - `export data` finding no tables to export: it exits with `os.Exit(0)`, which skips `PersistentPostRun` and the exit handlers, so it calls `Finalize` and then `CloseStep(nil)` itself.

## 8. Stamping

- While an attempt is active, every `migration_state` and `metadata` row Voyager writes carries `step_execution_id = <attempt id>`.
- With step claiming on (claim or join mode) and no active attempt (before the claim, after the close, or a command that did not claim), such a write is an error and writes nothing: "<event type> not recorded: step claiming is on but no step execution is held". There are no unstamped rows.
- With step claiming off, statements are exactly today's statements, so the column need not exist.
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
| Done event write fails (command otherwise succeeds) | Attempt closed `failed` with "recording the step's completion in the control plane failed: ..." |
| `migration_state`/`metadata` write with claiming on and no attempt held | The write returns an error; no row is written |

## 11. Tests

- Integration (`integration` tag, same style as `yugabytedb_test.go`, which creates the tables itself):
  - claim, heartbeat and stamped done entry: the done event leaves the attempt `running`, and `CloseStep` then closes it `succeeded`;
  - a failed write of the step's own done event makes the end-of-command close `failed` with the "recording the step's completion" message;
  - a write with claiming on and no attempt held fails and writes no row;
  - an attempt closed by the controller is reported by the heartbeat (`onLost`);
  - failure closes `failed` with the error;
  - a running attempt, fresh or stale, makes a second claim fail and is left unchanged;
  - failing heartbeats do not stop the command;
  - a missing planned step, or a foreign operation, fails fast and writes nothing;
  - env var unset: no new queries, no rows;
  - `step_execution_id` stamped on `migration_state` and `metadata` rows.
- Unit: step registry mapping, env parsing.
