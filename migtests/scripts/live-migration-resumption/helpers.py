#!/usr/bin/env python3

from typing import Any, Dict, Callable
import os
import time
import threading
import random
import subprocess
import shutil
from datetime import datetime
import json
import sys


# -------------------------
# Config / Context helpers
# -------------------------

def load_config(path: str) -> Dict[str, Any]:
    import yaml  # local import to keep this file lightweight
    with open(path, "r") as f:
        return yaml.safe_load(f)


def merge_env(base: Dict[str, str], override: Dict[str, str] | None) -> Dict[str, str]:
    env = dict(base)
    if override:
        env.update({k: str(v) for k, v in override.items()})
    return env


class Context:
    def __init__(self, cfg: Dict[str, Any], run_id: str, env: Dict[str, str], test_root: str | None = None):
        self.cfg = cfg
        self.run_id = run_id
        self.env = env
        self.processes: Dict[str, subprocess.Popen] = {}
        self.artifacts_dir: str = cfg["artifacts_dir"]
        self.test_root: str | None = test_root
        self.stop_event = threading.Event()
        self.resumer_threads: Dict[str, list[threading.Thread]] = {}
        self.process_lock = threading.Lock()
        self.resumer_stop_flags: Dict[str, threading.Event] = {}


class ResumptionPolicy:
    def __init__(self, cfg: Dict[str, Any]):
        self.max_restarts = cfg["max_restarts"]
        self.min_interrupt_seconds = cfg["min_interrupt_seconds"]
        self.max_interrupt_seconds = cfg["max_interrupt_seconds"]
        self.min_restart_wait_seconds = cfg["min_restart_wait_seconds"]
        self.max_restart_wait_seconds = cfg["max_restart_wait_seconds"]



# -------------------------
# Minimal logging helpers
# -------------------------

def _ts() -> str:
    return datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%S.%fZ")


def log(msg: str) -> None:
    print(f"[{_ts()}] {msg}")


def log_stage_start(name: str) -> None:
    log(f"==> Start: {name}")


def log_stage_end(name: str, status: str = "OK") -> None:
    log(f"<== End: {name} [{status}]")


def log_resumption_event(ctx: Context, command: str, message: str | None = None, *, event: str | None = None, **fields: Any) -> None:
    """Log the message to stdout and persist the structured event to artifacts."""
    text = message or event or ""
    if text:
        log(f"[resumption:{command}] {text}")

    record: Dict[str, Any] = {
        "ts": _ts(),
        "command": command,
    }
    if event:
        record["event"] = event
    if message:
        record["message"] = message
    if fields:
        record.update(fields)

    try:
        os.makedirs(ctx.artifacts_dir, exist_ok=True)
        path = os.path.join(ctx.artifacts_dir, "resumption_events.ndjson")
        with open(path, "a") as f:
            f.write(json.dumps(record) + "\n")
    except Exception as exc:
        log(f"[resumption:{command}] failed to persist event: {exc}")


# -------------------------
# Scenario validation
# -------------------------

def _require_key(d: Dict[str, Any], key: str, ctx: str) -> Any:
    if key not in d:
        raise ValueError(f"Missing required key '{key}' in {ctx}")
    return d[key]


def _require_type(val: Any, typ, ctx: str, key: str) -> None:
    if not isinstance(val, typ):
        raise ValueError(f"Key '{key}' in {ctx} must be of type {typ.__name__}")


