#!/usr/bin/env python3

import os
import sys
import glob
import argparse
import copy
import random
import subprocess
import time
from typing import Any, Dict, Callable
import helpers as H

# -------------------------
# Action registry
# -------------------------

ACTION_REGISTRY: Dict[str, Callable[[Dict[str, Any], Any], None]] = {}


def action(name: str):
    def _wrap(fn: Callable[[Dict[str, Any], Any], None]):
        ACTION_REGISTRY[name] = fn
        return fn
    return _wrap


def get_action(name: str) -> Callable[[Dict[str, Any], Any], None]:
    try:
        return ACTION_REGISTRY[name]
    except KeyError as e:
        raise ValueError(f"Unknown action: {name}") from e


# -------------------------
# Actions
# -------------------------

@action("run_sql")
def run_sql_action(stage: Dict[str, Any], ctx: Any) -> None:
    # Execute SQL against source/target based on stage params
    sql_path = stage.get("sql_path")
    target = stage.get("target", "source")
    use_admin = bool(stage.get("use_admin"))
    if not sql_path:
        raise ValueError("run_sql action requires non-empty 'sql_path'")
    # Resolve relative to test_root if provided
    if ctx.test_root and not os.path.isabs(sql_path):
        sql_path = os.path.join(ctx.test_root, sql_path)
    H.run_sql_file(ctx, sql_path, target, use_admin=use_admin)


@action("voyager_export_schema")
def export_schema_action(_stage, ctx: Any) -> None:
    H.export_schema(ctx.cfg, ctx.env)


@action("voyager_import_schema")
def import_schema_action(_stage, ctx: Any) -> None:
    H.import_schema(ctx.cfg, ctx.env)


@action("generator_start")
def generator_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    key = stage.get("generator_key", "generator")
    ctx.processes[key] = H.start_generator_from_context(ctx, key)


@action("generator_stop")
def generator_stop_action(stage: Dict[str, Any], ctx: Any) -> None:
    key = stage.get("generator_key", "generator")
    timeout = int(stage.get("graceful_timeout_sec", 60))
    H.stop_generator(ctx.processes.pop(key, None), timeout)


