#!/usr/bin/env python3

import os
import subprocess
import signal
import time
import random
import sys
import select
import re
import urllib.request
import urllib.error
import yaml
sys.path.append(os.path.join(os.getcwd(), 'migtests/lib'))
import yb
import argparse
import tempfile


# Global configuration variables

# import_type: Type of import ('file' or 'offline').
# additional_flags: Additional flags to be passed to the import command.
# resumption: Dictionary containing resumption settings.
# row_count: Dictionary containing expected row counts for validation.
# max_restarts: Maximum number of restarts / resumes.
# min_interrupt_seconds: Minimum interval between interrupts.
# max_interrupt_seconds: Maximum interval between interrupts.
# min_restart_wait_seconds: Minimum wait time before resuming.
# max_restart_wait_seconds: Maximum wait time before resuming.
# varying_flags: Flags which are run with varying values on each invocation.

import_type = None
additional_flags = {}
file_table_map = ''
resumption = {}
max_restarts = 0
min_interrupt_seconds = 0
max_interrupt_seconds = 0
min_restart_wait_seconds = 0
max_restart_wait_seconds = 0
row_count = {}
export_dir = ''
run_without_adaptive_parallelism = False
source_db_type = ''
target_db_host = ''
target_db_port = ''
target_db_user = ''
target_db_password = ''
target_db_schema = ''
target_db_name = ''
data_dir = ''
varying_flags = {}

# Metrics scraping configuration.
# Fixed port for the yb-voyager --metrics-port Prometheus server started by
# the import commands under test. Chosen to avoid clashing with the legacy
# profile default metrics ports (9101-9104).
METRICS_PORT = 9201
METRICS_SCRAPE_TIMEOUT_SECONDS = 2
SNAPSHOT_ROWS_TOTAL_METRIC = 'yb_voyager_import_data_snapshot_rows_total'
SNAPSHOT_TABLES_TOTAL_METRIC = 'yb_voyager_import_data_snapshot_tables_total'

# How long to let a `--start-clean` re-run go before interrupting it to
# sample the snapshot counters (see run_start_clean_check).
START_CLEAN_CHECK_SECONDS = 10
# Fraction of the total expected row count that a start-clean run is allowed
# to have imported by the time we sample it, while still counting as "reset".
START_CLEAN_ROWS_SANITY_RATIO = 0.1

# Baseline table count observed from the first metrics snapshot of the run.
# Used to assert that yb_voyager_import_data_snapshot_tables_total stays
# stable across resumes and start-clean re-runs.
expected_table_count = None

def parse_arguments():
    parser = argparse.ArgumentParser(description="YB Voyager Resumption Test")
    parser.add_argument('config_file', metavar='config.yaml', type=str, 
                        help="Path to the YAML configuration file")
    return parser.parse_args()

def load_config(config_file):
    """Load the configuration from the provided YAML file."""
    if not os.path.exists(config_file):
        raise FileNotFoundError(f"Config file not found: {config_file}")
    with open(config_file, 'r') as file:
        config = yaml.safe_load(file)        
    return config