def validate_scenario(cfg: Dict[str, Any]) -> None:
    """Perform minimal validation of scenario structure;
    Required top-level keys: name (str), workflow (str), stages (list>0).
    Optional but expected containers: voyager (dict), generator (dict), dvt (dict), env (dict).
    Each stage must have: name (str), action (str). Additional fields are action-specific.
    """
    ctx = "scenario"
    _require_type(cfg, dict, ctx, "<root>")

    name = _require_key(cfg, "name", ctx)
    _require_type(name, str, ctx, "name")

    workflow = _require_key(cfg, "workflow", ctx)
    _require_type(workflow, str, ctx, "workflow")

    stages = _require_key(cfg, "stages", ctx)
    _require_type(stages, list, ctx, "stages")
    if len(stages) == 0:
        raise ValueError("'stages' must contain at least one stage")

    # Optional containers
    for opt_key, typ in ("voyager", dict), ("generator", dict), ("dvt", dict), ("env", dict):
        if opt_key in cfg and cfg[opt_key] is not None:
            _require_type(cfg[opt_key], typ, ctx, opt_key)

    # Stage-level checks
    for idx, st in enumerate(stages):
        sctx = f"stage[{idx}]"
        _require_type(st, dict, sctx, "stage")
        sname = _require_key(st, "name", sctx)
        _require_type(sname, str, sctx, "name")
        action = _require_key(st, "action", sctx)
        _require_type(action, str, sctx, "action")
        if action == "wait_for":
            cond = _require_key(st, "condition", sctx)
            _require_type(cond, str, sctx, "condition")

# -------------------------
# Polling / Timeouts / Conditions
# -------------------------

def poll_until(timeout_sec: int, interval_sec: int, fn: Callable[[], bool]) -> bool:
    deadline = time.time() + timeout_sec if timeout_sec else None
    while True:
        try:
            if fn():
                return True
        except Exception:
            # swallow and retry for simple polling semantics
            pass
        if deadline and time.time() > deadline:
            return False
        time.sleep(interval_sec)



def exporter_streaming(export_dir: str) -> bool:
    """Heuristic: streaming considered started when first queue segment has at least one event.
    """
    seg0 = os.path.join(export_dir, "data", "queue", "segment.0.ndjson")
    try:
        if not os.path.isfile(seg0):
            return False
        return os.path.getsize(seg0) > 0
    except (OSError, IOError):
        return False


def get_cutover_status(export_dir: str) -> str:
    """Query yb-voyager cutover status and return the status string.
    Returns status like "COMPLETED", "IN_PROGRESS", etc., or empty string if not found.
    """
    cmd = ["yb-voyager", "cutover", "status", "--export-dir", export_dir]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, check=False)
        for line in proc.stdout.splitlines():
            if "cutover to target status" in line:
                parts = line.split(":", 1)
                if len(parts) == 2:
                    return parts[1].strip()
        return ""
    except Exception:
        return ""

def backlog_marker_present(export_dir: str) -> bool:
    """
    Return True when the latest queue segment contains the marker insert for cutover_table.
    Looks for "cutover_table" and the literal "Last event before cutover" in the last non-empty line.
    """
    queue_dir = os.path.join(export_dir, "data", "queue")
    try:
        names = [n for n in os.listdir(queue_dir) if n.startswith("segment.") and n.endswith(".ndjson")]
        if not names:
            return False
        # segment.N.ndjson -> pick max N
        latest = max(names, key=lambda n: int(n.split(".")[1]))
        path = os.path.join(queue_dir, latest)
        if os.path.getsize(path) == 0:
            return False
        last: str | None = None
        with open(path, "r") as f:
            for line in f:
                s = line.strip()
                if s:
                    last = s
        return last is not None and ("cutover_table" in last) and ("Last event before cutover" in last)
    except OSError:
        return False


# -------------------------
# Process utilities
# -------------------------

def _cmd_str(cmd: list[str]) -> str:
    try:
        return " ".join(cmd)
    except Exception:
        return str(cmd)


def spawn(cmd: list[str], env: Dict[str, str]) -> subprocess.Popen:
    log(f"spawn: {_cmd_str(cmd)}")
    return subprocess.Popen(cmd, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)


