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

# Order of env.sh import matters.
source ${TEST_DIR}/env.sh
source ${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh
source ${SCRIPTS}/yugabytedb/env.sh

source ${SCRIPTS}/functions.sh

main() {
	mkdir -p ${EXPORT_DIR}

	step "START: ${TEST_NAME}"
	print_env

	pushd ${TEST_DIR}

	step "Initialise source database."
	./init-db

	step "Export schema."
	export_schema
	find ${EXPORT_DIR}/schema -name '*.sql' | xargs grep -wh CREATE

	step "Analyze schema."
	analyze_schema
	tail -20 ${EXPORT_DIR}/reports/report.txt

	step "Fix schema."
	[ -x "${TEST_DIR}/fix-schema" ] && "${TEST_DIR}/fix-schema"

	step "Analyze schema."
	analyze_schema
	tail -20 ${EXPORT_DIR}/reports/report.txt

	step "Export data."
	export_data
	ls -l ${EXPORT_DIR}/data

	step "Create target database."
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
	run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME}"

	step "Import schema."
	import_schema
	run_ysql ${TARGET_DB_NAME} "\dt"

	step "Import data."
	import_data
}

main
