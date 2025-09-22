#!/usr/bin/env bash

set -e
set -x

if [ $# -lt 1 ] || [ $# -gt 3 ]; then
    echo "Usage: $0 TEST_NAME [--assess-only] [env.sh]"
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
        # Remove the mode flag from arguments (shift everything after $2)
        set -- "$1" "${@:3}"
    elif [ "$2" == "--all-commands" ]; then
        echo "INFO: --all-commands is the default behavior and need not to be specified explicitly"
        set -- "$1" "${@:3}"
    else
        echo "Error: Invalid flag '$2'"
        echo "Valid flags: --assess-only (optional)"
        echo "Usage: $0 TEST_NAME [--assess-only] [env.sh]"
        exit 1
    fi
fi

# Test Setup
export YB_VOYAGER_SEND_DIAGNOSTICS=true
export TEST_NAME=$1
export REPO_ROOT="${PWD}"
export SCRIPTS="${REPO_ROOT}/migtests/scripts"
export TESTS_DIR="${REPO_ROOT}/migtests/tests"
export TEST_DIR="${TESTS_DIR}/${TEST_NAME}"
export PYTHONPATH="${REPO_ROOT}/migtests/lib"

# Load environment
if [ -n "$2" ]; then
    if [ ! -f "${TEST_DIR}/$2" ]; then
        echo "$2 file not found in the test directory"
        exit 1
    fi
    source "${TEST_DIR}/$2"
else
    source "${TEST_DIR}/callhome-env.sh"
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

main() {
    echo "Deleting old export-dir"
    rm -rf "${EXPORT_DIR}"
    mkdir -p "${EXPORT_DIR}"
    chmod +x "${TEST_DIR}/init-db"

    step "START: ${TEST_NAME}"
    print_env

    pushd "${TEST_DIR}"

    step "Check Voyager version"
    yb-voyager version

    step "Initialise database"
    ./init-db

    step "Grant source database user permissions"
    grant_permissions "${SOURCE_DB_NAME}" "${SOURCE_DB_TYPE}" "${SOURCE_DB_SCHEMA}"

    # Starting Flask server to receive callhome data
    step "Start Flask server"
    # Kill any existing Flask server process
    lsof -ti:${FLASK_APP_PORT} | xargs -r kill -9
    flask run --host "$FLASK_SERVER_IP" --port "$FLASK_APP_PORT" &
    
    # Wait for Flask server to start
    step "Wait for Flask server to start"
    for i in {1..30}; do
        if curl -s "http://${FLASK_SERVER_IP}:${FLASK_APP_PORT}/" > /dev/null 2>&1; then
            echo "Flask server is running successfully"
            break
        fi
        if [ $i -eq 30 ]; then
            echo "ERROR: Flask server failed to start after 30 attempts"
            exit 1
        fi
        echo "Waiting for Flask server to start... (attempt $i/30)"
        sleep 1
    done

    step "Assess migration"
    assess_migration --send-diagnostics=true
    step "Compare actual and expected assess-migration callhome data"
    validate_callhome_reports "${TEST_DIR}/expected_callhome_reports/assess_migration_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

    if [ "$MODE" == "--all-commands" ]; then

        step "Create target database."
        run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
        if [ "${SOURCE_DB_TYPE}" = "postgresql" ] || [ "${SOURCE_DB_TYPE}" = "oracle" ]; then
            run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME} with COLOCATION=TRUE"
        else
            run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME}"
        fi

        step "Export schema"
        export_schema --send-diagnostics=true
        step "Compare actual and expected export-schema callhome data"
        validate_callhome_reports "${TEST_DIR}/expected_callhome_reports/export_schema_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

        step "Analyze schema"
        analyze_schema --output-format json --send-diagnostics=true
        step "Compare actual and expected analyze-schema callhome data"
        validate_callhome_reports "${TEST_DIR}/expected_callhome_reports/analyse_schema_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

        step "Import schema"
        import_schema --send-diagnostics=true
        step "Compare actual and expected import-schema callhome data"
        validate_callhome_reports "${TEST_DIR}/expected_callhome_reports/import_schema_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

        step "Run Export Data"
        export_data --send-diagnostics=true
        step "Compare actual and expected export-data callhome data"
        validate_callhome_reports "${TEST_DIR}/expected_callhome_reports/export_data_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

        step "Run Import Data"
        import_data --send-diagnostics=true
        step "Compare actual and expected import-data callhome data"
        validate_callhome_reports "${TEST_DIR}/expected_callhome_reports/import_data_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

        step "Finalize Schema"
        finalize_schema_post_data_import --send-diagnostics=true
        step "Compare actual and expected finalize-schema callhome data"
        validate_callhome_reports "${TEST_DIR}/expected_callhome_reports/finalize_schema_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

        step "End Migration"
        end_migration --yes --send-diagnostics=true
        step "Compare actual and expected end-migration callhome data"
        validate_callhome_reports "${TEST_DIR}/expected_callhome_reports/end_migration_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

        step "All commands passed successfully"
    fi
}

main
