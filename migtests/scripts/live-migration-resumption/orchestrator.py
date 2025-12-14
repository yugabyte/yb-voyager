#!/usr/bin/env python3

import os
import sys
import argparse
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
        return
    # Resolve relative to test_root if provided
    if ctx.test_root and not os.path.isabs(sql_path):
        sql_path = os.path.join(ctx.test_root, sql_path)
    H.run_sql_file(ctx, sql_path, target, use_admin=use_admin)


@action("voyager_export_schema")
def export_schema_action(stage: Dict[str, Any], ctx: Any) -> None:
    H.export_schema(ctx.cfg, ctx.env)

@action("voyager_import_schema")
def import_schema_action(stage: Dict[str, Any], ctx: Any) -> None:
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


@action("voyager_export_start")
def export_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    with ctx.process_lock:
        ctx.processes["export_data"] = H.start_exporter(ctx)


@action("voyager_export_from_target_start")
def export_from_target_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Start yb-voyager export-from-target process for fallback."""
    with ctx.process_lock:
        ctx.processes["export_from_target"] = H.export_from_target(ctx.cfg, ctx.env)


@action("voyager_import_start")
def import_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    with ctx.process_lock:
        ctx.processes["import_data"] = H.start_importer(ctx)


@action("voyager_import_to_source_start")
def import_to_source_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Start yb-voyager import-to-source process for fallback."""
    with ctx.process_lock:
        ctx.processes["import_to_source"] = H.import_to_source(ctx.cfg, ctx.env)


@action("voyager_import_to_source_replica_start")
def import_to_source_replica_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Start yb-voyager import-to-source-replica process for fall-forward."""
    with ctx.process_lock:
        ctx.processes["import_to_source_replica"] = H.import_to_source_replica(ctx.cfg, ctx.env)


@action("voyager_stop_command")
def stop_command_action(stage: Dict[str, Any], ctx: Any) -> None:
    command = stage.get("command")
    timeout = int(stage.get("graceful_timeout_sec", 20))
    H.stop_process(ctx, command, graceful_timeout=timeout)


_WAIT_FOR_CONDITIONS: Dict[str, Dict[str, Any]] = {
    "exporter_in_streaming_phase": {
        "interval": 5,
        "predicate": lambda ctx: H.exporter_streaming(ctx.cfg["export_dir"]),
    },
    "remaining_events_eq_0": {
        "interval": 5,
        "predicate": lambda ctx: H.backlog_marker_present(ctx.cfg["export_dir"]),
    },
    "cutover_to_target_status_completed": {
        "interval": 10,
        "predicate": lambda ctx: H.get_cutover_status(ctx.cfg["export_dir"], mode="target") == "COMPLETED",
    },
    "cutover_to_source_status_completed": {
        "interval": 10,
        "predicate": lambda ctx: H.get_cutover_status(ctx.cfg["export_dir"], mode="source") == "COMPLETED",
    },
    "cutover_to_source_replica_status_completed": {
        "interval": 10,
        "predicate": lambda ctx: H.get_cutover_status(ctx.cfg["export_dir"], mode="source-replica") == "COMPLETED",
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
def cutover_to_target_action(stage: Dict[str, Any], ctx: Any) -> None:
    H.initiate_cutover(ctx.cfg, ctx.env, "target")


@action("cutover_to_source")
def cutover_to_source_action(stage: Dict[str, Any], ctx: Any) -> None:
    H.initiate_cutover(ctx.cfg, ctx.env, "source")


@action("cutover_to_source_replica")
def cutover_to_source_replica_action(stage: Dict[str, Any], ctx: Any) -> None:
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


@action("start_resumptions")
def start_resumptions_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Start per-command resumption workers based on the provided resumption map.
    """
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
        import time as _t
        H.log(f"sleeping for {secs} seconds")
        _t.sleep(secs)


@action("reset_databases")
def reset_databases_action(stage: Dict[str, Any], ctx: Any) -> None:
    """Drop and recreate source/target databases using admin credentials."""
    targets = stage.get("targets") or ["source", "target"]
    for target_name in targets:
        H.reset_database_for_role(target_name, ctx)


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
    parser.add_argument("--resolve-only", action="store_true", help="Only load and resolve paths, then exit")
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

    run_id = cfg.get("run_id", "run")
    env = H.merge_env(os.environ, cfg.get("env"))

    if args.resolve_only:
        print(f"Scenario: {scenario_path}")
        print(f"Test root: {test_root}")
        print(f"Export dir: {cfg['export_dir']}")
        print(f"Artifacts dir: {cfg['artifacts_dir']}")
        sys.exit(0)

    # Prepare paths by cleaning and recreating export-dir and artifacts
    H.prepare_paths(cfg["export_dir"], cfg["artifacts_dir"])

    ctx = H.Context(cfg=cfg, run_id=run_id, env=env, test_root=test_root)
    had_failure = False

    try:
        for stage in cfg["stages"]:
            stage_name = stage.get("name", "<unnamed>")
            H.log_stage_start(stage_name)
            start_ts = H._ts()
            try:
                get_action(stage["action"])(stage, ctx)
                end_ts = H._ts()
                H.append_stage_summary(cfg["artifacts_dir"], stage_name, start_ts, end_ts, status="OK")
                H.log_stage_end(stage_name, status="OK")
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


if __name__ == "__main__":
    main()