def initialize_globals(config):
    """Initialize global variables from configuration."""
    global import_type, resumption, row_count, max_restarts, min_interrupt_seconds, max_interrupt_seconds, min_restart_wait_seconds, max_restart_wait_seconds
    global export_dir, additional_flags, file_table_map, run_without_adaptive_parallelism, source_db_type, target_db_host, target_db_port, target_db_user, target_db_password, target_db_schema, target_db_name, data_dir, varying_flags

    resumption = config.get('resumption', {})
    import_type = config.get('import_type', 'file')  # Default to 'file'
    additional_flags = config.get('additional_flags', {})
    file_table_map = config.get('file_table_map', '')
    varying_flags = config.get("varying_flags", {})

    # Resumption settings
    max_restarts = resumption.get('max_restarts', 5)
    min_interrupt_seconds = resumption.get('min_interrupt_seconds', 30)
    max_interrupt_seconds = resumption.get('max_interrupt_seconds', 60)
    min_restart_wait_seconds = resumption.get('min_restart_wait_seconds', 30)
    max_restart_wait_seconds = resumption.get('max_restart_wait_seconds', 60)

    # Validation
    row_count = config.get('row_count', {})

    # Export directory
    export_dir = os.getenv('EXPORT_DIR', os.getcwd())

    # Environment variables
    target_db_host = os.getenv('TARGET_DB_HOST', '')
    target_db_port = os.getenv('TARGET_DB_PORT', '')
    target_db_user = os.getenv('TARGET_DB_USER', '')
    target_db_password = os.getenv('TARGET_DB_PASSWORD', '')
    target_db_schema = os.getenv('TARGET_DB_SCHEMA', '')
    target_db_name = os.getenv('TARGET_DB_NAME', '')
    data_dir = os.getenv('DATA_DIR', '')

    # Adaptive parallelism
    run_without_adaptive_parallelism = os.getenv('RUN_WITHOUT_ADAPTIVE_PARALLELISM') == 'true'
    source_db_type = os.getenv('SOURCE_DB_TYPE', '')


def prepare_import_data_file_command():
    """Prepares the yb-voyager import data file command."""
    args = [
        'yb-voyager', 'import', 'data', 'file',
        '--export-dir', export_dir,
        '--target-db-host', target_db_host,
        '--target-db-port', target_db_port,
        '--target-db-user', target_db_user,
        '--target-db-password', target_db_password,
        '--target-db-schema', target_db_schema,
        '--target-db-name', target_db_name,
        '--disable-pb', 'true',
        '--send-diagnostics', 'false',
        '--data-dir', data_dir,
        '--file-table-map', file_table_map,
        '--skip-replication-checks', 'true',
        '--metrics-port', str(METRICS_PORT),
    ]

    if run_without_adaptive_parallelism:
        args.extend(['--adaptive-parallelism', 'disabled'])

    for flag, value in additional_flags.items():
        args.append(flag)
        args.append(value)

    return args


def prepare_import_data_command():
    """
    Prepares the yb-voyager import data command based on the given configuration.
    """

    args = [
        'yb-voyager', 'import', 'data',
        '--export-dir', export_dir,
        '--target-db-host', target_db_host,
        '--target-db-port', target_db_port,
        '--target-db-user', target_db_user,
        '--target-db-password', target_db_password,
        '--target-db-name', target_db_name,
        '--disable-pb', 'true',
        '--send-diagnostics', 'false',
        '--skip-replication-checks', 'true',
        '--metrics-port', str(METRICS_PORT),
    ]

    if source_db_type != 'postgresql':
        args.extend(['--target-db-schema', target_db_schema])

    if run_without_adaptive_parallelism:
        args.extend(['--adaptive-parallelism', 'disabled'])

    for flag, value in additional_flags.items():
        args.append(flag)
        args.append(value)

    return args

def inject_varying_flags_values(command):
    global varying_flags

    for flag, setting in varying_flags.items():
        value_list = setting["value"]
        if setting["type"] == "range":
            value = random.randint(value_list[0], value_list[1])
        elif setting["type"] == "choice":
            value = random.choice(value_list)
        else:
            raise ValueError(f"Unknown type for '{flag}': {setting['type']}")
        command.extend([flag, str(value)])

    return command


_METRIC_LINE_RE = re.compile(
    r'^(?P<name>[a-zA-Z_:][a-zA-Z0-9_:]*)(\{(?P<labels>[^}]*)\})?\s+(?P<value>\S+)\s*$'
)
_LABEL_RE = re.compile(r'(?P<key>[a-zA-Z_][a-zA-Z0-9_]*)="(?P<value>(?:[^"\\]|\\.)*)"')