def run_checked(cmd: list[str], env: Dict[str, str], description: str | None = None) -> None:
    """Run a command and raise RuntimeError on non-zero exit code.

    Captures stdout/stderr for error diagnostics.
    """
    log(f"run_checked: {_cmd_str(cmd)}")
    proc = subprocess.run(cmd, env=env, capture_output=True, text=True)
    if proc.returncode != 0:
        desc = f" ({description})" if description else ""
        raise RuntimeError(f"Command failed{desc}: {_cmd_str(cmd)}\nExit: {proc.returncode}\nSTDOUT:\n{proc.stdout}\nSTDERR:\n{proc.stderr}")


def wait_process(proc: subprocess.Popen, timeout_sec: int) -> bool:
    try:
        proc.wait(timeout=timeout_sec)
        return True
    except Exception:
        return False


def kill(proc: subprocess.Popen | None, timeout_sec: int = 10) -> None:
    if proc is None:
        return
    if proc.poll() is not None:
        return
    try:
        proc.terminate()
        if wait_process(proc, timeout_sec):
            return
        proc.kill()
        wait_process(proc, timeout_sec)
    except Exception:
        # Best effort; ignore
        pass


def restart_like(name: str, _old_proc: subprocess.Popen | None, ctx: Context) -> subprocess.Popen:
    # Reconstruct a command by semantic name.
    return start_command_by_name(name, ctx)


def stop_process(ctx: Context, name: str, graceful_timeout: int = 10) -> bool:
    with ctx.process_lock:
        proc = ctx.processes.get(name)
    if not proc:
        log(f"stop_process: no running process for {name}")
        return False
    kill(proc, timeout_sec=graceful_timeout)
    with ctx.process_lock:
        ctx.processes[name] = None
    log(f"stop_process: stopped {name}")
    return True


# -------------------------
# Voyager wrappers
# -------------------------

def to_kv_flags(d: Dict[str, Any] | None) -> list[str]:
    args: list[str] = []
    if not d:
        return args
    for k, v in d.items():
        if v is None or v == "":
            continue
        if isinstance(v, bool):
            args += [f"--{k}", "true" if v else "false"]
            continue
        args += [f"--{k}", str(v)]
    return args


def _merge_flags(base: Dict[str, Any], extra: Dict[str, Any] | None) -> Dict[str, Any]:
    merged = dict(base)
    if extra:
        for k, v in extra.items():
            merged[k] = v
    return merged


def _source_conn_flags(cfg: Dict[str, Any]) -> Dict[str, Any]:
    src = cfg.get("source", {})
    return {
        "source-db-type": src.get("type", "postgresql"),
        "source-db-host": src.get("host", ""),
        "source-db-port": src.get("port", ""),
        "source-db-user": src.get("user", ""),
        "source-db-password": src.get("password", ""),
        "source-db-name": src.get("database", ""),
        "source-db-schema": src.get("schema", ""),
    }


def _target_conn_flags(cfg: Dict[str, Any]) -> Dict[str, Any]:
    tgt = cfg.get("target", {})
    return {
        "target-db-host": tgt.get("host", ""),
        "target-db-port": tgt.get("port", ""),
        "target-db-user": tgt.get("user", ""),
        "target-db-password": tgt.get("password", ""),
        "target-db-name": tgt.get("database", ""),
    }


def _base_common_flags(cfg: Dict[str, Any]) -> Dict[str, Any]:
    return {
        "export-dir": cfg["export_dir"],
        "send-diagnostics": False,
    }


def build_export_data_cmd(cfg: Dict[str, Any]) -> list[str]:
    voyager_flags = (cfg.get("voyager", {}).get("export_data", {}) or {}).get("flags", {})
    base = _base_common_flags(cfg)
    base.update(_source_conn_flags(cfg))
    # Live migration default
    base["export-type"] = "snapshot-and-changes"
    # data command defaults
    base["disable-pb"] = True
    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "export", "data", "--yes"] + to_kv_flags(merged)


def build_import_data_cmd(cfg: Dict[str, Any]) -> list[str]:
    voyager_flags = (cfg.get("voyager", {}).get("import_data", {}) or {}).get("flags", {})
    base = _base_common_flags(cfg)
    base.update(_target_conn_flags(cfg))
    base["max-retries-streaming"] = 1
    base["skip-replication-checks"] = True
    # data command defaults
    base["disable-pb"] = True
    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "import", "data", "--yes"] + to_kv_flags(merged)