@action("conflict_generator_start")
def conflict_generator_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Start the conflict generator, modeled on `generator_start`.

    Reads a config block named by `generator_key` (default: conflict_generator),
    e.g.

        conflict_generator_source:
          config_inline:
            connection: { host, port, database, user, password }
            conflict:
              sql_path: ./source_dml.sql
              interval_seconds: 10

    The conflict generator re-applies the conflict-DML file every
    `interval_seconds` until stopped, passing `-v cycle=N` so each cycle hits a
    fresh set of rows.
    """
    key = stage.get("generator_key", "conflict_generator")
    cfg_block = ctx.cfg.get(key) or {}
    inline = cfg_block.get("config_inline") or cfg_block.get("config") or {}
    conflict_cfg = inline.get("conflict") or {}

    # Always take the connection from the top-level source/target (admin creds);
    # conflict_generator_source -> source, conflict_generator_target -> target.
    role = "target" if key.endswith("_target") else "source"
    top = ctx.cfg.get(role) or {}
    admin = top.get("admin") or {}
    connection = {
        "host": top.get("host"),
        "port": top.get("port"),
        "database": top.get("database"),
        "user": admin.get("user") or top.get("user"),
        "password": admin.get("password") or top.get("password"),
    }

    sql_path = conflict_cfg.get("sql_path")
    if not sql_path:
        raise ValueError(f"conflict_generator_start: '{key}.config_inline.conflict.sql_path' is required")
    if ctx.test_root and not os.path.isabs(sql_path):
        sql_path = os.path.join(ctx.test_root, sql_path)
    if not os.path.isfile(sql_path):
        raise FileNotFoundError(f"conflict_generator_start: SQL file not found at '{sql_path}'")
    interval = float(conflict_cfg.get("interval_seconds", 3))
    if interval <= 0:
        raise ValueError(f"conflict_generator_start: 'interval_seconds' must be positive, got {interval}")

    # Label distinguishes the phase in the logs (e.g. conflict_generator_source
    # = forward leg, conflict_generator_target = fallback leg), and includes the
    # loop iteration so per-iteration counts are readable in iterated tests.
    label = f"conflict-gen[{key} iter={ctx.loop_iteration}]"
    gen = H.ConflictGenerator(connection, sql_path, interval, ctx.env, label=label)
    gen.start()
    ctx.conflict_generators[key] = gen


@action("conflict_generator_stop")
def conflict_generator_stop_action(stage: Dict[str, Any], ctx: Any) -> None:
    key = stage.get("generator_key", "conflict_generator")
    timeout = int(stage.get("graceful_timeout_sec", 60))
    gen = ctx.conflict_generators.pop(key, None)
    if gen:
        gen.stop(timeout_sec=timeout)


@action("voyager_export_start")
def export_start_action(_stage, ctx: Any) -> None:
    with ctx.process_lock:
        ctx.processes["export_data"] = H.start_command_by_name("export_data", ctx)


@action("voyager_export_from_target_start")
def export_from_target_start_action(_stage, ctx: Any) -> None:
    """Start yb-voyager export-from-target process for fallback."""
    with ctx.process_lock:
        ctx.processes["export_from_target"] = H.start_command_by_name("export_from_target", ctx)


@action("voyager_import_start")
def import_start_action(_stage, ctx: Any) -> None:
    with ctx.process_lock:
        ctx.processes["import_data"] = H.start_command_by_name("import_data", ctx)


@action("voyager_import_to_source_start")
def import_to_source_start_action(_stage, ctx: Any) -> None:
    """Start yb-voyager import-to-source process for fallback."""
    with ctx.process_lock:
        ctx.processes["import_to_source"] = H.start_command_by_name("import_to_source", ctx)


@action("voyager_import_to_source_replica_start")
def import_to_source_replica_start_action(_stage, ctx: Any) -> None:
    """Start yb-voyager import-to-source-replica process for fall-forward."""
    with ctx.process_lock:
        ctx.processes["import_to_source_replica"] = H.start_command_by_name("import_to_source_replica", ctx)


@action("voyager_archive_changes_start")
def archive_changes_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Start yb-voyager archive changes process.

    Optional stage key:
      - policy: explicit policy ("delete" or "archive").
                When omitted, a policy is chosen at random.
    Skips if the archiver is already running (continuous archiver across iterations).
    """
    with ctx.process_lock:
        existing = ctx.processes.get("archive_changes")
        if existing and existing.poll() is None:
            H.log("archive_changes: already running, skipping start")
            return
    policy = stage.get("policy") or random.choice(["delete", "archive"])
    H.log(f"archive_changes: selected policy={policy}")
    ctx.archive_changes_policy = policy
    cmd = H.build_archive_changes_cmd(ctx, policy)
    with ctx.process_lock:
        ctx.processes["archive_changes"] = H.spawn(cmd, ctx.env)

@action("validate_archive_changes")
def validate_archive_changes_action(_stage, ctx: Any) -> None:
    check_post_cutover_to_source = bool(_stage.get("check_post_cutover_to_source", False))
    H.validate_archive_changes(ctx, check_post_cutover_to_source=check_post_cutover_to_source)

@action("voyager_stop_command")
def stop_command_action(stage: Dict[str, Any], ctx: Any) -> None:
    command = stage.get("command")
    timeout = int(stage.get("graceful_timeout_sec", 20))
    H.stop_process(ctx, command, graceful_timeout=timeout)


# Predicate dispatch:
#   - exporter_in_streaming_phase / remaining_events_eq_0 inspect the queue
#     directory of the *current* iteration's export-dir, so they use
#     ctx.iteration_export_dir.
#   - cutover_to_*_status_completed call `yb-voyager cutover status`, which is
#     a parent-scoped voyager operation from a user's perspective; pass
#     ctx.export_dir_base.
_WAIT_FOR_CONDITIONS: Dict[str, Dict[str, Any]] = {
    "exporter_in_streaming_phase": {
        "interval": 5,
        "predicate": lambda ctx: H.exporter_streaming(ctx.iteration_export_dir),
    },
    "remaining_events_eq_0": {
        "interval": 5,
        "predicate": lambda ctx: H.backlog_marker_present(ctx.iteration_export_dir),
    },
    "cutover_to_target_status_completed": {
        "interval": 10,
        "predicate": lambda ctx: H.get_cutover_status(ctx.export_dir_base, mode="target") == "COMPLETED",
    },
    "cutover_to_source_status_completed": {
        "interval": 10,
        "predicate": lambda ctx: H.get_cutover_status(ctx.export_dir_base, mode="source") == "COMPLETED",
    },
    "cutover_to_source_replica_status_completed": {
        "interval": 10,
        "predicate": lambda ctx: H.get_cutover_status(ctx.export_dir_base, mode="source-replica") == "COMPLETED",
    },
}


