#!/usr/bin/env python3

from typing import Any, Dict, Callable, Optional, List
import os
import time
import threading
import random
import subprocess
import psycopg2
from psycopg2 import sql
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
        self.process_lock = threading.Lock()
        self.active_resumers: Dict[str, "Resumer"] = {}


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
    print(msg)


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

def _ensure(obj: Any, key: str | None, typ, ctx: str, *, required: bool = True, allow_none: bool = False) -> Any:
    if key is None:
        val = obj
        label = ctx
    else:
        if not isinstance(obj, dict):
            raise ValueError(f"{ctx} must be a mapping to access '{key}'")
        if key not in obj:
            if required:
                raise ValueError(f"Missing required key '{key}' in {ctx}")
            return None
        val = obj[key]
        label = key
    if val is None:
        if allow_none:
            return None
        raise ValueError(f"Key '{label}' in {ctx} must not be null")
    if not isinstance(val, typ):
        raise ValueError(f"Key '{label}' in {ctx} must be of type {typ.__name__}")
    return val


def validate_scenario(cfg: Dict[str, Any]) -> None:
    """Perform minimal validation of scenario structure;
    Required top-level keys: name (str), workflow (str), stages (list>0).
    Optional but expected containers: voyager (dict), generator (dict), dvt (dict), env (dict).
    Each stage must have: name (str), action (str). Additional fields are action-specific.
    """
    ctx = "scenario"
    _ensure(cfg, None, dict, ctx)

    _ensure(cfg, "name", str, ctx)
    _ensure(cfg, "workflow", str, ctx)

    stages = _ensure(cfg, "stages", list, ctx)
    if len(stages) == 0:
        raise ValueError("'stages' must contain at least one stage")

    # Optional containers
    for opt_key, typ in ("voyager", dict), ("generator", dict), ("dvt", dict), ("env", dict):
        _ensure(cfg, opt_key, typ, ctx, required=False, allow_none=True)

    # Stage-level checks
    for idx, st in enumerate(stages):
        sctx = f"stage[{idx}]"
        st = _ensure(st, None, dict, sctx)
        _ensure(st, "name", str, sctx)
        action = _ensure(st, "action", str, sctx)
        if action == "wait_for":
            _ensure(st, "condition", str, sctx)

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
    helper_dir = os.path.dirname(__file__)
    generator_dir = os.path.abspath(os.path.join(helper_dir, "..", "event-generator"))
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


def run_dvt(ctx: Context) -> None:
    run_row_count_validations(ctx)


# -------------------------
# Resumption manager
# -------------------------