def parse_prometheus_text(text):
    """
    Minimal parser for the Prometheus text exposition format.

    Returns a dict of {metric_name: [(labels_dict, value_float), ...]}.
    Comment/HELP/TYPE lines are ignored; malformed lines are skipped.
    """
    metrics = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith('#'):
            continue
        match = _METRIC_LINE_RE.match(line)
        if not match:
            continue
        try:
            value = float(match.group('value'))
        except ValueError:
            continue

        labels = {}
        if match.group('labels'):
            for label_match in _LABEL_RE.finditer(match.group('labels')):
                labels[label_match.group('key')] = label_match.group('value').replace('\\"', '"')

        metrics.setdefault(match.group('name'), []).append((labels, value))
    return metrics


def scrape_metrics(port=METRICS_PORT, timeout=METRICS_SCRAPE_TIMEOUT_SECONDS):
    """
    Scrapes GET http://127.0.0.1:<port>/metrics and parses it.

    Returns the parsed metrics dict, or None if the endpoint could not be
    reached (e.g. the server hasn't started yet, or the process has already
    exited). Callers should treat None as "no snapshot available" rather
    than a hard error, since scraping races with process startup/shutdown.
    """
    url = f"http://127.0.0.1:{port}/metrics"
    try:
        with urllib.request.urlopen(url, timeout=timeout) as response:
            text = response.read().decode('utf-8')
    except (urllib.error.URLError, ConnectionError, TimeoutError, OSError):
        return None
    return parse_prometheus_text(text)


def sum_metric_value(metrics, name):
    """Sums the values of all label-series for a metric. Returns 0 if absent."""
    if metrics is None:
        return 0
    return sum(value for _, value in metrics.get(name, []))


def assert_metrics_snapshot(label, metrics, min_rows_total=None):
    """
    Validates a single metrics snapshot against the running invariants:
      - yb_voyager_import_data_snapshot_tables_total must match the baseline
        `expected_table_count` captured from the first snapshot of the run
        (it must not fluctuate across resumes).
      - yb_voyager_import_data_snapshot_rows_total (summed across all table
        series) must never regress below `min_rows_total`, if given -- a
        resumed run must seed its counters from the persisted per-table
        imported counts, not restart from zero.

    Returns the summed rows_total from this snapshot, to be used as the
    `min_rows_total` baseline for the next call. Exits the process on any
    violation, consistent with the rest of this script's error handling.
    """
    global expected_table_count

    if metrics is None:
        print(f"\u274C [{label}] Could not scrape metrics; is --metrics-port wired up and reachable?", flush=True)
        sys.exit(1)

    tables_total = sum_metric_value(metrics, SNAPSHOT_TABLES_TOTAL_METRIC)
    rows_total = sum_metric_value(metrics, SNAPSHOT_ROWS_TOTAL_METRIC)

    print(f"[{label}] {SNAPSHOT_TABLES_TOTAL_METRIC}={tables_total}, {SNAPSHOT_ROWS_TOTAL_METRIC}={rows_total}", flush=True)

    if expected_table_count is None:
        expected_table_count = tables_total
        print(f"[{label}] Recorded baseline table count: {expected_table_count}", flush=True)
    elif tables_total != expected_table_count:
        print(f"\u274C [{label}] {SNAPSHOT_TABLES_TOTAL_METRIC} changed: expected {expected_table_count}, got {tables_total}", flush=True)
        sys.exit(1)

    if min_rows_total is not None and rows_total < min_rows_total:
        print(f"\u274C [{label}] {SNAPSHOT_ROWS_TOTAL_METRIC} went backwards: expected >= {min_rows_total}, got {rows_total}", flush=True)
        sys.exit(1)

    return rows_total