@action("wait_for")
def wait_for_action(stage: Dict[str, Any], ctx: Any) -> None:
    cond = stage["condition"]
    timeout_sec = int(stage.get("timeout_sec", 0))  # 0 => no overall timeout
    try:
        cfg = _WAIT_FOR_CONDITIONS[cond]
    except KeyError as exc:
        raise ValueError(f"unknown condition: {cond}") from exc

    interval = cfg["interval"]
    predicate = cfg["predicate"]
    ok = H.poll_until(timeout_sec, interval, lambda: predicate(ctx))

    if not ok:
        raise TimeoutError(cond)


@action("cutover_to_target")
def cutover_to_target_action(_stage, ctx: Any) -> None:
    H.initiate_cutover(ctx.cfg, ctx.env, "target")


@action("cutover_to_source")
def cutover_to_source_action(_stage, ctx: Any) -> None:
    H.initiate_cutover(ctx.cfg, ctx.env, "source")


@action("cutover_to_source_replica")
def cutover_to_source_replica_action(_stage, ctx: Any) -> None:
    """Initiate cutover back to the source-replica database."""
    H.initiate_cutover(ctx.cfg, ctx.env, "source-replica")


@action("row_count_validations")
def row_count_validations_action(stage: Dict[str, Any], ctx: Any) -> None:
    left_role = stage.get("left_role", "source")
    right_role = stage.get("right_role", "target")
    H.run_row_count_validations(ctx, left_role, right_role)


@action("row_hash_validations")
def row_hash_validations_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Run segment-based row hash validations between two roles (default: source and target)."""
    helper_dir = os.path.dirname(__file__)
    sql_path = os.path.join(helper_dir, "segment_hash_validation.sql")

    left_role = stage.get("left_role", "source")
    right_role = stage.get("right_role", "target")

    for role in {left_role, right_role}:
        H.run_sql_file(ctx, sql_path, target=role, use_admin=False)

    H.run_segment_hash_validations(ctx, left_role, right_role)


def _conflict_log_table_marker(table: str | None) -> str | None:
    """Build the substring that identifies `table` in a "conflict detected" log
    line, e.g. "public.orders" -> 'table "public"."orders"' (conflictDetectionCache.go
    renders TableNameTup.ForKey() with each identifier quoted)."""
    if table is None:
        return None
    schema, sep, name = table.partition(".")
    if not sep:
        raise ValueError(f"_conflict_log_table_marker: 'table' must be schema-qualified (e.g. 'public.orders'), got {table!r}")
    return f'table "{schema}"."{name}"'


def _count_conflict_log_lines(log_dir: str, name: str, table: str | None) -> int:
    """Count "conflict detected" lines in `log_dir`/`name`* log files, optionally
    scoped to one table (see `_conflict_log_table_marker`)."""
    table_marker = _conflict_log_table_marker(table)
    count = 0
    for p in glob.glob(os.path.join(log_dir, name + "*")):
        with open(p, errors="ignore") as f:
            for line in f:
                if "conflict detected" in line and (table_marker is None or table_marker in line):
                    count += 1
    return count


@action("validate_conflicts_detected")
def validate_conflicts_detected_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Assert the import-data log recorded unique-key conflict detections.

    row_count/row_hash prove the data ended up correct; this proves the
    conflict-detection cache actually fired -- otherwise a run could pass
    without ever exercising it. Meaningful on the forward leg
    (yb-voyager-import-data.log); the fallback leg's import-to-source forces
    PARTITION_BY_TABLE and logs 0 conflicts by design, so don't assert there.

    Optional stage key:
      - table: schema-qualified table name (e.g. "public.orders"); when set,
        only counts lines also mentioning that table -- lets a run that mixes
        custom-cdc-partition-key tables (expected to log 0 conflicts) with
        normally-routed tables (expected to keep logging some) assert both
        against the same log file.
    """
    log_dir = os.path.join(ctx.iteration_export_dir, "logs")
    name = stage.get("log", "yb-voyager-import-data.log")
    min_count = int(stage.get("min_count", 1))
    table = stage.get("table")
    count = _count_conflict_log_lines(log_dir, name, table)
    if count < min_count:
        raise RuntimeError(
            f"validate_conflicts_detected: expected >= {min_count} 'conflict detected' "
            f"in {name}{f' for table {table}' if table else ''}, found {count} (dir={log_dir})"
        )
    H.log(f"validate_conflicts_detected: {count} conflicts detected in {name}" + (f" for table {table}" if table else ""))