def build_import_schema_cmd(cfg: Dict[str, Any]) -> list[str]:
    voyager_flags = (cfg.get("voyager", {}).get("import_schema", {}) or {}).get("flags", {})
    base = {
        "export-dir": cfg["export_dir"],
    }
    base.update(_target_conn_flags(cfg))
    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "import", "schema", "--yes"] + to_kv_flags(merged)

def build_export_schema_cmd(cfg: Dict[str, Any]) -> list[str]:
    voyager_flags = (cfg.get("voyager", {}).get("export_schema", {}) or {}).get("flags", {})
    base = {
        "export-dir": cfg["export_dir"],
    }
    base.update(_source_conn_flags(cfg))
    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "export", "schema", "--yes"] + to_kv_flags(merged)


def build_cutover_to_target_cmd(cfg: Dict[str, Any]) -> list[str]:
    voyager_flags = (cfg.get("voyager", {}).get("cutover_to_target", {}) or {}).get("flags", {})
    base = {
        "export-dir": cfg["export_dir"],
    }
    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "initiate", "cutover", "to", "target", "--yes"] + to_kv_flags(merged)


def import_schema(cfg: Dict[str, Any], env: Dict[str, str]) -> int:
    cmd = build_import_schema_cmd(cfg)
    # Use checked run so non-zero exit propagates with stdout/stderr for diagnostics
    run_checked(cmd, env, description="import_schema")
    return 0

def export_schema(cfg: Dict[str, Any], env: Dict[str, str]) -> int:
    cmd = build_export_schema_cmd(cfg)
    run_checked(cmd, env, description="export_schema")
    return 0


def cutover_to_target(cfg: Dict[str, Any], env: Dict[str, str]) -> None:
    cmd = build_cutover_to_target_cmd(cfg)
    run_checked(cmd, env, description="cutover_to_target")


def start_exporter(ctx: Context) -> subprocess.Popen:
    cmd = build_export_data_cmd(ctx.cfg)
    return spawn(cmd, ctx.env)


def start_importer(ctx: Context) -> subprocess.Popen:
    cmd = build_import_data_cmd(ctx.cfg)
    return spawn(cmd, ctx.env)


def start_command_by_name(name: str, ctx: Context) -> subprocess.Popen:
    mapping: Dict[str, Callable[[], subprocess.Popen]] = {
        "export_data": lambda: start_exporter(ctx),
        "import_data": lambda: start_importer(ctx),
        "export_from_target": lambda: export_from_target(ctx.cfg, ctx.env),
        "import_to_source_replica": lambda: import_to_source_replica(ctx.cfg, ctx.env),
        "import_to_source": lambda: import_to_source(ctx.cfg, ctx.env),
    }
    try:
        return mapping[name]()
    except KeyError as exc:
        raise ValueError(f"Unsupported command for restart: {name}") from exc


def export_from_target(cfg: Dict[str, Any], env: Dict[str, str]) -> subprocess.Popen:
    flags = cfg["voyager"].get("export_from_target", {}).get("flags", {})
    return spawn(["yb-voyager", "export", "data", "from", "target", "--export-dir", cfg["export_dir"], *to_kv_flags(flags)], env)


def import_to_source_replica(cfg: Dict[str, Any], env: Dict[str, str]) -> subprocess.Popen:
    flags = cfg["voyager"].get("import_to_source_or_replica", {}).get("flags", {})
    return spawn(["yb-voyager", "import", "data", "to", "source-replica", "--export-dir", cfg["export_dir"], *to_kv_flags(flags)], env)


def import_to_source(cfg: Dict[str, Any], env: Dict[str, str]) -> subprocess.Popen:
    flags = cfg["voyager"].get("import_to_source_or_replica", {}).get("flags", {})
    return spawn(["yb-voyager", "import", "data", "to", "source", "--export-dir", cfg["export_dir"], *to_kv_flags(flags)], env)



