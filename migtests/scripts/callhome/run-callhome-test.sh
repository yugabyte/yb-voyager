#!/usr/bin/env bash

set -e
set -x

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
    echo "Usage: $0 TEST_NAME [--assess-only]"
    echo "  --all-commands is the default behavior (no flag needed)"
    echo "  Use --assess-only to run only the assess migration step"
    exit 1
fi

# Set default mode to --all-commands
MODE="--all-commands"

# Check if second argument is a mode flag
if [ $# -ge 2 ] && [[ "$2" == --* ]]; then
    if [ "$2" == "--assess-only" ]; then
        MODE="--assess-only"
    else
        echo "Error: Invalid flag '$2'"
        echo "Valid flags: --assess-only (optional)"
        echo "Usage: $0 TEST_NAME [--assess-only]"
        exit 1
    fi
fi

# Test Setup
export TEST_NAME=$1
export REPO_ROOT="${PWD}"
export SCRIPTS="${REPO_ROOT}/migtests/scripts"
export TESTS_DIR="${REPO_ROOT}/migtests/tests"
export TEST_DIR="${TESTS_DIR}/${TEST_NAME}"
export PYTHONPATH="${REPO_ROOT}/migtests/lib"

# Load environment
if [ ! -f "${TEST_DIR}/env.sh" ]; then
    echo "${TEST_DIR}/env.sh file not found in the test directory"
    exit 1
else
    source "${TEST_DIR}/env.sh"
fi

if [ -n "${SOURCE_DB_TYPE}" ]; then
    source "${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh"
fi

source "${SCRIPTS}/functions.sh"

normalize_and_export_vars "callhome"

source "${SCRIPTS}/yugabytedb/env.sh"

# Callhome Server setup
export FLASK_APP=${SCRIPTS}/callhome/server.py
export FLASK_APP_PORT=5000
export FLASK_SERVER_IP=localhost
export LOCAL_CALL_HOME_SERVICE_HOST=$FLASK_SERVER_IP
export LOCAL_CALL_HOME_SERVICE_PORT=$FLASK_APP_PORT

# Anonymisation setup
export VOYAGER_ENABLE_DETERMINISTIC_ANON=true
export VOYAGER_TEST_ANON_SALT=${ANON_SALT}

start_flask_server() {
    step "Start Flask server"
    flask run --host "$FLASK_SERVER_IP" --port "$FLASK_APP_PORT" &
    FLASK_PID=$!

    # wait until its reachable (max 30s)
    curl -s \
    --retry 30 \
    --retry-delay 1 \
    --retry-connrefused \
    --max-time 1 \
    "http://${FLASK_SERVER_IP}:${FLASK_APP_PORT}/" >/dev/null || {
        echo "ERROR: Flask failed to start in time"
        kill "$FLASK_PID" 2>/dev/null || true
        exit 1
    }

    echo "Flask server is running"
}

stop_flask_server() {
    step "Stop the Flask server"
    lsof -ti:${FLASK_APP_PORT} | xargs -r kill -15
    
    # Wait for port to be free, checking every second for 30 seconds
    step "Wait for Flask server to stop gracefully"
    for i in {1..30}; do
        if ! lsof -ti:${FLASK_APP_PORT} > /dev/null 2>&1; then
            echo "Flask server stopped gracefully"
            break
        fi
        if [ $i -eq 30 ]; then
            echo "Flask server did not stop gracefully, force killing"
            lsof -ti:${FLASK_APP_PORT} | xargs -r kill -9
            echo "Flask server process killed"
        else
            echo "Waiting for Flask server to stop... (attempt $i/30)"
            sleep 1
        fi
    done
}

main() {
    echo "Deleting old export-dir"
    rm -rf "${EXPORT_DIR}"
    mkdir -p "${EXPORT_DIR}"

    step "START: ${TEST_NAME}"
    print_env

    pushd "${TEST_DIR}"

    step "Check Voyager version"
    yb-voyager version

    step "Initialise database"
    ./init-db

    if [ "${SOURCE_DB_TYPE}" = "postgresql" ]; then
		step "Creating pg_stat_statements for the compare-performance command"
		run_psql ${SOURCE_DB_NAME} "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
	fi

    step "Grant source database user permissions"
    grant_permissions "${SOURCE_DB_NAME}" "${SOURCE_DB_TYPE}" "${SOURCE_DB_SCHEMA}"

    # Starting Flask server to receive callhome data
    start_flask_server

    step "Assess migration"
    assess_migration --send-diagnostics=true
    step "Compare actual and expected assess-migration callhome data"
    expected_file="${TEST_DIR}/expected_callhome_payloads/assess_migration_callhome.json"
    compare_callhome_json_reports "${expected_file}" "assess-migration"

    if [ "$MODE" != "--all-commands" ]; then
        stop_flask_server
        return
    fi

    step "Create target database."
    run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
    run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME} with COLOCATION=TRUE;"

    step "Export schema"
    export_schema --send-diagnostics=true
    step "Compare actual and expected export-schema callhome data"
    expected_file="${TEST_DIR}/expected_callhome_payloads/export_schema_callhome.json"
    compare_callhome_json_reports "${expected_file}" "export-schema"

    step "Analyze schema"
    analyze_schema --output-format json --send-diagnostics=true
    step "Compare actual and expected analyze-schema callhome data"
    expected_file="${TEST_DIR}/expected_callhome_payloads/analyse_schema_callhome.json"
    compare_callhome_json_reports "${expected_file}" "analyze-schema"

    step "Fix schema."
	if [ -x "${TEST_DIR}/fix-schema" ]
	then
		 "${TEST_DIR}/fix-schema"
	fi

    step "Import schema"
    import_schema --send-diagnostics=true
    step "Compare actual and expected import-schema callhome data"
    expected_file="${TEST_DIR}/expected_callhome_payloads/import_schema_callhome.json"
    compare_callhome_json_reports "${expected_file}" "import-schema"

    step "Run Export Data"
    export_data --send-diagnostics=true
    step "Compare actual and expected export-data callhome data"
    expected_file="${TEST_DIR}/expected_callhome_payloads/export_data_callhome.json"
    compare_callhome_json_reports "${expected_file}" "export-data"

    step "Run Import Data"
    import_data --send-diagnostics=true
    step "Compare actual and expected import-data callhome data"
    expected_file="${TEST_DIR}/expected_callhome_payloads/import_data_callhome.json"
    compare_callhome_json_reports "${expected_file}" "import-data"

    step "Finalize Schema"
    finalize_schema_post_data_import --send-diagnostics=true
    step "Compare actual and expected finalize-schema callhome data"
    expected_file="${TEST_DIR}/expected_callhome_payloads/finalize_schema_callhome.json"
    compare_callhome_json_reports "${expected_file}" "finalize-schema"

    step "Run Performance Comparison"
    compare_performance --send-diagnostics=true
    step "Compare actual and expected compare-performance callhome data"
    expected_file="${TEST_DIR}/expected_callhome_payloads/compare_performance_callhome.json"
    compare_callhome_json_reports "${expected_file}" "compare-performance"

    step "End Migration"
    end_migration --yes --send-diagnostics=true
    step "Compare actual and expected end-migration callhome data"
    expected_file="${TEST_DIR}/expected_callhome_payloads/end_migration_callhome.json"
    compare_callhome_json_reports "${expected_file}" "end-migration"

    step "All commands passed successfully"

    stop_flask_server
}

main