class Resumer:
    """Encapsulates per-command resumption lifecycle."""

    def __init__(
        self,
        cmd: str,
        policy: ResumptionPolicy,
        ctx: Context,
        *,
        rng: Optional[random.Random] = None,
    ):
        self.cmd = cmd
        self.policy = policy
        self.ctx = ctx
        self.stop_flag = threading.Event()
        self._thread: threading.Thread | None = None
        self._rng = rng or random.Random()

    def start(self) -> None:
        if self._thread and self._thread.is_alive():
            return
        thread = threading.Thread(
            target=self._run_loop,
            name=f"resumer:{self.cmd}",
            daemon=True,
        )
        self._thread = thread
        thread.start()

    def stop(self, timeout_sec: int) -> None:
        self.stop_flag.set()
        thread = self._thread
        if thread:
            thread.join(timeout_sec)

    def is_alive(self) -> bool:
        return bool(self._thread and self._thread.is_alive())

    def _run_loop(self) -> None:
        attempt = 0
        try:
            while attempt < self.policy.max_restarts and not self._should_stop():
                proc = self._current_process()
                if not proc:
                    self._log("no running process; exiting resumer", event="no_process")
                    return

                interrupt_delay = self._rng.randint(
                    self.policy.min_interrupt_seconds,
                    self.policy.max_interrupt_seconds,
                )
                self._log(
                    f"attempt {attempt + 1}/{self.policy.max_restarts}: interrupting after {interrupt_delay}s",
                    event="scheduled_interrupt",
                    attempt=attempt + 1,
                    max_restarts=self.policy.max_restarts,
                    interrupt_delay_sec=interrupt_delay,
                )
                if self._wait(interrupt_delay):
                    self._log("stop requested before interrupt; exiting resumer", event="stop_before_interrupt")
                    return

                kill(proc)
                self._log(
                    f"killed process pid={proc.pid}",
                    event="killed",
                    pid=proc.pid,
                )

                restart_delay = self._rng.randint(
                    self.policy.min_restart_wait_seconds,
                    self.policy.max_restart_wait_seconds,
                )
                self._log(
                    f"waiting {restart_delay}s before restart",
                    event="scheduled_restart",
                    restart_delay_sec=restart_delay,
                    next_attempt=attempt + 1,
                )
                if self._wait(restart_delay):
                    self._log(
                        "stop requested before restart; attempting final restart before exit",
                        event="stop_before_restart",
                    )
                    self._safe_restart(proc, final=True)
                    return

                proc = self._safe_restart(proc)
                if not proc:
                    return
                attempt += 1
        finally:
            self._deregister()

    def _safe_restart(self, old_proc: subprocess.Popen | None, *, final: bool = False) -> subprocess.Popen | None:
        try:
            proc = restart_like(self.cmd, old_proc, self.ctx)
            with self.ctx.process_lock:
                self.ctx.processes[self.cmd] = proc
        except Exception as exc:
            event = "final_restart_failed" if final else "restart_failed"
            self._log(
                f"{'final ' if final else ''}restart failed: {exc}",
                event=event,
                error=str(exc),
            )
            return None

        if final:
            self._log(
                "final restart complete; exiting resumer",
                event="final_restart_success",
                pid=proc.pid,
            )
        else:
            self._log(f"restarted process pid={proc.pid}", event="restart", pid=proc.pid)
            self._log(
                f"restart succeeded with pid={proc.pid}",
                event="restart_success",
                pid=proc.pid,
            )
        return proc

    def _current_process(self) -> subprocess.Popen | None:
        with self.ctx.process_lock:
            return self.ctx.processes.get(self.cmd)

    def _wait(self, seconds: int) -> bool:
        if seconds <= 0:
            return self._should_stop()
        deadline = time.time() + seconds
        while not self._should_stop():
            remaining = deadline - time.time()
            if remaining <= 0:
                return False
            time.sleep(min(remaining, 1.0))
        return True

    def _should_stop(self) -> bool:
        return self.stop_flag.is_set() or self.ctx.stop_event.is_set()

    def _log(self, message: str, *, event: str | None = None, **fields: Any) -> None:
        log_resumption_event(self.ctx, self.cmd, message, event=event, **fields)

    def _deregister(self) -> None:
        with self.ctx.process_lock:
            current = self.ctx.active_resumers.get(self.cmd)
            if current is self:
                self.ctx.active_resumers.pop(self.cmd, None)


def start_resumptions_for_stage(resumption_cfg: Dict[str, Any] | None, ctx: Context) -> None:
    if not resumption_cfg:
        return
    to_start: list[Resumer] = []
    with ctx.process_lock:
        for cmd, cfg in resumption_cfg.items():
            if cmd in ctx.active_resumers:
                log_resumption_event(ctx, cmd, "resumer already running; skipping new start", event="duplicate_start")
                continue
            proc = ctx.processes.get(cmd)
            if proc is None:
                log_resumption_event(ctx, cmd, "requested resumption but process not running; skipping", event="missing_process")
                continue
            policy = ResumptionPolicy(cfg)
            if policy.max_restarts <= 0:
                continue
            resumer = Resumer(cmd, policy, ctx)
            ctx.active_resumers[cmd] = resumer
            to_start.append(resumer)
    for resumer in to_start:
        resumer.start()


def stop_resumptions_for_command(cmd: str, ctx: Context, timeout_sec: int = 30) -> None:
    with ctx.process_lock:
        resumer = ctx.active_resumers.pop(cmd, None)
    if not resumer:
        log_resumption_event(ctx, cmd, "no resumer threads to stop", event="no_threads")
        return
    resumer.stop(timeout_sec)
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
    patterns = patterns or ["ERROR", "FATAL", "WARN", "Discrepancy in committed batch", "unexpected rows affected for event with"]
    logs_dir = os.path.join(export_dir, "logs")
    scan_dir = os.path.join(artifacts_dir, "log_scans")
    os.makedirs(scan_dir, exist_ok=True)
    
    for fp in _iter_log_files(logs_dir):
        findings: list[str] = []
        try:
            with open(fp, "r", errors="ignore") as f:
                for line in f:
                    lower_line = line.lower()
                    if any(pat.lower() in lower_line for pat in patterns):
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