def run_command(command, allow_interruption=False, interrupt_after=None, metrics_port=None):
    with tempfile.TemporaryFile() as stdout_file, tempfile.TemporaryFile() as stderr_file:
        process = subprocess.Popen(
            command, stdout=stdout_file, stderr=stderr_file, text=True
        )
        start_time = time.time()
        interrupted = False
        chosen_signal = None
        # Latest successfully scraped /metrics snapshot. Refreshed on every
        # poll iteration, so it reflects the state just before we send an
        # interrupt signal, or the last observable state before the process
        # exits on its own.
        latest_metrics = None

        while process.poll() is None:
            if metrics_port is not None:
                snapshot = scrape_metrics(port=metrics_port)
                if snapshot is not None:
                    latest_metrics = snapshot

            if allow_interruption and interrupt_after is not None:
                elapsed_time = time.time() - start_time
                if elapsed_time > interrupt_after:
                    # Choose a random signal to send
                    interrupt_signals = [
                        signal.SIGTERM,
                        signal.SIGINT,
                        signal.SIGKILL
                    ]
                    chosen_signal = random.choice(interrupt_signals)
                    print(f"Interrupting the process (PID: {process.pid}) with signal {chosen_signal.name}...", flush=True)

                    try:
                        process.send_signal(chosen_signal)
                        print(f"{chosen_signal.name} sent to process (PID: {process.pid}). Waiting for process to exit...", flush=True)

                        process.wait(timeout=10)  # Wait for the process to exit
                        print(f"Process (PID: {process.pid}) terminated gracefully with exit code: {process.returncode}", flush=True)

                    except subprocess.TimeoutExpired:
                        print(f"Process (PID: {process.pid}) did not terminate in time. Forcing termination...", flush=True)
                        process.kill()
                        print(f"Process (PID: {process.pid}) force-killed with exit code: {process.returncode}", flush=True)

                    interrupted = True
                    break
            time.sleep(1)  # Avoid busy-waiting

        stdout_file.seek(0)
        stderr_file.seek(0)

        stdout = stdout_file.read().decode('utf-8').strip()
        stderr = stderr_file.read().decode('utf-8').strip()

        if stdout:
            print("\nCommand Output:\n")
            for line in stdout.splitlines():
                print(line)
        if stderr:
            print("\nCommand Errors:\n")
            for line in stderr.splitlines():
                print(line)
            # If there is any stderr output, treat it as an error and exit.
            # In the interrupt-retry scenario, we do not expect stderr output. The command should be interrupted without errors.
            sys.exit(1)

        # If interrupted, check the exit code
        if interrupted:
            # These exit codes are considered valid for interrupted processes:
            # Negative signal numbers (e.g., -SIGKILL) indicate termination by a specific signal.
            # 128 + signal number (e.g., 128 + SIGKILL) is the convention for processes terminated by signals.
            # Exit code 1 is expected for graceful termination by SIGTERM or SIGINT.
            valid_interrupt_exit_codes = {
                -signal.SIGKILL, 128 + signal.SIGKILL,
                1
            }

            if process.returncode not in valid_interrupt_exit_codes:
                print(f"Unexpected exit code after interruption ({chosen_signal.name}): {process.returncode}", flush=True)
                sys.exit(1)

        completed = process.returncode == 0 and not interrupted
        return completed, stdout, stderr, latest_metrics


