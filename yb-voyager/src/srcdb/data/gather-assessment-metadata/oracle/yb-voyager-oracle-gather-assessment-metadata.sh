#!/bin/bash
#   Copyright (c) YugabyteDB, Inc.
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   You may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   See the License for the specific language governing permissions and
#   limitations under the License.

set -e

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE:-$0}" )" &> /dev/null && pwd )
SCRIPT_NAME=$(basename $0)

HELP_TEXT="
Usage: $SCRIPT_NAME <oracle_connection_string> <schema_name> <assessment_metadata_dir>

Collects Oracle database statistics and schema information.
Note: The order of the arguments is important and must be followed.

Arguments:
  oracle_connection_string    Oracle connection string in the format:
                              'username@(DESCRIPTION = (ADDRESS = (PROTOCOL = TCP)(HOST = hostname)(PORT = port))(CONNECT_DATA = (SID = SID)))'
                              Ensure this string is properly quoted to avoid shell interpretation issues.

  schema_name                 The name of the schema for which statistics are to be collected.

  assessment_metadata_dir     The directory path where the assessment metadata will be stored.
                              This script will attempt to create the directory if it does not exist.

Example:
  ORACLE_PASSWORD=<password> $SCRIPT_NAME 'system@(DESCRIPTION = (ADDRESS = (PROTOCOL = TCP)(HOST = voyager-oracle.cbtcvpszcgdq.us-west-2.rds.amazonaws.com)(PORT = 1521))(CONNECT_DATA = (SID = DMS)))' 'HR' '/path/to/assessment/metadata'

Please ensure to replace the placeholders with actual values suited to your environment.
"

# Check for the --help option
if [ "$1" == "--help" ]; then
    echo "$HELP_TEXT"
    exit 0
fi

# Check if all required arguments are provided
if [ "$#" -lt 3 ]; then
    echo "Usage: $0 <oracle_connection_string> <schema_name> <assessment_metadata_dir>"
    exit 1
elif [ "$#" -gt 3 ]; then
    echo "Usage: $0 <oracle_connection_string> <schema_name> <assessment_metadata_dir>"
    exit 1
fi

oracle_connection_string=$1
schema_name=$2
assessment_metadata_dir=$3

# Check if assessment_metadata_dir exists, if not exit 1
if [ ! -d "$assessment_metadata_dir" ]; then
    echo "Directory $assessment_metadata_dir does not exist. Please create the directory and try again."
    exit 1
fi

# Resolve the absolute path of assessment_metadata_dir
assessment_metadata_dir=$(cd "$assessment_metadata_dir" && pwd)

# Switch to assessment_metadata_dir and remember the current directory
pushd "$assessment_metadata_dir" > /dev/null || exit

username=$(echo $oracle_connection_string | grep -oP '^[^/@]+')
echo "username: $username"

# Use the ORACLE_PASSWORD environment variable for the password
if [ -z "$ORACLE_PASSWORD" ]; then 
    echo -n "Enter Oracle password: "
    read -s ORACLE_PASSWORD
    echo
fi

oracle_connection_string="${username}/${ORACLE_PASSWORD}@${oracle_connection_string#*@}"

echo "Assessment metadata collection started"