@action("validate_no_conflicts_detected")
def validate_no_conflicts_detected_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Assert an import log recorded NO conflict detections.

    Used on the fallback leg (yb-voyager-import-data-to-source.log): the
    source-importer forces PARTITION_BY_TABLE and skips conflict detection, so
    the log must have 0 'conflict detected' -- this guards that the skip holds.

    Optional stage key:
      - table: schema-qualified table name (e.g. "public.orders"); when set,
        only counts lines also mentioning that table -- lets this assert 0
        conflicts for one table (e.g. a custom-cdc-partition-key table)
        within a log that also has real conflicts logged for other tables.
    """
    log_dir = os.path.join(ctx.iteration_export_dir, "logs")
    name = stage.get("log", "yb-voyager-import-data-to-source.log")
    table = stage.get("table")
    count = _count_conflict_log_lines(log_dir, name, table)
    if count != 0:
        raise RuntimeError(
            f"validate_no_conflicts_detected: expected 0 'conflict detected' "
            f"in {name}{f' for table {table}' if table else ''}, found {count} (dir={log_dir})"
        )
    H.log(f"validate_no_conflicts_detected: 0 conflicts detected in {name}" + (f" for table {table}" if table else "") + " (as expected)")


@action("pick_random_custom_key")
def pick_random_custom_key_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Randomly select ONE table/column(s) to route by `--cdc-partition-key-overrides`
    this run -- mirroring how a real user opts specific tables into custom-key
    routing -- and wire that pick through everywhere it needs to land:

      - appends to `voyager.import_data.flags.cdc-partition-key-overrides`.
      - adds the picked column(s) to the named generator's
        `exclude_columns_from_update[table]`, so the random generator never
        updates them (the importer requires custom key columns to be immutable).

    Must run before `start_event_generator`/`start_importer` -- both read
    `ctx.cfg` at start time, so mutating it here beforehand is sufficient; no
    orchestrator plumbing changes are needed for the pick to take effect.

    Required stage key:
      - candidates: list of {table: "schema.table", expect_conflicts: bool
        (optional, default false)} with EITHER:
          - columns: [col, ...] -- a fixed custom key, OR
          - random_columns_pool: [col, ...] -- the key's columns are ALSO
            randomized: 1..2 columns (or min_columns..max_columns) are sampled
            from the pool, in random order, so successive runs route the same
            table by different columns and column counts.
        Every fixed column and every pool column must already be safe to route
        by (never appears in an UPDATE for that table in the conflict DML, and
        its value must be identical across the rows of each conflict pair --
        e.g. a DML-churned key column, or a column the DML leaves at a
        transaction-constant default) -- this action only performs the pick
        and the wiring, not that verification.
        `expect_conflicts: true` marks a candidate whose DML deliberately
        creates conflicts that MUST still be detected when it is picked (e.g.
        a PK-recycle pattern); `validate_picked_custom_key_conflicts` uses it
        to decide which assertion to run. Such candidates should use fixed
        `columns` -- their assertion depends on the specific key.

    Optional stage key:
      - generator_key: which generator config block to inject
        exclude_columns_from_update into (default: "generator").

    The pick is stored on `ctx.picked_custom_key` for
    `validate_picked_custom_key_conflicts` to consume later.
    """
    if ctx.picked_custom_key is not None:
        raise RuntimeError(
            "pick_random_custom_key: ctx.picked_custom_key is already set "
            f"(previous pick: {ctx.picked_custom_key}) -- this action does not support "
            "running more than once per scenario (e.g. inside a loop_start/loop_end block), "
            "since a second pick would silently stack onto cdc-partition-key-overrides via "
            "';' while ctx.picked_custom_key would only reflect the newest pick."
        )

    candidates = stage.get("candidates")
    if not candidates:
        raise ValueError("pick_random_custom_key: 'candidates' stage key is required and must be non-empty")

    choice = random.choice(candidates)
    table = choice["table"]
    pool = choice.get("random_columns_pool")
    if pool:
        if choice.get("columns"):
            raise ValueError(f"pick_random_custom_key: candidate {table} must not set both 'columns' and 'random_columns_pool'")
        min_cols = int(choice.get("min_columns", 1))
        max_cols = min(int(choice.get("max_columns", 2)), len(pool))
        num_cols = random.randint(min_cols, max_cols)
        columns = random.sample(pool, num_cols)
    else:
        columns = choice["columns"]
    expect_conflicts = bool(choice.get("expect_conflicts", False))
    ctx.picked_custom_key = {"table": table, "columns": columns, "expect_conflicts": expect_conflicts}
    H.log(
        f"pick_random_custom_key: selected table={table} columns={columns} "
        f"expect_conflicts={expect_conflicts} "
        f"({'sampled from pool of ' + str(len(pool)) if pool else 'fixed'}; out of {len(candidates)} candidates)"
    )

    override = f"{table}:({','.join(columns)})"
    flags = ctx.cfg.setdefault("voyager", {}).setdefault("import_data", {}).setdefault("flags", {})
    existing = flags.get("cdc-partition-key-overrides")
    flags["cdc-partition-key-overrides"] = f"{existing};{override}" if existing else override

    generator_key = stage.get("generator_key", "generator")
    _, _, bare_table = table.partition(".")
    gen_cfg_block = ctx.cfg[generator_key]
    if "config_inline" not in gen_cfg_block:
        raise ValueError(
            f"pick_random_custom_key: generator block '{generator_key}' must use 'config_inline' "
            "(not 'config_path') -- a config_path generator loads a static file this action cannot mutate at run time"
        )
    gen_section = gen_cfg_block["config_inline"]["generator"]
    exclude_map = gen_section.setdefault("exclude_columns_from_update", {})
    excluded_for_table = exclude_map.setdefault(bare_table, [])
    for col in columns:
        if col not in excluded_for_table:
            excluded_for_table.append(col)