def run_and_resume_voyager(base_command):
    """
    Handles the interruption logic and manages retries for the command.

    After every attempt, scrapes /metrics (see scrape_metrics) and validates,
    via assert_metrics_snapshot, that yb_voyager_import_data_snapshot_tables_total
    stays constant and yb_voyager_import_data_snapshot_rows_total never
    regresses across the resume/restart cycle -- the counters must be seeded
    from the persisted per-table imported counts on resume, not reset.

    Args:
        base_command (list): The base command to execute.

    Returns:
        int: the final summed yb_voyager_import_data_snapshot_rows_total,
        captured just before the import completed.
    """
    min_rows_total = None

    for attempt in range(1, max_restarts + 1):
        print(f"\n--- Attempt {attempt} of {max_restarts} ---")

        # Clone base command
        command = base_command.copy()

        # Inject varying flags on each retry
        command = inject_varying_flags_values(command)

        # Randomly determine interruption timing
        interruption_time = random.randint(min_interrupt_seconds, max_interrupt_seconds)

        print(f"\nRunning command: {' '.join(command)}", flush=True)
        print(f"\nInterrupting the command in {interruption_time // 60}m {interruption_time % 60}s...", flush=True)

        completed, stdout, stderr, metrics = run_command(
            command, allow_interruption=True, interrupt_after=interruption_time, metrics_port=METRICS_PORT
        )

        min_rows_total = assert_metrics_snapshot(f"attempt {attempt}, pre-interrupt", metrics, min_rows_total)

        print("Process was interrupted. Preparing to resume...", flush=True)
        restart_wait_time_seconds = random.randint(min_restart_wait_seconds, max_restart_wait_seconds)
        print(f"Waiting {restart_wait_time_seconds // 60}m {restart_wait_time_seconds % 60}s before resuming...", flush=True)
        time.sleep(restart_wait_time_seconds)
        print("Completed waiting. Proceeding to next attempt...", flush=True)

    # Final attempt without interruption
    print("\n--- Final attempt to complete the import ---\n", flush=True)

    # Inject final set of varying flags before final run
    command = inject_varying_flags_values(base_command.copy())
    completed, stdout, stderr, metrics = run_command(command, allow_interruption=False, metrics_port=METRICS_PORT)

    if not completed:
        print("\nCommand failed on the final attempt.", flush=True)
        sys.exit(1)

    min_rows_total = assert_metrics_snapshot("final attempt, last observed before completion", metrics, min_rows_total)

    print("\nCommand completed successfully on the final attempt.", flush=True)
    return min_rows_total

def validate_row_counts():
    """
    Validates the row counts of the target tables after import.
    If the row count validation fails, it logs details and exits.
    """
    failed_validations = []

    for table_identifier, expected_row_count in row_count.items():
        print(f"\nValidating row count for table '{table_identifier}'...")

        if '.' in table_identifier:
            schema, table_name = table_identifier.split('.', 1)
        else:
            schema = "public"
            table_name = table_identifier

        tgt = None
        try:
            tgt = yb.new_target_db()
            tgt.connect()
            print(f"Connected to target database. Using schema: {schema}")
            actual_row_count = tgt.get_row_count(table_name, schema)

            if actual_row_count == expected_row_count:
                print(f"\u2714 Validation successful: {table_identifier} - Expected: {expected_row_count}, Actual: {actual_row_count}")
            else:
                print(f"\u274C Validation failed: {table_identifier} - Expected: {expected_row_count}, Actual: {actual_row_count}")
                failed_validations.append((table_identifier, expected_row_count, actual_row_count))
        except Exception as e:
            print(f"Error during validation for table '{table_identifier}': {e}")
            failed_validations.append((table_identifier, expected_row_count, "Error"))
        finally:
            if tgt:
                tgt.close()
                print("Disconnected from target database.")

    if failed_validations:
        print("\nValidation failed for the following tables:")
        for table, expected, actual in failed_validations:
            print(f"  Table: {table}, Expected: {expected}, Actual: {actual}")
        print(f"\nFor more details, check {export_dir}/logs")
        sys.exit(1)
    else:
        print("\nAll table row counts validated successfully.")

def validate_final_snapshot_rows_total(final_rows_total):
    """
    Validates that the final yb_voyager_import_data_snapshot_rows_total scraped
    from /metrics (summed across all table series) matches the expected total
    row count, reusing the same `row_count` map that validate_row_counts()
    already validates against the target database -- no separately-tracked
    total is maintained.
    """
    expected_total_rows = sum(row_count.values())
    print(f"\nValidating final snapshot rows total: expected {expected_total_rows}, got {final_rows_total}", flush=True)

    if final_rows_total != expected_total_rows:
        print(f"\u274C {SNAPSHOT_ROWS_TOTAL_METRIC} mismatch: expected {expected_total_rows}, got {final_rows_total}", flush=True)
        sys.exit(1)

    print(f"\u2714 {SNAPSHOT_ROWS_TOTAL_METRIC} matches expected total row count.", flush=True)


