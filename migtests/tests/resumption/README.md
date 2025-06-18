# Resumption Test Script for YB Voyager

The `resumption.py` script is designed to automate the testing of YB Voyager's resumption capabilities. It runs the import process with simulated interruptions according to the config provided and validates the results. It also supports multiple schemas and case-sensitive / reserved word table names. The parent script `resumption.sh` sets up the environment, initializes databases, orchestrates the execution of the Python script, and handles other things like flexible config injections etc.

## Features

- **Configurable Import Process**: Supports both `file` and `offline` import types.
- **Resumption Logic**: Simulates interruptions and retries based on configurable parameters.
- **Dynamic Flag Injection**: Allows varying configurations for flags during retries.
- **Row Count Validation**: Validates the row counts of target tables after the import process.
- **Customizable via YAML Configuration**: Reads settings from a YAML configuration file and supports additional configurations.

## Usage

```bash
[resumption.sh] TEST_NAME [env.sh] [config_file]
```

### Configuration Fields

- `import_type`: Type of import (`file` or `offline`) (default: `file`)
- `additional_flags`: List of additional flags passed to the import command
- `row_count`: A dictionary with expected row counts for each table
- `resumption`: A dictionary with resumption settings:
  - `max_restarts`: Max number of retries with interruption
  - `min_interrupt_seconds`: Minimum seconds before interrupting
  - `max_interrupt_seconds`: Maximum seconds before interrupting
  - `min_restart_wait_seconds`: Minimum seconds before resuming
  - `max_restart_wait_seconds`: Maximum seconds before resuming
- `varying_flags`: A list of flags to inject different values on each retry

## Sample Configurations

### Import File

```yaml
# File Table Map
file_table_map: "table1_data.sql:table1"

# Additional Flags to add to the import command
additional_flags:
  --delimiter: "\\t"
  --format: "text"

# Row Count Validation
row_count:
  table1: 35000000

# Resumption Settings
resumption:
  max_restarts: 20
  min_interrupt_seconds: 18
  max_interrupt_seconds: 48
  min_restart_wait_seconds: 3
  max_restart_wait_seconds: 6

# Configs which are run with varying values for the flags
varying_configs:
  --parallel-jobs:
    type: range
    value: [1, 10]
```

### Offline Import

```yaml
# Additional Flags
# Uncomment the below to add any additional flags to the command

# additional_flags:
#   --delimiter: "\\t"

# Import Type
import_type: offline

# Row Count Validation
row_count:
  table1: 5000000
  schema2.Case_Sensitive_Table: 5000000

# Resumption Settings
resumption:
  max_restarts: 25
  min_interrupt_seconds: 20
  max_interrupt_seconds: 45
  min_restart_wait_seconds: 15
  max_restart_wait_seconds: 30

```