@action("validate_picked_custom_key_conflicts")
def validate_picked_custom_key_conflicts_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Run the right conflict assertion for whatever table `pick_random_custom_key`
    selected earlier in this run:

      - expect_conflicts false (the usual case): the picked table's conflicts all
        share its custom key's value, so they land on one channel already and the
        conflict-detection cache must never fire for it -- assert 0.
      - expect_conflicts true (e.g. a PK-recycle candidate, where the SAME PK is
        reused with a DIFFERENT custom-key value -- a real cross-channel race
        custom-key routing does not eliminate): the synthetic-PK guard must still
        catch it -- assert >= min_count (default 1).
    """
    picked = ctx.picked_custom_key
    if not picked:
        raise RuntimeError(
            "validate_picked_custom_key_conflicts: ctx.picked_custom_key is unset "
            "-- run 'pick_random_custom_key' earlier in this scenario"
        )
    scoped_stage = dict(stage)
    scoped_stage["table"] = picked["table"]
    if picked["expect_conflicts"]:
        validate_conflicts_detected_action(scoped_stage, ctx)
    else:
        validate_no_conflicts_detected_action(scoped_stage, ctx)


@action("voyager_import_start_expect_fail")
def voyager_import_start_expect_fail_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Run `import data` in the foreground with the scenario's flags plus the
    stage's `flags` on top, and require it to FAIL with `error_contains` in its
    output -- used to verify resume guardrails at the real CLI level (e.g.
    changing cdc-partition-key / cdc-partition-key-overrides between runs must
    be rejected). The running importer must be stopped first
    (voyager_stop_command), or this run fails on the export-dir lock instead.

    Stage keys:
      - flags (required): flag overrides merged over voyager.import_data.flags
        for this one invocation only (ctx.cfg is not mutated). The placeholder
        "{picked_table}" in a value is replaced with the table
        pick_random_custom_key selected.
      - error_contains (required): substring that must appear in the failed
        run's stdout/stderr.
      - timeout_sec (optional, default 120): how long the invocation may run
        before being killed and the stage failed.
    """
    error_contains = stage.get("error_contains")
    if not error_contains:
        raise ValueError("voyager_import_start_expect_fail: 'error_contains' stage key is required")
    extra_flags = dict(stage.get("flags") or {})
    if not extra_flags:
        raise ValueError("voyager_import_start_expect_fail: 'flags' stage key is required and must be non-empty")
    for k, v in extra_flags.items():
        if isinstance(v, str) and "{picked_table}" in v:
            if not ctx.picked_custom_key:
                raise RuntimeError(
                    "voyager_import_start_expect_fail: '{picked_table}' used but no custom key was picked "
                    "-- run 'pick_random_custom_key' earlier in this scenario"
                )
            extra_flags[k] = v.replace("{picked_table}", ctx.picked_custom_key["table"])

    cfg = copy.deepcopy(ctx.cfg)
    flags = cfg.setdefault("voyager", {}).setdefault("import_data", {}).setdefault("flags", {})
    flags.update(extra_flags)
    cmd = H.build_import_data_cmd(cfg)
    timeout = int(stage.get("timeout_sec", 120))
    H.log(f"voyager_import_start_expect_fail: running import data expecting failure with {extra_flags}")
    proc = subprocess.run(cmd, env=ctx.env, capture_output=True, text=True, timeout=timeout)
    output = (proc.stdout or "") + (proc.stderr or "")
    if proc.returncode == 0:
        raise RuntimeError(f"voyager_import_start_expect_fail: import unexpectedly SUCCEEDED with flags {extra_flags}")
    if error_contains not in output:
        raise RuntimeError(
            f"voyager_import_start_expect_fail: import failed (exit {proc.returncode}) but the expected "
            f"error text was not found.\nexpected substring: {error_contains}\noutput (tail):\n{output[-3000:]}"
        )
    H.log(f"voyager_import_start_expect_fail: import correctly rejected (exit {proc.returncode})")


