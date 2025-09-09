#!/usr/bin/env bash

set -e
set -x

if [ $# -lt 2 ] || [ $# -gt 4 ]; then
    echo "Usage: $0 TEST_NAME (--assess-only|--all-commands) [env.sh] [config_file]"
    exit 1
fi

MODE=$2

# Validate compulsory flag
if [ "$MODE" != "--assess-only" ] && [ "$MODE" != "--all-commands" ]; then
    echo "Error: MODE must be either --assess-only or --all-commands"
    exit 1
fi

# Parse remaining arguments: [env.sh] [config_file]
# $1 = TEST_NAME, $2 = MODE, $3 = env.sh (optional), $4 = config_file (optional)
if [ $# -eq 2 ]; then
    ENV_SH=""
    CONFIG_FILE=""
elif [ $# -eq 3 ]; then
    ENV_SH=$3
    CONFIG_FILE=""
elif [ $# -eq 4 ]; then
    ENV_SH=$3
    CONFIG_FILE=$4
else
    ENV_SH=""
    CONFIG_FILE=""
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
if [ -n "$ENV_SH" ]; then
    if [ ! -f "${TEST_DIR}/$ENV_SH" ]; then
        echo "$ENV_SH file not found in the test directory"
        exit 1
    fi
    source "${TEST_DIR}/$ENV_SH"
else
    source "${TEST_DIR}/env.sh"
fi

if [ -n "${SOURCE_DB_TYPE}" ]; then
    source "${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh"
fi

source "${SCRIPTS}/yugabytedb/env.sh"
source "${SCRIPTS}/functions.sh"

# Config file
if [ -n "$CONFIG_FILE" ]; then
    CONFIG_FILE="${TEST_DIR}/configs/$CONFIG_FILE"
    if [ ! -f "${CONFIG_FILE}" ]; then
        echo "Error: Config file $CONFIG_FILE not found in the test directory."
        exit 1
    fi
    export CONFIG_FILE
else
    export CONFIG_FILE="${TEST_DIR}/configs/default_config.yaml"
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
