#!/usr/bin/env bash

set -e
set -x

if [ $# -lt 1 ] || [ $# -gt 4 ]; then
    echo "Usage: $0 TEST_NAME [--assess-only] [env.sh] [config_file]"
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
        # Remove the mode flag from arguments (shift everything after $1)
        set -- "$1" "${@:3}"
    elif [ "$2" == "--all-commands" ]; then
        echo "INFO: --all-commands is the default behavior and need not to be specified explicitly"
        set -- "$1" "${@:3}"
    else
        echo "Error: Invalid flag '$2'"
        echo "Valid flags: --assess-only (optional)"
        echo "Usage: $0 TEST_NAME [--assess-only] [env.sh] [config_file]"
        exit 1
    fi
fi

# If exactly 2 arguments are passed, and the second argument ends with .yaml,
# treat it as the config_file and leave env.sh as an empty string
if [ $# -eq 2 ] && [[ "$2" == *.yaml ]]; then
    set -- "$1" "" "$2"
fi

export YB_VOYAGER_SEND_DIAGNOSTICS=true
export TEST_NAME=$1

export REPO_ROOT="${PWD}"
export SCRIPTS="${REPO_ROOT}/migtests/scripts"
export TESTS_DIR="${REPO_ROOT}/migtests/tests"
export TEST_DIR="${TESTS_DIR}/${TEST_NAME}"
export EXPORT_DIR=${EXPORT_DIR:-"${TEST_DIR}/export-dir"}
export FLASK_APP=${SCRIPTS}/callhome/server.py
export PORT=5000
export SERVER_IP=localhost
export VOYAGER_ENABLE_DETERMINISTIC_ANON=true
export VOYAGER_TEST_ANON_SALT=a9f8c2d4e7b6f1a3d9c0b7e4f5a6c3d2

export PYTHONPATH="${REPO_ROOT}/migtests/lib"

# Load environment
if [ -n "$2" ]; then
    if [ ! -f "${TEST_DIR}/$2" ]; then
        echo "$2 file not found in the test directory"
        exit 1
    fi
    source "${TEST_DIR}/$2"
else
    source "${TEST_DIR}/env.sh"
fi

if [ -n "${SOURCE_DB_TYPE}" ]; then
    source "${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh"
fi

source "${SCRIPTS}/yugabytedb/env.sh"
source "${SCRIPTS}/functions.sh"

# Config file
export CONFIG_FILE="${TEST_DIR}/configs/default_config.yaml"
if [ -n "$3" ]; then
    CONFIG_FILE="${TEST_DIR}/configs/$3"
    if [ ! -f "${CONFIG_FILE}" ]; then
        echo "Error: Config file $3 not found in the test directory."
        exit 1
    fi
fi

main() {
    echo "Deleting old export-dir"
    rm -rf "${EXPORT_DIR}"
    mkdir -p "${EXPORT_DIR}"

    # Ensure helper scripts are executable
    for script in init-db init-target-db generate_config.py; do
        if [ -f "${TEST_DIR}/${script}" ]; then
            chmod +x "${TEST_DIR}/${script}"
        fi
    done

    step "START: ${TEST_NAME}"
    print_env

    pushd "${TEST_DIR}"

    step "Check Voyager version"
    yb-voyager version

    step "Initialise databases"
    for script in init-db init-target-db; do
        if [ -f "${TEST_DIR}/${script}" ]; then
            "${TEST_DIR}/${script}"
        fi
    done

    if [ -n "${SOURCE_DB_TYPE}" ]; then
        step "Grant source database user permissions"
        grant_permissions "${SOURCE_DB_NAME}" "${SOURCE_DB_TYPE}" "${SOURCE_DB_SCHEMA}"
        
        sudo apt install python3-flask -y

        step "Start Flask server"
        lsof -ti:${PORT} | xargs -r kill -9
        flask run --host "$SERVER_IP" --port "$PORT" &
        sleep 5

        export LOCAL_CALL_HOME_SERVICE_HOST=$SERVER_IP
        export LOCAL_CALL_HOME_SERVICE_PORT=$PORT

        rm -rf "${TEST_DIR}/actualCallhomeReport.json"
        step "Assess migration"
        assess_migration --send-diagnostics=true
        sleep 5

        step "Compare actual and expected assess-migration callhome data"
        compare_json_reports "${TEST_DIR}/expected_callhome_reports/assess_migration_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

        if [ "$MODE" == "--all-commands" ]; then
            rm -rf "${TEST_DIR}/actualCallhomeReport.json"
            step "Export schema"
            export_schema --send-diagnostics=true
            sleep 5

            step "Compare actual and expected export-schema callhome data"
            compare_json_reports "${TEST_DIR}/expected_callhome_reports/export_schema_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

            rm -rf "${TEST_DIR}/actualCallhomeReport.json"
            step "Analyze schema"
            analyze_schema --output-format json --send-diagnostics=true
            sleep 5

            step "Compare actual and expected analyze-schema callhome data"
            compare_json_reports "${TEST_DIR}/expected_callhome_reports/analyse_schema_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

            rm -rf "${TEST_DIR}/actualCallhomeReport.json"
            step "Import schema"
            import_schema --send-diagnostics=true
            sleep 5

            step "Compare actual and expected import-schema callhome data"
            compare_json_reports "${TEST_DIR}/expected_callhome_reports/import_schema_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

            rm -rf "${TEST_DIR}/actualCallhomeReport.json"
            step "Run Export Data"
            export_data --send-diagnostics=true
            sleep 5

            step "Compare actual and expected export-data callhome data"
            compare_json_reports "${TEST_DIR}/expected_callhome_reports/export_data_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

            rm -rf "${TEST_DIR}/actualCallhomeReport.json"
            step "Run Import Data"
            import_data --send-diagnostics=true
            sleep 5

            step "Compare actual and expected import-data callhome data"
            compare_json_reports "${TEST_DIR}/expected_callhome_reports/import_data_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

            rm -rf "${TEST_DIR}/actualCallhomeReport.json"
            step "Finalize Schema"
            finalize_schema_post_data_import --send-diagnostics=true
            sleep 5

            step "Compare actual and expected finalize-schema callhome data"
            compare_json_reports "${TEST_DIR}/expected_callhome_reports/finalize_schema_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

            rm -rf "${TEST_DIR}/actualCallhomeReport.json"
            step "End Migration"
            end_migration --yes --send-diagnostics=true
            sleep 5

            step "Compare actual and expected end-migration callhome data"
            compare_json_reports "${TEST_DIR}/expected_callhome_reports/end_migration_callhome.json" "${TEST_DIR}/actualCallhomeReport.json"

            step "All commands passed successfully"
        fi
    fi

    popd
}

main
