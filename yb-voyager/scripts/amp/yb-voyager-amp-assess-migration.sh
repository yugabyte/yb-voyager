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

# POC: a minimal migration assessment for a PostgreSQL -> YugabyteDB AMP
# migration. AMP's compute is PostgreSQL, so none of voyager's YugabyteDB
# compatibility analysis applies; the assessment reports LOW complexity and a
# vCPU recommendation matched to the source's own capacity.
#
# Writes the same control-plane rows voyager's yugabyted control plane writes
# for `assess-migration`, with a payload shaped like AssessMigrationPayloadYugabyteD,
# so the AMP controller can read it without changes.
#
# This is throwaway scaffolding. The real implementation belongs inside
# `yb-voyager assess-migration --target-db-type yugabytedb-amp`.

set -e

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE:-$0}" )" &> /dev/null && pwd )
SCRIPT_NAME=$(basename $0)

PAYLOAD_VERSION="1.8-amp.1"
VOYAGER_VERSION="amp-assess-poc"
RECOMMENDED_MEMORY_GB=16   # fixed for now; AMP provisioning takes vCPU only

HELP_TEXT="
Usage: $SCRIPT_NAME <pg_connection_string> <schema_list> <output_dir>

Arguments:
  pg_connection_string   Source PostgreSQL connection string, e.g.
                         'postgresql://username@hostname:port/dbname'
                         Quote it to avoid shell interpretation.

  schema_list            Pipe-separated schema list, e.g. 'public|sales'.
                         Recorded on the control-plane row; not otherwise used.

  output_dir             Directory for the local copy of the assessment report.

