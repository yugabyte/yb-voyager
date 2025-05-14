#!/usr/bin/env bash

set -e
set -x

if [ $# -gt 3 ]; then
    echo "Usage: $0 TEST_NAME [env.sh] [config_file]"
    exit 1
fi

# If exactly 2 arguments are passed, and the second argument ends with .yaml,
# treat it as the config_file and leave env.sh as an empty string
if [ $# -eq 2 ] && [[ "$2" == *.yaml ]]; then
    set -- "$1" "" "$2"
fi

export YB_VOYAGER_SEND_DIAGNOSTICS=false
export TEST_NAME=$1

export REPO_ROOT="${PWD}"
export SCRIPTS="${REPO_ROOT}/migtests/scripts"
export TESTS_DIR="${REPO_ROOT}/migtests/tests"
export TEST_DIR="${TESTS_DIR}/${TEST_NAME}"
export EXPORT_DIR=${EXPORT_DIR:-"${TEST_DIR}/export-dir"}

export PYTHONPATH="${REPO_ROOT}/migtests/lib"

# Order of env.sh import matters.
if [ $2 != "" ] #if env.sh is passed as an argument, source it
then
    if [ ! -f "${TEST_DIR}/$2" ]
	then
		echo "$2 file not found in the test directory"
		exit 1
	fi
	source ${TEST_DIR}/$2
else
	source ${TEST_DIR}/env.sh
fi

if [ "${SOURCE_DB_TYPE}" != "" ]; then
	source ${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh
fi

source ${SCRIPTS}/yugabytedb/env.sh
source ${SCRIPTS}/functions.sh

export CONFIG_FILE="${TEST_DIR}/configs/default_config.yaml"

# Handle optional additional config file ($3)
if [ -n "$3" ]; then
    CONFIG_FILE="${TEST_DIR}/configs/$3"
    if [ ! -f "${CONFIG_FILE}" ]; then
        echo "Error: Config file $3 not found in the test directory."
        exit 1
    fi
	export CONFIG_FILE
else
    echo "No Config file provided. Using default_config.yaml."
fi

main() {
	echo "Deleting the parent export-dir present in the test directory"
	rm -rf ${EXPORT_DIR}	
	echo "Creating export-dir in the parent test directory"
	mkdir -p ${EXPORT_DIR}
	echo "Assigning permissions to the export-dir to execute init-db script"

	for script in init-db init-target-db generate_config.py; do
	  if [ -f "${TEST_DIR}/${script}" ]; then
		chmod +x "${TEST_DIR}/${script}"
	  fi
	done

	step "START: ${TEST_NAME}"
	print_env

	pushd ${TEST_DIR}

	step "Check the Voyager version installed"
	yb-voyager version

	step "Initialise databases"

	for script in init-db init-target-db; do
	  if [ -f "${TEST_DIR}/${script}" ]; then
	    "${TEST_DIR}/${script}"
	  fi
	done

	step "Run additional steps in case of offline"
	if [ "${SOURCE_DB_TYPE}" != "" ]; then
		step "Grant source database user permissions"
		grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${SOURCE_DB_SCHEMA}

		step "Export data."
		# false if exit code of export_data is non-zero
		export_data || { 
			cat_log_file "yb-voyager-export-data.log"
			cat_log_file "debezium-source_db_exporter.log"
			exit 1
		}
	fi

	step "Run import with resumptions"
	${SCRIPTS}/resumption.py ${CONFIG_FILE}

	step "Run import-data-status"
	import_data_status

	expected_file="${TEST_DIR}/import_data_status-final-report.json"
	actual_file="${EXPORT_DIR}/reports/import-data-status-report.json"

	step "Verify import-data-status report"
	verify_report ${expected_file} ${actual_file}

	step "Clean up"
	rm -rf "${EXPORT_DIR}"

	if [ -n "${SOURCE_DB_NAME}" ]; then
		run_psql postgres "DROP DATABASE ${SOURCE_DB_NAME};"
	fi
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
}

main
