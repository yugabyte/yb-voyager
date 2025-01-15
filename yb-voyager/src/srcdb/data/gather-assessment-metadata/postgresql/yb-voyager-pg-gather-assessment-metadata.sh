#!/bin/bash
#   Copyright (c) YugabyteDB, Inc.
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#	    http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

set -e

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE:-$0}" )" &> /dev/null && pwd ) 
SCRIPT_NAME=$(basename $0)

HELP_TEXT="
Usage: $SCRIPT_NAME <pg_connection_string> <schema_list> <assessment_metadata_dir>

Collects PostgreSQL database statistics and schema information.
Note: The order of the arguments is important and must be followed.

Arguments:
  pg_connection_string        PostgreSQL connection string in the format:
                             'postgresql://username@hostname:port/dbname'
                              Ensure this string is properly quoted to avoid shell interpretation issues.

  schema_list                 Pipe-separated list of schemas for which statistics are to be collected.
                              Example: 'public|sales|inventory'

  assessment_metadata_dir     The directory path where the assessment metadata will be stored.
                              This script will attempt to create the directory if it does not exist.

  pgss_enabled                Determine whether the pg_stat_statements extension is correctly installed, 
                              configured, and enabled on the source database.

  iops_capture_interval       Configure the interval for measuring the IOPS metadata on source (in seconds). (Default 120)

Example:
  PGPASSWORD=<password> $SCRIPT_NAME 'postgresql://user@localhost:5432/mydatabase' 'public|sales' '/path/to/assessment/metadata' 'true' '60'

Please ensure to replace the placeholders with actual values suited to your environment.
"

# Check for the --help option
if [ "$1" == "--help" ]; then
    echo "$HELP_TEXT"
    exit 0
fi

# Check if all required arguments are provided
if [ "$#" -lt 4 ]; then
    echo "Usage: $0 <pg_connection_string> <schema_list> <assessment_metadata_dir> <pgss_enabled> [iops_capture_interval]"
    exit 1
elif [ "$#" -gt 5 ]; then
    echo "Usage: $0 <pg_connection_string> <schema_list> <assessment_metadata_dir> <pgss_enabled> [iops_capture_interval]"
    exit 1
fi

pg_connection_string=$1
schema_list=$2
assessment_metadata_dir=$3
# check if assessment_metadata_dir exists, if not exit 1
if [ ! -d "$assessment_metadata_dir" ]; then
    echo "Directory $assessment_metadata_dir does not exist. Please create the directory and try again."
    exit 1
fi

pgss_enabled=$4
iops_capture_interval=120 # default sleep for calculating iops
# Override default sleep interval if a fifth argument is provided
if [ "$#" -eq 5 ]; then
    iops_capture_interval=$5
    echo "sleep interval for calculating iops: $iops_capture_interval seconds"
fi



LOG_FILE=$assessment_metadata_dir/yb-voyager-assessment.log
log() {
    local level="$1"
    shift
    local message="$@"
    echo "[$(date -u +'%Y-%m-%d %H:%M:%S')] [$level] $message" | tee -a "$LOG_FILE" > /dev/null
}

print_and_log() {
    local level="$1"
    shift
    local message="$@"
    echo "$message"
    log "$level" "$message"
}

