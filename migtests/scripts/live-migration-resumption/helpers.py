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
import yaml


# -------------------------
# Config / Context helpers
# -------------------------

def load_config(path: str) -> Dict[str, Any]:
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
    Required top-level keys: name (str), stages (list>0).
    Each stage must have: name (str), action (str). Additional fields are action-specific.
    """
    ctx = "scenario"
    _ensure(cfg, None, dict, ctx)

    _ensure(cfg, "name", str, ctx)

    stages = _ensure(cfg, "stages", list, ctx)
    if len(stages) == 0:
        raise ValueError("'stages' must contain at least one stage")


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


def get_cutover_status(export_dir: str, mode: str = "target") -> str:
    mode_to_key = {
        "target": "cutover to target status",
        "source": "cutover to source status",
        "source-replica": "cutover to source-replica status",
    }
    key = mode_to_key[mode]

    cmd = ["yb-voyager", "cutover", "status", "--export-dir", export_dir]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, check=False)
        for line in proc.stdout.splitlines():
            if key in line:
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
    proc = subprocess.Popen(
        cmd,
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        text=True,
    )

    # Best-effort startup validation: if the process dies quickly with a non-zero
    # exit code, treat it as a failure instead of a successful background start.
    startup_wait_seconds = 2
    time.sleep(startup_wait_seconds)

    if proc.poll() is not None and proc.returncode != 0:
        stderr_text = ""
        try:
            _, stderr_text = proc.communicate(timeout=5)
        except Exception:
            pass
        raise RuntimeError(
            f"Command failed to start: {_cmd_str(cmd)}\n"
            f"Exit: {proc.returncode}\n"
            f"STDERR:\n{stderr_text}"
        )

    return proc


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
        ctx.processes.pop(name, None)
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
    merged.update(extra or {})
    return merged


def _get_voyager_flags(cfg: Dict[str, Any], op: str) -> Dict[str, Any]:
    """Return cfg['voyager'][op]['flags'] or {} if not present."""
    voyager = cfg.get("voyager", {}) or {}
    op_cfg = voyager.get(op, {}) or {}
    return op_cfg.get("flags", {}) or {}


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


def build_import_schema_cmd(cfg: Dict[str, Any]) -> list[str]:
    voyager_flags = _get_voyager_flags(cfg, "import_schema")
    base = _base_common_flags(cfg)
    base.update(_target_conn_flags(cfg))
    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "import", "schema", "--yes"] + to_kv_flags(merged)


def build_export_schema_cmd(cfg: Dict[str, Any]) -> list[str]:
    voyager_flags = _get_voyager_flags(cfg, "export_schema")
    base = _base_common_flags(cfg)
    base.update(_source_conn_flags(cfg))
    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "export", "schema", "--yes"] + to_kv_flags(merged)


def import_schema(cfg: Dict[str, Any], env: Dict[str, str]) -> None:
    cmd = build_import_schema_cmd(cfg)
    run_checked(cmd, env, description="import_schema")


def export_schema(cfg: Dict[str, Any], env: Dict[str, str]) -> None:
    cmd = build_export_schema_cmd(cfg)
    run_checked(cmd, env, description="export_schema")


def initiate_cutover(cfg: Dict[str, Any], env: Dict[str, str], direction: str) -> None:
    voyager_flags = _get_voyager_flags(cfg, f"cutover_to_{direction}")
    base = {"export-dir": cfg["export_dir"],}
    merged = _merge_flags(base, voyager_flags)
    cmd = ["yb-voyager", "initiate", "cutover", "to", direction, "--yes"] + to_kv_flags(merged)
    run_checked(cmd, env, description=f"cutover_to_{direction}")


def build_export_data_cmd(cfg: Dict[str, Any]) -> list[str]:
    voyager_flags = _get_voyager_flags(cfg, "export_data")
    base = _base_common_flags(cfg)
    base.update(_source_conn_flags(cfg))
    # Live migration default
    base["export-type"] = "snapshot-and-changes"
    # data command defaults
    base["disable-pb"] = True
    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "export", "data", "--yes"] + to_kv_flags(merged)


def build_import_data_cmd(cfg: Dict[str, Any]) -> list[str]:
    voyager_flags = _get_voyager_flags(cfg, "import_data")
    base = _base_common_flags(cfg)
    base.update(_target_conn_flags(cfg))
    base["max-retries-streaming"] = 1
    base["skip-replication-checks"] = True
    # data command defaults
    base["disable-pb"] = True
    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "import", "data", "--yes"] + to_kv_flags(merged)


def build_export_from_target_cmd(cfg: Dict[str, Any]) -> list[str]:
    """Build yb-voyager export-from-target command for fallback."""
    voyager_flags = _get_voyager_flags(cfg, "export_from_target")
    base = _base_common_flags(cfg)
    tgt = cfg.get("target", {})
    base["target-db-password"] = tgt["password"]

    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "export", "data", "from", "target", "--yes"] + to_kv_flags(merged)


def build_import_to_source_cmd(cfg: Dict[str, Any]) -> list[str]:
    """Build yb-voyager import-to-source command for fallback."""
    voyager_flags = _get_voyager_flags(cfg, "import_to_source")
    base = _base_common_flags(cfg)
    src = cfg.get("source", {})
    base["source-db-password"] = src["password"]

    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "import", "data", "to", "source", "--yes"] + to_kv_flags(merged)


def build_import_to_source_replica_cmd(cfg: Dict[str, Any]) -> list[str]:
    """Build yb-voyager import-to-source-replica command for fall-forward."""
    voyager_flags = _get_voyager_flags(cfg, "import_to_source_or_replica")

    # Base flags: export-dir + diagnostics defaults
    base = _base_common_flags(cfg)

    # Source-replica connection flags (all required for this command)
    src_rep = cfg.get("source_replica", {})
    if src_rep:
        base["source-replica-db-user"] = src_rep.get("user", "")
        base["source-replica-db-name"] = src_rep.get("database", "")
        base["source-replica-db-password"] = src_rep.get("password", "")
        base["source-replica-db-host"] = src_rep.get("host", "")
        base["source-replica-db-port"] = src_rep.get("port", "")

    merged = _merge_flags(base, voyager_flags)
    return ["yb-voyager", "import", "data", "to", "source-replica", "--yes"] + to_kv_flags(merged)


def start_command_by_name(name: str, ctx: Context) -> subprocess.Popen:
    mapping: Dict[str, Callable[[], subprocess.Popen]] = {
        "export_data": lambda: spawn(build_export_data_cmd(ctx.cfg), ctx.env),
        "import_data": lambda: spawn(build_import_data_cmd(ctx.cfg), ctx.env),
        "export_from_target": lambda: spawn(build_export_from_target_cmd(ctx.cfg), ctx.env),
        "import_to_source_replica": lambda: spawn(build_import_to_source_replica_cmd(ctx.cfg), ctx.env),
        "import_to_source": lambda: spawn(build_import_to_source_cmd(ctx.cfg), ctx.env),
    }
    try:
        return mapping[name]()
    except KeyError as exc:
        raise ValueError(f"Unsupported command to start: {name}") from exc



# -------------------------
# Generator
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


def start_generator_from_context(ctx: Context, config_key: str = "generator") -> subprocess.Popen:
    gen_cfg = ctx.cfg.get(config_key)
    final_cfg_path = resolve_generator_config(gen_cfg, ctx.run_id, ctx.test_root)
    return start_generator(final_cfg_path, ctx.env)


def stop_generator(proc: subprocess.Popen | None, graceful_timeout_sec: int) -> None:
    kill(proc, timeout_sec=graceful_timeout_sec)


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
    missing_or_stopped: list[str] = []
    with ctx.process_lock:
        for cmd, cfg in resumption_cfg.items():
            proc = ctx.processes.get(cmd)
            if proc is None or proc.poll() is not None:
                rc = None if proc is None else proc.poll()
                log_resumption_event(
                    ctx,
                    cmd,
                    f"requested resumption but process not running (exit_code={rc}); failing stage",
                    event="missing_process",
                    exit_code=rc,
                )
                missing_or_stopped.append(f"{cmd}[exit_code={rc}]")
                continue
            policy = ResumptionPolicy(cfg)
            if policy.max_restarts <= 0:
                continue
            resumer = Resumer(cmd, policy, ctx)
            ctx.active_resumers[cmd] = resumer
            to_start.append(resumer)
    if missing_or_stopped:
        raise RuntimeError(
            "Cannot start resumptions; requested commands are not running: "
            + ", ".join(missing_or_stopped)
        )
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
    base_patterns = patterns or [
        "ERROR",
        "FATAL",
        "WARN",
        "Discrepancy in committed batch",
        "unexpected rows affected for event with",
    ]
    logs_dir = os.path.join(export_dir, "logs")
    scan_dir = os.path.join(artifacts_dir, "log_scans")
    os.makedirs(scan_dir, exist_ok=True)
    
    grep_cmd = ["grep", "-aiF"]
    for pat in base_patterns:
        grep_cmd += ["-e", pat]

    for fp in _iter_log_files(logs_dir):
        try:
            proc = subprocess.run(
                [*grep_cmd, "--", fp],
                capture_output=True,
                text=True,
                errors="ignore",
                check=False,
            )
        except Exception:
            continue

        if proc.returncode != 0:
            continue

        findings = [line.rstrip() for line in proc.stdout.splitlines()]
        if findings:
            basename = os.path.basename(fp)
            out = os.path.join(scan_dir, f"{basename}.scan.txt")
            with open(out, "w") as outf:
                outf.write("\n".join(findings) + "\n")


def snapshot_msr_and_stats(export_dir: str, artifacts_dir: str) -> None:
    metainfo_dir = os.path.join(export_dir, "metainfo")
    dest_dir = os.path.join(artifacts_dir, "metainfo")

    shutil.copytree(metainfo_dir, dest_dir, dirs_exist_ok=True)


def copy_logs_directory(export_dir: str, artifacts_dir: str) -> None:
    logs_dir = os.path.join(export_dir, "logs")
    dest_dir = os.path.join(artifacts_dir, "logs")

    shutil.copytree(logs_dir, dest_dir, dirs_exist_ok=True)


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

def prepare_paths(export_dir: str, artifacts_dir: str) -> None:
    """Delete and recreate export_dir and artifacts_dir."""
    for p in (export_dir, artifacts_dir):
        shutil.rmtree(p, ignore_errors=True)
        os.makedirs(p, exist_ok=True)


# -------------------------
# SQL execution
# -------------------------

def db_connection(cfg: Dict[str, Any], role: str) -> psycopg2.extensions.connection:
    """Generic connection helper for source/target DBs."""
    db = cfg[role]
    return psycopg2.connect(
        host=str(db["host"]),
        port=int(db["port"]),
        dbname=str(db["database"]),
        user=str(db["user"]),
        password=str(db.get("password")),
    )


def run_psql(
    ctx,
    role: str,
    *args: str,
    db_override: str | None = None,
    user_override: str | None = None,
    password_override: str | None = None,
    stdin_input: str | None = None,
) -> None:
    db = ctx.cfg[role]
    env = dict(ctx.env)

    password = password_override or db.get("password")
    if password:
        env["PGPASSWORD"] = str(password)

    user = user_override or db["user"]

    cmd = [
        "psql",
        "-h", str(db["host"]),
        "-p", str(db["port"]),
        "-U", str(user),
        "-d", str(db_override or db["database"]),
        "-v", "ON_ERROR_STOP=1",
        *args,
    ]

    # Capture output so we can surface errors clearly in logs and exceptions.
    proc = subprocess.run(
        cmd,
        env=env,
        input=stdin_input,
        text=True,
        capture_output=True,
    )

    if proc.returncode != 0:
        # Log detailed diagnostics to help debug Jenkins / CI failures.
        log(f"psql failed (role={role}) exit_code={proc.returncode}")
        if proc.stdout:
            log("psql STDOUT:\n" + proc.stdout)
        if proc.stderr:
            log("psql STDERR:\n" + proc.stderr)

        raise RuntimeError(
            f"psql failed (role={role}) exit_code={proc.returncode}; "
            f"command={' '.join(cmd)}"
        )


def fetchall(cfg: Dict[str, Any], role: str, query: str, params=()) -> list[tuple]:
    """Generic helper for SELECT queries."""
    with db_connection(cfg, role) as conn, conn.cursor() as cur:
        cur.execute(query, params)
        return cur.fetchall()


def run_sql_file(ctx, sql_path: str, target: str = "source", *, use_admin: bool = False) -> None:
    """Executes SQL against source/target using psql."""
    user_override = password_override = None
    if use_admin:
        admin_cfg = ctx.cfg[target].get("admin")
        user_override = admin_cfg["user"]
        password_override = admin_cfg.get("password")
    run_psql(ctx, target, "-f", sql_path, user_override=user_override, password_override=password_override)


def grant_postgres_live_migration_permissions(ctx, *, is_live_migration_fall_back: int = 0) -> None:
    src = ctx.cfg["source"]
    admin = src["admin"]

    run_psql(
        ctx,
        "source",
        "-v", f"voyager_user={src['user']}",
        "-v", f"schema_list={src.get('schema', 'public')}",
        "-v", "replication_group=replication_group",
        "-v", "is_live_migration=1",
        "-v", f"is_live_migration_fall_back={is_live_migration_fall_back}",
        "-f", "/opt/yb-voyager/guardrails-scripts/yb-voyager-pg-grant-migration-permissions.sql",
        user_override=admin["user"],
        password_override=admin["password"],
        stdin_input="2\n",
    )


def list_source_tables(cfg: Dict[str, Any]) -> List[str]:
    schema = cfg["source"]["schema"]
    rows = fetchall(cfg, "source", """
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema = %s
          AND table_type = 'BASE TABLE'
        ORDER BY table_name
    """, (schema,))
    return [r[0] for r in rows]


def _fetch_table_count(conn, schema: str, table: str) -> int:
    with conn.cursor() as cur:
        query = sql.SQL("SELECT COUNT(*) FROM {}.{}").format(
            sql.Identifier(schema),
            sql.Identifier(table),
        )
        cur.execute(query)
        (count,) = cur.fetchone()
        return int(count)


def _reset_database(db_cfg: Dict[str, Any], *, admin_db_name: str, role: str) -> None:
    admin = db_cfg["admin"]
    dbname = db_cfg["database"]

    env = dict(os.environ)
    env["PGPASSWORD"] = str(admin["password"])

    host = str(db_cfg["host"])
    port = str(db_cfg["port"])
    admin_user = str(admin["user"])

    drop_cmd = [
        "psql", "-h", host, "-p", port, "-U", admin_user,
        "-d", admin_db_name, "-v", "ON_ERROR_STOP=1",
        "-c", f'DROP DATABASE IF EXISTS "{dbname}"',
    ]

    create_cmd = [
        "psql", "-h", host, "-p", port, "-U", admin_user,
        "-d", admin_db_name, "-v", "ON_ERROR_STOP=1",
        "-c", f'CREATE DATABASE "{dbname}"',
    ]

    log(f"[db-reset:{role}] dropping database '{dbname}'")
    run_checked(drop_cmd, env, description=f"reset_database:{role}:drop")

    log(f"[db-reset:{role}] creating database '{dbname}'")
    run_checked(create_cmd, env, description=f"reset_database:{role}:create")

    log(f"[db-reset:{role}] database '{dbname}' recreated")


def reset_database_for_role(role: str, ctx) -> None:
    admin_db_name_by_role = {
        "source": "postgres",
        "target": "yugabyte",
        "source_replica": "postgres",
    }

    try:
        db_cfg = ctx.cfg[role]
        admin_db_name = admin_db_name_by_role[role]
    except KeyError as exc:
        raise ValueError(f"Unsupported database role for reset: {role}") from exc

    _reset_database(db_cfg, admin_db_name=admin_db_name, role=role)


# -------------------------
# Validation helpers
# -------------------------

def _load_segment_map_for_side(
    cfg: Dict[str, Any],
    schema: str,
    role: str,
) -> Dict[tuple, Dict[str, Any]]:
    """Return a mapping (schema, table, segment_index) -> {row_count, segment_hash} for one side."""
    rows = fetchall(
        cfg,
        role,
        """
        SELECT schema_name,
               table_name,
               segment_index,
               row_count,
               segment_hash
        FROM public.migration_validate_segments
        WHERE side = %s
          AND schema_name = %s
          AND table_name <> 'migration_validate_segments'
        """,
        (role, schema),
    )
    by_key: Dict[tuple, Dict[str, Any]] = {}
    for (
        schema_name,
        table_name,
        segment_index,
        row_count,
        segment_hash,
    ) in rows:
        key = (schema_name, table_name, int(segment_index))
        by_key[key] = {
            "row_count": int(row_count),
            "segment_hash": str(segment_hash),
        }
    return by_key


def _build_segment_record(
    key: tuple,
    src: Dict[str, Any] | None,
    tgt: Dict[str, Any] | None,
) -> tuple[Dict[str, Any], bool]:
    """Build a per-segment record and return (record, is_mismatch)."""
    schema_name, table_name, segment_index = key
    record: Dict[str, Any] = {
        "schema_name": schema_name,
        "table_name": table_name,
        "segment_index": segment_index,
    }

    if src is None:
        record["status"] = "missing_source"
        return record, True
    if tgt is None:
        record["status"] = "missing_target"
        return record, True

    src_count = src["row_count"]
    tgt_count = tgt["row_count"]
    src_hash = src["segment_hash"]
    tgt_hash = tgt["segment_hash"]

    record["source_row_count"] = src_count
    record["target_row_count"] = tgt_count
    record["source_hash"] = src_hash
    record["target_hash"] = tgt_hash

    same_count = src_count == tgt_count
    same_hash = src_hash == tgt_hash

    if same_count and same_hash:
        record["status"] = "match"
        return record, False

    record["status"] = "mismatch"
    return record, True


def run_segment_hash_computation(ctx: Context, side: str, num_segments: int = 16) -> None:
    """Invoke compute_schema_segment_hashes on the given side (source/target/source_replica).

    This relies on the SQL primitives defined in segment_hash_validation.sql
    being installed on both source and target clusters.
    """
    if side not in ("source", "target", "source_replica"):
        raise ValueError(f"run_segment_hash_computation: unexpected side {side!r}")

    cfg = ctx.cfg
    schema = cfg["source"]["schema"]

    # We use the logical side ('source' / 'target') both as:
    #   - the DB role for run_psql
    #   - the side label recorded inside migration_validate_segments.
    sql_stmt = f"SELECT public.compute_schema_segment_hashes('{side}', '{schema}', {int(num_segments)});"
    run_psql(ctx, side, "-c", sql_stmt)


def compare_segment_hashes(
    ctx: Context,
    left_side: str = "source",
    right_side: str = "target",
) -> tuple[list[Dict[str, Any]], list[Dict[str, Any]]]:
    """Load segment hashes for two sides and compute in-memory differences.

    Returns:
        all_segments: list of per-segment records including status.
        mismatches: subset of all_segments where counts/hashes/side-presence differ.
    """
    cfg = ctx.cfg
    schema = cfg["source"]["schema"]

    left_map = _load_segment_map_for_side(cfg, schema, left_side)
    right_map = _load_segment_map_for_side(cfg, schema, right_side)

    all_keys = sorted(set(left_map.keys()) | set(right_map.keys()))

    all_segments: list[Dict[str, Any]] = []
    mismatches: list[Dict[str, Any]] = []

    for key in all_keys:
        src = left_map.get(key)
        tgt = right_map.get(key)
        record, is_mismatch = _build_segment_record(key, src, tgt)
        all_segments.append(record)
        if is_mismatch:
            mismatches.append(record)

    return all_segments, mismatches


def run_segment_hash_validations(
    ctx: Context,
    left_side: str = "source",
    right_side: str = "target",
) -> None:
    """End-to-end segment-hash validation: compute, compare, and persist artifacts.

    Raises:
        RuntimeError if any segment shows a mismatch or is missing on one side.
    """
    # 1) Compute (or refresh) segment hashes on both sides
    run_segment_hash_computation(ctx, left_side)
    run_segment_hash_computation(ctx, right_side)

    # 2) Compare in-memory
    all_segments, mismatches = compare_segment_hashes(ctx, left_side, right_side)

    # 3) Persist artifacts
    base_dir = os.path.join(ctx.artifacts_dir, "validation", "hash_segments")
    os.makedirs(base_dir, exist_ok=True)

    segments_path = os.path.join(base_dir, "segments.json")
    mismatches_path = os.path.join(base_dir, "mismatches.json")

    with open(segments_path, "w") as f:
        json.dump(all_segments, f)

    with open(mismatches_path, "w") as f:
        json.dump(mismatches, f)

    # 4) Raise on mismatches with a concise preview
    if mismatches:
        preview_items = mismatches[:5]
        preview = "; ".join(
            f"{r['schema_name']}.{r['table_name']}[seg={r['segment_index']}] status={r['status']}"
            for r in preview_items
        )
        if len(mismatches) > len(preview_items):
            preview += f"; ... and {len(mismatches) - len(preview_items)} more"
        raise RuntimeError(f"segment hash validation failed for segments: {preview}")


def run_row_count_validations(
    ctx: Context,
    left_role: str = "source",
    right_role: str = "target",
) -> None:
    """Compare row counts between two roles (default: source and target) using direct SQL."""
    cfg = ctx.cfg
    schema = cfg["source"]["schema"]
    tables = list_source_tables(cfg)
    out_dir = os.path.join(ctx.artifacts_dir, "validation", "row_counts")
    os.makedirs(out_dir, exist_ok=True)
    mismatches = []

    with db_connection(cfg, left_role) as left_conn, db_connection(cfg, right_role) as right_conn:
        for table in tables:
            left_count = _fetch_table_count(left_conn, schema, table)
            right_count = _fetch_table_count(right_conn, schema, table)

            record = {
                "table": table,
                # Field names kept for backward-compatibility; values come from left/right roles.
                "source_count": left_count,
                "target_count": right_count,
                "status": "success" if left_count == right_count else "mismatch",
            }

            # Write per-table result
            with open(os.path.join(out_dir, f"{table}.json"), "w") as f:
                json.dump(record, f)
            if record["status"] == "mismatch":
                mismatches.append(record)

    # If any mismatches, write summary + raise error
    if mismatches:
        summary_path = os.path.join(out_dir, "row_count_mismatches.json")
        with open(summary_path, "w") as f:
            json.dump({"mismatches": mismatches}, f)

        preview = "; ".join(
            f"{r['table']} (source={r['source_count']}, target={r['target_count']})"
            for r in mismatches
        )
        raise RuntimeError(f"row count validation failed for tables: {preview}")