def run_start_clean_check(base_command):
    """
    Runs a fresh `--start-clean true` import of the same base command after
    the main resumption flow has already fully imported all the data, and
    asserts that the snapshot counters reset rather than resume from the
    prior (fully-imported) metaDB state.

    The run is interrupted shortly after starting (START_CLEAN_CHECK_SECONDS)
    purely to sample /metrics; the harness's own cleanup (drop DB, remove
    export-dir) takes care of tearing the partially re-imported state down.
    """
    command = inject_varying_flags_values(base_command.copy())
    command.extend(['--start-clean', 'true'])

    print(f"\n--- Start-clean check: {' '.join(command)} ---", flush=True)
    completed, stdout, stderr, metrics = run_command(
        command,
        allow_interruption=True,
        interrupt_after=START_CLEAN_CHECK_SECONDS,
        metrics_port=METRICS_PORT,
    )

    if metrics is None:
        print("\u274C [start-clean] Could not scrape metrics before interrupting.", flush=True)
        sys.exit(1)

    tables_total = sum_metric_value(metrics, SNAPSHOT_TABLES_TOTAL_METRIC)
    rows_total = sum_metric_value(metrics, SNAPSHOT_ROWS_TOTAL_METRIC)
    print(f"[start-clean] {SNAPSHOT_TABLES_TOTAL_METRIC}={tables_total}, {SNAPSHOT_ROWS_TOTAL_METRIC}={rows_total}", flush=True)

    if expected_table_count is not None and tables_total != expected_table_count:
        print(f"\u274C [start-clean] {SNAPSHOT_TABLES_TOTAL_METRIC} expected {expected_table_count}, got {tables_total} "
              f"-- the full table set should be re-registered after --start-clean.", flush=True)
        sys.exit(1)

    sanity_threshold = sum(row_count.values()) * START_CLEAN_ROWS_SANITY_RATIO
    if rows_total > sanity_threshold:
        print(f"\u274C [start-clean] {SNAPSHOT_ROWS_TOTAL_METRIC}={rows_total} exceeds sanity threshold {sanity_threshold}; "
              f"--start-clean does not appear to have reset the snapshot counters.", flush=True)
        sys.exit(1)

    print("\u2714 [start-clean] Snapshot counters reset as expected after --start-clean.", flush=True)


def run_import_with_resumption():
    """
    Runs the import process with resumption logic based on the provided configuration.

    Args:
        config (dict): The configuration dictionary loaded from the YAML file.

    Returns:
        tuple: (base_command, final_rows_total) so callers can validate the
        final snapshot total and/or reuse the base command for a subsequent
        --start-clean check.
    """

    if import_type == 'file':
        command = prepare_import_data_file_command()
    elif import_type == 'offline':
        command = prepare_import_data_command()
    else:
        raise ValueError(f"Unsupported import_type: {import_type}")
        sys.exit(1)

    final_rows_total = run_and_resume_voyager(command)
    return command, final_rows_total


if __name__ == "__main__":
    try:
        args = parse_arguments()
        config = load_config(args.config_file)
        initialize_globals(config)

        print(f"Loaded configuration from {args.config_file}")

        # Run import process
        base_command, final_rows_total = run_import_with_resumption()

        # Validate rows
        validate_row_counts()
        validate_final_snapshot_rows_total(final_rows_total)

        # Validate that a fresh --start-clean run resets the snapshot counters
        run_start_clean_check(base_command)

    except Exception as e:
        print(f"Test failed: {e}")
        sys.exit(1)