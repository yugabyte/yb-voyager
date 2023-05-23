#!/usr/bin/env bash

set -e

if [ $# -ne 1 ]
then
	echo "Usage: $0 TEST_NAME"
	exit 1
fi

set -x

export TEST_NAME=$1

export REPO_ROOT="${PWD}"
export SCRIPTS="${REPO_ROOT}/migtests/scripts"
export TESTS_DIR="${REPO_ROOT}/migtests/tests"
export TEST_DIR="${TESTS_DIR}/${TEST_NAME}"
export EXPORT_DIR=${EXPORT_DIR:-"${TEST_DIR}/export-dir"}

export PYTHONPATH="${REPO_ROOT}/migtests/lib"

# Order of env.sh import matters.
source ${TEST_DIR}/env.sh
source ${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh
source ${SCRIPTS}/yugabytedb/env.sh

source ${SCRIPTS}/functions.sh

main() {
	echo "Deleting the parent export-dir present in the test directory"
	rm -rf ${EXPORT_DIR}	
	echo "Creating export-dir in the parent test directory"
	mkdir -p ${EXPORT_DIR}
	echo "Assigning permissions to the export-dir to execute init-db script"
	chmod +x ${TEST_DIR}/init-db

	step "START: ${TEST_NAME}"
	print_env

	pushd ${TEST_DIR}

	step "Initialise source database."
	./init-db

	step "Grant source database user permissions"
	grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${SOURCE_DB_SCHEMA}

	step "Export schema."
	export_schema
	find ${EXPORT_DIR}/schema -name '*.sql' -printf "'%p'\n"| xargs grep -wh CREATE

	step "Analyze schema."
	analyze_schema
	tail -20 ${EXPORT_DIR}/reports/report.txt

	step "Fix schema."
	if [ -x "${TEST_DIR}/fix-schema" ]
	then
		 "${TEST_DIR}/fix-schema"
	fi

	step "Analyze schema."
	analyze_schema
	tail -20 ${EXPORT_DIR}/reports/report.txt

	step "Export data."
	{ #TRY
		export_data
	}|| { #CATCH
		# Print debezium.log file if present and exportDataDone flag file is not present
		if [ -f "${EXPORT_DIR}/logs/debezium.log" ] && [ ! -f "${EXPORT_DIR}/metainfo/flags/exportDataDone" ]
		then
			echo "Printing debezium.log file"
			tail -n 100 ${EXPORT_DIR}/logs/debezium.log
		else
			echo "No debezium.log found."
		fi	
		exit 1 #since the TRY block failed
	}

	ls -l ${EXPORT_DIR}/data

	step "Fix data."
	if [ -x "${TEST_DIR}/fix-data" ]
	then
		"${TEST_DIR}/fix-data" ${EXPORT_DIR}
	fi

	step "Create target database."
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
	run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME}"

	step "Import schema."
	import_schema
	run_ysql ${TARGET_DB_NAME} "\dt"

	step "Import data."
	import_data
	
	step "Import remaining schema (FK, index, and trigger) and Refreshing MViews if present."
	import_schema --post-import-data --refresh-mviews
	run_ysql ${TARGET_DB_NAME} "\di"
	run_ysql ${TARGET_DB_NAME} "\dft" 

	step "Run validations."
	if [ -x "${TEST_DIR}/validate" ]
	then
		 "${TEST_DIR}/validate"
	fi
}

main