def _source_connection(cfg: Dict[str, Any]) -> psycopg2.extensions.connection:
    src = cfg["source"]
    return psycopg2.connect(
        host=str(src["host"]),
        port=int(src["port"]),
        dbname=str(src["database"]),
        user=str(src["user"]),
        password=str(src.get("password")),
    )


def _target_connection(cfg: Dict[str, Any]) -> psycopg2.extensions.connection:
    tgt = cfg["target"]
    return psycopg2.connect(
        host=str(tgt["host"]),
        port=int(tgt["port"]),
        dbname=str(tgt["database"]),
        user=str(tgt["user"]),
        password=str(tgt.get("password")),
    )


def run_sql_file(ctx: Context, sql_path: str, target: str = "source") -> None:
    conn_cfg = ctx.cfg[target]
    cmd = [
        "psql",
        "-h", str(conn_cfg["host"]),
        "-p", str(conn_cfg["port"]),
        "-U", str(conn_cfg["user"]),
        "-d", str(conn_cfg["database"]),
        "-v", "ON_ERROR_STOP=1",
        "-f", sql_path,
    ]
    env = dict(ctx.env)
    password = conn_cfg.get("password")
    if password is not None:
        env["PGPASSWORD"] = str(password)
    log(
        f"psql executing: {sql_path} on "
        f"{target}@{conn_cfg['host']}:{conn_cfg['port']}/{conn_cfg['database']}"
    )
    run_checked(cmd, env, description=f"run_sql_file:{target}")


def list_source_tables(cfg: Dict[str, Any]) -> List[str]:
    schema = cfg["source"]["schema"]
    conn = _source_connection(cfg)
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT table_name
                FROM information_schema.tables
                WHERE table_schema = %s
                  AND table_type = 'BASE TABLE'
                ORDER BY table_name
                """,
                (schema,),
            )
            rows = cur.fetchall()
    finally:
        conn.close()

    return [row[0] for row in rows]


def get_table_primary_key(cfg: Dict[str, Any], table: str) -> List[str]:
    schema = cfg["source"]["schema"]
    conn = _source_connection(cfg)
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT kcu.column_name
                FROM information_schema.table_constraints tc
                JOIN information_schema.key_column_usage kcu
                  ON tc.constraint_name = kcu.constraint_name
                 AND tc.table_schema = kcu.table_schema
                WHERE tc.constraint_type = 'PRIMARY KEY'
                  AND tc.table_schema = %s
                  AND tc.table_name = %s
                ORDER BY kcu.ordinal_position
                """,
                (schema, table),
            )
            rows = cur.fetchall()
    finally:
        conn.close()

    return [row[0] for row in rows]


def _fetch_table_count(conn: psycopg2.extensions.connection, schema: str, table: str) -> int:
    with conn.cursor() as cur:
        query = sql.SQL("SELECT COUNT(*) FROM {}.{}").format(
            sql.Identifier(schema),
            sql.Identifier(table),
        )
        cur.execute(query)
        (count,) = cur.fetchone()
        return int(count)


# -------------------------
# DVT helpers
# -------------------------

