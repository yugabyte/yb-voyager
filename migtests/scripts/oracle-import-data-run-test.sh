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
source ${SCRIPTS}/${SOURCE_DB_TYPE}/import_data_env.sh
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
	grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${SOURCE_DB_SCHEMA}

	step "Export data."
	# false if exit code of export_data is non-zero
	export_data || { 
		tail_log_file "yb-voyager.log"
		tail_log_file "debezium.log"
		exit 1
	}

	ls -l ${EXPORT_DIR}/data
	cat ${EXPORT_DIR}/data/export_status.json || echo "No export_status.json found."
	cat ${EXPORT_DIR}/metainfo/dataFileDescriptor.json

	step "Fix data."
	if [ -x "${TEST_DIR}/fix-data" ]
	then
		"${TEST_DIR}/fix-data" ${EXPORT_DIR}
	fi

    step "Truncate tables in target database"
    ./truncate_tables

    step "Create yb-voyager-metadata user"
    run_sqlplus_as_schema_owner ${TARGET_DB_NAME} ${TEST_DIR}/schema_and_data/create_metadata_user

	step "Import data."
	import_data

	step "Run validations."
	if [ -x "${TEST_DIR}/validate" ]
	then
		 "${TEST_DIR}/validate"
	fi

	step "Clean up"
	./cleanup-db
	rm -rf "${EXPORT_DIR}/*"
}

main