# -------------------------
# Generator / DVT
# -------------------------

def resolve_generator_config(gen_cfg: Dict[str, Any] | None, run_id: str, test_root: str | None) -> str:
    gen_cfg = gen_cfg or {}
    config_path = gen_cfg.get("config_path")
    if config_path:
        final_path = os.path.abspath(config_path)
        if not os.path.isfile(final_path):
            raise FileNotFoundError(f"generator config_path not found: {final_path}")
        return final_path

    inline_cfg = gen_cfg.get("config") or gen_cfg.get("config_inline")
    if inline_cfg:
        tmp_dir = os.path.join("/tmp", run_id)
        os.makedirs(tmp_dir, exist_ok=True)
        final_path = os.path.join(tmp_dir, "event-generator.yaml")
        import yaml  # local import
        with open(final_path, "w") as f:
            yaml.safe_dump(inline_cfg, f, sort_keys=False)
        return final_path

    fallback_root = test_root or os.getcwd()
    fallback_path = os.path.join(fallback_root, "event-generator.yaml")
    if not os.path.isfile(fallback_path):
        raise FileNotFoundError(f"generator fallback config not found: {fallback_path}")
    return fallback_path


def start_generator(final_cfg_path: str, env: Dict[str, str]) -> subprocess.Popen:
    generator_dir = "/home/ubuntu/yb-voyager/migtests/scripts/event-generator"
    generator_main = os.path.join(generator_dir, "generator.py")
    log(f"starting generator with --config {final_cfg_path}")
    return subprocess.Popen(
        [sys.executable, generator_main, "--config", final_cfg_path],
        env=env,
        stdout=subprocess.DEVNULL,
        text=True,
        cwd=generator_dir,
    )


def start_generator_from_context(ctx: Context) -> subprocess.Popen:
    gen_cfg = ctx.cfg.get("generator")
    final_cfg_path = resolve_generator_config(gen_cfg, ctx.run_id, ctx.test_root)
    return start_generator(final_cfg_path, ctx.env)


def stop_generator(proc: subprocess.Popen | None, graceful_timeout_sec: int) -> None:
    kill(proc, timeout_sec=graceful_timeout_sec)


def run_dvt(dvt_cfg: Dict[str, Any], env: Dict[str, str]) -> int:
    # Placeholder for DVT execution
    return 0


# -------------------------
# Resumption manager
# -------------------------

def start_resumptions_for_stage(resumption_cfg: Dict[str, Any] | None, ctx: Context) -> None:
    if not resumption_cfg:
        return
    to_start: list[tuple[str, ResumptionPolicy, threading.Event]] = []
    with ctx.process_lock:
        for cmd, cfg in resumption_cfg.items():
            if cmd in ctx.resumer_stop_flags:
                log_resumption_event(ctx, cmd, "resumer already running; skipping new start", event="duplicate_start")
                continue
            proc = ctx.processes.get(cmd)
            if proc is None:
                log_resumption_event(ctx, cmd, "requested resumption but process not running; skipping", event="missing_process")
                continue
            policy = ResumptionPolicy(cfg)
            if policy.max_restarts <= 0:
                continue
            stop_flag = threading.Event()
            ctx.resumer_stop_flags[cmd] = stop_flag
            ctx.resumer_threads.setdefault(cmd, [])
            to_start.append((cmd, policy, stop_flag))
    for cmd, policy, stop_flag in to_start:
        t = threading.Thread(
            target=_resumer_worker,
            args=(cmd, policy, stop_flag, ctx),
            daemon=True,
        )
        t.start()
        with ctx.process_lock:
            ctx.resumer_threads.setdefault(cmd, []).append(t)