def _run_dvt_command(cmd: list[str], env: Dict[str, str]) -> subprocess.CompletedProcess:
    log(f"dvt: {' '.join(cmd)}")
    proc = subprocess.run(cmd, env=env, capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError(
            "DVT command failed:\n"
            f"cmd: {' '.join(cmd)}\n"
            f"stdout:\n{proc.stdout}\n"
            f"stderr:\n{proc.stderr}"
        )
    return proc


def _dvt_connection_args(name: str, db_cfg: Dict[str, Any]) -> list[str]:
    args = [
        "data-validation",
        "connections",
        "add",
        "--connection-name",
        name,
        "Postgres",
        "--host",
        str(db_cfg["host"]),
        "--port",
        str(db_cfg["port"]),
        "--user",
        str(db_cfg["user"]),
        "--database",
        str(db_cfg["database"]),
    ]
    password = db_cfg.get("password")
    if password is not None:
        args += ["--password", str(password)]
    return args


def setup_dvt_connections(ctx: Context) -> tuple[Dict[str, str], str, str]:
    """Prepare isolated DVT connection configs for source and target."""
    conn_dir = os.path.join(ctx.artifacts_dir, "dvt", "connections")
    shutil.rmtree(conn_dir, ignore_errors=True)
    os.makedirs(conn_dir, exist_ok=True)

    env = dict(ctx.env)
    env["PSO_DV_CONN_HOME"] = conn_dir

    source_name = f"{ctx.run_id}_source"
    target_name = f"{ctx.run_id}_target"

    run_checked(_dvt_connection_args(source_name, ctx.cfg["source"]), env, description="dvt_connection:source")
    run_checked(_dvt_connection_args(target_name, ctx.cfg["target"]), env, description="dvt_connection:target")

    return env, source_name, target_name


def run_column_hash_validations(ctx: Context) -> None:
    """Run DVT column hash validations for each table."""
    env, source_conn, target_conn = setup_dvt_connections(ctx)
    schema = ctx.cfg["source"]["schema"]
    tables = list_source_tables(ctx.cfg)

    out_dir = os.path.join(ctx.artifacts_dir, "dvt", "column_hashes")
    os.makedirs(out_dir, exist_ok=True)

    for table in tables:
        pk = get_table_primary_key(ctx.cfg, table)
        pk_arg = ",".join(pk)
        tables_list = f"{schema}.{table}={schema}.{table}"
        cmd = [
            "data-validation",
            "validate",
            "row",
            "--source-conn", source_conn,
            "--target-conn", target_conn,
            "--tables-list", tables_list,
            "--primary-keys", pk_arg,
            "--hash", "*",
            "--format", "json",
        ]
        proc = _run_dvt_command(cmd, env)
        log_path = os.path.join(out_dir, f"{table}.json")
        with open(log_path, "w") as f:
            f.write(proc.stdout)
        if proc.stderr:
            err_path = os.path.join(out_dir, f"{table}.stderr")
            with open(err_path, "w") as f:
                f.write(proc.stderr)

        try:
            parsed = json.loads(proc.stdout)
        except json.JSONDecodeError as exc:
            raise RuntimeError(f"failed to parse DVT hash output for table {table}: {exc}") from exc

        def _records(obj: Any) -> List[Dict[str, Any]]:
            if isinstance(obj, list):
                return [x for x in obj if isinstance(x, dict)]
            if isinstance(obj, dict):
                values = list(obj.values())
                if all(isinstance(v, dict) for v in values):
                    return values
                return [obj]
            return []

        statuses = [
            rec.get("validation_status") or rec.get("status") or ""
            for rec in _records(parsed)
        ]
        statuses = [s.lower() for s in statuses if s]
        if not statuses or any(st != "success" for st in statuses):
            raise RuntimeError(f"column hash validation failed for table {table}")


def run_row_count_validations(ctx: Context) -> None:
    """Compare row counts between source and target using direct SQL."""
    cfg = ctx.cfg
    schema = cfg["source"]["schema"]
    tables = list_source_tables(cfg)

    out_dir = os.path.join(ctx.artifacts_dir, "dvt", "row_counts")
    os.makedirs(out_dir, exist_ok=True)

    src_conn = _source_connection(cfg)
    tgt_conn = _target_connection(cfg)
    try:
        for table in tables:
            src_count = _fetch_table_count(src_conn, schema, table)
            tgt_count = _fetch_table_count(tgt_conn, schema, table)

            record = {
                "table": table,
                "source_count": src_count,
                "target_count": tgt_count,
                "status": "success" if src_count == tgt_count else "mismatch",
            }
            out_path = os.path.join(out_dir, f"{table}.json")
            with open(out_path, "w") as f:
                f.write(json.dumps(record))

            if src_count != tgt_count:
                raise RuntimeError(
                    f"row count validation failed for table {table}: "
                    f"source={src_count}, target={tgt_count}"
                )
    finally:
        src_conn.close()
        tgt_conn.close()