Environment:
  PGPASSWORD                  Source database password.
  YB_VOYAGER_MIGRATION_UUID   Required. The migration UUID the control-plane
                              rows are keyed by (AMP's migration.id).
  CONTROL_PLANE_TYPE          'yugabyted' to write control-plane rows. Anything
                              else (or unset) writes only the local report.
  YUGABYTED_DB_CONN_STRING    Required when CONTROL_PLANE_TYPE=yugabyted.

Example:
  PGPASSWORD=secret \\
  YB_VOYAGER_MIGRATION_UUID=90b8b0b8-1111-2222-3333-444455556666 \\
  CONTROL_PLANE_TYPE=yugabyted \\
  YUGABYTED_DB_CONN_STRING='postgresql://yugabyte:yugabyte@127.0.0.1:5433' \\
  $SCRIPT_NAME 'postgresql://user@dbhost:5432/prod' 'public' /tmp/amp-assessment
"

if [ "$1" = "-h" ] || [ "$1" = "--help" ] || [ $# -lt 3 ]; then
    echo "$HELP_TEXT"
    [ $# -lt 3 ] && exit 1
    exit 0
fi

SOURCE_CONN="$1"
SCHEMA_LIST="$2"
OUTPUT_DIR="$3"

if [ -z "$YB_VOYAGER_MIGRATION_UUID" ]; then
    echo "ERROR: YB_VOYAGER_MIGRATION_UUID must be set." >&2
    exit 1
fi

WRITE_CONTROL_PLANE=false
if [ "$CONTROL_PLANE_TYPE" = "yugabyted" ]; then
    if [ -z "$YUGABYTED_DB_CONN_STRING" ]; then
        echo "ERROR: YUGABYTED_DB_CONN_STRING must be set when CONTROL_PLANE_TYPE=yugabyted." >&2
        exit 1
    fi
    WRITE_CONTROL_PLANE=true
fi

mkdir -p "$OUTPUT_DIR/assessment/reports"
OUTPUT_DIR=$( cd "$OUTPUT_DIR" && pwd )
REPORT_PATH="$OUTPUT_DIR/assessment/reports/migration_assessment_report.json"

src()  { psql -X -q -tA "$SOURCE_CONN" "$@"; }
cp_()  { psql -X -q -tA "$YUGABYTED_DB_CONN_STRING" "$@"; }

# --- source facts -------------------------------------------------------------

echo "Collecting source database facts..."
FACTS=$(src -v ON_ERROR_STOP=on -f "$SCRIPT_DIR/amp-source-facts.psql")
IFS='|' read -r IS_SUPERUSER HAS_READ_ROLE HAS_READ_EXEC \
                MAX_WORKER_PROCESSES MAX_PARALLEL_WORKERS \
                DB_NAME DB_VERSION SERVER_ADDR SERVER_PORT DB_SIZE <<< "$FACTS"

echo "  database=$DB_NAME version=$DB_VERSION"

# --- control plane: bootstrap and the IN PROGRESS row -------------------------

# Mirrors voyager: the started row is written before any assessment work, and
# the completed row gets the next invocation_sequence rather than updating it.
if [ "$WRITE_CONTROL_PLANE" = true ]; then
    echo "Bootstrapping control-plane schema..."
    cp_ -v ON_ERROR_STOP=on -f "$SCRIPT_DIR/amp-controlplane-bootstrap.psql" > /dev/null

    SEQ=$(cp_ -v ON_ERROR_STOP=on -c "
        SELECT coalesce(MAX(invocation_sequence), 0) + 1
          FROM ybvoyager_visualizer.ybvoyager_visualizer_metadata
         WHERE migration_uuid = '$YB_VOYAGER_MIGRATION_UUID' AND migration_phase = 1;")

    HOST_IP_JSON="{\"SourceDBIP\":\"${SERVER_ADDR}\"}"
    LOCAL_IP=$(hostname -i 2>/dev/null | awk '{print $1}')
    [ -z "$LOCAL_IP" ] && LOCAL_IP="127.0.0.1"
    DISK_AVAIL=$(df -Pk "$OUTPUT_DIR" | awk 'NR==2 {print $4 * 1024}')
    VOYAGER_INFO_JSON=$(printf '{"IP":"%s","OperatingSystem":"%s","DiskSpaceAvailable":%s,"ExportDirectory":"%s"}' \
        "$LOCAL_IP" "$(uname -s | tr 'A-Z' 'a-z')" "$DISK_AVAIL" "$OUTPUT_DIR")

    # psql interpolates :'var' only for script input, never for -c, so the
    # statement is fed on stdin.
    insert_cp_row() {   # $1 = invocation_sequence, $2 = status, $3 = payload
        cp_ -v ON_ERROR_STOP=on \
            -v uuid="$YB_VOYAGER_MIGRATION_UUID" -v seq="$1" -v status="$2" -v payload="$3" \
            -v mdir="$OUTPUT_DIR" -v dbname="$DB_NAME" -v schemas="$SCHEMA_LIST" \
            -v hostip="$HOST_IP_JSON" -v port="$SERVER_PORT" -v dbver="$DB_VERSION" \
            -v vinfo="$VOYAGER_INFO_JSON" <<'SQL' > /dev/null
INSERT INTO ybvoyager_visualizer.ybvoyager_visualizer_metadata (
    migration_uuid, migration_phase, invocation_sequence, migration_dir,
    database_name, schema_name, host_ip, port, db_version, payload,
    voyager_info, db_type, status, invocation_timestamp
) VALUES (
    :'uuid', 1, :'seq'::int, :'mdir', :'dbname', :'schemas', :'hostip',
    :'port'::int, :'dbver', :'payload', :'vinfo', 'postgresql', :'status', now()
);
SQL
    }

    echo "Recording assessment start (invocation_sequence=$SEQ)..."
    insert_cp_row "$SEQ" "IN PROGRESS" ""
fi

# --- vCPU detection -----------------------------------------------------------
#
#   exact  read the source host's real CPU limits through pg_read_file
#   high   invert a vCPU-derived GUC. PostgreSQL ships max_worker_processes and
#          max_parallel_workers at exactly 8, so a value above 8 means something
#          sized them from the machine, using the documented formulas
#          max_worker_processes = GREATEST(vCPU*2,8) and
#          max_parallel_workers = GREATEST(vCPU/2,8).
#   -1     no signal. The caller decides what to do.

VCPUS=-1
MEMORY_GB=-1
CONFIDENCE="unknown"
DETECTION_METHOD="none"

count_cpu_list() {   # "0-3,8,10-11" -> 7
    local total=0 part lo hi
    [ -z "$1" ] && { echo 0; return; }
    IFS=',' read -ra parts <<< "$1"
    for part in "${parts[@]}"; do
        if [[ "$part" == *-* ]]; then
            lo=${part%-*}; hi=${part#*-}
            total=$(( total + hi - lo + 1 ))
        else
            total=$(( total + 1 ))
        fi
    done
    echo "$total"
}

if [ "$IS_SUPERUSER" = "on" ] || { [ "$HAS_READ_ROLE" = "true" ] && [ "$HAS_READ_EXEC" = "true" ]; }; then
    echo "Reading source host CPU limits..."
    if OS_CAPACITY=$(src -v ON_ERROR_STOP=on -f "$SCRIPT_DIR/amp-source-os-capacity.psql" 2>/dev/null); then
        IFS='|' read -r CPU_MAX CPUS_ALLOWED CPUINFO_COUNT MEMTOTAL_KB <<< "$OS_CAPACITY"

        # cpuset-aware count, falling back to the host-wide core count
        DETECTED=$(count_cpu_list "$CPUS_ALLOWED")
        METHOD="Cpus_allowed_list"
        if [ "$DETECTED" -eq 0 ] && [ -n "$CPUINFO_COUNT" ]; then
            DETECTED=$CPUINFO_COUNT
            METHOD="/proc/cpuinfo"
        fi

        # a CFS quota caps it further; "max 100000" means no quota
        QUOTA=${CPU_MAX%% *}
        PERIOD=${CPU_MAX##* }
        if [ -n "$QUOTA" ] && [ "$QUOTA" != "max" ] && [ "$PERIOD" -gt 0 ] 2>/dev/null; then
            QUOTA_CPUS=$(( (QUOTA + PERIOD - 1) / PERIOD ))   # round up
            if [ "$QUOTA_CPUS" -gt 0 ] && [ "$QUOTA_CPUS" -lt "$DETECTED" ]; then
                DETECTED=$QUOTA_CPUS
                METHOD="cgroup cpu.max"
            fi
        fi

        if [ "$DETECTED" -gt 0 ]; then
            VCPUS=$DETECTED
            CONFIDENCE="exact"
            DETECTION_METHOD="$METHOD"
            [ -n "$MEMTOTAL_KB" ] && MEMORY_GB=$(( MEMTOTAL_KB / 1048576 ))
        fi
    else
        echo "  host read failed; falling back to parameter inference"
    fi
fi

if [ "$VCPUS" -le 0 ]; then
    if [ "$MAX_WORKER_PROCESSES" -gt 8 ]; then
        VCPUS=$(( MAX_WORKER_PROCESSES / 2 ))
        CONFIDENCE="high"
        DETECTION_METHOD="max_worker_processes/2"
    elif [ "$MAX_PARALLEL_WORKERS" -gt 8 ]; then
        VCPUS=$(( MAX_PARALLEL_WORKERS * 2 ))
        CONFIDENCE="high"
        DETECTION_METHOD="max_parallel_workers*2"
    fi
fi

if [ "$VCPUS" -le 0 ]; then
    echo "  could not determine source vCPU count; reporting -1"
else
    echo "  source vCPUs: $VCPUS (confidence=$CONFIDENCE via $DETECTION_METHOD)"
fi

# --- payload ------------------------------------------------------------------

# json_build_object, not jsonb_build_object: jsonb reorders keys, json preserves
# insertion order so the payload matches Go's marshal order field for field.
if [ "$VCPUS" -gt 0 ]; then
    SIZING_REASONING="Recommended AMP compute matched to source capacity: ${VCPUS} vCPU (confidence: ${CONFIDENCE}, via ${DETECTION_METHOD})."
    FAILURE_REASONING=""
else
    SIZING_REASONING="Source vCPU count could not be determined; VCPUsPerInstance is reported as -1."
    FAILURE_REASONING="Unable to determine source vCPU count from the source database."
fi

echo "Building assessment payload..."
PAYLOAD=$(src -v ON_ERROR_STOP=on \
    -v pver="$PAYLOAD_VERSION" -v vver="$VOYAGER_VERSION" \
    -v vcpus="$VCPUS" -v memgb="$RECOMMENDED_MEMORY_GB" -v srcmem="$MEMORY_GB" \
    -v conf="$CONFIDENCE" -v method="$DETECTION_METHOD" \
    -v dbsize="$DB_SIZE" -v reasoning="$SIZING_REASONING" -v failure="$FAILURE_REASONING" \
    <<'SQL'
SELECT json_build_object(
    'PayloadVersion', :'pver',
    'VoyagerVersion', :'vver',
    'TargetDBVersion', NULL,
    'MigrationComplexity', 'LOW',
    'MigrationComplexityExplanation',
        'YugabyteDB AMP is a PostgreSQL-compatible compute, so a PostgreSQL source needs no schema conversion. This assessment reports sizing only; it does not analyse the schema for incompatibilities.',
    'SchemaSummary', json_build_object(),
    'ParsedSchemaSummary', json_build_object(),
    'AssessmentIssues', json_build_array(),
    'SourceSizeDetails', json_build_object(
        'TotalDBSize', :'dbsize'::bigint,
        'TotalTableSize', 0,
        'TotalIndexSize', 0,
        'TotalTableRowCount', 0),
    'TargetRecommendations', json_build_object('TotalColocatedSize', 0, 'TotalShardedSize', 0),
    'ConversionIssues', json_build_array(),
    'Sizing', json_build_object(
        'SizingRecommendation', json_build_object(
            'ColocatedTables', json_build_array(),
            'ColocatedReasoning', :'reasoning',
            'ShardedTables', json_build_array(),
            'NumNodes', 1,
            'VCPUsPerInstance', :'vcpus'::int,
            'MemoryPerInstance', :'memgb'::int,
            'OptimalSelectConnectionsPerNode', 0,
            'OptimalInsertConnectionsPerNode', 0,
            'EstimatedTimeInMinForImport', 0,
            'EstimatedTimeInMinForImportWithoutRedundantIndexes', 0),
        'FailureReasoning', :'failure'),
    'TableIndexStats', NULL,
    'SourceEnvironment', json_build_object(
        'VCPUs', :'vcpus'::int,
        'MemoryGiB', :'srcmem'::int,
        'DetectionMethod', :'method',
        'Confidence', :'conf'),
    'GeneralNotes', json_build_array(
        'Generated by the AMP assessment POC script, not by yb-voyager assess-migration. Schema compatibility, datatypes, extensions and query constructs were not analysed.'),
    'ColocatedShardedNotes', json_build_array(),
    'SizingNotes', json_build_array(:'reasoning'),
    'Notes', json_build_array(:'reasoning')
);
SQL
)

printf '%s\n' "$PAYLOAD" > "$REPORT_PATH"
echo "Assessment report written to $REPORT_PATH"

# --- control plane: the COMPLETED row -----------------------------------------

if [ "$WRITE_CONTROL_PLANE" = true ]; then
    echo "Recording assessment completion (invocation_sequence=$(( SEQ + 1 )))..."
    insert_cp_row "$(( SEQ + 1 ))" "COMPLETED" "$PAYLOAD"
fi

echo "Done."