def _wait_for_stop(stop_flag: threading.Event, ctx: Context, seconds: int) -> bool:
    if seconds <= 0:
        return stop_flag.is_set() or ctx.stop_event.is_set()
    deadline = time.time() + seconds
    while True:
        if stop_flag.is_set() or ctx.stop_event.is_set():
            return True
        remaining = deadline - time.time()
        if remaining <= 0:
            return False
        time.sleep(min(remaining, 1.0))


def _restart_command(cmd: str, old_proc: subprocess.Popen | None, ctx: Context) -> subprocess.Popen:
    proc = restart_like(cmd, old_proc, ctx)
    with ctx.process_lock:
        ctx.processes[cmd] = proc
    log_resumption_event(ctx, cmd, f"restarted process pid={proc.pid}", event="restart")
    return proc


def _resumer_worker(cmd: str, policy: ResumptionPolicy, stop_flag: threading.Event, ctx: Context) -> None:
    attempt = 0
    try:
        while attempt < policy.max_restarts and not ctx.stop_event.is_set() and not stop_flag.is_set():
            with ctx.process_lock:
                proc = ctx.processes.get(cmd)
            if not proc:
                log_resumption_event(ctx, cmd, "no running process; exiting resumer", event="no_process")
                return

            interrupt_delay = random.randint(policy.min_interrupt_seconds, policy.max_interrupt_seconds)
            log_resumption_event(
                ctx,
                cmd,
                f"attempt {attempt + 1}/{policy.max_restarts}: interrupting after {interrupt_delay}s",
                event="scheduled_interrupt",
                attempt=attempt + 1,
                max_restarts=policy.max_restarts,
                interrupt_delay_sec=interrupt_delay,
            )
            if _wait_for_stop(stop_flag, ctx, interrupt_delay):
                log_resumption_event(ctx, cmd, "stop requested before interrupt; exiting resumer", event="stop_before_interrupt")
                return

            kill(proc)
            log_resumption_event(ctx, cmd, f"killed process pid={proc.pid}", event="killed", pid=proc.pid)

            restart_delay = random.randint(policy.min_restart_wait_seconds, policy.max_restart_wait_seconds)
            log_resumption_event(
                ctx,
                cmd,
                f"waiting {restart_delay}s before restart",
                event="scheduled_restart",
                restart_delay_sec=restart_delay,
                next_attempt=attempt + 1,
            )
            if _wait_for_stop(stop_flag, ctx, restart_delay):
                log_resumption_event(ctx, cmd, "stop requested before restart; attempting final restart before exit", event="stop_before_restart")
                try:
                    _restart_command(cmd, proc, ctx)
                    log_resumption_event(ctx, cmd, "final restart complete; exiting resumer", event="final_restart_success")
                except Exception as exc:
                    log_resumption_event(ctx, cmd, f"final restart failed: {exc}", event="final_restart_failed", error=str(exc))
                return

            try:
                proc = _restart_command(cmd, proc, ctx)
            except Exception as exc:
                log_resumption_event(ctx, cmd, f"restart failed: {exc}", event="restart_failed", error=str(exc))
                return

            log_resumption_event(ctx, cmd, f"restart succeeded with pid={proc.pid}", event="restart_success", pid=proc.pid)
            attempt += 1
    finally:
        current = threading.current_thread()
        with ctx.process_lock:
            threads = ctx.resumer_threads.get(cmd)
            if threads and current in threads:
                threads.remove(current)
                if not threads:
                    ctx.resumer_threads.pop(cmd, None)
                    ctx.resumer_stop_flags.pop(cmd, None)


def stop_resumptions_for_command(cmd: str, ctx: Context, timeout_sec: int = 30) -> None:
    with ctx.process_lock:
        threads = list(ctx.resumer_threads.get(cmd) or [])
        stop_flag = ctx.resumer_stop_flags.get(cmd)
    if not threads or not stop_flag:
        log_resumption_event(ctx, cmd, "no resumer threads to stop", event="no_threads")
        return
    stop_flag.set()
    for t in threads:
        t.join(timeout_sec)
    with ctx.process_lock:
        ctx.resumer_threads.pop(cmd, None)
        ctx.resumer_stop_flags.pop(cmd, None)
    log_resumption_event(ctx, cmd, "resumptions stopped", event="stopped")


