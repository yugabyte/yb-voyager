#!/usr/bin/env python3

import os
import sys
import argparse
from typing import Any, Dict, Callable

try:
    import helpers as H  # package-relative
except ImportError:
    import sys as _sys
    _sys.path.append(os.path.dirname(__file__))
    import helpers as H  # direct script execution


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
    # Execute SQL against source/target based on stage params (placeholder)
    sql_path = stage.get("sql_path")
    target = stage.get("target", "source")
    if not sql_path:
        return
    # Resolve relative to test_root if provided
    if ctx.test_root and not os.path.isabs(sql_path):
        sql_path = os.path.join(ctx.test_root, sql_path)
    H.run_sql_file(ctx, sql_path, target)


@action("voyager_export_schema")
def export_schema_action(stage: Dict[str, Any], ctx: Any) -> None:
    H.export_schema(ctx.cfg, ctx.env)

@action("voyager_import_schema")
def import_schema_action(stage: Dict[str, Any], ctx: Any) -> None:
    H.import_schema(ctx.cfg, ctx.env)

@action("generator_start")
def generator_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    ctx.processes["generator"] = H.start_generator_from_context(ctx)


@action("generator_stop")
def generator_stop_action(stage: Dict[str, Any], ctx: Any) -> None:
    H.stop_generator(ctx.processes.pop("generator", None), int(stage.get("graceful_timeout_sec", 60)))


@action("voyager_export_start")
def export_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    with ctx.process_lock:
        ctx.processes["export_data"] = H.start_exporter(ctx)


@action("voyager_import_start")
def import_start_action(stage: Dict[str, Any], ctx: Any) -> None:
    with ctx.process_lock:
        ctx.processes["import_data"] = H.start_importer(ctx)


@action("voyager_stop_command")
def stop_command_action(stage: Dict[str, Any], ctx: Any) -> None:
    command = stage.get("command")
    timeout = int(stage.get("graceful_timeout_sec", 20))
    H.stop_process(ctx, command, graceful_timeout=timeout)


@action("wait_for")
def wait_for_action(stage: Dict[str, Any], ctx: Any) -> None:
    cond = stage["condition"]
    timeout_sec = int(stage.get("timeout_sec", 0))  # 0 => no overall timeout
    if cond == "exporter_in_streaming_phase":
        ok = H.poll_until(timeout_sec, 5, lambda: H.exporter_streaming(ctx.cfg["export_dir"]))
    elif cond == "remaining_events_eq_0":
        ok = H.poll_until(timeout_sec, 5, lambda: H.backlog_marker_present(ctx.cfg["export_dir"]))
    elif cond == "cutover_status_completed":
        ok = H.poll_until(timeout_sec, 10, lambda: H.get_cutover_status(ctx.cfg["export_dir"]) == "COMPLETED")
    else:
        raise ValueError(f"unknown condition: {cond}")
    if not ok:
        raise TimeoutError(cond)


@action("voyager_cutover")
def cutover_action(stage: Dict[str, Any], ctx: Any) -> None:
    H.cutover_to_target(ctx.cfg, ctx.env)


@action("dvt_run")
def dvt_run_action(stage: Dict[str, Any], ctx: Any) -> None:
    H.run_dvt(ctx.cfg.get("dvt", {}), ctx.env)


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


# -------------------------
# Runner
# -------------------------

def when_clause_passes(stage: Dict[str, Any], cfg: Dict[str, Any]) -> bool:
    expr = stage.get("when")
    if not expr:
        return True
    wf = cfg.get("workflow")
    return ((wf == "fall-forward" and "fall-forward" in expr) or
            (wf == "fall-back" and "fall-back" in expr) or
            (wf == "normal" and "normal" in expr))


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
    H.prepare_paths(test_root, cfg["export_dir"], cfg["artifacts_dir"])

    ctx = H.Context(cfg=cfg, run_id=run_id, env=env, test_root=test_root)

    try:
        for stage in cfg["stages"]:
            if not when_clause_passes(stage, cfg):
                continue
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
                raise
    finally:
        # Always capture artifacts/logs at the end regardless of success or failure
        H.scan_logs_for_errors(cfg["export_dir"], cfg["artifacts_dir"])
        H.snapshot_msr_and_stats(cfg["export_dir"], cfg["artifacts_dir"])


if __name__ == "__main__":
    main()


