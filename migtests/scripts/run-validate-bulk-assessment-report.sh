#!/usr/bin/env bash

# The script takes up to two arguments:
# 1. TEST_NAME: The name of the test to run (mandatory).
# 2. env.sh: An optional environment setup script to source before running the test. If not provided, this file will be picked from the ${TESTS_DIR}

set -e

if [ $# -gt 2 ]
then
	echo "Usage: $0 TEST_NAME [env.sh]"
	exit 1
fi

set -x

export YB_VOYAGER_SEND_DIAGNOSTICS=false
export TEST_NAME=$1

export REPO_ROOT="${PWD}"
export SCRIPTS="${REPO_ROOT}/migtests/scripts"
export TESTS_DIR="${REPO_ROOT}/migtests/tests"
export TEST_DIR="${TESTS_DIR}/${TEST_NAME}"

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
source ${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh

source ${SCRIPTS}/functions.sh

export SOURCE_DB_TYPE="oracle"
export BULK_ASSESSMENT_DIR=${BULK_ASSESSMENT_DIR:-"${TEST_DIR}/bulk-assessment-dir"}
export EXPORT_DIR1=${EXPORT_DIR1:-"${BULK_ASSESSMENT_DIR}/${SOURCE_DB_NAME}-${SOURCE_DB_SCHEMA1}-export-dir"}
export EXPORT_DIR2=${EXPORT_DIR2:-"${BULK_ASSESSMENT_DIR}/${SOURCE_DB_ORACLE_TNS_ALIAS}-${SOURCE_DB_SCHEMA2}-export-dir"}

main() {

	echo "Deleting the parent bulk-assessment-dir present in the test directory"
	rm -rf ${BULK_ASSESSMENT_DIR}	
	echo "Creating export-dir in the parent test directory"
	mkdir -p ${BULK_ASSESSMENT_DIR}

	echo "Assigning permissions to the export-dir to execute init-db, cleanup-db scripts"
	chmod +x ${TEST_DIR}/init-db
	chmod +x ${TEST_DIR}/cleanup-db

	step "START: ${TEST_NAME}"
	print_env

	pushd ${TEST_DIR}

	step "Initialise source database."
	./init-db

	step "Grant source database user permissions"
	grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${SOURCE_DB_SCHEMA1}
	grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${SOURCE_DB_SCHEMA2}

	step "Bulk Assessment"
	bulk_assessment

	step "Validate Assessment Reports"

	echo "Verifying Bulk Assessment"

	compare_and_validate_reports "${BULK_ASSESSMENT_DIR}/bulk_assessment_report.html" "${BULK_ASSESSMENT_DIR}/bulk_assessment_report.json" "${TEST_DIR}/expected_reports/expectedErrorBulkAssessmentReport.json" "${BULK_ASSESSMENT_DIR}/logs/yb-voyager-assess-migration-bulk.log"

	echo "Verifying 1st child assessment"

	compare_and_validate_reports "${EXPORT_DIR1}/assessment/reports/migration_assessment_report.html" "${EXPORT_DIR1}/assessment/reports/migration_assessment_report.json" "${TEST_DIR}/expected_reports/expectedChild1AssessmentReport.json" "${EXPORT_DIR1}/logs/yb-voyager-assess-migration.log"
	
	step "Fix faulty parameter"
	fix_config_file fleet-config-file.csv

	step "Bulk Assessment"
	bulk_assessment

	step "Validate Assessment Reports"

	echo "Verifying Bulk Assessment"

	compare_and_validate_reports "${BULK_ASSESSMENT_DIR}/bulk_assessment_report.html" "${BULK_ASSESSMENT_DIR}/bulk_assessment_report.json" "${TEST_DIR}/expected_reports/expectedCompleteBulkAssessmentReport.json" "${BULK_ASSESSMENT_DIR}/logs/yb-voyager-assess-migration-bulk.log"

	echo "Verifying 1st child assessment"

	compare_and_validate_reports "${EXPORT_DIR1}/assessment/reports/migration_assessment_report.html" "${EXPORT_DIR1}/assessment/reports/migration_assessment_report.json" "${TEST_DIR}/expected_reports/expectedChild1AssessmentReport.json" "${EXPORT_DIR1}/logs/yb-voyager-assess-migration.log"

	echo "Verifying 2nd child assessment"

	compare_and_validate_reports "${EXPORT_DIR2}/assessment/reports/migration_assessment_report.html" "${EXPORT_DIR2}/assessment/reports/migration_assessment_report.json" "${TEST_DIR}/expected_reports/expectedChild2AssessmentReport.json" "${EXPORT_DIR2}/logs/yb-voyager-assess-migration.log"

	move_tables "${EXPORT_DIR2}/assessment/reports/migration_assessment_report.json" 100
	export_schema export_dir "${EXPORT_DIR1}" source_db_schema ${SOURCE_DB_SCHEMA1}
	export_schema export_dir "${EXPORT_DIR2}" source_db_schema ${SOURCE_DB_SCHEMA2}

	if [ -f "${EXPORT_DIR2}/schema/tables/table.sql.orig" ]; then
			echo "Assessment applied successfully to the schemas"
	fi

	step "Clean up"
	./cleanup-db

	rm -rf "${BULK_ASSESSMENT_DIR}"
}

main