@action("start_resumptions")
def start_resumptions_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Start per-command resumption workers based on the provided resumption map."""
    resumption_cfg = stage.get("resumption", {})
    H.start_resumptions_for_stage(resumption_cfg, ctx)


@action("voyager_stop_resumptions")
def stop_resumptions_action(stage: Dict[str, Any], ctx: Any) -> None:
    cmd = stage.get("command")
    if not cmd:
        raise ValueError("voyager_stop_resumptions requires 'command'")
    timeout = int(stage.get("timeout_sec", 30))
    H.stop_resumptions_for_command(cmd, ctx, timeout_sec=timeout)


@action("sleep")
def sleep_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Pause execution for a given number of seconds."""
    secs = int(stage.get("seconds", 0))
    if secs > 0:
        H.log(f"sleeping for {secs} seconds")
        time.sleep(secs)


@action("loop_start")
def loop_start_action(_stage, _ctx: Any) -> None:
    """No-op marker; the runner uses this to know where to jump back."""
    pass


@action("loop_end")
def loop_end_action(_stage, _ctx: Any) -> None:
    """No-op marker; the runner inline-handles loop-back at this stage."""
    pass


@action("reset_databases")
def reset_databases_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Drop and recreate source/target databases using admin credentials."""
    targets = stage.get("targets") or ["source", "target"]
    for target_name in targets:
        H.reset_database_for_role(target_name, ctx)


@action("create_cutover_table")
def create_cutover_table_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Create the cutover_table required for cutover/backlog checks in all tests."""
    targets = stage.get("target") or ["source", "source_replica"]
    for target_name in targets:
        H.create_cutover_table(ctx, target_name)