# -------------------------
# Artifacts / Logs 
# -------------------------

def _iter_log_files(logs_dir: str):
    if not os.path.isdir(logs_dir):
        return
    for root, _dirs, files in os.walk(logs_dir):
        for f in files:
            yield os.path.join(root, f)


def scan_logs_for_errors(export_dir: str, artifacts_dir: str, patterns: list[str] | None = None) -> None:
    patterns = patterns or ["ERROR", "FATAL", "WARN"]
    logs_dir = os.path.join(export_dir, "logs")
    scan_dir = os.path.join(artifacts_dir, "log_scans")
    os.makedirs(scan_dir, exist_ok=True)
    
    for fp in _iter_log_files(logs_dir):
        findings: list[str] = []
        try:
            with open(fp, "r", errors="ignore") as f:
                for line in f:
                    if any(pat in line for pat in patterns):
                        findings.append(line.rstrip())
        except Exception:
            continue
        
        if findings:
            basename = os.path.basename(fp)
            out = os.path.join(scan_dir, f"{basename}.scan.txt")
            with open(out, "w") as outf:
                for row in findings:
                    outf.write(row + "\n")


def snapshot_msr_and_stats(export_dir: str, artifacts_dir: str) -> None:
    metainfo_dir = os.path.join(export_dir, "metainfo")
    if not os.path.isdir(metainfo_dir):
        return

    dest_dir = os.path.join(artifacts_dir, "metainfo")

    try:
        shutil.copytree(metainfo_dir, dest_dir, dirs_exist_ok=True)
    except Exception:
        # Best effort; ignore failures
        pass


def append_stage_summary(artifacts_dir: str, stage_name: str, start_ts: str, end_ts: str, status: str, error: str | None = None) -> None:
    os.makedirs(artifacts_dir, exist_ok=True)
    summary_path = os.path.join(artifacts_dir, "stage_summary.ndjson")
    record = {
        "stage": stage_name,
        "start_ts": start_ts,
        "end_ts": end_ts,
        "status": status,
    }
    if error:
        record["error"] = error
    with open(summary_path, "a") as f:
        f.write(json.dumps(record) + "\n")


# -------------------------
# Path prep and cleanup (unconditional)
# -------------------------

def prepare_paths(test_root: str, export_dir: str, artifacts_dir: str) -> None:
    """Delete and recreate export_dir and artifacts_dir.

    Caller guarantees these paths are correct; this function does not add
    additional guardrails, per framework requirements.
    """
    for p in (export_dir, artifacts_dir):
        try:
            shutil.rmtree(p, ignore_errors=True)
        finally:
            os.makedirs(p, exist_ok=True)


# -------------------------
# SQL execution
# -------------------------

def run_sql_file(ctx: Context, sql_path: str, target: str = "source") -> None:
    """Execute an SQL file against a target using psql.

    - target: one of "source", "target", "source_replica" (if provided in cfg)
    """
    cfg = ctx.cfg or {}
    conn = cfg.get(target)
    if not isinstance(conn, dict):
        log(f"run_sql_file: missing connection for target={target}, skipping: {sql_path}")
        return
    host = str(conn.get("host", ""))
    port = str(conn.get("port", ""))
    db = str(conn.get("database", ""))
    user = str(conn.get("user", ""))
    password = conn.get("password")

    # Build psql command
    cmd = [
        "psql",
        "-h", host,
        "-p", port,
        "-U", user,
        "-d", db,
        "-v", "ON_ERROR_STOP=1",
        "-f", sql_path,
    ]
    env = dict(ctx.env)
    if password:
        env["PGPASSWORD"] = str(password)
    log(f"psql executing: {sql_path} on {target}@{host}:{port}/{db}")
    run_checked(cmd, env, description=f"run_sql_file:{target}")


