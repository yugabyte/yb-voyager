#!/usr/bin/env bash

set -e
set -m

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

if [ "${SOURCE_DB_TYPE}" = "oracle" ]
then
	source ${SCRIPTS}/${SOURCE_DB_TYPE}/live_env.sh 
else
	source ${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh
fi

source ${SCRIPTS}/yugabytedb/env.sh

source ${SCRIPTS}/functions.sh

main() {

	echo "Deleting the parent export-dir present in the test directory"
	rm -rf ${EXPORT_DIR}	
	echo "Creating export-dir in the parent test directory"
	mkdir -p ${EXPORT_DIR}
	echo "Assigning permissions to the export-dir to execute init-db, cleanup-db scripts"
	chmod +x ${TEST_DIR}/init-db ${TEST_DIR}/cleanup-db

	step "START: ${TEST_NAME}"
	print_env

	pushd ${TEST_DIR}

	step "Initialise source database."
	./init-db

	step "Grant source database user permissions"
	if [ "${SOURCE_DB_TYPE}" = "oracle" ]
	then
		grant_live_user_permission_oracle ${ORACLE_CDB_NAME} ${SOURCE_DB_NAME}
	else
		grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${SOURCE_DB_SCHEMA}
	fi
	
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

	step "Create target database."
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
	run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME}"

	step "Import schema."
	import_schema
	run_ysql ${TARGET_DB_NAME} "\dt"

	step "Export data."
	# false if exit code of export_data is non-zero
	export_data --live-migration || { 
		tail_log_file "yb-voyager.log"
		tail_log_file "debezium.log"
		exit 1
	} &

	# Storing the pid for the export data command
	exp_pid=$!

	# Killing the export process in case of failure
	trap "kill_process -${exp_pid}" SIGINT SIGTERM EXIT

	# Waiting for snapshot to complete
	( tail -f -n0 ${EXPORT_DIR}/logs/yb-voyager.log &) | timeout 100 grep -q "snapshot export is complete"

	ls -l ${EXPORT_DIR}/data
	cat ${EXPORT_DIR}/data/export_status.json || echo "No export_status.json found."
	cat ${EXPORT_DIR}/metainfo/dataFileDescriptor.json 

	step "Import data."
	import_data --live-migration &

	# Storing the pid for the import data command
	imp_pid=$!

	# Updating the trap command to include the importer
	trap "kill_process -${exp_pid} && kill_process -${imp_pid}" SIGINT SIGTERM EXIT

	sleep 30 
	
	step "Run snapshot validations."
	"${TEST_DIR}/snapvalidate"

	step "Inserting new events"
	run_file delta.sql

	sleep 30

	step "Shutting down exporter and importer"
	kill_process -${exp_pid}
	kill_process -${imp_pid}

	# Resetting the trap command
	trap - SIGINT SIGTERM EXIT

	step "Import remaining schema (FK, index, and trigger) and Refreshing MViews if present."
	import_schema --post-import-data --refresh-mviews
	run_ysql ${TARGET_DB_NAME} "\di"
	run_ysql ${TARGET_DB_NAME} "\dft" 

	step "Run final validations."
	if [ -x "${TEST_DIR}/validate" ]
	then
	"${TEST_DIR}/validate"
	fi

	step "Clean up"
	./cleanup-db
	rm -rf "${EXPORT_DIR}/*"
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
}

main