# Function used to quote the shell_variables where cmds are defined as strings and later used with eval command
# for example: psql_command and pg_dump_command
quote_string() {
    local str="$1"

    # Check if the string is already quoted with single or double quotes
    if [[ $str == \'*\' ]] || [[ $str == \"*\" ]]; then
        echo "$str"
    else # otherwise, add single quotes around the string
        echo "'$str'"
    fi
}

# the error returned by the command in `eval $command 2>&1 | tee -a "$LOG_FILE"` was getting ignored
# this function checks the PIPESTATUS[0] of the first command
run_command() {
    local command="$1"
    # print and log the stderr/stdout of the command
    eval $command 2>&1 | tee -a "$LOG_FILE"
    if [ ${PIPESTATUS[0]} -ne 0 ]; then
        print_and_log "ERROR" "command failed: $command"
        exit 1
    fi
}


# Function to convert schema list to an array and ensure 'public' is included
prepare_schema_array() {
    local schema_list=$1
    local -a schema_array

    # Convert the schema list (pipe-separated) to an array
    IFS='|' read -r -a schema_array <<< "$schema_list"
    local public_found=false
    for schema in "${schema_array[@]}"; do
        if [[ "$schema" == "public" ]]; then
            public_found=true
            break
        fi
    done

    if [[ $public_found == false ]]; then
        schema_array+=("public")
    fi

    echo "${schema_array[*]}"
}

main() {
    # Resolve the absolute path of assessment_metadata_dir
    assessment_metadata_dir=$(cd "$assessment_metadata_dir" && pwd)
    
    # Switch to assessment_metadata_dir and remember the current directory
    log "INFO" "switch to assessment_metadata_dir='$assessment_metadata_dir'"
    pushd "$assessment_metadata_dir" > /dev/null || exit

    if [ -z "$PGPASSWORD" ]; then 
        echo -n "Enter PostgreSQL password: "
        read -s PGPASSWORD
        echo
    fi
    # exporting irrespective it was set or not
    export PGPASSWORD

    track_counts_on=$(psql $pg_connection_string -tAqc "SELECT setting FROM pg_settings WHERE name = 'track_counts';")
    if [ "$track_counts_on" != "on" ]; then
        print_and_log "WARN" "Warning: track_counts is not enabled in the PostgreSQL configuration."
        echo "It's required for calculating reads/writes per second stats of tables/indexes. Do you still want to continue? (Y/N): "
        read continue_execution
        continue_execution=$(echo "$continue_execution" | tr '[:upper:]' '[:lower:]') # converting to lower case for easier comparison
        if [ "$continue_execution" != "yes" ] && [ "$continue_execution" != "y" ]; then
            print_and_log "INFO" "Exiting..."
            exit 2
        fi
    fi

    # checking before quoting connection_string
    pgss_ext_schema=$(psql -A -t -q $pg_connection_string -c "SELECT nspname FROM pg_extension e, pg_namespace n WHERE e.extnamespace = n.oid AND e.extname = 'pg_stat_statements'")
    log "INFO" "pg_stat_statements extension is available in schema: $pgss_ext_schema"

    schema_array=$(prepare_schema_array $schema_list)
    log "INFO" "schema_array for checking pgss_ext_schema: $schema_array"

    # quote the required shell variables
    pg_connection_string=$(quote_string "$pg_connection_string")
    schema_list=$(quote_string "$schema_list")

    print_and_log "INFO" "Assessment metadata collection started for $schema_list schema(s)"
    for script in $SCRIPT_DIR/*.psql; do
        script_name=$(basename "$script" .psql)
        script_action=$(basename "$script" .psql | sed 's/-/ /g')
        
        print_and_log "INFO" "Collecting $script_action..."
        
        case $script_name in
            "table-index-iops")
                psql_command="psql -q $pg_connection_string -f $script -v schema_list=$schema_list -v ON_ERROR_STOP=on -v measurement_type=initial"
                log "INFO" "Executing initial IOPS collection: $psql_command"
                run_command "$psql_command"
                mv table-index-iops.csv table-index-iops-initial.csv

                log "INFO" "Sleeping for $iops_capture_interval seconds to capture IOPS data"
                # sleeping to calculate the iops reading two different time intervals, to calculate reads_per_second and writes_per_second
                sleep $iops_capture_interval

                psql_command="psql -q $pg_connection_string -f $script -v schema_list=$schema_list -v ON_ERROR_STOP=on -v measurement_type=final"
                log "INFO" "Executing final IOPS collection: $psql_command"
                run_command "$psql_command"
                mv table-index-iops.csv table-index-iops-final.csv
            ;;
            "db-queries-summary")
                if [[ "$REPORT_UNSUPPORTED_QUERY_CONSTRUCTS" == "false" ]]; then
                    print_and_log "INFO" "Skipping $script_action: Reporting of unsupported query constructs is disabled."
                    continue
                fi

                log "INFO" "argument pgss_enabled=$pgss_enabled"
                if [[ "$pgss_enabled" == "false" ]]; then
                    print_and_log "WARN" "Skipping $script_action: argument pgss_enabled is set as false"
                    continue
                fi

                psql_command="psql -q $pg_connection_string -f $script -v schema_name=$pgss_ext_schema -v ON_ERROR_STOP=on"
                log "INFO" "Executing script: $psql_command"
                run_command "$psql_command"
            ;;
            *)
                psql_command="psql -q $pg_connection_string -f $script -v schema_list=$schema_list -v ON_ERROR_STOP=on"
                log "INFO" "Executing script: $psql_command"
                run_command "$psql_command"
            ;;
        esac
    done

    # check for pg_dump version
    pg_dump_version=$(pg_dump --version | awk '{print $3}' | awk -F. '{print $1}')
    log "INFO" "extracted pg_dump version: $pg_dump_version"
    if [ "$pg_dump_version" -lt 14 ]; then
        print_and_log "ERROR" "pg_dump version is less than 14. Please upgrade to version 14 or higher."
        exit 1
    fi

    mkdir -p schema
    print_and_log "INFO" "Collecting schema information..."
    pg_dump_command="pg_dump $pg_connection_string --schema-only --schema=$schema_list --extension=\"*\" --no-comments --no-owner --no-privileges --no-tablespaces --load-via-partition-root --file='schema/schema.sql'"
    log "INFO" "Executing pg_dump: $pg_dump_command"
    run_command "$pg_dump_command"

    # Return to the original directory after operations are done
    popd > /dev/null

    print_and_log "INFO" "Assessment metadata collection completed"
}

main