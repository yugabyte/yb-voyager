#!/usr/bin/env python3

import os
import subprocess
import signal
import time
import random
import sys
import select
import yaml
sys.path.append(os.path.join(os.getcwd(), 'migtests/lib'))
import yb
import argparse
import tempfile
import math


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
previous_row_count = {}
current_row_count = {}
total_row_count = 0
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
max_resumption_time = 1800 # 30 minutes

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
    global import_type, resumption, row_count, total_row_count, current_row_count, previous_row_count, max_restarts, min_interrupt_seconds, max_interrupt_seconds, min_restart_wait_seconds, max_restart_wait_seconds
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

    for table_identifier, expected_row_count in row_count.items():
        previous_row_count[table_identifier] = 0
        total_row_count += expected_row_count

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

def validate_ingestion(expected_threshold_ingestion_percentage):
    """
    Validates the ingestion of the data into the target database.
    If the row count validation fails, it logs details and exits.
    """
    global previous_row_count, current_row_count
    
    previous_total_row_count = 0
    for table_identifier, expected_row_count in previous_row_count.items():
        previous_total_row_count += expected_row_count

    for table_identifier, expected_row_count in row_count.items():
        print(f"\nValidating ingestion for table '{table_identifier}'...")

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
            current_row_count[table_identifier] = tgt.get_row_count(table_name, schema)

            if current_row_count[table_identifier] > previous_row_count[table_identifier]:
                print(f"\u2714 Ingestion successful: {table_identifier} - Previous: {previous_row_count[table_identifier]}, Current: {current_row_count[table_identifier]}")
            else:
                return [0, 0] # Ingestion failed
        except Exception as e:
            print(f"Error during validation for table '{table_identifier}': {e}")
            return [0, 0] # Ingestion failed
        finally:
            if tgt:
                tgt.close()
                print("Disconnected from target database.")

    current_total_row_count = 0
    for table_identifier, count in current_row_count.items():
        current_total_row_count += count

    iteration_ingestion_percentage = math.floor((current_total_row_count - previous_total_row_count) / total_row_count * 100)
    total_ingestion_percentage = math.floor(current_total_row_count / total_row_count * 100)

    if iteration_ingestion_percentage >= expected_threshold_ingestion_percentage or total_ingestion_percentage == 100:
        previous_row_count = current_row_count.copy()

    return [iteration_ingestion_percentage, total_ingestion_percentage] # Ingestion successful


def run_command(command, allow_interruption=False, interrupt_after=None):
    with tempfile.TemporaryFile() as stdout_file, tempfile.TemporaryFile() as stderr_file:
        process = subprocess.Popen(
            command, stdout=stdout_file, stderr=stderr_file, text=True
        )
        start_time = time.time()
        interrupted = False
        chosen_signal = None

        threshold_ingestion_percentage = random.randint(1,3)

        while process.poll() is None:
            if allow_interruption and interrupt_after is not None:
                elapsed_time = time.time() - start_time
                if elapsed_time > interrupt_after:
                    ingestion_percentage, total_ingestion_percentage = validate_ingestion(threshold_ingestion_percentage)
                    print(f"Iteration ingestion percentage: {ingestion_percentage}%, Expected: {threshold_ingestion_percentage}%", flush=True)
                    if ingestion_percentage>=threshold_ingestion_percentage or total_ingestion_percentage==100:
                        # Choose a random signal to send
                        interrupt_signals = [
                            signal.SIGTERM,
                            signal.SIGINT,
                            signal.SIGKILL
                        ]
                        chosen_signal = random.choice(interrupt_signals)
                        print(f"Total ingestion percentage: {total_ingestion_percentage}%", flush=True)
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
        return completed, stdout, stderr


def run_and_resume_voyager(base_command):
    """
    Handles the interruption logic and manages retries for the command.

    Args:
        base_command (list): The base command to execute.
    """
    start_time = time.time()
    attempt = 1
    while time.time() - start_time < max_resumption_time:
        print(f"\n--- Attempt {attempt} of resumption attempts --")

        # Clone base command
        command = base_command.copy()

        # Inject varying flags on each retry
        command = inject_varying_flags_values(command)

        # Randomly determine interruption timing
        interruption_time = random.randint(min_interrupt_seconds, max_interrupt_seconds)

        print(f"\nRunning command: {' '.join(command)}", flush=True)
        print(f"\nInterrupting the command in {interruption_time // 60}m {interruption_time % 60}s...", flush=True)

        completed, stdout, stderr = run_command(command, allow_interruption=True, interrupt_after=interruption_time)

        print("Process was interrupted. Preparing to resume...", flush=True)
        restart_wait_time_seconds = random.randint(min_restart_wait_seconds, max_restart_wait_seconds)
        print(f"Waiting {restart_wait_time_seconds // 60}m {restart_wait_time_seconds % 60}s before resuming...", flush=True)
        time.sleep(restart_wait_time_seconds)
        print("Completed waiting. Proceeding to next attempt...", flush=True)
        attempt += 1
        print(f"Time elapsed: {time.time() - start_time} seconds out of {max_resumption_time} seconds", flush=True)

    # Final attempt without interruption
    print("\n--- Final attempt to complete the import ---\n", flush=True)

    # Inject final set of varying flags before final run
    command = inject_varying_flags_values(base_command.copy())
    completed, stdout, stderr = run_command(command, allow_interruption=False)

    if not completed:
        print("\nCommand failed on the final attempt.", flush=True)
        sys.exit(1)

    print("\nCommand completed successfully on the final attempt.", flush=True)

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

def run_import_with_resumption():
    """
    Runs the import process with resumption logic based on the provided configuration.

    Args:
        config (dict): The configuration dictionary loaded from the YAML file.
    """

    if import_type == 'file':
        command = prepare_import_data_file_command()
    elif import_type == 'offline':
        command = prepare_import_data_command()
    else:
        raise ValueError(f"Unsupported import_type: {import_type}")
        sys.exit(1)

    run_and_resume_voyager(command)


if __name__ == "__main__":
    try:
        args = parse_arguments()
        config = load_config(args.config_file)
        initialize_globals(config)

        print(f"Loaded configuration from {args.config_file}")

        # Run import process
        run_import_with_resumption()

        # Validate rows
        validate_row_counts()

    except Exception as e:
        print(f"Test failed: {e}")
        sys.exit(1)