@action("grant_source_permissions")
def grant_source_permissions_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Grant source DB user permissions required for live migration.

    Optional stage key:
      - is_live_migration_fall_back: 0/1 flag; when 1, grant
        additional permissions required for fallback.
    """
    fallback = int(stage.get("is_live_migration_fall_back", 0))
    H.grant_postgres_live_migration_permissions(ctx, is_live_migration_fall_back=fallback)


# -------------------------
# Runner
# -------------------------

def _resolve_path(p: str | None, base_dir: str) -> str | None:
    if not p:
        return None
    p = os.path.expanduser(os.path.expandvars(p))
    if not os.path.isabs(p):
        p = os.path.join(base_dir, p)
    return os.path.abspath(p)


def main() -> None:
    parser = argparse.ArgumentParser(description="Live migration resiliency orchestrator")
    parser.add_argument("scenario", help="Path to scenario YAML")
    args = parser.parse_args()

    scenario_path = os.path.abspath(os.path.expanduser(args.scenario))
    cfg = H.load_config(scenario_path)
    # Lightweight validation of scenario structure
    H.validate_scenario(cfg)

    # Resolve test-root as the directory of the scenario file
    test_root = os.path.dirname(scenario_path)

    # Normalize key paths relative to test root
    cfg["export_dir"] = _resolve_path(cfg.get("export_dir"), test_root) or os.path.join(test_root, "export-dir")
    cfg["artifacts_dir"] = _resolve_path(cfg.get("artifacts_dir"), test_root) or os.path.join(test_root, "artifacts")

    env = H.merge_env(os.environ, cfg.get("env"))

    # Prepare paths by cleaning and recreating export-dir and artifacts
    H.prepare_paths(cfg["export_dir"], cfg["artifacts_dir"])

    ctx = H.Context(cfg=cfg, env=env, test_root=test_root)
    had_failure = False

    stages = cfg["stages"]
    num_iterations = int(cfg.get("num_iterations", 1))

    loop_start_idx = None
    for i, s in enumerate(stages):
        if s.get("action") == "loop_start":
            loop_start_idx = i
            break

    try:
        iteration = 0
        idx = 0
        while idx < len(stages):
            stage = stages[idx]
            stage_name = stage.get("name", "<unnamed>")
            ctx.loop_iteration = iteration
            H.log_stage_start(stage_name)
            start_ts = H._ts()
            try:
                get_action(stage["action"])(stage, ctx)
                end_ts = H._ts()
                H.append_stage_summary(cfg["artifacts_dir"], stage_name, start_ts, end_ts, status="OK")
                H.log_stage_end(stage_name, status="OK")
                if stage["action"] == "loop_end":
                    if loop_start_idx is None:
                        raise RuntimeError("loop_end: scenario has no loop_start stage")
                    iteration += 1
                    ctx.loop_iteration = iteration
                    if iteration >= num_iterations:
                        idx += 1
                    else:
                        H.apply_effective_export_dir(ctx)
                        idx = loop_start_idx
                else:
                    idx += 1
            except Exception as e:
                end_ts = H._ts()
                H.append_stage_summary(cfg["artifacts_dir"], stage_name, start_ts, end_ts, status="FAILED", error=str(e))
                H.log_stage_end(stage_name, status=f"FAILED: {e}")
                had_failure = True
                raise
    finally:
        # Always capture artifacts/logs at the end regardless of success or failure
        H.scan_logs_for_errors(cfg["export_dir"], cfg["artifacts_dir"])
        H.snapshot_msr_and_stats(cfg["export_dir"], cfg["artifacts_dir"])
        if had_failure:
            H.copy_logs_directory(cfg["export_dir"], cfg["artifacts_dir"])

        # Best-effort cleanup: ensure background processes started by the orchestrator
        # don't outlive the orchestrator itself.
        try:
            # Stop resumer threads first so they don't restart processes while we're shutting down.
            ctx.stop_event.set()
            with ctx.process_lock:
                resumers = list(ctx.active_resumers.values())
                ctx.active_resumers.clear()
            for r in resumers:
                try:
                    r.stop(timeout_sec=60)
                except Exception:
                    pass

            # Stop any still-running conflict generators so they don't outlive the run.
            conflict_generators = list(ctx.conflict_generators.values())
            ctx.conflict_generators.clear()
            for gen in conflict_generators:
                try:
                    gen.stop(timeout_sec=60)
                except Exception:
                    pass

            # Terminate any remaining background processes that were started.
            # Kill archiver last — it needs other processes to stop first so
            # migration status changes and it can exit gracefully.
            with ctx.process_lock:
                procs = list(ctx.processes.items())
                ctx.processes.clear()
            archiver_proc = None
            for name, proc in procs:
                if name == "archive_changes":
                    archiver_proc = (name, proc)
                    continue
                try:
                    H.log(f"cleanup: stopping process {name}")
                    H.kill(proc, timeout_sec=60)
                except Exception:
                    pass
            if archiver_proc:
                name, proc = archiver_proc
                H.log(f"cleanup: waiting 30s for archiver to exit gracefully...")
                try:
                    proc.wait(timeout=30)
                    H.log(f"cleanup: archiver exited on its own")
                except subprocess.TimeoutExpired:
                    H.log(f"cleanup: archiver did not exit, stopping it")
                    H.kill(proc, timeout_sec=60)
        except Exception:
            # Never fail the run due to cleanup.
            pass


if __name__ == "__main__":
    main()