# Loop through each SQLPlus script and execute it
for script in $SCRIPT_DIR/*.sqlplus; do
    script_name=$(basename "$script" .sqlplus)
    script_action=$(basename "$script" .sqlplus | sed 's/-/ /g')
    echo "Collecting $script_action..."
    sqlplus_command="sqlplus -S \"$oracle_connection_string\" @\"$script\" \"$schema_name\""
    eval $sqlplus_command

     # Post-processing step to remove the first line if it's empty in the generated CSV file
    csv_file_path="$assessment_metadata_dir/${script_name%.sqlplus}.csv"
    if [ -f "$csv_file_path" ]; then
        sed -i '1{/^$/d}' "$csv_file_path"
    fi
done

# Function to parse Oracle connection string and generate ORACLE_DSN
parse_oracle_dsn() {
    local conn_str=$1
    local host=$(echo $conn_str | grep -oP '(?<=HOST = )[^)]+')
    local sid=$(echo $conn_str | grep -oP '(?<=SID = )[^)]+')
    local port=$(echo $conn_str | grep -oP '(?<=PORT = )[^)]+')
    echo "dbi:Oracle:host=$host;service_name=$sid;port=$port"
}

ORACLE_DSN_VALUE=$(parse_oracle_dsn "$oracle_connection_string")

# Check for ora2pg installation
if ! command -v ora2pg &> /dev/null; then
    echo "ora2pg could not be found. Please install ora2pg and try again."
    exit 1
fi

rm -rf schema && mkdir -p schema
echo "Collecting schema information..."

ORACLE_HOME_VALUE="${ORACLE_HOME:-"/usr/lib/oracle/21/client64"}"
ORACLE_USER_VALUE=$username
SCHEMA_VALUE=$schema_name
DISABLE_COMMENT_VALUE="1"
DISABLE_PARTITION_VALUE="0"
USE_ORAFCE_VALUE="1"
PARALLEL_TABLES_VALUE="4"
DATA_TYPE_MAPPING_VALUE="VARCHAR2:varchar,NVARCHAR2:varchar,DATE:timestamp,LONG:text,LONG RAW:bytea,CLOB:text,NCLOB:text,BLOB:bytea,BFILE:bytea,RAW(16):uuid,RAW(32):uuid,RAW:bytea,UROWID:oid,ROWID:oid,FLOAT:double precision,DEC:decimal,DECIMAL:decimal,DOUBLE PRECISION:double precision,INT:integer,INTEGER:integer,REAL:real,SMALLINT:smallint,BINARY_FLOAT:double precision,BINARY_DOUBLE:double precision,TIMESTAMP:timestamp,XMLTYPE:xml,BINARY_INTEGER:integer,PLS_INTEGER:integer,TIMESTAMP WITH TIME ZONE:timestamp with time zone,TIMESTAMP WITH LOCAL TIME ZONE:timestamp with time zone"
ALLOW_VALUE=""

# Define the path to the template file and the output file
TEMPLATE_FILE_PATH="/etc/yb-voyager/base-ora2pg.conf"
OUTPUT_FILE_PATH=$assessment_metadata_dir/.ora2pg.conf

# Read the template file and replace placeholders
sed -e "s|{{ .OracleHome }}|$ORACLE_HOME_VALUE|g" \
    -e "s|{{ .OracleDSN }}|$ORACLE_DSN_VALUE|g" \
    -e "s|{{ .OracleUser }}|$ORACLE_USER_VALUE|g" \
    -e "s|{{ .Schema }}|$SCHEMA_VALUE|g" \
    -e "s|{{ .DisableComment }}|$DISABLE_COMMENT_VALUE|g" \
    -e "s|{{ .DisablePartition }}|$DISABLE_PARTITION_VALUE|g" \
    -e "s|{{ .UseOrafce }}|$USE_ORAFCE_VALUE|g" \
    -e "s|{{ .DataTypeMapping }}|$DATA_TYPE_MAPPING_VALUE|g" \
    -e "/{{if .Allow }}/d" \
    -e "s|{{ .Allow }}|$ALLOW_CONFIG|g" \
    -e "/{{end}}/d" \
    $TEMPLATE_FILE_PATH > $OUTPUT_FILE_PATH

# Types to be exported
types=("TYPE" "SEQUENCE" "TABLE" "PACKAGE" "TRIGGER" "FUNCTION" "PROCEDURE" "SYNONYM" "VIEW" "MVIEW")
for type in "${types[@]}"; do
    ltype=$(echo $type | tr '[:upper:]' '[:lower:]' | sed 's/y$/ie/')
    output_dir="$assessment_metadata_dir/schema/${ltype}s"
    output_file="$ltype.sql"
    mkdir -p $output_dir
    # echo "Exporting ${type} to $output_dir/$output_file..."
    ora2pg_cmd="ORA2PG_PASSWD=$ORACLE_PASSWORD ora2pg -p -q -t $type -o $output_file -b $output_dir -c $OUTPUT_FILE_PATH --no_header"
    # echo "executing: $ora2pg_cmd"
    eval $ora2pg_cmd
done

# Return to the original directory after operations are done
popd > /dev/null

echo "Assessment metadata collection completed"
